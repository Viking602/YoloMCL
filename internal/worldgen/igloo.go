package worldgen

// PiecePlacement is one template piece as StructureTemplate places it.
type PiecePlacement struct {
	Template string
	Rotation string
	Pos      [3]int // templatePosition (before terrain post-processing)
	Pivot    [3]int // rotation pivot (StructurePlaceSettings.setRotationPivot)
}

// Transform mirrors StructureTemplate.transform for Mirror.NONE.
func Transform(local [3]int, rotation string, pivot [3]int) [3]int {
	x, y, z := local[0], local[1], local[2]
	px, pz := pivot[0], pivot[2]
	switch rotation {
	case "COUNTERCLOCKWISE_90":
		return [3]int{px - pz + z, y, px + pz - x}
	case "CLOCKWISE_90":
		return [3]int{px + pz - z, y, pz - px + x}
	case "CLOCKWISE_180":
		return [3]int{px + px - x, y, pz + pz - z}
	}
	return local
}

// World returns the world position of a template-local block.
func (p PiecePlacement) World(local [3]int) [3]int {
	t := Transform(local, p.Rotation, p.Pivot)
	return [3]int{p.Pos[0] + t[0], p.Pos[1] + t[1], p.Pos[2] + t[2]}
}

var (
	iglooPivots  = map[string][3]int{"minecraft:igloo/top": {3, 5, 5}, "minecraft:igloo/middle": {1, 3, 1}, "minecraft:igloo/bottom": {3, 6, 7}}
	iglooOffsets = map[string][3]int{"minecraft:igloo/top": {0, 0, 0}, "minecraft:igloo/middle": {2, -3, 4}, "minecraft:igloo/bottom": {0, -3, -2}}
)

// IglooPieces mirrors IglooStructure.generatePieces + IglooPieces.addPieces:
// start (chunk min X, 90, chunk min Z), random rotation, optional basement.
func IglooPieces(seed int64, cx, cz int32) []PiecePlacement {
	l := Igloo(seed, cx, cz)
	start := [3]int{int(cx) * 16, 90, int(cz) * 16}
	piece := func(template string, down int) PiecePlacement {
		o := iglooOffsets[template]
		return PiecePlacement{Template: template, Rotation: l.Rotation, Pivot: iglooPivots[template], Pos: [3]int{start[0] + o[0], start[1] + o[1] - down, start[2] + o[2]}}
	}
	var out []PiecePlacement
	if l.Basement {
		out = append(out, piece("minecraft:igloo/bottom", l.Depth*3))
		for j := 0; j < l.Depth-1; j++ {
			out = append(out, piece("minecraft:igloo/middle", j*3))
		}
	}
	return append(out, piece("minecraft:igloo/top", 0))
}
