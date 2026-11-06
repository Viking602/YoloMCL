package nativeui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestSeedMapBlockZoomAndHeight(t *testing.T) {
	a, tt, _ := worldsTester(t)
	clickUI(t, tt, a.L("instance.tab_prefix")+" "+a.L("seedmap.open"))
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	v := a.seedMap
	if v.viewport.BlocksPerPixel != 1 || !tt.HasText(a.L("seedmap.block_detail")) {
		t.Fatal("map did not open at block detail")
	}
	before := v.viewport
	clickUI(t, tt, a.L("seedmap.y"))
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("65")
	clickUI(t, tt, a.L("seedmap.y_apply"))
	waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport.Y == 65 })
	for _, want := range []int{2, 4, 8} {
		clickUI(t, tt, a.L("seedmap.zoom_in"))
		waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport == v.viewport })
		if v.pixelsPerBlock != want || v.viewport.BlocksPerPixel != 1 || v.viewport.X != before.X || v.viewport.Z != before.Z {
			t.Fatal("magnification changed block precision or map center")
		}
	}
	if v.viewport.Width > before.Width/7 || v.raster.textures == nil || v.yText != "65" {
		t.Fatal("close zoom did not retain display tiles at exact block height")
	}
	for range 3 {
		clickUI(t, tt, a.L("seedmap.zoom_out"))
	}
	waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport == v.viewport })
	if v.viewport.Width != before.Width || v.viewport.Height != before.Height || v.viewport.BlocksPerPixel != 1 {
		t.Fatal("inverse zoom did not restore the visible block range")
	}
	tt.SetSize(1920, 1080)
	tt.Frame()
	waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport == v.viewport })
	stage, ok := tt.Find(a.L("seedmap.canvas"))
	if !ok || v.viewport.Width <= 768 || v.viewport.Width > launcher.SeedMapMaxWidth || v.viewport.Height > launcher.SeedMapMaxHeight || float32(v.viewport.Width) < stage.W-1 {
		t.Fatal("large canvas still stretches a low-resolution raster")
	}
}
