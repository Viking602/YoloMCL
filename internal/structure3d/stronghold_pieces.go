package structure3d

// Code generated from StrongholdPieces postProcess (vanilla 1.21.1) by
// j2go.py, completed by hand. DO NOT EDIT.

import (
	"fmt"

	"github.com/Viking602/YoloMCL/internal/worldgen"
)

func (g *pc) shStraight(p *worldgen.StrongholdPiece) {
	g.boxSel(0, 0, 0, 4, 4, 6, true, smoothSel{})
	g.smallDoor(p.Door, 1, 1, 0)
	g.smallDoor("OPENING", 1, 1, 6)
	var8 := st("wall_torch").with("facing", "east")
	var9 := st("wall_torch").with("facing", "west")
	g.maybeBlock(0.1, 1, 2, 1, var8)
	g.maybeBlock(0.1, 3, 2, 1, var9)
	g.maybeBlock(0.1, 1, 2, 5, var8)
	g.maybeBlock(0.1, 3, 2, 5, var9)
	if p.Flags[0] {
		g.box(0, 1, 2, 0, 3, 4, caveAir, caveAir, false)
	}

	if p.Flags[1] {
		g.box(4, 1, 2, 4, 3, 4, caveAir, caveAir, false)
	}

}

func (g *pc) shPrisonHall(p *worldgen.StrongholdPiece) {
	g.boxSel(0, 0, 0, 8, 4, 10, true, smoothSel{})
	g.smallDoor(p.Door, 1, 1, 0)
	g.box(1, 1, 10, 3, 3, 10, caveAir, caveAir, false)
	g.boxSel(4, 1, 1, 4, 3, 1, false, smoothSel{})
	g.boxSel(4, 1, 3, 4, 3, 3, false, smoothSel{})
	g.boxSel(4, 1, 7, 4, 3, 7, false, smoothSel{})
	g.boxSel(4, 1, 9, 4, 3, 9, false, smoothSel{})

	for var8 := 1; var8 <= 3; var8++ {
		g.place((st("iron_bars").with("north", "true")).with("south", "true"), 4, var8, 4)
		g.place(((st("iron_bars").with("north", "true")).with("south", "true")).with("east", "true"), 4, var8, 5)
		g.place((st("iron_bars").with("north", "true")).with("south", "true"), 4, var8, 6)
		g.place((st("iron_bars").with("west", "true")).with("east", "true"), 5, var8, 5)
		g.place((st("iron_bars").with("west", "true")).with("east", "true"), 6, var8, 5)
		g.place((st("iron_bars").with("west", "true")).with("east", "true"), 7, var8, 5)
	}

	g.place((st("iron_bars").with("north", "true")).with("south", "true"), 4, 3, 2)
	g.place((st("iron_bars").with("north", "true")).with("south", "true"), 4, 3, 8)
	var10 := st("iron_door").with("facing", "west")
	var9 := (st("iron_door").with("facing", "west")).with("half", "upper")
	g.place(var10, 4, 1, 2)
	g.place(var9, 4, 2, 2)
	g.place(var10, 4, 1, 8)
	g.place(var9, 4, 2, 8)
}

func (g *pc) shLeftTurn(p *worldgen.StrongholdPiece) {
	g.boxSel(0, 0, 0, 4, 4, 4, true, smoothSel{})
	g.smallDoor(p.Door, 1, 1, 0)
	var8 := g.dir
	if var8 != worldgen.DirNorth && var8 != worldgen.DirEast {
		g.box(4, 1, 1, 4, 3, 3, caveAir, caveAir, false)
	} else {
		g.box(0, 1, 1, 0, 3, 3, caveAir, caveAir, false)
	}

}

func (g *pc) shRightTurn(p *worldgen.StrongholdPiece) {
	g.boxSel(0, 0, 0, 4, 4, 4, true, smoothSel{})
	g.smallDoor(p.Door, 1, 1, 0)
	var8 := g.dir
	if var8 != worldgen.DirNorth && var8 != worldgen.DirEast {
		g.box(0, 1, 1, 0, 3, 3, caveAir, caveAir, false)
	} else {
		g.box(4, 1, 1, 4, 3, 3, caveAir, caveAir, false)
	}

}

func (g *pc) shRoomCrossing(p *worldgen.StrongholdPiece) {
	g.boxSel(0, 0, 0, 10, 6, 10, true, smoothSel{})
	g.smallDoor(p.Door, 4, 1, 0)
	g.box(4, 1, 10, 6, 3, 10, caveAir, caveAir, false)
	g.box(0, 1, 4, 0, 3, 6, caveAir, caveAir, false)
	g.box(10, 1, 4, 10, 3, 6, caveAir, caveAir, false)
	var var8 int
	switch p.Kind {
	case 0:
		g.place(st("stone_bricks"), 5, 1, 5)
		g.place(st("stone_bricks"), 5, 2, 5)
		g.place(st("stone_bricks"), 5, 3, 5)
		g.place(st("wall_torch").with("facing", "west"), 4, 3, 5)
		g.place(st("wall_torch").with("facing", "east"), 6, 3, 5)
		g.place(st("wall_torch").with("facing", "south"), 5, 3, 4)
		g.place(st("wall_torch").with("facing", "north"), 5, 3, 6)
		g.place(st("smooth_stone_slab"), 4, 1, 4)
		g.place(st("smooth_stone_slab"), 4, 1, 5)
		g.place(st("smooth_stone_slab"), 4, 1, 6)
		g.place(st("smooth_stone_slab"), 6, 1, 4)
		g.place(st("smooth_stone_slab"), 6, 1, 5)
		g.place(st("smooth_stone_slab"), 6, 1, 6)
		g.place(st("smooth_stone_slab"), 5, 1, 4)
		g.place(st("smooth_stone_slab"), 5, 1, 6)
	case 1:
		for var8 = 0; var8 < 5; var8++ {
			g.place(st("stone_bricks"), 3, 1, 3+var8)
			g.place(st("stone_bricks"), 7, 1, 3+var8)
			g.place(st("stone_bricks"), 3+var8, 1, 3)
			g.place(st("stone_bricks"), 3+var8, 1, 7)
		}

		g.place(st("stone_bricks"), 5, 1, 5)
		g.place(st("stone_bricks"), 5, 2, 5)
		g.place(st("stone_bricks"), 5, 3, 5)
		g.place(st("water"), 5, 4, 5)
	case 2:
		for var8 = 1; var8 <= 9; var8++ {
			g.place(st("cobblestone"), 1, 3, var8)
			g.place(st("cobblestone"), 9, 3, var8)
		}

		for var8 = 1; var8 <= 9; var8++ {
			g.place(st("cobblestone"), var8, 3, 1)
			g.place(st("cobblestone"), var8, 3, 9)
		}

		g.place(st("cobblestone"), 5, 1, 4)
		g.place(st("cobblestone"), 5, 1, 6)
		g.place(st("cobblestone"), 5, 3, 4)
		g.place(st("cobblestone"), 5, 3, 6)
		g.place(st("cobblestone"), 4, 1, 5)
		g.place(st("cobblestone"), 6, 1, 5)
		g.place(st("cobblestone"), 4, 3, 5)
		g.place(st("cobblestone"), 6, 3, 5)

		for var8 = 1; var8 <= 3; var8++ {
			g.place(st("cobblestone"), 4, var8, 4)
			g.place(st("cobblestone"), 6, var8, 4)
			g.place(st("cobblestone"), 4, var8, 6)
			g.place(st("cobblestone"), 6, var8, 6)
		}

		g.place(st("wall_torch"), 5, 3, 5)

		for var8 = 2; var8 <= 8; var8++ {
			g.place(st("oak_planks"), 2, 3, var8)
			g.place(st("oak_planks"), 3, 3, var8)
			if var8 <= 3 || var8 >= 7 {
				g.place(st("oak_planks"), 4, 3, var8)
				g.place(st("oak_planks"), 5, 3, var8)
				g.place(st("oak_planks"), 6, 3, var8)
			}

			g.place(st("oak_planks"), 7, 3, var8)
			g.place(st("oak_planks"), 8, 3, var8)
		}

		var9 := st("ladder").with("facing", "west")
		g.place(var9, 9, 1, 3)
		g.place(var9, 9, 2, 3)
		g.place(var9, 9, 3, 3)
		g.chest(3, 4, 8)
	}

}

func (g *pc) shStraightStairsDown(p *worldgen.StrongholdPiece) {
	g.boxSel(0, 0, 0, 4, 10, 7, true, smoothSel{})
	g.smallDoor(p.Door, 1, 7, 0)
	g.smallDoor("OPENING", 1, 1, 7)
	var8 := st("cobblestone_stairs").with("facing", "south")

	for var9 := 0; var9 < 6; var9++ {
		g.place(var8, 1, 6-var9, 1+var9)
		g.place(var8, 2, 6-var9, 1+var9)
		g.place(var8, 3, 6-var9, 1+var9)
		if var9 < 5 {
			g.place(st("stone_bricks"), 1, 5-var9, 1+var9)
			g.place(st("stone_bricks"), 2, 5-var9, 1+var9)
			g.place(st("stone_bricks"), 3, 5-var9, 1+var9)
		}
	}

}

func (g *pc) shStairsDown(p *worldgen.StrongholdPiece) {
	g.boxSel(0, 0, 0, 4, 10, 4, true, smoothSel{})
	g.smallDoor(p.Door, 1, 7, 0)
	g.smallDoor("OPENING", 1, 1, 4)
	g.place(st("stone_bricks"), 2, 6, 1)
	g.place(st("stone_bricks"), 1, 5, 1)
	g.place(st("smooth_stone_slab"), 1, 6, 1)
	g.place(st("stone_bricks"), 1, 5, 2)
	g.place(st("stone_bricks"), 1, 4, 3)
	g.place(st("smooth_stone_slab"), 1, 5, 3)
	g.place(st("stone_bricks"), 2, 4, 3)
	g.place(st("stone_bricks"), 3, 3, 3)
	g.place(st("smooth_stone_slab"), 3, 4, 3)
	g.place(st("stone_bricks"), 3, 3, 2)
	g.place(st("stone_bricks"), 3, 2, 1)
	g.place(st("smooth_stone_slab"), 3, 3, 1)
	g.place(st("stone_bricks"), 2, 2, 1)
	g.place(st("stone_bricks"), 1, 1, 1)
	g.place(st("smooth_stone_slab"), 1, 2, 1)
	g.place(st("stone_bricks"), 1, 1, 2)
	g.place(st("smooth_stone_slab"), 1, 1, 3)
}

func (g *pc) shFiveCrossing(p *worldgen.StrongholdPiece) {
	g.boxSel(0, 0, 0, 9, 8, 10, true, smoothSel{})
	g.smallDoor(p.Door, 4, 3, 0)
	if p.Flags[0] {
		g.box(0, 3, 1, 0, 5, 3, caveAir, caveAir, false)
	}

	if p.Flags[2] {
		g.box(9, 3, 1, 9, 5, 3, caveAir, caveAir, false)
	}

	if p.Flags[1] {
		g.box(0, 5, 7, 0, 7, 9, caveAir, caveAir, false)
	}

	if p.Flags[3] {
		g.box(9, 5, 7, 9, 7, 9, caveAir, caveAir, false)
	}

	g.box(5, 1, 10, 7, 3, 10, caveAir, caveAir, false)
	g.boxSel(1, 2, 1, 8, 2, 6, false, smoothSel{})
	g.boxSel(4, 1, 5, 4, 4, 9, false, smoothSel{})
	g.boxSel(8, 1, 5, 8, 4, 9, false, smoothSel{})
	g.boxSel(1, 4, 7, 3, 4, 9, false, smoothSel{})
	g.boxSel(1, 3, 5, 3, 3, 6, false, smoothSel{})
	g.box(1, 3, 4, 3, 3, 4, st("smooth_stone_slab"), st("smooth_stone_slab"), false)
	g.box(1, 4, 6, 3, 4, 6, st("smooth_stone_slab"), st("smooth_stone_slab"), false)
	g.boxSel(5, 1, 7, 7, 1, 8, false, smoothSel{})
	g.box(5, 1, 9, 7, 1, 9, st("smooth_stone_slab"), st("smooth_stone_slab"), false)
	g.box(5, 2, 7, 7, 2, 7, st("smooth_stone_slab"), st("smooth_stone_slab"), false)
	g.box(4, 5, 7, 4, 5, 9, st("smooth_stone_slab"), st("smooth_stone_slab"), false)
	g.box(8, 5, 7, 8, 5, 9, st("smooth_stone_slab"), st("smooth_stone_slab"), false)
	g.box(5, 5, 7, 7, 5, 9, st("smooth_stone_slab").with("type", "double"), st("smooth_stone_slab").with("type", "double"), false)
	g.place(st("wall_torch").with("facing", "south"), 6, 5, 6)
}

func (g *pc) shChestCorridor(p *worldgen.StrongholdPiece) {
	g.boxSel(0, 0, 0, 4, 4, 6, true, smoothSel{})
	g.smallDoor(p.Door, 1, 1, 0)
	g.smallDoor("OPENING", 1, 1, 6)
	g.box(3, 1, 2, 3, 1, 4, st("stone_bricks"), st("stone_bricks"), false)
	g.place(st("stone_brick_slab"), 3, 1, 1)
	g.place(st("stone_brick_slab"), 3, 1, 5)
	g.place(st("stone_brick_slab"), 3, 2, 2)
	g.place(st("stone_brick_slab"), 3, 2, 4)

	for var8 := 2; var8 <= 4; var8++ {
		g.place(st("stone_brick_slab"), 2, 1, var8)
	}

	if g.needChest && g.inside(3, 2, 3) {
		g.needChest = false
		g.chest(3, 2, 3)
	}

}

func (g *pc) shLibrary(p *worldgen.StrongholdPiece) {
	var8 := 11
	if !p.Tall {
		var8 = 6
	}

	g.boxSel(0, 0, 0, 13, var8-1, 14, true, smoothSel{})
	g.smallDoor(p.Door, 4, 1, 0)
	g.maybeBox(0.07, 2, 1, 1, 11, 4, 13, st("cobweb"), st("cobweb"), false, false)

	var var11 int
	for var11 = 1; var11 <= 13; var11++ {
		if (var11-1)%4 == 0 {
			g.box(1, 1, var11, 1, 4, var11, st("oak_planks"), st("oak_planks"), false)
			g.box(12, 1, var11, 12, 4, var11, st("oak_planks"), st("oak_planks"), false)
			g.place(st("wall_torch").with("facing", "east"), 2, 3, var11)
			g.place(st("wall_torch").with("facing", "west"), 11, 3, var11)
			if p.Tall {
				g.box(1, 6, var11, 1, 9, var11, st("oak_planks"), st("oak_planks"), false)
				g.box(12, 6, var11, 12, 9, var11, st("oak_planks"), st("oak_planks"), false)
			}
		} else {
			g.box(1, 1, var11, 1, 4, var11, st("bookshelf"), st("bookshelf"), false)
			g.box(12, 1, var11, 12, 4, var11, st("bookshelf"), st("bookshelf"), false)
			if p.Tall {
				g.box(1, 6, var11, 1, 9, var11, st("bookshelf"), st("bookshelf"), false)
				g.box(12, 6, var11, 12, 9, var11, st("bookshelf"), st("bookshelf"), false)
			}
		}
	}

	for var11 = 3; var11 < 12; var11 += 2 {
		g.box(3, 1, var11, 4, 3, var11, st("bookshelf"), st("bookshelf"), false)
		g.box(6, 1, var11, 7, 3, var11, st("bookshelf"), st("bookshelf"), false)
		g.box(9, 1, var11, 10, 3, var11, st("bookshelf"), st("bookshelf"), false)
	}

	if p.Tall {
		g.box(1, 5, 1, 3, 5, 13, st("oak_planks"), st("oak_planks"), false)
		g.box(10, 5, 1, 12, 5, 13, st("oak_planks"), st("oak_planks"), false)
		g.box(4, 5, 1, 9, 5, 2, st("oak_planks"), st("oak_planks"), false)
		g.box(4, 5, 12, 9, 5, 13, st("oak_planks"), st("oak_planks"), false)
		g.place(st("oak_planks"), 9, 5, 11)
		g.place(st("oak_planks"), 8, 5, 11)
		g.place(st("oak_planks"), 9, 5, 10)
		var20 := (st("oak_fence").with("west", "true")).with("east", "true")
		var12 := (st("oak_fence").with("north", "true")).with("south", "true")
		g.box(3, 6, 3, 3, 6, 11, var12, var12, false)
		g.box(10, 6, 3, 10, 6, 9, var12, var12, false)
		g.box(4, 6, 2, 9, 6, 2, var20, var20, false)
		g.box(4, 6, 12, 7, 6, 12, var20, var20, false)
		g.place((st("oak_fence").with("north", "true")).with("east", "true"), 3, 6, 2)
		g.place((st("oak_fence").with("south", "true")).with("east", "true"), 3, 6, 12)
		g.place((st("oak_fence").with("north", "true")).with("west", "true"), 10, 6, 2)

		for var13 := 0; var13 <= 2; var13++ {
			g.place((st("oak_fence").with("south", "true")).with("west", "true"), 8+var13, 6, 12-var13)
			if var13 != 2 {
				g.place((st("oak_fence").with("north", "true")).with("east", "true"), 8+var13, 6, 11-var13)
			}
		}

		var21 := st("ladder").with("facing", "south")
		g.place(var21, 10, 1, 13)
		g.place(var21, 10, 2, 13)
		g.place(var21, 10, 3, 13)
		g.place(var21, 10, 4, 13)
		g.place(var21, 10, 5, 13)
		g.place(var21, 10, 6, 13)
		g.place(var21, 10, 7, 13)
		var16 := st("oak_fence").with("east", "true")
		g.place(var16, 6, 9, 7)
		var17 := st("oak_fence").with("west", "true")
		g.place(var17, 7, 9, 7)
		g.place(var16, 6, 8, 7)
		g.place(var17, 7, 8, 7)
		var18 := (var12.with("west", "true")).with("east", "true")
		g.place(var18, 6, 7, 7)
		g.place(var18, 7, 7, 7)
		g.place(var16, 5, 7, 7)
		g.place(var17, 8, 7, 7)
		g.place(var16.with("north", "true"), 6, 7, 6)
		g.place(var16.with("south", "true"), 6, 7, 8)
		g.place(var17.with("north", "true"), 7, 7, 6)
		g.place(var17.with("south", "true"), 7, 7, 8)
		var19 := st("torch")
		g.place(var19, 5, 8, 7)
		g.place(var19, 8, 8, 7)
		g.place(var19, 6, 8, 6)
		g.place(var19, 6, 8, 8)
		g.place(var19, 7, 8, 6)
		g.place(var19, 7, 8, 8)
	}

	g.chest(3, 3, 5)
	if p.Tall {
		g.place(caveAir, 12, 9, 1)
		g.chest(12, 8, 1)
	}

}

func (g *pc) shPortalRoom(p *worldgen.StrongholdPiece) {
	g.boxSel(0, 0, 0, 10, 7, 15, false, smoothSel{})
	g.smallDoor("GRATES", 4, 1, 0)
	g.boxSel(1, 6, 1, 1, 6, 14, false, smoothSel{})
	g.boxSel(9, 6, 1, 9, 6, 14, false, smoothSel{})
	g.boxSel(2, 6, 1, 8, 6, 2, false, smoothSel{})
	g.boxSel(2, 6, 14, 8, 6, 14, false, smoothSel{})
	g.boxSel(1, 1, 1, 2, 1, 4, false, smoothSel{})
	g.boxSel(8, 1, 1, 9, 1, 4, false, smoothSel{})
	g.box(1, 1, 1, 1, 1, 3, st("lava"), st("lava"), false)
	g.box(9, 1, 1, 9, 1, 3, st("lava"), st("lava"), false)
	g.boxSel(3, 1, 8, 7, 1, 12, false, smoothSel{})
	g.box(4, 1, 9, 6, 1, 11, st("lava"), st("lava"), false)
	var9 := (st("iron_bars").with("north", "true")).with("south", "true")
	var10 := (st("iron_bars").with("west", "true")).with("east", "true")

	var var11 int
	for var11 = 3; var11 < 14; var11 += 2 {
		g.box(0, 3, var11, 0, 4, var11, var9, var9, false)
		g.box(10, 3, var11, 10, 4, var11, var9, var9, false)
	}

	for var11 = 2; var11 < 9; var11 += 2 {
		g.box(var11, 3, 15, var11, 4, 15, var10, var10, false)
	}

	var21 := st("stone_brick_stairs").with("facing", "north")
	g.boxSel(4, 1, 5, 6, 1, 7, false, smoothSel{})
	g.boxSel(4, 2, 6, 6, 2, 7, false, smoothSel{})
	g.boxSel(4, 3, 7, 6, 3, 7, false, smoothSel{})

	for var12 := 4; var12 <= 6; var12++ {
		g.place(var21, var12, 1, 4)
		g.place(var21, var12, 2, 5)
		g.place(var21, var12, 3, 6)
	}

	var22 := st("end_portal_frame").with("facing", "north")
	var13 := st("end_portal_frame").with("facing", "south")
	var14 := st("end_portal_frame").with("facing", "east")
	var15 := st("end_portal_frame").with("facing", "west")
	var17 := make([]bool, 12)
	var16 := true
	for var18 := range var17 {
		var17[var18] = g.rnd.nextFloat() > 0.9
		var16 = var16 && var17[var18]
	}

	g.place(var22.with("eye", fmt.Sprint(var17[0])), 4, 3, 8)
	g.place(var22.with("eye", fmt.Sprint(var17[1])), 5, 3, 8)
	g.place(var22.with("eye", fmt.Sprint(var17[2])), 6, 3, 8)
	g.place(var13.with("eye", fmt.Sprint(var17[3])), 4, 3, 12)
	g.place(var13.with("eye", fmt.Sprint(var17[4])), 5, 3, 12)
	g.place(var13.with("eye", fmt.Sprint(var17[5])), 6, 3, 12)
	g.place(var14.with("eye", fmt.Sprint(var17[6])), 3, 3, 9)
	g.place(var14.with("eye", fmt.Sprint(var17[7])), 3, 3, 10)
	g.place(var14.with("eye", fmt.Sprint(var17[8])), 3, 3, 11)
	g.place(var15.with("eye", fmt.Sprint(var17[9])), 7, 3, 9)
	g.place(var15.with("eye", fmt.Sprint(var17[10])), 7, 3, 10)
	g.place(var15.with("eye", fmt.Sprint(var17[11])), 7, 3, 11)
	if var16 {
		var23 := st("end_portal")
		g.place(var23, 4, 3, 9)
		g.place(var23, 5, 3, 9)
		g.place(var23, 6, 3, 9)
		g.place(var23, 4, 3, 10)
		g.place(var23, 5, 3, 10)
		g.place(var23, 6, 3, 10)
		g.place(var23, 4, 3, 11)
		g.place(var23, 5, 3, 11)
		g.place(var23, 6, 3, 11)
	}

	g.spawner(5, 3, 6)

}

func (g *pc) shFillerCorridor(p *worldgen.StrongholdPiece) {
	for var8 := 0; var8 < p.Steps; var8++ {
		g.place(st("stone_bricks"), 0, 0, var8)
		g.place(st("stone_bricks"), 1, 0, var8)
		g.place(st("stone_bricks"), 2, 0, var8)
		g.place(st("stone_bricks"), 3, 0, var8)
		g.place(st("stone_bricks"), 4, 0, var8)

		for var9 := 1; var9 <= 3; var9++ {
			g.place(st("stone_bricks"), 0, var9, var8)
			g.place(st("cave_air"), 1, var9, var8)
			g.place(st("cave_air"), 2, var9, var8)
			g.place(st("cave_air"), 3, var9, var8)
			g.place(st("stone_bricks"), 4, var9, var8)
		}

		g.place(st("stone_bricks"), 0, 4, var8)
		g.place(st("stone_bricks"), 1, 4, var8)
		g.place(st("stone_bricks"), 2, 4, var8)
		g.place(st("stone_bricks"), 3, 4, var8)
		g.place(st("stone_bricks"), 4, 4, var8)
	}

}
