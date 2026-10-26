package nativeui

import "github.com/egoist/mygo/ui"

type navItem struct{ page, text, icon string }

func (a *App) navItems() []navItem {
	return []navItem{{"home", a.L("navbar.instances"), "home"}, {"discover", a.L("navbar.discover"), "search"}, {"wardrobe", a.L("navbar.wardrobe"), "skin"}, {"tasks", a.L("navbar.tasks"), "tasks"}}
}

// navRects keeps each tab's previous-frame bounds so the selection
// indicator can slide to it.
var navRects = map[string]ui.Rect{}

// indicator returns the animated x/width of the selection (relative to the
// track) and its opacity: it fades out when no tab is active (设置).
func (a *App) indicator(track ui.Element) (x, w, alpha float32) {
	tb := track.LayoutBounds()
	r, ok := navRects[a.page]
	target := float32(0)
	if ok && tb.W > 0 {
		target = 1
		x, w = r.X-tb.X, r.W
	}
	x = track.AnimateTo("ind-x", x, motionMove)
	w = track.AnimateTo("ind-w", w, motionMove)
	alpha = track.AnimateTo("ind-a", target, motionFast)
	return
}

// Variant B — plain text tabs on the bar surface: no capsule; the active
// tab is accent-colored with a 2-DIP underline that slides between tabs.
func (a *App) navBar(c *ui.Context) {
	s := uiScale(c)
	track := ui.Row(c.Key("sidebar")).Label("导航标签").FocusGroup(ui.Horizontal).Height(34 * s).Padding(3 * s).Radius(10 * s).Background(ui.RGBA(120, 130, 160, .12)).Gap(2 * s).Shrink(1).MinWidth(0).AlignItems(ui.Center)
	x, w, alpha := a.indicator(track)
	track.Draw(func(p *ui.Painter, r ui.Rect) {
		if alpha <= .01 || w <= 0 {
			return
		}
		pill := ui.Rect{X: r.X + x, Y: r.Y + 3*s, W: w, H: r.H - 6*s}
		p.Shadow(pill, 7.5*s, 0, 1*s, 3*s, 0, ui.RGBA(15, 23, 42, .10*alpha))
		p.Fill(pill, panel.Alpha(alpha), 7.5*s)
	})
	track.Children(func() {
		for _, it := range a.navItems() {
			a.textTab(c, s, it)
		}
	})
}

func (a *App) textTab(c *ui.Context, s float32, it navItem) {
	selected := a.page == it.page
	b := ui.ButtonBase(c.Key("nav-" + it.page)).Label(it.text).Height(28 * s).PaddingX(14 * s).Gap(6 * s).AlignItems(ui.Center).Radius(radiusControl*s - 2*s).Shrink(0)
	fg := inkSecondary
	switch {
	case selected:
		fg = ink
	case b.Hovered():
		fg = ink
	}
	fg = b.AnimateColor("fg", fg, motionFast)
	weight := 500
	if selected {
		weight = 650
	}
	if b.Children(func() {
		ui.Text(c, it.text).FontSize(typeCallout * s).FontWeight(weight).TextColor(fg).NoWrap()
		a.taskCount(c, s, it.page)
	}).Clicked() {
		a.goPage(it.page)
	}
	navRects[it.page] = b.LayoutBounds()
}

func (a *App) taskCount(c *ui.Context, s float32, page string) {
	if page != "tasks" {
		return
	}
	n := 0
	for _, j := range a.snapshot.Jobs {
		if j.State == "running" || j.State == "queued" {
			n++
		}
	}
	if n == 0 {
		return
	}
	ui.Row(c).Shrink(0).Height(16 * s).PaddingX(5 * s).Radius(radiusPill).Background(accent).Center().Children(func() {
		ui.Text(c, itoa(n)).FontSize(typeCaption2 * s).FontWeight(700).TextColor(panel).NoWrap()
	})
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return "9+"
}
