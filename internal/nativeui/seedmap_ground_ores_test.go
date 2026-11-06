package nativeui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"path/filepath"
	"testing"

	"github.com/Viking602/YoloMCL/internal/launcher"
	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// Exercise asset loading, resource-pack model overrides, both rock variants
// and changing the cut on a reused frame. The original surface deliberately
// has no texture: its UVs must still support the newly revealed ore face.
func TestPreviewOreTextures(t *testing.T) {
	ores := []string{"coal", "iron", "copper", "gold", "redstone", "lapis", "diamond", "emerald"}
	files := map[string]string{}
	want := map[string][2]color.RGBA{}
	add := func(files map[string]string, id, texture string, pair [2]color.RGBA) {
		files["assets/minecraft/blockstates/"+id+".json"] = fmt.Sprintf(`{"variants":{"":{"model":"minecraft:block/%s"}}}`, id)
		files["assets/minecraft/models/block/"+id+".json"] = fmt.Sprintf(`{"textures":{"ore":"minecraft:block/%s"},"elements":[{"from":[0,0,0],"to":[16,16,16],"faces":{"up":{"texture":"#ore"}}}]}`, texture)
		img := image.NewRGBA(image.Rect(0, 0, 2, 2))
		for y := 0; y < 2; y++ {
			for x := 0; x < 2; x++ {
				img.SetRGBA(x, y, pair[(x+y)%2])
			}
		}
		var data bytes.Buffer
		if err := png.Encode(&data, img); err != nil {
			t.Fatal(err)
		}
		files["assets/minecraft/textures/block/"+texture+".png"] = data.String()
		want[id] = pair
	}
	for i, ore := range ores {
		for j, prefix := range []string{"", "deepslate_"} {
			id := prefix + ore + "_ore"
			n := uint8(i*2 + j)
			add(files, id, id, [2]color.RGBA{{20 + n*3, 60, 120, 255}, {180, 90 + n*3, 40, 255}})
		}
	}
	dir := t.TempDir()
	store := &structure3d.Store{}
	t.Cleanup(func() { _ = store.Close() })
	base := filepath.Join(dir, "game.jar")
	writeZip(t, base, files)
	if err := store.OpenJar("game", base); err != nil {
		t.Fatal(err)
	}
	packFiles := map[string]string{}
	add(packFiles, "deepslate_diamond_ore", "pack_diamond", [2]color.RGBA{{230, 20, 180, 255}, {20, 240, 160, 255}})
	pack := filepath.Join(dir, "pack.zip")
	writeZip(t, pack, packFiles)
	if err := store.OpenJar("pack", pack); err != nil {
		t.Fatal(err)
	}
	g := newPreviewGround(0, "minecraft:overworld", nil, func(_, _ int) int { return 100 }, 1, false, "", 0, 0, 0, 0, store)
	g.biomes[[2]int{0, 0}] = "minecraft:plains"
	for _, ore := range ores {
		t.Run(ore, func(t *testing.T) {
			g.ores = map[[3]int]string{{0, 12, 0}: ore, {0, -52, 0}: ore}
			m := &structure3d.Mesh{}
			m.AddTerrain(structure3d.Terrain{W: 1, D: 1, Step: 1, Bottom: -56,
				Height: func(_, _ int) int { return 20 }, Color: func(_, _, _ int) [3]uint8 { return groundWater },
				Section: g.Section, Textures: g.textures()})
			var frame structure3d.Frame
			for i, y := range []int{12, -52, 12} {
				id := ore + "_ore"
				if i == 1 {
					id = "deepslate_" + id
				}
				img := m.Render(&frame, 160, 160, structure3d.Camera{Pitch: -math.Pi / 2, Zoom: 100,
					Cut: true, CutY: float64(y + 1), Aimed: true, Aim: [3]float64{.5, float64(y + 1), .5}})
				for _, c := range want[id] {
					c.R, c.G, c.B = uint8(float32(c.R)*.85), uint8(float32(c.G)*.85), uint8(float32(c.B)*.85)
					n := 0
					for py := 0; py < 160; py++ {
						for px := 0; px < 160; px++ {
							if img.RGBAAt(px, py) == c {
								n++
							}
						}
					}
					if n < 1000 {
						t.Fatalf("%s: textured cut missing colour %v (%d pixels)", id, c, n)
					}
				}
			}
		})
	}
}

func TestPreviewOreFollowsSurfaceRule(t *testing.T) {
	g := newPreviewGround(0, "minecraft:overworld", nil, func(_, _ int) int { return 100 }, 1, false, "", 0, 0, 0, 0, nil)
	g.noise, g.rules = true, true
	g.info = launcher.SeedMapTerrainInfo{MinY: -64, Height: 192}
	g.rulePalette = []string{"", "minecraft:stone", "minecraft:deepslate"}
	c := &groundColumn{solidTop: 100, sub: make([]launcher.TerrainSubstance, 192), rules: &ruleColumn{lo: -64, idx: make([]uint8, 192)}}
	for i := range c.sub {
		c.sub[i] = launcher.TerrainSolid
	}
	g.cols = map[[2]int]*groundColumn{{0, 0}: c}
	g.ores = map[[3]int]string{{0, -52, 0}: "diamond", {0, 12, 0}: "diamond"}
	for _, tc := range []struct {
		y    int
		rule uint8
		want [3]uint8
	}{{-52, 1, groundOre["diamond"]}, {12, 2, deepslateOreColor(groundOre["diamond"])}} {
		c.rules.idx[tc.y+64] = tc.rule
		if got := g.Section(0, tc.y, 0); got != tc.want {
			t.Fatalf("Y %d: ore ignored the host rock rule: %v", tc.y, got)
		}
	}
}
