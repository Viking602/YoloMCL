package structure3d

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func fluidTestPNG(t *testing.T, root, name string, colors ...color.NRGBA) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2*len(colors)))
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < 2; x++ {
			img.SetNRGBA(x, y, colors[y/2])
		}
	}
	path := filepath.Join(root, "assets/minecraft/textures/block", name+".png")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func fluidTestFile(t *testing.T, root, name, value string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		t.Fatal(err)
	}
}

func fluidTestStore(t *testing.T, alpha uint8) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"water_still", "water_flow"} {
		fluidTestPNG(t, dir, name, color.NRGBA{255, 255, 255, alpha}, color.NRGBA{0, 255, 0, 255})
		fluidTestFile(t, dir, "assets/minecraft/textures/block/"+name+".png.mcmeta", `{"animation":{}}`)
	}
	for _, name := range []string{"lava_still", "lava_flow"} {
		fluidTestPNG(t, dir, name, color.NRGBA{250, 80, 10, 255}, color.NRGBA{0, 255, 0, 255})
		fluidTestFile(t, dir, "assets/minecraft/textures/block/"+name+".png.mcmeta", `{"animation":{"frames":[0,1,0]}}`)
	}
	fluidTestPNG(t, dir, "stone", color.NRGBA{200, 40, 30, 255})
	fluidTestFile(t, dir, "assets/minecraft/models/block/stone.json", `{"textures":{"all":"minecraft:block/stone"}}`)
	s := &Store{}
	if err := s.OpenDir("game", dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, dir
}

func TestFluidTexturesFramesAndPackOverrides(t *testing.T) {
	s, _ := fluidTestStore(t, 180)
	pack := t.TempDir()
	fluidTestPNG(t, pack, "water_still", color.NRGBA{0, 255, 0, 255}, color.NRGBA{255, 255, 255, 180})
	fluidTestFile(t, pack, "assets/minecraft/textures/block/water_still.png.mcmeta", `{"animation":{"frames":[{"index":1,"time":5},0]}}`)
	fluidTestPNG(t, pack, "lava_flow", color.NRGBA{230, 45, 10, 255}, color.NRGBA{0, 0, 255, 255})
	fluidTestFile(t, pack, "assets/minecraft/textures/block/lava_flow.png.mcmeta", `{"animation":{"frames":[0,1]}}`)
	if err := s.OpenDir("resource pack", pack); err != nil {
		t.Fatal(err)
	}
	tex := LoadTerrainTextures(s)
	for _, tc := range []struct {
		color [3]uint8
		top   bool
		want  color.NRGBA
	}{
		{[3]uint8{0x3f, 0x76, 0xe4}, true, color.NRGBA{63, 118, 228, 180}},
		{[3]uint8{0x3f, 0x76, 0xe4}, false, color.NRGBA{63, 118, 228, 180}},
		{[3]uint8{0xe3, 0x6b, 0x1c}, true, color.NRGBA{250, 80, 10, 255}},
		{[3]uint8{0xe3, 0x6b, 0x1c}, false, color.NRGBA{230, 45, 10, 255}},
	} {
		tx := pickTerrainTex(tex, tc.color, tc.top)
		if tx == nil || tx.w != 2 || tx.h != 2 {
			t.Fatalf("fluid atlas not cropped: %+v", tx)
		}
		got := color.NRGBA{tx.pix[0], tx.pix[1], tx.pix[2], tx.pix[3]}
		if got != tc.want {
			t.Fatalf("top=%t: got %v want %v", tc.top, got, tc.want)
		}
	}
}

func TestFluidWaterBlendsAfterSolidGeometry(t *testing.T) {
	s, _ := fluidTestStore(t, 64) // below the old alpha-test cutoff
	cam := Camera{Pitch: -math.Pi / 2, Zoom: 60, Aimed: true, Aim: [3]float64{.5, 1, .5}}
	water := WorldBlock{X: 0, Y: 1, Z: 0, State: BlockState{Name: "minecraft:water"}}
	stone := WorldBlock{X: 0, Y: 0, Z: 0, State: BlockState{Name: "minecraft:stone"}}
	want := color.RGBA{165, 59, 79, 255} // 64/255 water over the red stone
	for _, blocks := range [][]WorldBlock{{water, stone}, {stone, water}} {
		scene := &Scene{Blocks: blocks, Min: [3]int{0, 0, 0}, Max: [3]int{0, 1, 0}}
		m := BuildMesh(s, scene)
		frame := &Frame{}
		img := m.Render(frame, 120, 120, cam)
		if got := img.RGBAAt(57, 63); got != want {
			t.Fatalf("water alpha/order: got %v want %v", got, want)
		}
		// The shared diagonal must not blend the same water face twice.
		if got := img.RGBAAt(60, 66); got != want {
			t.Fatalf("water triangle seam: got %v want %v", got, want)
		}
		ref := NewRenderer(s, 120, 120).Render(scene, Camera{Pitch: -math.Pi / 2, Zoom: 60})
		if got := ref.RGBAAt(90, 45); got != want {
			t.Fatalf("reference water alpha/order: got %v want %v", got, want)
		}
		m.Render(frame, 120, 120, cam)
		if n := testing.AllocsPerRun(5, func() { m.Render(frame, 120, 120, cam) }); n != 0 {
			t.Fatalf("steady fluid rendering allocates: %g", n)
		}
	}
}

func TestFluidTerrainAndChangingCuts(t *testing.T) {
	s, _ := fluidTestStore(t, 180)
	tex := LoadTerrainTextures(s)
	m := &Mesh{}
	m.AddTerrain(Terrain{W: 1, D: 1, Step: 1, Bottom: -4, Height: func(int, int) int { return 4 },
		Color: func(int, int, int) [3]uint8 { return [3]uint8{0x3f, 0x76, 0xe4} },
		Section: func(_, y, _ int) [3]uint8 {
			if y >= 2 {
				return [3]uint8{0x3f, 0x76, 0xe4}
			}
			if y >= 0 {
				return [3]uint8{0xe3, 0x6b, 0x1c}
			}
			return [3]uint8{0x7d, 0x7d, 0x7d}
		}, Textures: tex})
	var frame Frame
	for _, tc := range []struct {
		y    float64
		want color.RGBA
	}{
		{3, color.RGBA{106, 140, 207, 255}}, // .85-shaded water (alpha 180) over the background
		{1, color.RGBA{250, 80, 10, 255}},   // emissive lava ignores the section shade
		{-1, color.RGBA{170, 34, 25, 255}},  // red stone under both fluids
		{3, color.RGBA{106, 140, 207, 255}},
	} {
		img := m.Render(&frame, 120, 120, Camera{Pitch: -math.Pi / 2, Zoom: 60, Persp: 300, Cut: true, CutY: tc.y, Aimed: true, Aim: [3]float64{.5, tc.y, .5}})
		if got := img.RGBAAt(57, 63); got != tc.want {
			t.Fatalf("fluid cut at %g: got %v want %v", tc.y, got, tc.want)
		}
	}
}

func TestFluidSharedFacesAndLavaBrightness(t *testing.T) {
	s, _ := fluidTestStore(t, 180)
	for _, kind := range []string{"water", "lava"} {
		scene := &Scene{Blocks: []WorldBlock{{State: BlockState{Name: "minecraft:" + kind}}, {X: 1, State: BlockState{Name: "minecraft:" + kind}}}, Max: [3]int{1, 0, 0}}
		m := BuildMesh(s, scene)
		if m.Faces() != 10 {
			t.Fatalf("%s internal fluid faces retained: %d", kind, m.Faces())
		}
		if kind == "lava" {
			for _, q := range m.quads {
				if !q.tex.emissive {
					t.Fatal("lava was not emissive")
				}
			}
			img := m.Render(&Frame{}, 120, 120, Camera{Yaw: math.Pi / 2, Pitch: 0, Zoom: 60, Aimed: true, Aim: [3]float64{1, .5, .5}})
			if got := img.RGBAAt(60, 66); got != (color.RGBA{250, 80, 10, 255}) {
				t.Fatalf("lava texture was shaded or lost: %v", got)
			}
		}
	}
}

func TestFluidTerrainKeepsSubmergedBlockPickable(t *testing.T) {
	s, _ := fluidTestStore(t, 180)
	m := BuildMesh(s, &Scene{Blocks: []WorldBlock{{State: BlockState{Name: "minecraft:stone"}}}})
	m.AddTerrain(Terrain{W: 1, D: 1, Step: 1, Bottom: -1, Height: func(int, int) int { return 2 },
		Color: func(int, int, int) [3]uint8 { return [3]uint8{0x3f, 0x76, 0xe4} }, Textures: LoadTerrainTextures(s)})
	frame := &Frame{}
	m.Render(frame, 120, 120, Camera{Pitch: -math.Pi / 2, Zoom: 60, Persp: 300, Aimed: true, Aim: [3]float64{.5, 1, .5}})
	if picked, ok := m.PickAt(frame, 57, 63); !ok || picked.Name != "minecraft:stone" {
		t.Fatalf("transparent terrain hid a visible block from picking: %+v %t", picked, ok)
	}
}

func TestSpriteExplicitFrameGrid(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x/2 + y/2*2), 0, 0, 255})
		}
	}
	got := firstSpriteFrame(img, []byte(`{"animation":{"width":2,"height":2,"frames":[3,0]}}`))
	if got == nil || got.Bounds() != image.Rect(0, 0, 2, 2) || color.NRGBAModel.Convert(got.At(0, 0)).(color.NRGBA).R != 3 {
		t.Fatal("wrong first scheduled atlas cell")
	}
	for _, meta := range []string{`{"animation":{"width":-1}}`, `{"animation":{"frames":[99]}}`, `{"animation":{"frames":[null]}}`, `{"animation":{"frames":[{"time":1}]}}`} {
		if firstSpriteFrame(img, []byte(meta)) != nil {
			t.Fatalf("invalid sprite accepted: %s", meta)
		}
	}
}
