package structure3d

import (
	"fmt"

	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// This file follows YUNG's API 4.0 (1.20.1) YungJigsawStructure,
// JigsawManager and JigsawStructureAssembler: the start piece is a weighted
// draw from the shuffled raw pool (priority pieces first), children are
// placed breadth-first, each jigsaw tries its target pool and only then its
// fallback pool, candidates are re-drawn by weight after every failure, and
// named pieces honour max_count across the whole structure. Collision uses
// one bounding box per structure (max_distance_from_center around the start
// centre, no dimension clamp) and a per-piece box for inward jigsaws.
// Conditions, deadend pools and modifiers are reported as unsupported.

type yungEntry struct {
	p     *jPiece
	free  *shape
	depth int
}

type yungPlacer struct {
	*jigsawPlacer
	minY, maxY *int
	counts     map[string]int
	queue      []yungEntry
}

func placeYung(store *Store, def *jsDef, structureID string, seed int64, cx, cz int, free FreeHeightFunc, dim DimHeight) (*JigsawLayout, error) {
	base := &jigsawPlacer{store: store, pools: map[string]*jPool{}, missing: map[string]bool{}, templates: map[string]*jTemplate{}, free: free, maxDepth: def.Size, expansion: def.UseExpansionHack}
	if base.free == nil {
		base.free = func(int, int) int { return 64 }
	}
	pl := &yungPlacer{jigsawPlacer: base, minY: def.MinY, maxY: def.MaxY, counts: map[string]int{}}
	rnd := worldgen.LargeFeatureRandom(seed, int32(cx), int32(cz))
	pl.rnd = rnd
	// x_offset_in_chunk / z_offset_in_chunk default to ConstantInt(0), which
	// draws nothing; BetterDungeons does not set them.
	startY, err := sampleHeight(def.StartHeight, rnd, dim)
	if err != nil {
		return nil, err
	}
	locate := [3]int{cx * 16, startY, cz * 16}
	startPool, err := pl.pool(normID(def.StartPool))
	if err != nil {
		return nil, err
	}
	if startPool == nil {
		return nil, &UnsupportedError{Kind: KindEmpty, Reason: "start pool " + def.StartPool + " is missing"}
	}
	cands := shuffledCopy(startPool.raw, rnd)
	rot := int(rnd.NextInt(4))
	var start *jElement
	anchor := locate
	total := 0
	for _, c := range cands {
		total += c.w
	}
	if len(cands) > 0 && total > 0 {
		idx := pl.draw(cands, total)
		el := cands[idx].el
		if el.kind == "empty" {
			return &JigsawLayout{}, nil
		}
		if def.StartJigsawName != "" {
			js, err := pl.shuffledJigsaws(el, locate, rot)
			if err != nil {
				return nil, err
			}
			found := false
			for _, j := range js {
				if j.name == normName(def.StartJigsawName) {
					anchor, found = j.pos, true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("structure3d: %s: no start jigsaw named %s", structureID, def.StartJigsawName)
			}
		}
		start = el // conditions always pass (unsupported ones fail at parse)
	}
	if start == nil {
		return &JigsawLayout{}, nil
	}
	vec := [3]int{anchor[0] - locate[0], anchor[1] - locate[1], anchor[2] - locate[2]}
	p2 := [3]int{locate[0] - vec[0], locate[1] - vec[1], locate[2] - vec[2]}
	bb, err := pl.bbox(start, p2, rot)
	if err != nil {
		return nil, err
	}
	i, j := (bb[3]+bb[0])/2, (bb[5]+bb[2])/2
	k := p2[1]
	if def.ProjectStartToHeightmap != "" {
		k = locate[1] + pl.free(i, j)
		pl.terrain = true
	}
	m := k + vec[1]
	dy := k - (bb[1] + 1)
	bb[1] += dy
	bb[4] += dy
	p2[1] += dy
	sp := &jPiece{el: start, pos: p2, rot: rot, bb: bb, raw: bb}
	pl.pieces = append(pl.pieces, sp)
	if def.Size > 0 {
		d, _ := maxDistance(def.MaxDistanceFromCenter)
		outer := &shape{base: worldgen.Box{i - d, m - d, j - d, i + d, m + d, j + d}, cut: []worldgen.Box{bb}}
		pl.queue = append(pl.queue, yungEntry{p: sp, free: outer})
		for pl.err == nil && len(pl.queue) > 0 {
			e := pl.queue[0]
			pl.queue = pl.queue[1:]
			pl.children(e)
		}
		if pl.err != nil {
			return nil, pl.err
		}
	}
	return pl.layout(), nil
}

// draw is the weighted pick shared by getStartPiece and
// chooseCandidateFromPool: the first priority piece, else nextInt(total)+1.
func (pl *yungPlacer) draw(c []weighted, total int) int {
	for i, w := range c {
		if w.el.yung && w.el.priority {
			return i
		}
	}
	n := int(pl.rnd.NextInt(int32(total))) + 1
	for i, w := range c {
		if n -= w.w; n <= 0 {
			return i
		}
	}
	return len(c) - 1
}

func (pl *yungPlacer) children(e yungEntry) {
	piece := e.p
	js, err := pl.shuffledJigsaws(piece.el, piece.pos, piece.rot)
	if err != nil {
		pl.err = err
		return
	}
	var inner *shape
	for _, jg := range js {
		pool, err := pl.pool(normID(jg.pool))
		if err != nil {
			pl.err = err
			return
		}
		if pool == nil || (len(pool.templates) == 0 && pool.id != "minecraft:empty") {
			continue
		}
		fb, err := pl.pool(normID(pool.fallback))
		if err != nil {
			pl.err = err
			return
		}
		if fb == nil || (len(fb.templates) == 0 && fb.id != "minecraft:empty") {
			continue
		}
		free := e.free
		if inside(piece.bb, add(jg.pos, dirVec(jg.front))) {
			if inner == nil {
				inner = &shape{base: piece.bb}
			}
			free = inner
		}
		placed := false
		if e.depth != pl.maxDepth {
			placed = pl.choose(e, jg, free, pool.raw)
		}
		if pl.err == nil && !placed {
			pl.choose(e, jg, free, fb.raw)
		}
		if pl.err != nil {
			return
		}
	}
}

func (pl *yungPlacer) choose(e yungEntry, jg jJigsaw, free *shape, raw []weighted) bool {
	piece := e.p
	rigid := piece.el.rigid
	pieceMinY := piece.bb[1]
	rel := jg.pos[1] - pieceMinY
	pos3 := add(jg.pos, dirVec(jg.front))
	surface, haveSurface := 0, false
	cands := shuffledCopy(raw, pl.rnd)
	total := 0
	for _, c := range cands {
		total += c.w
	}
	drop := func(i int) {
		total -= cands[i].w
		cands = append(cands[:i], cands[i+1:]...)
	}
	for len(cands) > 0 && total > 0 {
		idx := pl.draw(cands, total)
		el := cands[idx].el
		if el.kind == "empty" {
			return false
		}
		if el.maxCount > 0 && el.name != "" && pl.counts[el.name] >= el.maxCount {
			drop(idx)
			continue
		}
		if el.yung && ((el.minDepth != nil && *el.minDepth > e.depth) || (el.maxDepth != nil && *el.maxDepth < e.depth)) {
			drop(idx)
			continue
		}
		for _, rot := range shuffledCopy([]int{0, 1, 2, 3}, pl.rnd) {
			cj, err := pl.shuffledJigsaws(el, [3]int{}, rot)
			if err != nil {
				pl.err = err
				return false
			}
			bb2, err := pl.bbox(el, [3]int{}, rot)
			if err != nil {
				pl.err = err
				return false
			}
			m := 0
			if pl.expansion && bb2[4]-bb2[1]+1 <= 16 {
				for _, c := range cj {
					if !inside(bb2, add(c.pos, dirVec(c.front))) {
						continue
					}
					a, b := 0, 0
					if cp, _ := pl.pool(normID(c.pool)); cp != nil {
						a = pl.maxSize(cp)
						if f, _ := pl.pool(normID(cp.fallback)); f != nil {
							b = pl.maxSize(f)
						}
					}
					m = max(m, a, b)
				}
			}
			for _, c := range cj {
				if !canAttach(jg, c) {
					continue
				}
				p5 := [3]int{pos3[0] - c.pos[0], pos3[1] - c.pos[1], pos3[2] - c.pos[2]}
				bb3, err := pl.bbox(el, p5, rot)
				if err != nil {
					pl.err = err
					return false
				}
				o := c.pos[1]
				need := rel - o + stepY(jg.front)
				var q int
				if rigid && el.rigid {
					q = pieceMinY + need
				} else {
					if !haveSurface {
						surface, haveSurface = pl.free(jg.pos[0], jg.pos[2]), true
						pl.terrain = true
					}
					q = surface - o
				}
				r := q - bb3[1]
				bb4 := bb3
				bb4[1] += r
				bb4[4] += r
				raw := bb4
				p6 := [3]int{p5[0], p5[1] + r, p5[2]}
				if m > 0 {
					bb4[4] = bb4[1] + max(m+1, bb4[4]-bb4[1])
				}
				if (pl.maxY != nil && bb4[4] > *pl.maxY) || (pl.minY != nil && bb4[1] < *pl.minY) {
					continue
				}
				if !(el.yung && el.ignoreBounds) && !free.contains(bb4) {
					continue
				}
				free.cut = append(free.cut, bb4)
				np := &jPiece{el: el, pos: p6, rot: rot, bb: bb4, raw: raw, depth: e.depth + 1}
				pl.pieces = append(pl.pieces, np)
				if e.depth+1 <= pl.maxDepth {
					pl.queue = append(pl.queue, yungEntry{p: np, free: free, depth: e.depth + 1})
				}
				if el.maxCount > 0 && el.name != "" {
					pl.counts[el.name]++
				}
				return true
			}
		}
		drop(idx)
	}
	return false
}
