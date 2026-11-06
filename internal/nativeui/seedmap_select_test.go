package nativeui

import (
	"math"
	"os"
	"testing"
	"time"

	"github.com/Viking602/YoloMCL/internal/launcher"
	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// TestSeedMapSelectFlyShots dives into the live ancient city, picks a
// block under the shown frame, flies to it, then shows the piece boxes.
func TestSeedMapSelectFlyShots(t *testing.T) {
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
		t.Fatalf("ancient city: terrain=%v err=%q", p.terrain, p.err)
	}
	settle := func() {
		for i := 0; v.structurePreview == p && (p.rendering || p.dirty); i++ {
			if i > 1000 {
				t.Fatal("render did not settle")
			}
			tt.Frame()
			time.Sleep(10 * time.Millisecond)
		}
		tt.Frame()
	}
	worst := 0.0
	for i := 0; i < 14 && p.diving; i++ {
		tt.Frame()
		settle()
		worst = math.Max(worst, p.frameMs)
		if i == 4 {
			snapshotUI(t, tt, "seedmap-dive-ground-mid")
		}
		tt.Advance(diveDuration / 10)
	}
	settle()
	t.Logf("dive worst frame %.1f ms, faces %d", worst, p.mesh.Faces())
	snapshotUI(t, tt, "seedmap-dive-ground-arrived")

	shown := &p.frames[1-p.back]
	fw, fh := shown.Image.Rect.Dx(), shown.Image.Rect.Dy()
	var pk structure3d.Picked
	found := false
	for r := 0; r < fw/2 && !found; r += 3 {
		for _, d := range [][2]int{{r, 0}, {-r, 0}, {0, r}, {0, -r}} {
			if q, ok := p.mesh.PickAt(shown, fw/2+d[0], fh*55/100+d[1]); ok {
				pk, found = q, true
				break
			}
		}
	}
	if !found {
		t.Fatal("no structure block under the shown frame")
	}
	target := [3]float64{float64(pk.X) + .5, float64(pk.Y) + .5, float64(pk.Z) + .5}
	p.sel = pk
	p.fly = &seedMapFly{from: structure3d.Camera{Aim: p.curAim(), PanX: p.panX, PanY: p.panY, Zoom: p.zoom}, to: structure3d.Camera{Aim: target, Zoom: p.zoom}}
	for i := 0; i < 10 && p.fly != nil; i++ {
		tt.Frame()
		tt.Advance(flyDuration / 6)
	}
	settle()
	if p.fly != nil || p.aim != target {
		t.Fatalf("fly did not land: fly=%v aim=%v want %v", p.fly != nil, p.aim, target)
	}
	sx, sy, _ := p.mesh.Project(p.camera(), structurePreviewFullW, structurePreviewFullH, target[0], target[1], target[2])
	if math.Abs(sx-structurePreviewFullW/2) > 1 || math.Abs(sy-structurePreviewFullH*0.55) > 1 {
		t.Fatalf("selected block at (%.1f,%.1f), want the orbit centre", sx, sy)
	}
	t.Logf("selected %s at %d,%d,%d", pk.Name, pk.X, pk.Y, pk.Z)
	snapshotUI(t, tt, "seedmap-select-ancient-city")
	p.boxes = true
	a.rerenderStructurePreview(v)
	settle()
	snapshotUI(t, tt, "seedmap-boxes-ancient-city")
}
