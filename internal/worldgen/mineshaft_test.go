package worldgen_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/worldgen"
)

func savedKind(id string) string {
	id = strings.ToLower(id)
	for _, k := range []string{"corridor", "crossing", "room", "stairs"} {
		if strings.Contains(id, k) {
			return k
		}
	}
	return id
}

func pieceKeys(kinds []string, boxes [][6]int, dy int) map[string]int {
	m := map[string]int{}
	for i, b := range boxes {
		m[fmt.Sprintf("%s|%d,%d,%d,%d,%d,%d", kinds[i], b[0], b[1]-dy, b[2], b[3], b[4]-dy, b[5])]++
	}
	return m
}

// TestMineshaftAgainstWorld compares the full seeded mineshaft piece tree
// (kind + bounding box) with saved structure starts.
func TestMineshaftAgainstWorld(t *testing.T) {
	dirs := os.Getenv("WORLDGEN_VERIFY")
	if dirs == "" {
		t.Skip("set WORLDGEN_VERIFY")
	}
	for _, dir := range splitList(dirs) {
		seed, err := mcworld.LevelSeed(dir)
		if err != nil {
			t.Fatal(err)
		}
		paths, _ := mcworld.RegionPaths(dir, "")
		total, treeOK, absOK, pieceTotal, pieceHit := 0, 0, 0, 0, 0
		ids := map[string]bool{}
		flagTotal, flagOK, flagBad := 0, 0, 0
		shown := 0
		for _, p := range paths {
			r, err := mcworld.OpenRegion(p)
			if err != nil {
				continue
			}
			for lz := 0; lz < 32; lz++ {
				for lx := 0; lx < 32; lx++ {
					c, _ := r.Chunk(lx, lz)
					if c == nil {
						continue
					}
					for _, s := range c.Starts() {
						if (s.ID != "minecraft:mineshaft" && s.ID != "minecraft:mineshaft_mesa") || len(s.Pieces) == 0 {
							continue
						}
						total++
						flagSaved := map[string]string{}
						savedMin0 := 1 << 30
						for _, pc := range s.Pieces {
							savedMin0 = min(savedMin0, int(pc.BB[1]))
						}
						var sk []string
						var sb [][6]int
						savedMin := 1 << 30
						for _, pc := range s.Pieces {
							ids[pc.ID] = true
							if savedKind(pc.ID) == "corridor" || savedKind(pc.ID) == "crossing" {
								flagSaved[fmt.Sprintf("%d,%d,%d,%d", pc.BB[0], int(pc.BB[1])-savedMin0, pc.BB[2], pc.BB[3])] = fmt.Sprint(pc.Raw["hr"], pc.Raw["sc"], pc.Raw["Num"], pc.Raw["tf"])
							}
							sk = append(sk, savedKind(pc.ID))
							b := [6]int{int(pc.BB[0]), int(pc.BB[1]), int(pc.BB[2]), int(pc.BB[3]), int(pc.BB[4]), int(pc.BB[5])}
							sb = append(sb, b)
							savedMin = min(savedMin, b[1])
						}
						l := Mineshaft(seed, int32(s.ChunkX), int32(s.ChunkZ), true)
						var pk []string
						var pb [][6]int
						predMin := 1 << 30
						for _, p := range l.Pieces {
							pk = append(pk, p.Kind)
							pb = append(pb, [6]int(p.Box))
							predMin = min(predMin, p.Box[1])
						}
						got := pieceKeys(sk, sb, savedMin)
						want := pieceKeys(pk, pb, predMin)
						hit := 0
						for k, n := range want {
							hit += min(n, got[k])
						}
						pieceTotal += len(sb)
						pieceHit += hit
						if hit == len(sb) && len(sb) == len(pb) {
							treeOK++
							for _, p := range l.Pieces {
								if p.Kind != MineshaftCorridor && p.Kind != MineshaftCrossing {
									continue
								}
								flagTotal++
								v := flagSaved[fmt.Sprintf("%d,%d,%d,%d", p.Box[0], p.Box[1]-predMin, p.Box[2], p.Box[3])]
								if p.Kind == MineshaftCorridor && v == fmt.Sprint(b2i(p.Rails), b2i(p.Spider), p.Sections, nil) || p.Kind == MineshaftCrossing && v == fmt.Sprint(nil, nil, nil, b2i(p.TwoFloors)) {
									flagOK++
								} else if flagBad < 3 {
									flagBad++
									t.Logf("flags %s saved %q pred %+v", p.Kind, v, *p)
								}
							}
							if s.ID == "minecraft:mineshaft" {
								l.MoveBelowSeaLevel(63, -64)
								if l.Pieces[0].Box[1] == sb[0][1] {
									absOK++
								}
							}
						} else if shown < 3 {
							shown++
							t.Logf("%s chunk %d,%d saved %d pieces, predicted %d, matched %d; saved[0..2]=%v %v pred[0..2]=%v %v", s.ID, s.ChunkX, s.ChunkZ, len(sb), len(pb), hit, sk[:min(3, len(sk))], sb[:min(3, len(sb))], pk[:min(3, len(pk))], pb[:min(3, len(pb))])
						}
					}
				}
			}
		}
		t.Logf("%s: mineshaft trees exact %d/%d, pieces %d/%d, absolute Y (normal type) %d, corridor/crossing flags %d/%d; ids %v", dir, treeOK, total, pieceHit, pieceTotal, absOK, flagOK, flagTotal, ids)
	}
}
func b2i(b bool) int8 {
	if b {
		return 1
	}
	return 0
}
