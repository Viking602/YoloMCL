package structure3d

import (
	"image"
	"image/png"
	"os"
	"testing"
	"time"
)

const liveJar = `D:\MC\.minecraft\versions\1.21.1\1.21.1.jar`

func liveStore(t *testing.T) *Store {
	t.Helper()
	if _, err := os.Stat(liveJar); err != nil {
		t.Skip("live game jar required")
	}
	s := testStore(t)
	if err := s.OpenJar("game", liveJar); err != nil {
		t.Fatal(err)
	}
	return s
}

func diffPixels(a, b []uint8) int {
	n := 0
	for i := 0; i+3 < len(a); i += 4 {
		if a[i] != b[i] || a[i+1] != b[i+1] || a[i+2] != b[i+2] {
			n++
		}
	}
	return n
}

// TestMeshMatchesReference checks the baked mesh draws the same frame as the
// reference per-block renderer, and reports per-frame timings.
func TestMeshMatchesReference(t *testing.T) {
	s := liveStore(t)
	for _, tc := range []struct {
		id   string
		seed int64
	}{{"minecraft:village_plains", 42}, {"minecraft:bastion_remnant", 7}} {
		scene, err := Assemble(s, tc.id, tc.seed, 0, 0, func(x, z int) int { return 64 })
		if err != nil {
			t.Fatal(err)
		}
		cams := []Camera{DefaultCamera(), {Yaw: 2.4, Pitch: 0.9, Zoom: 10}, {Yaw: -1.1, Pitch: 0.3, Zoom: 14}, {Yaw: 0.6, Pitch: -0.8, Zoom: 12}, {Yaw: 3.0, Pitch: -1.5, Zoom: 10}}
		const w, h = 720, 540
		start := time.Now()
		ref := make([][]uint8, len(cams))
		for i, cam := range cams {
			ref[i] = NewRenderer(s, w, h).Render(scene, cam).Pix
		}
		oldMs := float64(time.Since(start).Microseconds()) / 1000 / float64(len(cams))
		start = time.Now()
		mesh := BuildMesh(s, scene)
		buildMs := float64(time.Since(start).Microseconds()) / 1000
		var f Frame
		worst := 0
		for i, cam := range cams {
			if d := diffPixels(ref[i], mesh.Render(&f, w, h, cam).Pix); d > worst {
				worst = d
			}
		}
		const n = 20
		start = time.Now()
		for i := 0; i < n; i++ {
			mesh.Render(&f, w, h, Camera{Yaw: float64(i) * 0.1, Pitch: 0.55, Zoom: 12})
		}
		newMs := float64(time.Since(start).Microseconds()) / 1000 / n
		start = time.Now()
		for i := 0; i < n; i++ {
			mesh.Render(&f, w/2, h/2, Camera{Yaw: float64(i) * 0.1, Pitch: 0.55, Zoom: 6})
		}
		lowMs := float64(time.Since(start).Microseconds()) / 1000 / n
		if os.Getenv("MCL_MESH_DUMP") != "" {
			dumpPNG(t, "testdata/"+tc.id[10:]+"_ref.png", NewRenderer(s, w, h).Render(scene, cams[0]))
			dumpPNG(t, "testdata/"+tc.id[10:]+"_mesh.png", mesh.Render(&Frame{}, w, h, cams[0]))
		}
		t.Logf("%s: %d blocks, %d faces (%d culled); reference %.1f ms/frame; mesh build %.1f ms once, %.1f ms/frame full, %.1f ms/frame half-res; max differing pixels %d/%d",
			tc.id, mesh.Blocks, mesh.Faces(), mesh.Culled, oldMs, buildMs, newMs, lowMs, worst, w*h)
		if worst > w*h*3/100 { // float32 edge rounding only
			t.Fatalf("%s: mesh differs from reference in %d pixels", tc.id, worst)
		}
	}
}
func dumpPNG(t *testing.T, path string, img *image.RGBA) {
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_ = png.Encode(f, img)
}
