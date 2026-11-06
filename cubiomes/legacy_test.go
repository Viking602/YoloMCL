package cubiomes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
)

func TestLegacyReference(t *testing.T) {
	var fixtures []struct {
		Version   string
		Seed      int64
		Dimension int
		Large     bool
		Points    [][5]int
		Ranges    []struct {
			Scale, X, Y, Z, W, H int
			Hash                 string
		}
	}
	b, err := os.ReadFile("testdata/legacy.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		d := Overworld
		if f.Dimension == -1 {
			d = Nether
		}
		if f.Dimension == 1 {
			d = End
		}
		t.Run(fmt.Sprintf("%s/%d/%d/%t", f.Version, f.Seed, d, f.Large), func(t *testing.T) {
			g, err := New(context.Background(), Config{Version: f.Version, Seed: f.Seed, Dimension: d, LargeBiomes: f.Large})
			if err != nil {
				t.Fatal(err)
			}
			s := g.NewSampler()
			for _, p := range f.Points {
				a, ok := s.NoiseBiomeID(p[0], p[1], p[2])
				if !ok || int(a) != p[3] {
					t.Fatalf("noise (%d,%d,%d): %d != %d", p[0], p[1], p[2], a, p[3])
				}
				b, ok := s.BiomeAtID(p[0], p[1], p[2])
				if !ok || int(b) != p[4] {
					t.Fatalf("block (%d,%d,%d): %d != %d", p[0], p[1], p[2], b, p[4])
				}
			}
			for _, r := range f.Ranges {
				out := make([]BiomeID, r.W*r.H)
				grid := Range{X: r.X * r.Scale, Y: r.Y * r.Scale, Z: r.Z * r.Scale, SizeX: r.W, SizeY: 1, SizeZ: r.H, StepXZ: r.Scale, StepY: r.Scale, Voronoi: r.Scale == 1}
				if err = s.GenerateIDs(context.Background(), grid, out); err != nil {
					t.Fatal(err)
				}
				hash := uint64(14695981039346656037)
				for _, id := range out {
					hash = (hash ^ uint64(id)) * 1099511628211
				}
				if got := fmt.Sprintf("%016x", hash); got != r.Hash {
					t.Fatalf("range %+v: hash %s != %s", grid, got, r.Hash)
				}
			}
		})
	}
}

func TestLegacyBulkReuseAndConcurrency(t *testing.T) {
	ctx := context.Background()
	for _, cfg := range []Config{{Version: "b1.7.3"}, {Version: "1.7.10"}, {Version: "1.17.1", LargeBiomes: true}, {Version: "1.17.1", Dimension: Nether}, {Version: "1.17.1", Dimension: End}} {
		t.Run(fmt.Sprintf("%s/%d", cfg.Version, cfg.Dimension), func(t *testing.T) {
			cfg.Seed = -123456789
			g, err := New(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			r := Range{X: -43, Y: -5, Z: 8193, SizeX: 13, SizeY: 3, SizeZ: 11, StepXZ: 3, StepY: 7, Voronoi: true}
			dst := make([]BiomeID, r.SizeX*r.SizeY*r.SizeZ)
			s := g.NewSampler()
			if err = s.GenerateIDs(ctx, r, dst); err != nil {
				t.Fatal(err)
			}
			for i, id := range dst {
				x := i % r.SizeX
				z := (i / r.SizeX) % r.SizeZ
				y := i / (r.SizeX * r.SizeZ)
				want, ok := s.BiomeAtID(r.X+x*r.StepXZ, r.Y+y*r.StepY, r.Z+z*r.StepXZ)
				if !ok || id != want || g.BiomeName(id) == "" {
					t.Fatalf("bulk/point mismatch at %d", i)
				}
			}
			if n := testing.AllocsPerRun(5, func() {
				if e := s.GenerateIDs(ctx, r, dst); e != nil {
					panic(e)
				}
			}); n != 0 {
				t.Fatalf("warm grid allocated %g times", n)
			}
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			if err = s.GenerateIDs(cancelled, r, dst); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if err = s.GenerateIDs(ctx, r, dst); err != nil {
				t.Fatal("sampler did not recover after cancellation")
			}
			var wg sync.WaitGroup
			for range 4 {
				wg.Go(func() {
					worker := g.NewSampler()
					out := make([]BiomeID, len(dst))
					if e := worker.GenerateIDs(ctx, r, out); e != nil {
						t.Error(e)
						return
					}
					for i, id := range out {
						if id != dst[i] {
							t.Errorf("concurrent result differs at %d", i)
							return
						}
					}
				})
			}
			wg.Wait()
		})
	}
}

func TestNumericBiomeAliasesRemainDistinct(t *testing.T) {
	g := &Generator{biomes: []string{"minecraft:mountains", "minecraft:windswept_hills", "pack:a"}}
	g.initBiomeIDs()
	for i, name := range g.biomes {
		if g.BiomeName(g.ids[i]) != name {
			t.Fatalf("ID collision for %s", name)
		}
	}
	if g.ids[0] < 256 || g.ids[1] != 3 || g.ids[2] < 256 {
		t.Fatalf("unstable IDs: %v", g.ids)
	}
}
