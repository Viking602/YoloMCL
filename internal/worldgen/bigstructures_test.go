package worldgen_test

import (
	"os"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/worldgen"
)

// TestMonumentAgainstWorld compares the seeded building box and facing with
// saved monument starts.
func TestMonumentAgainstWorld(t *testing.T) {
	dirs := os.Getenv("WORLDGEN_VERIFY")
	if dirs == "" {
		t.Skip("set WORLDGEN_VERIFY")
	}
	n, boxOK, dirOK := 0, 0, 0
	other := map[string]int{}
	altSea := map[int]int{}
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
					switch s.ID {
					case "minecraft:mansion", "minecraft:end_city":
						other[s.ID]++
					}
					if s.ID != "minecraft:monument" || len(s.Pieces) == 0 {
						continue
					}
					n++
					b := s.Pieces[0].BB
					sea := 63
					if b[1] != 39 {
						sea = int(b[1]) + 24
						altSea[sea]++
					}
					box, d := MonumentStart(seed, int32(s.ChunkX), int32(s.ChunkZ), sea)
					if [6]int32{int32(box[0]), int32(box[1]), int32(box[2]), int32(box[3]), int32(box[4]), int32(box[5])} == b {
						boxOK++
					} else {
						t.Logf("%s %d,%d box saved %v predicted %v", dir, s.ChunkX, s.ChunkZ, b, box)
					}
					if pieceO(s.Pieces[0]) == Data2D[d] {
						dirOK++
					} else {
						t.Logf("%s %d,%d O saved %d predicted %d", dir, s.ChunkX, s.ChunkZ, pieceO(s.Pieces[0]), Data2D[d])
					}
				}
			}
		}
	}
	t.Logf("monuments %d: box %d/%d, facing %d/%d; non-63 sea levels (from saved Y): %v; other big starts seen: %v", n, boxOK, n, dirOK, n, altSea, other)
	if boxOK != n || dirOK != n {
		t.Fail()
	}
}

func pieceO(p mcworld.Piece) int32 {
	switch v := p.Raw["O"].(type) {
	case int32:
		return v
	case int8:
		return int32(v)
	case int16:
		return int32(v)
	case int64:
		return int32(v)
	}
	return -1
}

// TestMansionStartAgainstWorld checks the seeded rotation and entrance
// template origin against each saved woodland mansion.
func TestMansionStartAgainstWorld(t *testing.T) {
	dirs := os.Getenv("WORLDGEN_VERIFY")
	if dirs == "" {
		t.Skip("set WORLDGEN_VERIFY")
	}
	n, rotOK, posOK := 0, 0, 0
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
					if s.ID != "minecraft:mansion" || len(s.Pieces) == 0 {
						continue
					}
					n++
					rot, x, z := MansionEntrance(seed, int32(s.ChunkX), int32(s.ChunkZ))
					e := s.Pieces[0]
					if e.Rotation == rot {
						rotOK++
					}
					if e.Template == "entrance" && rawInt(e, "TPX") == x && rawInt(e, "TPZ") == z {
						posOK++
					} else {
						t.Logf("%s %d,%d saved %s %s TP %d,%d predicted %s %d,%d", dir, s.ChunkX, s.ChunkZ, e.Template, e.Rotation, rawInt(e, "TPX"), rawInt(e, "TPZ"), rot, x, z)
					}
				}
			}
		}
	}
	t.Logf("mansions %d: rotation %d/%d, entrance origin %d/%d", n, rotOK, n, posOK, n)
	if rotOK != n || posOK != n {
		t.Fail()
	}
}

func rawInt(p mcworld.Piece, k string) int {
	v, _ := p.Raw[k].(int32)
	return int(v)
}
