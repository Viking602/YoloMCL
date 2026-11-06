package launcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
)

// TestSurfaceRulesSave compares traced mod surface rules with a real save,
// read-only: at the noise-stage solid top of each sampled column in a
// mod-namespace biome, the predicted rule block against the saved block.
// MCL_SURFACE_SAVE="<instance dir>|<world name>"; MCL_SURFACE_NS (default
// biomesoplenty); MCL_SEEDMAP_MAPPINGS as in TestSeedMapOpenProfile.
func TestSurfaceRulesSave(t *testing.T) {
	spec := os.Getenv("MCL_SURFACE_SAVE")
	if spec == "" {
		t.Skip("set MCL_SURFACE_SAVE")
	}
	ns := os.Getenv("MCL_SURFACE_NS")
	if ns == "" {
		ns = "biomesoplenty"
	}
	s := testService(t)
	if src := os.Getenv("MCL_SEEDMAP_MAPPINGS"); src != "" {
		dst := filepath.Join(s.dir, "cache", "game-mappings")
		os.MkdirAll(dst, 0o755)
		entries, _ := os.ReadDir(src)
		for _, e := range entries {
			if b, err := os.ReadFile(filepath.Join(src, e.Name())); err == nil {
				os.WriteFile(filepath.Join(dst, e.Name()), b, 0o644)
			}
		}
	}
	inst, world, _ := strings.Cut(spec, "|")
	root := Root{ID: "surface", Path: filepath.Dir(filepath.Dir(inst)), SelectedVersion: filepath.Base(inst)}
	s.cfg.Roots = []Root{root}
	ctx := context.Background()
	snap, level, err := s.SeedMapWorldgen(ctx, root.ID, root.SelectedVersion, world)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range snap.RegionPrograms {
		if p.Result != nil {
			t.Logf("program %s: %d surface rule sets err=%q", p.ModID, len(p.Result.SurfaceRules), p.Result.SurfaceRulesError)
		}
	}
	if drop := os.Getenv("MCL_SURFACE_DROP"); drop != "" { // hypothesis runs: save generated without these mods
		kept := snap.RegionPrograms[:0]
		for _, p := range snap.RegionPrograms {
			if !strings.Contains(","+drop+",", ","+p.ModID+",") {
				kept = append(kept, p)
			}
		}
		snap.RegionPrograms = kept
		mods := snap.Mods[:0]
		for _, m := range snap.Mods {
			if !strings.Contains(","+drop+",", ","+m.ID+",") {
				mods = append(mods, m)
			}
		}
		snap.Mods = mods
	}
	sampler, err := NewLocalSampler(ctx, snap, level.RandomSeed)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("MCL_SURFACE_BIOMES") != "" {
		for _, is := range sampler.dimensions["minecraft:overworld"].issues {
			t.Logf("issue %s: %.300s", is.Code, is.Source)
		}
		surfaceBiomeAgreement(t, sampler, inst, world)
		return
	}
	const dim = "minecraft:overworld"
	for _, is := range sampler.dimensions[dim].issues {
		t.Logf("issue %s: %.600s", is.Code, is.Source)
	}
	if !sampler.SurfaceRulesActive(dim) {
		t.Fatal("no surface rules compiled for the overworld")
	}
	t.Logf("seed %d rule namespaces %v, %d blocks", level.RandomSeed, sampler.dimensions[dim].surface.Namespaces(), len(sampler.SurfaceRuleBlocks(dim)))
	info, _ := sampler.TerrainInfo(dim)
	paths, _ := mcworld.RegionPaths(filepath.Join(inst, "saves", world), "")
	sort.Strings(paths)
	type tally struct{ match, miss, vanilla, terrain int }
	per := map[string]*tally{}
	pairs := map[string]int{}
	sub := make([]TerrainSubstance, info.Height)
	out := make([]string, info.Height)
	columns := 0
	limit := 40000 // per-biome cap below keeps common biomes from crowding out rule-covered ones
	for _, path := range paths {
		r, err := mcworld.OpenRegion(path)
		if err != nil {
			continue
		}
		for lz := 0; lz < 32 && columns < limit; lz++ {
			for lx := 0; lx < 32 && columns < limit; lx++ {
				ch, _ := r.Chunk(lx, lz)
				if ch == nil || (ch.Status != "minecraft:full" && ch.Status != "full") {
					continue
				}
				cx, cz := r.X*32+lx, r.Z*32+lz
				for dz := 1; dz < 16; dz += 5 {
					for dx := 1; dx < 16; dx += 5 {
						x, z := cx*16+dx, cz*16+dz
						solidTop, top, ok := sampler.TerrainColumn(dim, x, z, sub)
						if !ok || solidTop < info.MinY {
							continue
						}
						biome := ch.Biome(x, solidTop, z)
						if !strings.HasPrefix(biome, ns+":") {
							continue
						}
						tl := per[biome]
						if tl == nil {
							tl = &tally{}
							per[biome] = tl
						}
						if tl.match+tl.miss+tl.vanilla+tl.terrain >= 400 {
							continue
						}
						columns++
						sampler.SurfaceColumn(dim, x, z, sub, top, out)
						pred := out[solidTop-info.MinY]
						actual := ch.Block(x, solidTop, z)
						switch {
						case actual == "minecraft:air" || actual == "minecraft:cave_air" || actual == "minecraft:water":
							tl.terrain++
						case pred == "":
							tl.vanilla++
						case pred == actual:
							tl.match++
						default:
							tl.miss++
							pairs[biome+" "+pred+" -> "+actual]++
							near := map[string]bool{}
							for oy := -4; oy <= 4; oy += 4 {
								for oz := -4; oz <= 4; oz += 4 {
									for ox := -4; ox <= 4; ox += 4 {
										if (x+ox)>>4 == cx && (z+oz)>>4 == cz {
											near[ch.Biome(x+ox, solidTop+oy, z+oz)] = true
										}
									}
								}
							}
							predicted, _ := sampler.BiomeAt(dim, x, solidTop, z)
							t.Logf("miss at %d %d %d: quart-predicted %s, saved quarts nearby %v", x, solidTop, z, predicted, near)
						}
					}
				}
			}
		}
	}
	var keys []string
	total := tally{}
	for k, v := range per {
		keys = append(keys, k)
		total.match += v.match
		total.miss += v.miss
		total.vanilla += v.vanilla
		total.terrain += v.terrain
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := per[k]
		t.Logf("%-40s match %4d miss %4d vanilla-rule %4d terrain-diff %4d", k, v.match, v.miss, v.vanilla, v.terrain)
	}
	var pk []string
	for k, n := range pairs {
		pk = append(pk, fmt.Sprintf("%5d %s", n, k))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(pk)))
	for i, p := range pk {
		if i >= 25 {
			break
		}
		t.Logf("miss %s", p)
	}
	t.Logf("TOTAL columns %d: match %d miss %d vanilla-rule %d terrain-diff %d", columns, total.match, total.miss, total.vanilla, total.terrain)
	if total.match+total.miss == 0 {
		t.Fatal("no rule-covered columns compared")
	}
}
