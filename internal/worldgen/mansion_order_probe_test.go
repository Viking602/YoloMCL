package worldgen_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/worldgen"
)

// TestMansionPieceOrderProbe compares the order of MansionAllPieces with the
// saved piece list (vanilla places pieces in list order; later air wins).
func TestMansionPieceOrderProbe(t *testing.T) {
	dir := os.Getenv("MANSION_ORDER_DIR")
	if dir == "" {
		t.Skip()
	}
	seed, _ := mcworld.LevelSeed(dir)
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
				if s.ID != "minecraft:mansion" || len(s.Pieces) == 0 {
					continue
				}
				_, all, _ := MansionAllPieces(seed, int32(s.ChunkX), int32(s.ChunkZ), rawInt(s.Pieces[0], "TPY"))
				var sv, gv []string
				for _, pc := range s.Pieces {
					sv = append(sv, strings.TrimPrefix(pc.Template, "minecraft:woodland_mansion/"))
				}
				for _, a := range all {
					gv = append(gv, strings.TrimPrefix(a.Template, "minecraft:woodland_mansion/"))
				}
				same := 0
				for j := 0; j < min(len(sv), len(gv)) && sv[j] == gv[j]; j++ {
					same++
				}
				t.Logf("mansion %d,%d saved %d pieces, ours %d, same prefix %d", s.ChunkX, s.ChunkZ, len(sv), len(gv), same)
				t.Logf(" entrance %v", s.Pieces[0].Raw)
				t.Logf(" saved head %v", sv[:min(40, len(sv))])
				t.Logf(" ours  head %v", gv[:min(40, len(gv))])
				key := func(n string, rot string, mi string, x, y, z int) string {
					return fmt.Sprintf("%s %s %s %d,%d,%d", n, rot, mi, x, y, z)
				}
				w := map[string]int{}
				for _, pc := range s.Pieces {
					w[key(strings.TrimPrefix(pc.Template, "minecraft:woodland_mansion/"), pc.Rotation, savedMirror(pc), rawInt(pc, "TPX"), rawInt(pc, "TPY"), rawInt(pc, "TPZ"))]++
				}
				for _, a := range all {
					k := key(strings.TrimPrefix(a.Template, "minecraft:woodland_mansion/"), a.Rotation, a.Mirror, a.X, a.Y, a.Z)
					if w[k] > 0 {
						w[k]--
					} else if !isMansionRoomTemplate(a.Template) {
						t.Logf("  ours-only %s", k)
					}
				}
				for k, v := range w {
					if v > 0 && !isMansionRoomTemplate(strings.Fields(k)[0]) {
						t.Logf("  saved-only %s x%d", k, v)
					}
				}
				return
			}
		}
	}
}
