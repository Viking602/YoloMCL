package launcher

import (
	"image/color"
	"math"
	"testing"

	"github.com/Viking602/YoloMCL/cubiomes"
)

func TestSeedMapNoiseShade(t *testing.T) {
	cells := make([]cubiomes.SurfaceSample, seedMapSurfaceWidth*seedMapSurfaceWidth)
	lava := make([]bool, len(cells))
	for i := range cells {
		cells[i].Height = 60
	}
	at := func(x, z int) int { return z*seedMapSurfaceWidth + x }
	biome := color.RGBA{200, 200, 120, 255}

	cells[at(5, 5)].Height = float32(math.NaN())
	if got := seedMapNoiseShade(biome, "minecraft:the_end", cells, lava, 5, 5, 1, false, true); got != seedMapVoid {
		t.Fatalf("void pixel = %v", got)
	}
	// a void neighbour reads as level ground, not a cliff to -infinity
	edge := seedMapNoiseShade(biome, "minecraft:the_end", cells, lava, 6, 5, 1, false, false)
	flat := seedMapNoiseShade(biome, "minecraft:the_end", cells, lava, 20, 20, 1, false, false)
	if edge != flat {
		t.Fatalf("void edge %v, flat %v", edge, flat)
	}
	lava[at(9, 9)] = true
	if got := seedMapNoiseShade(biome, "minecraft:nether_wastes", cells, lava, 9, 9, 1, false, false); got.R <= got.B {
		t.Fatalf("lava not lava-coloured: %v", got)
	}
	if a, b := seedMapNoiseTerrainBase("minecraft:crimson_forest", false), seedMapNoiseTerrainBase("minecraft:warped_forest", false); a == b {
		t.Fatal("crimson and warped share a colour")
	}
	// relief: the shared hillshade separates a slope from its mirror
	for x := range seedMapSurfaceWidth {
		for z := range seedMapSurfaceWidth {
			cells[at(x, z)].Height = float32(-x)
		}
	}
	lit := seedMapNoiseShade(biome, "minecraft:the_end", cells, lava, 30, 30, 1, false, false)
	for x := range seedMapSurfaceWidth {
		for z := range seedMapSurfaceWidth {
			cells[at(x, z)].Height = float32(x)
		}
	}
	dark := seedMapNoiseShade(biome, "minecraft:the_end", cells, lava, 30, 30, 1, false, false)
	if lit.R == dark.R {
		t.Fatalf("hillshade lit %v, dark %v", lit, dark)
	}
}

func TestSeedMapPointAtVoid(t *testing.T) {
	key := SeedMapTileKey{BlocksPerPixel: 1, Surface: true}
	tile := &SeedMapTile{Key: key, palette: []SeedMapBiome{{}, {ID: "minecraft:the_end"}}}
	tile.surface = make([]cubiomes.SurfaceSample, seedMapSurfaceWidth*seedMapSurfaceWidth)
	for i := range tile.surface {
		tile.surface[i] = cubiomes.SurfaceSample{Height: 70, Biome: 1}
	}
	tile.surface[(2+1)*seedMapSurfaceWidth+3+1].Height = float32(math.NaN())
	f := SeedMapFrame{Viewport: SeedMapViewport{X: SeedMapTilePixels / 2, Z: SeedMapTilePixels / 2, Width: SeedMapTilePixels, Height: SeedMapTilePixels, BlocksPerPixel: 1, Surface: true}, tiles: []*SeedMapTile{tile}}
	p, ok := f.PointAt(3, 2)
	if !ok || !p.Void || p.Surface {
		t.Fatalf("void point %+v %v", p, ok)
	}
	p, ok = f.PointAt(4, 2)
	if !ok || p.Void || !p.Surface || p.Height != 70 {
		t.Fatalf("ground point %+v %v", p, ok)
	}
}
