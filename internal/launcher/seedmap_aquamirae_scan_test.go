package launcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
)

// TestAquamiraeStartsScan lists aquamirae structure starts in MCL_AQUA_SCAN_SAVES.
// Opt-in: set MCL_AQUA_SCAN_SAVES=path;path
func TestAquamiraeStartsScan(t *testing.T) {
	raw := os.Getenv("MCL_AQUA_SCAN_SAVES")
	if raw == "" {
		t.Skip("set MCL_AQUA_SCAN_SAVES")
	}
	for _, world := range strings.Split(raw, ";") {
		world = strings.TrimSpace(world)
		if world == "" {
			continue
		}
		count := map[string]int{}
		pirate, aqua, total := 0, 0, 0
		for _, dim := range []string{"", "DIM-1", "DIM1"} {
			paths, err := mcworld.RegionPaths(world, dim)
			if err != nil {
				continue
			}
			for _, path := range paths {
				r, err := mcworld.OpenRegion(path)
				if err != nil {
					continue
				}
				for lx := 0; lx < 32; lx++ {
					for lz := 0; lz < 32; lz++ {
						c, err := r.Chunk(lx, lz)
						if err != nil || c == nil {
							continue
						}
						for _, st := range c.Starts() {
							total++
							id := st.ID
							if !strings.Contains(id, "aquamirae") {
								continue
							}
							aqua++
							count[id]++
							if strings.Contains(id, "pirate") {
								pirate++
							}
							t.Logf("HIT %s %s @ chunk(%d,%d)", filepath.Base(world), id, st.ChunkX, st.ChunkZ)
						}
					}
				}
			}
		}
		t.Logf("SAVE %s aqua=%d pirate=%d starts_seen=%d", filepath.Base(world), aqua, pirate, total)
		for id, n := range count {
			t.Logf("  %s x%d", id, n)
		}
		_ = fmt.Sprintf("%v", count)
	}
}
