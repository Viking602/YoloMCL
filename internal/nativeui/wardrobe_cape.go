package nativeui

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/draw"
	"image/png"
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// capeFront crops the outer face of a cape texture (the side seen from behind).
func capeFront(img image.Image) image.Image {
	if img == nil {
		return nil
	}
	b := img.Bounds()
	u := float64(b.Dx()) / 64
	r := image.Rect(b.Min.X+int(1*u), b.Min.Y+int(1*u), b.Min.X+int(11*u), b.Min.Y+int(17*u))
	out := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(out, out.Bounds(), img, r.Min, draw.Src)
	return out
}

func (a *App) capeImage(cape launcher.Cape) image.Image {
	w := a.wd()
	if img := w.capeImgs[cape.URL]; img != nil {
		return img
	}
	if cape.Active && a.appearance.Cape != "" {
		if img := imageFromData(a.appearance.Cape); img != nil {
			w.capeImgs[cape.URL] = img
			return img
		}
	}
	if cape.URL != "" && a.service != nil && !w.capeAsked[cape.URL] && !time.Now().Before(w.capeRetry[cape.URL]) {
		w.capeAsked[cape.URL] = true
		account := a.appearanceAccount
		a.bgWG.Add(1)
		go func() {
			defer a.bgWG.Done()
			b, err := a.service.ResourceImage(a.ctx, cape.URL)
			var img image.Image
			if err == nil {
				img, err = decodeCapeImage(b)
			}
			a.post(func() {
				a.finishCapeImage(w, account, cape, b, img, err)
			})
		}()
	}
	return nil
}

func decodeCapeImage(data []byte) (image.Image, error) {
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if config.Width > 4096 || config.Height > 4096 {
		return nil, errors.New("cape texture dimensions exceed 4096 pixels")
	}
	return png.Decode(bytes.NewReader(data))
}

// finishCapeImage runs on the UI thread. A shared URL may be cached after an
// account switch, but its old request must not overwrite the current preview.
func (a *App) finishCapeImage(w *wardrobeState, account string, cape launcher.Cape, data []byte, img image.Image, err error) {
	delete(w.capeAsked, cape.URL)
	if err != nil {
		logUIError("cape image: " + err.Error())
		if w.capeRetry == nil {
			w.capeRetry = map[string]time.Time{}
		}
		w.capeRetry[cape.URL] = time.Now().Add(20 * time.Second)
		return
	}
	delete(w.capeRetry, cape.URL)
	if w.capeRaw == nil {
		w.capeRaw = map[string][]byte{}
	}
	w.capeRaw[cape.URL], w.capeImgs[cape.URL] = data, img
	if a.wardrobeSt != w || a.appearanceAccount != account || a.preview == nil || a.selectedCape != cape.Name {
		return
	}
	for _, current := range a.appearance.Capes {
		if current.Name == a.selectedCape && current.URL == cape.URL {
			a.preview.cape, a.preview.dirty = img, true
			return
		}
	}
}

func (a *App) activeCapeName() string {
	for _, c := range a.appearance.Capes {
		if c.Active {
			return c.Name
		}
	}
	return a.tr().WardrobeCapeNone
}

// previewCape shows a cape on the model and turns it around.
func (a *App) previewCape(name string) {
	a.selectedCape = name
	var img image.Image
	for _, c := range a.appearance.Capes {
		if c.Name == name {
			img = a.capeImage(c)
		}
	}
	a.preview.cape = img
	a.preview.dirty = true
	if name != a.tr().WardrobeCapeNone {
		a.preview.turnTo(math.Pi - .35)
	} else {
		a.preview.turnTo(-.35)
	}
}

func (a *App) wardrobeCapeCard(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	ac := a.account()
	w := a.wd()
	wardrobeCard(c, "cape-options", func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, t.WardrobeCapeTitle).FontSize(typeHeadline * s).FontWeight(700).TextColor(ink).Grow(1)
		})
		if ac.Kind != "microsoft" {
			ui.Text(c, t.WardrobeCapeLocalDesc).FontSize(typeFootnote * s).TextColor(muted).FillWidth()
			ui.Row(c).Gap(8 * s).Children(func() {
				if buttonWhy(c, "upload-cape", t.WardrobeUploadCapeBtn, "plus", true, whyOf(a.texturePending("cape"), t.WhyBusy, !a.appearance.CanCape, a.skinWhy()), false) {
					a.upload("cape")
				}
				// Nothing to remove without a cape, so the button only shows with one.
				if a.appearance.Cape != "" && button(c, "reset-cape", t.WardrobeRemoveCapeBtn, "", false, a.texturePending("cape")) {
					a.execute("resetTexture", map[string]string{"accountId": ac.ID, "kind": "cape"}, func(json.RawMessage) { a.loadAppearance() })
				}
			})
			if ac.Kind == "offline" {
				ui.Text(c, t.WardrobeOfflineNote1).FontSize(typeCaption * s).TextColor(muted).FillWidth()
			}
			return
		}
		if a.appearanceLoading && len(a.appearance.Capes) == 0 {
			ui.Row(c).Gap(8 * s).AlignItems(ui.Center).Children(func() {
				statusSpinner(c, 12*s)
				ui.Text(c, a.L("正在读取披风", "Loading capes")).FontSize(typeFootnote * s).TextColor(muted)
			})
			return
		}
		active := a.activeCapeName()
		const tileW, tileH = 72, 100
		ui.Row(c).FillWidth().Wrap().Gap(10 * s).Children(func() {
			a.capeTile(c, noCapeKey, t.WardrobeCapeNone, nil, active == t.WardrobeCapeNone, tileW*s, tileH*s)
			for _, cape := range a.appearance.Capes {
				cp := cape
				var thumb *ui.Bitmap
				if bm := w.capeThumbs[cp.URL]; bm != nil {
					thumb = bm
				} else if img := a.capeImage(cp); img != nil {
					thumb = ui.NewBitmap(capeFront(img)).Pixelated()
					w.capeThumbs[cp.URL] = thumb
				}
				a.capeTile(c, cp.ID, cp.Name, thumb, cp.Active, tileW*s, tileH*s)
			}
		})
		if len(a.appearance.Capes) == 0 {
			emptyNote(c, "capes-empty", "account", a.L("这个账号还没有披风", "This account has no capes yet"), "")
		}
		ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
			changed := a.selectedCape != active
			hint := ""
			if changed {
				hint = a.L("预览中，应用后对游戏生效", "Previewing. Apply to use it in game")
			}
			ui.Text(c, hint).FontSize(typeCaption * s).TextColor(muted).Grow(1).MinWidth(0)
			if raw := a.capeBytes(a.selectedCape); len(raw) > 0 && fixedButton(c, "save-cape", a.L("保存", "Save"), "download", 0, btnMd*s, false, false) {
				a.saveTexture(safePNGName(a.selectedCape, "_cape"), raw, true)
			}
			why := whyOf(a.texturePending("cape") || w.applying, t.WhyBusy, !a.appearance.CanCape, a.skinWhy())
			if fixedButton(c, "apply-cape", t.WardrobeApplyCapeBtn, "check", 0, btnMd*s, true, !changed || why != "") {
				a.applyCape()
			}
		})
	})
}

func itoaN(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func (a *App) capeTile(c *ui.Context, key, name string, thumb *ui.Bitmap, inUse bool, w, h float32) {
	s := uiScale(c)
	sel := a.selectedCape == name
	tip := name
	if inUse {
		tip = name + a.L("（使用中）", " (in use)")
	}
	// Fixed box: thumbnail, one name line, nothing else, so rows align.
	b := ui.ButtonBase(c.Key("cape-"+key)).Label(name).Tooltip(tip).Size(w, h).Shrink(0).Column().Gap(5 * s).AlignItems(ui.Center)
	edge := ui.Transparent
	if sel {
		edge = accent
	} else if b.Hovered() {
		edge = ui.RGBA(60, 72, 110, .22)
	}
	b.Children(func() {
		ui.Box(c).Size(w, h-22*s).Shrink(0).Radius(10*s).Background(ui.RGBA(120, 130, 160, .10)).Border(2, b.AnimateColor("edge", edge, fastAnim)).Center().Children(func() {
			if key == noCapeKey {
				ui.Icon(c, icons["cross"]).Size(16*s, 16*s).TextColor(muted)
			} else if thumb != nil {
				ui.Image(c, thumb).Size(30*s, 48*s)
			} else {
				statusSpinner(c, 12*s)
			}
			if inUse {
				ui.Box(c).Absolute().Top(5*s).Right(5*s).Size(16*s, 16*s).Radius(999).Background(successInk).Border(1.5, panel).Center().Children(func() {
					ui.Icon(c, icons["check"]).Size(9*s, 9*s).TextColor(panel)
				})
			}
		})
		fg := inkSecondary
		if sel {
			fg = accent
		}
		ui.Text(c, name).FontSize(typeCaption * s).TextColor(fg).SingleLine().Ellipsis("…").MaxWidth(w).Height(16 * s).Shrink(0)
	})
	if b.Clicked() && !sel {
		a.previewCape(name)
	}
}

// applyCape sets the selected cape; a failure puts the preview back.
func (a *App) applyCape() {
	ac := a.account()
	w := a.wd()
	if ac == nil || w.applying {
		return
	}
	id := ""
	for _, cape := range a.appearance.Capes {
		if cape.Name == a.selectedCape {
			id = cape.ID
		}
	}
	prev := a.activeCapeName()
	acc := ac.ID
	w.applying = true
	go func() {
		e := a.service.SetCape(a.ctx, acc, id)
		a.post(func() {
			w.applying = false
			if e != nil {
				a.message, a.errorMessage = a.friendlyErr(e), true
				a.previewCape(prev)
				return
			}
			for i := range a.appearance.Capes {
				a.appearance.Capes[i].Active = a.appearance.Capes[i].ID == id
			}
			a.notice(a.tr().WardrobeTextureSaved)
		})
	}()
}
