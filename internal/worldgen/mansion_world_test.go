package worldgen_test

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/worldgen"
)

func isMansionRoomTemplate(t string) bool {
	return strings.HasPrefix(t, "1x1_") || strings.HasPrefix(t, "1x2_") || strings.HasPrefix(t, "2x2_")
}

func savedMirror(p mcworld.Piece) string {
	if s, ok := p.Raw["Mi"].(string); ok && s != "" {
		return s
	}
	return "NONE"
}

// TestMansionRoomsAgainstWorld compares every predicted room template, its
// origin, rotation and mirror with the pieces saved for each woodland mansion.
// Directories come from WORLDGEN_MANSIONS (";"-separated), else WORLDGEN_VERIFY.
func TestMansionRoomsAgainstWorld(t *testing.T) {
	dirs := os.Getenv("WORLDGEN_MANSIONS")
	if dirs == "" {
		dirs = os.Getenv("WORLDGEN_VERIFY")
	}
	if dirs == "" {
		t.Skip("set WORLDGEN_MANSIONS or WORLDGEN_VERIFY")
	}
	// Vanilla picks the upper-floor 1x1 room from 4 templates; a template-list
	// datapack (e.g. Lithostitched) restores 1x1_b5. WORLDGEN_MANSION_B1X1 fixes
	// the count; otherwise a save whose mansion holds a 1x1_b5 piece uses 5.
	forced := 0
	if v := os.Getenv("WORLDGEN_MANSION_B1X1"); v != "" {
		fmt.Sscan(v, &forced)
	}
	n, exact, roomsTotal, roomsOK := 0, 0, 0, 0
	for _, dir := range strings.Split(dirs, ";") {
		seed, err := mcworld.LevelSeed(dir)
		if err != nil {
			continue
		}
		// The template list belongs to the world, so a save whose mansions
		// hold any 1x1_b5 piece uses 5 variants for all of them.
		var starts []mcworld.Start
		b5 := false
		paths, _ := mcworld.RegionPaths(dir, "")
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
					if s.ID == "minecraft:mansion" && len(s.Pieces) > 0 && s.Pieces[0].Template == "entrance" {
						starts = append(starts, s)
						b5 = b5 || hasB5(s.Pieces)
					}
				}
			}
		}
		{
			{
				for _, s := range starts {
					n++
					y := rawInt(s.Pieces[0], "TPY")
					opt := MansionOptions{SecondFloor1x1Variants: forced}
					if forced == 0 && b5 {
						opt.SecondFloor1x1Variants = 5
					}
					_, rooms, _ := MansionRoomsWith(seed, int32(s.ChunkX), int32(s.ChunkZ), y, opt)
					want := map[string]int{}
					if os.Getenv("MANSION_DUMP") != "" && n == 1 {
						for _, pc := range s.Pieces {
							t.Logf("piece %q %s %v", pc.Template, pc.Rotation, pc.Raw)
						}
					}
					for _, pc := range s.Pieces {
						name := strings.TrimPrefix(pc.Template, "minecraft:woodland_mansion/")
						if isMansionRoomTemplate(name) {
							want[fmt.Sprintf("%s %s %s %d,%d,%d", name, pc.Rotation, savedMirror(pc), rawInt(pc, "TPX"), rawInt(pc, "TPY"), rawInt(pc, "TPZ"))]++
						}
					}
					got := map[string]int{}
					for _, rm := range rooms {
						got[fmt.Sprintf("%s %s %s %d,%d,%d", rm.Template, rm.Rotation, rm.Mirror, rm.X, rm.Y, rm.Z)]++
					}
					ok := 0
					var miss, extra []string
					for k, v := range want {
						roomsTotal += v
						m := min(v, got[k])
						ok += m
						if v > m {
							miss = append(miss, k)
						}
					}
					for k, v := range got {
						if v > want[k] {
							extra = append(extra, k)
						}
					}
					roomsOK += ok
					if len(miss) == 0 && len(extra) == 0 {
						exact++
					} else {
						sort.Strings(miss)
						sort.Strings(extra)
						t.Logf("%s chunk %d,%d: %d/%d rooms; missing %v; extra %v", dir, s.ChunkX, s.ChunkZ, ok, len(want), miss, extra)
					}
				}
			}
		}
	}
	t.Logf("mansions %d: exact room sets %d/%d, rooms %d/%d", n, exact, n, roomsOK, roomsTotal)
	if n == 0 || exact != n {
		t.Fail()
	}
}

// hasB5 reports whether a saved mansion uses the 1x1_b5 room template.
func hasB5(pieces []mcworld.Piece) bool {
	for _, pc := range pieces {
		if strings.HasSuffix(pc.Template, "1x1_b5") {
			return true
		}
	}
	return false
}
