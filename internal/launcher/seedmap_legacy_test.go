package launcher

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/Viking602/YoloMCL/cubiomes"
)

func TestLegacySeedMapRaster(t *testing.T) {
	ctx := context.Background()
	for _, version := range []string{"b1.7.3", "b1.8.1", "1.0", "1.6.4", "1.7.10", "1.12.2", "1.16.1", "1.16.5", "1.17.1"} {
		t.Run(version, func(t *testing.T) {
			snap, err := WorldgenIngest(ctx, WorldgenInput{Minecraft: version})
			if err != nil {
				t.Fatal(err)
			}
			s, err := NewLocalSampler(ctx, snap, -42)
			if err != nil {
				t.Fatal(err)
			}
			if c := s.Coverage("minecraft:overworld"); c.Terrain != "vanilla_layered" || c.Opaque {
				t.Fatalf("coverage %+v", c)
			}
			for _, biome := range s.Biomes("minecraft:overworld") {
				if _, ok := seedMapVanillaColors[biome.ID]; !ok {
					t.Errorf("missing palette %s", biome.ID)
				}
			}
			g, err := cubiomes.New(ctx, cubiomes.Config{Version: version, Seed: -42})
			if err != nil {
				t.Fatal(err)
			}
			worker := g.NewSampler()
			for _, step := range []int{1, 2, 4, 7, 32} {
				key := SeedMapTileKey{X: -3, Z: 2, Y: 64, BlocksPerPixel: step, Dimension: "minecraft:overworld"}
				tile, err := s.Sample(ctx, key)
				if err != nil {
					t.Fatal(err)
				}
				for y := 0; y < 128; y += 19 {
					for x := 0; x < 128; x += 17 {
						bx := key.X*128*step + x*step
						bz := key.Z*128*step + y*step
						name, ok := worker.NoiseBiome(bx, key.Y, bz)
						if step < 4 {
							name, ok = worker.BiomeAt(bx, key.Y, bz)
						}
						if !ok || tile.Image.RGBAAt(x, y) != s.biomeColor(name) {
							t.Fatalf("step %d pixel %d,%d: want %s", step, x, y, name)
						}
					}
				}
			}
		})
	}
}

func TestLegacySavedWorldSettings(t *testing.T) {
	ctx := context.Background()
	for _, name := range []string{"default", "largeBiomes", "amplified", "customized", "flat", "default_1_1"} {
		t.Run(name, func(t *testing.T) {
			root := map[string]any{"Data": map[string]any{"RandomSeed": int64(42), "generatorName": name, "generatorVersion": int32(1), "generatorOptions": ""}}
			var buf bytes.Buffer
			writeSeedMapTestNBT(t, &buf, "", root)
			level, err := ParseLevelDat(bytes.NewReader(compressedNBT(t, buf.Bytes())))
			if err != nil {
				t.Fatal(err)
			}
			saved := seedMapLegacySavedDimensions("1.12.2", level)
			snap, err := WorldgenIngest(ctx, WorldgenInput{Minecraft: "1.12.2", SavedWorld: true, SavedDimensions: saved})
			if err != nil {
				t.Fatal(err)
			}
			s, err := NewLocalSampler(ctx, snap, level.RandomSeed)
			if err != nil {
				t.Fatal(err)
			}
			if name == "customized" || name == "flat" || name == "default_1_1" {
				if _, ok := s.BiomeAt("minecraft:overworld", 0, 64, 0); ok {
					t.Fatal("unsupported old settings replaced by default")
				}
				return
			}
			g, err := cubiomes.New(ctx, cubiomes.Config{Version: "1.12.2", Seed: 42, LargeBiomes: name == "largeBiomes"})
			if err != nil {
				t.Fatal(err)
			}
			worker := g.NewSampler()
			for x := -256; x <= 256; x += 64 {
				want, _ := worker.NoiseBiome(x, 64, -x)
				got, ok := s.BiomeAt("minecraft:overworld", x, 64, -x)
				if !ok || got != want {
					t.Fatalf("saved %s: %s != %s", name, got, want)
				}
			}
		})
	}
	level := LevelInfo{LegacyGenerator: "default", HasLegacyGeneratorVersion: true}
	dims := seedMapLegacySavedDimensions("1.12.2", level)
	if !bytes.Contains(dims["minecraft:overworld"], []byte(`"legacy_biome_init_layer":true`)) {
		t.Fatal("generatorVersion=0 lost")
	}
}

func TestLegacyNoiseOverrideStaysUnsupported(t *testing.T) {
	ctx := context.Background()
	snap, err := WorldgenIngest(ctx, WorldgenInput{Minecraft: "1.17.1"})
	if err != nil {
		t.Fatal(err)
	}
	snap.Documents = append(snap.Documents, WorldgenDocument{Kind: "noise_settings", ID: "minecraft:overworld", Source: "mod:custom", Data: json.RawMessage(`{}`)})
	s, err := NewLocalSampler(ctx, snap, 42)
	if err != nil {
		t.Fatal(err)
	}
	if c := s.Coverage("minecraft:overworld"); c.Terrain != "candidates" || !c.Opaque {
		t.Fatalf("overridden legacy noise silently replaced: %+v", c)
	}
}

func BenchmarkLegacyTile(b *testing.B) {
	for _, version := range []string{"1.7.10", "1.12.2", "1.17.1"} {
		b.Run(version, func(b *testing.B) {
			ctx := context.Background()
			snap, err := WorldgenIngest(ctx, WorldgenInput{Minecraft: version})
			if err != nil {
				b.Fatal(err)
			}
			s, err := NewLocalSampler(ctx, snap, 42)
			if err != nil {
				b.Fatal(err)
			}
			key := SeedMapTileKey{X: 16, Z: -16, Y: 64, BlocksPerPixel: 4, Dimension: "minecraft:overworld"}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := s.Sample(ctx, key); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
