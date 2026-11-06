package structure3d

import (
	"strings"

	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// assembleMonument places the seed-exact OceanMonumentPieces building and
// child rooms (room graph + postProcess) at the monument start.
func assembleMonument(seed int64, x, z, yOffset int) *Scene {
	cx, cz := int32(x>>4), int32(z>>4)
	sea := 63
	m, building := MonumentBlocksOffset(seed, cx, cz, sea, yOffset, nil, sea-24+min(yOffset, 0), 320)
	blocks := make([]WorldBlock, 0, len(m))
	for p, st := range m {
		if st.Name == "" || st.Name == "minecraft:air" {
			continue
		}
		blocks = append(blocks, WorldBlock{X: p[0], Y: p[1], Z: p[2], State: st})
	}
	s := BuildScene(blocks)
	s.Seeded = true
	s.Pieces = []PieceBox{{Min: [3]int{building[0], building[1], building[2]}, Max: [3]int{building[3], building[4], building[5]}, Name: "MonumentBuilding"}}
	return s
}

// assembleMansion places every woodland mansion template (rooms, entrance,
// walls, roofs, corridors, carpets) at the seed-exact grid position, rotation
// and mirror from MansionRooms / createMansion.
func assembleMansion(store *Store, seed int64, x, z int, height HeightFunc) (*Scene, error) {
	cx, cz := int32(x>>4), int32(z>>4)
	_, sx, sz, _ := worldgen.RotatedStart(seed, cx, cz)
	y := 70
	if height != nil {
		y = height(sx, sz)
	}
	_, rooms, _ := worldgen.MansionAllPieces(seed, cx, cz, y)
	var blocks []WorldBlock
	for _, rm := range rooms {
		id := rm.Template
		if !strings.Contains(id, ":") {
			id = "minecraft:woodland_mansion/" + id
		}
		if err := appendTemplate(store, id, [3]int{rm.X, rm.Y, rm.Z}, rm.Rotation, rm.Mirror, &blocks); err != nil {
			return nil, err
		}
	}
	if len(blocks) == 0 {
		return nil, &UnsupportedError{Kind: KindEmpty, Reason: "mansion placed no blocks"}
	}
	// Last write wins for overlapping template cells (rooms over walls).
	blocks = dedupeBlocks(blocks)
	s := BuildScene(blocks)
	s.Seeded = true
	return s, nil
}

// assembleEndCity places every end_city template from EndCity (piece boxes
// verified against saves) at the seed-exact position and rotation.
func assembleEndCity(store *Store, seed int64, x, z int, height HeightFunc) (*Scene, error) {
	cx, cz := int32(x>>4), int32(z>>4)
	y := 65
	if height != nil {
		y = height(int(cx)*16+7, int(cz)*16+7)
	}
	pieces := worldgen.EndCity(seed, cx, cz, y)
	var blocks []WorldBlock
	for _, p := range pieces {
		rot := worldgen.Rotations[p.Rot]
		if err := appendTemplate(store, "minecraft:end_city/"+p.Name, p.Pos, rot, "NONE", &blocks); err != nil {
			return nil, err
		}
	}
	if len(blocks) == 0 {
		return nil, &UnsupportedError{Kind: KindEmpty, Reason: "end city placed no blocks"}
	}
	blocks = dedupeBlocks(blocks)
	s := BuildScene(blocks)
	for _, p := range pieces {
		b := p.Box
		s.Pieces = append(s.Pieces, PieceBox{Min: [3]int{b[0], b[1], b[2]}, Max: [3]int{b[3], b[4], b[5]}, Name: p.Name})
	}
	s.Seeded = true
	return s, nil
}

// dedupeBlocks keeps the last WorldBlock at each (x,y,z).
func dedupeBlocks(in []WorldBlock) []WorldBlock {
	m := make(map[[3]int]BlockState, len(in))
	order := make([][3]int, 0, len(in))
	seen := make(map[[3]int]bool, len(in))
	for _, b := range in {
		k := [3]int{b.X, b.Y, b.Z}
		if !seen[k] {
			seen[k] = true
			order = append(order, k)
		}
		m[k] = b.State
	}
	out := make([]WorldBlock, len(order))
	for i, k := range order {
		out[i] = WorldBlock{X: k[0], Y: k[1], Z: k[2], State: m[k]}
	}
	return out
}
