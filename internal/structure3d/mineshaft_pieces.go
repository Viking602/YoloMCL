package structure3d

import "github.com/Viking602/YoloMCL/internal/worldgen"

// msPiece is one MineshaftPieces piece with the state its postProcess reads.
type msPiece struct {
	kind         string
	mesa         bool
	rails        bool
	spider       bool
	placedSpider bool
	sections     int
	twoFloor     bool
	entrances    []worldgen.Box
}

func (m *msPiece) planks() BlockState {
	if m.mesa {
		return st("dark_oak_planks")
	}
	return st("oak_planks")
}
func (m *msPiece) wood() BlockState {
	if m.mesa {
		return st("dark_oak_log")
	}
	return st("oak_log")
}
func (m *msPiece) fence() BlockState {
	if m.mesa {
		return st("dark_oak_fence")
	}
	return st("oak_fence")
}

var caveAir = st("cave_air")

// getBlockName is getBlock: the state name at a local position, "air"
// outside the chunk box, and a ground class name for unplaced terrain.
func (g *pc) blockName(x, y, z int) string {
	p := g.pos(x, y, z)
	if !g.insideW(p) {
		return "minecraft:air"
	}
	return g.w.nameAt(p)
}

// nameAt is the world state name: placed, else "minecraft:air",
// "minecraft:water" or "#solid" from ground.
func (w *pieceWorld) nameAt(p [3]int) string {
	if s, ok := w.m[p]; ok {
		return s.Name
	}
	switch w.kind(p[0], p[1], p[2]) {
	case GroundAir:
		return "minecraft:air"
	case GroundLiquid:
		return "minecraft:water"
	}
	return "#solid"
}

func isAirN(n string) bool {
	return n == "minecraft:air" || n == "minecraft:cave_air" || n == "minecraft:void_air"
}

// sturdy approximates isFaceSturdy/isSolidRender: full cubes are sturdy;
// the thin blocks mineshafts place are not.
func sturdy(n string) bool {
	if isAirN(n) || n == "minecraft:water" || n == "minecraft:lava" {
		return false
	}
	switch n {
	case "minecraft:oak_fence", "minecraft:dark_oak_fence", "minecraft:rail", "minecraft:cobweb",
		"minecraft:wall_torch", "minecraft:chain", "minecraft:chest":
		return false
	}
	return true
}

// supportsChain approximates canSupportCenter(DOWN) and not a falling block.
func supportsChain(n string) bool {
	switch n {
	case "minecraft:oak_fence", "minecraft:dark_oak_fence", "minecraft:chain":
		return true
	case "minecraft:sand", "minecraft:red_sand", "minecraft:gravel":
		return false
	}
	return sturdy(n)
}

func replaceable(n string) bool {
	return isAirN(n) || n == "minecraft:water" || n == "minecraft:lava" || n == "minecraft:glow_lichen" ||
		n == "minecraft:seagrass" || n == "minecraft:tall_seagrass"
}

func (m *msPiece) canBeReplaced(g *pc) func(x, y, z int) bool {
	pl, wd, fe := m.planks().Name, m.wood().Name, m.fence().Name
	return func(x, y, z int) bool {
		n := g.blockName(x, y, z)
		return n != pl && n != wd && n != fe && n != "minecraft:chain"
	}
}

// invalid is isInInvalidLocation: liquid on the faces of the piece box
// (grown by one) inside the chunk box. MINESHAFT_BLOCKING (deep dark) is
// not sampled.
func (g *pc) msInvalid() bool {
	b, c := g.bb, g.clip
	x0, y0, z0 := max(b[0]-1, c[0]), max(b[1]-1, c[1]), max(b[2]-1, c[2])
	x1, y1, z1 := min(b[3]+1, c[3]), min(b[4]+1, c[4]), min(b[5]+1, c[5])
	liq := func(x, y, z int) bool {
		n := g.w.nameAt([3]int{x, y, z})
		return n == "minecraft:water" || n == "minecraft:lava"
	}
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			if liq(x, y0, z) || liq(x, y1, z) {
				return true
			}
		}
	}
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			if liq(x, y, z0) || liq(x, y, z1) {
				return true
			}
		}
	}
	for z := z0; z <= z1; z++ {
		for y := y0; y <= y1; y++ {
			if liq(x0, y, z) || liq(x1, y, z) {
				return true
			}
		}
	}
	return false
}

func (g *pc) setPlanks(m *msPiece, x, y, z int) {
	if !g.isInterior(x, y, z) {
		return
	}
	p := g.pos(x, y, z)
	if !sturdy(g.w.nameAt(p)) {
		g.setW(p, m.planks())
	}
}

func (g *pc) upperHalfSphere(x0, y0, z0, x1, y1, z1 int, s BlockState) {
	fx, fy, fz := float32(x1-x0+1), float32(y1-y0+1), float32(z1-z0+1)
	cx, cz := float32(x0)+fx/2, float32(z0)+fz/2
	for y := y0; y <= y1; y++ {
		dy := float32(y-y0) / fy
		for x := x0; x <= x1; x++ {
			dx := (float32(x) - cx) / (fx * 0.5)
			for z := z0; z <= z1; z++ {
				dz := (float32(z) - cz) / (fz * 0.5)
				if dx*dx+dy*dy+dz*dz <= 1.05 {
					g.place(s, x, y, z)
				}
			}
		}
	}
}

func msPost(m *msPiece, g *pc) {
	g.canReplace = m.canBeReplaced(g)
	if g.msInvalid() {
		return
	}
	b := g.bb
	switch m.kind {
	case worldgen.MineshaftRoom:
		g.box(b[0], b[1]+1, b[2], b[3], min(b[1]+3, b[4]), b[5], caveAir, caveAir, false)
		for _, e := range m.entrances {
			g.box(e[0], e[4]-2, e[2], e[3], e[4], e[5], caveAir, caveAir, false)
		}
		g.upperHalfSphere(b[0], b[1]+4, b[2], b[3], b[4], b[5], caveAir)
	case worldgen.MineshaftCrossing:
		if m.twoFloor {
			g.box(b[0]+1, b[1], b[2], b[3]-1, b[1]+2, b[5], caveAir, caveAir, false)
			g.box(b[0], b[1], b[2]+1, b[3], b[1]+2, b[5]-1, caveAir, caveAir, false)
			g.box(b[0]+1, b[4]-2, b[2], b[3]-1, b[4], b[5], caveAir, caveAir, false)
			g.box(b[0], b[4]-2, b[2]+1, b[3], b[4], b[5]-1, caveAir, caveAir, false)
			g.box(b[0]+1, b[1]+3, b[2]+1, b[3]-1, b[1]+3, b[5]-1, caveAir, caveAir, false)
		} else {
			g.box(b[0]+1, b[1], b[2], b[3]-1, b[4], b[5], caveAir, caveAir, false)
			g.box(b[0], b[1], b[2]+1, b[3], b[4], b[5]-1, caveAir, caveAir, false)
		}
		pillar := func(x, z int) {
			if !isAirN(g.blockName(x, b[4]+1, z)) {
				g.box(x, b[1], z, x, b[4], z, m.planks(), caveAir, false)
			}
		}
		pillar(b[0]+1, b[2]+1)
		pillar(b[0]+1, b[5]-1)
		pillar(b[3]-1, b[2]+1)
		pillar(b[3]-1, b[5]-1)
		for x := b[0]; x <= b[3]; x++ {
			for z := b[2]; z <= b[5]; z++ {
				g.setPlanks(m, x, b[1]-1, z)
			}
		}
	case worldgen.MineshaftStairs:
		g.box(0, 5, 0, 2, 7, 1, caveAir, caveAir, false)
		g.box(0, 0, 7, 2, 2, 8, caveAir, caveAir, false)
		for i := 0; i < 5; i++ {
			d := 0
			if i < 4 {
				d = 1
			}
			g.box(0, 5-i-d, 2+i, 2, 7-i, 2+i, caveAir, caveAir, false)
		}
	case worldgen.MineshaftCorridor:
		msCorridor(m, g)
	}
}

func msCorridor(m *msPiece, g *pc) {
	r := g.rnd
	n := m.sections*5 - 1
	g.box(0, 0, 0, 2, 1, n, caveAir, caveAir, false)
	g.maybeBox(0.8, 0, 2, 0, 2, 2, n, caveAir, caveAir, false, false)
	if m.spider {
		g.maybeBox(0.6, 0, 0, 0, 2, 1, n, st("cobweb"), caveAir, false, true)
	}
	for i := 0; i < m.sections; i++ {
		z := 2 + i*5
		msSupport(m, g, 0, 0, z, 2, 2)
		web := func(f float32, x, y, z int) {
			if g.isInterior(x, y, z) && r.nextFloat() < f && msSturdyNeighbours(g, x, y, z, 2) {
				g.place(st("cobweb"), x, y, z)
			}
		}
		web(0.1, 0, 2, z-1)
		web(0.1, 2, 2, z-1)
		web(0.1, 0, 2, z+1)
		web(0.1, 2, 2, z+1)
		web(0.05, 0, 2, z-2)
		web(0.05, 2, 2, z-2)
		web(0.05, 0, 2, z+2)
		web(0.05, 2, 2, z+2)
		if r.nextInt(100) == 0 {
			msChest(g, 2, 0, z-1)
		}
		if r.nextInt(100) == 0 {
			msChest(g, 0, 0, z+1)
		}
		if m.spider && !m.placedSpider {
			sz := z - 1 + r.nextInt(3)
			p := g.pos(1, 0, sz)
			if g.insideW(p) && g.isInterior(1, 0, sz) {
				m.placedSpider = true
				g.setW(p, st("spawner"))
				// SpawnerBlockEntity.setEntityId draws nothing in 1.21.1.
			}
		}
	}
	for x := 0; x <= 2; x++ {
		for z := 0; z <= n; z++ {
			g.setPlanks(m, x, -1, z)
		}
	}
	msDoubleSupport(m, g, 0, -1, 2)
	if m.sections > 1 {
		msDoubleSupport(m, g, 0, -1, n-2)
	}
	if m.rails {
		rail := st("rail").with("shape", "north_south")
		for z := 0; z <= n; z++ {
			below := g.blockName(1, -1, z)
			if !isAirN(below) && sturdy(below) {
				f := float32(0.9)
				if g.isInterior(1, 0, z) {
					f = 0.7
				}
				g.maybeBlock(f, 1, 0, z, rail)
			}
		}
	}
}

func msChest(g *pc, x, y, z int) {
	p := g.pos(x, y, z)
	if !g.insideW(p) || !isAirN(g.w.nameAt(p)) || isAirN(g.w.nameAt([3]int{p[0], p[1] - 1, p[2]})) {
		return
	}
	shape := "east_west"
	if g.rnd.nextBool() {
		shape = "north_south"
	}
	g.place(st("rail").with("shape", shape), x, y, z)
	g.rnd.nextLong() // minecart loot seed
}

func msSupport(m *msPiece, g *pc, x0, y0, z, y1, x1 int) {
	for x := x0; x <= x1; x++ {
		if isAirN(g.blockName(x, y1+1, z)) {
			return
		}
	}
	pl, fe := m.planks(), m.fence()
	g.box(x0, y0, z, x0, y1-1, z, fe.with("west", "true"), caveAir, false)
	g.box(x1, y0, z, x1, y1-1, z, fe.with("east", "true"), caveAir, false)
	if g.rnd.nextInt(4) == 0 {
		g.box(x0, y1, z, x0, y1, z, pl, caveAir, false)
		g.box(x1, y1, z, x1, y1, z, pl, caveAir, false)
	} else {
		g.box(x0, y1, z, x1, y1, z, pl, caveAir, false)
		g.maybeBlock(0.05, x0+1, y1, z-1, st("wall_torch").with("facing", "south"))
		g.maybeBlock(0.05, x0+1, y1, z+1, st("wall_torch").with("facing", "north"))
	}
}

// msSturdyNeighbours: Direction.values() order is D, U, N, S, W, E; each
// neighbour inside the chunk box whose facing side is sturdy counts.
func msSturdyNeighbours(g *pc, x, y, z, need int) bool {
	p := g.pos(x, y, z)
	n := 0
	for _, d := range [][3]int{{0, -1, 0}, {0, 1, 0}, {0, 0, -1}, {0, 0, 1}, {-1, 0, 0}, {1, 0, 0}} {
		q := [3]int{p[0] + d[0], p[1] + d[1], p[2] + d[2]}
		if g.insideW(q) && sturdy(g.w.nameAt(q)) {
			n++
			if n >= need {
				return true
			}
		}
	}
	return false
}

func msDoubleSupport(m *msPiece, g *pc, x, y, z int) {
	pl := m.planks().Name
	if g.blockName(x, y, z) == pl {
		msPillarOrChain(m, g, x, y, z)
	}
	if g.blockName(x+2, y, z) == pl {
		msPillarOrChain(m, g, x+2, y, z)
	}
}

// msPillarOrChain is fillPillarDownOrChainUp: a log column down to a
// sturdy floor within 20, else a fence and chain up to a ceiling within 50.
func msPillarOrChain(m *msPiece, g *pc, x, y, z int) {
	p := g.pos(x, y, z)
	if !g.insideW(p) {
		return
	}
	y0 := p[1]
	down, up := true, true
	for i := 1; down || up; i++ {
		if down {
			q := [3]int{p[0], y0 - i, p[2]}
			n := g.w.nameAt(q)
			rep := replaceable(n) && n != "minecraft:lava"
			if !rep && sturdy(n) {
				for yy := y0 - i + 1; yy < y0; yy++ {
					g.setW([3]int{p[0], yy, p[2]}, m.wood())
				}
				return
			}
			down = i <= 20 && rep && q[1] > g.w.minY+1
		}
		if up {
			q := [3]int{p[0], y0 + i, p[2]}
			n := g.w.nameAt(q)
			rep := replaceable(n)
			if !rep && supportsChain(n) {
				g.setW([3]int{p[0], y0 + 1, p[2]}, m.fence())
				for yy := y0 + 2; yy < y0+i; yy++ {
					g.setW([3]int{p[0], yy, p[2]}, st("chain"))
				}
				return
			}
			up = i <= 50 && rep && q[1] < g.w.maxY-1
		}
	}
}
