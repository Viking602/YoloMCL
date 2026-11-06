package nativeui

import (
	"os"
	"testing"
	"time"
)

// TestSeedMapTerrainAndMarkerShots covers the world-map terrain view and the
// zoomed-out marker declutter. MCL_SEEDMAP_SHOT_TAG names the marker shot so
// a before/after pair can be captured across a change.
func TestSeedMapTerrainAndMarkerShots(t *testing.T) {
	a, tt, _ := worldsTester(t)
	a.OverrideLanguage(LangEN)
	tab := a.L("instance.tab_prefix") + " " + a.L("seedmap.open")
	waitUI(t, tt, func() bool { _, ok := tt.Find(tab); return ok })
	clickUI(t, tt, tab)
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	v := a.seedMap
	tt.SetSize(1440, 900)
	tt.Frame()
	settle := func() {
		tt.Frame()
		waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport == v.viewport && !v.frame.Partial })
		tt.Frame()
		tt.Advance(600 * time.Millisecond)
		tt.Frame()
	}
	if !v.viewport.Surface {
		clickUI(t, tt, a.L("seedmap.surface_show"))
		settle()
	}
	if !v.viewport.Surface {
		t.Fatal("surface view did not open")
	}
	snapshotUI(t, tt, "seedmap-xaero-before-biome")
	clickUI(t, tt, a.L("seedmap.terrain.toggle"))
	settle()
	if !v.viewport.Terrain || !tt.HasText(a.L("seedmap.terrain.note")) {
		t.Fatalf("terrain view not shown: %v", tt.Texts())
	}
	snapshotUI(t, tt, "seedmap-xaero-after-terrain")
	clickUI(t, tt, a.L("seedmap.zoom_in"))
	settle()
	clickUI(t, tt, a.L("seedmap.zoom_in"))
	settle()
	snapshotUI(t, tt, "seedmap-xaero-after-terrain-zoom")
	a.OverrideLanguage(LangZH)
	tt.Frame()
	tt.Frame()
	settle()
	snapshotUI(t, tt, "seedmap-xaero-after-terrain-zh")
	a.OverrideLanguage(LangEN)
	tt.Frame()
	clickUI(t, tt, a.L("seedmap.terrain.toggle"))
	settle()
	if v.viewport.Terrain {
		t.Fatal("terrain view did not turn off")
	}
	clickUI(t, tt, a.L("seedmap.surface_active"))
	settle()
	for i := 0; i < 12; i++ {
		before := v.viewport.BlocksPerPixel*16 + 16/max(1, v.pixelsPerBlock)
		clickUI(t, tt, a.L("seedmap.zoom_out"))
		settle()
		if v.viewport.BlocksPerPixel*16+16/max(1, v.pixelsPerBlock) == before {
			break
		}
	}
	tag := os.Getenv("MCL_SEEDMAP_SHOT_TAG")
	if tag == "" {
		tag = "after"
	}
	snapshotUI(t, tt, "seedmap-markers-"+tag+"-min-zoom")
	t.Logf("min zoom %d blocks/pixel, %d markers, %d hits", v.viewport.BlocksPerPixel, len(v.markerLayout.markers), len(v.hits))
}
