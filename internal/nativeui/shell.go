package nativeui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

// Native coordinates scale uniformly from the 1536×1024 reference canvas so
// font, icon, and control baselines stay aligned across DPIs and small windows.
func uiScale(c *ui.Context) float32 {
	w, h := c.Size()
	return max(.65, min(1, w/1536, (h-42)/982))
}

func (a *App) View(c *ui.Context) {
	for {
		select {
		case fn := <-a.events:
			fn()
		default:
			goto ready
		}
	}
ready:
	s := uiScale(c)
	localTheme := *theme
	localTheme.FontSize = 16 * s
	localTheme.Radius = 12 * s
	localTheme.Spacing = 5 * s
	// The toolkit's bars only hit-test; scrollArea paints the visible thumb.
	// Keyboard focus: an opaque accent ring, 2 DIPs, 1 DIP off the control,
	// drawn by the toolkit only when focus came from the keyboard.
	localTheme.Focus = accent
	localTheme.Scrollbar = ui.Transparent
	localTheme.ScrollbarWidth = scrollHitWidth
	c.SetTheme(&localTheme)
	c.After(250 * time.Millisecond)
	if a.dialog != "" && !a.dialogOpen {
		a.closeDialog()
	}
	rootCol := ui.Column(c).Fill().LinearGradient(ui.LinearGradient{
		Angle: 150,
		Oklab: true,
		Stops: []ui.GradientStop{
			{Color: ui.Hex("#eef3ff"), At: 0},
			{Color: ui.Hex("#f7f1fb"), At: 0.45},
			{Color: ui.Hex("#eef6fb"), At: 1},
		},
	}).Draw(paintAurora)
	draggingFiles := rootCol.FileDragOver()
	if dropped := rootCol.DroppedFiles(); len(dropped) > 0 {
		a.handleDroppedFiles(dropped)
	}
	rootCol.Children(func() {
		a.windowBar(c)
		ui.Row(c).Grow(1).MinHeight(0).FillWidth().Padding(0, 0, 18*s, 0).Gap(0).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
				// Pages share one keyed slot: the new page crossfades in with a
				// short slide in the direction of travel along the top nav.
				ui.Box(c.Key("page-slot")).Grow(1).MinHeight(0).FillWidth().Clip().Children(func() {
					ui.Column(c.Key("page-" + a.page)).Absolute().Left(0).Top(0).Right(0).Bottom(0).Transition(slideTransition(s, a.pageDir, false)).Children(func() {
						if a.page == "settings" {
							a.preferences(c)
							return
						}
						scrollArea(c, "page-scroll-"+a.page).Grow(1).MinHeight(0).Padding(pageTop*s, pageGutter*s, a.scrollEndPad(s)).Children(func() {
							ui.Column(c).FillWidth().MaxWidth(1220 * s).AlignSelf(ui.Center).Gap(pageHeaderGap * s).Children(func() {
								switch a.page {
								case "home":
									a.home(c)
								case "mods":
									a.modPage(c)
								case "discover":
									a.discover(c)
								case "wardrobe":
									a.wardrobe(c)
								case "tasks":
									a.tasks(c)
								}
							})
						})
					})
					a.toast(c)
				})
				a.launchDock(c)
			})
		})
		if draggingFiles {
			t := a.tr()
			popover(ui.Box(c).Absolute().Left(18*s).Right(18*s).Top(56*s).Bottom(18*s), 28*s, s).BorderLinearGradient(ui.LinearGradient{From: accent, To: accentViolet, Angle: 120, Oklab: true}).Border(2, accent).Center().Children(func() {
				ui.Column(c).AlignItems(ui.Center).Gap(14 * s).Padding(36 * s).Children(func() {
					appleSoftIconBox(c, 68*s, "tasks", accent, accentIndigo, panel)
					ui.Text(c, t.DropOverlayTitle).FontSize(24 * s).Bold().TextColor(ink)
					label(c, t.DropOverlayDesc).FontSize(15 * s)
				})
			})
		}
	})
	a.dialogView(c, rootCol)
}

// paintAurora lays soft blue, violet and teal glows over the canvas, the
// macOS wallpaper-like backdrop that the frosted surfaces tint against.
func paintAurora(p *ui.Painter, r ui.Rect) {
	// A shadow shows only outside the box casting it, so each glow casts
	// from a 2-DIP core grown by spread, leaving no hole in its middle.
	glow := func(cx, cy, size float32, col ui.Color) {
		x, y := r.X+r.W*cx, r.Y+r.H*cy
		p.Shadow(ui.Rect{X: x - 1, Y: y - 1, W: 2, H: 2}, 1, 0, 0, size*.5, size*.25, col)
	}
	unit := max(r.W, r.H)
	glow(.08, .05, unit*.30, auroraBlue)
	glow(.92, .10, unit*.34, auroraViolet)
	glow(.55, .45, unit*.22, auroraPink)
	glow(.70, .95, unit*.36, auroraCyan)
	glow(.20, .92, unit*.26, auroraPink)
}

// windowBar is the title bar of the frameless window and the app's top
// navigation in one row: brand once at the left, tabs, then status, language,
// account and the window buttons. Empty space drags the window and a double
// click maximizes it (DragWindow); the OS title stays set for the taskbar.
func (a *App) windowBar(c *ui.Context) {
	t := a.tr()
	s := uiScale(c)
	tb := a.titlebar(c)
	height := max(barHeight*s, 42)
	ui.Row(c.Key("window-title")).Label("主导航").Height(height).FillWidth().Padding(0, tb.Right, 0, tb.Left+18*s).Gap(10 * s).AlignItems(ui.Center).DragWindow().Children(func() {
		ui.Row(c.Key("sidebar-brand")).Label(t.BrandAria).Tooltip(t.AppTitle).AlignItems(ui.Center).Gap(space2 * s).Shrink(0).Children(func() {
			brandMark(c, 28*s)
			ui.Text(c, "MCL").FontSize(18 * s).FontWeight(800).TextColor(ink).NoWrap()
		})
		ui.Box(c).Width(space2 * s).Shrink(0)
		a.navBar(c)
		ui.Spacer(c).Grow(1)
		if len(a.snapshot.Jobs) > 0 {
			j := a.snapshot.Jobs[0]
			if j.State == "running" || j.State == "queued" {
				jobLabel := j.Title
				if j.Total > 0 {
					jobLabel = fmt.Sprintf("%s (%d/%d)", j.Title, j.Done, j.Total)
				}
				jobBtn := ui.ButtonBase(c.Key("title-job")).Label(t.HomeViewTasksBtn).Height(30 * s).PaddingX(12 * s).Gap(6 * s).Radius(999).Background(accentSoft).AlignItems(ui.Center)
				if jobBtn.Children(func() {
					ui.Icon(c, icons["tasks"]).Size(13*s, 13*s).TextColor(accent)
					ui.Text(c, jobLabel).FontSize(12.5 * s).FontWeight(600).TextColor(accent).NoWrap()
				}).Clicked() {
					a.goPage("tasks")
				}
			}
		}
		langBtn := ui.ButtonBase(c.Key("lang-toggle")).Label(t.LangToggleLabel).Tooltip(t.LangToggleTooltip).Height(30*s).PaddingX(12*s).Gap(5*s).Radius(999).Border(1, glassEdge).Shadow(0, 1, 3, 0, ui.RGBA(40, 52, 110, .08)).AlignItems(ui.Center)
		if langBtn.Hovered() {
			langBtn.Background(panel)
		} else {
			langBtn.Background(ui.RGBA(255, 255, 255, .6))
		}
		if langBtn.Children(func() {
			ui.Icon(c, icons["globe"]).Size(13*s, 13*s).TextColor(accent)
			ui.Text(c, t.LangToggleLabel).FontSize(12.5 * s).FontWeight(600).TextColor(ink).NoWrap()
		}).Clicked() {
			a.toggleLanguage()
		}
		a.accountEntry(c)
		if tb.CustomControls {
			ui.Row(c).Gap(0).AlignSelf(ui.Start).Margin(0, 0, 0, 6*s).Children(func() {
				for i, name := range []string{t.WinMinimize, t.WinMaximize, t.WinClose} {
					index := i
					btn := ui.ButtonBase(c.Key(name)).Label(name).Size(46, 42)
					hover, color := ui.Transparent, inkSecondary
					if btn.Hovered() {
						hover = fieldHover
						if index == 2 {
							hover, color = danger, panel
						}
					}
					if btn.Background(btn.AnimateColor("bg", hover, fastAnim)).Draw(func(p *ui.Painter, r ui.Rect) {
						x, y := r.X+r.W/2, r.Y+r.H/2
						switch index {
						case 0:
							p.Fill(ui.Rect{X: x - 5.5, Y: y, W: 11, H: 1.2}, color, 0)
						case 1:
							p.Stroke(ui.Rect{X: x - 5.5, Y: y - 5.5, W: 11, H: 11}, color, 2.5, 1.2)
						}
					}).Children(func() {
						if index == 2 {
							ui.Icon(c, icons["cross"]).Size(15, 15).TextColor(color)
						}
					}).Clicked() && a.win != nil {
						switch index {
						case 0:
							a.win.Minimize()
						case 1:
							a.win.ToggleMaximize()
						case 2:
							a.win.Close()
						}
					}
				}
			})
		}
	})
}

func brandMark(c *ui.Context, size float32) {
	appleSoftIconBox(c, size, "cube", accent, accentIndigo, panel)
}

// navTones gives each destination its own System Settings-style squircle color.
var navTones = map[string][2]ui.Color{
	"home":     {ui.Hex("#0a84ff"), ui.Hex("#5e5ce6")},
	"mods":     {ui.Hex("#ff9f0a"), ui.Hex("#ff6b3d")},
	"discover": {ui.Hex("#30b0c7"), ui.Hex("#0a84ff")},
	"wardrobe": {ui.Hex("#ff375f"), ui.Hex("#bf5af2")},
	"tasks":    {ui.Hex("#34c759"), ui.Hex("#00a37a")},
	"settings": {ui.Hex("#8e8e93"), ui.Hex("#5b5b66")},
}

func (a *App) accountEntry(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	name := t.SideAccountAdd
	if ac := a.account(); ac != nil {
		name = ac.Name
	}
	tip := t.SideAccountManage
	if ac := a.account(); ac != nil {
		tip = ac.Name + " · " + a.accountKindTag(*ac)
	}
	b := ui.ButtonBase(c.Key("account-menu")).Label(t.SideAccountManage).Tooltip(tip).Height(40*s).PaddingX(10*s).Gap(space2*s).AlignItems(ui.Center).Radius(999).Border(1, glassEdge).Shrink(0)
	bg := ui.RGBA(255, 255, 255, .6)
	if a.page == "settings" {
		bg = accentSoft
	} else if b.Hovered() {
		bg = panel
	}
	b.Background(b.AnimateColor("bg", bg, fastAnim))
	if b.Children(func() {
		a.headAvatar(c, a.account(), 28*s, func() {
			ui.Box(c).Size(28*s, 28*s).Radius(14 * s).LinearGradient(tintGradient(ui.Hex("#64d2ff"), accentIndigo)).Center().Children(func() {
				ui.Icon(c, icons["account"]).Size(14*s, 14*s).TextColor(panel)
			})
		})
		ui.Text(c, name).FontSize(typeCallout * s).FontWeight(600).TextColor(ink).SingleLine().MaxWidth(140 * s)
		if ac := a.account(); ac != nil {
			kind := ac.Kind
			if kind != "microsoft" && kind != "yggdrasil" {
				kind = "offline"
			}
			col := accountTypeColor(kind)
			ui.Box(c).Size(20*s, 20*s).Radius(999).Background(col.Alpha(.12)).Center().Shrink(0).Children(func() {
				accountTypeIconL(c, a, "当前账号类型 ", kind, ac.Server, 12*s, col)
			})
		}
		ui.Icon(c, icons["chevron-down"]).Size(10*s, 10*s).TextColor(muted)
	}).Clicked() {
		a.openDialog("account")
	}
}

func (a *App) nav(c *ui.Context, page, text, icon string) {
	s := uiScale(c)
	selected := a.page == page
	b := ui.ButtonBase(c.Key("nav-" + page)).Label(text).Height(36 * s).PaddingX(14 * s).Gap(space2 * s).AlignItems(ui.Center).Radius(10 * s).Shrink(1).MinWidth(0)
	fg := inkSecondary
	switch {
	case selected:
		b.LinearGradient(appleAccentGradient(false)).Shadow(0, 2*s, 6*s, -2*s, ui.RGBA(10, 100, 255, .3))
		fg = panel
	case b.Hovered():
		b.Background(ui.RGBA(255, 255, 255, .8))
	}
	if b.Children(func() {
		ui.Icon(c, icons[icon]).Size(16*s, 16*s).TextColor(fg).Shrink(0)
		ui.Text(c, text).FontSize(typeCallout * s).FontWeight(600).TextColor(fg).SingleLine().MinWidth(0)
		if page == "tasks" && len(a.snapshot.Jobs) > 0 {
			badge(c, fmt.Sprint(len(a.snapshot.Jobs)), accent)
		}
	}).Clicked() {
		a.goPage(page)
	}
}

// Floating Apple Glass-Style Launch Dock at the bottom of the workspace
func (a *App) launchDock(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	r := a.root()
	shown := a.page != "tasks" && a.page != "settings"
	dock := ui.Box(c.Key("launch-dock-slot")).FillWidth().Shrink(0).Transition(ui.ElementTransition{
		Animation: &motionEnter,
		Enter:     &ui.Motion{Collapse: true, Y: 20 * s, Opacity: 0},
		Exit:      &ui.Motion{Collapse: true, Y: 20 * s, Opacity: 0},
	})
	if shown {
		dock.Children(func() {
			ui.Box(c).FillWidth().Padding(0, pageGutter*s, 8*s, pageGutter*s).Children(func() {
				glass(ui.Row(c.Key("launch-dock")).Label("启动栏").Height(84*s).FillWidth().MaxWidth(1220*s).AlignSelf(ui.Center).Padding(14*s, 18*s, 14*s, 20*s).Gap(16*s), 26*s, s).AlignItems(ui.Center).Children(func() {
					iconName := "gamepad"
					if v := a.gameVersion(); v != nil {
						iconName = loaderIconName(v.Loader)
					} else if r != nil && r.Pack != nil {
						iconName = "layers"
					}
					tone := navTones["home"]
					if v := a.gameVersion(); v != nil {
						tone = [2]ui.Color{loaderTone(v.Loader), accentIndigo}
					}
					if v := a.gameVersion(); v == nil || !loaderLogo(c, v.Loader, 52*s) {
						appleSoftIconBox(c, 52*s, iconName, tone[0], tone[1], panel)
					}

					ui.Column(c).Grow(1).Basis(0).MinWidth(0).Clip().Gap(3 * s).Children(func() {
						v := a.gameVersion()
						switch {
						case r == nil:
							ui.Text(c, t.TopNoInstance).FillWidth().MinWidth(0).FontSize(16 * s).Bold().TextColor(ink).SingleLine()
						case v == nil:
							ui.Text(c, t.HomeHeroNoVersion).FillWidth().MinWidth(0).FontSize(16 * s).Bold().TextColor(ink).SingleLine()
							label(c, midEllipsis(c, r.Name, dockFolderMax*s, 12.5*s, 400)).FillWidth().MinWidth(0).FontSize(12.5 * s).SingleLine().Tooltip(r.Path)
						default:
							title, sub := a.cardText(*r, *v)
							titleWithTags(c, title, v.ID, 16*s, 700, a.versionTags(*r, *v, true), 3)
							label(c, joinDistinct(midEllipsis(c, r.Name, dockFolderMax*s, 12.5*s, 400), sub)).FillWidth().MinWidth(0).FontSize(12.5 * s).SingleLine().Tooltip(r.Path)
						}
					})
					if r != nil {
						if r.Running && button(c, "dock-stop", t.HomeStopBtn, "cross", false, a.busy) {
							a.execute("stop", map[string]string{"rootId": r.ID}, nil)
						}
					}
					// Launcher settings sit right before the primary launch button.
					if fixedButton(c, "dock-settings", t.NavSettings, "settings", 0, 52*s, false, false) {
						a.goPage("settings")
					}
					launchText := t.HomeLaunchBtn
					if r != nil && r.Running {
						launchText = t.HomeRunningBtn
					}
					why := whyOf(a.busy, t.WhyBusy, r != nil && r.Running, t.WhyRunning, r != nil && a.gameVersion() == nil, t.WhyNoVersion)
					pendingWhy = why
					if fixedButton(c, "launch", launchText, "play", 160*s, 52*s, true, why != "") {
						a.launch()
					}
				})
			})
		})
	}
}

// backCrumb leads the 设置 page header: settings opens from the bottom bar,
// so instead of a nav highlight the page offers a way back ("‹ 游戏库").
func (a *App) backCrumb(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	prev := a.prevPage
	if prev == "" || prev == "settings" {
		prev = "home"
	}
	name := map[string]string{"home": t.NavHome, "mods": t.NavMods, "discover": t.NavDiscover, "wardrobe": t.NavWardrobe, "tasks": t.NavTasks}[prev]
	b := ui.ButtonBase(c.Key("settings-back")).Label(t.BackLabel).Height(24 * s).PaddingX(0).Gap(space1 * s).AlignItems(ui.Center).AlignSelf(ui.Start)
	fg := accent
	if b.Hovered() {
		fg = accentIndigo
	}
	if b.Children(func() {
		ui.Text(c, "‹").FontSize(typeHeadline * s).FontWeight(600).TextColor(b.AnimateColor("fg", fg, motionFast)).NoWrap()
		ui.Text(c, name).FontSize(typeCallout * s).FontWeight(600).TextColor(b.AnimateColor("fg2", fg, motionFast)).NoWrap()
	}).Clicked() {
		a.goPage(prev)
	}
}

// dockFolderMax caps the folder name in the launch bar subtitle; longer
// names lose their middle, and the tooltip shows the full path.
const dockFolderMax float32 = 240

// toastReserve is the room a page notice takes above the launch bar
// (toast height plus its gap).
const toastReserve float32 = 64

// scrollEndPad is the bottom padding of every page scroll: the normal end
// gap plus, while a notice floats over the page bottom, the notice's room,
// so the last row is fully visible at scroll end. The launch bar itself is
// in the layout flow below the scroll area, so it never covers content.
func (a *App) scrollEndPad(s float32) float32 {
	pad := space6 * s
	if a.message != "" || a.noticeShown != "" {
		pad += toastReserve * s
	}
	return pad
}
