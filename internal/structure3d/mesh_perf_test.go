package structure3d

import (
	"bytes"
	"sync"
	"testing"
)

func TestPerspectiveCullMatchesUnculled(t *testing.T) {
	s := testStore(t)
	m := BuildMesh(s, BuildScene([]WorldBlock{
		{X: 0, Y: 64, Z: 0, State: BlockState{Name: "minecraft:snow_block"}},
		{X: -2, Y: 66, Z: 1, State: BlockState{Name: "minecraft:snow_block"}},
		{X: 3, Y: 65, Z: -1, State: BlockState{Name: "minecraft:snow_block"}},
	}))
	if m.Faces() == 0 {
		t.Fatal("empty mesh")
	}
	ref := m.Fork()
	ref.quads = append([]meshQuad(nil), m.quads...)
	for i := range ref.quads {
		ref.quads[i].flat = false
	}
	var a, b Frame
	for _, pitch := range []float64{-1.4, -.55, .55, 1.4} {
		for _, yaw := range []float64{-2.4, -.6, .6, 2.4} {
			cam := Camera{Yaw: yaw, Pitch: pitch, Zoom: 24, Persp: 20, Aimed: true, Aim: [3]float64{2, 65, 1}, Select: 1}
			m.Render(&a, 320, 240, cam)
			ref.Render(&b, 320, 240, cam)
			if n := diffPixels(a.Image.Pix, b.Image.Pix); n > 4 {
				t.Fatalf("culling changed %d pixels at yaw=%v pitch=%v", n, yaw, pitch)
			}
		}
	}
}

func TestFrameCachesSectionAndGhost(t *testing.T) {
	queries := 0
	m := &Mesh{Center: [3]float64{0, 4, 0}}
	m.AddTerrain(Terrain{X0: -12, Z0: -12, W: 24, D: 24, Step: 2, Bottom: 0,
		Height: func(_, _ int) int { return 8 },
		Section: func(_, y, _ int) [3]uint8 {
			queries++
			return [3]uint8{uint8(y * 20), 100, 80}
		}})
	cam := Camera{Yaw: .6, Pitch: -.8, Zoom: 3, Persp: 100, Cut: true, CutY: 5, Ghost: .16}
	var f Frame
	m.Render(&f, 200, 160, cam)
	q := queries
	cam.Yaw = .7
	m.Render(&f, 200, 160, cam)
	if queries != q {
		t.Fatal("orbit recomputed an unchanged section")
	}
	if allocs := testing.AllocsPerRun(20, func() { m.Render(&f, 200, 160, cam) }); allocs != 0 {
		t.Fatalf("steady orbit allocated %.1f objects per frame", allocs)
	}
	for _, cut := range []float64{4, 4.8, 3, 5} {
		cam.CutY = cut
		var fresh Frame
		want := m.Render(&fresh, 200, 160, cam)
		got := m.Render(&f, 200, 160, cam)
		if !bytes.Equal(got.Pix, want.Pix) {
			t.Fatalf("stale section texture at cut %v", cut)
		}
	}
	// Reusing a frame for another mesh at the same Y must also invalidate it.
	other := m.Fork()
	other.section = func(_, _, _ int) [3]uint8 { return [3]uint8{200, 0, 0} }
	var fresh Frame
	if !bytes.Equal(other.Render(&f, 200, 160, cam).Pix, other.Render(&fresh, 200, 160, cam).Pix) {
		t.Fatal("section cache leaked across meshes")
	}
}

func TestMeshForkCanBuildWhileRendering(t *testing.T) {
	m := BuildMesh(testStore(t), BuildScene([]WorldBlock{{State: BlockState{Name: "minecraft:snow_block"}}}))
	n := m.Faces()
	fork := m.Fork()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var f Frame
		for i := 0; i < 20; i++ {
			m.Render(&f, 160, 120, Camera{Yaw: float64(i) / 10, Pitch: -.6, Zoom: 12})
		}
	}()
	fork.AddTerrain(Terrain{X0: -20, Z0: -20, W: 40, D: 40, Step: 1, Bottom: -10, Height: func(_, _ int) int { return 0 }})
	wg.Wait()
	if m.HasTerrain || m.Faces() != n || !fork.HasTerrain || fork.Faces() <= n {
		t.Fatal("terrain build mutated the displayed mesh")
	}
}
