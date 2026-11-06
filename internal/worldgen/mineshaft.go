package worldgen

// Mineshaft piece kinds.
const (
	MineshaftRoom     = "room"
	MineshaftCorridor = "corridor"
	MineshaftCrossing = "crossing"
	MineshaftStairs   = "stairs"
)

// Horizontal directions in Direction order used by mineshaft code.
const (
	DirNorth = iota
	DirEast
	DirSouth
	DirWest
)

// Box is an inclusive block bounding box: minX, minY, minZ, maxX, maxY, maxZ.
type Box [6]int

func (b Box) XSpan() int { return b[3] - b[0] + 1 }
func (b Box) YSpan() int { return b[4] - b[1] + 1 }
func (b Box) ZSpan() int { return b[5] - b[2] + 1 }

func (b Box) Intersects(o Box) bool {
	return b[3] >= o[0] && b[0] <= o[3] && b[5] >= o[2] && b[2] <= o[5] && b[4] >= o[1] && b[1] <= o[4]
}

func (b Box) move(x, y, z int) Box {
	return Box{b[0] + x, b[1] + y, b[2] + z, b[3] + x, b[4] + y, b[5] + z}
}

// MineshaftPiece is one generated mineshaft piece.
type MineshaftPiece struct {
	Kind      string
	Box       Box
	Dir       int
	Depth     int
	Rails     bool // corridor
	Spider    bool // corridor
	Sections  int  // corridor
	TwoFloors bool // crossing
	// Entrances are the room's childEntranceBoxes.
	Entrances []Box
}

type mineshaftGen struct {
	r      *JavaRandom
	pieces []*MineshaftPiece
}

func (r *JavaRandom) NextBoolean() bool { return r.next(1) != 0 }

func (g *mineshaftGen) collides(b Box) bool {
	for _, p := range g.pieces {
		if p.Box.Intersects(b) {
			return true
		}
	}
	return false
}

func (g *mineshaftGen) gen(x, y, z, dir, depth int) *MineshaftPiece {
	if depth > 8 {
		return nil
	}
	start := g.pieces[0].Box
	if abs(x-start[0]) > 80 || abs(z-start[2]) > 80 {
		return nil
	}
	p := g.create(x, y, z, dir, depth+1)
	if p != nil {
		g.pieces = append(g.pieces, p)
		g.children(p)
	}
	return p
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (g *mineshaftGen) create(x, y, z, dir, depth int) *MineshaftPiece {
	r := g.r
	n := r.NextInt(100)
	switch {
	case n >= 80:
		h := 2
		if r.NextInt(4) == 0 {
			h = 6
		}
		var b Box
		switch dir {
		case DirSouth:
			b = Box{-1, 0, 0, 3, h, 4}
		case DirWest:
			b = Box{-4, 0, -1, 0, h, 3}
		case DirEast:
			b = Box{0, 0, -1, 4, h, 3}
		default:
			b = Box{-1, 0, -4, 3, h, 0}
		}
		b = b.move(x, y, z)
		if g.collides(b) {
			return nil
		}
		return &MineshaftPiece{Kind: MineshaftCrossing, Box: b, Dir: dir, Depth: depth, TwoFloors: b.YSpan() > 3}
	case n >= 70:
		var b Box
		switch dir {
		case DirSouth:
			b = Box{0, -5, 0, 2, 2, 8}
		case DirWest:
			b = Box{-8, -5, 0, 0, 2, 2}
		case DirEast:
			b = Box{0, -5, 0, 8, 2, 2}
		default:
			b = Box{0, -5, -8, 2, 2, 0}
		}
		b = b.move(x, y, z)
		if g.collides(b) {
			return nil
		}
		return &MineshaftPiece{Kind: MineshaftStairs, Box: b, Dir: dir, Depth: depth}
	default:
		for i := int(r.NextInt(3)) + 2; i > 0; i-- {
			l := i * 5
			var b Box
			switch dir {
			case DirSouth:
				b = Box{0, 0, 0, 2, 2, l - 1}
			case DirWest:
				b = Box{-(l - 1), 0, 0, 0, 2, 2}
			case DirEast:
				b = Box{0, 0, 0, l - 1, 2, 2}
			default:
				b = Box{0, 0, -(l - 1), 2, 2, 0}
			}
			b = b.move(x, y, z)
			if !g.collides(b) {
				p := &MineshaftPiece{Kind: MineshaftCorridor, Box: b, Dir: dir, Depth: depth}
				p.Rails = r.NextInt(3) == 0
				p.Spider = !p.Rails && r.NextInt(23) == 0
				if dir == DirNorth || dir == DirSouth {
					p.Sections = b.ZSpan() / 5
				} else {
					p.Sections = b.XSpan() / 5
				}
				return p
			}
		}
		return nil
	}
}

func (g *mineshaftGen) children(p *MineshaftPiece) {
	r, b, d := g.r, p.Box, p.Depth
	switch p.Kind {
	case MineshaftRoom:
		ys := b.YSpan() - 3 - 1
		if ys <= 0 {
			ys = 1
		}
		xs, zs := b.XSpan(), b.ZSpan()
		for i := 0; i < xs; i += 4 {
			i += int(r.NextInt(int32(xs)))
			if i+3 > xs {
				break
			}
			if c := g.gen(b[0]+i, b[1]+int(r.NextInt(int32(ys)))+1, b[2]-1, DirNorth, d); c != nil {
				p.Entrances = append(p.Entrances, Box{c.Box[0], c.Box[1], b[2], c.Box[3], c.Box[4], b[2] + 1})
			}
		}
		for i := 0; i < xs; i += 4 {
			i += int(r.NextInt(int32(xs)))
			if i+3 > xs {
				break
			}
			if c := g.gen(b[0]+i, b[1]+int(r.NextInt(int32(ys)))+1, b[5]+1, DirSouth, d); c != nil {
				p.Entrances = append(p.Entrances, Box{c.Box[0], c.Box[1], b[5] - 1, c.Box[3], c.Box[4], b[5]})
			}
		}
		for i := 0; i < zs; i += 4 {
			i += int(r.NextInt(int32(zs)))
			if i+3 > zs {
				break
			}
			if c := g.gen(b[0]-1, b[1]+int(r.NextInt(int32(ys)))+1, b[2]+i, DirWest, d); c != nil {
				p.Entrances = append(p.Entrances, Box{b[0], c.Box[1], c.Box[2], b[0] + 1, c.Box[4], c.Box[5]})
			}
		}
		for i := 0; i < zs; i += 4 {
			i += int(r.NextInt(int32(zs)))
			if i+3 > zs {
				break
			}
			if c := g.gen(b[3]+1, b[1]+int(r.NextInt(int32(ys)))+1, b[2]+i, DirEast, d); c != nil {
				p.Entrances = append(p.Entrances, Box{b[3] - 1, c.Box[1], c.Box[2], b[3], c.Box[4], c.Box[5]})
			}
		}
	case MineshaftCorridor:
		n := r.NextInt(4)
		y := func() int { return b[1] - 1 + int(r.NextInt(3)) }
		switch p.Dir {
		case DirSouth:
			if n <= 1 {
				g.gen(b[0], y(), b[5]+1, DirSouth, d)
			} else if n == 2 {
				g.gen(b[0]-1, y(), b[5]-3, DirWest, d)
			} else {
				g.gen(b[3]+1, y(), b[5]-3, DirEast, d)
			}
		case DirWest:
			if n <= 1 {
				g.gen(b[0]-1, y(), b[2], DirWest, d)
			} else if n == 2 {
				g.gen(b[0], y(), b[2]-1, DirNorth, d)
			} else {
				g.gen(b[0], y(), b[5]+1, DirSouth, d)
			}
		case DirEast:
			if n <= 1 {
				g.gen(b[3]+1, y(), b[2], DirEast, d)
			} else if n == 2 {
				g.gen(b[3]-3, y(), b[2]-1, DirNorth, d)
			} else {
				g.gen(b[3]-3, y(), b[5]+1, DirSouth, d)
			}
		default:
			if n <= 1 {
				g.gen(b[0], y(), b[2]-1, DirNorth, d)
			} else if n == 2 {
				g.gen(b[0]-1, y(), b[2], DirWest, d)
			} else {
				g.gen(b[3]+1, y(), b[2], DirEast, d)
			}
		}
		if d < 8 {
			if p.Dir != DirNorth && p.Dir != DirSouth {
				for i := b[0] + 3; i+3 <= b[3]; i += 5 {
					s := r.NextInt(5)
					if s == 0 {
						g.gen(i, b[1], b[2]-1, DirNorth, d+1)
					} else if s == 1 {
						g.gen(i, b[1], b[5]+1, DirSouth, d+1)
					}
				}
			} else {
				for i := b[2] + 3; i+3 <= b[5]; i += 5 {
					s := r.NextInt(5)
					if s == 0 {
						g.gen(b[0]-1, b[1], i, DirWest, d+1)
					} else if s == 1 {
						g.gen(b[3]+1, b[1], i, DirEast, d+1)
					}
				}
			}
		}
	case MineshaftCrossing:
		switch p.Dir {
		case DirSouth:
			g.gen(b[0]+1, b[1], b[5]+1, DirSouth, d)
			g.gen(b[0]-1, b[1], b[2]+1, DirWest, d)
			g.gen(b[3]+1, b[1], b[2]+1, DirEast, d)
		case DirWest:
			g.gen(b[0]+1, b[1], b[2]-1, DirNorth, d)
			g.gen(b[0]+1, b[1], b[5]+1, DirSouth, d)
			g.gen(b[0]-1, b[1], b[2]+1, DirWest, d)
		case DirEast:
			g.gen(b[0]+1, b[1], b[2]-1, DirNorth, d)
			g.gen(b[0]+1, b[1], b[5]+1, DirSouth, d)
			g.gen(b[3]+1, b[1], b[2]+1, DirEast, d)
		default:
			g.gen(b[0]+1, b[1], b[2]-1, DirNorth, d)
			g.gen(b[0]-1, b[1], b[2]+1, DirWest, d)
			g.gen(b[3]+1, b[1], b[2]+1, DirEast, d)
		}
		if p.TwoFloors {
			if r.NextBoolean() {
				g.gen(b[0]+1, b[1]+4, b[2]-1, DirNorth, d)
			}
			if r.NextBoolean() {
				g.gen(b[0]-1, b[1]+4, b[2]+1, DirWest, d)
			}
			if r.NextBoolean() {
				g.gen(b[3]+1, b[1]+4, b[2]+1, DirEast, d)
			}
			if r.NextBoolean() {
				g.gen(b[0]+1, b[1]+4, b[5]+1, DirSouth, d)
			}
		}
	case MineshaftStairs:
		switch p.Dir {
		case DirSouth:
			g.gen(b[0], b[1], b[5]+1, DirSouth, d)
		case DirWest:
			g.gen(b[0]-1, b[1], b[2], DirWest, d)
		case DirEast:
			g.gen(b[3]+1, b[1], b[2], DirEast, d)
		default:
			g.gen(b[0], b[1], b[2]-1, DirNorth, d)
		}
	}
}

// MineshaftLayout is the seeded piece tree of one mineshaft start.
type MineshaftLayout struct {
	Pieces []*MineshaftPiece
	r      *JavaRandom
}

// Bounds is the union of all piece boxes.
func (l *MineshaftLayout) Bounds() Box {
	u := l.Pieces[0].Box
	for _, p := range l.Pieces[1:] {
		b := p.Box
		u = Box{min(u[0], b[0]), min(u[1], b[1]), min(u[2], b[2]), max(u[3], b[3]), max(u[4], b[4]), max(u[5], b[5])}
	}
	return u
}

// Shift moves every piece vertically.
func (l *MineshaftLayout) Shift(dy int) {
	for _, p := range l.Pieces {
		p.Box = p.Box.move(0, dy, 0)
		for i := range p.Entrances {
			p.Entrances[i] = p.Entrances[i].move(0, dy, 0)
		}
	}
}

// MoveBelowSeaLevel mirrors StructurePiecesBuilder.moveBelowSeaLevel for the
// normal mineshaft (offset 10). It consumes the same random as the game.
func (l *MineshaftLayout) MoveBelowSeaLevel(seaLevel, minY int) int {
	top := seaLevel - 10
	b := l.Bounds()
	j := b.YSpan() + minY + 1
	if j < top {
		j += int(l.r.NextInt(int32(top - j)))
	}
	k := j - b[4]
	l.Shift(k)
	return k
}

// MesaY mirrors the badlands variant: it moves the layout's center to a
// height between sea level and the surface at the center (surface from WG heightmap).
func (l *MineshaftLayout) MesaY(seaLevel int, surface func(x, z int) int) int {
	b := l.Bounds()
	cx, cy, cz := b[0]+(b[3]-b[0]+1)/2, b[1]+(b[4]-b[1]+1)/2, b[2]+(b[5]-b[2]+1)/2
	h := surface(cx, cz)
	k := seaLevel
	if h > seaLevel {
		k = seaLevel + int(l.r.NextInt(int32(h-seaLevel+1)))
	}
	l.Shift(k - cy)
	return k - cy
}

// Mineshaft builds the piece tree before vertical adjustment (room at y=50).
// skipDouble consumes the findGenerationPoint nextDouble first.
func Mineshaft(seed int64, cx, cz int32, skipDouble bool) *MineshaftLayout {
	r := LargeFeatureRandom(seed, cx, cz)
	if skipDouble {
		r.NextDouble()
	}
	x, z := int(cx)*16+2, int(cz)*16+2
	room := &MineshaftPiece{Kind: MineshaftRoom, Box: Box{x, 50, z, x + 7 + int(r.NextInt(6)), 54 + int(r.NextInt(6)), z + 7 + int(r.NextInt(6))}}
	g := &mineshaftGen{r: r, pieces: []*MineshaftPiece{room}}
	g.children(room)
	return &MineshaftLayout{Pieces: g.pieces, r: r}
}

// FrequencyPasses mirrors StructurePlacement.FrequencyReductionMethod for the
// chunk (cx, cz). Unknown methods report ok=false.
func FrequencyPasses(method string, seed int64, salt int32, cx, cz int32, frequency float64) (pass, ok bool) {
	if frequency >= 1 {
		return true, true
	}
	r := &JavaRandom{}
	switch method {
	case "", "default":
		// StructurePlacement.probabilityReducer (javap 1.21.1): calls
		// setLargeFeatureWithSalt(seed, salt, chunkX, chunkZ) — salt is the
		// "x" multiplier, chunkX the "z", chunkZ the additive salt. That is
		// not the same argument order as setLargeFeatureWithSalt's names.
		r.SetSeed(seed + int64(salt)*341873128712 + int64(cx)*132897987541 + int64(cz))
		return r.NextFloat() < float32(frequency), true
	case "legacy_type_1":
		// StructurePlacement.legacyPillagerOutpostFrequencyReducer
		i, j := cx>>4, cz>>4
		r.SetSeed(int64(i^j<<4) ^ seed)
		r.next(32) // nextInt()
		n := int32(float32(1) / float32(frequency))
		if n < 1 {
			return true, true
		}
		return r.NextInt(n) == 0, true
	case "legacy_type_2":
		r.SetSeed(int64(cx)*341873128712 + int64(cz)*132897987541 + seed + 10387320)
		return r.NextFloat() < float32(frequency), true
	case "legacy_type_3":
		return LargeFeatureRandom(seed, cx, cz).NextDouble() < frequency, true
	}
	return false, false
}

// NextFloat mirrors java.util.Random.nextFloat.
func (r *JavaRandom) NextFloat() float32 { return float32(r.next(24)) / (1 << 24) }

// World maps piece-local coordinates to world coordinates the way
// StructurePiece.getWorldX/Y/Z does for the piece orientation.
func (p *MineshaftPiece) World(lx, ly, lz int) (int, int, int) {
	b := p.Box
	y := b[1] + ly
	switch p.Dir {
	case DirNorth:
		return b[0] + lx, y, b[5] - lz
	case DirSouth:
		return b[0] + lx, y, b[2] + lz
	case DirWest:
		return b[3] - lz, y, b[2] + lx
	default:
		return b[0] + lz, y, b[2] + lx
	}
}

// CorridorLength is the corridor's last local z (numSections*5 - 1).
func (p *MineshaftPiece) CorridorLength() int { return p.Sections*5 - 1 }

// SupportZ lists the local z of each corridor support (2 + n*5).
func (p *MineshaftPiece) SupportZ() []int {
	out := make([]int, 0, p.Sections)
	for n := 0; n < p.Sections; n++ {
		out = append(out, 2+n*5)
	}
	return out
}
