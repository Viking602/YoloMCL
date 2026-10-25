package nativeui

import (
	"encoding/base64"
	"encoding/json"
	"os"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func (a *App) loadAppearance() {
	ac := a.account()
	a.appearanceErr = ""
	if ac == nil {
		a.appearance = launcher.Appearance{}
		a.appearanceAccount = ""
		return
	}
	id := ac.ID
	if a.appearanceAccount != id {
		// The first frame already has a real vanilla texture; it never shows
		// a synthetic mannequin while the account's appearance is loading.
		slim := defaultIsAlex(ac.UUID)
		a.appearance = launcher.Appearance{}
		a.preview.set(nil, nil, slim)
		a.model = a.tr().WardrobeModelSteve
		if slim {
			a.model = a.tr().WardrobeModelAlex
		}
	}
	a.appearanceLoading = true
	go func() {
		v, e := a.service.Appearance(a.ctx, id)
		a.post(func() {
			if id != a.snapshot.ActiveAccount {
				return
			}
			a.appearanceLoading = false
			if e != nil {
				// Shown inside the wardrobe, next to what failed; a page-wide
				// banner would outlive the account it was about.
				a.appearanceErr = a.friendlyErr(e)
				a.appearance = launcher.Appearance{}
				return
			}
			a.appearanceErr = ""
			clear(a.wd().capeFailed)
			t := a.tr()
			a.appearance = v
			a.appearanceAccount = id
			a.model = t.WardrobeModelSteve
			if v.Model == "slim" {
				a.model = t.WardrobeModelAlex
			}
			a.preview.set(imageFromData(v.Skin), imageFromData(v.Cape), v.Model == "slim")
			if a.offlineDefault() {
				a.showVanilla(normalizeSkinModel(v.Model))
			}
			if h := skinHead(imageFromData(v.Skin)); h != nil {
				a.accountHead(launcher.Account{ID: id}) // ensure cache maps exist
				a.avatars.heads[id] = ui.NewBitmap(h).Pixelated()
			}
			a.selectedCape = t.WardrobeCapeNone
			for _, cape := range v.Capes {
				if cape.Active {
					a.selectedCape = cape.Name
				}
			}
		})
	}()
}

// skinWhy explains a disabled skin/cape action on the wardrobe page.
func (a *App) skinWhy() string {
	t := a.tr()
	if a.appearanceErr != "" {
		return t.WhySkinLoadFailed
	}
	return t.WhySkinUnsupported
}

func (a *App) upload(kind string) {
	ac := a.account()
	if ac == nil {
		return
	}
	id := ac.ID
	a.chooseFiles("png", func(paths []string) {
		b, e := os.ReadFile(paths[0])
		if e != nil {
			a.message = a.friendlyErr(e)
			return
		}
		model := normalizeSkinModel(a.model)
		a.execute("uploadTexture", map[string]string{"accountId": id, "kind": kind, "model": model, "data": base64.StdEncoding.EncodeToString(b)}, func(json.RawMessage) {
			a.loadAppearance()
			a.notice(a.tr().WardrobeTextureSaved)
		})
	})
}
