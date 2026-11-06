package mcworld

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// TestProbeWorld dumps structure starts and ore counts of a real save.
// MCWORLD_PROBE=<world dir>. Skipped by default.
func TestProbeWorld(t *testing.T) {
	dir := os.Getenv("MCWORLD_PROBE")
	if dir == "" {
		t.Skip("set MCWORLD_PROBE")
	}
	seed, err := LevelSeed(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("seed %d", seed)
	paths, _ := RegionPaths(dir, "")
	kinds := map[string]int{}
	var lines []string
	chunks, full := 0, 0
	ores := map[string]int{}
	for _, p := range paths {
		r, err := OpenRegion(p)
		if err != nil {
			t.Fatal(err)
		}
		for lz := 0; lz < 32; lz++ {
			for lx := 0; lx < 32; lx++ {
				c, err := r.Chunk(lx, lz)
				if err != nil || c == nil {
					continue
				}
				chunks++
				if strings.HasSuffix(c.Status, "full") {
					full++
					if full <= 400 {
						for k, n := range c.CountBlocks(IsOre) {
							ores[k] += n
						}
					}
				}
				for _, s := range c.Starts() {
					kinds[s.ID]++
					if len(lines) < 400 {
						var ps []string
						for i, pc := range s.Pieces {
							if i >= 3 {
								break
							}
							ps = append(ps, fmt.Sprintf("%s %s %s %v", pc.ID, pc.Template, pc.Rotation, pc.BB))
						}
						lines = append(lines, fmt.Sprintf("%s @%d,%d status=%s pieces=%d [%s]", s.ID, s.ChunkX, s.ChunkZ, c.Status, len(s.Pieces), strings.Join(ps, "; ")))
					}
				}
			}
		}
	}
	t.Logf("chunks %d full %d", chunks, full)
	var ks []string
	for k, n := range kinds {
		ks = append(ks, fmt.Sprintf("%s=%d", k, n))
	}
	sort.Strings(ks)
	t.Logf("starts: %s", strings.Join(ks, " "))
	for _, l := range lines {
		t.Log(l)
	}
	var os2 []string
	for k, n := range ores {
		os2 = append(os2, fmt.Sprintf("%s=%d", k, n))
	}
	sort.Strings(os2)
	t.Logf("ores in first 400 full chunks: %s", strings.Join(os2, " "))
}
