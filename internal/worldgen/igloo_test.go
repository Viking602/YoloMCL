package worldgen_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/worldgen"
)

func pieceInt(raw map[string]any, key string) (int, bool) {
	switch v := raw[key].(type) {
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case int:
		return v, true
	}
	return 0, false
}

// TestIglooPiecesAgainstWorld compares full predicted igloo piece lists
// (template, rotation, X/Z, Y relative to the top piece) with saved starts.
// WORLDGEN_VERIFY=<world dir>[;<world dir>...]
func TestIglooPiecesAgainstWorld(t *testing.T) {
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
		total, exact := 0, 0
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
						if s.ID != "minecraft:igloo" {
							continue
						}
						total++
						want := IglooPieces(seed, int32(s.ChunkX), int32(s.ChunkZ))
						got := map[string]int{}
						topY, predTop := 0, 0
						for _, pc := range s.Pieces {
							if pc.Template == "minecraft:igloo/top" {
								topY, _ = pieceInt(pc.Raw, "TPY")
							}
						}
						for _, w := range want {
							if w.Template == "minecraft:igloo/top" {
								predTop = w.Pos[1]
							}
						}
						for _, pc := range s.Pieces {
							x, _ := pieceInt(pc.Raw, "TPX")
							y, _ := pieceInt(pc.Raw, "TPY")
							z, _ := pieceInt(pc.Raw, "TPZ")
							got[fmt.Sprintf("%s|%s|%d|%d|%d", pc.Template, pc.Rotation, x, y-topY, z)]++
						}
						ok := len(s.Pieces) == len(want)
						for _, w := range want {
							k := fmt.Sprintf("%s|%s|%d|%d|%d", w.Template, w.Rotation, w.Pos[0], w.Pos[1]-predTop, w.Pos[2])
							if got[k] == 0 {
								ok = false
								t.Logf("%s chunk %d,%d missing %s; saved %v", dir, s.ChunkX, s.ChunkZ, k, got)
							}
							got[k]--
						}
						if ok {
							exact++
						}
					}
				}
			}
		}
		t.Logf("igloo full piece layout: %d/%d exact in %s", exact, total, dir)
		if exact != total {
			t.Errorf("igloo layout mismatches in %s", dir)
		}
	}
}

func splitList(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ';' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	return out
}
