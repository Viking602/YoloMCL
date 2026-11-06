package worldgen_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/worldgen"
)

// TestBatch1AgainstWorld checks temples, swamp huts, buried treasure and
// ocean ruins piece by piece against saved structure starts.
func TestBatch1AgainstWorld(t *testing.T) {
	dirs := os.Getenv("WORLDGEN_VERIFY")
	if dirs == "" {
		t.Skip("set WORLDGEN_VERIFY")
	}
	ruinCfg := map[string]OceanRuinConfig{
		"minecraft:ocean_ruin_cold": {"cold", 0.3, 0.9},
		"minecraft:ocean_ruin_warm": {"warm", 0.3, 0.9},
	}
	total, ok := map[string]int{}, map[string]int{}
	pieces, piecesOK := 0, 0
	shown := 0
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
					if len(s.Pieces) == 0 {
						continue
					}
					cx, cz := int32(s.ChunkX), int32(s.ChunkZ)
					var got, want string
					switch {
					case ScatteredSize[s.ID] != [3]int{}:
						o := ScatteredOrientation(seed, cx, cz)
						b := ScatteredBox(cx, cz, 0, ScatteredSize[s.ID], o)
						bb := s.Pieces[0].BB
						want = fmt.Sprint(o, b[0], b[2], b[3], b[5])
						got = fmt.Sprint(orientByData[fmt.Sprint(s.Pieces[0].Raw["O"])], bb[0], bb[2], bb[3], bb[5])
					case s.ID == "minecraft:buried_treasure":
						x, z := BuriedTreasurePos(cx, cz)
						want = fmt.Sprint(x, z)
						got = fmt.Sprint(s.Pieces[0].BB[0], s.Pieces[0].BB[2])
					case ruinCfg[s.ID] != OceanRuinConfig{}:
						pred := OceanRuin(seed, cx, cz, ruinCfg[s.ID])
						var a, b []string
						for _, p := range pred {
							a = append(a, fmt.Sprint(p.Template, p.Rotation, p.Pos[0], p.Pos[2]))
						}
						for _, pc := range s.Pieces {
							x, _ := pieceInt(pc.Raw, "TPX")
							z, _ := pieceInt(pc.Raw, "TPZ")
							b = append(b, fmt.Sprint(pc.Template, pc.Raw["Rot"], x, z))
						}
						for j := range b {
							pieces++
							if j < len(a) && a[j] == b[j] {
								piecesOK++
							}
						}
						want, got = strings.Join(a, "|"), strings.Join(b, "|")
					default:
						continue
					}
					total[s.ID]++
					if got == want {
						ok[s.ID]++
					} else if shown < 8 {
						shown++
						t.Logf("%s chunk %d,%d\n saved     %s\n predicted %s", s.ID, cx, cz, got, want)
					}
				}
			}
		}
	}
	for k, n := range total {
		t.Logf("%s: %d/%d exact", k, ok[k], n)
	}
	t.Logf("ocean ruin pieces: %d/%d", piecesOK, pieces)
}
