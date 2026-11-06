package worldgen

// Woodland mansion layout: a port of WoodlandMansionPieces.MansionGrid and
// MansionPiecePlacer.createMansion (rooms plus entrance, walls, roofs,
// corridors, carpets and indoor walls). Structural templates draw no random;
// door picks share the room-loop RNG.

// Grid cell values used by MansionGrid.
const (
	mgRoom1x1     = 65536
	mgRoom1x2     = 131072
	mgRoom2x2     = 262144
	mgRoomOrigin  = 1048576
	mgRoomDoor    = 2097152
	mgRoomStairs  = 4194304
	mgRoomCorrStr = 8388608
	mgRoomType    = 983040
	mgRoomID      = 65535
)

// Directions in clockwise order; mdUp and mdNone are the vanilla UP and null.
const (
	mdNorth = iota
	mdEast
	mdSouth
	mdWest
	mdUp
	mdNone = -1
)

var (
	mdStepX = [4]int{0, 1, 0, -1}
	mdStepZ = [4]int{-1, 0, 1, 0}
	// Direction.from2DDataValue: 0 SOUTH, 1 WEST, 2 NORTH, 3 EAST.
	mdFrom2D = [4]int{mdSouth, mdWest, mdNorth, mdEast}
)

func mdCW(d int) int       { return (d + 1) % 4 }
func mdCCW(d int) int      { return (d + 3) % 4 }
func mdOpposite(d int) int { return (d + 2) % 4 }

type mansionGrid struct {
	w, h, outside int
	v             [][]int
}

func newMansionGrid(w, h, outside int) *mansionGrid {
	g := &mansionGrid{w: w, h: h, outside: outside, v: make([][]int, w)}
	for i := range g.v {
		g.v[i] = make([]int, h)
	}
	return g
}

func (g *mansionGrid) set(x, y, v int) {
	if x >= 0 && x < g.w && y >= 0 && y < g.h {
		g.v[x][y] = v
	}
}

func (g *mansionGrid) fill(x0, y0, x1, y1, v int) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			g.set(x, y, v)
		}
	}
}

func (g *mansionGrid) get(x, y int) int {
	if x >= 0 && x < g.w && y >= 0 && y < g.h {
		return g.v[x][y]
	}
	return g.outside
}

func (g *mansionGrid) setIf(x, y, ifV, v int) {
	if g.get(x, y) == ifV {
		g.set(x, y, v)
	}
}

func (g *mansionGrid) edgesTo(x, y, v int) bool {
	return g.get(x-1, y) == v || g.get(x+1, y) == v || g.get(x, y+1) == v || g.get(x, y-1) == v
}

func mgIsHouse(g *mansionGrid, x, y int) bool {
	v := g.get(x, y)
	return v == 1 || v == 2 || v == 3 || v == 4
}

// MansionLayout is the seeded room grid of one mansion.
type MansionLayout struct {
	base, third *mansionGrid
	rooms       [3]*mansionGrid
	r           *JavaRandom
}

func newMansionLayout(r *JavaRandom) *MansionLayout {
	m := &MansionLayout{r: r}
	b := newMansionGrid(11, 11, 5)
	m.base = b
	b.fill(7, 4, 8, 5, 3)
	b.fill(6, 4, 6, 5, 2)
	b.fill(9, 2, 10, 7, 5)
	b.fill(8, 2, 8, 3, 1)
	b.fill(8, 6, 8, 7, 1)
	b.set(6, 3, 1)
	b.set(6, 6, 1)
	b.fill(0, 0, 11, 1, 5)
	b.fill(0, 9, 11, 11, 5)
	m.corridor(b, 7, 2, mdWest, 6)
	m.corridor(b, 7, 7, mdWest, 6)
	m.corridor(b, 5, 3, mdWest, 3)
	m.corridor(b, 5, 6, mdWest, 3)
	for mgCleanEdges(b) {
	}
	for i := range m.rooms {
		m.rooms[i] = newMansionGrid(11, 11, 5)
	}
	m.identifyRooms(b, m.rooms[0])
	m.identifyRooms(b, m.rooms[1])
	m.rooms[0].fill(8, 4, 8, 5, mgRoomCorrStr)
	m.rooms[1].fill(8, 4, 8, 5, mgRoomCorrStr)
	m.third = newMansionGrid(11, 11, 5)
	m.setupThirdFloor()
	m.identifyRooms(m.third, m.rooms[2])
	return m
}

func (m *MansionLayout) corridor(g *mansionGrid, x, y, dir, length int) {
	if length <= 0 {
		return
	}
	g.set(x, y, 1)
	g.setIf(x+mdStepX[dir], y+mdStepZ[dir], 0, 1)
	for i := 0; i < 8; i++ {
		d := mdFrom2D[m.r.NextInt(4)]
		if d == mdOpposite(dir) {
			continue
		}
		if d == mdEast && m.r.NextBoolean() {
			continue
		}
		j, k := x+mdStepX[dir], y+mdStepZ[dir]
		if g.get(j+mdStepX[d], k+mdStepZ[d]) == 0 && g.get(j+2*mdStepX[d], k+2*mdStepZ[d]) == 0 {
			m.corridor(g, j+mdStepX[d], k+mdStepZ[d], d, length-1)
			break
		}
	}
	cw, ccw := mdCW(dir), mdCCW(dir)
	dx, dz := mdStepX[dir], mdStepZ[dir]
	g.setIf(x+mdStepX[cw], y+mdStepZ[cw], 0, 2)
	g.setIf(x+mdStepX[ccw], y+mdStepZ[ccw], 0, 2)
	g.setIf(x+dx+mdStepX[cw], y+dz+mdStepZ[cw], 0, 2)
	g.setIf(x+dx+mdStepX[ccw], y+dz+mdStepZ[ccw], 0, 2)
	g.setIf(x+2*dx, y+2*dz, 0, 2)
	g.setIf(x+2*mdStepX[cw], y+2*mdStepZ[cw], 0, 2)
	g.setIf(x+2*mdStepX[ccw], y+2*mdStepZ[ccw], 0, 2)
}

func mgCleanEdges(g *mansionGrid) bool {
	changed := false
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			if g.get(x, y) != 0 {
				continue
			}
			k := 0
			for _, p := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				if mgIsHouse(g, x+p[0], y+p[1]) {
					k++
				}
			}
			if k >= 3 {
				g.set(x, y, 2)
				changed = true
			} else if k == 2 {
				l := 0
				for _, p := range [4][2]int{{1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
					if mgIsHouse(g, x+p[0], y+p[1]) {
						l++
					}
				}
				if l <= 1 {
					g.set(x, y, 2)
					changed = true
				}
			}
		}
	}
	return changed
}

func (m *MansionLayout) setupThirdFloor() {
	g := m.rooms[1]
	var list [][2]int
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			v := g.get(x, y)
			if v&mgRoomType == mgRoom1x2 && v&mgRoomDoor == mgRoomDoor {
				list = append(list, [2]int{x, y})
			}
		}
	}
	s := m.third
	if len(list) == 0 {
		s.fill(0, 0, s.w, s.h, 5)
		return
	}
	t := list[m.r.NextInt(int32(len(list)))]
	v := g.get(t[0], t[1])
	g.set(t[0], t[1], v|mgRoomStairs)
	d1 := m.sameRoomDir(t[0], t[1], 1, v&mgRoomID)
	if d1 == mdNone {
		d1 = mdNorth // unreachable for a 1x2 room
	}
	i6, i7 := t[0]+mdStepX[d1], t[1]+mdStepZ[d1]
	for y := 0; y < s.h; y++ {
		for x := 0; x < s.w; x++ {
			switch {
			case !mgIsHouse(m.base, x, y):
				s.set(x, y, 5)
			case x == t[0] && y == t[1]:
				s.set(x, y, 3)
			case x == i6 && y == i7:
				s.set(x, y, 3)
				m.rooms[2].set(x, y, mgRoomCorrStr)
			}
		}
	}
	var dirs []int
	for d := 0; d < 4; d++ {
		if s.get(i6+mdStepX[d], i7+mdStepZ[d]) == 0 {
			dirs = append(dirs, d)
		}
	}
	if len(dirs) == 0 {
		s.fill(0, 0, s.w, s.h, 5)
		g.set(t[0], t[1], v)
		return
	}
	d := dirs[m.r.NextInt(int32(len(dirs)))]
	m.corridor(s, i6+mdStepX[d], i7+mdStepZ[d], d, 4)
	for mgCleanEdges(s) {
	}
}

func (m *MansionLayout) sameRoom(x, y, floor, id int) bool {
	return m.rooms[floor].get(x, y)&mgRoomID == id
}

func (m *MansionLayout) sameRoomDir(x, y, floor, id int) int {
	for d := 0; d < 4; d++ {
		if m.sameRoom(x+mdStepX[d], y+mdStepZ[d], floor, id) {
			return d
		}
	}
	return mdNone
}

func (m *MansionLayout) identifyRooms(g, rooms *mansionGrid) {
	var list [][2]int
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			if g.get(x, y) == 2 {
				list = append(list, [2]int{x, y})
			}
		}
	}
	for i := len(list); i > 1; i-- {
		j := m.r.NextInt(int32(i))
		list[i-1], list[j] = list[j], list[i-1]
	}
	k := 10
	free := func(x, y int) bool { return rooms.get(x, y) == 0 && g.get(x, y) == 2 }
	for _, c := range list {
		x, y := c[0], c[1]
		if rooms.get(x, y) != 0 {
			continue
		}
		j1, k1, l1, i2 := x, x, y, y
		size := mgRoom1x1
		switch {
		case free(x+1, y) && free(x, y+1) && free(x+1, y+1):
			k1++
			i2++
			size = mgRoom2x2
		case free(x-1, y) && free(x, y+1) && free(x-1, y+1):
			j1--
			i2++
			size = mgRoom2x2
		case free(x-1, y) && free(x, y-1) && free(x-1, y-1):
			j1--
			l1--
			size = mgRoom2x2
		case free(x+1, y):
			k1++
			size = mgRoom1x2
		case free(x, y+1):
			i2++
			size = mgRoom1x2
		case free(x-1, y):
			j1--
			size = mgRoom1x2
		case free(x, y-1):
			l1--
			size = mgRoom1x2
		}
		k2 := k1
		if m.r.NextBoolean() {
			k2 = j1
		}
		l2 := i2
		if m.r.NextBoolean() {
			l2 = l1
		}
		door := mgRoomDoor
		tx := func() {
			if k2 == j1 {
				k2 = k1
			} else {
				k2 = j1
			}
		}
		tz := func() {
			if l2 == l1 {
				l2 = i2
			} else {
				l2 = l1
			}
		}
		if !g.edgesTo(k2, l2, 1) {
			tx()
			tz()
			if !g.edgesTo(k2, l2, 1) {
				tz()
				if !g.edgesTo(k2, l2, 1) {
					tx()
					tz()
					if !g.edgesTo(k2, l2, 1) {
						door = 0
						k2, l2 = j1, l1
					}
				}
			}
		}
		for zz := l1; zz <= i2; zz++ {
			for xx := j1; xx <= k1; xx++ {
				if xx == k2 && zz == l2 {
					rooms.set(xx, zz, mgRoomOrigin|door|size|k)
				} else {
					rooms.set(xx, zz, size|k)
				}
			}
		}
		k++
	}
}

// MansionRoom is one placed room template.
type MansionRoom struct {
	Floor    int
	Template string
	X, Y, Z  int
	Rotation string
	Mirror   string
	CellX    int // grid cell of the room's door/origin cell
	CellY    int
	Size     int // 1 (1x1), 2 (1x2) or 4 (2x2)
}

var mansionMirrors = [3]string{"NONE", "LEFT_RIGHT", "FRONT_BACK"}

type mansionRoomSet struct {
	second   bool
	b1x1Vars int32
}

// MansionOptions tunes data-driven template list sizes. Vanilla 1.21.1
// draws the second/third-floor 1x1 room from 4 variants (1x1_b5 ships but
// never generates); template-list datapacks that add it make the list 5 long.
type MansionOptions struct {
	SecondFloor1x1Variants int
}

func (c mansionRoomSet) r1x1(r *JavaRandom) string {
	if c.second {
		return "1x1_b" + itoa(r.NextInt(c.b1x1Vars)+1)
	}
	return "1x1_a" + itoa(r.NextInt(5)+1)
}
func (c mansionRoomSet) r1x1Secret(r *JavaRandom) string {
	return "1x1_as" + itoa(r.NextInt(4)+1)
}
func (c mansionRoomSet) r1x2Side(r *JavaRandom, stairs bool) string {
	if c.second {
		if stairs {
			return "1x2_c_stairs"
		}
		return "1x2_c" + itoa(r.NextInt(4)+1)
	}
	return "1x2_a" + itoa(r.NextInt(9)+1)
}
func (c mansionRoomSet) r1x2Front(r *JavaRandom, stairs bool) string {
	if c.second {
		if stairs {
			return "1x2_d_stairs"
		}
		return "1x2_d" + itoa(r.NextInt(5)+1)
	}
	return "1x2_b" + itoa(r.NextInt(5)+1)
}
func (c mansionRoomSet) r1x2Secret(r *JavaRandom) string {
	if c.second {
		return "1x2_se" + itoa(r.NextInt(1)+1)
	}
	return "1x2_s" + itoa(r.NextInt(2)+1)
}
func (c mansionRoomSet) r2x2(r *JavaRandom) string {
	if c.second {
		return "2x2_b" + itoa(r.NextInt(5)+1)
	}
	return "2x2_a" + itoa(r.NextInt(4)+1)
}

func itoa(v int32) string {
	if v < 10 {
		return string(rune('0' + v))
	}
	return string(rune('0'+v/10)) + string(rune('0'+v%10))
}

// rotation indices: 0 NONE, 1 CLOCKWISE_90, 2 CLOCKWISE_180, 3 COUNTERCLOCKWISE_90.
func mrRotateDir(rot, d int) int { return (d + rot) % 4 }

func mrRotatePos(rot, x, z int) (int, int) {
	switch rot {
	case 1:
		return -z, x
	case 2:
		return -x, -z
	case 3:
		return z, -x
	}
	return x, z
}

// mrZeroPos is StructureTemplate.getZeroPositionWithTransform(pos,
// Mirror.NONE, rot, w, h): the corner a w×h template keeps after rotation.
func mrZeroPos(rot, x, z, w, h int) (int, int) {
	w--
	h--
	switch rot {
	case 1:
		return x + h, z
	case 2:
		return x + w, z + h
	case 3:
		return x, z + w
	}
	return x, z
}

type mansionPlacer struct {
	r     *JavaRandom
	rot   int
	rooms []MansionRoom
}

type mpos struct{ x, y, z int }

func (p *mansionPlacer) rel(a mpos, dir, n int) mpos {
	d := mrRotateDir(p.rot, dir)
	return mpos{a.x + mdStepX[d]*n, a.y, a.z + mdStepZ[d]*n}
}

func (p *mansionPlacer) add(floor int, name string, a mpos, rot, mirror int) {
	p.rooms = append(p.rooms, MansionRoom{Floor: floor, Template: name, X: a.x, Y: a.y, Z: a.z, Rotation: Rotations[rot%4], Mirror: mansionMirrors[mirror]})
}

// MansionRooms runs the mansion grid and room placement for the start chunk
// (cx, cz) with the start position (x, y, z) from findGenerationPoint. y is
// the lowest surface height of the four corners, which needs terrain; the
// room layout and X/Z are independent of it.
func MansionRooms(seed int64, cx, cz int32, y int) (rotation string, rooms []MansionRoom, layout *MansionLayout) {
	return MansionRoomsWith(seed, cx, cz, y, MansionOptions{})
}

// MansionRoomsWith is MansionRooms with explicit template list sizes.
func MansionRoomsWith(seed int64, cx, cz int32, y int, opt MansionOptions) (rotation string, rooms []MansionRoom, layout *MansionLayout) {
	b1x1 := int32(opt.SecondFloor1x1Variants)
	if b1x1 <= 0 {
		b1x1 = 4
	}
	r := LargeFeatureRandom(seed, cx, cz)
	rot := int(r.NextInt(4))
	start := mpos{int(cx)*16 + 7, y, int(cz)*16 + 7}
	m := newMansionLayout(r)
	p := &mansionPlacer{r: r, rot: rot}
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
			}
		}
	}
	return Rotations[rot], p.rooms, m
}

func (p *mansionPlacer) add1x1(floor int, pos mpos, door int, set mansionRoomSet) {
	r6 := 0
	name := set.r1x1(p.r)
	switch door {
	case mdEast:
	case mdNorth:
		r6 = 3
	case mdWest:
		r6 = 2
	case mdSouth:
		r6 = 1
	default:
		name = set.r1x1Secret(p.r)
	}
	ox, oz := mrZeroPos(r6, 1, 0, 7, 7)
	ox, oz = mrRotatePos(p.rot, ox, oz)
	p.add(floor, name, mpos{pos.x + ox, pos.y, pos.z + oz}, r6+p.rot, 0)
}

func (p *mansionPlacer) add1x2(floor int, pos mpos, conn, door int, set mansionRoomSet, stairs bool) {
	R := p.rot
	type c struct{ door, conn int }
	switch (c{door, conn}) {
	case c{mdEast, mdSouth}:
		p.add(floor, set.r1x2Side(p.r, stairs), p.rel(pos, mdEast, 1), R, 0)
	case c{mdEast, mdNorth}:
		p.add(floor, set.r1x2Side(p.r, stairs), p.rel(p.rel(pos, mdEast, 1), mdSouth, 6), R, 1)
	case c{mdWest, mdNorth}:
		p.add(floor, set.r1x2Side(p.r, stairs), p.rel(p.rel(pos, mdEast, 7), mdSouth, 6), R+2, 0)
	case c{mdWest, mdSouth}:
		p.add(floor, set.r1x2Side(p.r, stairs), p.rel(pos, mdEast, 7), R, 2)
	case c{mdSouth, mdEast}:
		p.add(floor, set.r1x2Side(p.r, stairs), p.rel(pos, mdEast, 1), R+1, 1)
	case c{mdSouth, mdWest}:
		p.add(floor, set.r1x2Side(p.r, stairs), p.rel(pos, mdEast, 7), R+1, 0)
	case c{mdNorth, mdWest}:
		p.add(floor, set.r1x2Side(p.r, stairs), p.rel(p.rel(pos, mdEast, 7), mdSouth, 6), R+1, 2)
	case c{mdNorth, mdEast}:
		p.add(floor, set.r1x2Side(p.r, stairs), p.rel(p.rel(pos, mdEast, 1), mdSouth, 6), R+3, 0)
	case c{mdSouth, mdNorth}:
		p.add(floor, set.r1x2Front(p.r, stairs), p.rel(p.rel(pos, mdEast, 1), mdNorth, 8), R, 0)
	case c{mdNorth, mdSouth}:
		p.add(floor, set.r1x2Front(p.r, stairs), p.rel(p.rel(pos, mdEast, 7), mdSouth, 14), R+2, 0)
	case c{mdWest, mdEast}:
		p.add(floor, set.r1x2Front(p.r, stairs), p.rel(pos, mdEast, 15), R+1, 0)
	case c{mdEast, mdWest}:
		p.add(floor, set.r1x2Front(p.r, stairs), p.rel(p.rel(pos, mdWest, 7), mdSouth, 6), R+3, 0)
	case c{mdUp, mdEast}:
		p.add(floor, set.r1x2Secret(p.r), p.rel(pos, mdEast, 15), R+1, 0)
	case c{mdUp, mdSouth}:
		p.add(floor, set.r1x2Secret(p.r), p.rel(p.rel(pos, mdEast, 1), mdNorth, 0), R, 0)
	}
}

func (p *mansionPlacer) add2x2(floor int, pos mpos, conn, door int, set mansionRoomSet) {
	i7, i8, rot, mir := 0, 0, p.rot, 0
	type c struct{ door, conn int }
	switch (c{door, conn}) {
	case c{mdEast, mdSouth}:
		i7 = -7
	case c{mdEast, mdNorth}:
		i7, i8, mir = -7, 6, 1
	case c{mdNorth, mdEast}:
		i7, i8, rot = 1, 14, p.rot+3
	case c{mdNorth, mdWest}:
		i7, i8, rot, mir = 7, 14, p.rot+3, 1
	case c{mdSouth, mdWest}:
		i7, i8, rot = 7, -8, p.rot+1
	case c{mdSouth, mdEast}:
		i7, i8, rot, mir = 1, -8, p.rot+1, 1
	case c{mdWest, mdNorth}:
		i7, i8, rot = 15, 6, p.rot+2
	case c{mdWest, mdSouth}:
		i7, mir = 15, 2
	}
	at := p.rel(p.rel(pos, mdEast, i7), mdSouth, i8)
	p.add(floor, set.r2x2(p.r), at, rot, mir)
}

// MansionCell is one 8×8-block grid cell of a mansion floor.
type MansionCell struct {
	Kind int // 0 outside, 1 corridor, 2 room, 3 stairs or entrance hall
	Room int // room id; cells of one room share it
	Size int // 1, 2 or 4 grid cells for rooms
}

// Cell reports grid cell (x, y) of floor 0..2.
func (m *MansionLayout) Cell(floor, x, y int) MansionCell {
	g := m.base
	if floor == 2 {
		g = m.third
	}
	v := g.get(x, y)
	rv := m.rooms[floor].get(x, y)
	c := MansionCell{Room: rv & mgRoomID}
	switch v {
	case 1:
		c.Kind = 1
	case 2, 4:
		c.Kind = 2
	case 3:
		c.Kind = 3
	}
	if c.Kind == 2 {
		switch rv & mgRoomType {
		case mgRoom1x1:
			c.Size = 1
		case mgRoom1x2:
			c.Size = 2
		case mgRoom2x2:
			c.Size = 4
		}
		if floor == 1 && rv&mgRoomStairs == mgRoomStairs {
			c.Kind = 3
		}
	}
	return c
}

// MansionCellFrame returns the world X/Z corner of grid cell (gx, gy) for a
// mansion whose start column is (sx, sz), plus the unit steps of the cell's
// local east and south axes (createMansion's rotated EAST and SOUTH).
func MansionCellFrame(rotation string, sx, sz, gx, gy int) (x, z, ex, ez, wx, wz int) {
	rot := 0
	for i, r := range Rotations {
		if r == rotation {
			rot = i
		}
	}
	e, so := mrRotateDir(rot, mdEast), mrRotateDir(rot, mdSouth)
	ex, ez = mdStepX[e], mdStepZ[e]
	wx, wz = mdStepX[so], mdStepZ[so]
	ds, de := 8+(gy-5)*8, -1+(gx-8)*8
	return sx + wx*ds + ex*de, sz + wz*ds + ez*de, ex, ez, wx, wz
}
