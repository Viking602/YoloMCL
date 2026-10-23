package nativeui

import (
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type instTabItem struct{ key, text string }

func (a *App) instTabs(_ launcher.Instance) []instTabItem {
	return []instTabItem{
		{"overview", a.L("概览", "Overview")},
		{"mods", a.L("模组", "Mods")},
		{"resourcepack", a.L("资源包", "Resource packs")},
		{"shader", a.L("光影", "Shaders")},
		{"worlds", a.L("存档", "Worlds")},
		{"logs", a.L("日志", "Logs")},
		{"settings", a.L("设置", "Settings")},
	}
}

// setInstTab switches the right-side tab and loads what it shows.
func (a *App) setInstTab(tab string) {
	if tab == "" {
		tab = "overview"
	}
	if a.instTab != tab {
		a.addonFilterKind = "" // filters belong to one list
		if tab == "overview" {
			a.loadInstanceStatus()
		}
	}
	a.instTab = tab
	switch tab {
	case "overview", "mods":
		if a.addonTab != "mods" {
			a.addonTab = "mods"
			a.loadMods()
		}
	case "resourcepack", "shader":
		if a.addonTab != tab {
			a.addonTab = tab
			a.loadMods()
		}
	case "logs":
		a.loadLogs()
	case "worlds":
		a.loadWorlds()
	case "settings":
		if r := a.root(); r != nil {
			a.loadVersionConfigForm(r.SelectedVersion)
		}
	}
}

func (a *App) instanceView(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	if a.instTab == "" {
		a.instTab = "overview"
	}
	if r := a.root(); r != nil && a.instLoaded != r.ID {
		a.instLoaded = r.ID
		a.instScrollPending = true
		a.loadMods()
	}
	ui.Row(c.Key("instance-split")).Label("实例视图").Fill().Gap(0).AlignItems(ui.Stretch).Children(func() {
		a.instanceList(c)
		contentPanel(ui.Column(c.Key("instance-detail")), s).Grow(1).Basis(0).MinWidth(0).Children(func() {
			r := a.root()
			switch {
			case len(a.snapshot.Instances) == 0:
				ui.Column(c).Fill().Padding(pageTop*s, pageGutter*s).Children(func() {
					emptyState(c, "onboarding-card", "gamepad", t.HomeEmptyTitle, t.HomeEmptyDesc, func() {
						if button(c, "add-root", t.HomeAddFolderBtn, "folder-add", true, false) {
							a.openDialog("add")
						}
					})
				})
			case r == nil:
			default:
				a.instanceHero(c, *r)
				a.instanceTabBar(c, *r)
				ui.Box(c.Key("inst-tab-slot")).Grow(1).MinHeight(0).FillWidth().Clip().Children(func() {
					ui.Column(c.Key("inst-tab-" + a.instTab)).Absolute().Left(0).Top(0).Right(0).Bottom(0).Transition(versionSwap(s)).Children(func() {
						if a.instTab == "overview" {
							a.fixedOverviewPane(c, *r)
							return
						}
						if a.instTab == "logs" {
							// The log pane owns its scrolling. Its header and toolbar
							// stay fixed, with the line list taking the remaining space.
							ui.Column(c.Key("instance-log-pane")).Fill().MinHeight(0).Padding(22*s, pageGutter*s, 22*s).Gap(16 * s).Clip().Children(func() {
								a.instanceTabContent(c, *r)
							})
							return
						}
						tabScroll := scrollArea(c, "page-scroll-home-"+a.instTab)
						a.tabViewBounds = tabScroll.LayoutBounds()
						tabScroll.Grow(1).MinHeight(0).Padding(22*s, pageGutter*s, 32*s).Children(func() {
							ui.Column(c).FillWidth().Gap(20 * s).Children(func() {
								a.instanceTabContent(c, *r)
							})
						})
					})
				})
			}
		})
	})
}

// Keep the overview in one viewport. Its three dashboard rows keep their
// proportions; only compact windows reduce the canvas to fit the available
// height. It owns no scroll state, including during instance changes.
func (a *App) fixedOverviewPane(c *ui.Context, r launcher.Instance) {
	s := uiScale(c)
	ui.Column(c.Key("instance-overview-pane")).Fill().MinHeight(0).Padding(16*s, pageGutter*s, 16*s).Clip().Children(func() {
		viewport := ui.Box(c.Key("overview-viewport")).Label("概览固定视区").Fill().MinHeight(0).Clip()
		bounds := viewport.LayoutBounds()
		w, h := bounds.W, bounds.H
		if w <= 0 || h <= 0 {
			ww, wh := c.Size()
			w, h = ww-(instListWidth+2*pageGutter+12)*s, wh-(barHeight+132+56+44)*s
			c.AnimationFrame()
		}
		height := 740 * s
		fit := min(float32(1), max(float32(.1), h/height))
		viewport.Children(func() {
			ui.Box(c.Key("overview-canvas")).Absolute().Left(0).Top(0).Width(w/fit).Height(height).
				TransformOrigin(0, 0).Scale(fit, fit).Children(func() { a.instanceTabContent(c, r) })
		})
	})
}

func (a *App) instanceTabBar(c *ui.Context, r launcher.Instance) {
	s := uiScale(c)
	ui.Row(c.Key("inst-tabs")).Label("实例标签").FillWidth().Padding(16*s, pageGutter*s, 0, pageGutter*s).Gap(4 * s).AlignItems(ui.Center).Shrink(0).Draw(func(p *ui.Painter, b ui.Rect) {
		p.Fill(ui.Rect{X: b.X + pageGutter*s, Y: b.Y + b.H - 1, W: b.W - 2*pageGutter*s, H: 1}, line, 0)
	}).Children(func() {
		for _, it := range a.instTabs(r) {
			tab := it
			sel := a.instTab == tab.key
			b := ui.ButtonBase(c.Key("inst-tab-btn-" + tab.key)).Label(tab.text).Height(40 * s).PaddingX(12 * s).AlignItems(ui.Center).Shrink(0)
			fg := inkSecondary
			if sel {
				fg = accent
			} else if b.Hovered() {
				fg = ink
			}
			fg = b.AnimateColor("fg", fg, motionFast)
			w := 500
			if sel {
				w = 650
			}
			b.Draw(func(p *ui.Painter, rr ui.Rect) {
				if sel {
					p.Fill(ui.Rect{X: rr.X + 10*s, Y: rr.Y + rr.H - 2*s, W: rr.W - 20*s, H: 2 * s}, accent, 1*s)
				}
			})
			if b.Children(func() {
				ui.Text(c, tab.text).FontSize(typeCallout * s).FontWeight(w).TextColor(fg).NoWrap()
			}).Clicked() && !sel {
				a.setInstTab(tab.key)
			}
		}
	})
}

func (a *App) instanceTabContent(c *ui.Context, r launcher.Instance) {
	_ = uiScale(c)
	t := a.tr()
	switch a.instTab {
	case "mods", "resourcepack", "shader":
		if a.addonTab != a.instTab {
			a.addonTab = a.instTab
			a.loadMods()
		}
		a.embedMods = true
		a.modPage(c)
		a.embedMods = false
	case "worlds":
		a.worldsView(c, r)
	case "logs":
		if a.dialog != "logs" { // the all-games window owns the log pane while open
			a.logsDialog(c)
		}
	case "settings":
		if a.form.ConfigVersion != r.SelectedVersion {
			a.loadVersionConfigForm(r.SelectedVersion)
		}
		a.instanceSettingsView(c, r)
	default:
		if len(r.Versions) == 0 {
			emptyState(c, "root-empty", "layers", t.HomeRootEmptyTitle, r.Name, func() {
				if button(c, "install-empty-root", t.HomeInstallBtn, "plus", true, false) {
					a.openDialog("install")
				}
			})
			return
		}
		a.instanceOverview(c, r)
	}
}

// kvLabelW keeps overview values in one column next to their labels.
const kvLabelW float32 = 96
