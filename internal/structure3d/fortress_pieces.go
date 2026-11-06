package structure3d

// Code generated from NetherFortressPieces postProcess (Minecraft 1.21.1) by
// a mechanical Java-to-Go translation; see fortress.go for the driver.

func (g *pc) postBridgeStraight() {
	g.box(0, 3, 0, 4, 4, 18, st("nether_bricks"), st("nether_bricks"), false)
	g.box(1, 5, 0, 3, 7, 18, st("air"), st("air"), false)
	g.box(0, 5, 0, 0, 5, 18, st("nether_bricks"), st("nether_bricks"), false)
	g.box(4, 5, 0, 4, 5, 18, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 2, 0, 4, 2, 5, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 2, 13, 4, 2, 18, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 0, 0, 4, 1, 3, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 0, 15, 4, 1, 18, st("nether_bricks"), st("nether_bricks"), false)

	for var8 := 0; var8 <= 4; var8++ {
		for var9 := 0; var9 <= 2; var9++ {
			g.fillDown(st("nether_bricks"), var8, -1, var9)
			g.fillDown(st("nether_bricks"), var8, -1, 18-var9)
		}
	}

	var11 := (st("nether_brick_fence").with("north", "true")).with("south", "true")
	var12 := var11.with("east", "true")
	var10 := var11.with("west", "true")
	g.box(0, 1, 1, 0, 4, 1, var12, var12, false)
	g.box(0, 3, 4, 0, 4, 4, var12, var12, false)
	g.box(0, 3, 14, 0, 4, 14, var12, var12, false)
	g.box(0, 1, 17, 0, 4, 17, var12, var12, false)
	g.box(4, 1, 1, 4, 4, 1, var10, var10, false)
	g.box(4, 3, 4, 4, 4, 4, var10, var10, false)
	g.box(4, 3, 14, 4, 4, 14, var10, var10, false)
	g.box(4, 1, 17, 4, 4, 17, var10, var10, false)
}

func (g *pc) postBridgeCrossing() {
	g.box(7, 3, 0, 11, 4, 18, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 3, 7, 18, 4, 11, st("nether_bricks"), st("nether_bricks"), false)
	g.box(8, 5, 0, 10, 7, 18, st("air"), st("air"), false)
	g.box(0, 5, 8, 18, 7, 10, st("air"), st("air"), false)
	g.box(7, 5, 0, 7, 5, 7, st("nether_bricks"), st("nether_bricks"), false)
	g.box(7, 5, 11, 7, 5, 18, st("nether_bricks"), st("nether_bricks"), false)
	g.box(11, 5, 0, 11, 5, 7, st("nether_bricks"), st("nether_bricks"), false)
	g.box(11, 5, 11, 11, 5, 18, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 5, 7, 7, 5, 7, st("nether_bricks"), st("nether_bricks"), false)
	g.box(11, 5, 7, 18, 5, 7, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 5, 11, 7, 5, 11, st("nether_bricks"), st("nether_bricks"), false)
	g.box(11, 5, 11, 18, 5, 11, st("nether_bricks"), st("nether_bricks"), false)
	g.box(7, 2, 0, 11, 2, 5, st("nether_bricks"), st("nether_bricks"), false)
	g.box(7, 2, 13, 11, 2, 18, st("nether_bricks"), st("nether_bricks"), false)
	g.box(7, 0, 0, 11, 1, 3, st("nether_bricks"), st("nether_bricks"), false)
	g.box(7, 0, 15, 11, 1, 18, st("nether_bricks"), st("nether_bricks"), false)

	var var8 int
	var var9 int
	for var8 = 7; var8 <= 11; var8++ {
		for var9 = 0; var9 <= 2; var9++ {
			g.fillDown(st("nether_bricks"), var8, -1, var9)
			g.fillDown(st("nether_bricks"), var8, -1, 18-var9)
		}
	}

	g.box(0, 2, 7, 5, 2, 11, st("nether_bricks"), st("nether_bricks"), false)
	g.box(13, 2, 7, 18, 2, 11, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 0, 7, 3, 1, 11, st("nether_bricks"), st("nether_bricks"), false)
	g.box(15, 0, 7, 18, 1, 11, st("nether_bricks"), st("nether_bricks"), false)

	for var8 = 0; var8 <= 2; var8++ {
		for var9 = 7; var9 <= 11; var9++ {
			g.fillDown(st("nether_bricks"), var8, -1, var9)
			g.fillDown(st("nether_bricks"), 18-var8, -1, var9)
		}
	}

}

func (g *pc) postRoomCrossing() {
	g.box(0, 0, 0, 6, 1, 6, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 2, 0, 6, 7, 6, st("air"), st("air"), false)
	g.box(0, 2, 0, 1, 6, 0, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 2, 6, 1, 6, 6, st("nether_bricks"), st("nether_bricks"), false)
	g.box(5, 2, 0, 6, 6, 0, st("nether_bricks"), st("nether_bricks"), false)
	g.box(5, 2, 6, 6, 6, 6, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 2, 0, 0, 6, 1, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 2, 5, 0, 6, 6, st("nether_bricks"), st("nether_bricks"), false)
	g.box(6, 2, 0, 6, 6, 1, st("nether_bricks"), st("nether_bricks"), false)
	g.box(6, 2, 5, 6, 6, 6, st("nether_bricks"), st("nether_bricks"), false)
	var8 := (st("nether_brick_fence").with("west", "true")).with("east", "true")
	var9 := (st("nether_brick_fence").with("north", "true")).with("south", "true")
	g.box(2, 6, 0, 4, 6, 0, st("nether_bricks"), st("nether_bricks"), false)
	g.box(2, 5, 0, 4, 5, 0, var8, var8, false)
	g.box(2, 6, 6, 4, 6, 6, st("nether_bricks"), st("nether_bricks"), false)
	g.box(2, 5, 6, 4, 5, 6, var8, var8, false)
	g.box(0, 6, 2, 0, 6, 4, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 5, 2, 0, 5, 4, var9, var9, false)
	g.box(6, 6, 2, 6, 6, 4, st("nether_bricks"), st("nether_bricks"), false)
	g.box(6, 5, 2, 6, 5, 4, var9, var9, false)

	for var10 := 0; var10 <= 6; var10++ {
		for var11 := 0; var11 <= 6; var11++ {
			g.fillDown(st("nether_bricks"), var10, -1, var11)
		}
	}

}

func (g *pc) postStairsRoom() {
	g.box(0, 0, 0, 6, 1, 6, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 2, 0, 6, 10, 6, st("air"), st("air"), false)
	g.box(0, 2, 0, 1, 8, 0, st("nether_bricks"), st("nether_bricks"), false)
	g.box(5, 2, 0, 6, 8, 0, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 2, 1, 0, 8, 6, st("nether_bricks"), st("nether_bricks"), false)
	g.box(6, 2, 1, 6, 8, 6, st("nether_bricks"), st("nether_bricks"), false)
	g.box(1, 2, 6, 5, 8, 6, st("nether_bricks"), st("nether_bricks"), false)
	var8 := (st("nether_brick_fence").with("west", "true")).with("east", "true")
	var9 := (st("nether_brick_fence").with("north", "true")).with("south", "true")
	g.box(0, 3, 2, 0, 5, 4, var9, var9, false)
	g.box(6, 3, 2, 6, 5, 2, var9, var9, false)
	g.box(6, 3, 4, 6, 5, 4, var9, var9, false)
	g.place(st("nether_bricks"), 5, 2, 5)
	g.box(4, 2, 5, 4, 3, 5, st("nether_bricks"), st("nether_bricks"), false)
	g.box(3, 2, 5, 3, 4, 5, st("nether_bricks"), st("nether_bricks"), false)
	g.box(2, 2, 5, 2, 5, 5, st("nether_bricks"), st("nether_bricks"), false)
	g.box(1, 2, 5, 1, 6, 5, st("nether_bricks"), st("nether_bricks"), false)
	g.box(1, 7, 1, 5, 7, 4, st("nether_bricks"), st("nether_bricks"), false)
	g.box(6, 8, 2, 6, 8, 4, st("air"), st("air"), false)
	g.box(2, 6, 0, 4, 8, 0, st("nether_bricks"), st("nether_bricks"), false)
	g.box(2, 5, 0, 4, 5, 0, var8, var8, false)

	for var10 := 0; var10 <= 6; var10++ {
		for var11 := 0; var11 <= 6; var11++ {
			g.fillDown(st("nether_bricks"), var10, -1, var11)
		}
	}

}

func (g *pc) postMonsterThrone() {
	g.box(0, 2, 0, 6, 7, 7, st("air"), st("air"), false)
	g.box(1, 0, 0, 5, 1, 7, st("nether_bricks"), st("nether_bricks"), false)
	g.box(1, 2, 1, 5, 2, 7, st("nether_bricks"), st("nether_bricks"), false)
	g.box(1, 3, 2, 5, 3, 7, st("nether_bricks"), st("nether_bricks"), false)
	g.box(1, 4, 3, 5, 4, 7, st("nether_bricks"), st("nether_bricks"), false)
	g.box(1, 2, 0, 1, 4, 2, st("nether_bricks"), st("nether_bricks"), false)
	g.box(5, 2, 0, 5, 4, 2, st("nether_bricks"), st("nether_bricks"), false)
	g.box(1, 5, 2, 1, 5, 3, st("nether_bricks"), st("nether_bricks"), false)
	g.box(5, 5, 2, 5, 5, 3, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 5, 3, 0, 5, 8, st("nether_bricks"), st("nether_bricks"), false)
	g.box(6, 5, 3, 6, 5, 8, st("nether_bricks"), st("nether_bricks"), false)
	g.box(1, 5, 8, 5, 5, 8, st("nether_bricks"), st("nether_bricks"), false)
	var8 := (st("nether_brick_fence").with("west", "true")).with("east", "true")
	var9 := (st("nether_brick_fence").with("north", "true")).with("south", "true")
	g.place(st("nether_brick_fence").with("west", "true"), 1, 6, 3)
	g.place(st("nether_brick_fence").with("east", "true"), 5, 6, 3)
	g.place((st("nether_brick_fence").with("east", "true")).with("north", "true"), 0, 6, 3)
	g.place((st("nether_brick_fence").with("west", "true")).with("north", "true"), 6, 6, 3)
	g.box(0, 6, 4, 0, 6, 7, var9, var9, false)
	g.box(6, 6, 4, 6, 6, 7, var9, var9, false)
	g.place((st("nether_brick_fence").with("east", "true")).with("south", "true"), 0, 6, 8)
	g.place((st("nether_brick_fence").with("west", "true")).with("south", "true"), 6, 6, 8)
	g.box(1, 6, 8, 5, 6, 8, var8, var8, false)
	g.place(st("nether_brick_fence").with("east", "true"), 1, 7, 8)
	g.box(2, 7, 8, 4, 7, 8, var8, var8, false)
	g.place(st("nether_brick_fence").with("west", "true"), 5, 7, 8)
	g.place(st("nether_brick_fence").with("east", "true"), 2, 8, 8)
	g.place(var8, 3, 8, 8)
	g.place(st("nether_brick_fence").with("west", "true"), 4, 8, 8)
	g.spawner(3, 5, 5)

	for var13 := 0; var13 <= 6; var13++ {
		for var14 := 0; var14 <= 6; var14++ {
			g.fillDown(st("nether_bricks"), var13, -1, var14)
		}
	}

}

func (g *pc) postCastleEntrance() {
	g.box(0, 3, 0, 12, 4, 12, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 5, 0, 12, 13, 12, st("air"), st("air"), false)
	g.box(0, 5, 0, 1, 12, 12, st("nether_bricks"), st("nether_bricks"), false)
	g.box(11, 5, 0, 12, 12, 12, st("nether_bricks"), st("nether_bricks"), false)
	g.box(2, 5, 11, 4, 12, 12, st("nether_bricks"), st("nether_bricks"), false)
	g.box(8, 5, 11, 10, 12, 12, st("nether_bricks"), st("nether_bricks"), false)
	g.box(5, 9, 11, 7, 12, 12, st("nether_bricks"), st("nether_bricks"), false)
	g.box(2, 5, 0, 4, 12, 1, st("nether_bricks"), st("nether_bricks"), false)
	g.box(8, 5, 0, 10, 12, 1, st("nether_bricks"), st("nether_bricks"), false)
	g.box(5, 9, 0, 7, 12, 1, st("nether_bricks"), st("nether_bricks"), false)
	g.box(2, 11, 2, 10, 12, 10, st("nether_bricks"), st("nether_bricks"), false)
	g.box(5, 8, 0, 7, 8, 0, st("nether_brick_fence"), st("nether_brick_fence"), false)
	var8 := (st("nether_brick_fence").with("west", "true")).with("east", "true")
	var9 := (st("nether_brick_fence").with("north", "true")).with("south", "true")

	var var10 int
	for var10 = 1; var10 <= 11; var10 += 2 {
		g.box(var10, 10, 0, var10, 11, 0, var8, var8, false)
		g.box(var10, 10, 12, var10, 11, 12, var8, var8, false)
		g.box(0, 10, var10, 0, 11, var10, var9, var9, false)
		g.box(12, 10, var10, 12, 11, var10, var9, var9, false)
		g.place(st("nether_bricks"), var10, 13, 0)
		g.place(st("nether_bricks"), var10, 13, 12)
		g.place(st("nether_bricks"), 0, 13, var10)
		g.place(st("nether_bricks"), 12, 13, var10)
		if var10 != 11 {
			g.place(var8, var10+1, 13, 0)
			g.place(var8, var10+1, 13, 12)
			g.place(var9, 0, 13, var10+1)
			g.place(var9, 12, 13, var10+1)
		}
	}

	g.place((st("nether_brick_fence").with("north", "true")).with("east", "true"), 0, 13, 0)
	g.place((st("nether_brick_fence").with("south", "true")).with("east", "true"), 0, 13, 12)
	g.place((st("nether_brick_fence").with("south", "true")).with("west", "true"), 12, 13, 12)
	g.place((st("nether_brick_fence").with("north", "true")).with("west", "true"), 12, 13, 0)

	for var10 = 3; var10 <= 9; var10 += 2 {
		g.box(1, 7, var10, 1, 8, var10, var9.with("west", "true"), var9.with("west", "true"), false)
		g.box(11, 7, var10, 11, 8, var10, var9.with("east", "true"), var9.with("east", "true"), false)
	}

	g.box(4, 2, 0, 8, 2, 12, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 2, 4, 12, 2, 8, st("nether_bricks"), st("nether_bricks"), false)
	g.box(4, 0, 0, 8, 1, 3, st("nether_bricks"), st("nether_bricks"), false)
	g.box(4, 0, 9, 8, 1, 12, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 0, 4, 3, 1, 8, st("nether_bricks"), st("nether_bricks"), false)
	g.box(9, 0, 4, 12, 1, 8, st("nether_bricks"), st("nether_bricks"), false)

	var var11 int
	for var10 = 4; var10 <= 8; var10++ {
		for var11 = 0; var11 <= 2; var11++ {
			g.fillDown(st("nether_bricks"), var10, -1, var11)
			g.fillDown(st("nether_bricks"), var10, -1, 12-var11)
		}
	}

	for var10 = 0; var10 <= 2; var10++ {
		for var11 = 4; var11 <= 8; var11++ {
			g.fillDown(st("nether_bricks"), var10, -1, var11)
			g.fillDown(st("nether_bricks"), 12-var10, -1, var11)
		}
	}

	g.box(5, 5, 5, 7, 5, 7, st("nether_bricks"), st("nether_bricks"), false)
	g.box(6, 1, 6, 6, 4, 6, st("air"), st("air"), false)
	g.place(st("nether_bricks"), 6, 0, 6)
	g.place(st("lava"), 6, 5, 6)

}

func (g *pc) postCastleSmallCorridorPiece() {
	g.box(0, 0, 0, 4, 1, 4, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 2, 0, 4, 5, 4, st("air"), st("air"), false)
	var8 := (st("nether_brick_fence").with("north", "true")).with("south", "true")
	g.box(0, 2, 0, 0, 5, 4, st("nether_bricks"), st("nether_bricks"), false)
	g.box(4, 2, 0, 4, 5, 4, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 3, 1, 0, 4, 1, var8, var8, false)
	g.box(0, 3, 3, 0, 4, 3, var8, var8, false)
	g.box(4, 3, 1, 4, 4, 1, var8, var8, false)
	g.box(4, 3, 3, 4, 4, 3, var8, var8, false)
	g.box(0, 6, 0, 4, 6, 4, st("nether_bricks"), st("nether_bricks"), false)

	for var9 := 0; var9 <= 4; var9++ {
		for var10 := 0; var10 <= 4; var10++ {
			g.fillDown(st("nether_bricks"), var9, -1, var10)
		}
	}

}

func (g *pc) postCastleSmallCorridorRightTurnPiece() {
	g.box(0, 0, 0, 4, 1, 4, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 2, 0, 4, 5, 4, st("air"), st("air"), false)
	var8 := (st("nether_brick_fence").with("west", "true")).with("east", "true")
	var9 := (st("nether_brick_fence").with("north", "true")).with("south", "true")
	g.box(0, 2, 0, 0, 5, 4, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 3, 1, 0, 4, 1, var9, var9, false)
	g.box(0, 3, 3, 0, 4, 3, var9, var9, false)
	g.box(4, 2, 0, 4, 5, 0, st("nether_bricks"), st("nether_bricks"), false)
	g.box(1, 2, 4, 4, 5, 4, st("nether_bricks"), st("nether_bricks"), false)
	g.box(1, 3, 4, 1, 4, 4, var8, var8, false)
	g.box(3, 3, 4, 3, 4, 4, var8, var8, false)
	if g.needChest && g.inside(1, 2, 3) {
		g.needChest = false
		g.chest(1, 2, 3)
	}

	g.box(0, 6, 0, 4, 6, 4, st("nether_bricks"), st("nether_bricks"), false)

	for var10 := 0; var10 <= 4; var10++ {
		for var11 := 0; var11 <= 4; var11++ {
			g.fillDown(st("nether_bricks"), var10, -1, var11)
		}
	}

}

func (g *pc) postCastleSmallCorridorLeftTurnPiece() {
	g.box(0, 0, 0, 4, 1, 4, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 2, 0, 4, 5, 4, st("air"), st("air"), false)
	var8 := (st("nether_brick_fence").with("west", "true")).with("east", "true")
	var9 := (st("nether_brick_fence").with("north", "true")).with("south", "true")
	g.box(4, 2, 0, 4, 5, 4, st("nether_bricks"), st("nether_bricks"), false)
	g.box(4, 3, 1, 4, 4, 1, var9, var9, false)
	g.box(4, 3, 3, 4, 4, 3, var9, var9, false)
	g.box(0, 2, 0, 0, 5, 0, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 2, 4, 3, 5, 4, st("nether_bricks"), st("nether_bricks"), false)
	g.box(1, 3, 4, 1, 4, 4, var8, var8, false)
	g.box(3, 3, 4, 3, 4, 4, var8, var8, false)
	if g.needChest && g.inside(3, 2, 3) {
		g.needChest = false
		g.chest(3, 2, 3)
	}

	g.box(0, 6, 0, 4, 6, 4, st("nether_bricks"), st("nether_bricks"), false)

	for var10 := 0; var10 <= 4; var10++ {
		for var11 := 0; var11 <= 4; var11++ {
			g.fillDown(st("nether_bricks"), var10, -1, var11)
		}
	}

}

func (g *pc) postCastleCorridorStairsPiece() {
	var8 := st("nether_brick_stairs").with("facing", "south")
	var9 := (st("nether_brick_fence").with("north", "true")).with("south", "true")

	for var10 := 0; var10 <= 9; var10++ {
		var11 := max(1, 7-var10)
		var12 := min(max(var11+5, 14-var10), 13)
		var13 := var10
		g.box(0, 0, var10, 4, var11, var10, st("nether_bricks"), st("nether_bricks"), false)
		g.box(1, var11+1, var10, 3, var12-1, var10, st("air"), st("air"), false)
		if var10 <= 6 {
			g.place(var8, 1, var11+1, var10)
			g.place(var8, 2, var11+1, var10)
			g.place(var8, 3, var11+1, var10)
		}

		g.box(0, var12, var10, 4, var12, var10, st("nether_bricks"), st("nether_bricks"), false)
		g.box(0, var11+1, var10, 0, var12-1, var10, st("nether_bricks"), st("nether_bricks"), false)
		g.box(4, var11+1, var10, 4, var12-1, var10, st("nether_bricks"), st("nether_bricks"), false)
		if (var10 & 1) == 0 {
			g.box(0, var11+2, var10, 0, var11+3, var10, var9, var9, false)
			g.box(4, var11+2, var10, 4, var11+3, var10, var9, var9, false)
		}

		for var14 := 0; var14 <= 4; var14++ {
			g.fillDown(st("nether_bricks"), var14, -1, var13)
		}
	}

}

func (g *pc) postCastleCorridorTBalconyPiece() {
	var8 := (st("nether_brick_fence").with("north", "true")).with("south", "true")
	var9 := (st("nether_brick_fence").with("west", "true")).with("east", "true")
	g.box(0, 0, 0, 8, 1, 8, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 2, 0, 8, 5, 8, st("air"), st("air"), false)
	g.box(0, 6, 0, 8, 6, 5, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 2, 0, 2, 5, 0, st("nether_bricks"), st("nether_bricks"), false)
	g.box(6, 2, 0, 8, 5, 0, st("nether_bricks"), st("nether_bricks"), false)
	g.box(1, 3, 0, 1, 4, 0, var9, var9, false)
	g.box(7, 3, 0, 7, 4, 0, var9, var9, false)
	g.box(0, 2, 4, 8, 2, 8, st("nether_bricks"), st("nether_bricks"), false)
	g.box(1, 1, 4, 2, 2, 4, st("air"), st("air"), false)
	g.box(6, 1, 4, 7, 2, 4, st("air"), st("air"), false)
	g.box(1, 3, 8, 7, 3, 8, var9, var9, false)
	g.place((st("nether_brick_fence").with("east", "true")).with("south", "true"), 0, 3, 8)
	g.place((st("nether_brick_fence").with("west", "true")).with("south", "true"), 8, 3, 8)
	g.box(0, 3, 6, 0, 3, 7, var8, var8, false)
	g.box(8, 3, 6, 8, 3, 7, var8, var8, false)
	g.box(0, 3, 4, 0, 5, 5, st("nether_bricks"), st("nether_bricks"), false)
	g.box(8, 3, 4, 8, 5, 5, st("nether_bricks"), st("nether_bricks"), false)
	g.box(1, 3, 5, 2, 5, 5, st("nether_bricks"), st("nether_bricks"), false)
	g.box(6, 3, 5, 7, 5, 5, st("nether_bricks"), st("nether_bricks"), false)
	g.box(1, 4, 5, 1, 5, 5, var9, var9, false)
	g.box(7, 4, 5, 7, 5, 5, var9, var9, false)

	for var10 := 0; var10 <= 5; var10++ {
		for var11 := 0; var11 <= 8; var11++ {
			g.fillDown(st("nether_bricks"), var11, -1, var10)
		}
	}

}

func (g *pc) postCastleSmallCorridorCrossingPiece() {
	g.box(0, 0, 0, 4, 1, 4, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 2, 0, 4, 5, 4, st("air"), st("air"), false)
	g.box(0, 2, 0, 0, 5, 0, st("nether_bricks"), st("nether_bricks"), false)
	g.box(4, 2, 0, 4, 5, 0, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 2, 4, 0, 5, 4, st("nether_bricks"), st("nether_bricks"), false)
	g.box(4, 2, 4, 4, 5, 4, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 6, 0, 4, 6, 4, st("nether_bricks"), st("nether_bricks"), false)

	for var8 := 0; var8 <= 4; var8++ {
		for var9 := 0; var9 <= 4; var9++ {
			g.fillDown(st("nether_bricks"), var8, -1, var9)
		}
	}

}

func (g *pc) postCastleStalkRoom() {
	g.box(0, 3, 0, 12, 4, 12, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 5, 0, 12, 13, 12, st("air"), st("air"), false)
	g.box(0, 5, 0, 1, 12, 12, st("nether_bricks"), st("nether_bricks"), false)
	g.box(11, 5, 0, 12, 12, 12, st("nether_bricks"), st("nether_bricks"), false)
	g.box(2, 5, 11, 4, 12, 12, st("nether_bricks"), st("nether_bricks"), false)
	g.box(8, 5, 11, 10, 12, 12, st("nether_bricks"), st("nether_bricks"), false)
	g.box(5, 9, 11, 7, 12, 12, st("nether_bricks"), st("nether_bricks"), false)
	g.box(2, 5, 0, 4, 12, 1, st("nether_bricks"), st("nether_bricks"), false)
	g.box(8, 5, 0, 10, 12, 1, st("nether_bricks"), st("nether_bricks"), false)
	g.box(5, 9, 0, 7, 12, 1, st("nether_bricks"), st("nether_bricks"), false)
	g.box(2, 11, 2, 10, 12, 10, st("nether_bricks"), st("nether_bricks"), false)
	var8 := (st("nether_brick_fence").with("west", "true")).with("east", "true")
	var9 := (st("nether_brick_fence").with("north", "true")).with("south", "true")
	var10 := var9.with("west", "true")
	var11 := var9.with("east", "true")

	var var12 int
	for var12 = 1; var12 <= 11; var12 += 2 {
		g.box(var12, 10, 0, var12, 11, 0, var8, var8, false)
		g.box(var12, 10, 12, var12, 11, 12, var8, var8, false)
		g.box(0, 10, var12, 0, 11, var12, var9, var9, false)
		g.box(12, 10, var12, 12, 11, var12, var9, var9, false)
		g.place(st("nether_bricks"), var12, 13, 0)
		g.place(st("nether_bricks"), var12, 13, 12)
		g.place(st("nether_bricks"), 0, 13, var12)
		g.place(st("nether_bricks"), 12, 13, var12)
		if var12 != 11 {
			g.place(var8, var12+1, 13, 0)
			g.place(var8, var12+1, 13, 12)
			g.place(var9, 0, 13, var12+1)
			g.place(var9, 12, 13, var12+1)
		}
	}

	g.place((st("nether_brick_fence").with("north", "true")).with("east", "true"), 0, 13, 0)
	g.place((st("nether_brick_fence").with("south", "true")).with("east", "true"), 0, 13, 12)
	g.place((st("nether_brick_fence").with("south", "true")).with("west", "true"), 12, 13, 12)
	g.place((st("nether_brick_fence").with("north", "true")).with("west", "true"), 12, 13, 0)

	for var12 = 3; var12 <= 9; var12 += 2 {
		g.box(1, 7, var12, 1, 8, var12, var10, var10, false)
		g.box(11, 7, var12, 11, 8, var12, var11, var11, false)
	}

	var17 := st("nether_brick_stairs").with("facing", "north")

	var var13 int
	var var15 int
	for var13 = 0; var13 <= 6; var13++ {
		var14 := var13 + 4

		for var15 = 5; var15 <= 7; var15++ {
			g.place(var17, var15, 5+var13, var14)
		}

		if var14 >= 5 && var14 <= 8 {
			g.box(5, 5, var14, 7, var13+4, var14, st("nether_bricks"), st("nether_bricks"), false)
		} else if var14 >= 9 && var14 <= 10 {
			g.box(5, 8, var14, 7, var13+4, var14, st("nether_bricks"), st("nether_bricks"), false)
		}

		if var13 >= 1 {
			g.box(5, 6+var13, var14, 7, 9+var13, var14, st("air"), st("air"), false)
		}
	}

	for var13 = 5; var13 <= 7; var13++ {
		g.place(var17, var13, 12, 11)
	}

	g.box(5, 6, 7, 5, 7, 7, var11, var11, false)
	g.box(7, 6, 7, 7, 7, 7, var10, var10, false)
	g.box(5, 13, 12, 7, 13, 12, st("air"), st("air"), false)
	g.box(2, 5, 2, 3, 5, 3, st("nether_bricks"), st("nether_bricks"), false)
	g.box(2, 5, 9, 3, 5, 10, st("nether_bricks"), st("nether_bricks"), false)
	g.box(2, 5, 4, 2, 5, 8, st("nether_bricks"), st("nether_bricks"), false)
	g.box(9, 5, 2, 10, 5, 3, st("nether_bricks"), st("nether_bricks"), false)
	g.box(9, 5, 9, 10, 5, 10, st("nether_bricks"), st("nether_bricks"), false)
	g.box(10, 5, 4, 10, 5, 8, st("nether_bricks"), st("nether_bricks"), false)
	var18 := var17.with("facing", "east")
	var19 := var17.with("facing", "west")
	g.place(var19, 4, 5, 2)
	g.place(var19, 4, 5, 3)
	g.place(var19, 4, 5, 9)
	g.place(var19, 4, 5, 10)
	g.place(var18, 8, 5, 2)
	g.place(var18, 8, 5, 3)
	g.place(var18, 8, 5, 9)
	g.place(var18, 8, 5, 10)
	g.box(3, 4, 4, 4, 4, 8, st("soul_sand"), st("soul_sand"), false)
	g.box(8, 4, 4, 9, 4, 8, st("soul_sand"), st("soul_sand"), false)
	g.box(3, 5, 4, 4, 5, 8, st("nether_wart"), st("nether_wart"), false)
	g.box(8, 5, 4, 9, 5, 8, st("nether_wart"), st("nether_wart"), false)
	g.box(4, 2, 0, 8, 2, 12, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 2, 4, 12, 2, 8, st("nether_bricks"), st("nether_bricks"), false)
	g.box(4, 0, 0, 8, 1, 3, st("nether_bricks"), st("nether_bricks"), false)
	g.box(4, 0, 9, 8, 1, 12, st("nether_bricks"), st("nether_bricks"), false)
	g.box(0, 0, 4, 3, 1, 8, st("nether_bricks"), st("nether_bricks"), false)
	g.box(9, 0, 4, 12, 1, 8, st("nether_bricks"), st("nether_bricks"), false)

	var var16 int
	for var15 = 4; var15 <= 8; var15++ {
		for var16 = 0; var16 <= 2; var16++ {
			g.fillDown(st("nether_bricks"), var15, -1, var16)
			g.fillDown(st("nether_bricks"), var15, -1, 12-var16)
		}
	}

	for var15 = 0; var15 <= 2; var15++ {
		for var16 = 4; var16 <= 8; var16++ {
			g.fillDown(st("nether_bricks"), var15, -1, var16)
			g.fillDown(st("nether_bricks"), 12-var15, -1, var16)
		}
	}

}

func (g *pc) postBridgeEndFiller() {
	var8 := newJR(int64(g.selfSeed))

	var var9 int
	var var10 int
	var var11 int
	for var9 = 0; var9 <= 4; var9++ {
		for var10 = 3; var10 <= 4; var10++ {
			var11 = var8.nextInt(8)
			g.box(var9, var10, 0, var9, var10, var11, st("nether_bricks"), st("nether_bricks"), false)
		}
	}

	var9 = var8.nextInt(8)
	g.box(0, 5, 0, 0, 5, var9, st("nether_bricks"), st("nether_bricks"), false)
	var9 = var8.nextInt(8)
	g.box(4, 5, 0, 4, 5, var9, st("nether_bricks"), st("nether_bricks"), false)

	for var9 = 0; var9 <= 4; var9++ {
		var10 = var8.nextInt(5)
		g.box(var9, 2, 0, var9, 2, var10, st("nether_bricks"), st("nether_bricks"), false)
	}

	for var9 = 0; var9 <= 4; var9++ {
		for var10 = 0; var10 <= 1; var10++ {
			var11 = var8.nextInt(3)
			g.box(var9, var10, 0, var9, var10, var11, st("nether_bricks"), st("nether_bricks"), false)
		}
	}

}
