package launcher

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestSeedMapBlockRasterAndAdjacentTiles(t *testing.T) {
	// An asymmetric pair has no integer-distance tie. The scalar block query
	// therefore provides an independent oracle for every painted pixel.
	parameters := `{"biomes":[{"biome":"pack:cold","parameters":{"temperature":-1,"humidity":0,"continentalness":0,"erosion":0,"depth":0,"weirdness":0,"offset":0}},{"biome":"pack:warm","parameters":{"temperature":1.0001,"humidity":0,"continentalness":0,"erosion":0,"depth":0,"weirdness":0,"offset":0}}]}`
	s := testSeedMapSampler(t, &WorldgenSnapshot{Minecraft: "1.21.1", Documents: []WorldgenDocument{
		{Kind: "dimension", ID: "pack:moon", Data: json.RawMessage(testClimateDimension(testClimateSettings(`{"type":"noise","noise":"pack:heat","xz_scale":1,"y_scale":1}`)))},
		{Kind: "multi_noise_biome_source_parameter_list", ID: "pack:climate", Data: json.RawMessage(parameters)},
		{Kind: "noise", ID: "pack:heat", Data: json.RawMessage(`{"firstOctave":-2,"amplitudes":[1]}`)},
	}}, 42)
	d := s.dimensions["pack:moon"]
	if d.engine == nil {
		t.Fatal("fixture did not compile")
	}
	ref := d.engine.NewSampler()
	changed := 0
	for _, tx := range []int{-2, -1} {
		key := SeedMapTileKey{X: tx, Z: 2, Y: 65, BlocksPerPixel: 1, Dimension: "pack:moon"}
		tile, err := s.Sample(context.Background(), key)
		if err != nil {
			t.Fatal(err)
		}
		for z := range SeedMapTilePixels {
			for x := range SeedMapTilePixels {
				bx, bz := tx*SeedMapTilePixels+x, 2*SeedMapTilePixels+z
				want, ok := ref.BiomeAt(bx, key.Y, bz)
				if !ok || tile.Image.RGBAAt(x, z) != s.biomeColor(want) {
					t.Fatalf("tile %d pixel %d,%d differs from scalar block query", tx, x, z)
				}
				old, _ := ref.NoiseBiome((bx>>2)<<2, key.Y, (bz>>2)<<2)
				if old != want {
					changed++
				}
			}
		}
		key.Y = 64
		lower, err := s.Sample(context.Background(), key)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(lower.Image.Pix, tile.Image.Pix) {
			t.Fatal("adjacent block heights were rounded to a shared quart slice")
		}
	}
	if changed == 0 {
		t.Fatal("fixture did not distinguish block boundaries from stretched quart cells")
	}
}

func TestSeedMapLargeViewportCacheBudget(t *testing.T) {
	for _, step := range []int{1, 2, 4, 32} {
		v := SeedMapViewport{Width: SeedMapMaxWidth, Height: SeedMapMaxHeight, X: -1, Z: 1, BlocksPerPixel: step}
		visible, border := seedMapTileKeys(v)
		if len(visible)+len(border) > seedMapCacheTiles {
			t.Fatal("prefetch would evict visible tiles at the largest canvas size")
		}
	}
}
