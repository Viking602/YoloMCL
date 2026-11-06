package structure3d

// Code generated from OceanMonumentPieces (Minecraft 1.21.1).

var (
	baseGray  = st("prismarine")
	baseLight = st("prismarine_bricks")
	baseBlack = st("dark_prismarine")
	dotDeco   = baseLight
	lampBlock = st("sea_lantern")
	fillBlock = st("water")
)

func (g *pc) postOceanMonumentPenthouse() {
	g.box(2, -1, 2, 11, -1, 11, baseLight, baseLight, false)
	g.box(0, -1, 0, 1, -1, 11, baseGray, baseGray, false)
	g.box(12, -1, 0, 13, -1, 11, baseGray, baseGray, false)
	g.box(2, -1, 0, 11, -1, 1, baseGray, baseGray, false)
	g.box(2, -1, 12, 11, -1, 13, baseGray, baseGray, false)
	g.box(0, 0, 0, 0, 0, 13, baseLight, baseLight, false)
	g.box(13, 0, 0, 13, 0, 13, baseLight, baseLight, false)
	g.box(1, 0, 0, 12, 0, 0, baseLight, baseLight, false)
	g.box(1, 0, 13, 12, 0, 13, baseLight, baseLight, false)

	for var8 := 2; var8 <= 11; var8 += 3 {
		g.place(lampBlock, 0, 0, var8)
		g.place(lampBlock, 13, 0, var8)
		g.place(lampBlock, var8, 0, 0)
	}

	g.box(2, 0, 3, 4, 0, 9, baseLight, baseLight, false)
	g.box(9, 0, 3, 11, 0, 9, baseLight, baseLight, false)
	g.box(4, 0, 9, 9, 0, 11, baseLight, baseLight, false)
	g.place(baseLight, 5, 0, 8)
	g.place(baseLight, 8, 0, 8)
	g.place(baseLight, 10, 0, 10)
	g.place(baseLight, 3, 0, 10)
	g.box(3, 0, 3, 3, 0, 7, baseBlack, baseBlack, false)
	g.box(10, 0, 3, 10, 0, 7, baseBlack, baseBlack, false)
	g.box(6, 0, 10, 7, 0, 10, baseBlack, baseBlack, false)
	var11 := 3

	for var9 := 0; var9 < 2; var9++ {
		for var10 := 2; var10 <= 8; var10 += 3 {
			g.box(var11, 0, var10, var11, 2, var10, baseLight, baseLight, false)
		}

		var11 = 10
	}

	g.box(5, 0, 10, 5, 2, 10, baseLight, baseLight, false)
	g.box(8, 0, 10, 8, 2, 10, baseLight, baseLight, false)
	g.box(6, -1, 7, 7, -1, 8, baseBlack, baseBlack, false)
	g.waterBox(6, -1, 3, 7, -1, 4)
	// spawnElder omitted
}

func (g *pc) postOceanMonumentWingRoom() {
	if g.mainDesign == 0 {
		var var8 int
		for var8 = 0; var8 < 4; var8++ {
			g.box(10-var8, 3-var8, 20-var8, 12+var8, 3-var8, 20, baseLight, baseLight, false)
		}

		g.box(7, 0, 6, 15, 0, 16, baseLight, baseLight, false)
		g.box(6, 0, 6, 6, 3, 20, baseLight, baseLight, false)
		g.box(16, 0, 6, 16, 3, 20, baseLight, baseLight, false)
		g.box(7, 1, 7, 7, 1, 20, baseLight, baseLight, false)
		g.box(15, 1, 7, 15, 1, 20, baseLight, baseLight, false)
		g.box(7, 1, 6, 9, 3, 6, baseLight, baseLight, false)
		g.box(13, 1, 6, 15, 3, 6, baseLight, baseLight, false)
		g.box(8, 1, 7, 9, 1, 7, baseLight, baseLight, false)
		g.box(13, 1, 7, 14, 1, 7, baseLight, baseLight, false)
		g.box(9, 0, 5, 13, 0, 5, baseLight, baseLight, false)
		g.box(10, 0, 7, 12, 0, 7, baseBlack, baseBlack, false)
		g.box(8, 0, 10, 8, 0, 12, baseBlack, baseBlack, false)
		g.box(14, 0, 10, 14, 0, 12, baseBlack, baseBlack, false)

		for var8 = 18; var8 >= 7; var8 -= 3 {
			g.place(lampBlock, 6, 3, var8)
			g.place(lampBlock, 16, 3, var8)
		}

		g.place(lampBlock, 10, 0, 10)
		g.place(lampBlock, 12, 0, 10)
		g.place(lampBlock, 10, 0, 12)
		g.place(lampBlock, 12, 0, 12)
		g.place(lampBlock, 8, 3, 6)
		g.place(lampBlock, 14, 3, 6)
		g.place(baseLight, 4, 2, 4)
		g.place(lampBlock, 4, 1, 4)
		g.place(baseLight, 4, 0, 4)
		g.place(baseLight, 18, 2, 4)
		g.place(lampBlock, 18, 1, 4)
		g.place(baseLight, 18, 0, 4)
		g.place(baseLight, 4, 2, 18)
		g.place(lampBlock, 4, 1, 18)
		g.place(baseLight, 4, 0, 18)
		g.place(baseLight, 18, 2, 18)
		g.place(lampBlock, 18, 1, 18)
		g.place(baseLight, 18, 0, 18)
		g.place(baseLight, 9, 7, 20)
		g.place(baseLight, 13, 7, 20)
		g.box(6, 0, 21, 7, 4, 21, baseLight, baseLight, false)
		g.box(15, 0, 21, 16, 4, 21, baseLight, baseLight, false)
		// spawnElder omitted
	} else if g.mainDesign == 1 {
		g.box(9, 3, 18, 13, 3, 20, baseLight, baseLight, false)
		g.box(9, 0, 18, 9, 2, 18, baseLight, baseLight, false)
		g.box(13, 0, 18, 13, 2, 18, baseLight, baseLight, false)
		var12 := 9

		var var11 int
		for var11 = 0; var11 < 2; var11++ {
			g.place(baseLight, var12, 6, 20)
			g.place(lampBlock, var12, 5, 20)
			g.place(baseLight, var12, 4, 20)
			var12 = 13
		}

		g.box(7, 3, 7, 15, 3, 14, baseLight, baseLight, false)
		var12 = 10

		for var11 = 0; var11 < 2; var11++ {
			g.box(var12, 0, 10, var12, 6, 10, baseLight, baseLight, false)
			g.box(var12, 0, 12, var12, 6, 12, baseLight, baseLight, false)
			g.place(lampBlock, var12, 0, 10)
			g.place(lampBlock, var12, 0, 12)
			g.place(lampBlock, var12, 4, 10)
			g.place(lampBlock, var12, 4, 12)
			var12 = 12
		}

		var12 = 8

		for var11 = 0; var11 < 2; var11++ {
			g.box(var12, 0, 7, var12, 2, 7, baseLight, baseLight, false)
			g.box(var12, 0, 14, var12, 2, 14, baseLight, baseLight, false)
			var12 = 14
		}

		g.box(8, 3, 8, 8, 3, 13, baseBlack, baseBlack, false)
		g.box(14, 3, 8, 14, 3, 13, baseBlack, baseBlack, false)
		// spawnElder omitted
	}

}

func (g *pc) postOceanMonumentCoreRoom() {
	g.boxOnFillOnly(1, 8, 0, 14, 8, 14, baseGray)
	var9 := baseLight
	g.box(0, 7, 0, 0, 7, 15, var9, var9, false)
	g.box(15, 7, 0, 15, 7, 15, var9, var9, false)
	g.box(1, 7, 0, 15, 7, 0, var9, var9, false)
	g.box(1, 7, 15, 14, 7, 15, var9, var9, false)

	var var11 int
	for var11 = 1; var11 <= 6; var11++ {
		var9 = baseLight
		if var11 == 2 || var11 == 6 {
			var9 = baseGray
		}

		for var10 := 0; var10 <= 15; var10 += 15 {
			g.box(var10, var11, 0, var10, var11, 1, var9, var9, false)
			g.box(var10, var11, 6, var10, var11, 9, var9, var9, false)
			g.box(var10, var11, 14, var10, var11, 15, var9, var9, false)
		}

		g.box(1, var11, 0, 1, var11, 0, var9, var9, false)
		g.box(6, var11, 0, 9, var11, 0, var9, var9, false)
		g.box(14, var11, 0, 14, var11, 0, var9, var9, false)
		g.box(1, var11, 15, 14, var11, 15, var9, var9, false)
	}

	g.box(6, 3, 6, 9, 6, 9, baseBlack, baseBlack, false)
	g.box(7, 4, 7, 8, 5, 8, st("gold_block"), st("gold_block"), false)

	for var11 = 3; var11 <= 6; var11 += 3 {
		for var12 := 6; var12 <= 9; var12 += 3 {
			g.place(lampBlock, var12, var11, 6)
			g.place(lampBlock, var12, var11, 9)
		}
	}

	g.box(5, 1, 6, 5, 2, 6, baseLight, baseLight, false)
	g.box(5, 1, 9, 5, 2, 9, baseLight, baseLight, false)
	g.box(10, 1, 6, 10, 2, 6, baseLight, baseLight, false)
	g.box(10, 1, 9, 10, 2, 9, baseLight, baseLight, false)
	g.box(6, 1, 5, 6, 2, 5, baseLight, baseLight, false)
	g.box(9, 1, 5, 9, 2, 5, baseLight, baseLight, false)
	g.box(6, 1, 10, 6, 2, 10, baseLight, baseLight, false)
	g.box(9, 1, 10, 9, 2, 10, baseLight, baseLight, false)
	g.box(5, 2, 5, 5, 6, 5, baseLight, baseLight, false)
	g.box(5, 2, 10, 5, 6, 10, baseLight, baseLight, false)
	g.box(10, 2, 5, 10, 6, 5, baseLight, baseLight, false)
	g.box(10, 2, 10, 10, 6, 10, baseLight, baseLight, false)
	g.box(5, 7, 1, 5, 7, 6, baseLight, baseLight, false)
	g.box(10, 7, 1, 10, 7, 6, baseLight, baseLight, false)
	g.box(5, 7, 9, 5, 7, 14, baseLight, baseLight, false)
	g.box(10, 7, 9, 10, 7, 14, baseLight, baseLight, false)
	g.box(1, 7, 5, 6, 7, 5, baseLight, baseLight, false)
	g.box(1, 7, 10, 6, 7, 10, baseLight, baseLight, false)
	g.box(9, 7, 5, 14, 7, 5, baseLight, baseLight, false)
	g.box(9, 7, 10, 14, 7, 10, baseLight, baseLight, false)
	g.box(2, 1, 2, 2, 1, 3, baseLight, baseLight, false)
	g.box(3, 1, 2, 3, 1, 2, baseLight, baseLight, false)
	g.box(13, 1, 2, 13, 1, 3, baseLight, baseLight, false)
	g.box(12, 1, 2, 12, 1, 2, baseLight, baseLight, false)
	g.box(2, 1, 12, 2, 1, 13, baseLight, baseLight, false)
	g.box(3, 1, 13, 3, 1, 13, baseLight, baseLight, false)
	g.box(13, 1, 12, 13, 1, 13, baseLight, baseLight, false)
	g.box(12, 1, 13, 12, 1, 13, baseLight, baseLight, false)
}

func (g *pc) postOceanMonumentDoubleYZRoom() {
	var var13 BlockState
	var8 := g.roomDef.connections[2]
	var9 := g.roomDef
	var10 := var8.connections[1]
	var11 := var9.connections[1]
	if g.roomDef.index/25 > 0 {
		g.defaultFloor(0, 8, var8.hasOpening[0])
		g.defaultFloor(0, 0, var9.hasOpening[0])
	}

	if var11.connections[1] == nil {
		g.boxOnFillOnly(1, 8, 1, 6, 8, 7, baseGray)
	}

	if var10.connections[1] == nil {
		g.boxOnFillOnly(1, 8, 8, 6, 8, 14, baseGray)
	}

	var var12 int
	for var12 = 1; var12 <= 7; var12++ {
		var13 = baseLight
		if var12 == 2 || var12 == 6 {
			var13 = baseGray
		}

		g.box(0, var12, 0, 0, var12, 15, var13, var13, false)
		g.box(7, var12, 0, 7, var12, 15, var13, var13, false)
		g.box(1, var12, 0, 6, var12, 0, var13, var13, false)
		g.box(1, var12, 15, 6, var12, 15, var13, var13, false)
	}

	for var12 = 1; var12 <= 7; var12++ {
		var13 = baseBlack
		if var12 == 2 || var12 == 6 {
			var13 = lampBlock
		}

		g.box(3, var12, 7, 4, var12, 8, var13, var13, false)
	}

	if var9.hasOpening[3] {
		g.waterBox(3, 1, 0, 4, 2, 0)
	}

	if var9.hasOpening[5] {
		g.waterBox(7, 1, 3, 7, 2, 4)
	}

	if var9.hasOpening[4] {
		g.waterBox(0, 1, 3, 0, 2, 4)
	}

	if var8.hasOpening[2] {
		g.waterBox(3, 1, 15, 4, 2, 15)
	}

	if var8.hasOpening[4] {
		g.waterBox(0, 1, 11, 0, 2, 12)
	}

	if var8.hasOpening[5] {
		g.waterBox(7, 1, 11, 7, 2, 12)
	}

	if var11.hasOpening[3] {
		g.waterBox(3, 5, 0, 4, 6, 0)
	}

	if var11.hasOpening[5] {
		g.waterBox(7, 5, 3, 7, 6, 4)
		g.box(5, 4, 2, 6, 4, 5, baseLight, baseLight, false)
		g.box(6, 1, 2, 6, 3, 2, baseLight, baseLight, false)
		g.box(6, 1, 5, 6, 3, 5, baseLight, baseLight, false)
	}

	if var11.hasOpening[4] {
		g.waterBox(0, 5, 3, 0, 6, 4)
		g.box(1, 4, 2, 2, 4, 5, baseLight, baseLight, false)
		g.box(1, 1, 2, 1, 3, 2, baseLight, baseLight, false)
		g.box(1, 1, 5, 1, 3, 5, baseLight, baseLight, false)
	}

	if var10.hasOpening[2] {
		g.waterBox(3, 5, 15, 4, 6, 15)
	}

	if var10.hasOpening[4] {
		g.waterBox(0, 5, 11, 0, 6, 12)
		g.box(1, 4, 10, 2, 4, 13, baseLight, baseLight, false)
		g.box(1, 1, 10, 1, 3, 10, baseLight, baseLight, false)
		g.box(1, 1, 13, 1, 3, 13, baseLight, baseLight, false)
	}

	if var10.hasOpening[5] {
		g.waterBox(7, 5, 11, 7, 6, 12)
		g.box(5, 4, 10, 6, 4, 13, baseLight, baseLight, false)
		g.box(6, 1, 10, 6, 3, 10, baseLight, baseLight, false)
		g.box(6, 1, 13, 6, 3, 13, baseLight, baseLight, false)
	}

}

func (g *pc) postOceanMonumentDoubleXYRoom() {
	var8 := g.roomDef.connections[5]
	var9 := g.roomDef
	var10 := var9.connections[1]
	var11 := var8.connections[1]
	if g.roomDef.index/25 > 0 {
		g.defaultFloor(8, 0, var8.hasOpening[0])
		g.defaultFloor(0, 0, var9.hasOpening[0])
	}

	if var10.connections[1] == nil {
		g.boxOnFillOnly(1, 8, 1, 7, 8, 6, baseGray)
	}

	if var11.connections[1] == nil {
		g.boxOnFillOnly(8, 8, 1, 14, 8, 6, baseGray)
	}

	for var12 := 1; var12 <= 7; var12++ {
		var13 := baseLight
		if var12 == 2 || var12 == 6 {
			var13 = baseGray
		}

		g.box(0, var12, 0, 0, var12, 7, var13, var13, false)
		g.box(15, var12, 0, 15, var12, 7, var13, var13, false)
		g.box(1, var12, 0, 15, var12, 0, var13, var13, false)
		g.box(1, var12, 7, 14, var12, 7, var13, var13, false)
	}

	g.box(2, 1, 3, 2, 7, 4, baseLight, baseLight, false)
	g.box(3, 1, 2, 4, 7, 2, baseLight, baseLight, false)
	g.box(3, 1, 5, 4, 7, 5, baseLight, baseLight, false)
	g.box(13, 1, 3, 13, 7, 4, baseLight, baseLight, false)
	g.box(11, 1, 2, 12, 7, 2, baseLight, baseLight, false)
	g.box(11, 1, 5, 12, 7, 5, baseLight, baseLight, false)
	g.box(5, 1, 3, 5, 3, 4, baseLight, baseLight, false)
	g.box(10, 1, 3, 10, 3, 4, baseLight, baseLight, false)
	g.box(5, 7, 2, 10, 7, 5, baseLight, baseLight, false)
	g.box(5, 5, 2, 5, 7, 2, baseLight, baseLight, false)
	g.box(10, 5, 2, 10, 7, 2, baseLight, baseLight, false)
	g.box(5, 5, 5, 5, 7, 5, baseLight, baseLight, false)
	g.box(10, 5, 5, 10, 7, 5, baseLight, baseLight, false)
	g.place(baseLight, 6, 6, 2)
	g.place(baseLight, 9, 6, 2)
	g.place(baseLight, 6, 6, 5)
	g.place(baseLight, 9, 6, 5)
	g.box(5, 4, 3, 6, 4, 4, baseLight, baseLight, false)
	g.box(9, 4, 3, 10, 4, 4, baseLight, baseLight, false)
	g.place(lampBlock, 5, 4, 2)
	g.place(lampBlock, 5, 4, 5)
	g.place(lampBlock, 10, 4, 2)
	g.place(lampBlock, 10, 4, 5)
	if var9.hasOpening[3] {
		g.waterBox(3, 1, 0, 4, 2, 0)
	}

	if var9.hasOpening[2] {
		g.waterBox(3, 1, 7, 4, 2, 7)
	}

	if var9.hasOpening[4] {
		g.waterBox(0, 1, 3, 0, 2, 4)
	}

	if var8.hasOpening[3] {
		g.waterBox(11, 1, 0, 12, 2, 0)
	}

	if var8.hasOpening[2] {
		g.waterBox(11, 1, 7, 12, 2, 7)
	}

	if var8.hasOpening[5] {
		g.waterBox(15, 1, 3, 15, 2, 4)
	}

	if var10.hasOpening[3] {
		g.waterBox(3, 5, 0, 4, 6, 0)
	}

	if var10.hasOpening[2] {
		g.waterBox(3, 5, 7, 4, 6, 7)
	}

	if var10.hasOpening[4] {
		g.waterBox(0, 5, 3, 0, 6, 4)
	}

	if var11.hasOpening[3] {
		g.waterBox(11, 5, 0, 12, 6, 0)
	}

	if var11.hasOpening[2] {
		g.waterBox(11, 5, 7, 12, 6, 7)
	}

	if var11.hasOpening[5] {
		g.waterBox(15, 5, 3, 15, 6, 4)
	}

}

func (g *pc) postOceanMonumentDoubleZRoom() {
	var8 := g.roomDef.connections[2]
	var9 := g.roomDef
	if g.roomDef.index/25 > 0 {
		g.defaultFloor(0, 8, var8.hasOpening[0])
		g.defaultFloor(0, 0, var9.hasOpening[0])
	}

	if var9.connections[1] == nil {
		g.boxOnFillOnly(1, 4, 1, 6, 4, 7, baseGray)
	}

	if var8.connections[1] == nil {
		g.boxOnFillOnly(1, 4, 8, 6, 4, 14, baseGray)
	}

	g.box(0, 3, 0, 0, 3, 15, baseLight, baseLight, false)
	g.box(7, 3, 0, 7, 3, 15, baseLight, baseLight, false)
	g.box(1, 3, 0, 7, 3, 0, baseLight, baseLight, false)
	g.box(1, 3, 15, 6, 3, 15, baseLight, baseLight, false)
	g.box(0, 2, 0, 0, 2, 15, baseGray, baseGray, false)
	g.box(7, 2, 0, 7, 2, 15, baseGray, baseGray, false)
	g.box(1, 2, 0, 7, 2, 0, baseGray, baseGray, false)
	g.box(1, 2, 15, 6, 2, 15, baseGray, baseGray, false)
	g.box(0, 1, 0, 0, 1, 15, baseLight, baseLight, false)
	g.box(7, 1, 0, 7, 1, 15, baseLight, baseLight, false)
	g.box(1, 1, 0, 7, 1, 0, baseLight, baseLight, false)
	g.box(1, 1, 15, 6, 1, 15, baseLight, baseLight, false)
	g.box(1, 1, 1, 1, 1, 2, baseLight, baseLight, false)
	g.box(6, 1, 1, 6, 1, 2, baseLight, baseLight, false)
	g.box(1, 3, 1, 1, 3, 2, baseLight, baseLight, false)
	g.box(6, 3, 1, 6, 3, 2, baseLight, baseLight, false)
	g.box(1, 1, 13, 1, 1, 14, baseLight, baseLight, false)
	g.box(6, 1, 13, 6, 1, 14, baseLight, baseLight, false)
	g.box(1, 3, 13, 1, 3, 14, baseLight, baseLight, false)
	g.box(6, 3, 13, 6, 3, 14, baseLight, baseLight, false)
	g.box(2, 1, 6, 2, 3, 6, baseLight, baseLight, false)
	g.box(5, 1, 6, 5, 3, 6, baseLight, baseLight, false)
	g.box(2, 1, 9, 2, 3, 9, baseLight, baseLight, false)
	g.box(5, 1, 9, 5, 3, 9, baseLight, baseLight, false)
	g.box(3, 2, 6, 4, 2, 6, baseLight, baseLight, false)
	g.box(3, 2, 9, 4, 2, 9, baseLight, baseLight, false)
	g.box(2, 2, 7, 2, 2, 8, baseLight, baseLight, false)
	g.box(5, 2, 7, 5, 2, 8, baseLight, baseLight, false)
	g.place(lampBlock, 2, 2, 5)
	g.place(lampBlock, 5, 2, 5)
	g.place(lampBlock, 2, 2, 10)
	g.place(lampBlock, 5, 2, 10)
	g.place(baseLight, 2, 3, 5)
	g.place(baseLight, 5, 3, 5)
	g.place(baseLight, 2, 3, 10)
	g.place(baseLight, 5, 3, 10)
	if var9.hasOpening[3] {
		g.waterBox(3, 1, 0, 4, 2, 0)
	}

	if var9.hasOpening[5] {
		g.waterBox(7, 1, 3, 7, 2, 4)
	}

	if var9.hasOpening[4] {
		g.waterBox(0, 1, 3, 0, 2, 4)
	}

	if var8.hasOpening[2] {
		g.waterBox(3, 1, 15, 4, 2, 15)
	}

	if var8.hasOpening[4] {
		g.waterBox(0, 1, 11, 0, 2, 12)
	}

	if var8.hasOpening[5] {
		g.waterBox(7, 1, 11, 7, 2, 12)
	}

}

func (g *pc) postOceanMonumentDoubleXRoom() {
	var8 := g.roomDef.connections[5]
	var9 := g.roomDef
	if g.roomDef.index/25 > 0 {
		g.defaultFloor(8, 0, var8.hasOpening[0])
		g.defaultFloor(0, 0, var9.hasOpening[0])
	}

	if var9.connections[1] == nil {
		g.boxOnFillOnly(1, 4, 1, 7, 4, 6, baseGray)
	}

	if var8.connections[1] == nil {
		g.boxOnFillOnly(8, 4, 1, 14, 4, 6, baseGray)
	}

	g.box(0, 3, 0, 0, 3, 7, baseLight, baseLight, false)
	g.box(15, 3, 0, 15, 3, 7, baseLight, baseLight, false)
	g.box(1, 3, 0, 15, 3, 0, baseLight, baseLight, false)
	g.box(1, 3, 7, 14, 3, 7, baseLight, baseLight, false)
	g.box(0, 2, 0, 0, 2, 7, baseGray, baseGray, false)
	g.box(15, 2, 0, 15, 2, 7, baseGray, baseGray, false)
	g.box(1, 2, 0, 15, 2, 0, baseGray, baseGray, false)
	g.box(1, 2, 7, 14, 2, 7, baseGray, baseGray, false)
	g.box(0, 1, 0, 0, 1, 7, baseLight, baseLight, false)
	g.box(15, 1, 0, 15, 1, 7, baseLight, baseLight, false)
	g.box(1, 1, 0, 15, 1, 0, baseLight, baseLight, false)
	g.box(1, 1, 7, 14, 1, 7, baseLight, baseLight, false)
	g.box(5, 1, 0, 10, 1, 4, baseLight, baseLight, false)
	g.box(6, 2, 0, 9, 2, 3, baseGray, baseGray, false)
	g.box(5, 3, 0, 10, 3, 4, baseLight, baseLight, false)
	g.place(lampBlock, 6, 2, 3)
	g.place(lampBlock, 9, 2, 3)
	if var9.hasOpening[3] {
		g.waterBox(3, 1, 0, 4, 2, 0)
	}

	if var9.hasOpening[2] {
		g.waterBox(3, 1, 7, 4, 2, 7)
	}

	if var9.hasOpening[4] {
		g.waterBox(0, 1, 3, 0, 2, 4)
	}

	if var8.hasOpening[3] {
		g.waterBox(11, 1, 0, 12, 2, 0)
	}

	if var8.hasOpening[2] {
		g.waterBox(11, 1, 7, 12, 2, 7)
	}

	if var8.hasOpening[5] {
		g.waterBox(15, 1, 3, 15, 2, 4)
	}

}

func (g *pc) postOceanMonumentDoubleYRoom() {
	if g.roomDef.index/25 > 0 {
		g.defaultFloor(0, 0, g.roomDef.hasOpening[0])
	}

	var8 := g.roomDef.connections[1]
	if var8.connections[1] == nil {
		g.boxOnFillOnly(1, 8, 1, 6, 8, 6, baseGray)
	}

	g.box(0, 4, 0, 0, 4, 7, baseLight, baseLight, false)
	g.box(7, 4, 0, 7, 4, 7, baseLight, baseLight, false)
	g.box(1, 4, 0, 6, 4, 0, baseLight, baseLight, false)
	g.box(1, 4, 7, 6, 4, 7, baseLight, baseLight, false)
	g.box(2, 4, 1, 2, 4, 2, baseLight, baseLight, false)
	g.box(1, 4, 2, 1, 4, 2, baseLight, baseLight, false)
	g.box(5, 4, 1, 5, 4, 2, baseLight, baseLight, false)
	g.box(6, 4, 2, 6, 4, 2, baseLight, baseLight, false)
	g.box(2, 4, 5, 2, 4, 6, baseLight, baseLight, false)
	g.box(1, 4, 5, 1, 4, 5, baseLight, baseLight, false)
	g.box(5, 4, 5, 5, 4, 6, baseLight, baseLight, false)
	g.box(6, 4, 5, 6, 4, 5, baseLight, baseLight, false)
	var9 := g.roomDef

	for var10 := 1; var10 <= 5; var10 += 4 {
		var11 := 0
		if var9.hasOpening[3] {
			g.box(2, var10, var11, 2, var10+2, var11, baseLight, baseLight, false)
			g.box(5, var10, var11, 5, var10+2, var11, baseLight, baseLight, false)
			g.box(3, var10+2, var11, 4, var10+2, var11, baseLight, baseLight, false)
		} else {
			g.box(0, var10, var11, 7, var10+2, var11, baseLight, baseLight, false)
			g.box(0, var10+1, var11, 7, var10+1, var11, baseGray, baseGray, false)
		}

		var11 = 7
		if var9.hasOpening[2] {
			g.box(2, var10, var11, 2, var10+2, var11, baseLight, baseLight, false)
			g.box(5, var10, var11, 5, var10+2, var11, baseLight, baseLight, false)
			g.box(3, var10+2, var11, 4, var10+2, var11, baseLight, baseLight, false)
		} else {
			g.box(0, var10, var11, 7, var10+2, var11, baseLight, baseLight, false)
			g.box(0, var10+1, var11, 7, var10+1, var11, baseGray, baseGray, false)
		}

		var12 := 0
		if var9.hasOpening[4] {
			g.box(var12, var10, 2, var12, var10+2, 2, baseLight, baseLight, false)
			g.box(var12, var10, 5, var12, var10+2, 5, baseLight, baseLight, false)
			g.box(var12, var10+2, 3, var12, var10+2, 4, baseLight, baseLight, false)
		} else {
			g.box(var12, var10, 0, var12, var10+2, 7, baseLight, baseLight, false)
			g.box(var12, var10+1, 0, var12, var10+1, 7, baseGray, baseGray, false)
		}

		var12 = 7
		if var9.hasOpening[5] {
			g.box(var12, var10, 2, var12, var10+2, 2, baseLight, baseLight, false)
			g.box(var12, var10, 5, var12, var10+2, 5, baseLight, baseLight, false)
			g.box(var12, var10+2, 3, var12, var10+2, 4, baseLight, baseLight, false)
		} else {
			g.box(var12, var10, 0, var12, var10+2, 7, baseLight, baseLight, false)
			g.box(var12, var10+1, 0, var12, var10+1, 7, baseGray, baseGray, false)
		}

		var9 = var8
	}

}

func (g *pc) postOceanMonumentSimpleTopRoom() {
	if g.roomDef.index/25 > 0 {
		g.defaultFloor(0, 0, g.roomDef.hasOpening[0])
	}

	if g.roomDef.connections[1] == nil {
		g.boxOnFillOnly(1, 4, 1, 6, 4, 6, baseGray)
	}

	for var8 := 1; var8 <= 6; var8++ {
		for var9 := 1; var9 <= 6; var9++ {
			if g.rnd.nextInt(3) != 0 {
				var10 := 2 + (func() int {
					if g.rnd.nextInt(4) == 0 {
						return 0
					}
					return 1
				})()
				var11 := st("wet_sponge")
				g.box(var8, var10, var9, var8, 3, var9, var11, var11, false)
			}
		}
	}

	g.box(0, 1, 0, 0, 1, 7, baseLight, baseLight, false)
	g.box(7, 1, 0, 7, 1, 7, baseLight, baseLight, false)
	g.box(1, 1, 0, 6, 1, 0, baseLight, baseLight, false)
	g.box(1, 1, 7, 6, 1, 7, baseLight, baseLight, false)
	g.box(0, 2, 0, 0, 2, 7, baseBlack, baseBlack, false)
	g.box(7, 2, 0, 7, 2, 7, baseBlack, baseBlack, false)
	g.box(1, 2, 0, 6, 2, 0, baseBlack, baseBlack, false)
	g.box(1, 2, 7, 6, 2, 7, baseBlack, baseBlack, false)
	g.box(0, 3, 0, 0, 3, 7, baseLight, baseLight, false)
	g.box(7, 3, 0, 7, 3, 7, baseLight, baseLight, false)
	g.box(1, 3, 0, 6, 3, 0, baseLight, baseLight, false)
	g.box(1, 3, 7, 6, 3, 7, baseLight, baseLight, false)
	g.box(0, 1, 3, 0, 2, 4, baseBlack, baseBlack, false)
	g.box(7, 1, 3, 7, 2, 4, baseBlack, baseBlack, false)
	g.box(3, 1, 0, 4, 2, 0, baseBlack, baseBlack, false)
	g.box(3, 1, 7, 4, 2, 7, baseBlack, baseBlack, false)
	if g.roomDef.hasOpening[3] {
		g.waterBox(3, 1, 0, 4, 2, 0)
	}

}

func (g *pc) postOceanMonumentSimpleRoom() {
	if g.roomDef.index/25 > 0 {
		g.defaultFloor(0, 0, g.roomDef.hasOpening[0])
	}

	if g.roomDef.connections[1] == nil {
		g.boxOnFillOnly(1, 4, 1, 6, 4, 6, baseGray)
	}

	var8 := g.mainDesign != 0 && g.rnd.nextBool() && !g.roomDef.hasOpening[0] && !g.roomDef.hasOpening[1] && g.roomDef.countOpenings() > 1
	if g.mainDesign == 0 {
		g.box(0, 1, 0, 2, 1, 2, baseLight, baseLight, false)
		g.box(0, 3, 0, 2, 3, 2, baseLight, baseLight, false)
		g.box(0, 2, 0, 0, 2, 2, baseGray, baseGray, false)
		g.box(1, 2, 0, 2, 2, 0, baseGray, baseGray, false)
		g.place(lampBlock, 1, 2, 1)
		g.box(5, 1, 0, 7, 1, 2, baseLight, baseLight, false)
		g.box(5, 3, 0, 7, 3, 2, baseLight, baseLight, false)
		g.box(7, 2, 0, 7, 2, 2, baseGray, baseGray, false)
		g.box(5, 2, 0, 6, 2, 0, baseGray, baseGray, false)
		g.place(lampBlock, 6, 2, 1)
		g.box(0, 1, 5, 2, 1, 7, baseLight, baseLight, false)
		g.box(0, 3, 5, 2, 3, 7, baseLight, baseLight, false)
		g.box(0, 2, 5, 0, 2, 7, baseGray, baseGray, false)
		g.box(1, 2, 7, 2, 2, 7, baseGray, baseGray, false)
		g.place(lampBlock, 1, 2, 6)
		g.box(5, 1, 5, 7, 1, 7, baseLight, baseLight, false)
		g.box(5, 3, 5, 7, 3, 7, baseLight, baseLight, false)
		g.box(7, 2, 5, 7, 2, 7, baseGray, baseGray, false)
		g.box(5, 2, 7, 6, 2, 7, baseGray, baseGray, false)
		g.place(lampBlock, 6, 2, 6)
		if g.roomDef.hasOpening[3] {
			g.box(3, 3, 0, 4, 3, 0, baseLight, baseLight, false)
		} else {
			g.box(3, 3, 0, 4, 3, 1, baseLight, baseLight, false)
			g.box(3, 2, 0, 4, 2, 0, baseGray, baseGray, false)
			g.box(3, 1, 0, 4, 1, 1, baseLight, baseLight, false)
		}

		if g.roomDef.hasOpening[2] {
			g.box(3, 3, 7, 4, 3, 7, baseLight, baseLight, false)
		} else {
			g.box(3, 3, 6, 4, 3, 7, baseLight, baseLight, false)
			g.box(3, 2, 7, 4, 2, 7, baseGray, baseGray, false)
			g.box(3, 1, 6, 4, 1, 7, baseLight, baseLight, false)
		}

		if g.roomDef.hasOpening[4] {
			g.box(0, 3, 3, 0, 3, 4, baseLight, baseLight, false)
		} else {
			g.box(0, 3, 3, 1, 3, 4, baseLight, baseLight, false)
			g.box(0, 2, 3, 0, 2, 4, baseGray, baseGray, false)
			g.box(0, 1, 3, 1, 1, 4, baseLight, baseLight, false)
		}

		if g.roomDef.hasOpening[5] {
			g.box(7, 3, 3, 7, 3, 4, baseLight, baseLight, false)
		} else {
			g.box(6, 3, 3, 7, 3, 4, baseLight, baseLight, false)
			g.box(7, 2, 3, 7, 2, 4, baseGray, baseGray, false)
			g.box(6, 1, 3, 7, 1, 4, baseLight, baseLight, false)
		}
	} else if g.mainDesign == 1 {
		g.box(2, 1, 2, 2, 3, 2, baseLight, baseLight, false)
		g.box(2, 1, 5, 2, 3, 5, baseLight, baseLight, false)
		g.box(5, 1, 5, 5, 3, 5, baseLight, baseLight, false)
		g.box(5, 1, 2, 5, 3, 2, baseLight, baseLight, false)
		g.place(lampBlock, 2, 2, 2)
		g.place(lampBlock, 2, 2, 5)
		g.place(lampBlock, 5, 2, 5)
		g.place(lampBlock, 5, 2, 2)
		g.box(0, 1, 0, 1, 3, 0, baseLight, baseLight, false)
		g.box(0, 1, 1, 0, 3, 1, baseLight, baseLight, false)
		g.box(0, 1, 7, 1, 3, 7, baseLight, baseLight, false)
		g.box(0, 1, 6, 0, 3, 6, baseLight, baseLight, false)
		g.box(6, 1, 7, 7, 3, 7, baseLight, baseLight, false)
		g.box(7, 1, 6, 7, 3, 6, baseLight, baseLight, false)
		g.box(6, 1, 0, 7, 3, 0, baseLight, baseLight, false)
		g.box(7, 1, 1, 7, 3, 1, baseLight, baseLight, false)
		g.place(baseGray, 1, 2, 0)
		g.place(baseGray, 0, 2, 1)
		g.place(baseGray, 1, 2, 7)
		g.place(baseGray, 0, 2, 6)
		g.place(baseGray, 6, 2, 7)
		g.place(baseGray, 7, 2, 6)
		g.place(baseGray, 6, 2, 0)
		g.place(baseGray, 7, 2, 1)
		if !g.roomDef.hasOpening[3] {
			g.box(1, 3, 0, 6, 3, 0, baseLight, baseLight, false)
			g.box(1, 2, 0, 6, 2, 0, baseGray, baseGray, false)
			g.box(1, 1, 0, 6, 1, 0, baseLight, baseLight, false)
		}

		if !g.roomDef.hasOpening[2] {
			g.box(1, 3, 7, 6, 3, 7, baseLight, baseLight, false)
			g.box(1, 2, 7, 6, 2, 7, baseGray, baseGray, false)
			g.box(1, 1, 7, 6, 1, 7, baseLight, baseLight, false)
		}

		if !g.roomDef.hasOpening[4] {
			g.box(0, 3, 1, 0, 3, 6, baseLight, baseLight, false)
			g.box(0, 2, 1, 0, 2, 6, baseGray, baseGray, false)
			g.box(0, 1, 1, 0, 1, 6, baseLight, baseLight, false)
		}

		if !g.roomDef.hasOpening[5] {
			g.box(7, 3, 1, 7, 3, 6, baseLight, baseLight, false)
			g.box(7, 2, 1, 7, 2, 6, baseGray, baseGray, false)
			g.box(7, 1, 1, 7, 1, 6, baseLight, baseLight, false)
		}
	} else if g.mainDesign == 2 {
		g.box(0, 1, 0, 0, 1, 7, baseLight, baseLight, false)
		g.box(7, 1, 0, 7, 1, 7, baseLight, baseLight, false)
		g.box(1, 1, 0, 6, 1, 0, baseLight, baseLight, false)
		g.box(1, 1, 7, 6, 1, 7, baseLight, baseLight, false)
		g.box(0, 2, 0, 0, 2, 7, baseBlack, baseBlack, false)
		g.box(7, 2, 0, 7, 2, 7, baseBlack, baseBlack, false)
		g.box(1, 2, 0, 6, 2, 0, baseBlack, baseBlack, false)
		g.box(1, 2, 7, 6, 2, 7, baseBlack, baseBlack, false)
		g.box(0, 3, 0, 0, 3, 7, baseLight, baseLight, false)
		g.box(7, 3, 0, 7, 3, 7, baseLight, baseLight, false)
		g.box(1, 3, 0, 6, 3, 0, baseLight, baseLight, false)
		g.box(1, 3, 7, 6, 3, 7, baseLight, baseLight, false)
		g.box(0, 1, 3, 0, 2, 4, baseBlack, baseBlack, false)
		g.box(7, 1, 3, 7, 2, 4, baseBlack, baseBlack, false)
		g.box(3, 1, 0, 4, 2, 0, baseBlack, baseBlack, false)
		g.box(3, 1, 7, 4, 2, 7, baseBlack, baseBlack, false)
		if g.roomDef.hasOpening[3] {
			g.waterBox(3, 1, 0, 4, 2, 0)
		}

		if g.roomDef.hasOpening[2] {
			g.waterBox(3, 1, 7, 4, 2, 7)
		}

		if g.roomDef.hasOpening[4] {
			g.waterBox(0, 1, 3, 0, 2, 4)
		}

		if g.roomDef.hasOpening[5] {
			g.waterBox(7, 1, 3, 7, 2, 4)
		}
	}

	if var8 {
		g.box(3, 1, 3, 4, 1, 4, baseLight, baseLight, false)
		g.box(3, 2, 3, 4, 2, 4, baseGray, baseGray, false)
		g.box(3, 3, 3, 4, 3, 4, baseLight, baseLight, false)
	}

}

func (g *pc) postOceanMonumentEntryRoom() {
	g.box(0, 3, 0, 2, 3, 7, baseLight, baseLight, false)
	g.box(5, 3, 0, 7, 3, 7, baseLight, baseLight, false)
	g.box(0, 2, 0, 1, 2, 7, baseLight, baseLight, false)
	g.box(6, 2, 0, 7, 2, 7, baseLight, baseLight, false)
	g.box(0, 1, 0, 0, 1, 7, baseLight, baseLight, false)
	g.box(7, 1, 0, 7, 1, 7, baseLight, baseLight, false)
	g.box(0, 1, 7, 7, 3, 7, baseLight, baseLight, false)
	g.box(1, 1, 0, 2, 3, 0, baseLight, baseLight, false)
	g.box(5, 1, 0, 6, 3, 0, baseLight, baseLight, false)
	if g.roomDef.hasOpening[2] {
		g.waterBox(3, 1, 7, 4, 2, 7)
	}

	if g.roomDef.hasOpening[4] {
		g.waterBox(0, 1, 3, 1, 2, 4)
	}

	if g.roomDef.hasOpening[5] {
		g.waterBox(6, 1, 3, 7, 2, 4)
	}

}

func (g *pc) postMonumentBuilding() {
	var8 := max(g.seaLevel, 64) - g.bb[1]
	g.waterBox(0, 0, 0, 58, var8, 58)
	g.generateWing(false, 0)
	g.generateWing(true, 33)
	g.generateEntranceArchs()
	g.generateEntranceWall()
	g.generateRoofPiece()
	g.generateLowerWall()
	g.generateMiddleWall()
	g.generateUpperWall()

	for var9 := 0; var9 < 7; var9++ {
		for var10 := 0; var10 < 7; {
			if var10 == 0 && var9 == 3 {
				var10 = 6
			}
			var11 := var9 * 9
			var12 := var10 * 9
			for var13 := 0; var13 < 4; var13++ {
				for var14 := 0; var14 < 4; var14++ {
					g.place(baseLight, var11+var13, 0, var12+var14)
					g.fillDown(baseLight, var11+var13, -1, var12+var14)
				}
			}
			if var9 != 0 && var9 != 6 {
				var10 += 6
			} else {
				var10++
			}
		}
	}

	for var9 := 0; var9 < 5; var9++ {
		g.waterBox(-1-var9, 0+var9*2, -1-var9, -1-var9, 23, 58+var9)
		g.waterBox(58+var9, 0+var9*2, -1-var9, 58+var9, 23, 58+var9)
		g.waterBox(0-var9, 0+var9*2, -1-var9, 57+var9, 23, -1-var9)
		g.waterBox(0-var9, 0+var9*2, 58+var9, 57+var9, 23, 58+var9)
	}

}

func (g *pc) generateWing(var1 bool, var2 int) {
	if g.chunkIntersects(var2, 0, var2+23, 20) {
		g.box(var2+0, 0, 0, var2+24, 0, 20, baseGray, baseGray, false)
		g.waterBox(var2+0, 1, 0, var2+24, 10, 20)

		var var7 int
		for var7 = 0; var7 < 4; var7++ {
			g.box(var2+var7, var7+1, var7, var2+var7, var7+1, 20, baseLight, baseLight, false)
			g.box(var2+var7+7, var7+5, var7+7, var2+var7+7, var7+5, 20, baseLight, baseLight, false)
			g.box(var2+17-var7, var7+5, var7+7, var2+17-var7, var7+5, 20, baseLight, baseLight, false)
			g.box(var2+24-var7, var7+1, var7, var2+24-var7, var7+1, 20, baseLight, baseLight, false)
			g.box(var2+var7+1, var7+1, var7, var2+23-var7, var7+1, var7, baseLight, baseLight, false)
			g.box(var2+var7+8, var7+5, var7+7, var2+16-var7, var7+5, var7+7, baseLight, baseLight, false)
		}

		g.box(var2+4, 4, 4, var2+6, 4, 20, baseGray, baseGray, false)
		g.box(var2+7, 4, 4, var2+17, 4, 6, baseGray, baseGray, false)
		g.box(var2+18, 4, 4, var2+20, 4, 20, baseGray, baseGray, false)
		g.box(var2+11, 8, 11, var2+13, 8, 20, baseGray, baseGray, false)
		g.place(dotDeco, var2+12, 9, 12)
		g.place(dotDeco, var2+12, 9, 15)
		g.place(dotDeco, var2+12, 9, 18)
		var7 = var2 + (func() int {
			if var1 {
				return 19
			}
			return 5
		})()
		var8 := var2 + (func() int {
			if var1 {
				return 5
			}
			return 19
		})()

		var var9 int
		for var9 = 20; var9 >= 5; var9 -= 3 {
			g.place(dotDeco, var7, 5, var9)
		}

		for var9 = 19; var9 >= 7; var9 -= 3 {
			g.place(dotDeco, var8, 5, var9)
		}

		for var9 = 0; var9 < 4; var9++ {
			var10 := (func() int {
				if var1 {
					return var2 + 24 - (17 - var9*3)
				}
				return var2 + 17 - var9*3
			})()
			g.place(dotDeco, var10, 5, 5)
		}

		g.place(dotDeco, var8, 5, 5)
		g.box(var2+11, 1, 12, var2+13, 7, 12, baseGray, baseGray, false)
		g.box(var2+12, 1, 11, var2+12, 7, 13, baseGray, baseGray, false)
	}

}

func (g *pc) generateEntranceArchs() {
	if g.chunkIntersects(22, 5, 35, 17) {
		g.waterBox(25, 0, 0, 32, 8, 20)

		for var4 := 0; var4 < 4; var4++ {
			g.box(24, 2, 5+var4*4, 24, 4, 5+var4*4, baseLight, baseLight, false)
			g.box(22, 4, 5+var4*4, 23, 4, 5+var4*4, baseLight, baseLight, false)
			g.place(baseLight, 25, 5, 5+var4*4)
			g.place(baseLight, 26, 6, 5+var4*4)
			g.place(lampBlock, 26, 5, 5+var4*4)
			g.box(33, 2, 5+var4*4, 33, 4, 5+var4*4, baseLight, baseLight, false)
			g.box(34, 4, 5+var4*4, 35, 4, 5+var4*4, baseLight, baseLight, false)
			g.place(baseLight, 32, 5, 5+var4*4)
			g.place(baseLight, 31, 6, 5+var4*4)
			g.place(lampBlock, 31, 5, 5+var4*4)
			g.box(27, 6, 5+var4*4, 30, 6, 5+var4*4, baseGray, baseGray, false)
		}
	}

}

func (g *pc) generateEntranceWall() {
	if g.chunkIntersects(15, 20, 42, 21) {
		g.box(15, 0, 21, 42, 0, 21, baseGray, baseGray, false)
		g.waterBox(26, 1, 21, 31, 3, 21)
		g.box(21, 12, 21, 36, 12, 21, baseGray, baseGray, false)
		g.box(17, 11, 21, 40, 11, 21, baseGray, baseGray, false)
		g.box(16, 10, 21, 41, 10, 21, baseGray, baseGray, false)
		g.box(15, 7, 21, 42, 9, 21, baseGray, baseGray, false)
		g.box(16, 6, 21, 41, 6, 21, baseGray, baseGray, false)
		g.box(17, 5, 21, 40, 5, 21, baseGray, baseGray, false)
		g.box(21, 4, 21, 36, 4, 21, baseGray, baseGray, false)
		g.box(22, 3, 21, 26, 3, 21, baseGray, baseGray, false)
		g.box(31, 3, 21, 35, 3, 21, baseGray, baseGray, false)
		g.box(23, 2, 21, 25, 2, 21, baseGray, baseGray, false)
		g.box(32, 2, 21, 34, 2, 21, baseGray, baseGray, false)
		g.box(28, 4, 20, 29, 4, 21, baseLight, baseLight, false)
		g.place(baseLight, 27, 3, 21)
		g.place(baseLight, 30, 3, 21)
		g.place(baseLight, 26, 2, 21)
		g.place(baseLight, 31, 2, 21)
		g.place(baseLight, 25, 1, 21)
		g.place(baseLight, 32, 1, 21)

		var var4 int
		for var4 = 0; var4 < 7; var4++ {
			g.place(baseBlack, 28-var4, 6+var4, 21)
			g.place(baseBlack, 29+var4, 6+var4, 21)
		}

		for var4 = 0; var4 < 4; var4++ {
			g.place(baseBlack, 28-var4, 9+var4, 21)
			g.place(baseBlack, 29+var4, 9+var4, 21)
		}

		g.place(baseBlack, 28, 12, 21)
		g.place(baseBlack, 29, 12, 21)

		for var4 = 0; var4 < 3; var4++ {
			g.place(baseBlack, 22-var4*2, 8, 21)
			g.place(baseBlack, 22-var4*2, 9, 21)
			g.place(baseBlack, 35+var4*2, 8, 21)
			g.place(baseBlack, 35+var4*2, 9, 21)
		}

		g.waterBox(15, 13, 21, 42, 15, 21)
		g.waterBox(15, 1, 21, 15, 6, 21)
		g.waterBox(16, 1, 21, 16, 5, 21)
		g.waterBox(17, 1, 21, 20, 4, 21)
		g.waterBox(21, 1, 21, 21, 3, 21)
		g.waterBox(22, 1, 21, 22, 2, 21)
		g.waterBox(23, 1, 21, 24, 1, 21)
		g.waterBox(42, 1, 21, 42, 6, 21)
		g.waterBox(41, 1, 21, 41, 5, 21)
		g.waterBox(37, 1, 21, 40, 4, 21)
		g.waterBox(36, 1, 21, 36, 3, 21)
		g.waterBox(33, 1, 21, 34, 1, 21)
		g.waterBox(35, 1, 21, 35, 2, 21)
	}

}

func (g *pc) generateRoofPiece() {
	if g.chunkIntersects(21, 21, 36, 36) {
		g.box(21, 0, 22, 36, 0, 36, baseGray, baseGray, false)
		g.waterBox(21, 1, 22, 36, 23, 36)

		for var4 := 0; var4 < 4; var4++ {
			g.box(21+var4, 13+var4, 21+var4, 36-var4, 13+var4, 21+var4, baseLight, baseLight, false)
			g.box(21+var4, 13+var4, 36-var4, 36-var4, 13+var4, 36-var4, baseLight, baseLight, false)
			g.box(21+var4, 13+var4, 22+var4, 21+var4, 13+var4, 35-var4, baseLight, baseLight, false)
			g.box(36-var4, 13+var4, 22+var4, 36-var4, 13+var4, 35-var4, baseLight, baseLight, false)
		}

		g.box(25, 16, 25, 32, 16, 32, baseGray, baseGray, false)
		g.box(25, 17, 25, 25, 19, 25, baseLight, baseLight, false)
		g.box(32, 17, 25, 32, 19, 25, baseLight, baseLight, false)
		g.box(25, 17, 32, 25, 19, 32, baseLight, baseLight, false)
		g.box(32, 17, 32, 32, 19, 32, baseLight, baseLight, false)
		g.place(baseLight, 26, 20, 26)
		g.place(baseLight, 27, 21, 27)
		g.place(lampBlock, 27, 20, 27)
		g.place(baseLight, 26, 20, 31)
		g.place(baseLight, 27, 21, 30)
		g.place(lampBlock, 27, 20, 30)
		g.place(baseLight, 31, 20, 31)
		g.place(baseLight, 30, 21, 30)
		g.place(lampBlock, 30, 20, 30)
		g.place(baseLight, 31, 20, 26)
		g.place(baseLight, 30, 21, 27)
		g.place(lampBlock, 30, 20, 27)
		g.box(28, 21, 27, 29, 21, 27, baseGray, baseGray, false)
		g.box(27, 21, 28, 27, 21, 29, baseGray, baseGray, false)
		g.box(28, 21, 30, 29, 21, 30, baseGray, baseGray, false)
		g.box(30, 21, 28, 30, 21, 29, baseGray, baseGray, false)
	}

}

func (g *pc) generateLowerWall() {
	var var4 int
	if g.chunkIntersects(0, 21, 6, 58) {
		g.box(0, 0, 21, 6, 0, 57, baseGray, baseGray, false)
		g.waterBox(0, 1, 21, 6, 7, 57)
		g.box(4, 4, 21, 6, 4, 53, baseGray, baseGray, false)

		for var4 = 0; var4 < 4; var4++ {
			g.box(var4, var4+1, 21, var4, var4+1, 57-var4, baseLight, baseLight, false)
		}

		for var4 = 23; var4 < 53; var4 += 3 {
			g.place(dotDeco, 5, 5, var4)
		}

		g.place(dotDeco, 5, 5, 52)

		for var4 = 0; var4 < 4; var4++ {
			g.box(var4, var4+1, 21, var4, var4+1, 57-var4, baseLight, baseLight, false)
		}

		g.box(4, 1, 52, 6, 3, 52, baseGray, baseGray, false)
		g.box(5, 1, 51, 5, 3, 53, baseGray, baseGray, false)
	}

	if g.chunkIntersects(51, 21, 58, 58) {
		g.box(51, 0, 21, 57, 0, 57, baseGray, baseGray, false)
		g.waterBox(51, 1, 21, 57, 7, 57)
		g.box(51, 4, 21, 53, 4, 53, baseGray, baseGray, false)

		for var4 = 0; var4 < 4; var4++ {
			g.box(57-var4, var4+1, 21, 57-var4, var4+1, 57-var4, baseLight, baseLight, false)
		}

		for var4 = 23; var4 < 53; var4 += 3 {
			g.place(dotDeco, 52, 5, var4)
		}

		g.place(dotDeco, 52, 5, 52)
		g.box(51, 1, 52, 53, 3, 52, baseGray, baseGray, false)
		g.box(52, 1, 51, 52, 3, 53, baseGray, baseGray, false)
	}

	if g.chunkIntersects(0, 51, 57, 57) {
		g.box(7, 0, 51, 50, 0, 57, baseGray, baseGray, false)
		g.waterBox(7, 1, 51, 50, 10, 57)

		for var4 = 0; var4 < 4; var4++ {
			g.box(var4+1, var4+1, 57-var4, 56-var4, var4+1, 57-var4, baseLight, baseLight, false)
		}
	}

}

func (g *pc) generateMiddleWall() {
	var var4 int
	if g.chunkIntersects(7, 21, 13, 50) {
		g.box(7, 0, 21, 13, 0, 50, baseGray, baseGray, false)
		g.waterBox(7, 1, 21, 13, 10, 50)
		g.box(11, 8, 21, 13, 8, 53, baseGray, baseGray, false)

		for var4 = 0; var4 < 4; var4++ {
			g.box(var4+7, var4+5, 21, var4+7, var4+5, 54, baseLight, baseLight, false)
		}

		for var4 = 21; var4 <= 45; var4 += 3 {
			g.place(dotDeco, 12, 9, var4)
		}
	}

	if g.chunkIntersects(44, 21, 50, 54) {
		g.box(44, 0, 21, 50, 0, 50, baseGray, baseGray, false)
		g.waterBox(44, 1, 21, 50, 10, 50)
		g.box(44, 8, 21, 46, 8, 53, baseGray, baseGray, false)

		for var4 = 0; var4 < 4; var4++ {
			g.box(50-var4, var4+5, 21, 50-var4, var4+5, 54, baseLight, baseLight, false)
		}

		for var4 = 21; var4 <= 45; var4 += 3 {
			g.place(dotDeco, 45, 9, var4)
		}
	}

	if g.chunkIntersects(8, 44, 49, 54) {
		g.box(14, 0, 44, 43, 0, 50, baseGray, baseGray, false)
		g.waterBox(14, 1, 44, 43, 10, 50)

		for var4 = 12; var4 <= 45; var4 += 3 {
			g.place(dotDeco, var4, 9, 45)
			g.place(dotDeco, var4, 9, 52)
			if var4 == 12 || var4 == 18 || var4 == 24 || var4 == 33 || var4 == 39 || var4 == 45 {
				g.place(dotDeco, var4, 9, 47)
				g.place(dotDeco, var4, 9, 50)
				g.place(dotDeco, var4, 10, 45)
				g.place(dotDeco, var4, 10, 46)
				g.place(dotDeco, var4, 10, 51)
				g.place(dotDeco, var4, 10, 52)
				g.place(dotDeco, var4, 11, 47)
				g.place(dotDeco, var4, 11, 50)
				g.place(dotDeco, var4, 12, 48)
				g.place(dotDeco, var4, 12, 49)
			}
		}

		for var4 = 0; var4 < 3; var4++ {
			g.box(8+var4, 5+var4, 54, 49-var4, 5+var4, 54, baseGray, baseGray, false)
		}

		g.box(11, 8, 54, 46, 8, 54, baseLight, baseLight, false)
		g.box(14, 8, 44, 43, 8, 53, baseGray, baseGray, false)
	}

}

func (g *pc) generateUpperWall() {
	var var4 int
	if g.chunkIntersects(14, 21, 20, 43) {
		g.box(14, 0, 21, 20, 0, 43, baseGray, baseGray, false)
		g.waterBox(14, 1, 22, 20, 14, 43)
		g.box(18, 12, 22, 20, 12, 39, baseGray, baseGray, false)
		g.box(18, 12, 21, 20, 12, 21, baseLight, baseLight, false)

		for var4 = 0; var4 < 4; var4++ {
			g.box(var4+14, var4+9, 21, var4+14, var4+9, 43-var4, baseLight, baseLight, false)
		}

		for var4 = 23; var4 <= 39; var4 += 3 {
			g.place(dotDeco, 19, 13, var4)
		}
	}

	if g.chunkIntersects(37, 21, 43, 43) {
		g.box(37, 0, 21, 43, 0, 43, baseGray, baseGray, false)
		g.waterBox(37, 1, 22, 43, 14, 43)
		g.box(37, 12, 22, 39, 12, 39, baseGray, baseGray, false)
		g.box(37, 12, 21, 39, 12, 21, baseLight, baseLight, false)

		for var4 = 0; var4 < 4; var4++ {
			g.box(43-var4, var4+9, 21, 43-var4, var4+9, 43-var4, baseLight, baseLight, false)
		}

		for var4 = 23; var4 <= 39; var4 += 3 {
			g.place(dotDeco, 38, 13, var4)
		}
	}

	if g.chunkIntersects(15, 37, 42, 43) {
		g.box(21, 0, 37, 36, 0, 43, baseGray, baseGray, false)
		g.waterBox(21, 1, 37, 36, 14, 43)
		g.box(21, 12, 37, 36, 12, 39, baseGray, baseGray, false)

		for var4 = 0; var4 < 4; var4++ {
			g.box(15+var4, var4+9, 43-var4, 42-var4, var4+9, 43-var4, baseLight, baseLight, false)
		}

		for var4 = 21; var4 <= 36; var4 += 3 {
			g.place(dotDeco, var4, 13, 38)
		}
	}

}
