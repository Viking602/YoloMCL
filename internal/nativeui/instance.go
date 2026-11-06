package nativeui

import (
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func seedMapAvailable(r launcher.Instance) bool {
	for _, v := range r.Versions {
		if v.ID == r.SelectedVersion {
			return !v.SeedMapDisabled
		}
	}
	return false
}

func (a *App) ensureSeedMapAvailability(r launcher.Instance) {
	if a.service == nil || !r.Available {
		return
	}
	for _, v := range r.Versions {
		if v.ID == r.SelectedVersion && v.SeedMapCheckPending {
			a.execute("checkSeedMapMods", map[string]string{"rootId": r.ID, "version": v.ID}, nil)
			return
		}
	}
}

func (a *App) instTabs(r launcher.Instance) []underlineTab {
	tabs := []underlineTab{{"overview", a.L("instance.overview")}}
	if seedMapAvailable(r) {
		tabs = append(tabs, underlineTab{"seedmap", a.L("seedmap.open")})
	}
	return append(tabs, []underlineTab{
		{"mods", a.L("storage.label.mods")},
		{"resourcepack", a.L("storage.label.resourcepacks")},
		{"shader", a.L("storage.label.shaderpacks")},
		{"worlds", a.L("storage.label.saves")},
		{"logs", a.L("instance.logs")},
		{"settings", a.L("instance.settings")},
	}...)
}

// setInstTab switches the right-side tab and loads what it shows.
func (a *App) setInstTab(tab string) {
	if a.launcherUpdateRequired() {
		return
	}
	if r := a.root(); tab == "seedmap" && r != nil && !seedMapAvailable(*r) {
		tab = "overview"
	}
	if tab != a.instTab && a.guardDiscard(a.hasUnsavedDraft(), func() { a.setInstTab(tab) }) {
		return
	}
	if tab == "" {
		tab = "overview"
	}
	if a.instTab != tab {
		a.addonListData = addonListData{}
		a.resetImageRequests()
		if tab != "overview" {
			a.cancelInstanceStatus()
		}
		if tab != "mods" && tab != "resourcepack" && tab != "shader" {
			a.cancelAddonLoad()
		}
		a.addonFilterKind = "" // filters belong to one list
		if tab == "overview" {
			a.loadInstanceStatus()
		}
	}
	a.instTab = tab
	switch tab {
	case "overview":
		a.addonTab = "mods"
	case "mods":
		a.addonTab = "mods"
		a.ensureAddons()
	case "resourcepack", "shader":
		a.addonTab = tab
		a.ensureAddons()
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
	if r := a.root(); r != nil {
		a.ensureSeedMapAvailability(*r)
	}
	if a.instTab == "" {
		a.instTab = "overview"
	}
	if r := a.root(); r != nil && a.instLoaded != r.ID {
		a.instLoaded = r.ID
		a.instScrollPending = true
		a.setInstTab(a.instTab)
	}
	if r := a.root(); r != nil && a.instTab == "seedmap" && !seedMapAvailable(*r) {
		a.closeSeedMap()
		a.setInstTab("overview")
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
					a.instanceTabBody(c).Children(func() {
						ui.Column(c.Key("inst-tab-" + a.instTab)).Fill().MinHeight(0).Children(func() {
							if a.instTab == "overview" {
								a.fixedOverviewPane(c, *r)
								return
							}
							if a.instTab == "logs" {
								// The log pane owns its scrolling. Its header and toolbar
								// stay fixed, with the line list taking the remaining space.
								ui.Column(c.Key("instance-log-pane")).Fill().MinHeight(0).Padding(24*s, pageGutter*s, 24*s).Gap(16 * s).Clip().Children(func() {
									a.instanceTabContent(c, *r)
								})
								return
							}
							if a.instTab == "seedmap" {
								ui.Column(c.Key("instance-seedmap-pane")).Fill().MinHeight(0).Padding(16*s, pageGutter*s, 16*s).Children(func() { a.instanceTabContent(c, *r) })
								return
							}
							tabScroll := scrollArea(c, "page-scroll-home-"+a.instTab)
							a.tabViewBounds = tabScroll.LayoutBounds()
							tabScroll.Grow(1).MinHeight(0).Padding(24*s, pageGutter*s, 32*s).Children(func() {
								ui.Column(c).FillWidth().Gap(20 * s).Children(func() {
									a.instanceTabContent(c, *r)
								})
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
	case "seedmap":
		a.seedMapPanel(c)
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
