package structure3d

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s := &Store{}
	dir := filepath.Join("testdata", "jar")
	if err := s.OpenDir("fixture", dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestLoadAndRenderIglooTemplate(t *testing.T) {
	s := testStore(t)
	scene, err := AssembleTemplate(s, "minecraft:igloo/top", 0, 64, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(scene.Blocks) < 20 {
		t.Fatalf("expected igloo blocks, got %d", len(scene.Blocks))
	}
	img := NewRenderer(s, 640, 480).Render(scene, DefaultCamera())
	var opaque int
	for i := 0; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 0xe8 || img.Pix[i+1] != 0xec {
			opaque++
		}
	}
	if opaque < 500 {
		t.Fatalf("expected textured pixels, got %d", opaque)
	}
	out := filepath.Join(t.TempDir(), "igloo_render.png")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestJigsawVillageAssembles(t *testing.T) {
	s := testStore(t)
	// Full village pools may miss street pieces in the trimmed fixture; open the
	// live game jar when present so assembly can expand.
	if jar := os.Getenv("MCL_GAME_JAR"); jar != "" {
		if err := s.OpenJar("game", jar); err != nil {
			t.Fatal(err)
		}
	} else if _, err := os.Stat(`D:\MC\.minecraft\versions\1.21.1\1.21.1.jar`); err == nil {
		_ = s.OpenJar("game", `D:\MC\.minecraft\versions\1.21.1\1.21.1.jar`)
	}
	res, err := Preview(s, PreviewRequest{
		StructureID: "minecraft:village_plains",
		Seed:        42,
		X:           0, Z: 0,
		Width: 800, Height: 600,
		Surface: func(x, z int) int { return 64 },
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Unsupported != "" {
		t.Fatalf("unsupported: %s", res.Unsupported)
	}
	if res.BlockCount < 50 {
		t.Fatalf("village too small: %d blocks", res.BlockCount)
	}
	out := filepath.Join(t.TempDir(), "village_render.png")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, res.Image); err != nil {
		t.Fatal(err)
	}
}

func TestJigsawBastionAssembles(t *testing.T) {
	s := testStore(t)
	if jar := os.Getenv("MCL_GAME_JAR"); jar != "" {
		if err := s.OpenJar("game", jar); err != nil {
			t.Fatal(err)
		}
	} else if _, err := os.Stat(`D:\MC\.minecraft\versions\1.21.1\1.21.1.jar`); err == nil {
		_ = s.OpenJar("game", `D:\MC\.minecraft\versions\1.21.1\1.21.1.jar`)
	} else {
		t.Skip("live game jar required for bastion pools")
	}
	res, err := Preview(s, PreviewRequest{
		StructureID: "minecraft:bastion_remnant",
		Seed:        7,
		X:           0, Z: 0,
		Width: 800, Height: 600,
		Surface: func(x, z int) int { return 64 },
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Unsupported != "" {
		t.Fatalf("unsupported: %s", res.Unsupported)
	}
	if res.BlockCount < 50 {
		t.Fatalf("bastion too small: %d blocks", res.BlockCount)
	}
	out := filepath.Join(t.TempDir(), "bastion_render.png")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, res.Image); err != nil {
		t.Fatal(err)
	}
}

func TestUnsupportedCustomElement(t *testing.T) {
	s := &Store{}
	_ = s.OpenBytes("pack", zipWith(`data/pack/worldgen/structure/x.json`, []byte(`{
		"type":"minecraft:jigsaw","start_pool":"pack:start","size":1,"max_distance_from_center":16
	}`), `data/pack/worldgen/template_pool/start.json`, []byte(`{
		"elements":[{"weight":1,"element":{"element_type":"coolmod:java_piece","location":"pack:a"}}]}`)))
	res, err := Preview(s, PreviewRequest{StructureID: "pack:x", Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.Unsupported == "" {
		t.Fatal("expected unsupported reason")
	}
}
