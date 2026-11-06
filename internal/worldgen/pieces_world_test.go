package worldgen_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/worldgen"
)

// eachStart calls f for every saved start with the given id in a dimension.
func eachStart(dirs, dim, id string, f func(dir string, seed int64, s mcworld.Start)) {
	for _, dir := range strings.Split(dirs, ";") {
		seed, err := mcworld.LevelSeed(dir)
		if err != nil {
			continue
		}
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
					if s.ID == id && len(s.Pieces) > 0 {
						f(dir, seed, s)
					}
				}
			}
		}
	}
}

func boxKey(name string, b [6]int32) string { return fmt.Sprintf("%s %v", name, b) }

func box32(b Box) [6]int32 {
	return [6]int32{int32(b[0]), int32(b[1]), int32(b[2]), int32(b[3]), int32(b[4]), int32(b[5])}
}

func sameMultiset(a, b []string) (bool, string) {
	m := map[string]int{}
	for _, x := range b {
		m[x]++
	}
	for _, x := range a {
		m[x]--
	}
	for k, v := range m {
		if v != 0 {
			return false, fmt.Sprintf("%s (%+d)", k, v)
		}
	}
	return true, ""
}

// TestFortressAgainstWorld compares every saved nether fortress piece (id and
// box, after the 48..70 height move) with Fortress.
func TestFortressAgainstWorld(t *testing.T) {
	dirs := os.Getenv("WORLDGEN_VERIFY")
	if dirs == "" {
		t.Skip("set WORLDGEN_VERIFY")
	}
	n, ok, pieces, piecesOK := 0, 0, 0, 0
	eachStart(dirs, "DIM-1", "minecraft:fortress", func(dir string, seed int64, s mcworld.Start) {
		n++
		var saved, ours []string
		for _, p := range s.Pieces {
			saved = append(saved, boxKey(strings.TrimPrefix(p.ID, "minecraft:"), p.BB))
		}
		for _, p := range Fortress(seed, int32(s.ChunkX), int32(s.ChunkZ), true) {
			id := strings.ToLower(p.ID)
			if id == "nestart" {
				id = "nebcr" // StartPiece saves under the bridge crossing type
			}
			ours = append(ours, boxKey(id, box32(p.Box)))
		}
		pieces += len(saved)
		m := map[string]int{}
		for _, x := range ours {
			m[x]++
		}
		for _, x := range saved {
			if m[x] > 0 {
				m[x]--
				piecesOK++
			}
		}
		if same, diff := sameMultiset(ours, saved); same {
			ok++
		} else if n-ok <= 3 {
			t.Logf("%s fortress %d,%d: saved %d ours %d, first diff %s", dir, s.ChunkX, s.ChunkZ, len(saved), len(ours), diff)
		}
	})
	t.Logf("SAVE fortresses %d: exact %d/%d, pieces %d/%d", n, ok, n, piecesOK, pieces)
	if ok != n {
		t.Fail()
	}
}

// TestEndCityAgainstWorld compares saved end city templates, rotations and
// boxes with EndCity, taking the base height from the saved first piece.
func TestEndCityAgainstWorld(t *testing.T) {
	dirs := os.Getenv("WORLDGEN_VERIFY")
	if dirs == "" {
		t.Skip("set WORLDGEN_VERIFY")
	}
	n, ok, pieces, piecesOK, ships := 0, 0, 0, 0, 0
	eachStart(dirs, "DIM1", "minecraft:end_city", func(dir string, seed int64, s mcworld.Start) {
		n++
		var saved, ours []string
		for _, p := range s.Pieces {
			name := p.Template[strings.LastIndex(p.Template, "/")+1:]
			if name == "ship" {
				ships++
			}
			saved = append(saved, boxKey(name+" "+p.Rotation, p.BB))
		}
		for _, p := range EndCity(seed, int32(s.ChunkX), int32(s.ChunkZ), int(s.Pieces[0].BB[1])) {
			ours = append(ours, boxKey(p.Name+" "+Rotations[p.Rot], box32(p.Box)))
		}
		pieces += len(saved)
		m := map[string]int{}
		for _, x := range ours {
			m[x]++
		}
		for _, x := range saved {
			if m[x] > 0 {
				m[x]--
				piecesOK++
			}
		}
		if same, diff := sameMultiset(ours, saved); same {
			ok++
		} else if n-ok <= 3 {
			t.Logf("%s end city %d,%d: saved %d ours %d, first diff %s", dir, s.ChunkX, s.ChunkZ, len(saved), len(ours), diff)
		}
	})
	t.Logf("SAVE end cities %d (%d ships): exact %d/%d, pieces %d/%d", n, ships, ok, n, piecesOK, pieces)
	if ok != n {
		t.Fail()
	}
}
