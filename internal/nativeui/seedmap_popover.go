package nativeui

import "github.com/egoist/mygo/ui"

func (a *App) seedMapPopover(c *ui.Context, v *seedMapView, key, name string, x, y, width, s, mapW, mapH float32, children func()) {
	x = max(8*s, min(mapW-8*s, x))
	y = max(8*s, min(mapH-8*s, y))
	anchor := ui.Box(c.Key(key+"-anchor")).Absolute().Left(x).Top(y).Size(1, 1).PassThrough()
	ui.PopoverBase(c.Key(key), anchor, &v.pointOpen, func(p ui.Element) {
		popover(p.Label(name).Lang(a.lang()).AttachTo(anchor, ui.AnchorTopStart, ui.AnchorBottomStart).Margin(0, 0, 8*s, 16*s).Width(width).Padding(12*s), radiusCard*s, s)
		ui.Column(c).FillWidth().Gap(8 * s).Children(children)
	})
}
