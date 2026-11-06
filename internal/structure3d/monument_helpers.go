package structure3d

// monumentRoomDef is OceanMonumentPieces.RoomDefinition.
type monumentRoomDef struct {
	index       int
	connections [6]*monumentRoomDef
	hasOpening  [6]bool
	claimed     bool
	isSource    bool
	scanIndex   int
}

func dirOpposite(d int) int {
	switch d {
	case 0:
		return 1
	case 1:
		return 0
	case 2:
		return 3
	case 3:
		return 2
	case 4:
		return 5
	case 5:
		return 4
	}
	return d
}

func (r *monumentRoomDef) connect(dir int, other *monumentRoomDef) {
	r.connections[dir] = other
	other.connections[dirOpposite(dir)] = r
}

func (r *monumentRoomDef) updateOpenings() {
	for i := 0; i < 6; i++ {
		r.hasOpening[i] = r.connections[i] != nil
	}
}

func (r *monumentRoomDef) findSource(scan int) bool {
	if r.isSource {
		return true
	}
	r.scanIndex = scan
	for i := 0; i < 6; i++ {
		c := r.connections[i]
		if c != nil && r.hasOpening[i] && c.scanIndex != scan && c.findSource(scan) {
			return true
		}
	}
	return false
}

func (r *monumentRoomDef) isSpecial() bool { return r.index >= 75 }

func (r *monumentRoomDef) countOpenings() int {
	n := 0
	for i := 0; i < 6; i++ {
		if r.hasOpening[i] {
			n++
		}
	}
	return n
}

func monumentRoomIndex(x, y, z int) int { return y*25 + z*5 + x }

// waterBox is OceanMonumentPiece.generateWaterBox.
func (g *pc) waterBox(x0, y0, z0, x1, y1, z1 int) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			for z := z0; z <= z1; z++ {
				p := g.pos(x, y, z)
				if !g.insideW(p) {
					continue
				}
				cur := g.w.m[p]
				name := cur.Name
				if name == "minecraft:water" || name == "minecraft:ice" ||
					name == "minecraft:packed_ice" || name == "minecraft:blue_ice" {
					continue
				}
				if name == "" {
					if g.w.kind(p[0], p[1], p[2]) == GroundLiquid {
						continue
					}
				}
				if g.worldY(y) >= g.seaLevel && name != "minecraft:water" {
					g.place(st("air"), x, y, z)
				} else {
					g.place(fillBlock, x, y, z)
				}
			}
		}
	}
}

// chunkIntersects is OceanMonumentPiece.chunkIntersects (XZ only).
func (g *pc) chunkIntersects(x0, z0, x1, z1 int) bool {
	ax, az := g.worldX(x0, z0), g.worldZ(x0, z0)
	bx, bz := g.worldX(x1, z1), g.worldZ(x1, z1)
	minX, maxX := min(ax, bx), max(ax, bx)
	minZ, maxZ := min(az, bz), max(az, bz)
	c := g.clip
	return !(maxX < c[0] || minX > c[3] || maxZ < c[2] || minZ > c[5])
}

// defaultFloor is OceanMonumentPiece.generateDefaultFloor.
func (g *pc) defaultFloor(x, z int, openingDown bool) {
	if openingDown {
		g.box(x+0, 0, z+0, x+2, 0, z+8-1, baseGray, baseGray, false)
		g.box(x+5, 0, z+0, x+8-1, 0, z+8-1, baseGray, baseGray, false)
		g.box(x+3, 0, z+0, x+4, 0, z+2, baseGray, baseGray, false)
		g.box(x+3, 0, z+5, x+4, 0, z+8-1, baseGray, baseGray, false)
		g.box(x+3, 0, z+2, x+4, 0, z+2, baseLight, baseLight, false)
		g.box(x+3, 0, z+5, x+4, 0, z+5, baseLight, baseLight, false)
		g.box(x+2, 0, z+3, x+2, 0, z+4, baseLight, baseLight, false)
		g.box(x+5, 0, z+3, x+5, 0, z+4, baseLight, baseLight, false)
	} else {
		g.box(x+0, 0, z+0, x+8-1, 0, z+8-1, baseGray, baseGray, false)
	}
}

// boxOnFillOnly is OceanMonumentPiece.generateBoxOnFillOnly.
func (g *pc) boxOnFillOnly(x0, y0, z0, x1, y1, z1 int, s BlockState) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			for z := z0; z <= z1; z++ {
				p := g.pos(x, y, z)
				if !g.insideW(p) {
					continue
				}
				if g.w.m[p].Name == "minecraft:water" {
					g.place(s, x, y, z)
				}
			}
		}
	}
}
