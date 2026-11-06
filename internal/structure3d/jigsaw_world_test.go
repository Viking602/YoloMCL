package structure3d_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/structure3d"
)

// TestJigsawAgainstWorld lays out every saved jigsaw start with PlaceJigsaw
// and compares the piece multiset (template, rotation, bounding box).
// WORLDGEN_JIGSAW is ";"-separated "worldDir|clientJar[|dim[|flatY[|instanceRoot]]]" entries; dim is
// "" (overworld) or DIM-1, flatY is a superflat world's first free Y. Terrain comes from the save's WORLD_SURFACE
// heightmap, except that a projected start is aligned to its saved Y, so
// terrain-matching pieces are reported separately rather than failed.
func TestJigsawAgainstWorld(t *testing.T) {
	spec := os.Getenv("WORLDGEN_JIGSAW")
	if spec == "" {
		t.Skip("set WORLDGEN_JIGSAW")
	}
	only := os.Getenv("WORLDGEN_JIGSAW_ONLY")
	type tally struct{ n, exact, terrain, terrainExact, pieces, piecesOK, rawBB int }
	stats := map[string]*tally{}
	shown := 0
	for _, entry := range strings.Split(spec, ";") {
		parts := strings.Split(entry, "|")
		if len(parts) < 2 {
			continue
		}
		dir, jar, dimName := parts[0], parts[1], ""
		if len(parts) > 2 {
			dimName = parts[2]
		}
		dim := OverworldHeight
		if dimName == "DIM-1" {
			dim = DimHeight{MinY: 0, Height: 256}
		}
		seed, err := mcworld.LevelSeed(dir)
		if err != nil {
			t.Logf("%s: %v", dir, err)
			continue
		}
		store := &Store{}
		if err := store.OpenJar("game", jar); err != nil {
			t.Fatalf("%s: %v", jar, err)
		}
		// Optional 5th field: the instance root. Its mods/ jars and the
		// world's datapacks layer over vanilla in game order.
		if len(parts) > 4 && parts[4] != "" {
			var mods int
			if before := os.Getenv("WORLDGEN_JIGSAW_MODS_BEFORE"); before != "" {
				mods = openModsBefore(t, store, parts[4]+"/mods", before)
			} else {
				mods = OpenModsDir(store, parts[4]+"/mods")
			}
			packs := OpenWorldDatapacks(store, "world", dir)
			t.Logf("%s: %d mod jars, datapacks %v (level.dat order %v)", dir, mods, packs, WorldDatapackOrder(dir))
		}
		// WORLDGEN_JIGSAW_EDITS names a JSON list of PoolEdit to try (code-side
		// pool changes ported from mod bytecode).
		if f := os.Getenv("WORLDGEN_JIGSAW_EDITS"); f == "auto" {
			t.Logf("%d known mod pool edits", ApplyKnownModPoolEdits(store))
		} else if f != "" {
			var edits []PoolEdit
			if b, err := os.ReadFile(f); err == nil && json.Unmarshal(b, &edits) == nil {
				store.AddPoolEdits(edits...)
				t.Logf("%d pool edits from %s", len(edits), f)
			}
		}
		if os.Getenv("WORLDGEN_JIGSAW_LITHO") == "1" {
			store.SetLithostitchedJigsaw(true)
			t.Logf("Lithostitched jigsaw generator on")
		}
		heights := map[[2]int]*mcworld.Chunk{}
		var starts []mcworld.Start
		dataVersion := map[[2]int]any{}
		paths, _ := mcworld.RegionPaths(dir, dimName)
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
				heights[[2]int{c.X, c.Z}] = c
				for _, s := range c.Starts() {
					if len(s.Pieces) == 0 || s.Pieces[0].ID != "minecraft:jigsaw" {
						continue
					}
					if only != "" && !strings.Contains(s.ID, only) {
						continue
					}
					starts = append(starts, s)
					dataVersion[[2]int{s.ChunkX, s.ChunkZ}] = fmt.Sprint(c.Root["DataVersion"], " ", c.Status)
				}
			}
		}
		flat, isFlat := 0, false
		if len(parts) > 3 {
			_, err := fmt.Sscan(parts[3], &flat)
			isFlat = err == nil
		}
		surface := func(x, z int) int {
			if isFlat {
				return flat
			}
			c := heights[[2]int{x >> 4, z >> 4}]
			if c == nil {
				return 64
			}
			if h, ok := c.Heightmap("WORLD_SURFACE", x&15, z&15, dim.MinY); ok {
				return h
			}
			return 64
		}
		for _, s := range starts {
			want := map[string]int{}
			for _, p := range s.Pieces {
				want[pieceKey(p.Template, p.Rotation, p.BB)]++
			}
			// Align a projected start: place once with zero terrain, then
			// shift the first height answer by the saved start offset.
			probe, err := PlaceJigsaw(store, s.ID, seed, s.ChunkX, s.ChunkZ, func(int, int) int { return 0 }, dim)
			if err != nil {
				t.Errorf("%s @%d,%d: %v", s.ID, s.ChunkX, s.ChunkZ, err)
				continue
			}
			first := true
			delta := int(s.Pieces[0].BB[1]) - probe.Pieces[0].BB[1]
			free := func(x, z int) int {
				if first {
					first = false
					return delta
				}
				return surface(x, z)
			}
			if !probe.TerrainDependent || probe.Pieces[0].BB[1] == int(s.Pieces[0].BB[1]) {
				free = surface
			}
			got, err := PlaceJigsaw(store, s.ID, seed, s.ChunkX, s.ChunkZ, free, dim)
			if err != nil {
				t.Errorf("%s: %v", s.ID, err)
				continue
			}
			sid := s.ID + " @ " + dir
			st := stats[sid]
			if st == nil {
				st = &tally{}
				stats[sid] = st
			}
			st.n++
			ok := 0
			left := map[string]int{}
			for k, v := range want {
				left[k] = v
			}
			var extra []string
			for _, p := range got.Pieces {
				loc := p.Location
				if p.Kind != "single" {
					loc = ""
				}
				k := pieceKey(loc, RotationName(p.Rotation), box32(p.BB))
				raw := pieceKey(loc, RotationName(p.Rotation), box32(p.RawBB))
				if left[k] > 0 {
					left[k]--
					ok++
				} else if raw != k && left[raw] > 0 {
					// Same piece; this save kept the box without the
					// expansion-hack headroom (seen on some village saves).
					left[raw]--
					ok++
					st.rawBB++
				} else {
					extra = append(extra, k)
				}
			}
			st.pieces += len(s.Pieces)
			st.piecesOK += ok
			exact := ok == len(s.Pieces) && len(got.Pieces) == len(s.Pieces)
			if got.TerrainDependent {
				st.terrain++
				if exact {
					st.terrainExact++
				}
			}
			if exact {
				st.exact++
				continue
			}
			if shown < 12 {
				shown++
				var missing []string
				for k, v := range left {
					for ; v > 0; v-- {
						missing = append(missing, k)
					}
				}
				sort.Strings(missing)
				sort.Strings(extra)
				t.Logf("%s @%d,%d (DataVersion %v) terrain=%v: saved %d, got %d, matched %d\n  first missing: %v\n  first extra:   %v", s.ID, s.ChunkX, s.ChunkZ, dataVersion[[2]int{s.ChunkX, s.ChunkZ}], got.TerrainDependent, len(s.Pieces), len(got.Pieces), ok, head(missing, 4), head(extra, 4))
				if os.Getenv("WORLDGEN_JIGSAW_SEQ") == "1" {
					n := min(len(s.Pieces), len(got.Pieces))
					for k := 0; k < n; k++ {
						a, b := s.Pieces[k], got.Pieces[k]
						if a.Template != b.Location || a.Rotation != RotationName(b.Rotation) || box32(b.BB) != a.BB {
							for j := max(0, k-2); j < min(n, k+3); j++ {
								t.Logf("  seq %d saved %s %s %v | got %s %s %v", j, s.Pieces[j].Template, s.Pieces[j].Rotation, s.Pieces[j].BB, got.Pieces[j].Location, RotationName(got.Pieces[j].Rotation), got.Pieces[j].BB)
							}
							break
						}
					}
				}
			}
		}
	}
	ids := make([]string, 0, len(stats))
	for id := range stats {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		st := stats[id]
		t.Logf("%s\n    starts %3d exact %3d (terrain-dependent %d, exact %d) pieces %d/%d (%d saved without expansion headroom)", id, st.n, st.exact, st.terrain, st.terrainExact, st.piecesOK, st.pieces, st.rawBB)
		if st.exact < st.n-st.terrain {
			t.Errorf("%s: %d terrain-independent starts not exact", id, st.n-st.terrain-(st.exact-st.terrainExact))
		}
	}
}

func head(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func box32(b [6]int) [6]int32 {
	return [6]int32{int32(b[0]), int32(b[1]), int32(b[2]), int32(b[3]), int32(b[4]), int32(b[5])}
}

func pieceKey(loc, rot string, bb [6]int32) string {
	if os.Getenv("WORLDGEN_JIGSAW_NOY") == "1" {
		// Layout check without heights: chunks saved before terrain
		// (structure_starts status) have no heightmaps to replay.
		bb[1], bb[4] = 0, 0
	}
	return fmt.Sprintf("%s %s %v", loc, rot, bb)
}

// openModsBefore is OpenModsDir limited to jars whose file creation time is
// before the RFC 3339 time before: an estimate of the mod set when a save's
// chunks were generated (a jar upgraded later reads as added later).
func openModsBefore(t *testing.T, s *Store, dir, before string) int {
	cut, err := time.Parse(time.RFC3339, before)
	if err != nil {
		t.Fatalf("WORLDGEN_JIGSAW_MODS_BEFORE: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".jar") {
			continue
		}
		if c, ok := fileCreated(filepath.Join(dir, e.Name())); ok && c.Before(cut) {
			names = append(names, e.Name())
		}
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	n := 0
	for _, name := range names {
		if s.OpenJar("mod:"+name, filepath.Join(dir, name)) == nil {
			n++
		}
	}
	return n
}
