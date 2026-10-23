package nativeui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// Native coordinates scale uniformly from the 1536×1024 reference canvas so
// font, icon, and control baselines stay aligned across DPIs and small windows.
func uiScale(c *ui.Context) float32 {
	w, h := c.Size()
	return max(.65, min(1, w/1536, (h-42)/982)) * uiDensity
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
	a.refreshSystemLanguage()
	s := uiScale(c)
	localTheme := *theme
	localTheme.FontSize = 14.5 * s
	localTheme.Radius = 9 * s
	localTheme.Spacing = 16.0 / 3 * s // TextInput pads 1.5×/2.5× this: 8 / 13 DIPs, so 35-DIP fields center their text
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
	// One continuous chrome material for the title bar and the sidebar; the
	// content sits on a lighter inset panel (contentPanel).
	rootCol := ui.Column(c).Fill().Background(chromeBg)
	draggingFiles := rootCol.FileDragOver()
	if dropped := rootCol.DroppedFiles(); len(dropped) > 0 {
		a.handleDroppedFiles(dropped)
	}
	rootCol.Children(func() {
		a.windowBar(c)
		ui.Row(c).Grow(1).MinHeight(0).FillWidth().Gap(0).Children(func() {
			ui.Box(c).Grow(1).MinWidth(0).MinHeight(0).FillHeight().Children(func() {
				// Pages share one keyed slot: the new page crossfades in with a
				// short slide in the direction of travel along the top nav.
				ui.Box(c.Key("page-slot")).Label("页面视口").Fill().Clip().Children(func() {
					ui.Column(c.Key("page-" + a.page)).Absolute().Left(0).Top(0).Right(0).Bottom(0).Transition(slideTransition(s, a.pageDir, false)).Children(func() {
						if a.page == "settings" {
							contentPanel(ui.Column(c.Key("content-panel")), s).Grow(1).MinHeight(0).Margin(0, 10*s, 10*s, 10*s).Children(func() { a.preferences(c) })
							return
						}
						if a.page == "home" {
							a.instanceView(c)
							return
						}
						if a.page == "discover" {
							a.discoverView(c)
							return
						}
						contentPanel(ui.Column(c.Key("content-panel")), s).Grow(1).MinHeight(0).Children(func() {
							scrollArea(c, "page-scroll-"+a.page).Grow(1).MinHeight(0).Padding(pageTop*s, pageGutter*s, a.scrollEndPad(s)).Children(func() {
								ui.Column(c).FillWidth().MaxWidth(1160 * s).AlignSelf(ui.Center).Gap(pageHeaderGap * s).Children(func() {
									switch a.page {
									case "mods":
										a.modPage(c)
									case "wardrobe":
										a.wardrobe(c)
									case "tasks":
										a.tasks(c)
									}
								})
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
			popover(ui.Box(c).Absolute().Left(18*s).Right(18*s).Top(56*s).Bottom(18*s), 28*s, s).BorderLinearGradient(ui.LinearGradient{From: accent, To: accentViolet, Angle: 120}).Border(2, accent).Center().Children(func() {
				ui.Column(c).AlignItems(ui.Center).Gap(14 * s).Padding(36 * s).Children(func() {
					appleSoftIconBox(c, 68*s, "tasks", accent, accentIndigo, panel)
					ui.Text(c, t.DropOverlayTitle).FontSize(typeTitle1 * s).Bold().TextColor(ink)
					label(c, t.DropOverlayDesc).FontSize(typeBody * s)
				})
			})
		}
	})
	a.dialogView(c, rootCol)
}

// windowBar is the title bar of the frameless window and the app's top
// navigation in one row: brand once at the left, tabs, then status, language,
// account and the window buttons. Empty space drags the window and a double
// click maximizes it (DragWindow); the OS title stays set for the taskbar.
func (a *App) windowBar(c *ui.Context) {
	t := a.tr()
	s := uiScale(c)
	tb := a.titlebar(c)
	height := max(barHeight*s, 36)
	ui.Row(c.Key("window-title")).Label("主导航").Height(height).FillWidth().Padding(0, tb.Right, 0, tb.Left+18*s).Gap(10 * s).AlignItems(ui.Center).DragWindow().Children(func() {
		ui.Row(c.Key("sidebar-brand")).Label(t.BrandAria).Tooltip(t.AppTitle).AlignItems(ui.Center).Gap(space2 * s).Shrink(0).Children(func() {
			brandMark(c, 23.5*s)
			ui.Text(c, "YoloMCL").FontSize(typeHeadline * s).FontWeight(800).TextColor(ink).NoWrap()
		})
		ui.Box(c).Width(space2 * s).Shrink(0)
		a.navBar(c)
		ui.Spacer(c).Grow(1)
		a.jobPill(c)
		langBtn := ui.ButtonBase(c.Key("lang-toggle")).Label(t.LangToggleLabel).Tooltip(t.LangToggleTooltip).Height(btnSm*s).PaddingX(12*s).Gap(5*s).Radius(999).Border(1, glassEdge).Shadow(0, 1, 3, 0, ui.RGBA(40, 52, 110, .08)).AlignItems(ui.Center)
		if langBtn.Hovered() {
			langBtn.Background(panel)
		} else {
			langBtn.Background(ui.RGBA(255, 255, 255, .6))
		}
		if langBtn.Children(func() {
			ui.Icon(c, icons["globe"]).Size(12*s, 12*s).TextColor(accent)
			ui.Text(c, t.LangToggleLabel).FontSize(typeFootnote * s).FontWeight(600).TextColor(ink).NoWrap()
		}).Clicked() {
			a.toggleLanguage()
		}
		if iconOnlyButton(c, "title-settings", "settings", t.NavSettings, btnSm*s, false) {
			a.goPage("settings")
		}
		a.accountEntry(c)
		if tb.CustomControls {
			ui.Row(c).Gap(0).AlignSelf(ui.Start).Margin(0, 0, 0, 6*s).Children(func() {
				for i, name := range []string{t.WinMinimize, t.WinMaximize, t.WinClose} {
					index := i
					btn := ui.ButtonBase(c.Key(name)).Label(name).Size(44, 40)
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

// brandMark is the YoloMCL grass block app icon; the hand-tuned 32 px
// bitmap is used when it is drawn small so it stays crisp.
func brandMark(c *ui.Context, size float32) {
	bmp := brandLogos[64]
	if size <= 32 && brandLogos[32] != nil {
		bmp = brandLogos[32]
	} else if size > 64 && brandLogos[128] != nil {
		bmp = brandLogos[128]
	}
	if bmp == nil {
		appleSoftIconBox(c, size, "cube", accent, accentIndigo, panel)
		return
	}
	ui.Image(c, bmp).Size(size, size).Fit(ui.Contain).Label("YoloMCL").Shrink(0)
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
		tip = ac.Name + "   " + a.accountKindTag(*ac)
	}
	b := ui.ButtonBase(c.Key("account-menu")).Label(t.SideAccountManage).Tooltip(tip).Height(btnSm*s).PaddingX(8*s).Gap(6*s).AlignItems(ui.Center).Radius(999).Border(1, glassEdge).Shrink(0)
	bg := ui.RGBA(255, 255, 255, .6)
	if a.page == "settings" {
		bg = accentSoft
	} else if b.Hovered() {
		bg = panel
	}
	b.Background(b.AnimateColor("bg", bg, fastAnim))
	if b.Children(func() {
		a.headAvatar(c, a.account(), 22*s, func() {
			ui.Box(c).Size(22*s, 22*s).Radius(11 * s).LinearGradient(tintGradient(ui.Hex("#64d2ff"), accentIndigo)).Center().Children(func() {
				ui.Icon(c, icons["account"]).Size(12*s, 12*s).TextColor(panel)
			})
		})
		ui.Text(c, midEllipsis(c, name, 130*s, typeCallout*s, 600)).FontSize(typeCallout * s).FontWeight(600).TextColor(ink).NoWrap()
		if ac := a.account(); ac != nil {
			kind := ac.Kind
			if kind != "microsoft" && kind != "yggdrasil" {
				kind = "offline"
			}
			col := accountTypeColor(kind)
			ui.Box(c).Size(18*s, 18*s).Radius(999).Background(col.Alpha(.12)).Center().Shrink(0).Children(func() {
				accountTypeIconL(c, a, "当前账号类型 ", kind, ac.Server, 11*s, col)
			})
		}
		ui.Icon(c, icons["chevron-down"]).Size(9.5*s, 9.5*s).TextColor(muted)
	}).Clicked() {
		if len(a.snapshot.Accounts) == 0 {
			a.accountAdding = false
			a.openDialog("account")
		} else {
			a.accountMenuOpen = !a.accountMenuOpen
		}
	}
	a.accountMenu(c, b)
}

// accountMenu is the quick switcher under the top-bar account chip: every
// account, one click to switch, then the manager and add entries. A click
// outside or Escape closes it.
func (a *App) accountMenu(c *ui.Context, anchor ui.Element) {
	s := uiScale(c)
	t := a.tr()
	ui.PopoverBase(c.Key("account-menu-pop"), anchor, &a.accountMenuOpen, func(pop ui.Element) {
		popover(pop.Label("账号快速切换").AttachTo(anchor, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(6*s, 0, 0, 0).Width(264*s).MaxWidthPercent(40).Padding(6*s), 14*s, s)
		ui.Column(c).FillWidth().Gap(2 * s).Children(func() {
			cur := a.account()
			scrollArea(c, "account-menu-scroll").FillWidth().MaxHeight(320 * s).Gap(2 * s).Children(func() {
				for _, acc := range a.snapshot.Accounts {
					ac := acc
					active := cur != nil && cur.ID == ac.ID
					row := ui.ButtonBase(c.Key("qs-" + ac.ID)).Label("切换到 " + ac.Name).FillWidth().Height(44 * s).PaddingX(8 * s).Gap(10 * s).Radius(9 * s).AlignItems(ui.Center)
					bg := ui.Transparent
					if active {
						bg = accentSoft
					} else if row.Hovered() {
						bg = ui.RGBA(120, 130, 160, .10)
					}
					row.Background(bg)
					if row.Children(func() {
						a.headAvatar(c, &ac, 26*s, func() {
							ui.Box(c).Size(26*s, 26*s).Radius(6.5*s).Background(panel).Border(1, line).Center().Shrink(0).Children(func() {
								ui.Icon(c, icons["account"]).Size(13*s, 13*s).TextColor(accent)
							})
						})
						fg := ink
						if active {
							fg = accent
						}
						ui.Text(c, ac.Name).FontSize(typeCallout * s).FontWeight(600).TextColor(fg).SingleLine().Grow(1).MinWidth(0).Tooltip(ac.Name)
						a.accountTypeTag(c, ac)
						if active {
							ui.Icon(c, icons["check"]).Size(13*s, 13*s).TextColor(accent).Shrink(0)
						} else {
							ui.Box(c).Size(13*s, 13*s).Shrink(0)
						}
					}).Clicked() && !a.pendingOp("select") {
						a.accountMenuOpen = false
						if !active {
							a.execute("select", map[string]string{"accountId": ac.ID}, func(json.RawMessage) {
								if a.page == "wardrobe" {
									a.loadAppearance()
								}
							})
						}
					}
				}
			})
			ui.Box(c).FillWidth().Height(1).Margin(4*s, 0).Background(line)
			// Footer rows share the account rows' geometry: same height, hover,
			// a 26-DIP icon column, so the text lines up with the names.
			item := func(key, text, icon string) bool {
				b := ui.ButtonBase(c.Key(key)).Label(text).FillWidth().Height(44 * s).PaddingX(8 * s).Radius(9 * s)
				if b.Hovered() {
					b.Background(ui.RGBA(120, 130, 160, .10))
				}
				return b.Children(func() {
					ui.Row(c).FillWidth().FillHeight().Gap(10 * s).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(26*s, 26*s).Center().Shrink(0).Children(func() {
							ui.Icon(c, icons[icon]).Size(14*s, 14*s).TextColor(inkSecondary)
						})
						ui.Text(c, text).FontSize(typeCallout * s).TextColor(ink).SingleLine().Grow(1).MinWidth(0)
					})
				}).Clicked()
			}
			if item("qs-manage", a.L("账号管理", "Manage accounts"), "account") {
				a.accountMenuOpen, a.accountAdding = false, false
				a.openDialog("account")
			}
			if item("qs-add", t.AccAddAccountBtn, "plus") {
				a.accountMenuOpen, a.accountAdding = false, true
				a.openDialog("account")
			}
		})
	})
}

const launchDockHeight float32 = 54

const launchDockBottom float32 = 16

// The launch dock gave way to the instance hero's 启动游戏 button.
func (a *App) launchDockVisible() bool { return false }

// Keep an opaque panel, but let the page extend behind its rounded corners.
// Transparent wrapper nodes must not paint or catch input in those corners.
func (a *App) launchDock(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	r := a.root()
	shown := a.launchDockVisible()
	dock := ui.Box(c.Key("launch-dock-slot")).Absolute().Left(0).Right(0).Bottom(launchDockBottom * s).Height(launchDockHeight * s).PassThrough().Transition(ui.ElementTransition{
		Animation: &motionEnter,
		Enter:     &ui.Motion{Y: 20 * s, Opacity: 0},
		Exit:      &ui.Motion{Y: 20 * s, Opacity: 0},
	})
	if shown {
		dock.Children(func() {
			ui.Box(c).Fill().PaddingX(pageGutter * s).PassThrough().Children(func() {
				glass(ui.Row(c.Key("launch-dock")).Label("启动栏").Height(launchDockHeight*s).FillWidth().MaxWidth(1160*s).AlignSelf(ui.Center).Padding(8*s, 14*s, 8*s, 16*s).Gap(12*s), 16*s, s).
					LinearGradient(ui.LinearGradient{From: panel, To: ui.Hex("#f8fbff"), Angle: 180}).
					PassThrough().HandleInput(a.forwardDockScroll).AlignItems(ui.Center).Children(func() {
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
					if v := a.gameVersion(); v == nil || !loaderLogo(c, v.Loader, 34*s) {
						appleSoftIconBox(c, 34*s, iconName, tone[0], tone[1], panel)
					}

					ui.Column(c.Key("dock-version-" + a.versionSwapKey())).Grow(1).Basis(0).MinWidth(0).Clip().PassThrough().Gap(3 * s).Transition(versionSwap(s)).Children(func() {
						v := a.gameVersion()
						switch {
						case r == nil:
							ui.Text(c, t.TopNoInstance).FillWidth().MinWidth(0).FontSize(typeSubhead * s).Bold().TextColor(ink).SingleLine()
						case v == nil:
							ui.Text(c, t.HomeHeroNoVersion).FillWidth().MinWidth(0).FontSize(typeSubhead * s).Bold().TextColor(ink).SingleLine()
							label(c, midEllipsis(c, r.Name, dockFolderMax*s, 12.5*s, 400)).FillWidth().MinWidth(0).FontSize(typeFootnote * s).SingleLine().Tooltip(r.Path)
						default:
							title, sub := a.cardText(*r, *v)
							titleWithTags(c, title, v.ID, typeSubhead*s, 700, a.versionTags(*r, *v, true), 3)
							label(c, joinDistinct(midEllipsis(c, r.Name, dockFolderMax*s, 12.5*s, 400), sub)).FillWidth().MinWidth(0).FontSize(typeFootnote * s).SingleLine().Tooltip(r.Path)
						}
					})
					if r != nil {
						if verRunning(r) && button(c, "dock-stop", t.HomeStopBtn, "cross", false, a.instancePending("stop")) {
							a.execute("stop", map[string]string{"rootId": r.ID}, nil)
						}
					}
					// Launcher settings sit right before the primary launch button.
					if fixedButton(c, "dock-settings", t.NavSettings, "settings", 0, 29*s, false, false) {
						a.goPage("settings")
					}
					launchText := t.HomeLaunchBtn
					if verRunning(r) {
						launchText = t.HomeRunningBtn
					}
					why := whyOf(a.instancePending("launch", "launchAgain"), t.WhyBusy, verRunning(r), t.WhyRunning, r != nil && a.gameVersion() == nil, t.WhyNoVersion)
					pendingWhy = why
					if fixedButton(c, "launch", launchText, "play", 118*s, 30*s, true, why != "") {
						a.launch()
					}
				})
			})
		})
	}
}

// A wheel or touchpad gesture over the dock's labels/buttons still scrolls
// the page; pointer presses continue to use the buttons' normal handlers.
func (a *App) forwardDockScroll(ev ui.InputEvent) bool {
	if ev.Kind != ui.InputScroll || ev.Mods&ui.Shift != 0 {
		return false
	}
	track := scrollTracks["page-scroll-"+a.page]
	if track == nil {
		return false
	}
	y := max(0, min(track.st.MaxY, track.st.Y+ev.DY))
	if y == track.st.Y {
		return false
	}
	track.st.Y = y
	return true
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

// End padding lets the last row clear the floating controls and notices at
// scroll end. It does not shorten the viewport or block content moving below.
func (a *App) scrollEndPad(s float32) float32 {
	pad := space6 * s
	if a.launchDockVisible() {
		pad += (launchDockHeight + launchDockBottom) * s
	}
	if a.message != "" || a.noticeShown != "" {
		pad += toastReserve * s
	}
	return pad
}

// chromeBg is the shared title bar + sidebar material.
var chromeBg = ui.Hex("#edf0f7")

// contentPanel is the lighter, inset content surface: one hairline, one
// radius, the same 10-DIP margin to the window edges everywhere.
func contentPanel(e ui.Element, s float32) ui.Element {
	return e.Margin(0, 10*s, 10*s, 0).Radius(14*s).Background(ui.Hex("#f9fafd")).Border(1, ui.RGBA(60, 70, 110, .10)).Clip().Shadow(0, 1*s, 3*s, 0, ui.RGBA(30, 40, 80, .04))
}

// jobTitles renders a task title from its key at display time, so it
// follows the UI language and re-renders on a live switch.
var jobTitles = map[string][2]string{
	"pack.download":      {"下载 %s", "Download %s"},
	"pack.update":        {"更新 %s", "Update %s"},
	"pack.rollback":      {"回滚 %s", "Roll back %s"},
	"game.install":       {"安装 Minecraft %s", "Install Minecraft %s"},
	"java.download":      {"下载 Java %s", "Download Java %s"},
	"game.launch":        {"启动 %s", "Launch %s"},
	"world.restore":      {"还原存档 %s", "Restore world %s"},
	"world.backup":       {"备份存档 %s", "Back up world %s"},
	"world.backupAll":    {"备份全部存档%s", "Back up all worlds%s"},
	"mods.installDeps":   {"安装模组及前置%s", "Install mods and dependencies%s"},
	"mods.detect":        {"检测模组版本%s", "Check mod versions%s"},
	"addon.resourcepack": {"安装材质包%s", "Install resource packs%s"},
	"addon.shader":       {"安装光影包%s", "Install shaders%s"},
}

// jobTitle is a task's display name in the current language.
func (a *App) jobTitle(j launcher.Job) string {
	key, arg := j.TitleKey, j.TitleArg
	if key == "" {
		key, arg, _ = launcher.JobTitleKey(j.Title)
	}
	if f, ok := jobTitles[key]; ok {
		if a.isEN() {
			return fmt.Sprintf(f[1], arg)
		}
		return fmt.Sprintf(f[0], arg)
	}
	return j.Title
}

// jobPill is the fixed-width title-bar progress pill: name, thin bar,
// smoothed speed (or the step name) and percent; several tasks aggregate.
func (a *App) jobPill(c *ui.Context) {
	s := uiScale(c)
	var run []launcher.Job
	for _, j := range a.snapshot.Jobs {
		if j.State == "running" || j.State == "queued" {
			run = append(run, j)
		}
	}
	if len(run) == 0 {
		return
	}
	var done, total, speed, bDone, bTotal int64
	for _, j := range run {
		speed += j.Speed
		bDone += j.BytesDone
		bTotal += j.BytesTotal
		done += int64(j.Done)
		total += int64(j.Total)
	}
	frac := 0.0
	switch {
	case bTotal > 0:
		frac = float64(bDone) / float64(bTotal)
	case total > 0:
		frac = float64(done) / float64(total)
	}
	frac = min(1, max(0, frac))
	name := a.jobTitle(run[0])
	tip := name
	if run[0].Total > 0 {
		tip += fmt.Sprintf("  (%d/%d)", run[0].Done, run[0].Total)
	}
	if len(run) > 1 {
		name = fmt.Sprintf(a.L("%d 个任务", "%d tasks"), len(run))
		tip = ""
		for _, j := range run {
			tip += a.jobTitle(j) + "\n"
		}
	}
	right := fmt.Sprintf("%d%%", int(frac*100))
	mid := ""
	if speed >= 1024 {
		mid = fmtSize(speed) + "/s"
	} else if len(run) == 1 {
		mid = jobStep(a.jobMsg(run[0].Message))
	}
	b := ui.ButtonBase(c.Key("title-job")).Label(a.L("查看下载", "View downloads")).Tooltip(strings.TrimSpace(tip)).Width(300 * s).Height(btnSm * s).Shrink(0).Radius(999).Clip().Background(accentSoft)
	if b.Hovered() {
		b.Background(accent.Alpha(.16))
	}
	if b.Children(func() {
		ui.Column(c).Fill().Gap(0).Children(func() {
			ui.Row(c).FillWidth().Grow(1).PaddingX(12 * s).Gap(8 * s).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons["download"]).Size(12*s, 12*s).TextColor(accent).Shrink(0)
				ui.Text(c, name).FontSize(typeFootnote * s).FontWeight(600).TextColor(accent).SingleLine().Grow(1).Basis(0).MinWidth(0)
				ui.Box(c).Width(70 * s).Shrink(0).AlignItems(ui.End).Children(func() {
					numText(c, mid, typeCaption*s, inkSecondary).SingleLine()
				})
				ui.Box(c).Width(32 * s).Shrink(0).AlignItems(ui.End).Children(func() {
					numText(c, right, typeCaption*s, accent).FontWeight(650)
				})
			})
			ui.Row(c).FillWidth().Height(3 * s).Shrink(0).Background(accent.Alpha(.12)).Children(func() {
				ui.Box(c).FillHeight().Width(0).Grow(float32(frac)).Background(accent)
				ui.Box(c).FillHeight().Width(0).Grow(float32(1 - frac))
			})
		})
	}).Clicked() {
		a.goPage("tasks")
	}
}

// jobStep is the step part of a progress message ("正在校验" from
// "正在校验 (3/10): x.jar"), shown instead of a speed.
func jobStep(msg string) string {
	if i := strings.IndexAny(msg, "(（:："); i > 0 {
		msg = msg[:i]
	}
	return strings.TrimSpace(msg)
}
