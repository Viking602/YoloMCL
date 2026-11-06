package structure3d

import "github.com/Viking602/YoloMCL/internal/worldgen"

// assembleStronghold builds the seed-exact stronghold piece tree (verified
// piece-for-piece against saves) and runs every piece's vanilla
// StrongholdPieces.postProcess (stronghold_pieces.go) chunk by chunk with
// the SURFACE_STRUCTURES-step decoration random (index 19): the mossy/cracked/infested brick
// selector, doors, torches, cobwebs, library shelves, chests, portal frame
// eyes and the silverfish spawner. Walls skip air (caves the stronghold
// cuts through); without a ground sampler everything below the surface
// height counts as solid.
func assembleStronghold(seed int64, index int, x, z int, height HeightFunc, ground Ground) *Scene {
	pieces := worldgen.Stronghold(seed, int32(x>>4), int32(z>>4), 63, -64)
	if ground == nil {
		surface := height
		if surface == nil {
			surface = func(int, int) int { return 63 }
		}
		ground = func(x, y, z int) GroundKind {
			if y < surface(x, z) {
				return GroundSolid
			}
			return GroundAir
		}
	}
	w := newPieceWorld(ground, -64, -64)
	w.maxY = 320
	states := make([]*pc, len(pieces))
	boxesOut := make([]PieceBox, len(pieces))
	for i, p := range pieces {
		states[i] = &pc{w: w, bb: p.Box, dir: p.Dir, needChest: true}
		boxesOut[i] = PieceBox{Min: [3]int{p.Box[0], p.Box[1], p.Box[2]}, Max: [3]int{p.Box[3], p.Box[4], p.Box[5]}, Name: p.ID, Depth: p.Depth}
	}
	runPieces(w, boxes(pieces, func(p *worldgen.StrongholdPiece) worldgen.Box { return p.Box }), states,
		decoRand(seed, index, worldgen.StepSurfaceStructures), func(i int, g *pc) { strongholdPost(pieces[i], g) })
	s := BuildScene(w.blocks())
	s.LayoutSeeded = true
	s.Carved = w.carved()
	s.Pieces = boxesOut
	return s
}

func decoRand(seed int64, index, step int) func(cx, cz int) pieceRand {
	return func(cx, cz int) pieceRand {
		return &dr{worldgen.NewDecorationRandom(seed, cx, cz, index, step)}
	}
}

func strongholdPost(p *worldgen.StrongholdPiece, g *pc) {
	switch p.ID {
	case worldgen.ShStraight:
		g.shStraight(p)
	case worldgen.ShPrison:
		g.shPrisonHall(p)
	case worldgen.ShLeft:
		g.shLeftTurn(p)
	case worldgen.ShRight:
		g.shRightTurn(p)
	case worldgen.ShRoom:
		g.shRoomCrossing(p)
	case worldgen.ShStraightS:
		g.shStraightStairsDown(p)
	case worldgen.ShStairs, worldgen.ShStart:
		g.shStairsDown(p)
	case worldgen.ShFive:
		g.shFiveCrossing(p)
	case worldgen.ShChest:
		g.shChestCorridor(p)
	case worldgen.ShLibrary:
		g.shLibrary(p)
	case worldgen.ShPortal:
		g.shPortalRoom(p)
	case worldgen.ShFiller:
		g.shFillerCorridor(p)
	}
}

// smoothSel is StrongholdPieces.SmoothStoneSelector.
type smoothSel struct{}

func (smoothSel) next(r pieceRand, x, y, z int, wall bool) BlockState {
	if !wall {
		return caveAir
	}
	f := r.nextFloat()
	switch {
	case f < 0.2:
		return st("cracked_stone_bricks")
	case f < 0.5:
		return st("mossy_stone_bricks")
	case f < 0.55:
		return st("infested_stone_bricks")
	}
	return st("stone_bricks")
}

// smallDoor is StrongholdPiece.generateSmallDoor.
func (g *pc) smallDoor(kind string, x, y, z int) {
	sb := st("stone_bricks")
	frame := func() {
		g.place(sb, x, y, z)
		g.place(sb, x, y+1, z)
		g.place(sb, x, y+2, z)
		g.place(sb, x+1, y+2, z)
		g.place(sb, x+2, y+2, z)
		g.place(sb, x+2, y+1, z)
		g.place(sb, x+2, y, z)
	}
	switch kind {
	case "OPENING":
		g.box(x, y, z, x+2, y+2, z, caveAir, caveAir, false)
	case "WOOD_DOOR":
		frame()
		g.place(st("oak_door"), x+1, y, z)
		g.place(st("oak_door").with("half", "upper"), x+1, y+1, z)
	case "GRATES":
		g.place(caveAir, x+1, y, z)
		g.place(caveAir, x+1, y+1, z)
		w := st("iron_bars").with("west", "true")
		ew := st("iron_bars").with("east", "true").with("west", "true")
		e := st("iron_bars").with("east", "true")
		g.place(w, x, y, z)
		g.place(w, x, y+1, z)
		g.place(ew, x, y+2, z)
		g.place(ew, x+1, y+2, z)
		g.place(ew, x+2, y+2, z)
		g.place(e, x+2, y+1, z)
		g.place(e, x+2, y, z)
	case "IRON_DOOR":
		frame()
		g.place(st("iron_door"), x+1, y, z)
		g.place(st("iron_door").with("half", "upper"), x+1, y+1, z)
		g.place(st("stone_button").with("facing", "north"), x+2, y+1, z+1)
		g.place(st("stone_button").with("facing", "south"), x+2, y+1, z-1)
	}
}
