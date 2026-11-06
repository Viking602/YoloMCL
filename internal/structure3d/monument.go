package structure3d

import (
	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// monumentPiece is one OceanMonumentPiece ready for postProcess.
type monumentPiece struct {
	id         string // post* suffix key
	bb         worldgen.Box
	dir        int
	roomDef    *monumentRoomDef
	mainDesign int
}

const (
	omDirDown  = 0
	omDirUp    = 1
	omDirNorth = 2
	omDirSouth = 3
	omDirWest  = 4
	omDirEast  = 5
)

func monumentRoomBox(dir int, room *monumentRoomDef, cellsW, cellsH, cellsD int) worldgen.Box {
	idx := room.index
	gx, gz, gy := idx%5, idx/5%5, idx/25
	w, h, d := cellsW*8, cellsH*4, cellsD*8
	// StructurePiece.makeBoundingBox(0,0,0,dir,w,h,d): on Z axis w along X; else w along Z.
	var box worldgen.Box
	if dir == worldgen.DirNorth || dir == worldgen.DirSouth {
		box = worldgen.Box{0, 0, 0, w - 1, h - 1, d - 1}
	} else {
		box = worldgen.Box{0, 0, 0, d - 1, h - 1, w - 1}
	}
	var mx, my, mz int
	switch dir {
	case worldgen.DirNorth:
		mx, my, mz = gx*8, gy*4, -(gz+cellsD)*8+1
	case worldgen.DirSouth:
		mx, my, mz = gx*8, gy*4, gz*8
	case worldgen.DirWest:
		mx, my, mz = -(gz+cellsD)*8+1, gy*4, gx*8
	default: // East
		mx, my, mz = gz*8, gy*4, gx*8
	}
	return worldgen.Box{box[0] + mx, box[1] + my, box[2] + mz, box[3] + mx, box[4] + my, box[5] + mz}
}

func moveBox(b worldgen.Box, dx, dy, dz int) worldgen.Box {
	return worldgen.Box{b[0] + dx, b[1] + dy, b[2] + dz, b[3] + dx, b[4] + dy, b[5] + dz}
}

// worldPosLocal mirrors StructurePiece.getWorldPos for a building bb+dir.
func worldPosLocal(bb worldgen.Box, dir, x, y, z int) (int, int, int) {
	g := &pc{bb: bb, dir: dir}
	p := g.pos(x, y, z)
	return p[0], p[1], p[2]
}

func shuffleRooms(list []*monumentRoomDef, r *worldgen.JavaRandom) {
	for i := len(list); i > 1; i-- {
		j := int(r.NextInt(int32(i)))
		list[i-1], list[j] = list[j], list[i-1]
	}
}

func generateMonumentRoomGraph(r *worldgen.JavaRandom) (source, core *monumentRoomDef, rooms []*monumentRoomDef) {
	grid := make([]*monumentRoomDef, 75)
	for x := 0; x < 5; x++ {
		for z := 0; z < 4; z++ {
			i := monumentRoomIndex(x, 0, z)
			grid[i] = &monumentRoomDef{index: i}
		}
	}
	for x := 0; x < 5; x++ {
		for z := 0; z < 4; z++ {
			i := monumentRoomIndex(x, 1, z)
			grid[i] = &monumentRoomDef{index: i}
		}
	}
	for x := 1; x < 4; x++ {
		for z := 0; z < 2; z++ {
			i := monumentRoomIndex(x, 2, z)
			grid[i] = &monumentRoomDef{index: i}
		}
	}
	source = grid[monumentRoomIndex(2, 0, 0)]

	// Connect adjacent cells. Java uses Direction.values() and for Z!=same uses opposite.
	dirs := [][3]int{
		{0, -1, 0}, {0, 1, 0}, {0, 0, -1}, {0, 0, 1}, {-1, 0, 0}, {1, 0, 0}, // D U N S W E steps
	}
	dirIdx := []int{omDirDown, omDirUp, omDirNorth, omDirSouth, omDirWest, omDirEast}
	for x := 0; x < 5; x++ {
		for z := 0; z < 5; z++ {
			for y := 0; y < 3; y++ {
				i := monumentRoomIndex(x, y, z)
				if i >= 75 || grid[i] == nil {
					continue
				}
				for di, step := range dirs {
					nx, ny, nz := x+step[0], y+step[1], z+step[2]
					if nx < 0 || nx >= 5 || nz < 0 || nz >= 5 || ny < 0 || ny >= 3 {
						continue
					}
					j := monumentRoomIndex(nx, ny, nz)
					if j >= 75 || grid[j] == nil {
						continue
					}
					d := dirIdx[di]
					if nz == z {
						grid[i].connect(d, grid[j])
					} else {
						grid[i].connect(dirOpposite(d), grid[j])
					}
				}
			}
		}
	}

	top := &monumentRoomDef{index: 1003}
	left := &monumentRoomDef{index: 1001}
	right := &monumentRoomDef{index: 1002}
	grid[monumentRoomIndex(2, 2, 0)].connect(omDirUp, top)
	grid[monumentRoomIndex(0, 1, 0)].connect(omDirSouth, left)
	grid[monumentRoomIndex(4, 1, 0)].connect(omDirSouth, right)
	top.claimed, left.claimed, right.claimed = true, true, true
	source.isSource = true

	core = grid[monumentRoomIndex(int(r.NextInt(4)), 0, 2)]
	core.claimed = true
	core.connections[omDirEast].claimed = true
	core.connections[omDirNorth].claimed = true
	core.connections[omDirEast].connections[omDirNorth].claimed = true
	core.connections[omDirUp].claimed = true
	core.connections[omDirEast].connections[omDirUp].claimed = true
	core.connections[omDirNorth].connections[omDirUp].claimed = true
	core.connections[omDirEast].connections[omDirNorth].connections[omDirUp].claimed = true

	list := make([]*monumentRoomDef, 0, 80)
	for _, rd := range grid {
		if rd != nil {
			rd.updateOpenings()
			list = append(list, rd)
		}
	}
	top.updateOpenings()
	shuffleRooms(list, r)

	scan := 1
	for _, rd := range list {
		opened := 0
		tries := 0
		for opened < 2 && tries < 5 {
			tries++
			d := int(r.NextInt(6))
			if !rd.hasOpening[d] {
				continue
			}
			opp := dirOpposite(d)
			rd.hasOpening[d] = false
			rd.connections[d].hasOpening[opp] = false
			if rd.findSource(scan) && rd.connections[d].findSource(scan+1) {
				scan += 2
				opened++
			} else {
				rd.hasOpening[d] = true
				rd.connections[d].hasOpening[opp] = true
			}
		}
	}
	list = append(list, top, left, right)
	return source, core, list
}

type monumentFitter func(r *monumentRoomDef) bool
type monumentFactory func(dir int, r *monumentRoomDef, rnd *worldgen.JavaRandom) monumentPiece

func fitDoubleYZ(r *monumentRoomDef) bool {
	if !(r.hasOpening[omDirNorth] && !r.connections[omDirNorth].claimed &&
		r.hasOpening[omDirUp] && !r.connections[omDirUp].claimed) {
		return false
	}
	n := r.connections[omDirNorth]
	return n.hasOpening[omDirUp] && !n.connections[omDirUp].claimed
}
func makeDoubleYZ(dir int, r *monumentRoomDef, _ *worldgen.JavaRandom) monumentPiece {
	r.claimed = true
	r.connections[omDirNorth].claimed = true
	r.connections[omDirUp].claimed = true
	r.connections[omDirNorth].connections[omDirUp].claimed = true
	return monumentPiece{id: "OceanMonumentDoubleYZRoom", bb: monumentRoomBox(dir, r, 1, 2, 2), dir: dir, roomDef: r}
}

func fitDoubleXY(r *monumentRoomDef) bool {
	if !(r.hasOpening[omDirEast] && !r.connections[omDirEast].claimed &&
		r.hasOpening[omDirUp] && !r.connections[omDirUp].claimed) {
		return false
	}
	e := r.connections[omDirEast]
	return e.hasOpening[omDirUp] && !e.connections[omDirUp].claimed
}
func makeDoubleXY(dir int, r *monumentRoomDef, _ *worldgen.JavaRandom) monumentPiece {
	r.claimed = true
	r.connections[omDirEast].claimed = true
	r.connections[omDirUp].claimed = true
	r.connections[omDirEast].connections[omDirUp].claimed = true
	return monumentPiece{id: "OceanMonumentDoubleXYRoom", bb: monumentRoomBox(dir, r, 2, 2, 1), dir: dir, roomDef: r}
}

func fitDoubleZ(r *monumentRoomDef) bool {
	return r.hasOpening[omDirNorth] && !r.connections[omDirNorth].claimed
}
func makeDoubleZ(dir int, r *monumentRoomDef, _ *worldgen.JavaRandom) monumentPiece {
	base := r
	if !r.hasOpening[omDirNorth] || r.connections[omDirNorth].claimed {
		base = r.connections[omDirSouth]
	}
	base.claimed = true
	base.connections[omDirNorth].claimed = true
	return monumentPiece{id: "OceanMonumentDoubleZRoom", bb: monumentRoomBox(dir, base, 1, 1, 2), dir: dir, roomDef: base}
}

func fitDoubleX(r *monumentRoomDef) bool {
	return r.hasOpening[omDirEast] && !r.connections[omDirEast].claimed
}
func makeDoubleX(dir int, r *monumentRoomDef, _ *worldgen.JavaRandom) monumentPiece {
	r.claimed = true
	r.connections[omDirEast].claimed = true
	return monumentPiece{id: "OceanMonumentDoubleXRoom", bb: monumentRoomBox(dir, r, 2, 1, 1), dir: dir, roomDef: r}
}

func fitDoubleY(r *monumentRoomDef) bool {
	return r.hasOpening[omDirUp] && !r.connections[omDirUp].claimed
}
func makeDoubleY(dir int, r *monumentRoomDef, _ *worldgen.JavaRandom) monumentPiece {
	r.claimed = true
	r.connections[omDirUp].claimed = true
	return monumentPiece{id: "OceanMonumentDoubleYRoom", bb: monumentRoomBox(dir, r, 1, 2, 1), dir: dir, roomDef: r}
}

func fitSimpleTop(r *monumentRoomDef) bool {
	return !r.hasOpening[omDirWest] && !r.hasOpening[omDirEast] &&
		!r.hasOpening[omDirNorth] && !r.hasOpening[omDirSouth] && !r.hasOpening[omDirUp]
}
func makeSimpleTop(dir int, r *monumentRoomDef, _ *worldgen.JavaRandom) monumentPiece {
	r.claimed = true
	return monumentPiece{id: "OceanMonumentSimpleTopRoom", bb: monumentRoomBox(dir, r, 1, 1, 1), dir: dir, roomDef: r}
}

func fitSimple(r *monumentRoomDef) bool { return true }
func makeSimple(dir int, r *monumentRoomDef, rnd *worldgen.JavaRandom) monumentPiece {
	r.claimed = true
	return monumentPiece{
		id: "OceanMonumentSimpleRoom", bb: monumentRoomBox(dir, r, 1, 1, 1), dir: dir, roomDef: r,
		mainDesign: int(rnd.NextInt(3)),
	}
}

// createMonumentPieces mirrors MonumentBuilding's constructor child list.
func createMonumentPieces(seed int64, cx, cz int32, seaLevel, yOffset int) (building worldgen.Box, dir int, pieces []monumentPiece) {
	r := worldgen.LargeFeatureRandom(seed, cx, cz)
	dir = int(r.NextInt(4))
	x, z := int(cx)*16-29, int(cz)*16-29
	y := seaLevel - 24 + yOffset
	building = worldgen.Box{x, y, z, x + 57, y + 22, z + 57}

	source, core, graph := generateMonumentRoomGraph(r)
	source.claimed = true

	pieces = append(pieces,
		monumentPiece{id: "OceanMonumentEntryRoom", bb: monumentRoomBox(dir, source, 1, 1, 1), dir: dir, roomDef: source},
		monumentPiece{id: "OceanMonumentCoreRoom", bb: monumentRoomBox(dir, core, 2, 2, 2), dir: dir, roomDef: core},
	)

	fitters := []struct {
		fit monumentFitter
		mk  monumentFactory
	}{
		{fitDoubleXY, makeDoubleXY},
		{fitDoubleYZ, makeDoubleYZ},
		{fitDoubleZ, makeDoubleZ},
		{fitDoubleX, makeDoubleX},
		{fitDoubleY, makeDoubleY},
		{fitSimpleTop, makeSimpleTop},
		{fitSimple, makeSimple},
	}
	for _, rd := range graph {
		if rd.claimed || rd.isSpecial() {
			continue
		}
		for _, f := range fitters {
			if f.fit(rd) {
				pieces = append(pieces, f.mk(dir, rd, r))
				break
			}
		}
	}

	// Move room pieces by getWorldPos(9,0,22).
	dx, dy, dz := worldPosLocal(building, dir, 9, 0, 22)
	for i := range pieces {
		pieces[i].bb = moveBox(pieces[i].bb, dx, dy, dz)
	}

	// Wing + penthouse boxes from getWorldPos corners (already world-abs after fromCorners).
	// BoundingBox.fromCorners(getWorldPos(a), getWorldPos(b))
	wingSeed := r.NextIntFull() // Random.nextInt() unbounded
	mkWing := func(x0, y0, z0, x1, y1, z1 int, design int) monumentPiece {
		ax, ay, az := worldPosLocal(building, dir, x0, y0, z0)
		bx, by, bz := worldPosLocal(building, dir, x1, y1, z1)
		bb := worldgen.Box{min(ax, bx), min(ay, by), min(az, bz), max(ax, bx), max(ay, by), max(az, bz)}
		return monumentPiece{id: "OceanMonumentWingRoom", bb: bb, dir: dir, mainDesign: design & 1}
	}
	pieces = append(pieces,
		mkWing(1, 1, 1, 23, 8, 21, int(wingSeed)),
		mkWing(34, 1, 1, 56, 8, 21, int(wingSeed)+1),
	)
	ax, ay, az := worldPosLocal(building, dir, 22, 13, 22)
	bx, by, bz := worldPosLocal(building, dir, 35, 17, 35)
	pent := worldgen.Box{min(ax, bx), min(ay, by), min(az, bz), max(ax, bx), max(ay, by), max(az, bz)}
	pieces = append(pieces, monumentPiece{id: "OceanMonumentPenthouse", bb: pent, dir: dir})

	return building, dir, pieces
}

func monumentPost(id string, g *pc) {
	switch id {
	case "MonumentBuilding":
		g.postMonumentBuilding()
	case "OceanMonumentEntryRoom":
		g.postOceanMonumentEntryRoom()
	case "OceanMonumentCoreRoom":
		g.postOceanMonumentCoreRoom()
	case "OceanMonumentDoubleYZRoom":
		g.postOceanMonumentDoubleYZRoom()
	case "OceanMonumentDoubleXYRoom":
		g.postOceanMonumentDoubleXYRoom()
	case "OceanMonumentDoubleZRoom":
		g.postOceanMonumentDoubleZRoom()
	case "OceanMonumentDoubleXRoom":
		g.postOceanMonumentDoubleXRoom()
	case "OceanMonumentDoubleYRoom":
		g.postOceanMonumentDoubleYRoom()
	case "OceanMonumentSimpleTopRoom":
		g.postOceanMonumentSimpleTopRoom()
	case "OceanMonumentSimpleRoom":
		g.postOceanMonumentSimpleRoom()
	case "OceanMonumentWingRoom":
		g.postOceanMonumentWingRoom()
	case "OceanMonumentPenthouse":
		g.postOceanMonumentPenthouse()
	}
}

// MonumentBlocks places the seed-exact ocean monument into a pieceWorld.
func MonumentBlocks(seed int64, cx, cz int32, seaLevel int, ground Ground, minY, maxY int) map[[3]int]BlockState {
	m, _ := MonumentBlocksOffset(seed, cx, cz, seaLevel, 0, ground, minY, maxY)
	return m
}

// MonumentBlocksOffset is MonumentBlocks with the building moved by yOffset
// (Tectonic monument_offset), plus the building box.
func MonumentBlocksOffset(seed int64, cx, cz int32, seaLevel, yOffset int, ground Ground, minY, maxY int) (map[[3]int]BlockState, worldgen.Box) {
	building, dir, children := createMonumentPieces(seed, cx, cz, seaLevel, yOffset)
	w := newPieceWorld(ground, minY, seaLevel-24+yOffset)
	w.maxY = maxY
	// Building first (needs seaLevel on pc), then children.
	all := make([]monumentPiece, 0, 1+len(children))
	all = append(all, monumentPiece{id: "MonumentBuilding", bb: building, dir: dir})
	all = append(all, children...)

	bbs := make([]worldgen.Box, len(all))
	states := make([]*pc, len(all))
	for i, p := range all {
		bbs[i] = p.bb
		states[i] = &pc{
			w: w, bb: p.bb, dir: p.dir,
			roomDef: p.roomDef, mainDesign: p.mainDesign, seaLevel: seaLevel,
		}
	}
	runPieces(w, bbs, states, decoRand(seed, 0 /*unused feature index for monument?*/, worldgen.StepSurfaceStructures),
		func(i int, g *pc) {
			g.seaLevel = seaLevel
			g.roomDef = all[i].roomDef
			g.mainDesign = all[i].mainDesign
			monumentPost(all[i].id, g)
		})
	return w.m, building
}
