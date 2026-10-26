package nativeui

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// Native coordinates scale uniformly from the 1536×1024 reference canvas so
// font, icon, and control baselines stay aligned across DPIs and small windows.
func uiScale(c *ui.Context) float32 {
	w, h := c.Size()
	return max(.65, min(1, w/1536, (h-42)/982)) * uiDensity
}

func (a *App) View(c *ui.Context) {
	a.frameSerial.Add(1)
	a.drainEvents()
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
	a.prepareLauncherUpdateDialog()
	if a.launcherUpdateRequired() {
		a.launcherUpdateGate(c)
		return
	}
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
		tooltip(c, ui.Row(c.Key("sidebar-brand")).Label(t.BrandAria), t.AppTitle).AlignItems(ui.Center).Gap(space2 * s).Shrink(0).Children(func() {
			brandMark(c, 23.5*s)
			ui.Text(c, "YoloMCL").FontSize(typeHeadline * s).FontWeight(800).TextColor(ink).NoWrap()
		})
		ui.Box(c).Width(space2 * s).Shrink(0)
		a.navBar(c)
		ui.Spacer(c).Grow(1)
		a.jobPill(c)
		langBtn := tooltip(c, ui.ButtonBase(c.Key("lang-toggle")).Label(t.LangToggleLabel), t.LangToggleTooltip).Height(btnSm*s).PaddingX(12*s).Gap(5*s).Radius(999).Border(1, glassEdge).Shadow(0, 1, 3, 0, ui.RGBA(40, 52, 110, .08)).AlignItems(ui.Center)
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

// A separate back control keeps navigation distinct from the settings title.
func (a *App) settingsBack(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	prev := a.prevPage
	if prev == "" || prev == "settings" {
		prev = "home"
	}
	name := map[string]string{"home": t.NavHome, "mods": t.NavMods, "discover": t.NavDiscover, "wardrobe": t.NavWardrobe, "tasks": t.NavTasks}[prev]
	b := tooltip(c, ui.ButtonBase(c.Key("settings-back")).Label(t.BackLabel), fmt.Sprintf(a.L("shell.back_to"), name)).Size(34*s, 34*s).Shrink(0).Radius(11 * s).Center()
	styleButton(b, s, false, false)
	if b.Children(func() {
		ui.Icon(c, icons["chevron-left"]).Size(15*s, 15*s).TextColor(inkSecondary)
	}).Clicked() {
		a.goPage(prev)
	}
}

// toastReserve is the room reserved for a floating page notice
// (toast height plus its gap).
const toastReserve float32 = 64

// End padding lets the last row clear the floating controls and notices at
// scroll end. It does not shorten the viewport or block content moving below.
func (a *App) scrollEndPad(s float32) float32 {
	pad := space6 * s
	if a.message != "" || a.noticeShown != "" {
		pad += toastReserve * s
	}
	return pad
}

// chromeBg is the shared title bar + sidebar material.
var chromeBg = ui.Hex("#edf0f7")

var contentBackground = ui.Hex("#f9fafd")

// contentPanel is the lighter, inset content surface: one hairline, one
// radius, the same 10-DIP margin to the window edges everywhere.
func contentPanel(e ui.Element, s float32) ui.Element {
	return e.Margin(0, 10*s, 10*s, 0).Radius(14*s).Background(contentBackground).Border(1, ui.RGBA(60, 70, 110, .10)).Clip().Shadow(0, 1*s, 3*s, 0, ui.RGBA(30, 40, 80, .04))
}
