package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
)

func TestCountStructureStarts(t *testing.T) {
	raw := os.Getenv("MCL_COUNT_SAVES")
	if raw == "" {
		t.Skip("MCL_COUNT_SAVES")
	}
	wantSub := os.Getenv("MCL_COUNT_SUBSTR")
	if wantSub == "" {
		wantSub = "monument,mansion,stronghold,iceandfire,end_city"
	}
	subs := strings.Split(wantSub, ",")
	for _, world := range strings.Split(raw, ";") {
		world = strings.TrimSpace(world)
		if world == "" {
			continue
		}
		count := map[string]int{}
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
							for _, s := range subs {
								if strings.Contains(st.ID, s) {
									count[st.ID]++
									break
								}
							}
						}
					}
				}
			}
		}
		t.Logf("SAVE %s", filepath.Base(world))
		for id, n := range count {
			t.Logf("  %s x%d", id, n)
		}
	}
}
