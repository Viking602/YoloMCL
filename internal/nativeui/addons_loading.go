package nativeui

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

const (
	addonCacheSize   = 24
	addonCacheBudget = 4 << 20
)

// loadMods loads the current version's mods or packs (per a.addonTab).
// Each request carries a generation: a result that arrives after another
// switch is dropped, so stale lists never flash in. Results are cached per
// root|version|tab, so switching back shows the list at once while a
// quiet refresh runs.
func (a *App) loadMods() { a.loadAddons(true) }

// Navigation reuses a finished recent scan and joins an in-flight one. Explicit
// refreshes and file mutations still call loadMods to bypass this short grace.
func (a *App) ensureAddons() { a.loadAddons(false) }

func (a *App) cancelAddonLoad() {
	a.addonListData = addonListData{}
	if a.modsRefreshing && a.modsLoadKey == a.modsKey {
		// A streaming snapshot is temporary, not a completed list to retain
		// offscreen. Keep the last complete result if this was a refresh.
		a.mods = a.addonCache[a.modsKey].mods
	}
	a.modsGen++
	if a.modsCancel != nil {
		a.modsCancel()
		a.modsCancel = nil
	}
	if a.modCheckCancel != nil {
		a.modCheckCancel()
		a.modCheckCancel, a.modChecking = nil, false
		// A quick return must resume an interrupted update check. Keep the
		// ready rows but do not treat that list as a fresh completed scan.
		if cached, ok := a.addonCache[a.modsLoadKey]; ok {
			cached.loaded = time.Time{}
			a.addonCache[a.modsLoadKey] = cached
		}
	}
	a.modsRefreshing, a.modsLoading = false, false
}

func (a *App) loadAddons(force bool) {
	r := a.root()
	if r == nil {
		a.cancelAddonLoad()
		a.mods = nil
		a.assetPacks = nil
		a.modsLoading = false
		return
	}
	// The first view must not repeat the request started by ShowPage.
	if a.instLoaded != r.ID {
		a.instScrollPending = true
	}
	a.instLoaded = r.ID
	id, ver := r.ID, r.SelectedVersion
	tab := a.addonTab
	if tab == "" {
		tab = "mods"
	}
	key := id + "|" + ver + "|" + tab
	modKey := id + "|" + ver + "|mods"
	if !force && a.modsRefreshing && a.modsLoadKey == key {
		return
	}
	a.cancelAddonLoad()
	gen := a.modsGen
	a.modsLoadKey = key
	if a.modsKey != modKey {
		a.mods, a.assetPacks = nil, nil
		a.modsKey = modKey
		if c, ok := a.addonCache[modKey]; ok {
			a.mods = c.mods
		}
	}
	if c, ok := a.addonCache[key]; ok {
		a.touchAddons(key)
		if tab == "mods" {
			a.mods = c.mods
		} else {
			a.assetPacks = c.packs
		}
		a.modsLoading = false
		if !force && !c.loaded.IsZero() && time.Since(c.loaded) < 5*time.Second {
			return
		}
	} else {
		if tab == "mods" {
			a.mods = nil
		} else {
			a.assetPacks = nil
		}
		a.modsLoading = true
	}
	if a.service == nil {
		return
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.modsCancel = cancel
	a.modsRefreshing = true
	a.refreshGameStateContext(ctx)
	ready := a.firstFrame
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		if !waitForFirstFrame(ctx, ready) {
			return
		}
		var mods []launcher.Mod
		var packs []launcher.AssetPack
		var e error
		if tab == "resourcepack" || tab == "shader" {
			packs, e = a.service.ListAssetPacksAt(id, ver, tab)
		} else {
			// Rows stream in while new jars are read; cached ones are instant.
			var progress modListProgress
			defer progress.clear()
			mods, e = a.service.ListModsStreamContext(ctx, id, ver, func(part []launcher.Mod) {
				progress.publish(a, ctx, gen, part)
			})
		}
		if ctx.Err() != nil {
			return
		}
		// This checks files and takes the service lock: never do it inside
		// the UI callback, where a selection save can otherwise block a frame.
		check := tab == "mods" && e == nil && len(mods) > 0 && a.service.ModChecksAge(id, ver) > 6*time.Hour
		a.post(func() {
			if gen != a.modsGen || ctx.Err() != nil {
				return // superseded by a later switch
			}
			if check {
				a.startModCheck(gen, key, id, ver)
			}
			a.modsLoading, a.modsRefreshing = false, false
			if e != nil {
				a.message = a.friendlyErr(e)
				a.errorMessage = true
				return
			}
			if tab == "mods" {
				a.mods = mods
			} else {
				a.assetPacks = packs
			}
			a.cacheAddons(key, addonCacheEntry{mods: mods, packs: packs, loaded: time.Now()})
		})
	}()
}

type addonCacheEntry struct {
	mods   []launcher.Mod
	packs  []launcher.AssetPack
	bytes  int
	loaded time.Time
}

// modListProgress keeps only the latest snapshot while the UI is busy. The
// queued callback owns this slot, not every intermediate full mod list.
type modListProgress struct {
	mu     sync.Mutex
	latest []launcher.Mod
	queued bool
}

func (p *modListProgress) clear() {
	p.mu.Lock()
	p.latest = nil
	p.mu.Unlock()
}

func (p *modListProgress) publish(a *App, ctx context.Context, gen int, mods []launcher.Mod) {
	p.mu.Lock()
	if ctx.Err() != nil {
		p.mu.Unlock()
		return
	}
	p.latest = mods
	if p.queued {
		p.mu.Unlock()
		return
	}
	p.queued = true
	p.mu.Unlock()
	a.post(func() {
		p.mu.Lock()
		latest := p.latest
		p.latest, p.queued = nil, false
		p.mu.Unlock()
		if latest != nil && gen == a.modsGen && ctx.Err() == nil {
			a.mods = latest
		}
	})
}

// cacheAddons bounds recent lists by both entries and their retained data.
// Keep the current mod list and newly requested tab even if a single pack is
// oversized: a.mods still owns it, so evicting it saves nothing and would only
// trigger a reload on return. Jar metadata stays on disk after eviction.
func (a *App) cacheAddons(key string, v addonCacheEntry) {
	if a.addonCache == nil {
		a.addonCache = map[string]addonCacheEntry{}
	}
	a.addonCacheBytes -= a.addonCache[key].bytes
	v.bytes = v.memoryBytes()
	a.addonCache[key] = v
	a.addonCacheBytes += v.bytes
	a.touchAddons(key)
	evicted := false
	for len(a.addonOrder) > addonCacheSize || (a.addonCacheBytes > addonCacheBudget && len(a.addonOrder) > 1) {
		drop := -1
		for i, candidate := range a.addonOrder {
			if candidate != key && (candidate != a.modsKey || len(a.mods) == 0) {
				drop = i
				break
			}
		}
		if drop < 0 {
			break
		}
		oldest := a.addonOrder[drop]
		a.addonCacheBytes -= a.addonCache[oldest].bytes
		delete(a.addonCache, oldest)
		a.addonOrder = slices.Delete(a.addonOrder, drop, drop+1)
		evicted = true
	}
	if evicted {
		a.pruneAddonMetadata()
	}
}

func (a *App) touchAddons(key string) {
	for i, k := range a.addonOrder {
		if k == key {
			a.addonOrder = slices.Delete(a.addonOrder, i, i+1)
			break
		}
	}
	a.addonOrder = append(a.addonOrder, key)
}

func (v addonCacheEntry) memoryBytes() int {
	stringsBytes := func(values ...string) int {
		n := 0
		for _, value := range values {
			n += len(value)
		}
		return n
	}
	sliceBytes := func(values []string) int {
		return cap(values)*int(unsafe.Sizeof("")) + stringsBytes(values...)
	}
	n := cap(v.mods)*int(unsafe.Sizeof(launcher.Mod{})) + cap(v.packs)*int(unsafe.Sizeof(launcher.AssetPack{}))
	for _, m := range v.mods {
		n += stringsBytes(m.File, m.Name, m.ModID, m.Version, m.Loader, m.SHA1, m.Source, m.ProjectID, m.VersionID, m.Status, m.Description)
		n += sliceBytes(m.Provides) + sliceBytes(m.Requires)
		if p := m.Latest; p != nil {
			n += int(unsafe.Sizeof(*p)) + stringsBytes(p.ID, p.ProjectID, p.Source, p.Name, p.Number, p.Published, p.Type, p.Changelog)
			n += sliceBytes(p.GameVersions) + sliceBytes(p.Loaders)
			n += cap(p.Files)*int(unsafe.Sizeof(launcher.DownloadFile{})) + cap(p.Dependencies)*int(unsafe.Sizeof(launcher.Dependency{}))
			for _, f := range p.Files {
				n += stringsBytes(f.Name, f.URL, f.SHA1, f.SHA512, f.SHA256)
			}
			for _, d := range p.Dependencies {
				n += stringsBytes(d.ProjectID, d.VersionID, d.Kind)
			}
		}
	}
	for _, p := range v.packs {
		n += stringsBytes(p.File, p.Name, p.Description, p.Kind)
	}
	return n
}

// Platform descriptions and icons follow the same lifetime as their lists.
// Keeping them in separate maps must not undo eviction of an old pack.
func (a *App) pruneAddonMetadata() {
	keep := map[string]bool{}
	requests := map[string]bool{}
	add := func(mods []launcher.Mod, packs []launcher.AssetPack) {
		for _, m := range mods {
			keep[m.SHA1] = true
		}
		for _, p := range packs {
			keep[p.Kind+"|"+stableFile(p.File)] = true
		}
	}
	add(a.mods, a.assetPacks)
	for key, entry := range a.addonCache {
		add(entry.mods, entry.packs)
		if i := strings.LastIndex(key, "|"); i >= 0 {
			prefix, kind := key[:i], key[i+1:]
			if kind == "mods" {
				requests[prefix+"|"] = true
			} else {
				requests[kind+"|"+prefix+"|"] = true
			}
		}
	}
	if r := a.root(); r != nil {
		prefix := r.ID + "|" + r.SelectedVersion + "|"
		requests[prefix], requests["shader|"+prefix], requests["resourcepack|"+prefix] = true, true, true
	}
	for key := range a.addonRemote {
		if !keep[key] {
			delete(a.addonRemote, key)
		}
	}
	for key := range a.modIconURLs {
		if !keep[key] {
			delete(a.modIconURLs, key)
		}
	}
	for _, asked := range []map[string]bool{a.modIconAsked, a.packRemoteAsked} {
		for key := range asked {
			if i := strings.LastIndex(key, "|"); i >= 0 && !requests[key[:i+1]] {
				delete(asked, key)
			}
		}
	}
}

// startModCheck runs the version check in the background once the list is
// on screen; update badges are patched into rows as results arrive. It is
// cancelled when the tab or pack changes (loadMods).
func (a *App) startModCheck(gen int, key, id, ver string) {
	ctx, cancel := context.WithCancel(a.ctx)
	a.modCheckCancel, a.modChecking = cancel, true
	go func() {
		defer cancel()
		e := a.service.CheckModsBackground(ctx, id, ver, func(batch map[string]launcher.Mod) {
			a.post(func() {
				if gen != a.modsGen {
					return
				}
				a.patchModChecks(key, batch)
			})
		})
		a.post(func() {
			if gen != a.modsGen {
				return
			}
			a.modChecking, a.modCheckCancel = false, nil
			if e != nil && ctx.Err() == nil {
				logUIError("mod check: " + e.Error())
			}
		})
	}()
}

func (a *App) patchModChecks(key string, batch map[string]launcher.Mod) {
	patch := func(list []launcher.Mod) {
		for i := range list {
			if c, ok := batch[list[i].SHA1]; ok && list[i].SHA1 != "" {
				list[i].Source, list[i].ProjectID, list[i].VersionID, list[i].Latest, list[i].Status = c.Source, c.ProjectID, c.VersionID, c.Latest, c.Status
			}
		}
	}
	patch(a.mods)
	if c, ok := a.addonCache[key]; ok {
		patch(c.mods)
		a.cacheAddons(key, c)
	}
}
