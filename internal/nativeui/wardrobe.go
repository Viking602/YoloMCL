package nativeui

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/draw"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// wardrobeState is the wardrobe's view state beyond the account appearance.
type wardrobeState struct {
	capeImgs    map[string]image.Image // cape URL -> texture
	capeThumbs  map[string]*ui.Bitmap
	capeAsked   map[string]bool
	pendingSkin []byte // a local file chosen but not uploaded yet
	pendingName string
	recent      []recentSkin
	applying    bool
	capeRaw     map[string][]byte // cape URL -> original PNG bytes
	saveMsg     string            // last save result, shown inline
	savePath    string
	saveErr     bool
}

type recentSkin struct {
	name string
	data []byte
	head *ui.Bitmap
}

const noCapeKey = "\x00none"

func (a *App) wd() *wardrobeState {
	if a.wardrobeSt == nil {
		a.wardrobeSt = &wardrobeState{capeImgs: map[string]image.Image{}, capeThumbs: map[string]*ui.Bitmap{}, capeAsked: map[string]bool{}}
	}
	return a.wardrobeSt
}

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
	if cape.URL != "" && !w.capeAsked[cape.URL] {
		w.capeAsked[cape.URL] = true
		u := cape.URL
		a.bgWG.Add(1)
		go func() {
			defer a.bgWG.Done()
			req, _ := http.NewRequestWithContext(a.ctx, "GET", u, nil)
			res, e := http.DefaultClient.Do(req)
			if e == nil && res.StatusCode != 200 {
				res.Body.Close()
				e = errStatus
			}
			if e != nil {
				logUIError("cape image: " + e.Error())
				// Placeholder stays; try once more a little later.
				time.AfterFunc(20*time.Second, func() { a.post(func() { delete(w.capeAsked, u) }) })
				return
			}
			defer res.Body.Close()
			b, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
			img, e := png.Decode(bytes.NewReader(b))
			if e != nil {
				return
			}
			a.post(func() {
				if w.capeRaw == nil {
					w.capeRaw = map[string][]byte{}
				}
				w.capeRaw[u] = b
				w.capeImgs[u] = img
				if a.selectedCape == cape.Name {
					a.preview.cape = img
					a.preview.dirty = true
				}
			})
		}()
	}
	return nil
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

func (p *skinPreview) turnTo(yaw float64) {
	// Nearest equivalent angle, so it never spins the long way round.
	for yaw-p.yaw > math.Pi {
		yaw -= 2 * math.Pi
	}
	for p.yaw-yaw > math.Pi {
		yaw += 2 * math.Pi
	}
	p.target = &yaw
	p.spin = false
	p.dirty = true
}

func (a *App) wardrobe(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	a.normalizeStateValues()
	ac := a.account()
	ui.Row(c).Label("页面标题栏").FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
		ui.Text(c, t.WardrobeTitle).Label("页面标题 " + t.WardrobeTitle).FontSize(typeDisplay * s).FontWeight(750).TextColor(ink).SingleLine()
		ui.Spacer(c).Grow(1)
		if ac != nil {
			if iconOnlyButton(c, "refresh-skin", "refresh", t.WardrobeRefreshBtn, btnMd*s, a.appearanceLoading) {
				a.loadAppearance()
			}
		}
	})
	if ac == nil {
		emptyState(c, "empty-wardrobe", "account", t.WardrobeNoAccountTitle, t.WardrobeNoAccountDesc, func() {
			if button(c, "wardrobe-add-account", t.AccAddAccountBtn, "plus", true, false) {
				a.openDialog("account")
			}
		})
		return
	}
	if a.appearanceErr != "" {
		if a.loadFailed(c, "appearance-failed", failCard, a.appearanceLoading) {
			a.loadAppearance()
		}
	}
	_, winH := c.Size()
	// The stage takes about a third of the row (300-440 pt) and stops at
	// 540 pt tall; the skin and cape panels get the rest.
	stageH := min(540*s, max(380*s, float32(winH)-200*s))
	ui.Row(c).FillWidth().Gap(20 * s).AlignItems(ui.Start).Children(func() {
		a.wardrobeStage(c, stageH)
		ui.Column(c).Grow(2).Basis(0).MinWidth(400 * s).Gap(16 * s).Children(func() {
			a.wardrobeSkinCard(c)
			a.wardrobeCapeCard(c)
		})
	})
}

func (a *App) wardrobeStage(c *ui.Context, h float32) {
	s := uiScale(c)
	t := a.tr()
	p := a.preview
	now := c.Now()
	dt := 0.0
	if !p.last.IsZero() {
		dt = math.Min(now.Sub(p.last).Seconds(), .1)
	}
	p.last = now
	animating := false
	if p.target != nil {
		d := *p.target - p.yaw
		if math.Abs(d) < .01 {
			p.yaw, p.target = *p.target, nil
		} else {
			p.yaw += d * math.Min(1, dt*7)
			animating = true
		}
		p.dirty = true
	} else if p.spin {
		p.yaw += dt * .6
		p.dirty = true
		animating = true
	}
	if p.dirty || p.bitmap == nil {
		img := p.render(440, 520)
		if p.bitmap == nil {
			p.bitmap = ui.NewBitmap(img).Pixelated()
		} else {
			p.bitmap.Update(img)
		}
		p.dirty = false
	}
	stage := ui.ButtonBase(c.Key("skin-orbit")).Label(t.WardrobeOrbitAria).Grow(1).Basis(0).MinWidth(300*s).MaxWidth(440*s).Height(h).Radius(radiusCard*s).Clip().
		LinearGradient(ui.LinearGradient{From: ui.RGBA(236, 241, 252, 1), To: ui.RGBA(214, 224, 244, 1), Angle: 180}).Border(1, glassEdge).Column().AlignItems(ui.Center).Justify(ui.Center)
	if dx, dy, ok := stage.Dragged(); ok {
		p.yaw += float64(dx) * .014
		p.pitch = math.Max(-.65, math.Min(.55, p.pitch+float64(dy)*.009))
		p.target, p.spin = nil, false
		p.dirty = true
	}
	imgH := h - 96*s // room for the control pill plus padding
	stage.Children(func() {
		ui.Image(c, p.bitmap).Size(imgH*440/520, imgH).Fit(ui.Contain)
		ui.Row(c).Absolute().Left(0).Right(0).Bottom(16 * s).Justify(ui.Center).Children(func() {
			ui.Row(c).Height(40*s).PaddingX(6*s).Gap(2*s).Radius(999).Background(ui.RGBA(255, 255, 255, .86)).Border(1, glassEdge).Shadow(0, 4*s, 16*s, -4*s, ui.RGBA(30, 40, 90, .18)).AlignItems(ui.Center).Children(func() {
				if stageIcon(c, "stage-front", "account", t.WardrobeFrontBtn, false) {
					p.turnTo(-.35)
				}
				if stageIcon(c, "stage-back", "skin", t.WardrobeBackBtn, false) {
					p.turnTo(math.Pi - .35)
				}
				ui.Box(c).Size(1, 18*s).Background(line).Margin(0, 4*s)
				spinIcon := "play"
				if p.spin {
					spinIcon = "pause"
				}
				if stageIcon(c, "stage-spin", spinIcon, t.WardrobeAutoSpin, p.spin) {
					p.spin, p.target = !p.spin, nil
				}
				ui.Box(c).Size(1, 18*s).Background(line).Margin(0, 4*s)
				if stageIcon(c, "stage-zoom-out", "minus", t.WardrobeZoomAria+" −", false) {
					p.zoom = math.Max(.65, p.zoom-.1)
					p.dirty = true
				}
				if stageIcon(c, "stage-zoom-in", "plus", t.WardrobeZoomAria+" +", false) {
					p.zoom = math.Min(1.4, p.zoom+.1)
					p.dirty = true
				}
				ui.Box(c).Size(1, 18*s).Background(line).Margin(0, 4*s)
				if stageIcon(c, "stage-save", "download", a.L("保存皮肤", "Save skin"), false) {
					name := "skin"
					if ac := a.account(); ac != nil {
						name = ac.Name
					}
					if w := a.wd(); len(w.pendingSkin) > 0 && w.pendingName != "" {
						a.saveTexture(safePNGName(w.pendingName, ""), w.pendingSkin, false)
					} else {
						a.saveTexture(safePNGName(name, "_skin"), a.currentSkinBytes(), false)
					}
				}
			})
		})
		if a.appearanceLoading {
			ui.Row(c).Absolute().Top(14 * s).Left(14 * s).Gap(6 * s).AlignItems(ui.Center).Children(func() {
				statusSpinner(c, 12*s)
				ui.Text(c, a.L("正在读取外观", "Loading appearance")).FontSize(typeCaption * s).TextColor(muted)
			})
		}
		ui.Text(c, a.L("拖动旋转", "Drag to rotate")).Absolute().Top(14 * s).Right(16 * s).FontSize(typeCaption * s).TextColor(muted)
	})
	if animating {
		c.After(16 * time.Millisecond)
	}
}

func stageIcon(c *ui.Context, key, icon, label string, on bool) bool {
	s := uiScale(c)
	b := ui.ButtonBase(c.Key(key)).Label(label).Tooltip(label).Size(32*s, 32*s).Radius(999).AlignItems(ui.Center).Justify(ui.Center)
	bg, fg := ui.Transparent, inkSecondary
	if on {
		bg, fg = accentSoft, accent
	} else if b.Hovered() {
		bg = ui.RGBA(120, 130, 160, .12)
	}
	b.Background(b.AnimateColor("bg", bg, fastAnim))
	return b.Children(func() { ui.Icon(c, icons[icon]).Size(14*s, 14*s).TextColor(fg) }).Clicked()
}

func wardrobeCard(c *ui.Context, key string, body func()) {
	s := uiScale(c)
	glass(ui.Column(c.Key(key)).FillWidth().Gap(12*s).Padding(18*s, 20*s), radiusCard*s, s).Font(uiSansFamily()).Children(body)
}

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
				ui.Text(c, a.L("预览中：", "Previewing: ")+w.pendingName).FontSize(typeFootnote * s).TextColor(inkSecondary).SingleLine().Grow(1).MinWidth(0)
			})
		}
		ui.Row(c).FillWidth().Gap(8 * s).Children(func() {
			why := whyOf(a.texturePending("skin"), t.WhyBusy, !a.appearance.CanSkin, a.skinWhy())
			if w.pendingSkin != nil {
				if buttonWhy(c, "upload-skin", a.L("上传此皮肤", "Upload this skin"), "check", true, why, false) {
					a.uploadPendingSkin()
				}
				if button(c, "cancel-skin", a.L("取消", "Cancel"), "", false, a.texturePending("skin")) {
					w.pendingSkin, w.pendingName = nil, ""
					a.preview.skin = imageFromData(a.appearance.Skin)
					if a.preview.skin == nil {
						a.preview.skin = builtinSkin(a.preview.slim)
					}
					a.preview.dirty = true
				}
				return
			}
			if buttonWhy(c, "choose-skin", a.L("选择皮肤文件", "Choose skin file"), "folder", true, why, false) {
				a.chooseFiles("png", func(paths []string) { a.previewSkinFile(paths[0]) })
			}
			if button(c, "reset-skin", t.WardrobeResetSkinBtn, "", false, a.texturePending("skin")) {
				a.execute("resetTexture", map[string]string{"accountId": ac.ID, "kind": "skin"}, func(json.RawMessage) { a.loadAppearance() })
			}
		})
		if len(w.recent) > 0 {
			ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
				ui.Text(c, a.L("最近使用", "Recent")).FontSize(typeFootnote * s).TextColor(muted)
				ui.Row(c).FillWidth().Gap(8 * s).Children(func() {
					for i, r := range w.recent {
						rs := r
						// Tile plus a hover-only save badge in its corner; both
						// stay inside the fixed 38 pt box.
						ui.Box(c.Key("recent-wrap-"+itoa(i))).Size(38*s, 38*s).Shrink(0).Children(func() {
							b := ui.ButtonBase(c.Key("recent-skin-"+itoa(i))).Label(rs.name).Tooltip(rs.name).Size(38*s, 38*s).Radius(9*s).Background(panel).Border(1, line).Clip().Center()
							hot := b.Hovered()
							if b.Children(func() {
								if rs.head != nil {
									ui.Image(c, rs.head).Size(30*s, 30*s)
								}
							}).Clicked() {
								a.setPendingSkin(rs.name, rs.data)
							}
							sb := ui.ButtonBase(c.Key("recent-save-"+itoa(i))).Label(a.L("保存皮肤", "Save skin")).Tooltip(a.L("保存皮肤", "Save skin")).Absolute().Right(-4*s).Bottom(-4*s).Size(18*s, 18*s).Radius(999).Background(panel).Border(1, line).Center()
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
		ui.Text(c, a.L("支持 64×64 与旧版 64×32 PNG。切换模型需重新上传后生效。", "64×64 and legacy 64×32 PNG. A model change takes effect on upload.")).FontSize(typeCaption * s).TextColor(muted).FillWidth()
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
		a.message, a.errorMessage = a.L("不是有效的皮肤 PNG（需要 64×64 或 64×32）", "Not a valid skin PNG (64×64 or 64×32)"), true
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
			ui.Text(c, a.L("这个账号还没有披风", "This account has no capes yet")).FontSize(typeFootnote * s).TextColor(muted)
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
