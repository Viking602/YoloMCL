package nativeui

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// Adapted from CrychicUI's Skeleton sheen (MIT; see the embedded license).
// Only painted placeholders ask for another frame; reduced motion is static.
func skeleton(c *ui.Context, key string) ui.Element {
	s := uiScale(c)
	still := c.Preferences().ReduceMotion
	base, shine := ink.Alpha(.055), ink.Alpha(.075)
	return ui.Box(c.Key(key)).Role(ui.RoleNone).FillWidth().Height(14 * s).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
		view := p.Visible()
		if r.W <= 0 || r.H <= 0 || r.X >= view.X+view.W || r.Y >= view.Y+view.H || r.X+r.W <= view.X || r.Y+r.H <= view.Y {
			return
		}
		radius := radiusMin * s
		p.Fill(r, base, radius)
		if still {
			return
		}
		p.AnimationFrame()
		w := min(220*s, view.W*.4)
		phase := float32(p.Now().UnixMilli()%1800) / 1800
		x := view.X - w + (view.W+2*w)*phase
		p.Clip(r, radius, func() {
			p.FillGradient(ui.Rect{X: x, Y: r.Y, W: w, H: r.H}, ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{
				{Color: ui.Transparent, At: 0}, {Color: shine, At: .5}, {Color: ui.Transparent, At: 1},
			}}, 0)
		})
	})
}

func (a *App) gameSettingsSkeleton(c *ui.Context, pack bool) {
	s := uiScale(c)
	ui.Column(c.Key("preferences-loading")).Label("游戏设置加载占位").FillWidth().Grow(1).Basis(0).MinHeight(0).Gap(16 * s).Children(func() {
		ui.Row(c).FillWidth().Height(36 * s).Shrink(0).Gap(4 * s).Children(func() {
			n := 5
			if pack {
				n++
			}
			for i := 0; i < n; i++ {
				ui.Box(c).Grow(1).Basis(0).MinWidth(0).FillHeight().Center().Children(func() {
					skeleton(c, fmt.Sprint("tab-", i)).WidthPercent(55).Height(12 * s)
				})
			}
		})
		skeleton(c, "search").Height(34 * s)
		ui.Column(c).FillWidth().Grow(1).Basis(0).MinHeight(0).Clip().Children(func() {
			settingsRowsSkeleton(c, "rows", 5)
		})
		label(c, a.L("keybindings.loading")).FontSize(typeCaption * s).FillWidth().MinWidth(0).Shrink(0)
	})
}

func settingsRowsSkeleton(c *ui.Context, key string, rows int) {
	s := uiScale(c)
	ui.Column(c.Key(key)).FillWidth().Shrink(0).Border(1, line).Radius(radiusControl * s).Clip().Children(func() {
		ui.Row(c).FillWidth().Height(38 * s).Shrink(0).PaddingX(16 * s).AlignItems(ui.Center).Children(func() {
			skeleton(c, "heading").Width(100 * s).Height(12 * s)
		})
		for i := 0; i < rows; i++ {
			ui.Row(c.Key(i)).FillWidth().Height(60 * s).Shrink(0).PaddingX(16 * s).Gap(20 * s).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(8 * s).Children(func() {
					skeleton(c, "name").WidthPercent(float32(32 + i%3*8)).Height(12 * s)
					skeleton(c, "hint").WidthPercent(24).Height(9 * s)
				})
				skeleton(c, "value").Width(120 * s).Height(32 * s)
			})
		}
	})
}
