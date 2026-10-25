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

func appleSoftIconBox(c *ui.Context, size float32, iconName string, from, to, fg ui.Color) {
	ui.Box(c).Size(size, size).Radius(size*.3).Shrink(0).LinearGradient(tintGradient(from, to)).InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, .35)).Shadow(0, size*.08, size*.25, -size*.06, from.Alpha(.35)).Center().Children(func() {
		if icons[iconName] != nil {
			ui.Icon(c, icons[iconName]).Size(size*.52, size*.52).TextColor(fg)
		}
	})
}

// Apple Pill Badge with soft gradient fill
func badge(c *ui.Context, s string, color ui.Color) {
	scale := uiScale(c)
	from, to := ui.Hex("#f1f5f9"), ui.Hex("#e8eef6")
	fg := color
	switch color {
	case accent, blue:
		from, to = ui.Hex("#eaf3ff"), ui.Hex("#dcebff")
		fg = accent
	case success, successInk:
		from, to = ui.Hex("#e8faed"), ui.Hex("#d3f5dd")
		fg = successInk
	case danger:
		from, to = ui.Hex("#ffeceb"), ui.Hex("#ffdfdd")
		fg = danger
	case warning, warningInk:
		from, to = ui.Hex("#fff6e5"), ui.Hex("#ffebcc")
		fg = warningInk
	}
	ui.Row(c).Shrink(0).Padding(3*scale, 9*scale).Radius(999).LinearGradient(tintGradient(from, to)).Border(1, ui.RGBA(255, 255, 255, .7)).Children(func() {
		ui.Text(c, s).FontSize(typeCaption * scale).FontWeight(600).TextColor(fg).NoWrap().Shrink(0)
	})
}

func tagChip(c *ui.Context, s string) {
	scale := uiScale(c)
	ui.Row(c).Shrink(0).Padding(3*scale, 9*scale).Radius(999).Background(fieldColor).Children(func() {
		ui.Text(c, s).FontSize(typeCaption2 * scale).FontWeight(500).TextColor(inkSecondary).NoWrap().Shrink(0)
	})
}

func card(c *ui.Context, key string, fn func()) {
	s := uiScale(c)
	glass(ui.Column(c.Key(key)).FillWidth().Clip().Gap(14*s).Padding(18*s, 22*s), radiusCard*s, s).Children(fn)
}

// textField is the one single-line input: a fixed control height with no
// vertical padding, so the toolkit centers the line box and mono/CJK text
// sit on the same middle line in every field; 13-DIP side insets.
func textField(c *ui.Context, value *string) ui.Element {
	s := uiScale(c)
	return ui.TextInput(c, value).Height(btnMd*s).Padding(0, 13*s).FontSize(typeCallout * s)
}

func empty(c *ui.Context, titleText, description string) {
	emptyState(c, "empty", "folder", titleText, description, nil)
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
		tooltip(c, ui.Text(c, text).FontSize(typeCallout*s).FontWeight(500).TextColor(color).TextAlign(ui.Center).MinWidth(0).Shrink(1).SingleLine(), text)
	})
}

// statusSpinner draws a ring with an accent arc that advances with the clock.
func statusSpinner(c *ui.Context, size float32) {
	phase := float32(0)
	if !c.Preferences().ReduceMotion {
		phase = float32(c.Now().UnixMilli()%1200) / 1200
		c.After(40 * time.Millisecond)
	}
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
