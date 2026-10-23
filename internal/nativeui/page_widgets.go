package nativeui

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

func heading(c *ui.Context, _, text, description string, actions func()) {
	pageHeader(c, nil, text, description, actions)
}

// pageHeader is the one page header: optional breadcrumb, large title,
// muted subtitle, actions on the right — aligned to the content grid.
func pageHeader(c *ui.Context, crumb func(), text, description string, actions func()) {
	s := uiScale(c)
	ui.Row(c).Label("页面标题栏").FillWidth().Gap(space5 * s).AlignItems(ui.End).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Shrink(1).Clip().Gap(space1 * s).Children(func() {
			if crumb != nil {
				crumb()
			}
			ui.Text(c, text).Label("页面标题 " + text).FontSize(typeDisplay * s).FontWeight(750).TextColor(ink).SingleLine()
			if description != "" {
				ui.Text(c, description).FontSize(typeCallout * s).TextColor(muted).MinWidth(0) // wraps: descriptive text never ellipsizes
			}
		})
		if actions != nil {
			ui.Row(c).Shrink(0).Gap(space2 * s).AlignItems(ui.Center).Children(actions)
		}
	})
}

func empty(c *ui.Context, titleText, description string) {
	emptyState(c, "empty", "folder", titleText, description, nil)
}

// emptyState is the one placeholder used by every list: a frosted card with
// a tinted squircle, a headline, a muted explanation and optional actions.
func emptyState(c *ui.Context, key, icon, titleText, description string, actions func()) {
	s := uiScale(c)
	glass(ui.Column(c.Key(key)).FillWidth().Gap(space2*s).Padding(space8*s+space4*s, space8*s), radiusCard*s, s).AlignItems(ui.Center).Children(func() {
		appleSoftIconBox(c, 52*s, icon, accent, accentIndigo, panel)
		ui.Box(c).Height(space1 * s)
		ui.Text(c, titleText).FontSize(typeTitle * s).Bold().TextColor(ink).SingleLine()
		label(c, description).MaxWidth(560 * s).TextAlign(ui.Center)
		if actions != nil {
			ui.Row(c).Gap(space2 * s).PaddingY(space2 * s).AlignItems(ui.Center).Children(actions)
		}
	})
}

// loadingState is the shared in-progress placeholder: a frosted card whose
// icon and message are centered on both axes, so loading text never hugs a
// corner of its container.
func loadingState(c *ui.Context, key, text string) {
	s := uiScale(c)
	glass(ui.Column(c.Key(key)).Label(text).FillWidth().MinHeight(132*s).Padding(space6*s), radiusCard*s, s).AlignItems(ui.Center).Justify(ui.Center).Gap(space3 * s).Children(func() {
		statusSpinner(c, 22*s)
		ui.Text(c, text).FontSize(typeCallout * s).FontWeight(500).TextColor(muted).TextAlign(ui.Center).SingleLine()
	})
}

// statusLine centers a short status message inside a form or dialog body.
func statusLine(c *ui.Context, text string, color ui.Color, busy bool) {
	s := uiScale(c)
	ui.Row(c).FillWidth().MinHeight(44*s).Padding(space2*s, space4*s).Radius(radiusControl * s).Background(fieldColor.Alpha(.6)).AlignItems(ui.Center).Justify(ui.Center).Gap(space2 * s).Children(func() {
		if busy {
			statusSpinner(c, 16*s)
		}
		ui.Text(c, text).FontSize(typeCallout * s).FontWeight(500).TextColor(color).TextAlign(ui.Center).MinWidth(0).Shrink(1).SingleLine().Tooltip(text)
	})
}

// statusSpinner draws a ring with an accent arc that advances with the clock.
func statusSpinner(c *ui.Context, size float32) {
	phase := float32(time.Now().UnixMilli()%1200) / 1200
	ui.Box(c).Size(size, size).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		rad := min(r.W, r.H) / 2
		dots := 8
		for i := 0; i < dots; i++ {
			ang := (float64(i)/float64(dots) + float64(phase)) * 2 * math.Pi
			x := cx + float32(math.Cos(ang))*rad*.72
			y := cy + float32(math.Sin(ang))*rad*.72
			alpha := .15 + .85*float32(i)/float32(dots-1)
			d := rad * .32
			p.Fill(ui.Rect{X: x - d/2, Y: y - d/2, W: d, H: d}, accent.Alpha(alpha), d/2)
		}
	})
}
