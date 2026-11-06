package cubiomes

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"strconv"
	"sync"
	"testing"
)

func testGenerator(t testing.TB, dimension Dimension) *Generator {
	t.Helper()
	g, err := New(context.Background(), Config{Version: "1.21.1", Seed: 42, Dimension: dimension})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestVanillaProfilesAndInvalidConfig(t *testing.T) {
	for _, version := range []string{"1.18", "1.18.2", "1.19.2", "1.19.4", "1.20.6", "1.21.3", "1.21.4", "1.21.5", "1.21.11"} {
		for _, dimension := range []Dimension{Overworld, Nether, End} {
			g, err := New(context.Background(), Config{Version: version, Seed: math.MinInt64, Dimension: dimension})
			if err != nil {
				t.Fatalf("%s %d: %v", version, dimension, err)
			}
			if biome, ok := g.NewSampler().NoiseBiome(-1, -64, -17); !ok || biome == "" {
				t.Fatalf("empty biome for %s %d", version, dimension)
			}
		}
	}
	for _, cfg := range []Config{
		{Version: "1.17.2"}, {Version: "1.21.12"}, {Version: "26.4"}, {Version: "25w02a"},
		{Version: "1.20.1", Dimension: 3}, {Version: "1.20.1", Dimension: End, LargeBiomes: true},
	} {
		if _, err := New(context.Background(), cfg); err == nil {
			t.Fatalf("accepted unsupported configuration: %+v", cfg)
		}
	}
}

func TestGeneratorConcurrentSamplers(t *testing.T) {
	for _, dimension := range []Dimension{Overworld, Nether, End} {
		g := testGenerator(t, dimension)
		r := Range{X: -1296, Y: -32, Z: 2048, SizeX: 16, SizeY: 2, SizeZ: 16, StepXZ: 4, StepY: 128}
		want := make([]string, 512)
		if err := g.NewSampler().Generate(context.Background(), r, want); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		for range 6 {
			wg.Go(func() {
				s := g.NewSampler()
				got := make([]string, len(want))
				for range 3 {
					if err := s.Generate(context.Background(), r, got); err != nil || !reflect.DeepEqual(got, want) {
						t.Errorf("concurrent dimension %d: error %v or inconsistent raster", dimension, err)
					}
				}
			})
		}
		wg.Wait()
	}
}

func TestSamplerRangeOrderBoundsAndCancellation(t *testing.T) {
	s := testGenerator(t, Overworld).NewSampler()
	r := Range{X: -7, Y: -64, Z: -129, SizeX: 7, SizeY: 3, SizeZ: 9, StepXZ: 3, StepY: 100}
	for _, zoom := range []bool{false, true} {
		r.Voronoi = zoom
		dst := make([]string, r.SizeX*r.SizeY*r.SizeZ)
		if err := s.Generate(context.Background(), r, dst); err != nil {
			t.Fatal(err)
		}
		s.Reset()
		for iy := range r.SizeY {
			for iz := range r.SizeZ {
				for ix := range r.SizeX {
					x, y, z := r.X+ix*r.StepXZ, r.Y+iy*r.StepY, r.Z+iz*r.StepXZ
					if zoom {
						x, y, z = voronoiAccess(s.g.sha, x, y, z)
						x, y, z = x*4, y*4, z*4
					}
					want, ok := s.NextNoiseBiome(x, y, z)
					if got := dst[(iy*r.SizeZ+iz)*r.SizeX+ix]; !ok || got != want {
						t.Fatalf("grid order (%d,%d,%d): %s != %s", ix, iy, iz, got, want)
					}
				}
			}
		}
	}
	for _, invalid := range []Range{
		{}, {SizeX: math.MaxInt, SizeY: math.MaxInt, SizeZ: math.MaxInt, StepXZ: 1, StepY: 1},
		{SizeX: 2, SizeY: 1, SizeZ: 1, StepXZ: math.MaxInt, StepY: 1},
		{X: math.MinInt, SizeX: 1, SizeY: 1, SizeZ: 1, StepXZ: 1, StepY: 1},
	} {
		if err := s.Generate(context.Background(), invalid, make([]string, 16)); err == nil {
			t.Fatalf("accepted invalid range %+v", invalid)
		}
	}
	if err := s.Generate(context.Background(), r, nil); err == nil {
		t.Fatal("accepted undersized output")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Generate(ctx, r, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := New(ctx, Config{Version: "1.21.1"}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := s.g.EstimateSpawn(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestSamplingDoesNotAllocate(t *testing.T) {
	for _, dimension := range []Dimension{Overworld, Nether, End} {
		s := testGenerator(t, dimension).NewSampler()
		i := 0
		if allocs := testing.AllocsPerRun(100, func() {
			i++
			// Move beyond previous End cache coverage to detect map growth.
			if _, ok := s.NoiseBiome(8192+i*64, 64, -8192-i*80); !ok {
				panic("invalid vanilla biome")
			}
		}); allocs != 0 {
			t.Fatalf("dimension %d allocated %g times per query", dimension, allocs)
		}
		dst := make([]string, 64)
		r := Range{X: -8192, Y: 64, Z: 8192, SizeX: 8, SizeY: 1, SizeZ: 8, StepXZ: 4, StepY: 4}
		if allocs := testing.AllocsPerRun(10, func() {
			if err := s.Generate(context.Background(), r, dst); err != nil {
				panic(err)
			}
		}); allocs != 0 {
			t.Fatalf("dimension %d allocated %g times per grid", dimension, allocs)
		}
	}
}

func TestVoronoiCubiomesReference(t *testing.T) {
	b, err := os.ReadFile("testdata/voronoi.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Seed, SHA           string
		X, Y, Z, QX, QY, QZ int
	}
	if err = json.Unmarshal(b, &fixtures); err != nil {
		t.Fatal(err)
	}
	var s *Sampler
	last := ""
	for _, f := range fixtures {
		if f.Seed != last {
			seed, err := strconv.ParseInt(f.Seed, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			g, err := New(context.Background(), Config{Version: "1.20.1", Seed: seed})
			if err != nil {
				t.Fatal(err)
			}
			s, last = g.NewSampler(), f.Seed
		}
		if got := strconv.FormatUint(s.g.sha, 10); got != f.SHA {
			t.Fatalf("seed %s SHA %s != %s", f.Seed, got, f.SHA)
		}
		x, y, z := voronoiAccess(s.g.sha, f.X, f.Y, f.Z)
		if x != f.QX || y != f.QY || z != f.QZ {
			t.Fatalf("seed %s (%d,%d,%d): quart %d,%d,%d != %d,%d,%d", f.Seed, f.X, f.Y, f.Z, x, y, z, f.QX, f.QY, f.QZ)
		}
		want, valid := s.NoiseBiome(f.QX*4, f.QY*4, f.QZ*4)
		if got, ok := s.BiomeAt(f.X, f.Y, f.Z); got != want || ok != valid {
			t.Fatal("block biome did not use the reference Voronoi cell")
		}
	}
}

func TestEndCacheIncludesHeight(t *testing.T) {
	g, err := Compile(context.Background(), nil, Definition{
		Version: "1.21.1", BiomeSource: json.RawMessage(`{"type":"minecraft:the_end"}`),
		Settings: json.RawMessage(`{"noise_router":{"erosion":{"type":"y_clamped_gradient","from_y":0,"to_y":128,"from_value":-1,"to_value":1}}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	s := g.NewSampler()
	for range 3 {
		for _, tc := range []struct {
			y    int
			want string
		}{{0, "minecraft:small_end_islands"}, {128, "minecraft:end_highlands"}, {64, "minecraft:end_midlands"}} {
			got, ok := s.NoiseBiome(8192, tc.y, 8192)
			if !ok || got != tc.want {
				t.Fatalf("Y=%d: %s != %s", tc.y, got, tc.want)
			}
		}
	}
}

func TestEndCacheCollisions(t *testing.T) {
	g := testGenerator(t, End)
	s := g.NewSampler()
	for i := range 64 {
		// Far-apart chunk coordinates deliberately reuse the direct-cache slots.
		x, z := 8192+(i%17)*1024, -8192-(i%11)*1024
		want, valid := g.NewSampler().NoiseBiome(x, 64, z)
		got, ok := s.NoiseBiome(x, 64, z)
		if ok != valid || got != want {
			t.Fatalf("cache collision at %d,%d: %s != %s", x, z, got, want)
		}
	}
}

func BenchmarkNoiseBiome(b *testing.B) {
	for _, tc := range []struct {
		name      string
		dimension Dimension
	}{{"Overworld", Overworld}, {"Nether", Nether}, {"End", End}} {
		b.Run(tc.name, func(b *testing.B) {
			s := testGenerator(b, tc.dimension).NewSampler()
			b.ReportAllocs()
			i := 0
			for b.Loop() {
				i++
				if _, ok := s.NoiseBiome(8192+(i&1023)*4, 64, -8192+((i>>10)&1023)*4); !ok {
					b.Fatal("invalid biome")
				}
			}
		})
	}
}

func BenchmarkGenerate(b *testing.B) {
	s := testGenerator(b, Overworld).NewSampler()
	dst := make([]string, 128*128)
	r := Range{X: 512, Y: 64, Z: 512, SizeX: 128, SizeY: 1, SizeZ: 128, StepXZ: 4, StepY: 4}
	b.ReportAllocs()
	for b.Loop() {
		if err := s.Generate(context.Background(), r, dst); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEndScattered(b *testing.B) {
	s := testGenerator(b, End).NewSampler()
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		i++
		if _, ok := s.NoiseBiome(8192+(i*7919)%100000, 64, -8192-(i*1543)%100000); !ok {
			b.Fatal("invalid biome")
		}
	}
}

func BenchmarkNew(b *testing.B) {
	ctx := context.Background()
	var seed int64
	b.ReportAllocs()
	for b.Loop() {
		seed++
		if _, err := New(ctx, Config{Version: "1.20.1", Seed: seed}); err != nil {
			b.Fatal(err)
		}
	}
}
