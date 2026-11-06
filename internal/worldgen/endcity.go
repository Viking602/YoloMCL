package worldgen

// endCitySizes are the end_city/*.nbt template sizes (vanilla data).
var endCitySizes = map[string][3]int{
	"base_floor": {10, 4, 10}, "base_roof": {12, 2, 12}, "bridge_end": {5, 6, 2},
	"bridge_gentle_stairs": {5, 7, 8}, "bridge_piece": {5, 6, 4}, "bridge_steep_stairs": {5, 7, 4},
	"fat_tower_base": {13, 4, 13}, "fat_tower_middle": {13, 8, 13}, "fat_tower_top": {17, 6, 17},
	"second_floor_1": {12, 8, 12}, "second_floor_2": {12, 8, 12}, "second_roof": {14, 2, 14},
	"ship": {13, 24, 29}, "third_floor_1": {14, 8, 14}, "third_floor_2": {14, 8, 14},
	"third_roof": {16, 2, 16}, "tower_base": {7, 7, 7}, "tower_piece": {7, 4, 7}, "tower_top": {9, 5, 9},
}

// EndCityPiece is one placed end city template.
type EndCityPiece struct {
	Name     string
	Pos      [3]int // template position
	Rot      int    // index into Rotations
	Box      Box
	genDepth int
}

func rotatePos(x, y, z, rot int) (int, int, int) {
	switch rot {
	case 1:
		return -z, y, x
	case 2:
		return -x, y, -z
	case 3:
		return z, y, -x
	}
	return x, y, z
}

func newEndCityPiece(name string, x, y, z, rot int) *EndCityPiece {
	s := endCitySizes[name]
	ex, ey, ez := rotatePos(s[0]-1, s[1]-1, s[2]-1, rot)
	b := Box{min(x, x+ex), min(y, y+ey), min(z, z+ez), max(x, x+ex), max(y, y+ey), max(z, z+ez)}
	return &EndCityPiece{Name: name, Pos: [3]int{x, y, z}, Rot: rot, Box: b}
}

// next is EndCityPieces.addPiece: offset is in the parent's rotated frame.
func (p *EndCityPiece) next(ox, oy, oz int, name string, rot int) *EndCityPiece {
	dx, dy, dz := rotatePos(ox, oy, oz, p.Rot)
	return newEndCityPiece(name, p.Pos[0]+dx, p.Pos[1]+dy, p.Pos[2]+dz, rot)
}

type endCityGen struct {
	r    *JavaRandom
	ship bool
}

type ecList struct{ ps []*EndCityPiece }

func (l *ecList) add(p *EndCityPiece) *EndCityPiece { l.ps = append(l.ps, p); return p }

const (
	ecTower = iota
	ecHouse
	ecBridge
	ecFat
)

var ecTowerBridges = [4][4]int{{0, 1, -1, 0}, {1, 6, -1, 1}, {3, 0, -1, 5}, {2, 5, -1, 6}}
var ecFatBridges = [4][4]int{{0, 4, -1, 0}, {1, 12, -1, 4}, {3, 0, -1, 8}, {2, 8, -1, 12}}

func (g *endCityGen) recurse(kind, depth int, parent *EndCityPiece, pos *[3]int, out *ecList) bool {
	if depth > 8 {
		return false
	}
	list := &ecList{}
	if !g.generate(kind, depth, parent, pos, list) {
		return false
	}
	id := int(g.r.next(32))
	for _, p := range list.ps {
		p.genDepth = id
		for _, q := range out.ps {
			if q.Box.Intersects(p.Box) {
				if q.genDepth != parent.genDepth {
					return false
				}
				break
			}
		}
	}
	out.ps = append(out.ps, list.ps...)
	return true
}

func (g *endCityGen) generate(kind, depth int, parent *EndCityPiece, pos *[3]int, l *ecList) bool {
	r, rot := g.r, parent.Rot
	switch kind {
	case ecHouse:
		if depth > 8 {
			return false
		}
		p := l.add(parent.next(pos[0], pos[1], pos[2], "base_floor", rot))
		switch r.NextInt(3) {
		case 0:
			l.add(p.next(-1, 4, -1, "base_roof", rot))
		case 1:
			p = l.add(p.next(-1, 0, -1, "second_floor_2", rot))
			p = l.add(p.next(-1, 8, -1, "second_roof", rot))
			g.recurse(ecTower, depth+1, p, nil, l)
		case 2:
			p = l.add(p.next(-1, 0, -1, "second_floor_2", rot))
			p = l.add(p.next(-1, 4, -1, "third_floor_2", rot))
			p = l.add(p.next(-1, 8, -1, "third_roof", rot))
			g.recurse(ecTower, depth+1, p, nil, l)
		}
		return true
	case ecTower:
		ox := 3 + int(r.NextInt(2))
		oz := 3 + int(r.NextInt(2))
		p := l.add(parent.next(ox, -3, oz, "tower_base", rot))
		p = l.add(p.next(0, 7, 0, "tower_piece", rot))
		var bridge *EndCityPiece
		if r.NextInt(3) == 0 {
			bridge = p
		}
		n := 1 + int(r.NextInt(3))
		for j := 0; j < n; j++ {
			p = l.add(p.next(0, 4, 0, "tower_piece", rot))
			if j < n-1 && r.NextBoolean() {
				bridge = p
			}
		}
		if bridge != nil {
			for _, t := range ecTowerBridges {
				if r.NextBoolean() {
					// Bridges hang off the chosen floor (saves; cubiomes uses the top piece).
					b := l.add(bridge.next(t[1], t[2], t[3], "bridge_end", (rot+t[0])%4))
					g.recurse(ecBridge, depth+1, b, nil, l)
				}
			}
		} else if depth != 7 {
			return g.recurse(ecFat, depth+1, p, nil, l)
		}
		l.add(p.next(-1, 4, -1, "tower_top", rot))
		return true
	case ecBridge:
		n := int(r.NextInt(4)) + 1
		p := l.add(parent.next(0, 0, -4, "bridge_piece", rot))
		p.genDepth = -1
		j := 0
		for k := 0; k < n; k++ {
			if r.NextBoolean() {
				p = l.add(p.next(0, j, -4, "bridge_piece", rot))
				j = 0
			} else {
				if r.NextBoolean() {
					p = l.add(p.next(0, j, -4, "bridge_steep_stairs", rot))
				} else {
					p = l.add(p.next(0, j, -8, "bridge_gentle_stairs", rot))
				}
				j = 4
			}
		}
		if !g.ship && r.NextInt(int32(10-depth)) == 0 {
			sx := -8 + int(r.NextInt(8))
			sz := -70 + int(r.NextInt(10))
			l.add(p.next(sx, j, sz, "ship", rot)) // the closing bridge_end stays on the bridge (saves; cubiomes moves it to the ship)
			g.ship = true
		} else if !g.recurse(ecHouse, depth+1, p, &[3]int{-3, j + 1, -11}, l) {
			return false
		}
		p = l.add(p.next(4, j, 0, "bridge_end", (rot+2)%4))
		p.genDepth = -1
		return true
	case ecFat:
		p := l.add(parent.next(-3, 4, -3, "fat_tower_base", rot))
		p = l.add(p.next(0, 4, 0, "fat_tower_middle", rot))
		for i := 0; i < 2 && r.NextInt(3) != 0; i++ {
			p = l.add(p.next(0, 8, 0, "fat_tower_middle", rot))
			for _, t := range ecFatBridges {
				if r.NextBoolean() {
					b := l.add(p.next(t[1], t[2], t[3], "bridge_end", (rot+t[0])%4))
					g.recurse(ecBridge, depth+1, b, nil, l)
				}
			}
		}
		l.add(p.next(-2, 8, -2, "fat_tower_top", rot))
		return true
	}
	return false
}

// EndCity mirrors EndCityStructure + EndCityPieces.startHouseTower for the
// start in chunk (cx, cz) with base height y (the surface sample is the
// caller's job; it does not change the layout).
func EndCity(seed int64, cx, cz int32, y int) []*EndCityPiece {
	r := LargeFeatureRandom(seed, cx, cz)
	rot := int(r.NextInt(4))
	g := &endCityGen{r: r}
	l := &ecList{}
	// getLowestYIn5by5BoxOffset7Blocks: block 7 of the chunk (cubiomes uses 8; saves say 7).
	p := l.add(newEndCityPiece("base_floor", int(cx)*16+7, y, int(cz)*16+7, rot))
	p = l.add(p.next(-1, 0, -1, "second_floor_1", rot))
	p = l.add(p.next(-1, 4, -1, "third_floor_1", rot))
	p = l.add(p.next(-1, 8, -1, "third_roof", rot))
	g.recurse(ecTower, 1, p, nil, l)
	return l.ps
}
