package structure3d_test

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
)

// TestStartSurvey lists structure start ids and their first piece id in
// saves named by STRUCTURE_SURVEY (";"-separated "worldDir|dimName").
func TestStartSurvey(t *testing.T) {
	spec := os.Getenv("STRUCTURE_SURVEY")
	if spec == "" {
		t.Skip("set STRUCTURE_SURVEY")
	}
	for _, entry := range strings.Split(spec, ";") {
		dir, dim, _ := strings.Cut(entry, "|")
		count := map[string]int{}
		paths, _ := mcworld.RegionPaths(dir, dim)
		for _, p := range paths {
			r, err := mcworld.OpenRegion(p)
			if err != nil {
				continue
			}
			for i := 0; i < 1024; i++ {
				c, _ := r.Chunk(i%32, i/32)
				if c == nil {
					continue
				}
				for _, s := range c.Starts() {
					first := "-"
					if len(s.Pieces) > 0 {
						first = s.Pieces[0].ID
					}
					count[fmt.Sprintf("%s [%s]", s.ID, first)]++
				}
			}
		}
		var keys []string
		for k := range count {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		t.Logf("%s %s: %d kinds", dir, dim, len(keys))
		for _, k := range keys {
			t.Logf("  %4d %s", count[k], k)
		}
	}
}
