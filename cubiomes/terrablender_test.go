package cubiomes

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

func terraBlenderTestGenerator(t testing.TB, dimension Dimension, seed int64) *Generator {
	return terraBlenderProfileGenerator(t, dimension, seed, "26.3.0.0.15")
}

func terraBlenderProfileGenerator(t testing.TB, dimension Dimension, seed int64, bopVersion string) *Generator {
	t.Helper()
	ctx := context.Background()
	g, err := New(ctx, Config{Version: "26.3", Dimension: dimension, Seed: seed})
	if err != nil {
		t.Fatal(err)
	}
	if dimension == End {
		cfg, e := BuiltinTerraBlenderEndProfile("26.3", bopVersion)
		if e != nil {
			t.Fatal(e)
		}
		g, err = WithTerraBlenderEnd(ctx, g, cfg)
	} else {
		cfg, e := BuiltinTerraBlenderProfile("26.3", bopVersion, dimension)
		if e != nil {
			t.Fatal(e)
		}
		g, err = WithTerraBlender(ctx, g, cfg)
	}
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func readTerraBlenderFixture(t testing.TB, path string, value any) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var reader io.Reader = f
	if len(path) > 3 && path[len(path)-3:] == ".gz" {
		r, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		reader = r
	}
	if err := json.NewDecoder(reader).Decode(value); err != nil {
		t.Fatal(err)
	}
}

// The bundled fixture hashes every cell; deterministic sample subsets retain
// coordinates/climate for diagnostics. MCL_TERRABLENDER_DENSE can additionally
// name the full external oracle export without shipping a Java project.
func TestTerraBlenderBOPOracle(t *testing.T) {
	path := os.Getenv("MCL_TERRABLENDER_DENSE")
	if path == "" {
		path = "testdata/terrablender-bop-26.3.json.gz"
	}
	testTerraBlenderOracle(t, path, "26.3.0.0.15")
}

func TestTerraBlenderOnlyOracle(t *testing.T) {
	path := os.Getenv("MCL_TERRABLENDER_ONLY_DENSE")
	if path == "" {
		path = "testdata/terrablender-only-26.3.json.gz"
	}
	testTerraBlenderOracle(t, path, "")
}

type terraBlenderOracleCase struct {
	Dimension, Seed string
	Grids           []struct {
		Range   [6]int
		Names   string `json:"fnv1a_names"`
		Regions string `json:"fnv1a_regions"`
		Samples []struct {
			Position [3]int
			Climate  [6]int64
			Region   int
			Biome    string
		}
	}
}

type terraBlenderBoundaryCase struct {
	Dimension, Seed string
	Region          int
	Quart           [3]int
	Samples         []struct {
		Climate [6]int64
		Biome   string
	}
}

type terraBlenderTestFactory func(*testing.T, Dimension, int64) *Generator

func testTerraBlenderOracle(t *testing.T, path, bopVersion string) {
	var fixture struct{ Cases []terraBlenderOracleCase }
	readTerraBlenderFixture(t, path, &fixture)
	checkTerraBlenderOracle(t, fixture.Cases, func(t *testing.T, dimension Dimension, seed int64) *Generator {
		return terraBlenderProfileGenerator(t, dimension, seed, bopVersion)
	})
}

func checkTerraBlenderOracle(t *testing.T, cases []terraBlenderOracleCase, factory terraBlenderTestFactory) {
	for _, c := range cases {
		t.Run(c.Dimension+"/"+c.Seed, func(t *testing.T) {
			dimension, exists := map[string]Dimension{"overworld": Overworld, "nether": Nether, "end": End}[c.Dimension]
			if !exists {
				t.Fatal("invalid fixture dimension")
			}
			seed, err := strconv.ParseInt(c.Seed, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			g := factory(t, dimension, seed)
			s := g.NewSampler()
			for _, grid := range c.Grids {
				r := Range{X: grid.Range[0], Y: grid.Range[1], Z: grid.Range[2], StepXZ: grid.Range[3], SizeX: grid.Range[4], SizeY: 1, SizeZ: grid.Range[5], StepY: 1}
				ids := make([]BiomeID, r.SizeX*r.SizeZ)
				if err := s.GenerateIDs(context.Background(), r, ids); err != nil {
					t.Fatal(err)
				}
				names, regions := fnv.New64a(), fnv.New64a()
				for i, id := range ids {
					fmt.Fprintln(names, g.BiomeName(id))
					index := -1
					if dimension != End {
						index, _ = s.TerraBlenderRegionIndex(r.X+(i%r.SizeX)*r.StepXZ, r.Z+(i/r.SizeX)*r.StepXZ)
					}
					fmt.Fprintln(regions, index)
				}
				for _, sample := range grid.Samples {
					x, y, z := sample.Position[0], sample.Position[1], sample.Position[2]
					i := ((z-r.Z)/r.StepXZ)*r.SizeX + (x-r.X)/r.StepXZ
					if got := g.BiomeName(ids[i]); got != sample.Biome {
						t.Fatalf("at %v: biome %s, want %s", sample.Position, got, sample.Biome)
					}
					if dimension != End {
						if got, ok := s.Climate(x, y, z); !ok || got != sample.Climate {
							t.Fatalf("at %v: climate %v/%v, want %v", sample.Position, got, ok, sample.Climate)
						}
						if got, ok := s.TerraBlenderRegionIndex(x, z); !ok || got != sample.Region {
							t.Fatalf("at %v: region %d/%v, want %d", sample.Position, got, ok, sample.Region)
						}
					}
				}
				wantNames, err := strconv.ParseUint(grid.Names, 16, 64)
				if err != nil {
					t.Fatal(err)
				}
				if got := names.Sum64(); got != wantNames {
					t.Errorf("grid %v: biome hash %016x, want %s", grid.Range, got, grid.Names)
				}
				wantRegions, err := strconv.ParseUint(grid.Regions, 16, 64)
				if err != nil {
					t.Fatal(err)
				}
				if got := regions.Sum64(); got != wantRegions {
					t.Errorf("grid %v: region hash %016x, want %s", grid.Range, got, grid.Regions)
				}
			}
		})
	}
}

func TestTerraBlenderBoundaryHistory(t *testing.T) {
	testTerraBlenderBoundaryHistory(t, "testdata/terrablender-bop-26.3-boundaries.json.gz", "26.3.0.0.15")
}

func TestTerraBlenderOnlyBoundaryHistory(t *testing.T) {
	testTerraBlenderBoundaryHistory(t, "testdata/terrablender-only-26.3-boundaries.json.gz", "")
}

func testTerraBlenderBoundaryHistory(t *testing.T, path, bopVersion string) {
	var fixture struct{ Cases []terraBlenderBoundaryCase }
	readTerraBlenderFixture(t, path, &fixture)
	checkTerraBlenderBoundaryHistory(t, fixture.Cases, func(t *testing.T, dimension Dimension, seed int64) *Generator {
		return terraBlenderProfileGenerator(t, dimension, seed, bopVersion)
	})
}

func checkTerraBlenderBoundaryHistory(t *testing.T, cases []terraBlenderBoundaryCase, factory terraBlenderTestFactory) {
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s/%d", c.Dimension, c.Region), func(t *testing.T) {
			dimension := Overworld
			if c.Dimension == "nether" {
				dimension = Nether
			}
			seed, err := strconv.ParseInt(c.Seed, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			g := factory(t, dimension, seed)
			s := g.NewSampler()
			region, ok := s.TerraBlenderRegionIndex(c.Quart[0]*4, c.Quart[2]*4)
			if !ok || region != c.Region {
				t.Fatalf("region %d/%v, want %d", region, ok, c.Region)
			}
			for i, sample := range c.Samples {
				var p climatePoint
				copy(p[:], sample.Climate[:])
				if got := g.BiomeName(s.terraBlenderClimate(&p, region)); got != sample.Biome {
					t.Fatalf("target %d %v: got %s want %s", i, sample.Climate, got, sample.Biome)
				}
			}
		})
	}
}

func TestTerraBlenderConfigOracle(t *testing.T) {
	var fixture struct {
		Variants []struct {
			Name          string
			Profile       terraBlenderProfileData
			Toggles       map[string]bool
			Cases         []terraBlenderOracleCase
			BoundaryCases []terraBlenderBoundaryCase `json:"boundary_cases"`
		}
	}
	readTerraBlenderFixture(t, "testdata/terrablender-bop-26.3-configs.json.gz", &fixture)
	for _, variant := range fixture.Variants {
		t.Run(variant.Name, func(t *testing.T) {
			if root := os.Getenv("MCL_TERRABLENDER_CONFIG_DENSE"); root != "" {
				var dense struct{ Cases []terraBlenderOracleCase }
				readTerraBlenderFixture(t, filepath.Join(root, "native-variant-"+variant.Name, "terrablender-bop-26.3-oracle-dense.json"), &dense)
				variant.Cases = dense.Cases
			}
			factory := func(t *testing.T, dimension Dimension, seed int64) *Generator {
				t.Helper()
				ctx := context.Background()
				base, err := New(ctx, Config{Version: "26.3", Dimension: dimension, Seed: seed})
				if err != nil {
					t.Fatal(err)
				}
				var g *Generator
				if dimension == End {
					cfg, err := BuiltinTerraBlenderEndProfileWithToggles("26.3", "26.3.0.0.15", variant.Toggles)
					if err != nil {
						t.Fatal(err)
					}
					for name, table := range map[string]*TerraBlenderEndTable{"highlands": &cfg.Highlands, "midlands": &cfg.Midlands, "edge": &cfg.Edge, "islands": &cfg.Islands} {
						want := variant.Profile.Dimensions.End[name]
						table.Size = want.Size
						if len(table.Biomes) != len(want.Entries) {
							t.Fatalf("End %s entries differ", name)
						}
						for i := range table.Biomes {
							if table.Biomes[i].Biome != want.Entries[i].Biome {
								t.Fatal("End entry ordering differs")
							}
							table.Biomes[i].Weight = want.Entries[i].Weight
						}
					}
					g, err = WithTerraBlenderEnd(ctx, base, cfg)
					if err != nil {
						t.Fatal(err)
					}
				} else {
					cfg, err := BuiltinTerraBlenderProfileWithToggles("26.3", "26.3.0.0.15", dimension, variant.Toggles)
					if err != nil {
						t.Fatal(err)
					}
					want := variant.Profile.Dimensions.Overworld
					if dimension == Nether {
						want = variant.Profile.Dimensions.Nether
					}
					cfg.RegionSize = want.RegionSize
					if len(cfg.Regions) != len(want.Regions) {
						t.Fatal("region count differs")
					}
					for i := range cfg.Regions {
						if cfg.Regions[i].ID != want.Regions[i].ID || cfg.Regions[i].Index != want.Regions[i].Index {
							t.Fatal("region order differs")
						}
						cfg.Regions[i].Weight = want.Regions[i].Weight
					}
					g, err = WithTerraBlender(ctx, base, cfg)
					if err != nil {
						t.Fatal(err)
					}
				}
				return g
			}
			checkTerraBlenderOracle(t, variant.Cases, factory)
			checkTerraBlenderBoundaryHistory(t, variant.BoundaryCases, factory)
		})
	}
}

func TestTerraBlenderReseedPaletteAndAllocations(t *testing.T) {
	ctx := context.Background()
	for _, dim := range []Dimension{Overworld, Nether, End} {
		base := terraBlenderTestGenerator(t, dim, 0)
		before := base.BiomePalette()
		for _, seed := range []int64{-1, 42, 1234567890123456789} {
			rebound, err := base.WithSeed(ctx, seed)
			if err != nil {
				t.Fatal(err)
			}
			fresh := terraBlenderTestGenerator(t, dim, seed)
			a, b, names := rebound.NewSampler(), fresh.NewSampler(), rebound.NewSampler()
			for i := range 128 {
				x, z := i*1099-14389, 993843-i*1583
				got, gok := a.NextNoiseBiomeID(x, 64, z)
				want, wok := b.NextNoiseBiomeID(x, 64, z)
				if got != want || gok != wok {
					t.Fatalf("dim%d seed%d: %v/%v != %v/%v", dim, seed, got, gok, want, wok)
				}
				if name, ok := names.NextNoiseBiome(x, 64, z); !ok || name != rebound.BiomeName(got) {
					t.Fatal("numeric/string palette differs")
				}
			}
		}
		if !reflect.DeepEqual(before, base.BiomePalette()) {
			t.Fatal("reseed mutated palette")
		}
		s := base.NewSampler()
		point := 0
		if got := testing.AllocsPerRun(100, func() { point++; s.NextNoiseBiomeID(point*2017, 64, -point*1091) }); got != 0 {
			t.Fatalf("dim%d: %g allocations per point", dim, got)
		}
	}
}

func TestTerraBlenderEndDuplicateWeights(t *testing.T) {
	ctx := context.Background()
	base, err := New(ctx, Config{Version: "26.3", Dimension: End, Seed: 42})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := BuiltinTerraBlenderEndProfile("26.3", "26.3.0.0.15")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Highlands.Biomes = []TerraBlenderWeightedBiome{{"test:a", 2}, {"test:a", 3}, {"test:b", 4}}
	split, err := WithTerraBlenderEnd(ctx, base, cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Highlands.Biomes = []TerraBlenderWeightedBiome{{"test:a", 5}, {"test:b", 4}}
	merged, err := WithTerraBlenderEnd(ctx, base, cfg)
	if err != nil {
		t.Fatal(err)
	}
	a, b := split.NewSampler(), merged.NewSampler()
	for i := range 4096 {
		x, z := -8192+(i%64)*61, 8192+(i/64)*61
		an, ao := a.NextNoiseBiome(x, 64, z)
		bn, bo := b.NextNoiseBiome(x, 64, z)
		if an != bn || ao != bo {
			t.Fatalf("split adjacent weight changed biome at %d,%d: %s != %s", x, z, an, bn)
		}
	}
}

func TestTerraBlenderRejectsUnknownOrInvalidProfiles(t *testing.T) {
	ctx := context.Background()
	base, err := New(ctx, Config{Version: "26.3", Dimension: Overworld})
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*TerraBlenderConfig){
		func(c *TerraBlenderConfig) { c.Version = "future" },
		func(c *TerraBlenderConfig) { c.Minecraft = "26.2" },
		func(c *TerraBlenderConfig) { c.Dimension = End },
		func(c *TerraBlenderConfig) { c.RegionSize = 1 },
		func(c *TerraBlenderConfig) { c.RegionSize = 7 },
		func(c *TerraBlenderConfig) { c.Regions[1].Index = 7 },
		func(c *TerraBlenderConfig) { c.Regions[1].Weight = -1 },
		func(c *TerraBlenderConfig) { c.Regions[1].Weight = 2147483647 },
		func(c *TerraBlenderConfig) { c.Regions[1].Parameters = nil },
		func(c *TerraBlenderConfig) { c.Regions[1].Parameters[0].Space[0] = [2]int64{1, -1} },
	} {
		cfg, err := BuiltinTerraBlenderProfile("26.3", "26.3.0.0.15", Overworld)
		if err != nil {
			t.Fatal(err)
		}
		change(&cfg)
		if _, err := WithTerraBlender(ctx, base, cfg); err == nil {
			t.Fatal("invalid profile accepted")
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	cfg, _ := BuiltinTerraBlenderProfile("26.3", "26.3.0.0.15", Overworld)
	if _, err := WithTerraBlender(cancelled, base, cfg); err != context.Canceled {
		t.Fatalf("cancelled compilation: %v", err)
	}
}

func BenchmarkTerraBlenderTile26_3(b *testing.B) {
	for _, d := range []struct {
		name string
		dim  Dimension
	}{{"overworld", Overworld}, {"nether", Nether}, {"end", End}} {
		b.Run(d.name, func(b *testing.B) {
			g := terraBlenderTestGenerator(b, d.dim, 42)
			s := g.NewSampler()
			r := Range{X: 8192, Y: 64, Z: -8192, SizeX: 128, SizeY: 1, SizeZ: 128, StepXZ: 4, StepY: 1}
			out := make([]BiomeID, 128*128)
			ctx := context.Background()
			if err := s.GenerateIDs(ctx, r, out); err != nil {
				b.Fatal(err)
			}
			hash := fnv.New64a()
			for _, id := range out {
				fmt.Fprintln(hash, g.BiomeName(id))
			}
			want := map[Dimension]uint64{Overworld: 0xab0ba4dd63717fca, Nether: 0x7d81057eaf752324, End: 0x4bc91a14694ced36}[d.dim]
			if got := hash.Sum64(); got != want {
				b.Fatalf("benchmark reference differs: %016x want %016x", got, want)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := s.GenerateIDs(ctx, r, out); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			hash.Reset()
			for _, id := range out {
				fmt.Fprintln(hash, g.BiomeName(id))
			}
			if got := hash.Sum64(); got != want {
				b.Fatalf("repeated raster differs: %016x want %016x", got, want)
			}
		})
	}
}
