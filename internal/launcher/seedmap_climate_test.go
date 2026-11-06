package launcher

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"github.com/Viking602/YoloMCL/cubiomes"
)

func TestClimateCubiomesReference(t *testing.T) {
	b, err := os.ReadFile("../../cubiomes/testdata/cubiomes.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Version, Seed          string
		Large                  bool
		X, Y, Z, Biome, Nether int
		Climate                [6]int64
	}
	if err := json.Unmarshal(b, &fixtures); err != nil {
		t.Fatal(err)
	}
	var pale = fixtures[:0:0]
	b, err = os.ReadFile("../../cubiomes/testdata/cubiomes-pale.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &pale); err != nil {
		t.Fatal(err)
	}
	fixtures = append(fixtures, pale...)
	var sampler *LocalSampler
	last := ""
	for _, f := range fixtures {
		key := f.Version + "/" + f.Seed + "/" + strconv.FormatBool(f.Large)
		if key != last {
			seed, err := strconv.ParseInt(f.Seed, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			snap, err := WorldgenIngest(context.Background(), WorldgenInput{Minecraft: f.Version})
			if err != nil {
				t.Fatal(err)
			}
			if f.Large {
				snap.Documents = append(snap.Documents, WorldgenDocument{ID: "minecraft:overworld", Kind: "dimension", Data: json.RawMessage(`{"generator":{"type":"minecraft:noise","settings":"minecraft:large_biomes","biome_source":{"type":"minecraft:multi_noise","preset":"minecraft:overworld"}}}`)})
			}
			sampler = testSeedMapSampler(t, snap, seed)
			last = key
		}
		for dimension, id := range map[string]int{"minecraft:overworld": f.Biome, "minecraft:the_nether": f.Nether} {
			want := cubiomes.BiomeName(id)
			if dimension == "minecraft:the_nether" {
				want = map[int]string{8: "minecraft:nether_wastes", 170: "minecraft:soul_sand_valley", 171: "minecraft:crimson_forest", 172: "minecraft:warped_forest", 173: "minecraft:basalt_deltas"}[id]
			}
			got, ok := sampler.BiomeAt(dimension, f.X, f.Y, f.Z)
			if !ok || got != want {
				if c := sampler.dimensions[dimension].engine; c != nil {
					p := c.NewSampler()
					target, _ := p.Climate(f.X, f.Y, f.Z)
					t.Logf("climate %v expected %v", target, f.Climate)
				}
				t.Fatalf("%s %s (%d,%d,%d): %s want %s; coverage %+v", key, dimension, f.X, f.Y, f.Z, got, want, sampler.Coverage(dimension))
			}
		}
		c := sampler.dimensions["minecraft:overworld"].engine
		p := c.NewSampler()
		point, ok := p.Climate(f.X, f.Y, f.Z)
		if !ok {
			t.Fatal("invalid climate")
		}
		for i, v := range f.Climate {
			// Cubiomes uses a slightly rearranged depth expression; its final
			// float can differ by one quantization unit at a rounding boundary.
			delta := point[i] - v
			if delta < 0 {
				delta = -delta
			}
			if delta > 1 || (i != 4 && delta != 0) {
				t.Fatalf("%s (%d,%d,%d) climate %d: %d want %d", key, f.X, f.Y, f.Z, i, point[i], v)
			}
		}
	}
}

func BenchmarkMultiNoiseTile(b *testing.B) {
	ctx := context.Background()
	snapshot, err := WorldgenIngest(ctx, WorldgenInput{Minecraft: "1.20.1"})
	if err != nil {
		b.Fatal(err)
	}
	sampler, err := NewLocalSampler(ctx, snapshot, 42)
	if err != nil {
		b.Fatal(err)
	}
	key := SeedMapTileKey{Dimension: "minecraft:overworld", X: 1, Z: 1, Y: 64, BlocksPerPixel: 4}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := sampler.Sample(ctx, key); err != nil {
			b.Fatal(err)
		}
	}
}
