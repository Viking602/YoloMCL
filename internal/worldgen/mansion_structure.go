package worldgen

// Woodland mansion structural templates from MansionPiecePlacer.createMansion:
// entrance, outer walls, roofs, corridor floors, carpets and indoor walls/doors.
// These draw no further random after the room-layout RNG (door picks reuse the
// same NextInt already consumed for rooms).

const (
	mansionEntranceX = 7
	mansionEntranceY = 4
	mansionStartX    = mansionEntranceX + 1 // 8
	mansionStartY    = mansionEntranceY + 1 // 5
)

type mansionPlace struct {
	x, y, z int
	rot     int // index into Rotations
	wall    string
}

func (p *mansionPlacer) addAt(name string, x, y, z, rot int) {
	p.rooms = append(p.rooms, MansionRoom{
		Template: name, X: x, Y: y, Z: z,
		Rotation: Rotations[rot%4], Mirror: "NONE",
	})
}

func (p *mansionPlacer) relXZ(x, z, dir, n int) (int, int) {
	d := mrRotateDir(p.rot, dir)
	return x + mdStepX[d]*n, z + mdStepZ[d]*n
}

// mansionStructure places entrance, outer walls, roofs, corridor floors and
// carpets for the layout at start. Indoor walls are added in the room loop.
func (p *mansionPlacer) mansionStructure(m *MansionLayout, start mpos) {
	// Ground floor: entrance then outer walls (PlacementData advanced by entrance).
	pd := mansionPlace{x: start.x, y: start.y, z: start.z, rot: p.rot, wall: "wall_flat"}
	p.entrance(&pd)
	p.traverseOuterWalls(&pd, m.base, mdSouth, mansionStartX, mansionStartY, mansionEntranceX+1, mansionEntranceY)

	// Second floor walls: same XZ as post-entrance ground, Y+8, wall_window.
	ux, uz := mrRel(p.rot, start.x, start.z, mdSouth, 16)
	upper := mansionPlace{x: ux, y: start.y + 8, z: uz, rot: p.rot, wall: "wall_window"}
	p.traverseOuterWalls(&upper, m.base, mdSouth, mansionStartX, mansionStartY, mansionEntranceX+1, mansionEntranceY)

	// third-floor outer walls: find first house cell scanning y then x descending
	third := mansionPlace{x: start.x, y: start.y + 19, z: start.z, rot: p.rot, wall: "wall_window"}
	found := false
	for gy := 0; gy < m.third.h && !found; gy++ {
		for gx := m.third.w - 1; gx >= 0 && !found; gx-- {
			if mgIsHouse(m.third, gx, gy) {
				sx, sz := p.relXZ(start.x, start.z, mdSouth, 8+(gy-mansionStartY)*8)
				sx, sz = p.relXZ(sx, sz, mdEast, (gx-mansionStartX)*8)
				third.x, third.z = sx, sz
				third.y = start.y + 19
				p.traverseWallPiece(&third)
				p.traverseOuterWalls(&third, m.third, mdSouth, gx, gy, gx, gy)
				found = true
			}
		}
	}

	p.createRoof(start.x, start.y+16, start.z, m.base, m.third)
	p.createRoof(start.x, start.y+27, start.z, m.third, nil)

	for floor := 0; floor < 3; floor++ {
		lift := 8 * floor
		if floor == 2 {
			lift += 3
		}
		baseY := start.y + lift
		rg := m.rooms[floor]
		g := m.base
		if floor == 2 {
			g = m.third
		}
		carpetS := "carpet_south_1"
		carpetW := "carpet_west_1"
		if floor != 0 {
			carpetS, carpetW = "carpet_south_2", "carpet_west_2"
		}
		for gy := 0; gy < g.h; gy++ {
			for gx := 0; gx < g.w; gx++ {
				if g.get(gx, gy) != 1 {
					continue
				}
				cx, cz := p.relXZ(start.x, start.z, mdSouth, 8+(gy-mansionStartY)*8)
				cx, cz = p.relXZ(cx, cz, mdEast, (gx-mansionStartX)*8)
				p.addAt("corridor_floor", cx, baseY, cz, p.rot)
				if g.get(gx, gy-1) == 1 || (rg.get(gx, gy-1)&mgRoomCorrStr) == mgRoomCorrStr {
					nx, nz := p.relXZ(cx, cz, mdEast, 1)
					p.addAt("carpet_north", nx, baseY+1, nz, p.rot)
				}
				if g.get(gx+1, gy) == 1 || (rg.get(gx+1, gy)&mgRoomCorrStr) == mgRoomCorrStr {
					nx, nz := p.relXZ(cx, cz, mdSouth, 1)
					nx, nz = p.relXZ(nx, nz, mdEast, 5)
					p.addAt("carpet_east", nx, baseY+1, nz, p.rot)
				}
				if g.get(gx, gy+1) == 1 || (rg.get(gx, gy+1)&mgRoomCorrStr) == mgRoomCorrStr {
					nx, nz := p.relXZ(cx, cz, mdSouth, 5)
					nx, nz = p.relXZ(nx, nz, mdWest, 1)
					p.addAt(carpetS, nx, baseY, nz, p.rot)
				}
				if g.get(gx-1, gy) == 1 || (rg.get(gx-1, gy)&mgRoomCorrStr) == mgRoomCorrStr {
					nx, nz := p.relXZ(cx, cz, mdWest, 1)
					nx, nz = p.relXZ(nx, nz, mdNorth, 1)
					p.addAt(carpetW, nx, baseY, nz, p.rot)
				}
			}
		}
	}
}

func (p *mansionPlacer) entrance(pd *mansionPlace) {
	// relative uses p.rot; but pd.rot may differ after turns — use pd.rot
	ex, ez := mrRel(pd.rot, pd.x, pd.z, mdWest, 9)
	p.addAt("entrance", ex, pd.y, ez, pd.rot)
	pd.x, pd.z = mrRel(pd.rot, pd.x, pd.z, mdSouth, 16)
}

func mrRel(rot, x, z, dir, n int) (int, int) {
	d := mrRotateDir(rot, dir)
	return x + mdStepX[d]*n, z + mdStepZ[d]*n
}

func (p *mansionPlacer) traverseWallPiece(pd *mansionPlace) {
	wx, wz := mrRel(pd.rot, pd.x, pd.z, mdEast, 7)
	p.addAt(pd.wall, wx, pd.y, wz, pd.rot)
	pd.x, pd.z = mrRel(pd.rot, pd.x, pd.z, mdSouth, 8)
}

func (p *mansionPlacer) traverseTurn(pd *mansionPlace) {
	pd.x, pd.z = mrRel(pd.rot, pd.x, pd.z, mdSouth, -1)
	p.addAt("wall_corner", pd.x, pd.y, pd.z, pd.rot)
	pd.x, pd.z = mrRel(pd.rot, pd.x, pd.z, mdSouth, -7)
	pd.x, pd.z = mrRel(pd.rot, pd.x, pd.z, mdWest, -6)
	pd.rot = (pd.rot + 1) % 4 // CLOCKWISE_90
}

func (p *mansionPlacer) traverseInnerTurn(pd *mansionPlace) {
	pd.x, pd.z = mrRel(pd.rot, pd.x, pd.z, mdSouth, 6)
	pd.x, pd.z = mrRel(pd.rot, pd.x, pd.z, mdEast, 8)
	pd.rot = (pd.rot + 3) % 4 // COUNTERCLOCKWISE_90
}

func (p *mansionPlacer) traverseOuterWalls(pd *mansionPlace, g *mansionGrid, dir, x0, y0, endX, endY int) {
	x, y := x0, y0
	startDir := dir
	for {
		nx, ny := x+mdStepX[dir], y+mdStepZ[dir]
		if !mgIsHouse(g, nx, ny) {
			p.traverseTurn(pd)
			dir = mdCW(dir)
			if x != endX || y != endY || startDir != dir {
				p.traverseWallPiece(pd)
			}
		} else if mgIsHouse(g, nx, ny) && mgIsHouse(g, nx+mdStepX[mdCCW(dir)], ny+mdStepZ[mdCCW(dir)]) {
			p.traverseInnerTurn(pd)
			x, y = nx, ny
			dir = mdCCW(dir)
		} else {
			x, y = nx, ny
			if x != endX || y != endY || startDir != dir {
				p.traverseWallPiece(pd)
			}
		}
		if x == endX && y == endY && startDir == dir {
			break
		}
	}
}

func (p *mansionPlacer) createRoof(sx, sy, sz int, g, above *mansionGrid) {
	rot := p.rot
	for gy := 0; gy < g.h; gy++ {
		for gx := 0; gx < g.w; gx++ {
			cx, cz := mrRel(rot, sx, sz, mdSouth, 8+(gy-mansionStartY)*8)
			cx, cz = mrRel(rot, cx, cz, mdEast, (gx-mansionStartX)*8)
			covered := above != nil && mgIsHouse(above, gx, gy)
			if mgIsHouse(g, gx, gy) && !covered {
				p.addAt("roof", cx, sy+3, cz, rot)
				if !mgIsHouse(g, gx+1, gy) {
					tx, tz := mrRel(rot, cx, cz, mdEast, 6)
					p.addAt("roof_front", tx, sy, tz, rot)
				}
				if !mgIsHouse(g, gx-1, gy) {
					tx, tz := mrRel(rot, cx, cz, mdEast, 0)
					tx, tz = mrRel(rot, tx, tz, mdSouth, 7)
					p.addAt("roof_front", tx, sy, tz, (rot+2)%4)
				}
				if !mgIsHouse(g, gx, gy-1) {
					tx, tz := mrRel(rot, cx, cz, mdWest, 1)
					p.addAt("roof_front", tx, sy, tz, (rot+3)%4)
				}
				if !mgIsHouse(g, gx, gy+1) {
					tx, tz := mrRel(rot, cx, cz, mdEast, 6)
					tx, tz = mrRel(rot, tx, tz, mdSouth, 6)
					p.addAt("roof_front", tx, sy, tz, (rot+1)%4)
				}
			}
		}
	}
	if above != nil {
		for gy := 0; gy < g.h; gy++ {
			for gx := 0; gx < g.w; gx++ {
				cx, cz := mrRel(rot, sx, sz, mdSouth, 8+(gy-mansionStartY)*8)
				cx, cz = mrRel(rot, cx, cz, mdEast, (gx-mansionStartX)*8)
				if !(mgIsHouse(g, gx, gy) && mgIsHouse(above, gx, gy)) {
					continue
				}
				if !mgIsHouse(g, gx+1, gy) {
					tx, tz := mrRel(rot, cx, cz, mdEast, 7)
					p.addAt("small_wall", tx, sy, tz, rot)
				}
				if !mgIsHouse(g, gx-1, gy) {
					tx, tz := mrRel(rot, cx, cz, mdWest, 1)
					tx, tz = mrRel(rot, tx, tz, mdSouth, 6)
					p.addAt("small_wall", tx, sy, tz, (rot+2)%4)
				}
				if !mgIsHouse(g, gx, gy-1) {
					tx, tz := mrRel(rot, cx, cz, mdWest, 0)
					tx, tz = mrRel(rot, tx, tz, mdNorth, 1)
					p.addAt("small_wall", tx, sy, tz, (rot+3)%4)
				}
				if !mgIsHouse(g, gx, gy+1) {
					tx, tz := mrRel(rot, cx, cz, mdEast, 6)
					tx, tz = mrRel(rot, tx, tz, mdSouth, 7)
					p.addAt("small_wall", tx, sy, tz, (rot+1)%4)
				}
				if !mgIsHouse(g, gx+1, gy) {
					if !mgIsHouse(g, gx, gy-1) {
						tx, tz := mrRel(rot, cx, cz, mdEast, 7)
						tx, tz = mrRel(rot, tx, tz, mdNorth, 2)
						p.addAt("small_wall_corner", tx, sy, tz, rot)
					}
					if !mgIsHouse(g, gx, gy+1) {
						tx, tz := mrRel(rot, cx, cz, mdEast, 8)
						tx, tz = mrRel(rot, tx, tz, mdSouth, 7)
						p.addAt("small_wall_corner", tx, sy, tz, (rot+1)%4)
					}
				}
				if !mgIsHouse(g, gx-1, gy) {
					if !mgIsHouse(g, gx, gy-1) {
						tx, tz := mrRel(rot, cx, cz, mdWest, 2)
						tx, tz = mrRel(rot, tx, tz, mdNorth, 1)
						p.addAt("small_wall_corner", tx, sy, tz, (rot+3)%4)
					}
					if !mgIsHouse(g, gx, gy+1) {
						tx, tz := mrRel(rot, cx, cz, mdWest, 1)
						tx, tz = mrRel(rot, tx, tz, mdSouth, 8)
						p.addAt("small_wall_corner", tx, sy, tz, (rot+2)%4)
					}
				}
			}
		}
	}
	for gy := 0; gy < g.h; gy++ {
		for gx := 0; gx < g.w; gx++ {
			cx, cz := mrRel(rot, sx, sz, mdSouth, 8+(gy-mansionStartY)*8)
			cx, cz = mrRel(rot, cx, cz, mdEast, (gx-mansionStartX)*8)
			covered := above != nil && mgIsHouse(above, gx, gy)
			if !(mgIsHouse(g, gx, gy) && !covered) {
				continue
			}
			if !mgIsHouse(g, gx+1, gy) {
				tx, tz := mrRel(rot, cx, cz, mdEast, 6)
				if !mgIsHouse(g, gx, gy+1) {
					ux, uz := mrRel(rot, tx, tz, mdSouth, 6)
					p.addAt("roof_corner", ux, sy, uz, rot)
				} else if mgIsHouse(g, gx+1, gy+1) {
					ux, uz := mrRel(rot, tx, tz, mdSouth, 5)
					p.addAt("roof_inner_corner", ux, sy, uz, rot)
				}
				if !mgIsHouse(g, gx, gy-1) {
					p.addAt("roof_corner", tx, sy, tz, (rot+3)%4)
				} else if mgIsHouse(g, gx+1, gy-1) {
					ux, uz := mrRel(rot, cx, cz, mdEast, 9)
					ux, uz = mrRel(rot, ux, uz, mdNorth, 2)
					p.addAt("roof_inner_corner", ux, sy, uz, (rot+1)%4)
				}
			}
			if !mgIsHouse(g, gx-1, gy) {
				tx, tz := mrRel(rot, cx, cz, mdEast, 0)
				tx, tz = mrRel(rot, tx, tz, mdSouth, 0)
				if !mgIsHouse(g, gx, gy+1) {
					ux, uz := mrRel(rot, tx, tz, mdSouth, 6)
					p.addAt("roof_corner", ux, sy, uz, (rot+1)%4)
				} else if mgIsHouse(g, gx-1, gy+1) {
					ux, uz := mrRel(rot, tx, tz, mdSouth, 8)
					ux, uz = mrRel(rot, ux, uz, mdWest, 3)
					p.addAt("roof_inner_corner", ux, sy, uz, (rot+3)%4)
				}
				if !mgIsHouse(g, gx, gy-1) {
					p.addAt("roof_corner", tx, sy, tz, (rot+2)%4)
				} else if mgIsHouse(g, gx-1, gy-1) {
					ux, uz := mrRel(rot, tx, tz, mdSouth, 1)
					p.addAt("roof_inner_corner", ux, sy, uz, (rot+2)%4)
				}
			}
		}
	}
}

// addIndoorWalls places indoors_wall / indoors_door around one room cell.
func (p *mansionPlacer) addIndoorWalls(floor int, g *mansionGrid, m *MansionLayout, gx, gy, door int, pos mpos) {
	wall, doorT := "indoors_wall_1", "indoors_door_1"
	if floor != 0 {
		wall, doorT = "indoors_wall_2", "indoors_door_2"
	}
	v := m.rooms[floor].get(gx, gy)
	id := v & mgRoomID
	stairsHall := floor == 2 && g.get(gx, gy) == 3 && (v&mgRoomCorrStr) == mgRoomCorrStr

	name := func(want int) string {
		if door == want {
			return doorT
		}
		return wall
	}
	if mgIsHouse(g, gx-1, gy) && !m.sameRoom(gx-1, gy, floor, id) {
		p.addAt(name(mdWest), pos.x, pos.y, pos.z, p.rot)
	}
	if g.get(gx+1, gy) == 1 && !stairsHall {
		ex, ez := p.relXZ(pos.x, pos.z, mdEast, 8)
		p.addAt(name(mdEast), ex, pos.y, ez, p.rot)
	}
	if mgIsHouse(g, gx, gy+1) && !m.sameRoom(gx, gy+1, floor, id) {
		ex, ez := p.relXZ(pos.x, pos.z, mdSouth, 7)
		ex, ez = p.relXZ(ex, ez, mdEast, 7)
		p.addAt(name(mdSouth), ex, pos.y, ez, (p.rot+1)%4)
	}
	if g.get(gx, gy-1) == 1 && !stairsHall {
		ex, ez := p.relXZ(pos.x, pos.z, mdNorth, 1)
		ex, ez = p.relXZ(ex, ez, mdEast, 7)
		p.addAt(name(mdNorth), ex, pos.y, ez, (p.rot+1)%4)
	}
}

// MansionAllPieces is createMansion: structural templates then rooms (with
// indoor walls). Room templates match MansionRooms; structural pieces draw
// no extra random.
func MansionAllPieces(seed int64, cx, cz int32, y int) (rotation string, pieces []MansionRoom, layout *MansionLayout) {
	return MansionAllPiecesWith(seed, cx, cz, y, MansionOptions{})
}

// MansionAllPiecesWith is MansionAllPieces with explicit template list sizes.
func MansionAllPiecesWith(seed int64, cx, cz int32, y int, opt MansionOptions) (rotation string, pieces []MansionRoom, layout *MansionLayout) {
	b1x1 := int32(opt.SecondFloor1x1Variants)
	if b1x1 <= 0 {
		b1x1 = 4
	}
	r := LargeFeatureRandom(seed, cx, cz)
	rot := int(r.NextInt(4))
	start := mpos{int(cx)*16 + 7, y, int(cz)*16 + 7}
	m := newMansionLayout(r)
	p := &mansionPlacer{r: r, rot: rot}
	p.mansionStructure(m, start)
	const c, d = 8, 5
	for floor := 0; floor < 3; floor++ {
		lift := 8 * floor
		if floor == 2 {
			lift += 3
		}
		base := mpos{start.x, start.y + lift, start.z}
		rg := m.rooms[floor]
		g := m.base
		if floor == 2 {
			g = m.third
		}
		set := mansionRoomSet{second: floor > 0, b1x1Vars: b1x1}
		for gy := 0; gy < g.h; gy++ {
			for gx := 0; gx < g.w; gx++ {
				stairs := floor == 2 && g.get(gx, gy) == 3
				if g.get(gx, gy) != 2 && !stairs {
					continue
				}
				v := rg.get(gx, gy)
				size := v & mgRoomType
				id := v & mgRoomID
				var dirs []int
				if v&mgRoomDoor == mgRoomDoor {
					for dd := 0; dd < 4; dd++ {
						if g.get(gx+mdStepX[dd], gy+mdStepZ[dd]) == 1 {
							dirs = append(dirs, dd)
						}
					}
				}
				door := mdNone
				if len(dirs) > 0 {
					door = dirs[r.NextInt(int32(len(dirs)))]
				} else if v&mgRoomOrigin == mgRoomOrigin {
					door = mdUp
				}
				pos := p.rel(base, mdSouth, 8+(gy-d)*8)
				pos = p.rel(pos, mdEast, -1+(gx-c)*8)
				p.addIndoorWalls(floor, g, m, gx, gy, door, pos)
				n := len(p.rooms)
				switch {
				case size == mgRoom1x1:
					p.add1x1(floor, pos, door, set)
				case size == mgRoom1x2 && door != mdNone:
					conn := m.sameRoomDir(gx, gy, floor, id)
					p.add1x2(floor, pos, conn, door, set, v&mgRoomStairs == mgRoomStairs)
				case size == mgRoom2x2 && door != mdNone && door != mdUp:
					conn := mdCW(door)
					if !m.sameRoom(gx+mdStepX[conn], gy+mdStepZ[conn], floor, id) {
						conn = mdOpposite(conn)
					}
					p.add2x2(floor, pos, conn, door, set)
				case size == mgRoom2x2 && door == mdUp:
					p.add(floor, "2x2_s1", p.rel(pos, mdEast, 1), p.rot, 0)
				}
				for i := n; i < len(p.rooms); i++ {
					p.rooms[i].CellX, p.rooms[i].CellY = gx, gy
					switch size {
					case mgRoom1x1:
						p.rooms[i].Size = 1
					case mgRoom1x2:
						p.rooms[i].Size = 2
					default:
						p.rooms[i].Size = 4
					}
				}
				_ = id
			}
		}
	}
	return Rotations[rot], p.rooms, m
}
