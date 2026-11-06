package structure3d

import "github.com/Viking602/YoloMCL/internal/worldgen"

var orientDir = map[string]int{"north": worldgen.DirNorth, "east": worldgen.DirEast, "south": worldgen.DirSouth, "west": worldgen.DirWest}

// pieceBuilder writes blocks in a StructurePiece's local frame.
type pieceBuilder struct {
	p      worldgen.MineshaftPiece
	blocks map[[3]int]BlockState
}

func newPieceBuilder(box worldgen.Box, orientation string) *pieceBuilder {
	return &pieceBuilder{p: worldgen.MineshaftPiece{Box: box, Dir: orientDir[orientation]}, blocks: map[[3]int]BlockState{}}
}

func (b *pieceBuilder) set(x, y, z int, n string) {
	wx, wy, wz := b.p.World(x, y, z)
	if n == "" {
		delete(b.blocks, [3]int{wx, wy, wz})
		return
	}
	b.blocks[[3]int{wx, wy, wz}] = BlockState{Name: "minecraft:" + n}
}

func (b *pieceBuilder) box(x0, y0, z0, x1, y1, z1 int, n string) {
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			for z := z0; z <= z1; z++ {
				b.set(x, y, z, n)
			}
		}
	}
}

// SwampHutBlocks returns the hut's blocks for a piece box and orientation.
func SwampHutBlocks(box worldgen.Box, o string) map[[3]int]BlockState {
	b := newPieceBuilder(box, o)
	pl, log := "spruce_planks", "oak_log"
	b.box(1, 1, 1, 5, 1, 7, pl)
	b.box(1, 4, 2, 5, 4, 7, pl)
	b.box(2, 1, 0, 4, 1, 0, pl)
	b.box(2, 2, 2, 3, 3, 2, pl)
	b.box(1, 2, 3, 1, 3, 6, pl)
	b.box(5, 2, 3, 5, 3, 6, pl)
	b.box(2, 2, 7, 4, 3, 7, pl)
	for _, c := range [][2]int{{1, 2}, {5, 2}, {1, 7}, {5, 7}} {
		b.box(c[0], 0, c[1], c[0], 3, c[1], log)
	}
	b.set(2, 3, 2, "oak_fence")
	b.set(3, 3, 7, "oak_fence")
	b.set(1, 3, 4, "")
	b.set(5, 3, 4, "")
	b.set(5, 3, 5, "")
	b.set(1, 3, 5, "potted_red_mushroom")
	b.set(3, 2, 6, "crafting_table")
	b.set(4, 2, 6, "cauldron")
	b.set(1, 2, 1, "oak_fence")
	b.set(5, 2, 1, "oak_fence")
	b.box(0, 4, 1, 6, 4, 1, "spruce_stairs")
	b.box(0, 4, 2, 0, 4, 7, "spruce_stairs")
	b.box(6, 4, 2, 6, 4, 7, "spruce_stairs")
	b.box(0, 4, 8, 6, 4, 8, "spruce_stairs")
	for _, zz := range []int{2, 7} {
		for _, xx := range []int{1, 5} {
			b.box(xx, -3, zz, xx, -1, zz, log) // fillColumnDown reaches the ground
		}
	}
	return b.blocks
}

// assembleBuriedTreasure shows the seed-placed chest (BuriedTreasurePieces
// column chunk+9,9) below the surface with its sand cap.
func assembleBuriedTreasure(seed int64, x, z int, height HeightFunc) *Scene {
	wx, wz := worldgen.BuriedTreasurePos(int32(x>>4), int32(z>>4))
	y := 60
	if height != nil {
		y = height(wx, wz) - 3
	}
	bl := []WorldBlock{{X: wx, Y: y, Z: wz, State: BlockState{Name: "minecraft:chest"}}}
	for dx := -1; dx <= 1; dx++ {
		for dz := -1; dz <= 1; dz++ {
			bl = append(bl, WorldBlock{X: wx + dx, Y: y - 1, Z: wz + dz, State: BlockState{Name: "minecraft:sandstone"}})
			bl = append(bl, WorldBlock{X: wx + dx, Y: y + 1, Z: wz + dz, State: BlockState{Name: "minecraft:sand"}})
		}
	}
	s := BuildScene(bl)
	s.Seeded = true
	return s
}

// assembleOceanRuin places the seed-exact ruin templates (including clusters)
// from OceanRuinPieces. Integrity decay is not simulated; cold ruins show the
// brick layer only.
func assembleOceanRuin(store *Store, cfg worldgen.OceanRuinConfig, seed int64, x, z int, height HeightFunc) (*Scene, error) {
	var blocks []WorldBlock
	var pieces []PieceBox
	for _, p := range worldgen.OceanRuin(seed, int32(x>>4), int32(z>>4), cfg) {
		t, err := LoadTemplate(store, p.Template)
		if err != nil {
			return nil, err
		}
		y := 40
		if height != nil {
			y = height(p.Pos[0], p.Pos[2]) + 1 // OCEAN_FLOOR_WG is the first free Y
		}
		a := worldgen.Transform([3]int{0, 0, 0}, p.Rotation, [3]int{})
		b := worldgen.Transform([3]int{t.Size[0] - 1, 0, t.Size[2] - 1}, p.Rotation, [3]int{})
		pieces = append(pieces, PieceBox{Name: p.Template,
			Min: [3]int{p.Pos[0] + min(a[0], b[0]), y, p.Pos[2] + min(a[2], b[2])},
			Max: [3]int{p.Pos[0] + max(a[0], b[0]), y + t.Size[1] - 1, p.Pos[2] + max(a[2], b[2])}})
		if p.Integrity < 0.75 {
			continue
		}
		for _, bl := range t.Blocks {
			w := worldgen.Transform([3]int{bl.X, bl.Y, bl.Z}, p.Rotation, [3]int{})
			blocks = append(blocks, WorldBlock{X: p.Pos[0] + w[0], Y: y + w[1], Z: p.Pos[2] + w[2], State: rotateState(bl.State, p.Rotation)})
		}
	}
	s := BuildScene(blocks)
	s.Seeded = true
	s.Pieces = pieces
	return s, nil
}
