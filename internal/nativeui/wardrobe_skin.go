package nativeui

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

func (a *App) wardrobeSkinCard(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	w := a.wd()
	ac := a.account()
	wardrobeCard(c, "skin-options", func() {
		ui.Text(c, t.WardrobeSkinTitle).FontSize(typeHeadline * s).FontWeight(700).TextColor(ink)
		ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
			ui.Text(c, t.WardrobeModelLabel).FontSize(typeFootnote * s).TextColor(muted)
			ui.Row(c).Children(func() {
				if v, ok := segmented(c, "skin-model", []string{t.WardrobeModelSteve, t.WardrobeModelAlex}, a.model, 30*s); ok {
					a.model = v
					m := normalizeSkinModel(a.model)
					a.preview.slim = m == "slim"
					a.preview.dirty = true
					if a.offlineDefault() {
						// Official Steve/Alex: swap texture and arms, and remember it.
						a.showVanilla(m)
						if ac := a.account(); ac != nil && a.service != nil {
							id := ac.ID
							go func() {
								if e := a.service.SetOfflineModel(id, m); e != nil {
									a.post(func() { a.message, a.errorMessage = a.friendlyErr(e), true })
								} else {
									a.post(func() { a.invalidateHead(id) })
								}
							}()
						}
					}
				}
			})
		})
		if w.pendingSkin != nil {
			ui.Row(c).FillWidth().Padding(10*s, 12*s).Radius(10 * s).Background(accentSoft).Gap(10 * s).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons["info"]).Size(13*s, 13*s).TextColor(accent).Shrink(0)
				ui.Text(c, a.L("wardrobe_skin.previewing")+w.pendingName).FontSize(typeFootnote * s).TextColor(inkSecondary).SingleLine().Grow(1).MinWidth(0)
			})
		}
		ui.Row(c).FillWidth().Gap(8 * s).Children(func() {
			why := whyOf(a.texturePending("skin"), t.WhyBusy, !a.appearance.CanSkin, a.skinWhy())
			if w.pendingSkin != nil {
				if buttonWhy(c, "upload-skin", a.L("wardrobe_skin.upload_this_skin"), "check", true, why, false) {
					a.uploadPendingSkin()
				}
				if button(c, "cancel-skin", a.L("install_dialog.cancel"), "", false, a.texturePending("skin")) {
					w.pendingSkin, w.pendingName = nil, ""
					a.preview.skin = imageFromData(a.appearance.Skin)
					if a.preview.skin == nil {
						a.preview.skin = builtinSkin(a.preview.slim)
					}
					a.preview.dirty = true
				}
				return
			}
			if buttonWhy(c, "choose-skin", a.L("wardrobe_skin.choose_skin_file"), "folder", true, why, false) {
				a.chooseFiles("png", func(paths []string) { a.previewSkinFile(paths[0]) })
			}
			if button(c, "reset-skin", t.WardrobeResetSkinBtn, "", false, a.texturePending("skin")) {
				a.execute("resetTexture", map[string]string{"accountId": ac.ID, "kind": "skin"}, func(json.RawMessage) { a.loadAppearance() })
			}
		})
		if len(w.recent) > 0 {
			ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
				ui.Text(c, a.L("wardrobe_skin.recent")).FontSize(typeFootnote * s).TextColor(muted)
				ui.Row(c).FillWidth().Gap(8 * s).Children(func() {
					for i, r := range w.recent {
						rs := r
						// Tile plus a hover-only save badge in its corner; both
						// stay inside the fixed 38 pt box.
						ui.Box(c.Key("recent-wrap-"+itoa(i))).Size(38*s, 38*s).Shrink(0).Children(func() {
							b := tooltip(c, ui.ButtonBase(c.Key("recent-skin-"+itoa(i))).Label(rs.name), rs.name).Size(38*s, 38*s).Radius(9*s).Background(panel).Border(1, line).Clip().Center()
							hot := b.Hovered()
							if b.Children(func() {
								if rs.head != nil {
									ui.Image(c, rs.head).Size(30*s, 30*s)
								}
							}).Clicked() {
								a.setPendingSkin(rs.name, rs.data)
							}
							sb := tooltip(c, ui.ButtonBase(c.Key("recent-save-"+itoa(i))).Label(a.L("wardrobe.save_skin")), a.L("wardrobe.save_skin")).Absolute().Right(-4*s).Bottom(-4*s).Size(18*s, 18*s).Radius(999).Background(panel).Border(1, line).Center()
							op := float32(0)
							if hot || sb.Hovered() {
								op = 1
							}
							sb.Opacity(sb.AnimateTo("op", op, fastAnim))
							if sb.Children(func() {
								ui.Icon(c, icons["download"]).Size(9*s, 9*s).TextColor(accent)
							}).Clicked() {
								a.saveTexture(safePNGName(rs.name, ""), rs.data, false)
							}
						})
					}
				})
			})
		}
		a.saveNotice(c)
		ui.Text(c, a.L("wardrobe_skin.64_64_and_legacy_64_32_png_a")).FontSize(typeCaption * s).TextColor(muted).FillWidth()
		if ac.Kind == "yggdrasil" && !a.appearance.CanSkin {
			ui.Text(c, t.WardrobeYggNoUpload).FontSize(typeFootnote * s).TextColor(inkSecondary).FillWidth()
		}
	})
}

func (a *App) previewSkinFile(path string) {
	b, e := os.ReadFile(path)
	if e != nil {
		a.message, a.errorMessage = a.friendlyErr(e), true
		return
	}
	a.setPendingSkin(filepath.Base(path), b)
}

func (a *App) setPendingSkin(name string, b []byte) {
	img, e := png.Decode(bytes.NewReader(b))
	if e != nil || img.Bounds().Dx() != 64 || (img.Bounds().Dy() != 64 && img.Bounds().Dy() != 32) {
		a.message, a.errorMessage = a.L("wardrobe_skin.not_a_valid_skin_png_64_64_or"), true
		return
	}
	w := a.wd()
	w.pendingSkin, w.pendingName = b, name
	a.preview.skin = img
	a.preview.turnTo(-.35)
	// recent list, newest first, unique by name, at most 6
	var head *ui.Bitmap
	if h := skinHead(img); h != nil {
		head = ui.NewBitmap(h).Pixelated()
	}
	out := []recentSkin{{name, b, head}}
	for _, r := range w.recent {
		if r.name != name && len(out) < 6 {
			out = append(out, r)
		}
	}
	w.recent = out
}

func (a *App) uploadPendingSkin() {
	ac := a.account()
	w := a.wd()
	if ac == nil || w.pendingSkin == nil {
		return
	}
	data := w.pendingSkin
	a.execute("uploadTexture", map[string]string{"accountId": ac.ID, "kind": "skin", "model": normalizeSkinModel(a.model), "data": base64.StdEncoding.EncodeToString(data)}, func(json.RawMessage) {
		w.pendingSkin, w.pendingName = nil, ""
		a.loadAppearance()
		a.notice(a.tr().WardrobeTextureSaved)
	})
}
