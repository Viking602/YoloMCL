package nativeui

import (
	"context"
	"time"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// loadMods loads the current version's mods or packs (per a.addonTab).
// Each request carries a generation: a result that arrives after another
// switch is dropped, so stale lists never flash in. Results are cached per
// root|version|tab, so switching back shows the list at once while a
// quiet refresh runs.
func (a *App) loadMods() {
	a.modsGen++
	gen := a.modsGen
	if a.modsCancel != nil {
		a.modsCancel()
		a.modsCancel = nil
	}
	if a.modCheckCancel != nil {
		a.modCheckCancel()
		a.modCheckCancel, a.modChecking = nil, false
	}
	r := a.root()
	if r == nil {
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
	if c, ok := a.addonCache[key]; ok {
		if tab == "mods" {
			a.mods = c.mods
		} else {
			a.assetPacks = c.packs
		}
		a.modsLoading = false
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
			mods, e = a.service.ListModsStreamContext(ctx, id, ver, func(part []launcher.Mod) {
				a.post(func() {
					if gen == a.modsGen && ctx.Err() == nil {
						a.mods = part
					}
				})
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
			a.modsLoading = false
			if e != nil {
				a.message = a.friendlyErr(e)
				a.errorMessage = true
				return
			}
			a.cacheAddons(key, addonCacheEntry{mods: mods, packs: packs})
			if tab == "mods" {
				a.mods = mods
			} else {
				a.assetPacks = packs
			}
		})
	}()
}

type addonCacheEntry struct {
	mods  []launcher.Mod
	packs []launcher.AssetPack
}

// cacheAddons keeps the most recent addonCacheSize lists (LRU).
func (a *App) cacheAddons(key string, v addonCacheEntry) {
	if a.addonCache == nil {
		a.addonCache = map[string]addonCacheEntry{}
	}
	a.addonCache[key] = v
	for i, k := range a.addonOrder {
		if k == key {
			a.addonOrder = append(a.addonOrder[:i], a.addonOrder[i+1:]...)
			break
		}
	}
	a.addonOrder = append(a.addonOrder, key)
	for len(a.addonOrder) > addonCacheSize {
		delete(a.addonCache, a.addonOrder[0])
		a.addonOrder = a.addonOrder[1:]
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
	}
}
