package nativeui

import (
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// The bar and its legend share a highlight. Dividers are painted inside each
// segment so gaps never distort byte proportions, including during animation.
func (a *App) storageBar(c *ui.Context, st launcher.InstanceStatus, highlighted *string) {
	s := uiScale(c)
	ui.Box(c.Key("storage-bar")).Label("容量占比").FillWidth().Height(14*s).Shrink(0).Radius(radiusChip*s).Clip().
		Background(fieldColor).Margin(4*s, 0, 8*s, 0).Children(func() {
		if st.StoragePending && st.Total == 0 {
			skeleton(c, "storage-loading").Label(a.L("overview.calculating")).Fill()
			return
		}
		bar := ui.Row(c.Key("distribution")).Fill().Role(ui.RoleMeter).Label(a.L("overview_storage.distribution")).
			Range(0, float64(max(1, st.Total)), float64(max(0, st.Total)))
		if st.Total <= 0 {
			tooltip(c, bar, a.L("overview_storage.empty"))
			return
		}
		bar.Children(func() {
			for _, part := range dirParts {
				size := storageBytes(st, part.sub)
				share := float32(float64(size) / float64(st.Total))
				hint := a.L("storage.label."+part.sub) + "  " + fmtSize(size) + "  " + storagePercent(size, st.Total)
				segment := tooltip(c, ui.ButtonBase(c.Key("storage-segment-"+part.sub)).Label("存储占比 "+part.sub).
					FillHeight().Basis(0).MinWidth(0).Disabled(size == 0), hint)
				segment.Grow(segment.AnimateTo("share", share, motionPack))
				if segment.Hovered() || segment.FocusVisible() {
					*highlighted = part.sub
				}
				segment.Draw(func(p *ui.Painter, r ui.Rect) {
					tone := part.tone
					if *highlighted != "" && *highlighted != part.sub {
						tone = tone.Alpha(.45)
					}
					p.Fill(r, tone, 0)
					if r.X > bar.LayoutBounds().X+.5 && r.W > 2*s {
						p.Fill(ui.Rect{X: r.X, Y: r.Y, W: s, H: r.H}, fieldColor, 0)
					}
				})
				if size > 0 && segment.Clicked() {
					a.openStoragePart(part.sub)
				}
			}
		})
	})
}
