package cubiomes

import (
	"context"
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestWithSeedMatchesFreshCompilation(t *testing.T) {
	ctx := context.Background()
	for _, version := range []string{"1.18.2", "1.19.4", "1.20.1", "1.21.11", "26.1", "26.2", "26.3"} {
		documents, err := BuiltinDocuments(version)
		if err != nil {
			t.Fatal(err)
		}
		docs := make(map[string]Document)
		for _, d := range documents {
			docs[d.Kind+"/"+d.ID] = d
		}
		for _, dimension := range []string{"overworld", "large_biomes", "nether", "end"} {
			t.Run(version+"/"+dimension, func(t *testing.T) {
				source := dimension
				if source == "large_biomes" {
					source = "overworld"
				}
				def := Definition{Version: version, Seed: 42, Settings: json.RawMessage(`"minecraft:` + dimension + `"`), BiomeSource: json.RawMessage(`{"type":"minecraft:multi_noise","preset":"minecraft:` + source + `"}`)}
				if source == "end" {
					def.BiomeSource = json.RawMessage(`{"type":"minecraft:the_end"}`)
				}
				base, err := compileFresh(ctx, docs, def)
				if err != nil {
					t.Fatal(err)
				}
				original := base.NewSampler()
				before, _ := original.NoiseBiome(9183, 64, -4218)
				for _, seed := range []int64{0, -1, 42, math.MinInt64, math.MaxInt64} {
					bound, err := base.WithSeed(ctx, seed)
					if err != nil {
						t.Fatal(err)
					}
					def.Seed = seed
					fresh, err := compileFresh(ctx, docs, def)
					if err != nil {
						t.Fatal(err)
					}
					a, b := bound.NewSampler(), fresh.NewSampler()
					for i := range 80 {
						x, y, z := i*7959-320000, i*7-64, 192812-i*1791
						ac, aok := a.Climate(x, y, z)
						bc, bok := b.Climate(x, y, z)
						if ac != bc || aok != bok {
							t.Fatalf("seed %d climate (%d,%d,%d): %v/%v != %v/%v", seed, x, y, z, ac, aok, bc, bok)
						}
						ab, aok := a.NextNoiseBiome(x, y, z)
						bb, bok := b.NextNoiseBiome(x, y, z)
						if ab != bb || aok != bok {
							t.Fatalf("seed %d biome: %s/%v != %s/%v", seed, ab, aok, bb, bok)
						}
					}
				}
				if after, _ := original.NoiseBiome(9183, 64, -4218); after != before {
					t.Fatal("WithSeed modified its source generator")
				}
			})
		}
	}
}

func TestBuiltinDocumentsRemainCallerOwned(t *testing.T) {
	a, err := BuiltinDocuments("1.20.1")
	if err != nil || len(a) == 0 {
		t.Fatal(err)
	}
	want := append([]byte(nil), a[0].Data...)
	a[0].Data[0] ^= 0xff
	b, err := BuiltinDocuments("1.20.1")
	if err != nil || !reflect.DeepEqual([]byte(b[0].Data), want) {
		t.Fatal("caller modified the shared builtin registry", err)
	}
	c, err := BuiltinDocuments("1.20.2")
	if err != nil || c[0].Source != "builtin:1.20.2" {
		t.Fatal("cached provenance used another version", err)
	}
}

func TestWithSeedPreservesExplicitHistoricalSourceSeed(t *testing.T) {
	ctx := context.Background()
	for _, version := range []string{"1.12.2", "1.17.1"} {
		for _, source := range []string{
			`{"type":"minecraft:vanilla_layered"}`,
			`{"type":"minecraft:vanilla_layered","seed":-321}`,
		} {
			def := Definition{Version: version, Seed: 42, Settings: json.RawMessage(`"minecraft:overworld"`), BiomeSource: json.RawMessage(source)}
			base, err := Compile(ctx, nil, def)
			if err != nil {
				t.Fatal(err)
			}
			bound, err := base.WithSeed(ctx, -72)
			if err != nil {
				t.Fatal(err)
			}
			def.Seed = -72
			fresh, err := Compile(ctx, nil, def)
			if err != nil {
				t.Fatal(err)
			}
			a, b := bound.NewSampler(), fresh.NewSampler()
			for i := range 50 {
				x, z := i*179-251, 180-i*571
				got, gok := a.BiomeAt(x, 64, z)
				want, wok := b.BiomeAt(x, 64, z)
				if got != want || gok != wok {
					t.Fatalf("%s %s at %d,%d: %s != %s", version, source, x, z, got, want)
				}
			}
		}
	}
}

func TestCompileCacheTracksRegistryChanges(t *testing.T) {
	ctx := context.Background()
	def := Definition{Version: "custom", Settings: json.RawMessage(`"test:settings"`), BiomeSource: json.RawMessage(`{"type":"minecraft:multi_noise","biomes":[{"biome":"test:a","parameters":{"temperature":0,"humidity":0,"continentalness":0,"erosion":0,"depth":0,"weirdness":0,"offset":0}}]}`)}
	docs := map[string]Document{"noise_settings/test:settings": {Data: json.RawMessage(`{"noise_router":{"temperature":0,"vegetation":0,"continents":0,"erosion":0,"depth":0,"ridges":0}}`)}}
	a, err := Compile(ctx, docs, def)
	if err != nil {
		t.Fatal(err)
	}
	docs["noise_settings/test:settings"] = Document{Data: json.RawMessage(`{"noise_router":{"temperature":0.5,"vegetation":0,"continents":0,"erosion":0,"depth":0,"ridges":0}}`)}
	b, err := Compile(ctx, docs, def)
	if err != nil {
		t.Fatal(err)
	}
	old, _ := a.NewSampler().Climate(0, 0, 0)
	changed, _ := b.NewSampler().Climate(0, 0, 0)
	if old[0] != 0 || changed[0] != 5000 {
		t.Fatalf("override cache collision: %v -> %v", old, changed)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := b.WithSeed(cancelled, 0); err == nil {
		t.Fatal("cached generator ignored cancellation")
	}
}

func TestLegacySkipMatchesSequentialRandom(t *testing.T) {
	for _, seed := range []int64{0, 42, -1, math.MinInt64, math.MaxInt64} {
		for _, count := range []uint64{0, 1, 262, 17292, 1 << 20} {
			a, b := newNoiseRandom(seed, true), newNoiseRandom(seed, true)
			a.skipLegacy(count)
			for range count {
				b.nextBits(32)
			}
			if a.lo != b.lo || a.nextLong() != b.nextLong() {
				t.Fatalf("seed %d skip %d differs", seed, count)
			}
		}
	}
}

func TestEndSlidingCacheMatchesExhaustiveHeight(t *testing.T) {
	for _, seed := range []int64{0, 42, math.MinInt64} {
		n := newEndIslands(seed)
		p := &densitySample{endCache: &endCache{}}
		for i := range 1200 {
			x, z := i-601, i/7-83
			if i%11 == 0 {
				x, z = (i*19851)%4000000-2000000, (i*8749)%4000000-2000000
			}
			got, want := n.height(x, z, p), exhaustiveEndHeight(n, x, z)
			if math.Float32bits(got) != math.Float32bits(want) && !(math.IsNaN(float64(got)) && math.IsNaN(float64(want))) {
				t.Fatalf("seed %d at %d,%d: %g != %g", seed, x, z, got, want)
			}
		}
	}
}

// Original 25x25 search, intentionally with no row cache or occupancy pruning.
func exhaustiveEndHeight(n *endIslands, x, z int) float32 {
	r2 := int32(x)*int32(x) + int32(z)*int32(z)
	height := max(float32(-100), min(float32(80), 100-float32(math.Sqrt(float64(float32(r2))))*8))
	for dx := -12; dx <= 12; dx++ {
		for dz := -12; dz <= 12; dz++ {
			cx, cz := x/2+dx, z/2+dz
			if int64(cx)*int64(cx)+int64(cz)*int64(cz) <= 4096 || n.simplex(float64(cx), float64(cz)) >= float64(float32(-.9)) {
				continue
			}
			v := float32(float32(math.Abs(float64(float32(cx))))*3439) + float32(float32(math.Abs(float64(float32(cz))))*147)
			factor := float32(math.Mod(float64(v), 13)) + 9
			a, b := float32(x%2-dx*2), float32(z%2-dz*2)
			island := 100 - float32(float32(math.Sqrt(float64(a*a+b*b)))*factor)
			height = max(height, max(float32(-100), min(float32(80), island)))
		}
	}
	return height
}
