package nativeui

import (
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type instanceTabMotion struct {
	scope, tab string
	progress   float32
	x, y       float32
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
			m.x, m.y = 0, 10
			if m.tab != tab {
				m.x, m.y = 14, 0
				if instanceTabIndex(tab) < instanceTabIndex(m.tab) {
					m.x = -m.x
				}
			}
		}
		m.scope, m.tab, m.waiting = scope, tab, waiting
	} else if m.waiting && !waiting {
		// A slow disk may finish after navigation settles. Give its first
		// result a gentle entrance too; subsequent batches remain stationary.
		if m.progress == 1 {
			body.Jump("reveal", .25)
			m.x, m.y = 0, 10
		}
		m.waiting = false
	}
	if c.Preferences().ReduceMotion {
		body.Jump("reveal", 1)
	}
	m.progress = body.AnimateTo("reveal", 1, motionEnter)
	x, y := m.x*(1-m.progress)*s, m.y*(1-m.progress)*s
	// Native layout offsets preserve the viewport size and text sharpness.
	// The outer slot clips movement; the hero and tabs never move with it.
	return body.Left(x).Right(-x).Top(y).Bottom(-y).Opacity(.45 + .55*m.progress)
}

func instanceTabIndex(tab string) int {
	for i, key := range []string{"overview", "mods", "resourcepack", "shader", "worlds", "logs", "settings"} {
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
	case "logs":
		return a.logView.loading && len(a.logView.lines) == 0
	}
	return false
}

func (a *App) instanceTabBar(c *ui.Context, r launcher.Instance) {
	s := uiScale(c)
	tabs := a.instTabs(r)
	widths := make([]float32, len(tabs))
	for i, tab := range tabs {
		w, _ := c.MeasureText(0, ui.Span{Text: tab.text, Font: uiSansFamily(), Size: typeCallout * s, Weight: 600})
		widths[i] = w + 24*s
	}
	track := ui.Row(c.Key("inst-tabs")).Label("实例标签").Font(uiSansFamily()).FocusGroup(ui.Horizontal).
		FillWidth().Padding(16*s, pageGutter*s, 0, pageGutter*s).Gap(4 * s).AlignItems(ui.Center).Shrink(0)
	track.Draw(func(p *ui.Painter, b ui.Rect) {
		p.Fill(ui.Rect{X: b.X + pageGutter*s, Y: b.Y + b.H - 1, W: b.W - 2*pageGutter*s, H: 1}, line, 0)
	}).Children(func() {
		buttons := make([]ui.Element, len(tabs))
		// Read the click before styling any tab so label, indicator and body
		// use the same selection, including when motion is disabled.
		for i, tab := range tabs {
			buttons[i] = ui.ButtonBase(c.Key("inst-tab-btn-" + tab.key)).Label("实例标签 " + tab.text).Width(widths[i]).Height(40 * s).
				AlignItems(ui.Center).Justify(ui.Center).Shrink(0)
			if buttons[i].Clicked() && a.instTab != tab.key {
				a.setInstTab(tab.key)
			}
		}
		for i, tab := range tabs {
			b := buttons[i]
			fg := inkSecondary
			if a.instTab == tab.key {
				fg = accent
			} else if b.Hovered() {
				fg = ink
			}
			if !c.Preferences().ReduceMotion {
				fg = b.AnimateColor("fg", fg, motionFast)
			}
			b.Children(func() {
				ui.Text(c, tab.text).FontSize(typeCallout * s).FontWeight(600).TextColor(fg).NoWrap()
			})
		}
		// The indicator uses the latest click in this frame. Stable measured
		// button widths prevent the remaining tabs from shifting on selection.
		x, w := pageGutter*s, widths[0]
		for i, tab := range tabs {
			w = widths[i]
			if a.instTab == tab.key {
				break
			}
			x += w + 4*s
		}
		x = track.AnimateTo("indicator-x", x+10*s, motionMove)
		w = track.AnimateTo("indicator-width", w-20*s, motionMove)
		ui.Box(c.Key("inst-tab-indicator")).Label("实例标签选中线").Absolute().Left(x).Bottom(0).
			Width(w).Height(2 * s).Radius(s).Background(accent)
	})
}
