package worldgen_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/worldgen"
)

// TestStrongholdAgainstWorld compares the stronghold piece tree (id and
// bounding box, in order) with saved starts.
func TestStrongholdAgainstWorld(t *testing.T) {
	dirs := os.Getenv("WORLDGEN_VERIFY")
	if dirs == "" {
		t.Skip("set WORLDGEN_VERIFY")
	}
	trees, treesOK, pieces, piecesOK, altY := 0, 0, 0, 0, 0
	for _, dir := range strings.Split(dirs, ";") {
		seed, err := mcworld.LevelSeed(dir)
		if err != nil {
			continue
		}
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
					if s.ID != "minecraft:stronghold" || len(s.Pieces) == 0 {
						continue
					}
					pred := Stronghold(seed, int32(s.ChunkX), int32(s.ChunkZ), 63, -64)
					if len(pred) > 0 && int32(pred[0].Box[1]) != s.Pieces[0].BB[1] {
						// Some packs give the overworld other noise settings; the
						// tree is unchanged, only moveBelowSeaLevel differs.
						if alt := Stronghold(seed, int32(s.ChunkX), int32(s.ChunkZ), 32, 0); len(alt) > 0 && int32(alt[0].Box[1]) == s.Pieces[0].BB[1] {
							altY++
							pred = alt
						}
					}
					set := map[string]bool{}
					for _, p := range pred {
						set[fmt.Sprint(p.ID, p.Box)] = true
					}
					hit := 0
					for _, pc := range s.Pieces {
						b := pc.BB
						if set[fmt.Sprint(pc.ID, Box{int(b[0]), int(b[1]), int(b[2]), int(b[3]), int(b[4]), int(b[5])})] {
							hit++
						}
					}
					trees++
					pieces += len(s.Pieces)
					piecesOK += hit
					if hit == len(s.Pieces) && len(pred) == len(s.Pieces) {
						treesOK++
					} else {
						first := ""
						for j, pc := range s.Pieces {
							if j < len(pred) {
								b := pred[j].Box
								if fmt.Sprint(pc.ID, pc.BB) != fmt.Sprint(pred[j].ID, [6]int32{int32(b[0]), int32(b[1]), int32(b[2]), int32(b[3]), int32(b[4]), int32(b[5])}) {
									first = fmt.Sprintf("piece %d saved %s %v predicted %s %v", j, pc.ID, pc.BB, pred[j].ID, b)
									break
								}
							}
						}
						t.Logf("%s chunk %d,%d: %d/%d pieces (predicted %d); first diff: %s", dir, s.ChunkX, s.ChunkZ, hit, len(s.Pieces), len(pred), first)
					}
				}
			}
		}
	}
	t.Logf("stronghold trees %d/%d exact, pieces %d/%d (%d placed with sea level 32 / min Y 0)", treesOK, trees, piecesOK, pieces, altY)
}
