package structure3d_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/structure3d"
)

// TestMonumentPiecesProbe compares createMonumentPieces with saved pieces.
func TestMonumentPiecesProbe(t *testing.T) {
	dir := os.Getenv("MONUMENT_PROBE_DIR")
	if dir == "" {
		t.Skip()
	}
	seed, _ := mcworld.LevelSeed(filepath.Dir(dir))
	paths, _ := filepath.Glob(filepath.Join(dir, "*.mca"))
	shown := 0
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
				if s.ID != "minecraft:monument" || len(s.Pieces) == 0 {
					continue
				}
				pcs := MonumentPieceList(seed, int32(s.ChunkX), int32(s.ChunkZ), 63, 0)
				got := map[string]int{}
				for _, q := range pcs {
					got[q.ID+" "+fmtBox(q.BB)]++
				}
				ok := 0
				var miss []string
				for _, sp := range s.Pieces {
					k := sp.ID + " " + fmtBox6(sp.BB)
					if got[k] > 0 {
						got[k]--
						ok++
					} else {
						miss = append(miss, k)
					}
				}
				t.Logf("monument %d,%d saved %d ours %d matched %d", s.ChunkX, s.ChunkZ, len(s.Pieces), len(pcs), ok)
				if shown < 2 && len(miss) > 0 {
					shown++
					for _, m := range miss[:min(8, len(miss))] {
						t.Logf("  saved-only %s", m)
					}
					n := 0
					for k, v := range got {
						if v > 0 && n < 8 {
							n++
							t.Logf("  ours-only %s", k)
						}
					}
				}
			}
		}
	}
}
func fmtBox(b [6]int) string { return fmt.Sprint(b) }

func fmtBox6(b [6]int32) string {
	return fmt.Sprint([6]int{int(b[0]), int(b[1]), int(b[2]), int(b[3]), int(b[4]), int(b[5])})
}
