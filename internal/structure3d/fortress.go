package structure3d

import "github.com/Viking602/YoloMCL/internal/worldgen"

// assembleFortress builds the seeded nether fortress block for block:
// worldgen.Fortress mirrors NetherFortressStructure.generatePieces
// (including the move into Y 48..70) and every piece runs its vanilla
// postProcess (fortress_pieces.go) chunk by chunk, like
// StructureStart.placeInChunk. Support pillars (fillColumnDown) run down to
// the ground sampler's first solid block; without one they stop at the
// lava sea floor (y 31), which is approximate.
func assembleFortress(seed int64, x, z int, ground Ground) *Scene {
	pieces := worldgen.Fortress(seed, int32(x>>4), int32(z>>4), true)
	w := newPieceWorld(ground, 0, 31)
	states := make([]*pc, len(pieces))
	for i, p := range pieces {
		states[i] = &pc{w: w, bb: p.Box, dir: p.Dir, needChest: p.Chest, selfSeed: p.Seed}
	}
	runPieces(w, boxes(pieces, func(p *worldgen.FortressPiece) worldgen.Box { return p.Box }), states, nil, func(i int, g *pc) {
		fortressPost(pieces[i].ID, g)
	})
	s := BuildScene(w.blocks())
	s.LayoutSeeded = true
	s.Carved = w.carved()
	return s
}

func fortressPost(id string, g *pc) {
	switch id {
	case worldgen.NeStart, worldgen.NeBridgeCross:
		g.postBridgeCrossing()
	case worldgen.NeBridge:
		g.postBridgeStraight()
	case worldgen.NeBridgeEnd:
		g.postBridgeEndFiller()
	case worldgen.NeRoomCross:
		g.postRoomCrossing()
	case worldgen.NeStairsRoom:
		g.postStairsRoom()
	case worldgen.NeThrone:
		g.postMonsterThrone()
	case worldgen.NeEntrance:
		g.postCastleEntrance()
	case worldgen.NeCorridor:
		g.postCastleSmallCorridorPiece()
	case worldgen.NeCorridorX:
		g.postCastleSmallCorridorCrossingPiece()
	case worldgen.NeCorridorR:
		g.postCastleSmallCorridorRightTurnPiece()
	case worldgen.NeCorridorL:
		g.postCastleSmallCorridorLeftTurnPiece()
	case worldgen.NeCorridorStep:
		g.postCastleCorridorStairsPiece()
	case worldgen.NeBalcony:
		g.postCastleCorridorTBalconyPiece()
	case worldgen.NeStalkRoom:
		g.postCastleStalkRoom()
	}
}

func boxes[T any](ps []T, f func(T) worldgen.Box) []worldgen.Box {
	out := make([]worldgen.Box, len(ps))
	for i, p := range ps {
		out[i] = f(p)
	}
	return out
}

// runPieces is StructureStart.placeInChunk over every chunk the start
// covers: each piece that intersects the chunk post-processes with the
// chunk's column as its clip box and the chunk's decoration random.
func runPieces(w *pieceWorld, bbs []worldgen.Box, states []*pc, rnd func(cx, cz int) pieceRand, post func(i int, g *pc)) {
	if len(bbs) == 0 {
		return
	}
	u := bbs[0]
	for _, b := range bbs[1:] {
		u = worldgen.Box{min(u[0], b[0]), min(u[1], b[1]), min(u[2], b[2]), max(u[3], b[3]), max(u[4], b[4]), max(u[5], b[5])}
	}
	for cz := u[2] >> 4; cz <= u[5]>>4; cz++ {
		for cx := u[0] >> 4; cx <= u[3]>>4; cx++ {
			clip := worldgen.Box{cx * 16, w.minY, cz * 16, cx*16 + 15, w.maxY - 1, cz*16 + 15}
			var r pieceRand
			if rnd != nil {
				r = rnd(cx, cz)
			}
			for i, b := range bbs {
				if !b.Intersects(clip) {
					continue
				}
				g := states[i]
				g.clip, g.rnd = clip, r
				post(i, g)
			}
		}
	}
}
