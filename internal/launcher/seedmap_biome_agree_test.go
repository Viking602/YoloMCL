package launcher

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
)

// surfaceBiomeAgreement compares predicted noise biomes with the saved
// quart biomes of every full chunk (one sample per quart column at y=64),
// read-only. Run through TestSurfaceRulesSave with MCL_SURFACE_BIOMES=1.
func surfaceBiomeAgreement(t *testing.T, sampler *LocalSampler, inst, world string) {
	const dim = "minecraft:overworld"
	paths, _ := mcworld.RegionPaths(filepath.Join(inst, "saves", world), "")
	pairs := map[string]int{}
	same, total := 0, 0
	type cell struct{ x, z int }
	var bad []cell
	for _, path := range paths {
		r, err := mcworld.OpenRegion(path)
		if err != nil {
			continue
		}
		for lz := 0; lz < 32; lz++ {
			for lx := 0; lx < 32; lx++ {
				ch, _ := r.Chunk(lx, lz)
				if ch == nil || (ch.Status != "minecraft:full" && ch.Status != "full") {
					continue
				}
				for qz := 0; qz < 4; qz++ {
					for qx := 0; qx < 4; qx++ {
						x, z := (r.X*32+lx)*16+qx*4, (r.Z*32+lz)*16+qz*4
						saved := ch.Biome(x, 64, z)
						pred, _ := sampler.BiomeAt(dim, x, 64, z)
						total++
						if saved == pred {
							same++
							continue
						}
						pairs[pred+" -> "+saved]++
						bad = append(bad, cell{x, z})
					}
				}
			}
		}
	}
	t.Logf("biome agreement at y=64: %d / %d quarts (%.3f%%)", same, total, 100*float64(same)/float64(max(total, 1)))
	var keys []string
	for k, n := range pairs {
		keys = append(keys, fmt.Sprintf("%6d %s", n, k))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	for i, k := range keys {
		if i == 25 {
			break
		}
		t.Logf("predicted -> saved %s", k)
	}
	// Clusters of mismatches, by 64-block tile.
	tiles := map[[2]int]int{}
	for _, c := range bad {
		tiles[[2]int{c.x >> 6, c.z >> 6}]++
	}
	var tk []string
	for k, n := range tiles {
		tk = append(tk, fmt.Sprintf("%4d tile x%d..%d z%d..%d", n, k[0]*64, k[0]*64+63, k[1]*64, k[1]*64+63))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(tk)))
	for i, k := range tk {
		if i == 15 {
			break
		}
		t.Logf("mismatch %s", k)
	}
}
