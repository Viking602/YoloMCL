package worldgen

// Nether fortress piece ids (StructurePieceType names as saved in starts).
const (
	NeStart        = "NeStart"
	NeBridge       = "NeBS"
	NeBridgeEnd    = "NeBEF"
	NeBridgeCross  = "NeBCr"
	NeRoomCross    = "NeRC"
	NeStairsRoom   = "NeSR"
	NeThrone       = "NeMT"
	NeEntrance     = "NeCE"
	NeCorridor     = "NeSC"
	NeCorridorX    = "NeSCSC"
	NeCorridorR    = "NeSCRT"
	NeCorridorL    = "NeSCLT"
	NeCorridorStep = "NeCCS"
	NeBalcony      = "NeCTB"
	NeStalkRoom    = "NeCSR"
)

// FortressPiece is one generated nether fortress piece.
type FortressPiece struct {
	ID    string
	Box   Box
	Dir   int
	Depth int
	// Seed is BridgeEndFiller's selfSeed; Chest is a corridor turn's
	// isNeedingChest. Both are drawn in the piece constructor.
	Seed  int32
	Chest bool
}

type neWeight struct {
	id          string
	weight, max int
	inRow       bool
	placed      int
}

func (w *neWeight) ok() bool { return w.max == 0 || w.placed < w.max }

type neShape struct{ ox, oy, oz, w, h, d int }

var neShapes = map[string]neShape{
	NeBridge:       {-1, -3, 0, 5, 10, 19},
	NeBridgeEnd:    {-1, -3, 0, 5, 10, 8},
	NeBridgeCross:  {-8, -3, 0, 19, 10, 19},
	NeRoomCross:    {-2, 0, 0, 7, 9, 7},
	NeStairsRoom:   {-2, 0, 0, 7, 11, 7},
	NeThrone:       {-2, 0, 0, 7, 8, 9},
	NeEntrance:     {-5, -3, 0, 13, 14, 13},
	NeCorridor:     {-1, 0, 0, 5, 7, 5},
	NeCorridorX:    {-1, 0, 0, 5, 7, 5},
	NeCorridorR:    {-1, 0, 0, 5, 7, 5},
	NeCorridorL:    {-1, 0, 0, 5, 7, 5},
	NeCorridorStep: {-1, -7, 0, 5, 14, 10},
	NeBalcony:      {-3, 0, 0, 9, 7, 9},
	NeStalkRoom:    {-5, -3, 0, 13, 14, 13},
}

type fortressGen struct {
	r       *JavaRandom
	pieces  []*FortressPiece
	pending []*FortressPiece
	bridge  []*neWeight
	castle  []*neWeight
	prev    *neWeight
}

func (g *fortressGen) collides(b Box) bool {
	for _, p := range g.pieces {
		if p.Box.Intersects(b) {
			return true
		}
	}
	return false
}

// create mirrors each piece's static createPiece: ok box, no collision, then
// the constructor (whose random draws only happen once the piece exists).
func (g *fortressGen) create(id string, x, y, z, dir, depth int) *FortressPiece {
	s := neShapes[id]
	b := OrientBox(x, y, z, s.ox, s.oy, s.oz, s.w, s.h, s.d, dir)
	if b[1] <= 10 || g.collides(b) {
		return nil
	}
	p := &FortressPiece{ID: id, Box: b, Dir: dir, Depth: depth}
	switch id {
	case NeBridgeEnd:
		p.Seed = g.r.next(32) // selfSeed = random.nextInt()
	case NeCorridorR, NeCorridorL:
		p.Chest = g.r.NextInt(3) == 0 // isNeedingChest
	}
	return p
}

func (g *fortressGen) pick(list *[]*neWeight, x, y, z, dir, depth int) *FortressPiece {
	total, any := 0, false
	for _, w := range *list {
		if w.max > 0 && w.placed < w.max {
			any = true
		}
		total += w.weight
	}
	if any && total > 0 && depth <= 30 {
		for try := 0; try < 5; try++ {
			k := int(g.r.NextInt(int32(total)))
			for i, w := range *list {
				k -= w.weight
				if k >= 0 {
					continue
				}
				if !w.ok() || (w == g.prev && !w.inRow) {
					break
				}
				if p := g.create(w.id, x, y, z, dir, depth); p != nil {
					w.placed++
					g.prev = w
					if !w.ok() {
						*list = append((*list)[:i:i], (*list)[i+1:]...)
					}
					return p
				}
			}
		}
	}
	return g.create(NeBridgeEnd, x, y, z, dir, depth)
}

func (g *fortressGen) add(x, y, z, dir, depth int, castle bool) {
	start := g.pieces[0].Box
	if abs(x-start[0]) > 112 || abs(z-start[2]) > 112 {
		g.create(NeBridgeEnd, x, y, z, dir, depth) // built but never added
		return
	}
	list := &g.bridge
	if castle {
		list = &g.castle
	}
	if p := g.pick(list, x, y, z, dir, depth+1); p != nil {
		g.pieces = append(g.pieces, p)
		g.pending = append(g.pending, p)
	}
}

func (g *fortressGen) forward(p *FortressPiece, xo, yo int, castle bool) {
	b := p.Box
	switch p.Dir {
	case DirNorth:
		g.add(b[0]+xo, b[1]+yo, b[2]-1, p.Dir, p.Depth, castle)
	case DirSouth:
		g.add(b[0]+xo, b[1]+yo, b[5]+1, p.Dir, p.Depth, castle)
	case DirWest:
		g.add(b[0]-1, b[1]+yo, b[2]+xo, p.Dir, p.Depth, castle)
	default:
		g.add(b[3]+1, b[1]+yo, b[2]+xo, p.Dir, p.Depth, castle)
	}
}

func (g *fortressGen) left(p *FortressPiece, yo, xo int, castle bool) {
	b := p.Box
	if p.Dir == DirNorth || p.Dir == DirSouth {
		g.add(b[0]-1, b[1]+yo, b[2]+xo, DirWest, p.Depth, castle)
	} else {
		g.add(b[0]+xo, b[1]+yo, b[2]-1, DirNorth, p.Depth, castle)
	}
}

func (g *fortressGen) right(p *FortressPiece, yo, xo int, castle bool) {
	b := p.Box
	if p.Dir == DirNorth || p.Dir == DirSouth {
		g.add(b[3]+1, b[1]+yo, b[2]+xo, DirEast, p.Depth, castle)
	} else {
		g.add(b[0]+xo, b[1]+yo, b[5]+1, DirSouth, p.Depth, castle)
	}
}

func (g *fortressGen) children(p *FortressPiece) {
	switch p.ID {
	case NeStart, NeBridgeCross:
		g.forward(p, 8, 3, false)
		g.left(p, 3, 8, false)
		g.right(p, 3, 8, false)
	case NeBridge:
		g.forward(p, 1, 3, false)
	case NeRoomCross:
		g.forward(p, 2, 0, false)
		g.left(p, 0, 2, false)
		g.right(p, 0, 2, false)
	case NeStairsRoom:
		g.right(p, 6, 2, false)
	case NeEntrance:
		g.forward(p, 5, 3, true)
	case NeStalkRoom:
		g.forward(p, 5, 3, true)
		g.forward(p, 5, 11, true)
	case NeCorridor, NeCorridorStep:
		g.forward(p, 1, 0, true)
	case NeCorridorX:
		g.forward(p, 1, 0, true)
		g.left(p, 0, 1, true)
		g.right(p, 0, 1, true)
	case NeCorridorR:
		g.right(p, 0, 1, true)
	case NeCorridorL:
		g.left(p, 0, 1, true)
	case NeBalcony:
		i := 1
		if p.Dir == DirWest || p.Dir == DirNorth {
			i = 5
		}
		g.left(p, 0, i, g.r.NextInt(8) > 0)
		g.right(p, 0, i, g.r.NextInt(8) > 0)
	}
}

// Fortress mirrors NetherFortressStructure.generatePieces for the start in
// chunk (cx, cz). When heights is set the whole start is moved into y 48..70
// like StructurePiecesBuilder.moveInsideHeights.
func Fortress(seed int64, cx, cz int32, heights bool) []*FortressPiece {
	r := LargeFeatureRandom(seed, cx, cz)
	g := &fortressGen{r: r,
		bridge: []*neWeight{{NeBridge, 30, 0, true, 0}, {NeBridgeCross, 10, 4, false, 0}, {NeRoomCross, 10, 4, false, 0},
			{NeStairsRoom, 10, 3, false, 0}, {NeThrone, 5, 2, false, 0}, {NeEntrance, 5, 1, false, 0}},
		castle: []*neWeight{{NeCorridor, 25, 0, true, 0}, {NeCorridorX, 15, 5, false, 0}, {NeCorridorR, 5, 10, false, 0},
			{NeCorridorL, 5, 10, false, 0}, {NeCorridorStep, 10, 3, true, 0}, {NeBalcony, 7, 2, false, 0}, {NeStalkRoom, 5, 2, false, 0}},
	}
	dir := int(r.NextInt(4))
	x, z := int(cx)*16+2, int(cz)*16+2
	start := &FortressPiece{ID: NeStart, Box: Box{x, 64, z, x + 18, 73, z + 18}, Dir: dir} // 19x10x19 either way
	g.pieces = []*FortressPiece{start}
	g.children(start)
	for len(g.pending) > 0 {
		i := int(r.NextInt(int32(len(g.pending))))
		p := g.pending[i]
		g.pending = append(g.pending[:i:i], g.pending[i+1:]...)
		g.children(p)
	}
	if heights {
		lo, hi := g.pieces[0].Box[1], g.pieces[0].Box[4]
		for _, p := range g.pieces {
			lo, hi = min(lo, p.Box[1]), max(hi, p.Box[4])
		}
		span := 70 - 48 + 1 - (hi - lo + 1)
		y := 48
		if span > 1 {
			y += int(r.NextInt(int32(span)))
		}
		for _, p := range g.pieces {
			p.Box = p.Box.move(0, y-lo, 0)
		}
	}
	return g.pieces
}
