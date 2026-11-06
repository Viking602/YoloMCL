package cubiomes

import (
	"context"
	"fmt"
	"math"
	"slices"
	"testing"
)

// The oracle deliberately uses the original scalar Voronoi and noise paths.
// It retains history in raster order, including separate TerraBlender trees.
func checkBlockRange(t *testing.T, g *Generator, s *Sampler, r Range) []BiomeID {
	t.Helper()
	got := make([]BiomeID, r.SizeX*r.SizeY*r.SizeZ)
	if err := s.GenerateIDs(context.Background(), r, got); err != nil {
		t.Fatal(err)
	}
	ref := g.NewSampler()
	i := 0
	for iy := range r.SizeY {
		for iz := range r.SizeZ {
			for ix := range r.SizeX {
				x, y, z := ref.voronoi(r.X+ix*r.StepXZ, r.Y+iy*r.StepY, r.Z+iz*r.StepXZ)
				want, ok := ref.NextNoiseBiomeID(x*4, y*4, z*4)
				if !ok || got[i] != want {
					t.Fatalf("%+v cell %d,%d,%d: %d, want %d", r, ix, iy, iz, got[i], want)
				}
				i++
			}
		}
	}
	// Names share the same block path and must not change the ordering.
	names := make([]string, len(got))
	if err := s.Generate(context.Background(), r, names); err != nil {
		t.Fatal(err)
	}
	for i, id := range got {
		if names[i] != g.BiomeName(id) {
			t.Fatalf("name/ID disagreement at %d", i)
		}
	}
	return got
}

func TestBlockRangeMatchesScalar(t *testing.T) {
	for _, version := range []string{"1.18", "1.21.1", "26.3", "BOP26.3"} {
		for _, dim := range []Dimension{Overworld, Nether, End} {
			t.Run(fmt.Sprintf("%s/%d", version, dim), func(t *testing.T) {
				var g *Generator
				if version == "BOP26.3" {
					g = terraBlenderTestGenerator(t, dim, -42)
				} else {
					var err error
					g, err = New(context.Background(), Config{Version: version, Dimension: dim, Seed: -42})
					if err != nil {
						t.Fatal(err)
					}
				}
				s := g.NewSampler()
				for _, r := range []Range{
					{X: -129, Y: -3, Z: 255, SizeX: 67, SizeY: 5, SizeZ: 19, StepXZ: 1, StepY: 1, Voronoi: true},
					{X: 1280, Y: 65, Z: -2049, SizeX: 67, SizeY: 2, SizeZ: 19, StepXZ: 2, StepY: 120, Voronoi: true},
					{X: -30000000, Y: -2048, Z: 30000000, SizeX: 33, SizeY: 2, SizeZ: 17, StepXZ: 3, StepY: 4095, Voronoi: true},
					{X: math.MinInt32 + 4, Y: 0, Z: math.MaxInt32 - 100, SizeX: 13, SizeY: 1, SizeZ: 13, StepXZ: 3, StepY: 1, Voronoi: true},
				} {
					checkBlockRange(t, g, s, r)
				}
			})
		}
	}
}

func TestBlockRangePreservesHistoryAcrossCachedQuart(t *testing.T) {
	a, b := &climateNode{Biome: "test:a"}, &climateNode{Biome: "test:b"}
	a.Space[0], b.Space[0] = climateRange{-1, -1}, climateRange{1, 1}
	root, err := buildClimateTree(context.Background(), []*climateNode{a, b}, 6)
	if err != nil {
		t.Fatal(err)
	}
	climate := &climateSampler{root: compactClimateTree(root, []string{a.Biome, b.Biome}), nodes: 2}
	for i := range climate.router {
		climate.router[i] = &densityNode{id: 0, kind: "constant"}
	}
	// Y=0 is a tie, Y=4 selects b. Voronoi repeatedly revisits both planes.
	climate.router[0] = &densityNode{id: 1, kind: "y_clamped_gradient", q: 4, s: .0001}
	g := &Generator{climate: climate, sha: testGenerator(t, Overworld).sha}
	g.initBiomeIDs()
	r := Range{X: -128, Y: 4, Z: -64, SizeX: 64, SizeY: 1, SizeZ: 32, StepXZ: 1, StepY: 1, Voronoi: true}
	// Find a starting tie so caching the first winner would be observably wrong.
	for {
		_, y, _ := voronoiAccess(g.sha, r.X, r.Y, r.Z)
		if y == 0 {
			break
		}
		r.X++
	}
	got := checkBlockRange(t, g, g.NewSampler(), r)
	if got[0] == got[len(got)-1] || !slices.Contains(got, g.ids[0]) || !slices.Contains(got, g.ids[1]) {
		t.Fatal("fixture did not exercise a history-dependent tie")
	}
	ref := g.NewSampler()
	seen := map[[3]int]BiomeID{}
	changed := false
	for z := range r.SizeZ {
		for x := range r.SizeX {
			qx, qy, qz := ref.voronoi(r.X+x, r.Y, r.Z+z)
			key := [3]int{qx, qy, qz}
			id := got[z*r.SizeX+x]
			old, ok := seen[key]
			changed = changed || ok && old != id
			seen[key] = id
		}
	}
	if !changed {
		t.Fatal("fixture never revisited a quart cell after its winning biome changed")
	}
}

func TestBlockRangeScratchBoundsAndReuse(t *testing.T) {
	g := testGenerator(t, Overworld)
	s := g.NewSampler()
	r := Range{SizeX: 32768, SizeY: 1, SizeZ: 1, StepXZ: 2, StepY: 1, Voronoi: true}
	checkBlockRange(t, g, s, r)
	if len(s.blocks) != 0 {
		t.Fatal("wide fallback allocated an excessive corner rectangle")
	}
	r.SizeX, r.SizeZ, r.StepXZ = 128, 128, 1
	dst := make([]BiomeID, r.SizeX*r.SizeZ)
	if err := s.GenerateIDs(context.Background(), r, dst); err != nil {
		t.Fatal(err)
	}
	if allocs := testing.AllocsPerRun(10, func() {
		r.X += 128 // do not measure a finished-raster cache hit
		if err := s.GenerateIDs(context.Background(), r, dst); err != nil {
			t.Fatal(err)
		}
	}); allocs != 0 {
		t.Fatalf("warm block raster allocated %g times", allocs)
	}
}

func BenchmarkBlockRange(b *testing.B) {
	ctx := context.Background()
	for _, dim := range []Dimension{Overworld, Nether, End} {
		for _, step := range []int{1, 2, 4} {
			b.Run(fmt.Sprintf("dimension%d/step%d", dim, step), func(b *testing.B) {
				g := testGenerator(b, dim)
				s := g.NewSampler()
				r := Range{X: -1296, Y: 65, Z: 2048, SizeX: 128, SizeY: 1, SizeZ: 128, StepXZ: step, StepY: 1, Voronoi: true}
				ids := make([]BiomeID, 128*128)
				if err := s.GenerateIDs(ctx, r, ids); err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					if err := s.GenerateIDs(ctx, r, ids); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
