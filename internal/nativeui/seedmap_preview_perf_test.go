package nativeui

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// Opt-in, real-asset timing harness: MCL_PREVIEW_PERF=1 go test ...
// Use the same seed, coordinates and cameras when comparing CPU profiles.
func TestSeedMapPreviewPerformance(t *testing.T) {
	if os.Getenv("MCL_PREVIEW_PERF") == "" {
		t.Skip("set MCL_PREVIEW_PERF to time live structure previews")
	}
	const jar = `D:\MC\.minecraft\versions\1.21.1\1.21.1.jar`
	if _, err := os.Stat(jar); err != nil {
		t.Skip("live 1.21.1 game jar required")
	}
	a, tt, saves := worldsTester(t)
	copyLiveJar(t, jar, saves)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tab := a.L("instance.tab_prefix") + " " + a.L("seedmap.open")
	waitUI(t, tt, func() bool { _, ok := tt.Find(tab); return ok })
	clickUI(t, tt, tab)
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	v := a.seedMap
	for _, tc := range []struct {
		id, dim string
		x, z    int
	}{{"minecraft:ancient_city", "minecraft:overworld", 1240, -856}, {"minecraft:bastion_remnant", "minecraft:the_nether", 0, 0}} {
		t.Run(tc.id, func(t *testing.T) {
			a.seedMapDimension(v, tc.dim)
			waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport.Dimension == tc.dim })
			hit := launcher.FeatureHit{ID: tc.id, X: tc.x, Z: tc.z}
			start := time.Now()
			b := buildStructurePreviewProgressive(context.Background(), a.service, v.rootID, v.versionID, v.world, v.seed, tc.dim, v.sampler, v.seedMapCavesSupported(), "1.21.1", nil, hit, func(first structurePreviewBuild) {
				t.Logf("first structure=%s", time.Since(start).Round(time.Millisecond))
			})
			if b.err != nil || b.res == nil || b.res.Mesh == nil {
				t.Fatalf("preview: %+v", b)
			}
			m := b.res.Mesh
			t.Logf("build=%s blocks=%d faces=%d", time.Since(start).Round(time.Millisecond), b.res.BlockCount, m.Faces())
			p := &seedMapStructurePreview{mesh: m, terrain: m.HasTerrain, dim: tc.dim, surfaceY: b.surfY, terrainTop: m.TerrainTop,
				structMinY: m.MinY, structMaxY: m.MaxY, diveT: 1, ghost: true, yaw: .6, pitch: .55, zoom: slabFitZoom(previewSlabSize(m))}
			for _, size := range [][2]int{{720, 540}, {360, 270}, {240, 180}} {
				var f structure3d.Frame
				cam := p.camera()
				cam.Zoom *= float64(size[0]) / structurePreviewFullW
				m.Render(&f, size[0], size[1], cam)
				var before, after runtime.MemStats
				runtime.ReadMemStats(&before)
				start = time.Now()
				var worst time.Duration
				const frames = 40
				for i := 0; i < frames; i++ {
					cam.Yaw = .6 + float64(i)*.035
					s := time.Now()
					m.Render(&f, size[0], size[1], cam)
					worst = max(worst, time.Since(s))
				}
				runtime.ReadMemStats(&after)
				t.Logf("%dx%d orbit: mean=%s worst=%s allocated=%d bytes/frame", size[0], size[1], time.Since(start)/frames, worst, (after.TotalAlloc-before.TotalAlloc)/frames)
			}
		})
	}
}
