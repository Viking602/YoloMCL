package nativeui

import "github.com/egoist/mygo/ui"

type underlineTab struct{ key, text string }

type underlineTabStyle struct {
	label, prefix, indicator string
	expanded                 bool
}

// TabsBase supplies keyboard navigation and tab semantics. The indicator keeps
// its own identity so rapid changes retarget one line instead of leaving ghosts.
func underlineTabs(c *ui.Context, key string, tabs []underlineTab, selected string, style underlineTabStyle) string {
	if len(tabs) == 0 {
		return selected
	}
	s := uiScale(c)
	active := 0
	widths := make([]float32, len(tabs))
	for i, tab := range tabs {
		if tab.key == selected {
			active = i
		}
		w, _ := c.MeasureText(0, ui.Span{Text: tab.text, Font: uiSansFamily(), Size: typeCallout * s, Weight: 600})
		widths[i] = w + 24*s
	}
	parts := ui.TabsBase(c.Key(key), &active, len(tabs))
	track := parts.List.Label(style.label).Font(uiSansFamily()).FillWidth().MinWidth(0).Gap(4 * s).AlignItems(ui.Center)
	track.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H - 1, W: r.W, H: 1}, line, 0)
	}).Children(func() {
		buttons := make([]ui.Element, len(tabs))
		// Process every click/key before painting, including a backwards switch.
		for i, tab := range tabs {
			c.Key(tab.key)
			b := parts.Tab(i).Label(style.prefix + tab.text).Height(40 * s).Center()
			if style.expanded {
				b.Height(36 * s).Grow(1).Basis(0).MinWidth(0)
			} else {
				b.Width(widths[i])
			}
			buttons[i] = b
		}
		for i, tab := range tabs {
			b := buttons[i]
			fg := inkSecondary
			if i == active {
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
		indicator := ui.Box(c.Key("indicator")).Label(style.indicator).Absolute().Bottom(0).Height(2 * s).Radius(radiusPill).Background(accent)
		if style.expanded {
			// Percent offsets also follow resizes on the first layout pass. The
			// margins account for the gaps between equal-width flex children.
			i := track.AnimateTo("indicator-index", float32(active), motionMove)
			n := float32(len(tabs))
			indicator.LeftPercent(100*i/n).RightPercent(100*(n-i-1)/n).
				Margin(0, 10*s+4*s*(n-i-1)/n, 0, 10*s+4*s*i/n)
		} else {
			x := float32(10) * s
			for i := 0; i < active; i++ {
				x += widths[i] + 4*s
			}
			indicator.Left(track.AnimateTo("indicator-x", x, motionMove)).
				Width(track.AnimateTo("indicator-width", widths[active]-20*s, motionMove))
		}
	})
	return tabs[active].key
}
