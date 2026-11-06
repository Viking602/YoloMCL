package nativeui

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// TestSeedMapDiveShots plays the dive into a woodland mansion on the test
// clock, snapshotting each step (seedmap-dive-NN), then the X-ray view and
// the dive back out. It logs render times of the dive frames.
func TestSeedMapDiveShots(t *testing.T) {
	const jar = `D:\MC\.minecraft\versions\1.21.1\1.21.1.jar`
	if _, err := os.Stat(jar); err != nil {
		t.Skip("live 1.21.1 game jar required")
	}
	a, tt, saves := worldsTester(t)
	copyLiveJar(t, jar, saves)
	a.OverrideLanguage(LangZH)
	tab := a.L("instance.tab_prefix") + " " + a.L("seedmap.open")
	waitUI(t, tt, func() bool { _, ok := tt.Find(tab); return ok })
	clickUI(t, tt, tab)
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	v := a.seedMap
	tt.SetSize(1440, 900)
	tt.Frame()
	// A mansion sits on land; the fixture seed's sea floor would make a pit.
	hx, hz := 1240, -856
	for r := 0; r < 4096; r += 64 {
		if h, ok := v.sampler.SurfaceHeightAt(v.viewport.Dimension, 1240+r, -856); ok && h >= 66 && h <= 90 {
			hx = 1240 + r
			break
		}
	}
	v.selected = &launcher.FeatureHit{ID: "minecraft:mansion", X: hx, Z: hz, Confidence: "biome_candidate"}
	tt.Frame()
	clickUI(t, tt, a.L("seedmap.structure_preview"))
	waitUI(t, tt, func() bool { p := v.structurePreview; return p != nil && !p.loading })
	p := v.structurePreview
	if !p.terrain || !p.mesh.HasTerrain {
		t.Fatalf("no terrain slab or dive: terrain=%v diving=%v err=%q", p.terrain, p.diving, p.err)
	}
	// The opening dive can finish while the slab loads. Replay it to test
	// every cut height on the completed terrain as well.
	p.startDive(false)
	var times []float64
	settle := func() {
		for i := 0; v.structurePreview == p && (p.rendering || p.dirty); i++ {
			if i > 500 {
				t.Fatalf("render did not settle: rendering=%v dirty=%v diving=%v t=%.2f mesh=%v", p.rendering, p.dirty, p.diving, p.diveT, p.mesh != nil)
			}
			tt.Frame()
			time.Sleep(10 * time.Millisecond)
		}
		tt.Frame()
	}
	for i := 0; i <= 16 && v.structurePreview != nil; i++ {
		tt.Frame()
		settle()
		times = append(times, p.frameMs)
		snapshotUI(t, tt, fmt.Sprintf("seedmap-dive-%02d", i))
		tt.Advance(diveDuration / 15)
	}
	if p.diving || p.diveT != 1 {
		t.Fatalf("dive did not finish: t=%.2f", p.diveT)
	}
	settle()
	if !tt.HasText(fmt.Sprintf(a.L("seedmap.gauge.cut"), int(p.finalCutY()))) {
		t.Fatalf("gauge needle label missing: %v", tt.Texts())
	}
	snapshotUI(t, tt, "seedmap-dive-arrived")
	clickUI(t, tt, a.L("seedmap.structure_preview.xray"))
	tt.Frame()
	settle()
	snapshotUI(t, tt, "seedmap-dive-xray")
	clickUI(t, tt, a.L("seedmap.structure_preview.close"))
	for i := 0; i < 12 && v.structurePreview != nil; i++ {
		tt.Frame()
		settle()
		if i == 4 {
			snapshotUI(t, tt, "seedmap-dive-out")
		}
		tt.Advance(diveOutDuration / 8)
	}
	tt.Frame()
	if v.structurePreview != nil {
		t.Fatal("reverse dive did not close the preview")
	}
	sum, worst := 0.0, 0.0
	for _, ms := range times {
		sum += ms
		worst = max(worst, ms)
	}
	t.Logf("dive frames %d (480x360): mean %.1f ms, worst %.1f ms; slab top %d, structure Y %d..%d, surface ≈%d",
		len(times), sum/float64(len(times)), worst, p.terrainTop, p.structMinY, p.structMaxY, p.surfaceY)
}

// TestSeedMapAncientCityDiveShot dives to a buried jigsaw structure: once
// arrived, the terrain is cut down to the city's floor so none of it covers
// the city (seedmap-dive-ancient-city).
func TestSeedMapAncientCityDiveShot(t *testing.T) {
	const jar = `D:\MC\.minecraft\versions\1.21.1\1.21.1.jar`
	if _, err := os.Stat(jar); err != nil {
		t.Skip("live 1.21.1 game jar required")
	}
	a, tt, saves := worldsTester(t)
	copyLiveJar(t, jar, saves)
	a.OverrideLanguage(LangZH)
	tab := a.L("instance.tab_prefix") + " " + a.L("seedmap.open")
	waitUI(t, tt, func() bool { _, ok := tt.Find(tab); return ok })
	clickUI(t, tt, tab)
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	v := a.seedMap
	tt.SetSize(1440, 900)
	tt.Frame()
	v.selected = &launcher.FeatureHit{ID: "minecraft:ancient_city", X: 1240, Z: -856, Confidence: "biome_candidate"}
	tt.Frame()
	clickUI(t, tt, a.L("seedmap.structure_preview"))
	waitUI(t, tt, func() bool { p := v.structurePreview; return p != nil && !p.loading })
	p := v.structurePreview
	if !p.terrain || p.err != "" {
		t.Fatalf("ancient city: terrain=%v err=%q unsupported=%q", p.terrain, p.err, p.unsupported)
	}
	for i := 0; i < 14 && p.diving; i++ {
		tt.Frame()
		tt.Advance(diveDuration / 10)
	}
	tt.Frame()
	for i := 0; v.structurePreview == p && (p.rendering || p.dirty); i++ {
		if i > 500 {
			t.Fatal("render did not settle")
		}
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	tt.Frame()
	if p.diving || p.cutY() != float64(p.structMinY) {
		t.Fatalf("dive did not end at the city floor: diving=%v cut=%.0f floor=%d", p.diving, p.cutY(), p.structMinY)
	}
	snapshotUI(t, tt, "seedmap-dive-ancient-city")
	t.Logf("ancient city Y %d..%d, slab %d, surface ≈%d", p.structMinY, p.structMaxY, previewSlabSize(p.mesh), p.surfaceY)
}
