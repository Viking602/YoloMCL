package nativeui

import (
	"image"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// wardrobeState is the wardrobe's view state beyond the account appearance.
type wardrobeState struct {
	capeImgs    map[string]image.Image // cape URL -> texture
	capeThumbs  map[string]*ui.Bitmap
	capeAsked   map[string]bool
	capeFailed  map[string]bool
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

// Slow, continuous rotation does not need the display's full refresh rate.
// Dragging, zooming and short front/back transitions remain interactive.
const previewSpinInterval = time.Second / 30

func (a *App) wd() *wardrobeState {
	if a.wardrobeSt == nil {
		a.wardrobeSt = &wardrobeState{capeImgs: map[string]image.Image{}, capeThumbs: map[string]*ui.Bitmap{}, capeAsked: map[string]bool{}}
	}
	return a.wardrobeSt
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
				a.openDialog("accountAdd")
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
	stage := ui.ButtonBase(c.Key("skin-orbit")).Label(t.WardrobeOrbitAria).Grow(1).Basis(0).MinWidth(300*s).MaxWidth(440*s).Height(h).Radius(radiusCard*s).Clip().
		LinearGradient(ui.LinearGradient{From: ui.RGBA(236, 241, 252, 1), To: ui.RGBA(214, 224, 244, 1), Angle: 180}).Border(1, glassEdge).Column().AlignItems(ui.Center).Justify(ui.Center).
		HandleInput(func(ev ui.InputEvent) bool {
			if ev.Kind != ui.InputScroll || ev.DY == 0 || ev.Mods&ui.Shift != 0 {
				return false
			}
			// Deltas are in DIPs, including fractional touchpad movement.
			// Consume the wheel at either limit so the page stays still.
			p.zoomBy(-float64(ev.DY) / 400)
			return true
		})
	zoom := stage.AnimateTo("zoom", float32(p.zoomGoal), motionFast)
	if c.Preferences().ReduceMotion {
		zoom = stage.Jump("zoom", float32(p.zoomGoal))
	}
	if p.zoom != float64(zoom) {
		p.zoom, p.dirty = float64(zoom), true
	}
	if dx, dy, ok := stage.Dragged(); ok {
		p.yaw += float64(dx) * .014
		p.pitch = math.Max(-.65, math.Min(.55, p.pitch+float64(dy)*.009))
		p.target, p.spin = nil, false
		p.dirty = true
	}
	now := c.Now()
	dt := 0.0
	if !p.last.IsZero() {
		dt = math.Min(now.Sub(p.last).Seconds(), .1)
	}
	p.last = now
	if !p.spin {
		p.spinFrame = time.Time{}
	}
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
		elapsed := now.Sub(p.spinFrame)
		if p.spinFrame.IsZero() || elapsed >= previewSpinInterval {
			if !p.spinFrame.IsZero() {
				dt = min(elapsed.Seconds(), .1)
			}
			p.yaw += dt * .6
			p.spinFrame = now
			p.dirty = true
		}
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
					p.zoomBy(-.1)
					c.After(0)
				}
				if stageIcon(c, "stage-zoom-in", "plus", t.WardrobeZoomAria+" +", false) {
					p.zoomBy(.1)
					c.After(0)
				}
				ui.Box(c).Size(1, 18*s).Background(line).Margin(0, 4*s)
				if stageIcon(c, "stage-save", "download", a.L("wardrobe.save_skin"), false) {
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
				ui.Text(c, a.L("wardrobe.loading_appearance")).FontSize(typeCaption * s).TextColor(muted)
			})
		} else {
			ui.Text(c, t.WardrobeOrbitHint).Absolute().Top(14 * s).Right(16 * s).FontSize(typeCaption * s).TextColor(muted)
		}
	})
	if animating {
		if p.spin {
			c.After(max(0, previewSpinInterval-now.Sub(p.spinFrame)))
		} else {
			c.After(16 * time.Millisecond)
		}
	}
}

func stageIcon(c *ui.Context, key, icon, label string, on bool) bool {
	s := uiScale(c)
	b := tooltip(c, ui.ButtonBase(c.Key(key)).Label(label), label).Size(32*s, 32*s).Radius(999).AlignItems(ui.Center).Justify(ui.Center)
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
