package nativeui

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestSeedMapSurfaceInteraction(t *testing.T) {
	a, tt, _ := worldsTesterVersion(t, "26.3")
	clickUI(t, tt, a.L("instance.tab_prefix")+" "+a.L("seedmap.open"))
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	v := a.seedMap
	snapshot, err := launcher.WorldgenIngest(context.Background(), launcher.WorldgenInput{Minecraft: "26.3"})
	if err != nil {
		t.Fatal(err)
	}
	v.snapshot, v.seedText = snapshot, "-557181663597411692"
	v.viewport.BlocksPerPixel = 4
	v.center(0, 0)
	tt.SetSize(1536, 1024)
	a.loadSeedMap(v, false)
	waitUI(t, tt, func() bool { return !v.loading && !v.pending && v.mapReady() && v.frame.Viewport == v.viewport })
	if !v.viewport.Surface || !tt.HasText(a.L("seedmap.surface_active")) {
		t.Fatal("surface mode was not enabled")
	}
	tt.Advance(400 * time.Millisecond)
	snapshotUI(t, tt, "seedmap-surface-overview")
	v.hideMarkers = true
	v.center(-100, 240)
	v.viewport.BlocksPerPixel, v.pixelsPerBlock = 1, 4
	tt.Frame()
	waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport == v.viewport })
	tt.Advance(200 * time.Millisecond)
	stage, ok := tt.Find(a.L("seedmap.canvas"))
	if !ok {
		t.Fatal("canvas")
	}
	x, z := stage.X+stage.W*.45, stage.Y+stage.H*.55
	tt.ClickAt(x, z)
	tt.Frame()
	if v.inspected == nil || !v.inspected.Surface || v.inspected.Biome.ID == "" {
		t.Fatal("surface point was not selected")
	}
	p := *v.inspected
	clickUI(t, tt, a.L("seedmap.copy_tp"))
	if got, want := tt.Clipboard(), fmt.Sprintf("/tp %d ~ %d", p.X, p.Z); got != want {
		t.Fatalf("clipboard: %q want %q", got, want)
	}
	snapshotUI(t, tt, "seedmap-surface-detail")
	// Larger material controls must fit translated point cards at narrow sizes.
	for _, lang := range []string{LangEN, LangZH} {
		a.OverrideLanguage(lang)
		for _, size := range [][2]int{{900, 700}, {1536, 1024}} {
			tt.SetSize(size[0], size[1])
			waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport == v.viewport })
			tt.Advance(200 * time.Millisecond)
			canvas := addonFilterBounds(t, tt, a.L("seedmap.canvas"))
			tt.ClickAt(canvas.X+canvas.W*.45, canvas.Y+canvas.H*.55)
			tt.Frame()
			card := addonFilterBounds(t, tt, a.L("floating.map_point_popup"))
			for _, key := range []string{"seedmap.apply", "seedmap.y_apply", "seedmap.jump", "seedmap.zoom_in"} {
				control := addonFilterBounds(t, tt, a.L(key))
				if control.X+control.W > float32(size[0]) {
					t.Fatalf("%s toolbar control escaped window: %s %+v", lang, key, control)
				}
			}
			for _, key := range []string{"seedmap.copy_tp", "seedmap.biome_wiki", "seedmap.point_close"} {
				control := addonFilterBounds(t, tt, a.L(key))
				if control.X < card.X || control.X+control.W > card.X+card.W+.5 || control.Y+control.H > card.Y+card.H+.5 {
					t.Fatalf("%s action escaped point card: %+v in %+v", lang, control, card)
				}
			}
			if card.X < canvas.X || card.X+card.W > canvas.X+canvas.W+.5 || card.Y+card.H > canvas.Y+canvas.H+.5 {
				t.Fatalf("point card escaped map: %+v in %+v", card, canvas)
			}
			snapshotUI(t, tt, fmt.Sprintf("seedmap-card-%s-%d", lang, size[0]))
		}
	}
	clickUI(t, tt, a.L("seedmap.point_close"))
	if v.inspected != nil {
		t.Fatal("popup not dismissed")
	}
	clickUI(t, tt, a.L("seedmap.y_apply"))
	waitUI(t, tt, func() bool { return !v.pending && !v.frame.Viewport.Surface })
	if v.viewport.Surface || !v.sliceMode {
		t.Fatal("Y slice still samples surface")
	}
	clickUI(t, tt, a.L("seedmap.surface_show"))
	waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport.Surface })
	a.seedMapDimension(v, "minecraft:the_nether")
	tt.Frame()
	waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport == v.viewport })
	if v.viewport.Surface || len(v.frame.Biomes) == 0 {
		t.Fatal("unsupported relief removed the biome map")
	}
	a.seedMapDimension(v, "minecraft:overworld")
	tt.Frame()
	waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport == v.viewport })
	if !v.viewport.Surface {
		t.Fatal("surface preference lost across dimension switch")
	}
}
