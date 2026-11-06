package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
)

func terrainTestInt(v any) (int, bool) {
	switch t := v.(type) {
	case int32:
		return int(t), true
	case int64:
		return int(t), true
	case int:
		return t, true
	case int16:
		return int(t), true
	case int8:
		return int(t), true
	}
	return 0, false
}

// TestTerrainHeightsVsSave compares the noise-stage solid top with saved
// OCEAN_FLOOR heightmaps of fully generated chunks. Opt-in, read-only:
// MCL_TERRAIN_HEIGHTS="instanceDir|world;...".
func TestTerrainHeightsVsSave(t *testing.T) {
	spec := os.Getenv("MCL_TERRAIN_HEIGHTS")
	if spec == "" {
		t.Skip("set MCL_TERRAIN_HEIGHTS")
	}
	s := testService(t)
	tectonicPackDisabled = os.Getenv("MCL_TERRAIN_NOTECTONIC") != ""
	defer func() { tectonicPackDisabled = false }()
	for _, item := range strings.Split(spec, ";") {
		inst, world, _ := strings.Cut(item, "|")
		root := Root{ID: "th", Path: filepath.Dir(filepath.Dir(inst)), SelectedVersion: filepath.Base(inst)}
		s.cfg.Roots = []Root{root}
		snap, level, err := s.SeedMapWorldgen(context.Background(), root.ID, root.SelectedVersion, world)
		if err != nil {
			t.Fatalf("%s: %v", world, err)
		}
		tect := 0
		for _, d := range snap.Documents {
			if strings.Contains(d.Source, "embedded:tectonic") {
				tect++
			}
		}
		t.Logf("%s: enabled packs %v; %d docs from embedded tectonic", world, snap.EnabledPacks, tect)
		for _, d := range snap.Documents {
			if d.ID == "minecraft:overworld" && d.Kind == "noise_settings" || strings.HasSuffix(d.ID, "noise_router/final_density") || strings.HasSuffix(d.ID, "noise_router/depth") || strings.Contains(string(d.Data), "tectonic:config") || d.Kind == "lithostitched_worldgen_modifier" && !strings.Contains(string(d.Data), "add_") || d.Kind == "density_function" && !strings.HasPrefix(d.ID, "tectonic:") && !strings.HasPrefix(d.ID, "minecraft:") || d.Kind == "noise_settings" {
				t.Logf("doc %s/%s from %s len %d", d.Kind, d.ID, d.Source, len(d.Data))
			}
		}
		if os.Getenv("MCL_TERRAIN_DUMP") != "" {
			for _, d := range snap.Documents {
				if d.Kind == "noise_settings" && d.ID == "minecraft:overworld" {
					os.WriteFile(os.Getenv("MCL_TERRAIN_DUMP"), d.Data, 0o644)
				}
				if strings.Contains(d.Source, "tectonic") && strings.Contains(d.Kind, "density") {
					name := filepath.Join(os.Getenv("MCL_TERRAIN_DUMP")+".d", strings.ReplaceAll(strings.ReplaceAll(d.ID, ":", "_"), "/", "_")+".json")
					os.MkdirAll(filepath.Dir(name), 0o755)
					os.WriteFile(name, d.Data, 0o644)
				}
			}
		}
		sampler, err := NewLocalSampler(context.Background(), snap, level.RandomSeed)
		if err != nil {
			t.Fatal(err)
		}
		const dim = "minecraft:overworld"
		info, ok := sampler.TerrainInfo(dim)
		t.Logf("terrain %v %+v", ok, info)
		for _, is := range sampler.coverage.Issues {
			if strings.Contains(is.Code, "density") || strings.Contains(is.Code, "terrain") || strings.Contains(is.Source, "tectonic") {
				t.Logf("issue %s %s", is.Code, is.Source)
			}
		}
		paths, _ := mcworld.RegionPaths(filepath.Join(inst, "saves", world), "")
		var n, exact, le1, le3, le8 int
		var sum, nb, bok int
		shown := 0
		// Per-region [samples, exact]: mixed-generation saves show up as
		// regions far below the rest.
		reg := map[string]*[2]int{}
		for _, p := range paths {
			r, err := mcworld.OpenRegion(p)
			if err != nil {
				continue
			}
			for i := 0; i < 1024 && n < 60000; i++ {
				c, _ := r.Chunk(i%32, i/32)
				if c == nil {
					continue
				}
				st, _ := c.Root["Status"].(string)
				if !strings.HasSuffix(st, "full") {
					continue
				}
				cx, ok1 := terrainTestInt(c.Root["xPos"])
				cz, ok2 := terrainTestInt(c.Root["zPos"])
				if !ok1 || !ok2 {
					continue
				}
				for _, lz := range []int{3, 11} {
					for _, lx := range []int{3, 11} {
						want, ok := c.Heightmap("OCEAN_FLOOR", lx, lz, -64)
						if !ok {
							continue
						}
						// OCEAN_FLOOR counts feature blocks (trees, huge mushrooms);
						// strip them to reach the noise-stage terrain top.
						for want > -64 && terrainFeatureBlock(c.Block(lx, want-1, lz)) {
							want--
						}
						x, z := cx*16+lx, cz*16+lz
						got, ok := sampler.TerrainSurfaceAt(dim, x, z)
						if !ok {
							continue
						}
						if b, ok := sampler.BiomeAt(dim, x, want, z); ok {
							nb++
							if b == c.Biome(lx, want, lz) {
								bok++
							}
						}
						d := got + 1 - want
						if d < 0 {
							d = -d
						}
						n++
						sum += d
						rt := reg[filepath.Base(p)]
						if rt == nil {
							rt = &[2]int{}
							reg[filepath.Base(p)] = rt
						}
						rt[0]++
						if d == 0 {
							exact++
							rt[1]++
						}
						if d <= 1 {
							le1++
						}
						if d <= 3 {
							le3++
						}
						if d <= 8 {
							le8++
						}
						if d > 8 && shown < 6 {
							shown++
							t.Logf("  @%d,%d saved %d got %d", x, z, want, got+1)
							for y := want - 1; y >= got-2 && y >= want-14; y-- {
								t.Logf("    y%d %s", y, c.Block(lx, y, lz))
							}
						}
					}
				}
			}
		}
		for name, rt := range reg {
			if rt[0] >= 64 {
				t.Logf("  region %s: %d samples, %.1f%% exact", name, rt[0], 100*float64(rt[1])/float64(rt[0]))
			}
		}
		if n == 0 {
			t.Logf("%s: no full chunks", world)
			continue
		}
		t.Logf("biomes %d/%d", bok, nb)
		t.Logf("%s: %d columns exact %d (%.1f%%) <=1 %.1f%% <=3 %.1f%% <=8 %.1f%% mean|d| %.2f", world, n, exact, 100*float64(exact)/float64(n), 100*float64(le1)/float64(n), 100*float64(le3)/float64(n), 100*float64(le8)/float64(n), float64(sum)/float64(n))
	}
}

// terrainFeatureBlock reports feature-placed blocks that OCEAN_FLOOR counts
// but the noise stage never places.
func terrainFeatureBlock(name string) bool {
	for _, s := range []string{"_log", "_leaves", "_wood", "_stem", "mushroom_block", "bamboo", "cactus", "vine", "bee_nest", "pumpkin", "melon", "cocoa", "minecraft:air", "cave_air", "snow", "packed_ice", "blue_ice", "minecraft:ice", "minecraft:water", "snow_block", "_hyphae", "mangrove_roots", "pointed_dripstone", "sugar_cane"} {
		if strings.HasSuffix(name, s) || strings.Contains(name, s+"[") {
			return true
		}
	}
	return false
}
