package worldgen_test

import (
	"os"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/worldgen"
)

func TestJavaRandomKnownValues(t *testing.T) {
	r := &JavaRandom{}
	r.SetSeed(0)
	if got := r.NextInt(100); got != 60 { // new Random(0).nextInt(100) == 60
		t.Fatalf("nextInt = %d", got)
	}
	r.SetSeed(42)
	if got := r.NextLong(); got != -5025562857975149833 { // new Random(42).nextLong()
		t.Fatalf("nextLong = %d", got)
	}
}

// TestAgainstWorld compares seeded predictions with saved structure starts.
// WORLDGEN_VERIFY=<world dir>.
func TestAgainstWorld(t *testing.T) {
	dir := os.Getenv("WORLDGEN_VERIFY")
	if dir == "" {
		t.Skip("set WORLDGEN_VERIFY")
	}
	seed, err := mcworld.LevelSeed(dir)
	if err != nil {
		t.Fatal(err)
	}
	paths, _ := mcworld.RegionPaths(dir, "")
	ig, igOK, ms, msA, msB := 0, 0, 0, 0, 0
	for _, p := range paths {
		r, err := mcworld.OpenRegion(p)
		if err != nil {
			t.Fatal(err)
		}
		for lz := 0; lz < 32; lz++ {
			for lx := 0; lx < 32; lx++ {
				c, _ := r.Chunk(lx, lz)
				if c == nil {
					continue
				}
				for _, s := range c.Starts() {
					cx, cz := int32(s.ChunkX), int32(s.ChunkZ)
					switch s.ID {
					case "minecraft:igloo":
						ig++
						l := Igloo(seed, cx, cz)
						rot := ""
						for _, pc := range s.Pieces {
							if pc.Template == "minecraft:igloo/top" {
								rot = pc.Rotation
							}
						}
						ok := rot == l.Rotation && len(s.Pieces) == l.Pieces()
						if ok {
							igOK++
						}
						t.Logf("igloo %d,%d real rot=%s pieces=%d | pred %+v pieces=%d ok=%v", cx, cz, rot, len(s.Pieces), l, l.Pieces(), ok)
					case "minecraft:mineshaft", "minecraft:mineshaft_mesa":
						if len(s.Pieces) == 0 {
							continue
						}
						ms++
						bb := s.Pieces[0].BB
						rx, ry, rz := int(bb[3]-bb[0]), int(bb[4]-bb[1]), int(bb[5]-bb[2])
						ax, ay, az := MineshaftRoomSize(seed, cx, cz, true)
						bx, by, bz := MineshaftRoomSize(seed, cx, cz, false)
						if rx == ax && ry == ay && rz == az {
							msA++
						}
						if rx == bx && ry == by && rz == bz {
							msB++
						}
						if ms <= 5 {
							t.Logf("mineshaft %d,%d room %s real %d,%d,%d skip=%d,%d,%d noskip=%d,%d,%d", cx, cz, s.Pieces[0].ID, rx, ry, rz, ax, ay, az, bx, by, bz)
						}
					}
				}
			}
		}
	}
	t.Logf("igloo %d/%d match; mineshaft room size skipDouble %d/%d, noSkip %d/%d", igOK, ig, msA, ms, msB, ms)
}
