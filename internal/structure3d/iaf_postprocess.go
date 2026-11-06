package structure3d

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// iafPostProcessBox applies DragonRoostPiece / SirenIslandPiece.postProcess
// to s: once the start chunk is decorated, the piece box is reset to the
// drawn roost shell or island. The radius comes from the chunk's
// SURFACE_STRUCTURES decoration random, seeded with the structure's index
// in the alphabetical surface_structures list. The pre-decoration single
// block box stays in s.StartPieces.
func iafPostProcessBox(store *Store, s *Scene, typ string, seed int64, cx, cz int) {
	s.StartPieces = s.Pieces
	idx := surfaceStructureIndex(store, typ)
	if idx < 0 || len(s.Pieces) == 0 {
		return
	}
	p := s.Pieces[0].Min
	x, y, z := p[0], p[1], p[2]
	r := worldgen.NewDecorationRandom(seed, cx, cz, idx, worldgen.StepSurfaceStructures)
	var b PieceBox
	if typ == "iceandfire:siren_island" {
		up := 1 + int(r.NextInt(4)) // getRadius(up, up) = min(up, 10)
		b = PieceBox{Min: [3]int{x - up, y, z - up}, Max: [3]int{x + up, y + up + 1, z + up}, Name: typ}
	} else {
		sr := iafShellRadius(12 + int(r.NextInt(8))) // generateShell
		b = PieceBox{Min: [3]int{x - sr, y, z - sr}, Max: [3]int{x + sr, y + 3, z + sr}, Name: typ}
	}
	s.Pieces = []PieceBox{b}
	s.Min, s.Max = b.Min, b.Max
}

// iafShellRadius is (int)Math.sqrt(getCircularArea(r, r/5)) with Java's
// float arithmetic: floor(((2r + r/5) * 0.333f + 0.5f)^2).
func iafShellRadius(r int) int {
	v := float32(2*r+r/5)*float32(0.333) + float32(0.5)
	d := float64(v)
	return int(math.Sqrt(math.Floor(d * d)))
}

var surfIdxCache sync.Map // *Store -> map[string]int

// surfaceStructureIndex is id's position in the store's sorted list of
// structures with step surface_structures, or -1.
func surfaceStructureIndex(store *Store, id string) int {
	if store == nil {
		return -1
	}
	if m, ok := surfIdxCache.Load(store); ok {
		if i, ok := m.(map[string]int)[id]; ok {
			return i
		}
		return -1
	}
	seen := map[string]bool{}
	var ids []string
	for p := range store.List("data/") {
		rest := strings.TrimPrefix(p, "data/")
		i := strings.Index(rest, "/worldgen/structure/")
		if i < 0 || !strings.HasSuffix(p, ".json") {
			continue
		}
		name := strings.TrimSuffix(rest[i+len("/worldgen/structure/"):], ".json")
		key := rest[:i] + ":" + name
		if seen[key] || strings.Contains(name, "/") {
			continue
		}
		b, err := store.Read(p)
		if err != nil {
			continue
		}
		var d struct {
			Step string `json:"step"`
		}
		if json.Unmarshal(b, &d) != nil || d.Step != "surface_structures" {
			continue
		}
		seen[key] = true
		ids = append(ids, key)
	}
	sort.Strings(ids)
	m := make(map[string]int, len(ids))
	for k, v := range ids {
		m[v] = k
	}
	surfIdxCache.Store(store, m)
	if i, ok := m[id]; ok {
		return i
	}
	return -1
}

// decoIndex is the SURFACE_STRUCTURES decoration feature index of id in
// this install (mods add structures before it alphabetically), falling
// back to vanilla 1.21.1's index for vanillaID.
func decoIndex(store *Store, id, vanillaID string) int {
	if i := surfaceStructureIndex(store, id); i >= 0 {
		return i
	}
	if vanillaID == "minecraft:stronghold" {
		return 19
	}
	return templeIndex[vanillaID]
}
