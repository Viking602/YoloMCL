package nativeui

import (
	"math"
	"os"
	"testing"
	"time"
)

// TestSeedMapHeightDimensionShots renders the 2D map's height display for the
// Overworld, Nether and End on the live 1.21.1 jar: the Y-slice map, the
// relief (Surface) view, the terrain toggle and a hover with ground Y.
func TestSeedMapHeightDimensionShots(t *testing.T) {
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
	settle := func(dim string) time.Duration {
		start := time.Now()
		tt.Frame()
		waitUI(t, tt, func() bool {
			tt.Frame()
			if v.pending || v.frame.Viewport.Dimension != dim || v.frame.Viewport.Surface != v.viewport.Surface || v.frame.Viewport.Terrain != v.viewport.Terrain {
				return false
			}
			// every corner and the centre painted
			w, h := v.frame.Viewport.Width, v.frame.Viewport.Height
			for _, q := range [][2]int{{1, 1}, {w - 2, 1}, {1, h - 2}, {w - 2, h - 2}, {w / 2, h / 2}} {
				if _, ok := v.frame.PointAt(q[0], q[1]); !ok {
					return false
				}
			}
			return true
		})
		for range 20 {
			tt.Advance(100 * time.Millisecond) // let tile fade-ins finish
			tt.Frame()
			time.Sleep(15 * time.Millisecond)
		}
		tt.Frame()
		return time.Since(start)
	}
	for _, c := range []struct{ name, dim string }{
		{"overworld", "minecraft:overworld"},
		{"nether", "minecraft:the_nether"},
		{"end", "minecraft:the_end"},
	} {
		if v.viewport.Dimension != c.dim {
			a.seedMapDimension(v, c.dim)
		}
		v.viewport.Surface, v.sliceMode, v.viewport.Terrain = false, true, false
		settle(c.dim)
		snapshotUI(t, tt, "map-"+c.name+"-slice")
		if !v.sampler.SupportsSurface(c.dim) {
			t.Errorf("%s: no height display", c.name)
			continue
		}
		v.viewport.Surface, v.sliceMode = true, false
		took := settle(c.dim)
		_, surfaceButton := tt.Find(a.L("seedmap.surface_active"))
		px, pz := v.frame.Viewport.Width/2+17, v.frame.Viewport.Height/2+9
		p, ok := v.frame.PointAt(px, pz)
		if ok {
			v.hover = &p
		}
		tt.Frame()
		t.Logf("%s: relief frame %v, surface button %v, hover %+v", c.name, took.Round(time.Millisecond), surfaceButton, p)
		snapshotUI(t, tt, "map-"+c.name+"-relief")
		v.viewport.Terrain = true
		settle(c.dim)
		snapshotUI(t, tt, "map-"+c.name+"-terrain")
		v.viewport.Terrain, v.hover = false, nil
		samples := 0
		for x := -2000; x <= 2000; x += 500 {
			h, ok := v.sampler.SurfaceHeightAt(c.dim, x, 300)
			if ok {
				samples++
			}
			t.Logf("%s: ground at %d,300 = %d (%v)", c.name, x, h, ok)
			if c.name == "nether" && ok && (h < 0 || h > 127) {
				t.Errorf("nether floor %d out of range", h)
			}
		}
		switch c.name {
		case "nether":
			if samples == 0 {
				t.Error("nether: no floor found")
			}
		case "end":
			if h, ok := v.sampler.SurfaceHeightAt(c.dim, 0, 0); !ok || h < 40 || h > 100 {
				t.Errorf("end main island at 0,0 = %d %v", h, ok)
			}
			if _, ok := v.sampler.SurfaceHeightAt(c.dim, 400, 400); ok {
				t.Error("end: 400,400 should be void")
			}
			// hover over the void says so
			for x := 0; x < v.frame.Viewport.Width; x += 7 {
				if p, ok := v.frame.PointAt(x, 3); ok && p.Void {
					t.Logf("end: void pixel at %d,%d", p.X, p.Z)
					break
				}
			}
		}
		_ = math.NaN
	}
}
