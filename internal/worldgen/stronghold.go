package worldgen

// OrientBox is BoundingBox.orientBox.
func OrientBox(x, y, z, ox, oy, oz, w, h, d, dir int) Box {
	switch dir {
	case DirNorth:
		return Box{x + ox, y + oy, z - d + 1 + oz, x + w - 1 + ox, y + h - 1 + oy, z + oz}
	case DirSouth:
		return Box{x + ox, y + oy, z + oz, x + w - 1 + ox, y + h - 1 + oy, z + d - 1 + oz}
	case DirWest:
		return Box{x - d + 1 + oz, y + oy, z + ox, x + oz, y + h - 1 + oy, z + w - 1 + ox}
	default:
		return Box{x + oz, y + oy, z + ox, x + d - 1 + oz, y + h - 1 + oy, z + w - 1 + ox}
	}
}

// Stronghold piece ids as saved in structure starts.
const (
	ShStart     = "minecraft:shstart"
	ShStraight  = "minecraft:shs"
	ShPrison    = "minecraft:shph"
	ShLeft      = "minecraft:shlt"
	ShRight     = "minecraft:shrt"
	ShRoom      = "minecraft:shrc"
	ShStraightS = "minecraft:shssd"
	ShStairs    = "minecraft:shsd"
	ShFive      = "minecraft:sh5c"
	ShChest     = "minecraft:shcc"
	ShLibrary   = "minecraft:shli"
	ShPortal    = "minecraft:shpr"
	ShFiller    = "minecraft:shfc"
)

// StrongholdPiece is one piece of a generated stronghold.
type StrongholdPiece struct {
	ID    string
	Box   Box
	Dir   int
	Depth int
	Door  string
	// Straight: Left/Right children; FiveCrossing: LeftLow, LeftHigh, RightLow, RightHigh.
	Flags  [4]bool
	Source bool
	Tall   bool
	Steps  int
	Kind   int // RoomCrossing variant
}

// World maps local piece coordinates like StructurePiece.getWorldX/Y/Z.
func (p *StrongholdPiece) World(lx, ly, lz int) (int, int, int) {
	m := MineshaftPiece{Box: p.Box, Dir: p.Dir}
	return m.World(lx, ly, lz)
}

type shWeight struct {
	id           string
	weight, max  int
	placed       int
	minDepthOver int // doPlace requires depth > this (-1 = none)
}

type shGen struct {
	r        *JavaRandom
	pieces   []*StrongholdPiece
	weights  []*shWeight
	total    int
	imposed  string
	previous *shWeight
	pending  []*StrongholdPiece
	start    *StrongholdPiece
	portal   bool
}

var doorKinds = []string{"OPENING", "OPENING", "WOOD_DOOR", "GRATES", "IRON_DOOR"}

func (g *shGen) door() string { return doorKinds[g.r.NextInt(5)] }

func (g *shGen) collides(b Box) *StrongholdPiece {
	for _, p := range g.pieces {
		if p.Box.Intersects(b) {
			return p
		}
	}
	return nil
}

func okBox(b Box) bool { return b[1] > 10 }

func (g *shGen) updateWeights() bool {
	ok := false
	g.total = 0
	for _, w := range g.weights {
		if w.max > 0 && w.placed < w.max {
			ok = true
		}
		g.total += w.weight
	}
	return ok
}

func (w *shWeight) doPlace(depth int) bool {
	return (w.max == 0 || w.placed < w.max) && depth > w.minDepthOver
}

func (g *shGen) create(id string, x, y, z, dir, depth int) *StrongholdPiece {
	mk := func(b Box) *StrongholdPiece { return &StrongholdPiece{ID: id, Box: b, Dir: dir, Depth: depth} }
	try := func(ox, oy, oz, w, h, d int) *StrongholdPiece {
		b := OrientBox(x, y, z, ox, oy, oz, w, h, d, dir)
		if !okBox(b) || g.collides(b) != nil {
			return nil
		}
		return mk(b)
	}
	var p *StrongholdPiece
	switch id {
	case ShStraight:
		if p = try(-1, -1, 0, 5, 5, 7); p != nil {
			p.Door = g.door()
			p.Flags[0] = g.r.NextInt(2) == 0
			p.Flags[1] = g.r.NextInt(2) == 0
		}
	case ShPrison:
		if p = try(-1, -1, 0, 9, 5, 11); p != nil {
			p.Door = g.door()
		}
	case ShLeft, ShRight:
		if p = try(-1, -1, 0, 5, 5, 5); p != nil {
			p.Door = g.door()
		}
	case ShRoom:
		if p = try(-4, -1, 0, 11, 7, 11); p != nil {
			p.Door = g.door()
			p.Kind = int(g.r.NextInt(5))
		}
	case ShStraightS:
		if p = try(-1, -7, 0, 5, 11, 8); p != nil {
			p.Door = g.door()
		}
	case ShStairs:
		if p = try(-1, -7, 0, 5, 11, 5); p != nil {
			p.Door = g.door()
		}
	case ShFive:
		if p = try(-4, -3, 0, 10, 9, 11); p != nil {
			p.Door = g.door()
			p.Flags[0] = g.r.NextBoolean()
			p.Flags[1] = g.r.NextBoolean()
			p.Flags[2] = g.r.NextBoolean()
			p.Flags[3] = g.r.NextInt(3) > 0
		}
	case ShChest:
		if p = try(-1, -1, 0, 5, 5, 7); p != nil {
			p.Door = g.door()
		}
	case ShLibrary:
		p = try(-4, -1, 0, 14, 11, 15)
		if p == nil {
			p = try(-4, -1, 0, 14, 6, 15)
		}
		if p != nil {
			p.Door = g.door()
			p.Tall = p.Box.YSpan() > 6
		}
	case ShPortal:
		p = try(-4, -1, 0, 11, 8, 16)
	}
	return p
}

func (g *shGen) fromSmallDoor(x, y, z, dir, depth int) *StrongholdPiece {
	if !g.updateWeights() {
		return nil
	}
	if g.imposed != "" {
		id := g.imposed
		g.imposed = ""
		if p := g.create(id, x, y, z, dir, depth); p != nil {
			return p
		}
	}
	for i := 0; i < 5; i++ {
		j := int(g.r.NextInt(int32(g.total)))
		for wi, w := range g.weights {
			j -= w.weight
			if j < 0 {
				if !w.doPlace(depth) || w == g.previous {
					break
				}
				if p := g.create(w.id, x, y, z, dir, depth); p != nil {
					w.placed++
					g.previous = w
					if !(w.max == 0 || w.placed < w.max) {
						g.weights = append(g.weights[:wi:wi], g.weights[wi+1:]...)
					}
					return p
				}
			}
		}
	}
	b := OrientBox(x, y, z, -1, -1, 0, 5, 5, 4, dir)
	if c := g.collides(b); c != nil && c.Box[1] == b[1] {
		for i := 2; i >= 1; i-- {
			if !c.Box.Intersects(OrientBox(x, y, z, -1, -1, 0, 5, 5, i, dir)) {
				fb := OrientBox(x, y, z, -1, -1, 0, 5, 5, i+1, dir)
				if fb[1] > 1 {
					steps := fb.ZSpan()
					if dir == DirWest || dir == DirEast {
						steps = fb.XSpan()
					}
					return &StrongholdPiece{ID: ShFiller, Box: fb, Dir: dir, Depth: depth, Steps: steps}
				}
				return nil
			}
		}
	}
	return nil
}

func (g *shGen) add(x, y, z, dir, depth int) {
	if depth > 50 {
		return
	}
	if abs(x-g.start.Box[0]) > 112 || abs(z-g.start.Box[2]) > 112 {
		return
	}
	if p := g.fromSmallDoor(x, y, z, dir, depth+1); p != nil {
		g.pieces = append(g.pieces, p)
		g.pending = append(g.pending, p)
	}
}

func (g *shGen) forward(p *StrongholdPiece, xo, yo int) {
	b := p.Box
	switch p.Dir {
	case DirNorth:
		g.add(b[0]+xo, b[1]+yo, b[2]-1, p.Dir, p.Depth)
	case DirSouth:
		g.add(b[0]+xo, b[1]+yo, b[5]+1, p.Dir, p.Depth)
	case DirWest:
		g.add(b[0]-1, b[1]+yo, b[2]+xo, p.Dir, p.Depth)
	case DirEast:
		g.add(b[3]+1, b[1]+yo, b[2]+xo, p.Dir, p.Depth)
	}
}

func (g *shGen) left(p *StrongholdPiece, yo, zo int) {
	b := p.Box
	switch p.Dir {
	case DirNorth, DirSouth:
		g.add(b[0]-1, b[1]+yo, b[2]+zo, DirWest, p.Depth)
	default:
		g.add(b[0]+zo, b[1]+yo, b[2]-1, DirNorth, p.Depth)
	}
}

func (g *shGen) right(p *StrongholdPiece, yo, zo int) {
	b := p.Box
	switch p.Dir {
	case DirNorth, DirSouth:
		g.add(b[3]+1, b[1]+yo, b[2]+zo, DirEast, p.Depth)
	default:
		g.add(b[0]+zo, b[1]+yo, b[5]+1, DirSouth, p.Depth)
	}
}

func (g *shGen) children(p *StrongholdPiece) {
	switch p.ID {
	case ShStart, ShStairs:
		if p.Source {
			g.imposed = ShFive
		}
		g.forward(p, 1, 1)
	case ShStraight:
		g.forward(p, 1, 1)
		if p.Flags[0] {
			g.left(p, 1, 2)
		}
		if p.Flags[1] {
			g.right(p, 1, 2)
		}
	case ShChest, ShStraightS, ShPrison:
		g.forward(p, 1, 1)
	case ShLeft:
		if p.Dir == DirNorth || p.Dir == DirEast {
			g.left(p, 1, 1)
		} else {
			g.right(p, 1, 1)
		}
	case ShRight:
		if p.Dir == DirNorth || p.Dir == DirEast {
			g.right(p, 1, 1)
		} else {
			g.left(p, 1, 1)
		}
	case ShRoom:
		g.forward(p, 4, 1)
		g.left(p, 1, 4)
		g.right(p, 1, 4)
	case ShFive:
		i, j := 3, 5
		if p.Dir == DirWest || p.Dir == DirNorth {
			i, j = 8-i, 8-j
		}
		g.forward(p, 5, 1)
		if p.Flags[0] {
			g.left(p, i, 1)
		}
		if p.Flags[1] {
			g.left(p, j, 7)
		}
		if p.Flags[2] {
			g.right(p, i, 1)
		}
		if p.Flags[3] {
			g.right(p, j, 7)
		}
	case ShPortal:
		g.portal = true
	}
}

// Stronghold mirrors StrongholdStructure.generatePieces for the start chunk.
func Stronghold(seed int64, cx, cz int32, seaLevel, minY int) []*StrongholdPiece {
	for attempt := int64(0); attempt < 64; attempt++ {
		g := &shGen{r: LargeFeatureRandom(seed+attempt, cx, cz)}
		for _, w := range []shWeight{{ShStraight, 40, 0, 0, -1}, {ShPrison, 5, 5, 0, -1}, {ShLeft, 20, 0, 0, -1}, {ShRight, 20, 0, 0, -1}, {ShRoom, 10, 6, 0, -1}, {ShStraightS, 5, 5, 0, -1}, {ShStairs, 5, 5, 0, -1}, {ShFive, 5, 4, 0, -1}, {ShChest, 5, 4, 0, -1}, {ShLibrary, 10, 2, 0, 4}, {ShPortal, 20, 1, 0, 5}} {
			w := w
			g.weights = append(g.weights, &w)
		}
		dir := []int{DirNorth, DirEast, DirSouth, DirWest}[g.r.NextInt(4)]
		x, z := int(cx)*16+2, int(cz)*16+2
		w, d := 5, 5
		g.start = &StrongholdPiece{ID: ShStart, Dir: dir, Source: true, Door: "OPENING", Box: Box{x, 64, z, x + w - 1, 64 + 10, z + d - 1}}
		g.pieces = []*StrongholdPiece{g.start}
		g.children(g.start)
		for len(g.pending) > 0 {
			j := g.r.NextInt(int32(len(g.pending)))
			p := g.pending[j]
			g.pending = append(g.pending[:j], g.pending[j+1:]...)
			g.children(p)
		}
		// moveBelowSeaLevel(seaLevel, minY, random, 10)
		maxY, minYb := g.pieces[0].Box[4], g.pieces[0].Box[1]
		for _, p := range g.pieces {
			maxY, minYb = max(maxY, p.Box[4]), min(minYb, p.Box[1])
		}
		limit := seaLevel - 10
		k := maxY - minYb + 1 + minY + 1
		if k < limit {
			k += int(g.r.NextInt(int32(limit - k)))
		}
		dy := k - maxY
		for _, p := range g.pieces {
			p.Box[1] += dy
			p.Box[4] += dy
		}
		if g.portal {
			return g.pieces
		}
	}
	return nil
}
