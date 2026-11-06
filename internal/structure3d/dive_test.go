package structure3d

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTerrainSlabLooksSolidFromBelow(t *testing.T) {
	for _, withSection := range []bool{false, true} {
		m := &Mesh{Center: [3]float64{0, 48, 0}}
		terrain := Terrain{X0: -8, Z0: -8, W: 16, D: 16, Step: 2, Bottom: 32,
			Height: func(_, _ int) int { return 64 },
			Color:  func(_, _, _ int) [3]uint8 { return [3]uint8{40, 200, 40} },
		}
		if withSection {
			terrain.Section = func(_, _, _ int) [3]uint8 { return [3]uint8{125, 125, 125} }
		}
		m.AddTerrain(terrain)
		for _, above := range []bool{false, true} {
			pitch := math.Pi / 2
			if above {
				pitch = -pitch
			}
			var frame Frame
			img := m.Render(&frame, 160, 160, Camera{Pitch: pitch, Zoom: 8})
			c := img.RGBAAt(80, 88)
			if above && int(c.G) < 2*int(c.R) {
				t.Errorf("surface lost its grass: %+v", c)
			}
			if !above && (c.R != c.G || c.G != c.B) {
				t.Errorf("section=%t: open underside shows the grass through the slab: %+v", withSection, c)
			}
			if dir := os.Getenv("DIVE_PNG"); dir != "" {
				name := "terrain-below"
				if above {
					name = "terrain-above"
				}
				if withSection {
					name += "-section"
				}
				f, err := os.Create(filepath.Join(dir, name+".png"))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(f, img)
				_ = f.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

// TestDiveFrameTimes renders the dive over a 128-block terrain slab at
// half-ish resolution and logs per-frame render times.
func TestDiveFrameTimes(t *testing.T) {
	m := &Mesh{Center: [3]float64{0, 80, 0}, MinY: 70, MaxY: 100}
	m.AddTerrain(Terrain{X0: -64, Z0: -64, W: 128, D: 128, Step: 2, Bottom: 30,
		Height: func(x, z int) int { return 70 + int(8*math.Sin(float64(x)/9)+6*math.Cos(float64(z)/13)) }})
	final := Camera{Yaw: 0.6, Pitch: 0.55, Zoom: 8, Persp: 220, Cut: true, CutY: 101}
	var f Frame
	worst := time.Duration(0)
	for i := 0; i <= 15; i++ {
		cam := DiveCamera(float64(i)/15, final, float64(m.TerrainTop), 101)
		start := time.Now()
		m.Render(&f, 480, 360, cam)
		d := time.Since(start)
		worst = max(worst, d)
		t.Logf("t=%.2f pitch=%.2f persp=%.0f cut=%.0f: %v", float64(i)/15, cam.Pitch, cam.Persp, cam.CutY, d)
	}
	t.Logf("faces %d, worst %v", m.Faces(), worst)
	if dir := os.Getenv("DIVE_PNG"); dir != "" {
		for name, cam := range map[string]Camera{"ortho": {Yaw: 0.6, Pitch: -0.55, Zoom: 3}, "persp": {Yaw: 0.6, Pitch: -0.55, Zoom: 3, Persp: 300}} {
			var g Frame
			img := m.Render(&g, 480, 360, cam)
			out, _ := os.Create(filepath.Join(dir, "dive-"+name+".png"))
			_ = png.Encode(out, img)
			_ = out.Close()
		}
	}
}
