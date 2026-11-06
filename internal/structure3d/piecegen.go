package structure3d

import (
	"fmt"
	"sort"

	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// Ground classifies the world block a structure piece builds into before
// the piece touches it, for the vanilla reads in StructurePiece (generateBox
// with skipAir, fillColumnDown, isInterior).
type Ground func(x, y, z int) GroundKind

// GroundKind is the class of a terrain block.
type GroundKind uint8

const (
	GroundAir    GroundKind = iota // air or cave air
	GroundLiquid                   // water or lava (replaceable by structures)
	GroundSolid
)

// pieceWorld is the block grid StructurePiece.postProcess writes into: the
// placed states (air included, so carving shows) over the terrain.
type pieceWorld struct {
	m      map[[3]int]BlockState
	ground Ground
	minY   int // getMinBuildHeight
	// fallbackFloor ends fillColumnDown when there is no ground sampler.
	fallbackFloor int
	// maxY is getMaxBuildHeight; the chunk's writable box spans minY..maxY-1.
	maxY int
	// floor is the OCEAN_FLOOR_WG height isInterior reads (nil: estimated
	// from ground).
	floor func(x, z int) int
}

func newPieceWorld(ground Ground, minY, fallbackFloor int) *pieceWorld {
	return &pieceWorld{m: map[[3]int]BlockState{}, ground: ground, minY: minY, maxY: 4096, fallbackFloor: fallbackFloor}
}

func (w *pieceWorld) kind(x, y, z int) GroundKind {
	if st, ok := w.m[[3]int{x, y, z}]; ok {
		switch st.Name {
		case "minecraft:air", "minecraft:cave_air", "minecraft:void_air":
			return GroundAir
		case "minecraft:water", "minecraft:lava":
			return GroundLiquid
		}
		return GroundSolid
	}
	if w.ground == nil {
		if y <= w.fallbackFloor {
			return GroundSolid
		}
		return GroundAir
	}
	return w.ground(x, y, z)
}

// blocks lists the non-air placed states sorted by position.
func (w *pieceWorld) blocks() []WorldBlock {
	out := make([]WorldBlock, 0, len(w.m))
	for k, v := range w.m {
		if v.Name == "" || v.Name == "minecraft:air" || v.Name == "minecraft:cave_air" {
			continue
		}
		out = append(out, WorldBlock{X: k[0], Y: k[1], Z: k[2], State: v})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Y != b.Y {
			return a.Y < b.Y
		}
		if a.Z != b.Z {
			return a.Z < b.Z
		}
		return a.X < b.X
	})
	return out
}

// carved lists positions a piece set to air (terrain it cuts away).
func (w *pieceWorld) carved() [][3]int {
	var out [][3]int
	for k, v := range w.m {
		if v.Name == "minecraft:air" || v.Name == "minecraft:cave_air" {
			out = append(out, k)
		}
	}
	return out
}

// pieceRand is the RandomSource postProcess gets.
type pieceRand interface {
	nextInt(n int) int
	nextFloat() float32
	nextBool() bool
	nextLong() int64
}

// jr is LegacyRandomSource (java.util.Random).
type jr struct{ r worldgen.JavaRandom }

func newJR(seed int64) *jr {
	j := &jr{}
	j.r.SetSeed(seed)
	return j
}
func (j *jr) nextInt(n int) int  { return int(j.r.NextInt(int32(n))) }
func (j *jr) nextFloat() float32 { return j.r.NextFloat() }
func (j *jr) nextBool() bool     { return j.r.NextBoolean() }
func (j *jr) nextLong() int64    { return j.r.NextLong() }

// pc is one StructurePiece being post-processed into one chunk.
type pc struct {
	w    *pieceWorld
	bb   worldgen.Box
	dir  int // worldgen.Dir*, or -1 for no orientation
	clip worldgen.Box
	rnd  pieceRand

	needChest   bool
	selfSeed    int32
	spawnerDone bool

	// canReplace is StructurePiece.canBeReplaced (nil: always).
	canReplace func(x, y, z int) bool

	// Ocean monument piece state (OceanMonumentPiece).
	roomDef    *monumentRoomDef
	mainDesign int
	seaLevel   int
}

func st(name string) BlockState { return BlockState{Name: "minecraft:" + name} }

func (s BlockState) with(k, v string) BlockState {
	p := make(map[string]string, len(s.Properties)+1)
	for a, b := range s.Properties {
		p[a] = b
	}
	p[k] = v
	return BlockState{Name: s.Name, Properties: p}
}

func (g *pc) worldX(x, z int) int {
	switch g.dir {
	case worldgen.DirNorth, worldgen.DirSouth:
		return g.bb[0] + x
	case worldgen.DirWest:
		return g.bb[3] - z
	case worldgen.DirEast:
		return g.bb[0] + z
	}
	return x
}

func (g *pc) worldY(y int) int {
	if g.dir < 0 {
		return y
	}
	return y + g.bb[1]
}

func (g *pc) worldZ(x, z int) int {
	switch g.dir {
	case worldgen.DirNorth:
		return g.bb[5] - z
	case worldgen.DirSouth:
		return g.bb[2] + z
	case worldgen.DirWest, worldgen.DirEast:
		return g.bb[2] + x
	}
	return z
}

func (g *pc) pos(x, y, z int) [3]int { return [3]int{g.worldX(x, z), g.worldY(y), g.worldZ(x, z)} }

func (g *pc) insideW(p [3]int) bool {
	c := g.clip
	return p[0] >= c[0] && p[0] <= c[3] && p[1] >= c[1] && p[1] <= c[4] && p[2] >= c[2] && p[2] <= c[5]
}

func (g *pc) inside(x, y, z int) bool { return g.insideW(g.pos(x, y, z)) }

// orient applies the piece's mirror and rotation (setOrientation) to a state.
func (g *pc) orient(s BlockState) BlockState {
	switch g.dir {
	case worldgen.DirSouth:
		return mirrorLR(s)
	case worldgen.DirWest:
		return rotCW(mirrorLR(s))
	case worldgen.DirEast:
		return rotCW(s)
	}
	return s
}

var swapNS = map[string]string{"north": "south", "south": "north"}
var cw = map[string]string{"north": "east", "east": "south", "south": "west", "west": "north"}

func remap(s BlockState, m map[string]string, rail func(string) string) BlockState {
	if len(s.Properties) == 0 {
		return s
	}
	p := make(map[string]string, len(s.Properties))
	for k, v := range s.Properties {
		if n, ok := m[k]; ok {
			p[n] = v
			continue
		}
		if _, ok := p[k]; !ok {
			p[k] = v
		}
	}
	for k, v := range s.Properties {
		if _, ok := m[k]; ok {
			continue
		}
		p[k] = v
	}
	if f, ok := p["facing"]; ok {
		if n, ok := m[f]; ok {
			p["facing"] = n
		}
	}
	if r, ok := p["shape"]; ok && rail != nil {
		p["shape"] = rail(r)
	}
	return BlockState{Name: s.Name, Properties: p}
}

func railMap(r string, m map[string]string) string {
	parts := map[string][2]string{
		"north_south": {"north", "south"}, "east_west": {"east", "west"},
		"south_east": {"south", "east"}, "south_west": {"south", "west"},
		"north_west": {"north", "west"}, "north_east": {"north", "east"},
	}
	if len(r) > 10 && r[:10] == "ascending_" {
		return "ascending_" + m[r[10:]]
	}
	ab, ok := parts[r]
	if !ok {
		return r
	}
	a, b := m[ab[0]], m[ab[1]]
	for k, v := range parts {
		if (v[0] == a && v[1] == b) || (v[0] == b && v[1] == a) {
			return k
		}
	}
	return r
}

func mirrorLR(s BlockState) BlockState {
	ns := map[string]string{"north": "south", "south": "north", "east": "east", "west": "west"}
	return remap(s, swapNS, func(r string) string { return railMap(r, ns) })
}

func rotCW(s BlockState) BlockState {
	return remap(s, cw, func(r string) string { return railMap(r, cw) })
}

func (g *pc) setW(p [3]int, s BlockState) { g.w.m[p] = s }

// place is StructurePiece.placeBlock.
func (g *pc) place(s BlockState, x, y, z int) {
	p := g.pos(x, y, z)
	if !g.insideW(p) {
		return
	}
	if g.canReplace != nil && !g.canReplace(x, y, z) {
		return
	}
	g.setW(p, g.orient(s))
}

func (g *pc) isAir(x, y, z int) bool {
	p := g.pos(x, y, z)
	if !g.insideW(p) {
		return true // getBlock outside the box reads AIR
	}
	return g.w.kind(p[0], p[1], p[2]) == GroundAir
}

// box is generateBox with an edge and an inner state.
func (g *pc) box(x0, y0, z0, x1, y1, z1 int, edge, in BlockState, skipAir bool) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			for z := z0; z <= z1; z++ {
				if skipAir && g.isAir(x, y, z) {
					continue
				}
				if y != y0 && y != y1 && x != x0 && x != x1 && z != z0 && z != z1 {
					g.place(in, x, y, z)
				} else {
					g.place(edge, x, y, z)
				}
			}
		}
	}
}

func (g *pc) airBox(x0, y0, z0, x1, y1, z1 int) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			for z := z0; z <= z1; z++ {
				g.place(st("air"), x, y, z)
			}
		}
	}
}

// selector is StructurePiece.BlockSelector.
type selector interface {
	next(r pieceRand, x, y, z int, wall bool) BlockState
}

func (g *pc) boxSel(x0, y0, z0, x1, y1, z1 int, skipAir bool, sel selector) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			for z := z0; z <= z1; z++ {
				if skipAir && g.isAir(x, y, z) {
					continue
				}
				wall := y == y0 || y == y1 || x == x0 || x == x1 || z == z0 || z == z1
				g.place(sel.next(g.rnd, x, y, z, wall), x, y, z)
			}
		}
	}
}

// isInterior: the block above is below the OCEAN_FLOOR_WG height.
func (g *pc) isInterior(x, y, z int) bool {
	p := g.pos(x, y+1, z)
	if !g.insideW(p) {
		return false
	}
	if g.w.floor != nil {
		return p[1] < g.w.floor(p[0], p[2])
	}
	return g.w.kind(p[0], p[1], p[2]) == GroundSolid || g.w.solidAbove(p)
}

func (w *pieceWorld) solidAbove(p [3]int) bool {
	if w.ground == nil {
		return p[1] < 63
	}
	for y := p[1] + 1; y < p[1]+48; y++ {
		if w.ground(p[0], y, p[2]) == GroundSolid {
			return true
		}
	}
	return false
}

func (g *pc) maybeBox(r float32, x0, y0, z0, x1, y1, z1 int, edge, in BlockState, skipAir, interior bool) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			for z := z0; z <= z1; z++ {
				if g.rnd.nextFloat() > r {
					continue
				}
				if skipAir && g.isAir(x, y, z) {
					continue
				}
				if interior && !g.isInterior(x, y, z) {
					continue
				}
				if y != y0 && y != y1 && x != x0 && x != x1 && z != z0 && z != z1 {
					g.place(in, x, y, z)
				} else {
					g.place(edge, x, y, z)
				}
			}
		}
	}
}

func (g *pc) maybeBlock(r float32, x, y, z int, s BlockState) {
	if g.rnd.nextFloat() < r {
		g.place(s, x, y, z)
	}
}

// fillDown is fillColumnDown: it writes the state unrotated down through
// air and liquid.
func (g *pc) fillDown(s BlockState, x, y, z int) {
	p := g.pos(x, y, z)
	if !g.insideW(p) {
		return
	}
	for p[1] > g.w.minY+1 && g.w.kind(p[0], p[1], p[2]) != GroundSolid {
		g.setW(p, s)
		p[1]--
	}
}

// chest is createChest: a chest unless one is already there; the loot seed
// draws one long.
func (g *pc) chest(x, y, z int) bool {
	p := g.pos(x, y, z)
	if !g.insideW(p) || g.w.m[p].Name == "minecraft:chest" {
		return false
	}
	g.setW(p, st("chest"))
	if g.rnd != nil {
		g.rnd.nextLong()
	}
	return true
}

func (g *pc) spawner(x, y, z int) {
	if g.spawnerDone {
		return
	}
	p := g.pos(x, y, z)
	if g.insideW(p) {
		g.spawnerDone = true
		g.setW(p, st("spawner"))
	}
}

var _ = fmt.Sprint
