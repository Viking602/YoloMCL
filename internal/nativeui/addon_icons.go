package nativeui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// localIcon decodes an icon read from disk once, off the UI thread, and
// caches it (misses too) under key; nil until it is ready.
func (a *App) localIcon(key string, load func() ([]byte, error)) ui.ImageSource {
	a.imageMu.Lock()
	cached, pending, retry := a.cachedImageLocked(key), a.imagePendingLocked(key), a.imageRetry[key]
	if cached != nil || pending || !retry.IsZero() {
		a.imageMu.Unlock()
		return cached
	}
	ctx := a.imageContextLocked()
	a.reserveImageLocked(key, ctx)
	a.imageMu.Unlock()
	ready := a.firstFrame
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		if !waitForFirstFrame(ctx, ready) {
			a.clearImagePending(key, ctx)
			return
		}
		a.loadLocalImage(ctx, key, load)
		if ctx.Err() == nil {
			a.post(func() {})
		}
	}()
	return nil
}

// Both overview preloading and visible widgets reserve imagePending before
// entering here, so a large screenshot is never decoded by two workers.
func (a *App) loadLocalImage(ctx context.Context, key string, load func() ([]byte, error)) {
	defer a.clearImagePending(key, ctx)
	slots := a.imageSlots
	if strings.HasPrefix(key, "screenshot:") {
		slots = a.overviewImageSlots
	}
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	case <-ctx.Done():
		return
	}
	if ctx.Err() != nil {
		return
	}
	b, err := load()
	var source ui.ImageSource
	var content imageContentKey
	if err == nil && ctx.Err() == nil {
		source, content, _ = a.decodeImageContent(ctx, b, localImageEdge(key))
	}
	if ctx.Err() != nil {
		return
	}
	a.imageMu.Lock()
	defer a.imageMu.Unlock()
	if ctx.Err() != nil || a.imageRequests[key] != ctx {
		return
	}
	if source != nil {
		a.cacheImageContentLocked(key, source, len(b), content)
	} else {
		a.failImageLocked(key, 24*time.Hour)
	}
}

// Lists and dialogs share artwork and its cache. An explicit version keeps a
// closing dialog tied to its original mod if the selected instance changes.
func (a *App) addonIcon(rootID, version, kind, file, iconURL, sha1 string) ui.ImageSource {
	var key string
	if sha1 != "" {
		// The same jar in several packs has identical embedded artwork.
		// Share one bitmap even when its path or enabled state differs.
		key = "addon:sha1:" + sha1
	} else {
		key = "addon:" + rootID + ":" + version + ":" + kind + ":" + stableFile(file)
	}
	icon := a.localIcon(key, func() ([]byte, error) {
		b, e := a.service.AddonIcon(rootID, version, kind, file)
		if e != nil { // renamed meanwhile: try the other name
			b, e = a.service.AddonIcon(rootID, version, kind, toggledName(file))
		}
		return b, e
	})
	a.imageMu.Lock()
	missed := !a.imageRetry[key].IsZero()
	a.imageMu.Unlock()
	if icon == nil && iconURL != "" && missed {
		return a.remoteImageSize(iconURL, 96)
	}
	return icon
}

// fetchModIcons looks up project icons for the listed mods once per list.
func (a *App) fetchModIcons() {
	r := a.root()
	if r == nil || a.service == nil || len(a.mods) == 0 {
		return
	}
	key := r.ID + "|" + r.SelectedVersion + "|" + fmt.Sprint(len(a.mods))
	if a.modIconAsked[key] {
		return
	}
	if a.modIconAsked == nil {
		a.modIconAsked = map[string]bool{}
		a.modIconURLs = map[string]string{}
	}
	a.modIconAsked[key] = true
	id, ver := r.ID, r.SelectedVersion
	refs := make([]launcher.AddonRef, 0, len(a.mods))
	for _, m := range a.mods {
		refs = append(refs, launcher.AddonRef{SHA1: m.SHA1, File: m.File, Fingerprint: m.Fingerprint})
	}
	a.imageMu.Lock()
	ctx := a.imageContextLocked()
	a.imageMu.Unlock()
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		info, _ := a.service.AddonRemoteInfo(ctx, id, ver, "mod", refs)
		a.post(func() {
			if ctx.Err() != nil {
				delete(a.modIconAsked, key)
				return
			}
			if _, cached := a.addonCache[id+"|"+ver+"|mods"]; !cached && a.modsKey != id+"|"+ver+"|mods" {
				delete(a.modIconAsked, key)
				return
			}
			if a.addonRemote == nil {
				a.addonRemote = map[string]launcher.RemoteInfo{}
			}
			for k, v := range info {
				a.addonRemote[k] = v
				if v.Icon != "" {
					a.modIconURLs[k] = v.Icon
				}
			}
		})
	}()
}

// fetchPackIcons does the same for resource packs and shaders: most shader
// zips carry no icon, but Modrinth and CurseForge know the project.
func (a *App) fetchPackIcons(kind string) {
	r := a.root()
	if r == nil || a.service == nil || len(a.assetPacks) == 0 {
		return
	}
	key := kind + "|" + r.ID + "|" + r.SelectedVersion + "|" + fmt.Sprint(len(a.assetPacks))
	if a.packRemoteAsked[key] {
		return
	}
	if a.packRemoteAsked == nil {
		a.packRemoteAsked = map[string]bool{}
	}
	a.packRemoteAsked[key] = true
	id, ver := r.ID, r.SelectedVersion
	packs := append([]launcher.AssetPack(nil), a.assetPacks...)
	a.imageMu.Lock()
	ctx := a.imageContextLocked()
	a.imageMu.Unlock()
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		refs := a.service.PackRefsContext(ctx, id, ver, kind, packs)
		info, _ := a.service.AddonRemoteInfo(ctx, id, ver, kind, refs)
		a.post(func() {
			if ctx.Err() != nil {
				delete(a.packRemoteAsked, key)
				return
			}
			if a.addonRemote == nil {
				a.addonRemote = map[string]launcher.RemoteInfo{}
			}
			for _, ref := range refs {
				if v, ok := info[ref.SHA1]; ok {
					a.addonRemote[kind+"|"+stableFile(ref.File)] = v
				}
			}
		})
	}()
}

func (a *App) versionIcon(rootID, versionID string) ui.ImageSource {
	isPack := false
	if r := a.rootByID(rootID); r != nil && r.Pack != nil {
		isPack = true
	}
	return a.localIcon("version:"+rootID+":"+versionID, func() ([]byte, error) {
		if isPack {
			a.service.EnsurePackIcon(rootID)
		}
		return a.service.VersionIcon(rootID, versionID)
	})
}
