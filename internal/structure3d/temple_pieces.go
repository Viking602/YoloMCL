package structure3d

import "github.com/Viking602/YoloMCL/internal/worldgen"

// Vanilla 1.21.1 DesertPyramidPiece, JungleTemplePiece and SwampHutPiece
// postProcess, translated from the decompiled sources (j2go.py plus hand
// fixes). Local y 0 is the piece box's minY after the ground fit.

// templeState is the per-piece saved state: the fitted height, the desert
// chest flags (by 2D direction value) and the jungle trap/chest flags.
type templeState struct {
	hpos   int
	chests [4]bool
	flags  [4]bool
}

// mossSel is JungleTemplePiece.MossStoneSelector.
type mossSel struct{}

func (mossSel) next(r pieceRand, x, y, z int, wall bool) BlockState {
	if r.nextFloat() < 0.4 {
		return st("cobblestone")
	}
	return st("mossy_cobblestone")
}

// dispenser is createDispenser: the loot seed draws one long.
func (g *pc) dispenser(x, y, z int, facing string) bool {
	p := g.pos(x, y, z)
	if !g.insideW(p) || g.w.m[p].Name == "minecraft:dispenser" {
		return false
	}
	g.place(st("dispenser").with("facing", facing), x, y, z)
	g.rnd.nextLong()
	return true
}

// desertCellar is addCellar. Sand positions become sand (vanilla turns a
// few of them into suspicious sand in afterPlace), and the stair sand and
// collapsed roof use the WorldGenRegion random, so those few blocks are
// approximate: a position hash stands in for it.
func (g *pc) desertCellar() {
	sand, sst := st("sand"), st("sandstone")
	stairs := st("sandstone_stairs").with("facing", "west")
	g.place(stairs, 13, -1, 17)
	g.place(stairs, 14, -2, 17)
	g.place(stairs, 15, -3, 17)
	x, y, z := 16, -4, 13
	b := g.regionBool(x, y, z)
	for dx := -4; dx <= 0; dx++ {
		g.place(sand, x+dx, y+4, z+4)
	}
	g.place(sand, x-2, y+3, z+4)
	g.place(map[bool]BlockState{true: sand, false: sst}[b], x-1, y+3, z+4)
	g.place(map[bool]BlockState{true: sst, false: sand}[b], x, y+3, z+4)
	g.place(sand, x-1, y+2, z+4)
	g.place(sst, x, y+2, z+4)
	g.place(sand, x, y+1, z+4)
	cut, chis := st("cut_sandstone"), st("chiseled_sandstone")
	for _, r := range [][2]int{{1, 0}, {2, 1}} {
		s := cut
		if r[1] == 1 {
			s = chis
		}
		yy := y + r[0]
		g.box(x-3, yy, z-3, x-3, yy, z+2, s, s, true)
		g.box(x+3, yy, z-3, x+3, yy, z+2, s, s, true)
		g.box(x-3, yy, z-3, x+3, yy, z-2, s, s, true)
		g.box(x-3, yy, z+3, x+3, yy, z+3, s, s, true)
	}
	g.box(x-3, -1, z-3, x-3, -1, z+2, cut, cut, true)
	g.box(x+3, -1, z-3, x+3, -1, z+2, cut, cut, true)
	g.box(x-3, -1, z-3, x+3, -1, z-2, cut, cut, true)
	g.box(x-3, -1, z+3, x+3, -1, z+3, cut, cut, true)
	for yy := y + 1; yy <= y+3; yy++ {
		for xx := x - 2; xx <= x+2; xx++ {
			for zz := z - 2; zz <= z+2; zz++ {
				g.place(sand, xx, yy, zz)
			}
		}
	}
	for xx := x - 2; xx <= x+2; xx++ {
		for zz := z - 2; zz <= z+2; zz++ {
			if g.regionFloat(xx, y+4, zz) < 0.33 {
				g.place(sst, xx, y+4, zz)
			} else {
				g.place(sand, xx, y+4, zz)
			}
		}
	}
	or, blue := st("orange_terracotta"), st("blue_terracotta")
	g.place(blue, x, y, z)
	for _, d := range [][2]int{{1, -1}, {1, 1}, {-1, -1}, {-1, 1}, {2, 0}, {-2, 0}, {0, 2}, {0, -2}, {3, 0}} {
		g.place(or, x+d[0], y, z+d[1])
	}
	g.place(sand, x+3, y+1, z)
	g.place(sand, x+3, y+2, z)
	g.place(cut, x+4, y+1, z)
	g.place(chis, x+4, y+2, z)
	g.place(or, x-3, y, z)
	g.place(sand, x-3, y+1, z)
	g.place(sand, x-3, y+2, z)
	g.place(cut, x-4, y+1, z)
	g.place(chis, x-4, y+2, z)
	g.place(or, x, y, z+3)
	g.place(sand, x, y+1, z+3)
	g.place(sand, x, y+2, z+3)
	g.place(or, x, y, z-3)
	g.place(sand, x, y+1, z-3)
	g.place(sand, x, y+2, z-3)
	g.place(cut, x, y+1, z-4)
	g.place(chis, x, -2, z-4)
}

func (g *pc) regionFloat(x, y, z int) float32 {
	p := g.pos(x, y, z)
	h := uint32(p[0]*73856093) ^ uint32(p[1]*19349663) ^ uint32(p[2]*83492791)
	h ^= h >> 13
	h *= 0x5bd1e995
	h ^= h >> 15
	return float32(h&0xffffff) / (1 << 24)
}

func (g *pc) regionBool(x, y, z int) bool { return g.regionFloat(x, y, z) < 0.5 }

// templeFit is updateAverageGroundHeight (jungle temple, swamp hut) or
// updateHeightPositionToLowestGroundHeight (desert pyramid, offset
// -nextInt(3)) over the part of the footprint in the current chunk, using
// the MOTION_BLOCKING_NO_LEAVES height. It moves the box once.
func templeFit(id string, t *templeState, g *pc, surface func(x, z int) int) bool {
	if t.hpos >= 0 {
		return true
	}
	if id == "minecraft:desert_pyramid" {
		lo := 1 << 30
		for z := g.bb[2]; z <= g.bb[5]; z++ {
			for x := g.bb[0]; x <= g.bb[3]; x++ {
				lo = min(lo, surface(x, z))
			}
		}
		t.hpos = lo
		g.bb = moveY(g.bb, lo-g.bb[1]-g.rnd.nextInt(3))
		return true
	}
	sum, n := 0, 0
	for z := max(g.bb[2], g.clip[2]); z <= min(g.bb[5], g.clip[5]); z++ {
		for x := max(g.bb[0], g.clip[0]); x <= min(g.bb[3], g.clip[3]); x++ {
			sum += surface(x, z)
			n++
		}
	}
	if n == 0 {
		return false
	}
	t.hpos = sum / n
	g.bb = moveY(g.bb, t.hpos-g.bb[1])
	return true
}

func templePost(id string, t *templeState, g *pc) {
	switch id {
	case "minecraft:desert_pyramid":
		g.postDesertPyramid(t)
	case "minecraft:jungle_pyramid":
		g.postJungleTemple(t)
	case "minecraft:swamp_hut":
		g.postSwampHut(t)
	}
}

// templeIndex is each temple's place in 1.21.1's alphabetical
// SURFACE_STRUCTURES list (the per-chunk decoration feature index).
var templeIndex = map[string]int{"minecraft:desert_pyramid": 1, "minecraft:jungle_pyramid": 4, "minecraft:swamp_hut": 20}

// assembleTemple runs the vanilla temple postProcess at the seed-exact
// footprint and orientation. The ground fit uses the approximate surface
// height, so the Y and anything touching terrain are approximate.
func assembleTemple(id string, index int, seed int64, x, z int, height HeightFunc, ground Ground) *Scene {
	cx, cz := int32(x>>4), int32(z>>4)
	o := worldgen.ScatteredOrientation(seed, cx, cz)
	box := worldgen.ScatteredBox(cx, cz, 64, worldgen.ScatteredSize[id], o)
	surface := height
	if surface == nil {
		surface = func(int, int) int { return 64 }
	}
	if ground == nil {
		ground = func(x, y, z int) GroundKind {
			if y < surface(x, z) {
				return GroundSolid
			}
			return GroundAir
		}
	}
	w := newPieceWorld(ground, -64, -64)
	w.maxY = 320
	g := &pc{w: w, bb: box, dir: orientDir[o]}
	t := &templeState{hpos: -1}
	rnd := decoRand(seed, index, worldgen.StepSurfaceStructures)
	runPieces(w, []worldgen.Box{box}, []*pc{g}, rnd, func(_ int, g *pc) {
		if templeFit(id, t, g, surface) {
			templePost(id, t, g)
		}
	})
	s := BuildScene(w.blocks())
	s.LayoutSeeded = true
	s.Carved = w.carved()
	s.Pieces = []PieceBox{{Min: [3]int{g.bb[0], g.bb[1], g.bb[2]}, Max: [3]int{g.bb[3], g.bb[4], g.bb[5]}, Name: id}}
	return s
}

func (g *pc) postDesertPyramid(t *templeState) {
	{
		g.box(0, -4, 0, 21-1, 0, 21-1, st("sandstone"), st("sandstone"), false)

		var var8 int
		for var8 = 1; var8 <= 9; var8++ {
			g.box(var8, var8, var8, 21-1-var8, var8, 21-1-var8, st("sandstone"), st("sandstone"), false)
			g.box(var8+1, var8, var8+1, 21-2-var8, var8, 21-2-var8, st("air"), st("air"), false)
		}

		for var8 = 0; var8 < 21; var8++ {
			for var9 := 0; var9 < 21; var9++ {
				g.fillDown(st("sandstone"), var8, -5, var9)
			}
		}

		var16 := st("sandstone_stairs").with("facing", "north")
		var17 := st("sandstone_stairs").with("facing", "south")
		var18 := st("sandstone_stairs").with("facing", "east")
		var11 := st("sandstone_stairs").with("facing", "west")
		g.box(0, 0, 0, 4, 9, 4, st("sandstone"), st("air"), false)
		g.box(1, 10, 1, 3, 10, 3, st("sandstone"), st("sandstone"), false)
		g.place(var16, 2, 10, 0)
		g.place(var17, 2, 10, 4)
		g.place(var18, 0, 10, 2)
		g.place(var11, 4, 10, 2)
		g.box(21-5, 0, 0, 21-1, 9, 4, st("sandstone"), st("air"), false)
		g.box(21-4, 10, 1, 21-2, 10, 3, st("sandstone"), st("sandstone"), false)
		g.place(var16, 21-3, 10, 0)
		g.place(var17, 21-3, 10, 4)
		g.place(var18, 21-5, 10, 2)
		g.place(var11, 21-1, 10, 2)
		g.box(8, 0, 0, 12, 4, 4, st("sandstone"), st("air"), false)
		g.box(9, 1, 0, 11, 3, 4, st("air"), st("air"), false)
		g.place(st("cut_sandstone"), 9, 1, 1)
		g.place(st("cut_sandstone"), 9, 2, 1)
		g.place(st("cut_sandstone"), 9, 3, 1)
		g.place(st("cut_sandstone"), 10, 3, 1)
		g.place(st("cut_sandstone"), 11, 3, 1)
		g.place(st("cut_sandstone"), 11, 2, 1)
		g.place(st("cut_sandstone"), 11, 1, 1)
		g.box(4, 1, 1, 8, 3, 3, st("sandstone"), st("air"), false)
		g.box(4, 1, 2, 8, 2, 2, st("air"), st("air"), false)
		g.box(12, 1, 1, 16, 3, 3, st("sandstone"), st("air"), false)
		g.box(12, 1, 2, 16, 2, 2, st("air"), st("air"), false)
		g.box(5, 4, 5, 21-6, 4, 21-6, st("sandstone"), st("sandstone"), false)
		g.box(9, 4, 9, 11, 4, 11, st("air"), st("air"), false)
		g.box(8, 1, 8, 8, 3, 8, st("cut_sandstone"), st("cut_sandstone"), false)
		g.box(12, 1, 8, 12, 3, 8, st("cut_sandstone"), st("cut_sandstone"), false)
		g.box(8, 1, 12, 8, 3, 12, st("cut_sandstone"), st("cut_sandstone"), false)
		g.box(12, 1, 12, 12, 3, 12, st("cut_sandstone"), st("cut_sandstone"), false)
		g.box(1, 1, 5, 4, 4, 11, st("sandstone"), st("sandstone"), false)
		g.box(21-5, 1, 5, 21-2, 4, 11, st("sandstone"), st("sandstone"), false)
		g.box(6, 7, 9, 6, 7, 11, st("sandstone"), st("sandstone"), false)
		g.box(21-7, 7, 9, 21-7, 7, 11, st("sandstone"), st("sandstone"), false)
		g.box(5, 5, 9, 5, 7, 11, st("cut_sandstone"), st("cut_sandstone"), false)
		g.box(21-6, 5, 9, 21-6, 7, 11, st("cut_sandstone"), st("cut_sandstone"), false)
		g.place(st("air"), 5, 5, 10)
		g.place(st("air"), 5, 6, 10)
		g.place(st("air"), 6, 6, 10)
		g.place(st("air"), 21-6, 5, 10)
		g.place(st("air"), 21-6, 6, 10)
		g.place(st("air"), 21-7, 6, 10)
		g.box(2, 4, 4, 2, 6, 4, st("air"), st("air"), false)
		g.box(21-3, 4, 4, 21-3, 6, 4, st("air"), st("air"), false)
		g.place(var16, 2, 4, 5)
		g.place(var16, 2, 3, 4)
		g.place(var16, 21-3, 4, 5)
		g.place(var16, 21-3, 3, 4)
		g.box(1, 1, 3, 2, 2, 3, st("sandstone"), st("sandstone"), false)
		g.box(21-3, 1, 3, 21-2, 2, 3, st("sandstone"), st("sandstone"), false)
		g.place(st("sandstone"), 1, 1, 2)
		g.place(st("sandstone"), 21-2, 1, 2)
		g.place(st("sandstone_slab"), 1, 2, 2)
		g.place(st("sandstone_slab"), 21-2, 2, 2)
		g.place(var11, 2, 1, 2)
		g.place(var18, 21-3, 1, 2)
		g.box(4, 3, 5, 4, 3, 17, st("sandstone"), st("sandstone"), false)
		g.box(21-5, 3, 5, 21-5, 3, 17, st("sandstone"), st("sandstone"), false)
		g.box(3, 1, 5, 4, 2, 16, st("air"), st("air"), false)
		g.box(21-6, 1, 5, 21-5, 2, 16, st("air"), st("air"), false)

		var var12 int
		for var12 = 5; var12 <= 17; var12 += 2 {
			g.place(st("cut_sandstone"), 4, 1, var12)
			g.place(st("chiseled_sandstone"), 4, 2, var12)
			g.place(st("cut_sandstone"), 21-5, 1, var12)
			g.place(st("chiseled_sandstone"), 21-5, 2, var12)
		}

		g.place(st("orange_terracotta"), 10, 0, 7)
		g.place(st("orange_terracotta"), 10, 0, 8)
		g.place(st("orange_terracotta"), 9, 0, 9)
		g.place(st("orange_terracotta"), 11, 0, 9)
		g.place(st("orange_terracotta"), 8, 0, 10)
		g.place(st("orange_terracotta"), 12, 0, 10)
		g.place(st("orange_terracotta"), 7, 0, 10)
		g.place(st("orange_terracotta"), 13, 0, 10)
		g.place(st("orange_terracotta"), 9, 0, 11)
		g.place(st("orange_terracotta"), 11, 0, 11)
		g.place(st("orange_terracotta"), 10, 0, 12)
		g.place(st("orange_terracotta"), 10, 0, 13)
		g.place(st("blue_terracotta"), 10, 0, 10)

		for var12 := 0; var12 <= 21-1; var12 += 21 - 1 {
			g.place(st("cut_sandstone"), var12, 2, 1)
			g.place(st("orange_terracotta"), var12, 2, 2)
			g.place(st("cut_sandstone"), var12, 2, 3)
			g.place(st("cut_sandstone"), var12, 3, 1)
			g.place(st("orange_terracotta"), var12, 3, 2)
			g.place(st("cut_sandstone"), var12, 3, 3)
			g.place(st("orange_terracotta"), var12, 4, 1)
			g.place(st("chiseled_sandstone"), var12, 4, 2)
			g.place(st("orange_terracotta"), var12, 4, 3)
			g.place(st("cut_sandstone"), var12, 5, 1)
			g.place(st("orange_terracotta"), var12, 5, 2)
			g.place(st("cut_sandstone"), var12, 5, 3)
			g.place(st("orange_terracotta"), var12, 6, 1)
			g.place(st("chiseled_sandstone"), var12, 6, 2)
			g.place(st("orange_terracotta"), var12, 6, 3)
			g.place(st("orange_terracotta"), var12, 7, 1)
			g.place(st("orange_terracotta"), var12, 7, 2)
			g.place(st("orange_terracotta"), var12, 7, 3)
			g.place(st("cut_sandstone"), var12, 8, 1)
			g.place(st("cut_sandstone"), var12, 8, 2)
			g.place(st("cut_sandstone"), var12, 8, 3)
		}

		for var12 := 2; var12 <= 21-3; var12 += 21 - 3 - 2 {
			g.place(st("cut_sandstone"), var12-1, 2, 0)
			g.place(st("orange_terracotta"), var12, 2, 0)
			g.place(st("cut_sandstone"), var12+1, 2, 0)
			g.place(st("cut_sandstone"), var12-1, 3, 0)
			g.place(st("orange_terracotta"), var12, 3, 0)
			g.place(st("cut_sandstone"), var12+1, 3, 0)
			g.place(st("orange_terracotta"), var12-1, 4, 0)
			g.place(st("chiseled_sandstone"), var12, 4, 0)
			g.place(st("orange_terracotta"), var12+1, 4, 0)
			g.place(st("cut_sandstone"), var12-1, 5, 0)
			g.place(st("orange_terracotta"), var12, 5, 0)
			g.place(st("cut_sandstone"), var12+1, 5, 0)
			g.place(st("orange_terracotta"), var12-1, 6, 0)
			g.place(st("chiseled_sandstone"), var12, 6, 0)
			g.place(st("orange_terracotta"), var12+1, 6, 0)
			g.place(st("orange_terracotta"), var12-1, 7, 0)
			g.place(st("orange_terracotta"), var12, 7, 0)
			g.place(st("orange_terracotta"), var12+1, 7, 0)
			g.place(st("cut_sandstone"), var12-1, 8, 0)
			g.place(st("cut_sandstone"), var12, 8, 0)
			g.place(st("cut_sandstone"), var12+1, 8, 0)
		}

		g.box(8, 4, 0, 12, 6, 0, st("cut_sandstone"), st("cut_sandstone"), false)
		g.place(st("air"), 8, 6, 0)
		g.place(st("air"), 12, 6, 0)
		g.place(st("orange_terracotta"), 9, 5, 0)
		g.place(st("chiseled_sandstone"), 10, 5, 0)
		g.place(st("orange_terracotta"), 11, 5, 0)
		g.box(8, -14, 8, 12, -11, 12, st("cut_sandstone"), st("cut_sandstone"), false)
		g.box(8, -10, 8, 12, -10, 12, st("chiseled_sandstone"), st("chiseled_sandstone"), false)
		g.box(8, -9, 8, 12, -9, 12, st("cut_sandstone"), st("cut_sandstone"), false)
		g.box(8, -8, 8, 12, -1, 12, st("sandstone"), st("sandstone"), false)
		g.box(9, -11, 9, 11, -1, 11, st("air"), st("air"), false)
		g.place(st("stone_pressure_plate"), 10, -11, 10)
		g.box(9, -13, 9, 11, -13, 11, st("tnt"), st("air"), false)
		g.place(st("air"), 8, -11, 10)
		g.place(st("air"), 8, -10, 10)
		g.place(st("chiseled_sandstone"), 7, -10, 10)
		g.place(st("cut_sandstone"), 7, -11, 10)
		g.place(st("air"), 12, -11, 10)
		g.place(st("air"), 12, -10, 10)
		g.place(st("chiseled_sandstone"), 13, -10, 10)
		g.place(st("cut_sandstone"), 13, -11, 10)
		g.place(st("air"), 10, -11, 8)
		g.place(st("air"), 10, -10, 8)
		g.place(st("chiseled_sandstone"), 10, -10, 7)
		g.place(st("cut_sandstone"), 10, -11, 7)
		g.place(st("air"), 10, -11, 12)
		g.place(st("air"), 10, -10, 12)
		g.place(st("chiseled_sandstone"), 10, -10, 13)
		g.place(st("cut_sandstone"), 10, -11, 13)
		for _, dd := range [4][3]int{{2, 0, -1}, {3, 1, 0}, {0, 0, 1}, {1, -1, 0}} { // N E S W: 2D value, stepX, stepZ
			if !t.chests[dd[0]] {
				t.chests[dd[0]] = g.chest(10+dd[1]*2, -11, 10+dd[2]*2)
			}
		}
		g.desertCellar()
	}
}

func (g *pc) postJungleTemple(t *templeState) {
	{
		g.boxSel(0, -4, 0, 12-1, 0, 15-1, false, mossSel{})
		g.boxSel(2, 1, 2, 9, 2, 2, false, mossSel{})
		g.boxSel(2, 1, 12, 9, 2, 12, false, mossSel{})
		g.boxSel(2, 1, 3, 2, 2, 11, false, mossSel{})
		g.boxSel(9, 1, 3, 9, 2, 11, false, mossSel{})
		g.boxSel(1, 3, 1, 10, 6, 1, false, mossSel{})
		g.boxSel(1, 3, 13, 10, 6, 13, false, mossSel{})
		g.boxSel(1, 3, 2, 1, 6, 12, false, mossSel{})
		g.boxSel(10, 3, 2, 10, 6, 12, false, mossSel{})
		g.boxSel(2, 3, 2, 9, 3, 12, false, mossSel{})
		g.boxSel(2, 6, 2, 9, 6, 12, false, mossSel{})
		g.boxSel(3, 7, 3, 8, 7, 11, false, mossSel{})
		g.boxSel(4, 8, 4, 7, 8, 10, false, mossSel{})
		g.airBox(3, 1, 3, 8, 2, 11)
		g.airBox(4, 3, 6, 7, 3, 9)
		g.airBox(2, 4, 2, 9, 5, 12)
		g.airBox(4, 6, 5, 7, 6, 9)
		g.airBox(5, 7, 6, 6, 7, 8)
		g.airBox(5, 1, 2, 6, 2, 2)
		g.airBox(5, 2, 12, 6, 2, 12)
		g.airBox(5, 5, 1, 6, 5, 1)
		g.airBox(5, 5, 13, 6, 5, 13)
		g.place(st("air"), 1, 5, 5)
		g.place(st("air"), 10, 5, 5)
		g.place(st("air"), 1, 5, 9)
		g.place(st("air"), 10, 5, 9)

		var var8 int
		for var8 = 0; var8 <= 14; var8 += 14 {
			g.boxSel(2, 4, var8, 2, 5, var8, false, mossSel{})
			g.boxSel(4, 4, var8, 4, 5, var8, false, mossSel{})
			g.boxSel(7, 4, var8, 7, 5, var8, false, mossSel{})
			g.boxSel(9, 4, var8, 9, 5, var8, false, mossSel{})
		}

		g.boxSel(5, 6, 0, 6, 6, 0, false, mossSel{})

		for var8 = 0; var8 <= 11; var8 += 11 {
			for var9 := 2; var9 <= 12; var9 += 2 {
				g.boxSel(var8, 4, var9, var8, 5, var9, false, mossSel{})
			}

			g.boxSel(var8, 6, 5, var8, 6, 5, false, mossSel{})
			g.boxSel(var8, 6, 9, var8, 6, 9, false, mossSel{})
		}

		g.boxSel(2, 7, 2, 2, 9, 2, false, mossSel{})
		g.boxSel(9, 7, 2, 9, 9, 2, false, mossSel{})
		g.boxSel(2, 7, 12, 2, 9, 12, false, mossSel{})
		g.boxSel(9, 7, 12, 9, 9, 12, false, mossSel{})
		g.boxSel(4, 9, 4, 4, 9, 4, false, mossSel{})
		g.boxSel(7, 9, 4, 7, 9, 4, false, mossSel{})
		g.boxSel(4, 9, 10, 4, 9, 10, false, mossSel{})
		g.boxSel(7, 9, 10, 7, 9, 10, false, mossSel{})
		g.boxSel(5, 9, 7, 6, 9, 7, false, mossSel{})
		var14 := st("cobblestone_stairs").with("facing", "east")
		var15 := st("cobblestone_stairs").with("facing", "west")
		var10 := st("cobblestone_stairs").with("facing", "south")
		var11 := st("cobblestone_stairs").with("facing", "north")
		g.place(var11, 5, 9, 6)
		g.place(var11, 6, 9, 6)
		g.place(var10, 5, 9, 8)
		g.place(var10, 6, 9, 8)
		g.place(var11, 4, 0, 0)
		g.place(var11, 5, 0, 0)
		g.place(var11, 6, 0, 0)
		g.place(var11, 7, 0, 0)
		g.place(var11, 4, 1, 8)
		g.place(var11, 4, 2, 9)
		g.place(var11, 4, 3, 10)
		g.place(var11, 7, 1, 8)
		g.place(var11, 7, 2, 9)
		g.place(var11, 7, 3, 10)
		g.boxSel(4, 1, 9, 4, 1, 9, false, mossSel{})
		g.boxSel(7, 1, 9, 7, 1, 9, false, mossSel{})
		g.boxSel(4, 1, 10, 7, 2, 10, false, mossSel{})
		g.boxSel(5, 4, 5, 6, 4, 5, false, mossSel{})
		g.place(var14, 4, 4, 5)
		g.place(var15, 7, 4, 5)

		var var12 int
		for var12 = 0; var12 < 4; var12++ {
			g.place(var10, 5, 0-var12, 6+var12)
			g.place(var10, 6, 0-var12, 6+var12)
			g.airBox(5, 0-var12, 7+var12, 6, 0-var12, 9+var12)
		}

		g.airBox(1, -3, 12, 10, -1, 13)
		g.airBox(1, -3, 1, 3, -1, 13)
		g.airBox(1, -3, 1, 9, -1, 5)

		for var12 = 1; var12 <= 13; var12 += 2 {
			g.boxSel(1, -3, var12, 1, -2, var12, false, mossSel{})
		}

		for var12 = 2; var12 <= 12; var12 += 2 {
			g.boxSel(1, -1, var12, 3, -1, var12, false, mossSel{})
		}

		g.boxSel(2, -2, 1, 5, -2, 1, false, mossSel{})
		g.boxSel(7, -2, 1, 9, -2, 1, false, mossSel{})
		g.boxSel(6, -3, 1, 6, -3, 1, false, mossSel{})
		g.boxSel(6, -1, 1, 6, -1, 1, false, mossSel{})
		g.place((st("tripwire_hook").with("facing", "east")).with("attached", "true"), 1, -3, 8)
		g.place((st("tripwire_hook").with("facing", "west")).with("attached", "true"), 4, -3, 8)
		g.place(((st("tripwire").with("east", "true")).with("west", "true")).with("attached", "true"), 2, -3, 8)
		g.place(((st("tripwire").with("east", "true")).with("west", "true")).with("attached", "true"), 3, -3, 8)
		var16 := (st("redstone_wire").with("north", "side")).with("south", "side")
		g.place(var16, 5, -3, 7)
		g.place(var16, 5, -3, 6)
		g.place(var16, 5, -3, 5)
		g.place(var16, 5, -3, 4)
		g.place(var16, 5, -3, 3)
		g.place(var16, 5, -3, 2)
		g.place((st("redstone_wire").with("north", "side")).with("west", "side"), 5, -3, 1)
		g.place((st("redstone_wire").with("east", "side")).with("west", "side"), 4, -3, 1)
		g.place(st("mossy_cobblestone"), 3, -3, 1)
		if !t.flags[0] {
			t.flags[0] = g.dispenser(3, -2, 1, "north")
		}

		g.place(st("vine").with("south", "true"), 3, -2, 2)
		g.place((st("tripwire_hook").with("facing", "north")).with("attached", "true"), 7, -3, 1)
		g.place((st("tripwire_hook").with("facing", "south")).with("attached", "true"), 7, -3, 5)
		g.place(((st("tripwire").with("north", "true")).with("south", "true")).with("attached", "true"), 7, -3, 2)
		g.place(((st("tripwire").with("north", "true")).with("south", "true")).with("attached", "true"), 7, -3, 3)
		g.place(((st("tripwire").with("north", "true")).with("south", "true")).with("attached", "true"), 7, -3, 4)
		g.place((st("redstone_wire").with("east", "side")).with("west", "side"), 8, -3, 6)
		g.place((st("redstone_wire").with("west", "side")).with("south", "side"), 9, -3, 6)
		g.place((st("redstone_wire").with("north", "side")).with("south", "up"), 9, -3, 5)
		g.place(st("mossy_cobblestone"), 9, -3, 4)
		g.place(var16, 9, -2, 4)
		if !t.flags[1] {
			t.flags[1] = g.dispenser(9, -2, 3, "west")
		}

		g.place(st("vine").with("east", "true"), 8, -1, 3)
		g.place(st("vine").with("east", "true"), 8, -2, 3)
		if !t.flags[2] {
			t.flags[2] = g.chest(8, -3, 3)
		}

		g.place(st("mossy_cobblestone"), 9, -3, 2)
		g.place(st("mossy_cobblestone"), 8, -3, 1)
		g.place(st("mossy_cobblestone"), 4, -3, 5)
		g.place(st("mossy_cobblestone"), 5, -2, 5)
		g.place(st("mossy_cobblestone"), 5, -1, 5)
		g.place(st("mossy_cobblestone"), 6, -3, 5)
		g.place(st("mossy_cobblestone"), 7, -2, 5)
		g.place(st("mossy_cobblestone"), 7, -1, 5)
		g.place(st("mossy_cobblestone"), 8, -3, 5)
		g.boxSel(9, -1, 1, 9, -1, 5, false, mossSel{})
		g.airBox(8, -3, 8, 10, -1, 10)
		g.place(st("chiseled_stone_bricks"), 8, -2, 11)
		g.place(st("chiseled_stone_bricks"), 9, -2, 11)
		g.place(st("chiseled_stone_bricks"), 10, -2, 11)
		var13 := (st("lever").with("facing", "north")).with("face", "wall")
		g.place(var13, 8, -2, 12)
		g.place(var13, 9, -2, 12)
		g.place(var13, 10, -2, 12)
		g.boxSel(8, -3, 8, 8, -3, 10, false, mossSel{})
		g.boxSel(10, -3, 8, 10, -3, 10, false, mossSel{})
		g.place(st("mossy_cobblestone"), 10, -2, 9)
		g.place(var16, 8, -2, 9)
		g.place(var16, 8, -2, 10)
		g.place((((st("redstone_wire").with("north", "side")).with("south", "side")).with("east", "side")).with("west", "side"), 10, -1, 9)
		g.place(st("sticky_piston").with("facing", "up"), 9, -2, 8)
		g.place(st("sticky_piston").with("facing", "west"), 10, -2, 8)
		g.place(st("sticky_piston").with("facing", "west"), 10, -1, 8)
		g.place(st("repeater").with("facing", "north"), 10, -2, 10)
		if !t.flags[3] {
			t.flags[3] = g.chest(9, -3, 10)
		}

	}

}
func (g *pc) postSwampHut(t *templeState) {
	{
		g.box(1, 1, 1, 5, 1, 7, st("spruce_planks"), st("spruce_planks"), false)
		g.box(1, 4, 2, 5, 4, 7, st("spruce_planks"), st("spruce_planks"), false)
		g.box(2, 1, 0, 4, 1, 0, st("spruce_planks"), st("spruce_planks"), false)
		g.box(2, 2, 2, 3, 3, 2, st("spruce_planks"), st("spruce_planks"), false)
		g.box(1, 2, 3, 1, 3, 6, st("spruce_planks"), st("spruce_planks"), false)
		g.box(5, 2, 3, 5, 3, 6, st("spruce_planks"), st("spruce_planks"), false)
		g.box(2, 2, 7, 4, 3, 7, st("spruce_planks"), st("spruce_planks"), false)
		g.box(1, 0, 2, 1, 3, 2, st("oak_log"), st("oak_log"), false)
		g.box(5, 0, 2, 5, 3, 2, st("oak_log"), st("oak_log"), false)
		g.box(1, 0, 7, 1, 3, 7, st("oak_log"), st("oak_log"), false)
		g.box(5, 0, 7, 5, 3, 7, st("oak_log"), st("oak_log"), false)
		g.place(st("oak_fence"), 2, 3, 2)
		g.place(st("oak_fence"), 3, 3, 7)
		g.place(st("air"), 1, 3, 4)
		g.place(st("air"), 5, 3, 4)
		g.place(st("air"), 5, 3, 5)
		g.place(st("potted_red_mushroom"), 1, 3, 5)
		g.place(st("crafting_table"), 3, 2, 6)
		g.place(st("cauldron"), 4, 2, 6)
		g.place(st("oak_fence"), 1, 2, 1)
		g.place(st("oak_fence"), 5, 2, 1)
		var8 := st("spruce_stairs").with("facing", "north")
		var9 := st("spruce_stairs").with("facing", "east")
		var10 := st("spruce_stairs").with("facing", "west")
		var11 := st("spruce_stairs").with("facing", "south")
		g.box(0, 4, 1, 6, 4, 1, var8, var8, false)
		g.box(0, 4, 2, 0, 4, 7, var9, var9, false)
		g.box(6, 4, 2, 6, 4, 7, var10, var10, false)
		g.box(0, 4, 8, 6, 4, 8, var11, var11, false)
		g.place(var8.with("shape", "outer_right"), 0, 4, 1)
		g.place(var8.with("shape", "outer_left"), 6, 4, 1)
		g.place(var11.with("shape", "outer_left"), 0, 4, 8)
		g.place(var11.with("shape", "outer_right"), 6, 4, 8)

		for var12 := 2; var12 <= 7; var12 += 5 {
			for var13 := 1; var13 <= 5; var13 += 4 {
				g.fillDown(st("oak_log"), var13, -1, var12)
			}
		}

	}
}

func moveY(b worldgen.Box, dy int) worldgen.Box {
	b[1] += dy
	b[4] += dy
	return b
}
