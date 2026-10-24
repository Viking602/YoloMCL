package nativeui

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

type addonFilterMotion struct {
	scope, key string
	progress   float32
}

// Keep the buttons stationary and move just one selection surface beneath
// them. Measured widths also accommodate English labels and large counts.
func (a *App) addonFilterPills(c *ui.Context, total, on, off, updates int) {
	s := uiScale(c)
	type option struct {
		key, text, count string
		x, width         float32
	}
	options := []option{
		{key: "", text: a.L("全部", "All"), count: fmt.Sprint(total)},
		{key: "on", text: a.L("已启用", "Enabled"), count: fmt.Sprint(on)},
	}
	if off > 0 || a.addonFilterKind == "off" {
		options = append(options, option{key: "off", text: a.L("已禁用", "Disabled"), count: fmt.Sprint(off)})
	}
	if updates > 0 || a.addonFilterKind == "update" {
		options = append(options, option{key: "update", text: a.L("可更新", "Updates"), count: fmt.Sprint(updates)})
	}
	selected := a.addonFilterKind
	var width, x, w float32
	for i := range options {
		o := &options[i]
		labelW, _ := c.MeasureText(0, ui.Span{Text: o.text, Font: uiSansFamily(), Size: typeFootnote * s, Weight: 600})
		countW, _ := c.MeasureText(0, ui.Span{Text: o.count, Font: uiSansFamily(), Features: "tnum", Size: typeFootnote * s, Weight: 500})
		o.x, o.width = width, labelW+countW+28*s
		if o.key == selected {
			x, w = o.x, o.width
		}
		width += o.width + 6*s
	}
	track := ui.Row(c.Key("addon-filter-track")).Label("资源筛选").Font(uiSansFamily()).FocusGroup(ui.Horizontal).Width(width - 6*s).Height(28 * s).Gap(6 * s).Shrink(0)
	x = track.AnimateTo("selection-x", x, motionMove)
	w = track.AnimateTo("selection-width", w, motionMove)
	track.Draw(func(p *ui.Painter, r ui.Rect) {
		for _, o := range options {
			p.Fill(ui.Rect{X: r.X + o.x, Y: r.Y, W: o.width, H: r.H}, ui.RGBA(120, 130, 160, .08), r.H/2)
		}
	}).Children(func() {
		ui.Box(c.Key("addon-filter-selection")).Label("筛选选中背景").Absolute().Left(x).Top(0).Width(w).Height(28 * s).Radius(999).Background(accentSoft)
		for _, o := range options {
			b := ui.ButtonBase(c.Key("addon-filter-" + o.key)).Label(o.text).Width(o.width).Height(28 * s).Gap(6 * s).Radius(999).AlignItems(ui.Center).Justify(ui.Center).Shrink(0)
			fg, hover := inkSecondary, ui.RGBA(120, 130, 160, 0)
			if selected == o.key {
				fg = accent
			} else if b.Hovered() {
				hover = ui.RGBA(120, 130, 160, .06)
			}
			fg = b.AnimateColor("fg", fg, motionFast)
			b.Background(b.AnimateColor("hover", hover, motionFast))
			if b.Children(func() {
				ui.Text(c, o.text).FontSize(typeFootnote * s).FontWeight(600).TextColor(fg).NoWrap()
				ui.Text(c, o.count).FontFeatures("tnum").FontSize(typeFootnote * s).FontWeight(500).TextColor(fg.Alpha(.7)).NoWrap()
			}).Clicked() {
				a.addonFilterKind = o.key
			}
		}
	})
}

// Animate only a filter change, not background metadata refreshes. The new
// results are immediately interactive; rapid switches continue the entrance
// already in progress instead of flashing or retaining stale clickable cards.
func (a *App) addonResultBody(c *ui.Context) ui.Element {
	s := uiScale(c)
	body := ui.Column(c.Key("addon-results")).Label("筛选结果").FillWidth().Gap(20 * s)
	m := &a.addonFilterMotion
	scope := a.versionSwapKey() + "|" + a.addonTab
	if m.scope != scope || c.Preferences().ReduceMotion {
		body.Jump("reveal", 1)
	} else if m.key != a.addonFilterKind && m.progress == 1 {
		body.Jump("reveal", 0)
	}
	m.scope, m.key = scope, a.addonFilterKind
	m.progress = body.AnimateTo("reveal", 1, motionEnter)
	y := (1 - m.progress) * 6 * s
	// Layout offsets keep text at native resolution, without resampling the
	// whole virtualized list as a transformed layer on every animation frame.
	return body.Opacity(.55+.45*m.progress).Margin(y, 0, -y, 0)
}
