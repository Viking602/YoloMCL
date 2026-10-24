package nativeui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// localIcon decodes an icon read from disk once, off the UI thread, and
// caches it (misses too) under key; nil until it is ready.
func (a *App) localIcon(key string, load func() ([]byte, error)) ui.ImageSource {
	a.imageMu.Lock()
	cached, pending, retry := a.images[key], a.imagePending[key], a.imageRetry[key]
	if cached != nil || pending || !retry.IsZero() {
		a.imageMu.Unlock()
		return cached
	}
	a.imagePending[key] = true
	a.imageMu.Unlock()
	ready := a.firstFrame
	go func() {
		if !waitForFirstFrame(a.ctx, ready) {
			return
		}
		select {
		case a.imageSlots <- struct{}{}:
			defer func() { <-a.imageSlots }()
		case <-a.ctx.Done():
			return
		}
		var src ui.ImageSource
		if b, e := load(); e == nil {
			src, _ = decodeResourceImage(b)
		}
		a.imageMu.Lock()
		delete(a.imagePending, key)
		if src != nil {
			a.images[key] = src
		} else {
			a.imageRetry[key] = time.Now().Add(24 * time.Hour)
		}
		a.imageMu.Unlock()
		a.post(func() {})
	}()
	return nil
}

// Lists and dialogs share artwork and its cache. An explicit version keeps a
// closing dialog tied to its original mod if the selected instance changes.
func (a *App) addonIcon(rootID, version, kind, file, iconURL string) ui.ImageSource {
	key := "addon:" + rootID + ":" + version + ":" + kind + ":" + stableFile(file)
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
		return a.remoteImage(iconURL)
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
		refs = append(refs, launcher.AddonRef{SHA1: m.SHA1, File: m.File})
	}
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		info, _ := a.service.AddonRemoteInfo(a.ctx, id, ver, "mod", refs)
		a.post(func() {
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
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		refs := a.service.PackRefs(id, ver, kind, packs)
		info, _ := a.service.AddonRemoteInfo(a.ctx, id, ver, kind, refs)
		a.post(func() {
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
