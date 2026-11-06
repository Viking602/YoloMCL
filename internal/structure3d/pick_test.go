package structure3d

import "testing"

// A block drawn by Render is found again under the pixel Project gives for
// its centre, and the cut opens the ground's Section colour.
func TestPickAtProjectAndSection(t *testing.T) {
	m := &Mesh{Center: [3]float64{0.5, 70.5, 0.5}, MinY: 70, MaxY: 70}
	m.blocks = []WorldBlock{{X: 0, Y: 70, Z: 0, State: BlockState{Name: "minecraft:stone"}}}
	red := m.solid([3]uint8{200, 20, 20})
	// top face of the block at y=71, relative to the centre
	m.quads = append(m.quads, meshQuad{
		p: [4][3]float32{{-.5, .5, -.5}, {.5, .5, -.5}, {.5, .5, .5}, {-.5, .5, .5}},
		n: [3]float32{0, 1, 0}, tex: red, shade: 1, top: .5, block: 1,
	})
	m.AddTerrain(Terrain{X0: -20, Z0: -20, W: 40, D: 40, Step: 2, Bottom: 40,
		Height:  func(x, z int) int { return 80 },
		Section: func(x, y, z int) [3]uint8 { return [3]uint8{10, 200, 10} },
		YStep:   2})
	cam := Camera{Yaw: 0.6, Pitch: -0.9, Zoom: 8, Cut: true, CutY: 70, Ghost: 0.16}
	var f Frame
	img := m.Render(&f, 320, 240, cam)
	sx, sy, ok := m.Project(cam, 320, 240, 0.5, 71, 0.5)
	if !ok {
		t.Fatal("block centre behind the eye")
	}
	pk, ok := m.PickAt(&f, int(sx), int(sy))
	if !ok || pk.Index != 1 || pk.Y != 70 || pk.Name != "minecraft:stone" {
		t.Fatalf("pick at (%.0f,%.0f) = %+v, %v", sx, sy, pk, ok)
	}
	// a cap of the cut ground away from the block shows the section green
	gx, gy, _ := m.Project(cam, 320, 240, 12, 70, 12)
	o := img.PixOffset(int(gx), int(gy))
	if r, g := img.Pix[o], img.Pix[o+1]; g < 100 || r > 80 {
		t.Fatalf("cut cap at (%.0f,%.0f) = %v, want section green under a faint ghost", gx, gy, img.Pix[o:o+4])
	}
	if _, ok := m.PickAt(&f, int(gx), int(gy)); ok {
		t.Fatal("terrain must not be pickable")
	}
}
