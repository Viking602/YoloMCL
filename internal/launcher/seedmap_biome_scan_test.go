package launcher

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
)

// TestBiomeScan lists the biomes of a namespace found in saved full chunks.
// MCL_BIOME_SCAN="<instance>|<world>;..." (read-only); MCL_BIOME_NS defaults
// to biomesoplenty. Used to pick saves for TestSurfaceRulesSave.
func TestBiomeScan(t *testing.T) {
	spec := os.Getenv("MCL_BIOME_SCAN")
	if spec == "" {
		t.Skip("MCL_BIOME_SCAN not set")
	}
	ns := os.Getenv("MCL_BIOME_NS")
	if ns == "" {
		ns = "biomesoplenty"
	}
	for _, entry := range strings.Split(spec, ";") {
		inst, world, ok := strings.Cut(entry, "|")
		if !ok {
			continue
		}
		paths, _ := mcworld.RegionPaths(filepath.Join(inst, "saves", world), "")
		counts := map[string]int{}
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
					x, z := (r.X*32+lx)*16+8, (r.Z*32+lz)*16+8
					if b := ch.Biome(x, 64, z); strings.HasPrefix(b, ns+":") {
						counts[b]++
					}
				}
			}
		}
		var keys []string
		for k, n := range counts {
			keys = append(keys, k+"="+scanItoa(n))
		}
		sort.Strings(keys)
		t.Logf("%s | %s | %d regions: %v", filepath.Base(inst), world, len(paths), keys)
	}
}

func scanItoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for ; n > 0; n /= 10 {
		s = string(rune('0'+n%10)) + s
	}
	return s
}
