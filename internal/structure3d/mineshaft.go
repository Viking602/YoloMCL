package structure3d

import "github.com/Viking602/YoloMCL/internal/worldgen"

// mineshaftFeatureIndex is the mineshaft's position among vanilla 1.21.1's
// UNDERGROUND_STRUCTURES structures in registry order (buried_treasure,
// mineshaft, mineshaft_mesa, trail_ruins, trial_chambers); both variants are
// placed under their own index.
func mineshaftFeatureIndex(mesa bool) int {
	if mesa {
		return 2
	}
	return 1
}

// assembleMineshaft builds the seed-exact mineshaft piece tree and runs each
// piece's vanilla MineshaftPieces.postProcess chunk by chunk with the
// per-chunk decoration random (mineshaft_pieces.go): carved tunnels,
// supports, cobwebs, rails, torches, chest minecarts' rails, spawners,
// plank floors over gaps and log/chain pillars. The terrain the pieces read
// comes from ground; without it everything below the surface height (or
// y 63) counts as solid stone, so terrain-dependent pieces are approximate.
// Pieces touching liquid are skipped and planks only go where the
// OCEAN_FLOOR height is above them, as in vanilla, so nothing floats.
func assembleMineshaft(seed int64, mesa bool, originX, originZ int, height HeightFunc, ground Ground) (*Scene, error) {
	l := worldgen.Mineshaft(seed, int32(originX>>4), int32(originZ>>4), true)
	surface := height
	if surface == nil {
		surface = func(int, int) int { return 63 }
	}
	if mesa {
		l.MesaY(63, surface)
	} else {
		l.MoveBelowSeaLevel(63, -64)
	}
	if ground == nil {
		ground = func(x, y, z int) GroundKind {
			if y < surface(x, z) {
				return GroundSolid
			}
			return GroundAir
		}
	}
	w := newPieceWorld(ground, -64, -64)
	w.maxY = 320
	w.floor = surface
	ms := make([]*msPiece, len(l.Pieces))
	states := make([]*pc, len(l.Pieces))
	pieces := make([]PieceBox, len(l.Pieces))
	for i, p := range l.Pieces {
		ms[i] = &msPiece{kind: p.Kind, mesa: mesa, rails: p.Rails, spider: p.Spider, sections: p.Sections, twoFloor: p.TwoFloors, entrances: p.Entrances}
		dir := p.Dir
		if p.Kind == worldgen.MineshaftRoom || p.Kind == worldgen.MineshaftCrossing {
			dir = -1
		}
		states[i] = &pc{w: w, bb: p.Box, dir: dir}
		pieces[i] = PieceBox{Min: [3]int{p.Box[0], p.Box[1], p.Box[2]}, Max: [3]int{p.Box[3], p.Box[4], p.Box[5]}, Name: "mineshaft:" + p.Kind, Depth: p.Depth}
	}
	runPieces(w, boxes(l.Pieces, func(p *worldgen.MineshaftPiece) worldgen.Box { return p.Box }), states,
		msRand(seed, mineshaftFeatureIndex(mesa)), func(i int, g *pc) { msPost(ms[i], g) })
	s := BuildScene(w.blocks())
	s.LayoutSeeded = true
	s.Carved = w.carved()
	s.Pieces = pieces
	return s, nil
}
