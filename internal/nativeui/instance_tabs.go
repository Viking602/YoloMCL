package nativeui

import (
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type instanceTabMotion struct {
	scope, tab string
	progress   float32
	x          float32
	waiting    bool
}

// Keep a single live pane. Replacing its contents never retains a painted copy
// of the previous instance, and interrupted switches keep the ongoing reveal.
// Only navigation and the first loaded result trigger it, not streamed metadata.
func (a *App) instanceTabBody(c *ui.Context) ui.Element {
	s := uiScale(c)
	body := ui.Column(c.Key("inst-tab-body")).Label("实例标签内容").Absolute()
	m := &a.instTabMotion
	scope, tab := a.versionSwapKey(), a.instTab
	changed := m.scope != scope || m.tab != tab
	waiting := a.instanceTabWaiting()
	if changed {
		if m.tab != "" && (m.tab != tab || tab != "overview") && m.progress == 1 {
			body.Jump("reveal", 0)
			m.x = 0
			if m.tab != tab {
				m.x = 14
				if instanceTabIndex(tab) < instanceTabIndex(m.tab) {
					m.x = -m.x
				}
			}
		}
		m.scope, m.tab, m.waiting = scope, tab, waiting
	} else if m.waiting && !waiting {
		// A slow disk may finish after navigation settles. Give its first
		// result a fade too; subsequent batches remain stationary.
		if m.progress == 1 {
			body.Jump("reveal", .25)
			m.x = 0
		}
		m.waiting = false
	}
	if c.Preferences().ReduceMotion {
		body.Jump("reveal", 1)
	}
	m.progress = body.AnimateTo("reveal", 1, motionEnter)
	x := m.x * (1 - m.progress) * s
	// Native layout offsets preserve the viewport size and text sharpness.
	// The outer slot clips movement; the hero and tabs never move with it.
	return body.Left(x).Right(-x).Top(0).Bottom(0).Opacity(.45 + .55*m.progress)
}

func instanceTabIndex(tab string) int {
	for i, key := range []string{"overview", "seedmap", "mods", "resourcepack", "shader", "worlds", "logs", "settings"} {
		if key == tab {
			return i
		}
	}
	return 0
}

func (a *App) instanceTabWaiting() bool {
	switch a.instTab {
	case "mods":
		return a.modsLoading && len(a.mods) == 0
	case "resourcepack", "shader":
		return a.modsLoading && len(a.assetPacks) == 0
	case "worlds":
		r := a.root()
		return r != nil && ((a.service != nil && a.wv.key != r.ID+"|"+r.SelectedVersion) || (a.wv.loading && len(a.wv.list) == 0))
	case "seedmap":
		return a.seedMap == nil && a.service != nil || a.seedMap != nil && a.seedMap.loading
	case "logs":
		return a.logView.loading && len(a.logView.lines) == 0
	}
	return false
}

func (a *App) instanceTabBar(c *ui.Context, r launcher.Instance) {
	s := uiScale(c)
	ui.Column(c.Key("inst-tabs-slot")).Label("实例标签").FillWidth().Padding(16*s, pageGutter*s, 0, pageGutter*s).Shrink(0).Children(func() {
		tab := underlineTabs(c, "inst-tabs", a.instTabs(r), a.instTab, underlineTabStyle{
			label: a.L("instance.tabs"), prefix: a.L("instance.tab_prefix") + " ", indicator: a.L("instance.tab_indicator"),
		})
		if tab != a.instTab {
			a.setInstTab(tab)
		}
	})
}
