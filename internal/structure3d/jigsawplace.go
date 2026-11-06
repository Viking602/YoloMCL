package structure3d

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// This file follows vanilla JigsawStructure.findGenerationPoint and
// JigsawPlacement (1.20.3+): the structure-start random, start height, pool
// aliases, weighted shuffled pools with fallbacks, rotation shuffles, jigsaw
// selection/placement priority, the free-space collision shape and the
// expansion hack. Terrain-matching pieces ask FreeHeight for the
// WORLD_SURFACE_WG first free Y, so they are only as exact as that function.

// FreeHeightFunc is ChunkGenerator.getFirstFreeHeight(x, z, WORLD_SURFACE_WG).
type FreeHeightFunc func(x, z int) int

// JigsawPiece is one placed pool element.
type JigsawPiece struct {
	Location  string       // template location; list elements report their first template
	Templates []string     // every template the element places (list elements have several)
	Kind      string       // single, feature, list
	Rotation  int          // vanilla Rotation ordinal: NONE, CLOCKWISE_90, CLOCKWISE_180, COUNTERCLOCKWISE_90
	Pos       [3]int       // template origin in the world
	BB        worldgen.Box // with the expansion-hack headroom
	RawBB     worldgen.Box // the element's own box
	Depth     int
	Rigid     bool
	Elements  []PieceElement // per placed template: its processors
}

// PieceElement is one single (or legacy) element a piece places.
type PieceElement struct {
	Location   string
	Processors json.RawMessage
	Legacy     bool
	Rigid      bool
}

// JigsawLayout is the result of placing one jigsaw structure start.
type JigsawLayout struct {
	Pieces []JigsawPiece
	// TerrainDependent is true when a FreeHeight answer fed the layout
	// (start projection or a terrain-matching piece).
	TerrainDependent bool
}

// DimHeight describes the dimension heights for the free-space box.
type DimHeight struct{ MinY, Height int }

var OverworldHeight = DimHeight{-64, 384}

type jsDef struct {
	Type                    string            `json:"type"`
	MaxY                    *int              `json:"max_y"`
	MinY                    *int              `json:"min_y"`
	StartPool               string            `json:"start_pool"`
	Size                    int               `json:"size"`
	StartHeight             json.RawMessage   `json:"start_height"`
	StartJigsawName         string            `json:"start_jigsaw_name"`
	ProjectStartToHeightmap string            `json:"project_start_to_heightmap"`
	MaxDistanceFromCenter   json.RawMessage   `json:"max_distance_from_center"`
	UseExpansionHack        bool              `json:"use_expansion_hack"`
	PoolAliases             []json.RawMessage `json:"pool_aliases"`
	DimensionPadding        json.RawMessage   `json:"dimension_padding"`
}

type jElement struct {
	kind     string // single, feature, list, empty
	location string
	rigid    bool
	list     []*jElement
	// YUNG's API extensions (yungsapi:*_element); zero for vanilla elements.
	yung         bool // a YungJigsawPoolElement (depth limits, priority, bounds)
	name         string
	maxCount     int // 0: no max_count
	minDepth     *int
	maxDepth     *int
	priority     bool
	ignoreBounds bool
	processors   json.RawMessage
	legacy       bool
}

type weighted struct {
	el *jElement
	w  int
}

type jPool struct {
	id        string
	templates []*jElement // weight-expanded, empties included
	raw       []weighted  // rawTemplates: (element, weight) in file order
	fallback  string
	maxSize   int // -1 until computed
}

type jJigsaw struct {
	pos          [3]int
	front, top   string // up, down, north, south, east, west
	name, target string
	pool         string
	joint        string
	placePrio    int
	selPrio      int
}

type jTemplate struct {
	size    [3]int
	jigsaws []jJigsaw // StructureTemplate order: y, x, z
}

type jigsawPlacer struct {
	store     *Store
	pools     map[string]*jPool
	missing   map[string]bool
	templates map[string]*jTemplate
	aliases   map[string]string
	free      FreeHeightFunc
	terrain   bool
	rnd       *worldgen.JavaRandom
	maxDepth  int
	expansion bool
	litho     bool // Lithostitched AlternateJigsawGenerator (breaks_seed_parity)
	pieces    []*jPiece
	queue     []queued
	seq       int
	err       error
}

type jPiece struct {
	el    *jElement
	pos   [3]int
	rot   int
	bb    worldgen.Box
	raw   worldgen.Box
	depth int
}

// shape is a box with boxes removed: VoxelShape ONLY_FIRST of AABBs.
type shape struct {
	base worldgen.Box
	cut  []worldgen.Box
}

type queued struct {
	p     *jPiece
	free  *shape
	depth int
	prio  int
	seq   int
}

// PlaceJigsaw lays out the jigsaw structure structureID whose start chunk is
// (cx, cz). free may be nil when no terrain is known (then Y uses 64).
func PlaceJigsaw(store *Store, structureID string, seed int64, cx, cz int, free FreeHeightFunc, dim DimHeight) (*JigsawLayout, error) {
	return PlaceJigsawFloor(store, structureID, seed, cx, cz, free, nil, dim)
}

// PlaceJigsawFloor is PlaceJigsaw with a separate OCEAN_FLOOR first-free
// height. getFirstFreeHeight(OCEAN_FLOOR[_WG]) skips water, so a start
// projected to it sits on the sea bed (dungeons_arise:typhon) while
// WORLD_SURFACE_WG stops at the noise sea. floor nil falls back to free.
func PlaceJigsawFloor(store *Store, structureID string, seed int64, cx, cz int, free, floor FreeHeightFunc, dim DimHeight) (*JigsawLayout, error) {
	ns, name := Namespaced(structureID)
	data, err := store.Read(DataPath(ns, "worldgen/structure", name, ".json"))
	if err != nil {
		return nil, &PreviewError{Kind: KindMissingStructure, ID: structureID, Err: err}
	}
	var def jsDef
	if err := json.Unmarshal(lenientJSON(data), &def); err != nil {
		return nil, &PreviewError{Kind: KindBadStructure, ID: structureID, Err: err}
	}
	if def.StartPool == "" {
		return nil, &UnsupportedError{Kind: KindUnsupportedPiece, Reason: "jigsaw structure missing start_pool"}
	}
	if t := normID(def.Type); t == "yungsapi:yung_jigsaw" || t == "cataclysm:cataclysm_jigsaw" {
		// Cataclysm's CataclysmJigsawManager/JigsawStructureAssembler is a
		// copy of YUNG's API assembler (same start pick and RNG order).
		return placeYung(store, &def, structureID, seed, cx, cz, free, dim)
	}
	startFree := free
	if floor != nil && strings.HasPrefix(def.ProjectStartToHeightmap, "OCEAN_FLOOR") {
		startFree = floor
	}
	pl := &jigsawPlacer{store: store, pools: map[string]*jPool{}, missing: map[string]bool{}, templates: map[string]*jTemplate{}, free: free, maxDepth: def.Size, expansion: def.UseExpansionHack, litho: store.lithoShuffle && normID(def.Type) == "minecraft:jigsaw"}
	if pl.free == nil {
		pl.free = func(int, int) int { return 64 }
	}
	rnd := worldgen.LargeFeatureRandom(seed, int32(cx), int32(cz))
	pl.rnd = rnd
	var pos [3]int
	if normID(def.Type) == "touhou_little_maid_spell:relic_sanctum" {
		// RelicSanctumStructure: start at the chunk centre, one above the
		// noise-depth surface estimate (≈ free height here), no projection.
		x, z := cx*16+8, cz*16+8
		pos = [3]int{x, pl.free(x, z) + 1, z}
		pl.terrain = true
	} else {
		startY, err := sampleHeight(def.StartHeight, rnd, dim)
		if err != nil {
			return nil, err
		}
		pos = [3]int{cx * 16, startY, cz * 16}
	}
	if pl.aliases, err = resolveAliases(def.PoolAliases, seed, pos); err != nil {
		return nil, err
	}
	// legendary_monsters:lm_jigsaw (LMJigsawPlacement) keeps the start at
	// Rotation.NONE without drawing one, and centres it on the start pos.
	lm := normID(def.Type) == "legendary_monsters:lm_jigsaw"
	rot := 0
	if !lm {
		rot = int(rnd.NextInt(4))
	}
	startPool, err := pl.pool(pl.alias(normID(def.StartPool)))
	if err != nil {
		return nil, err
	}
	if startPool == nil || len(startPool.templates) == 0 {
		return nil, &UnsupportedError{Kind: KindEmpty, Reason: "start pool " + def.StartPool + " is empty"}
	}
	start := startPool.templates[rnd.NextInt(int32(len(startPool.templates)))]
	if start.kind == "empty" {
		return &JigsawLayout{}, nil
	}
	anchor := pos
	if def.StartJigsawName != "" {
		js, err := pl.shuffledJigsaws(start, pos, rot)
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
	vec := [3]int{anchor[0] - pos[0], anchor[1] - pos[1], anchor[2] - pos[2]}
	p2 := [3]int{pos[0] - vec[0], pos[1] - vec[1], pos[2] - vec[2]}
	bb, err := pl.bbox(start, p2, rot)
	if err != nil {
		return nil, err
	}
	if lm {
		dx, dz := pos[0]-(bb[0]+bb[3])/2, pos[2]-(bb[2]+bb[5])/2
		bb[0], bb[3], bb[2], bb[5] = bb[0]+dx, bb[3]+dx, bb[2]+dz, bb[5]+dz
		p2[0], p2[2] = p2[0]+dx, p2[2]+dz
	}
	i, j := (bb[3]+bb[0])/2, (bb[5]+bb[2])/2
	k := p2[1]
	if def.ProjectStartToHeightmap != "" {
		if startFree != nil {
			k = pos[1] + startFree(i, j)
		} else {
			k = pos[1] + pl.free(i, j)
		}
		pl.terrain = true
	}
	dy := k - (bb[1] + 1) // getGroundLevelDelta() is 1 for pool elements
	bb[1] += dy
	bb[4] += dy
	p2[1] += dy
	sp := &jPiece{el: start, pos: p2, rot: rot, bb: bb, raw: bb}
	pl.pieces = append(pl.pieces, sp)
	if def.Size > 0 {
		m := k + vec[1]
		h, v := maxDistance(def.MaxDistanceFromCenter)
		pb, pt := padding(def.DimensionPadding)
		outer := worldgen.Box{i - h, max(m-v, dim.MinY+pb), j - h, i + h, min(m+v+1, dim.MinY+dim.Height-pt) - 1, j + h}
		freeOuter := &shape{base: outer, cut: []worldgen.Box{bb}}
		pl.tryChildren(sp, freeOuter, 0)
		for pl.err == nil && len(pl.queue) > 0 {
			q := pl.pop()
			pl.tryChildren(q.p, q.free, q.depth)
		}
		if pl.err != nil {
			return nil, pl.err
		}
	}
	return pl.layout(), nil
}

func (pl *jigsawPlacer) layout() *JigsawLayout {
	out := &JigsawLayout{TerrainDependent: pl.terrain}
	for _, p := range pl.pieces {
		loc := p.el.location
		if p.el.kind == "list" && len(p.el.list) > 0 {
			loc = p.el.list[0].location
		}
		var tmpls []string
		var els []PieceElement
		switch p.el.kind {
		case "single":
			tmpls = []string{p.el.location}
			els = []PieceElement{{p.el.location, p.el.processors, p.el.legacy, p.el.rigid}}
		case "list":
			for _, e := range p.el.list {
				if e.kind == "single" {
					tmpls = append(tmpls, e.location)
					els = append(els, PieceElement{e.location, e.processors, e.legacy, e.rigid})
				}
			}
		}
		out.Pieces = append(out.Pieces, JigsawPiece{Location: loc, Templates: tmpls, Kind: p.el.kind, Rotation: p.rot, Pos: p.pos, BB: p.bb, RawBB: p.raw, Depth: p.depth, Rigid: p.el.rigid, Elements: els})
	}
	return out
}

func (pl *jigsawPlacer) pop() queued {
	best := 0
	for i, q := range pl.queue {
		b := pl.queue[best]
		if q.prio > b.prio || (q.prio == b.prio && q.seq < b.seq) {
			best = i
		}
	}
	q := pl.queue[best]
	pl.queue = append(pl.queue[:best], pl.queue[best+1:]...)
	return q
}

func (pl *jigsawPlacer) alias(id string) string {
	if a, ok := pl.aliases[id]; ok {
		return a
	}
	return id
}

func (pl *jigsawPlacer) tryChildren(piece *jPiece, freeOuter *shape, depth int) {
	rigid := piece.el.rigid
	var inner *shape
	bb := piece.bb
	i := bb[1]
	js, err := pl.shuffledJigsaws(piece.el, piece.pos, piece.rot)
	if err != nil {
		pl.err = err
		return
	}
	for _, jg := range js {
		step := stepY(jg.front)
		pos3 := add(jg.pos, dirVec(jg.front))
		j := jg.pos[1] - i
		k := 0
		haveK := false
		pool, err := pl.pool(pl.alias(normID(jg.pool)))
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
		if !pl.litho && (fb == nil || (len(fb.templates) == 0 && fb.id != "minecraft:empty")) {
			continue
		}
		var free *shape
		if inside(bb, pos3) {
			if inner == nil {
				inner = &shape{base: bb}
			}
			free = inner
		} else {
			free = freeOuter
		}
		try := func(cands []*jElement) bool {
			for _, el := range cands {
				if el.kind == "empty" {
					return true
				}
				for _, rot := range shuffledCopy([]int{0, 1, 2, 3}, pl.rnd) {
					cj, err := pl.shuffledJigsaws(el, [3]int{}, rot)
					if err != nil {
						pl.err = err
						return true
					}
					bb2, err := pl.bbox(el, [3]int{}, rot)
					if err != nil {
						pl.err = err
						return true
					}
					m := 0
					if pl.expansion && bb2[4]-bb2[1]+1 <= 16 {
						for _, c := range cj {
							if !inside(bb2, add(c.pos, dirVec(c.front))) {
								continue
							}
							a, b := 0, 0
							if cp, _ := pl.pool(pl.alias(normID(c.pool))); cp != nil {
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
						p4 := c.pos
						p5 := [3]int{pos3[0] - p4[0], pos3[1] - p4[1], pos3[2] - p4[2]}
						bb3, err := pl.bbox(el, p5, rot)
						if err != nil {
							pl.err = err
							return true
						}
						n := bb3[1]
						rigid2 := el.rigid
						o := p4[1]
						p := j - o + step
						var q int
						if rigid && rigid2 {
							q = i + p
						} else {
							if !haveK {
								k, haveK = pl.free(jg.pos[0], jg.pos[2]), true
								pl.terrain = true
							}
							q = k - o
						}
						r := q - n
						bb4 := bb3
						bb4[1] += r
						bb4[4] += r
						raw := bb4
						p6 := [3]int{p5[0], p5[1] + r, p5[2]}
						if m > 0 {
							s := max(m+1, bb4[4]-bb4[1])
							bb4[4] = max(bb4[4], bb4[1]+s)
						}
						if !free.contains(bb4) {
							continue
						}
						free.cut = append(free.cut, bb4)
						np := &jPiece{el: el, pos: p6, rot: rot, bb: bb4, raw: raw, depth: depth + 1}
						pl.pieces = append(pl.pieces, np)
						if depth+1 <= pl.maxDepth {
							pl.seq++
							pl.queue = append(pl.queue, queued{p: np, free: free, depth: depth + 1, prio: jg.placePrio, seq: pl.seq})
						}
						return true
					}
				}
			}
			return false
		}
		if pl.litho {
			pl.lithoChildren(pool.id, nil, depth, true, try)
		} else {
			var cands []*jElement
			if depth != pl.maxDepth {
				cands = append(cands, shuffledCopy(pool.templates, pl.rnd)...)
			}
			cands = append(cands, shuffledCopy(fb.templates, pl.rnd)...)
			try(cands)
		}
		if pl.err != nil {
			return
		}
	}
}

// contains reports Shapes.joinIsNotEmpty(free, AABB.of(b).deflate(0.25), ONLY_SECOND) == false.
func (s *shape) contains(b worldgen.Box) bool {
	if b[0] < s.base[0] || b[1] < s.base[1] || b[2] < s.base[2] || b[3] > s.base[3] || b[4] > s.base[4] || b[5] > s.base[5] {
		return false
	}
	for _, c := range s.cut {
		if c[0] <= b[3] && c[3] >= b[0] && c[1] <= b[4] && c[4] >= b[1] && c[2] <= b[5] && c[5] >= b[2] {
			return false
		}
	}
	return true
}

func inside(b worldgen.Box, p [3]int) bool {
	return p[0] >= b[0] && p[0] <= b[3] && p[1] >= b[1] && p[1] <= b[4] && p[2] >= b[2] && p[2] <= b[5]
}

func add(a, b [3]int) [3]int { return [3]int{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }

func dirVec(d string) [3]int {
	switch d {
	case "up":
		return [3]int{0, 1, 0}
	case "down":
		return [3]int{0, -1, 0}
	case "north":
		return [3]int{0, 0, -1}
	case "south":
		return [3]int{0, 0, 1}
	case "east":
		return [3]int{1, 0, 0}
	case "west":
		return [3]int{-1, 0, 0}
	}
	return [3]int{}
}

func stepY(d string) int { return dirVec(d)[1] }

var opposite = map[string]string{"up": "down", "down": "up", "north": "south", "south": "north", "east": "west", "west": "east"}

// canAttach is JigsawBlock.canAttach(parent, child).
func canAttach(a, b jJigsaw) bool {
	joint := a.joint
	if joint != "rollable" && joint != "aligned" {
		joint = "aligned"
		if a.front != "up" && a.front != "down" {
			joint = "rollable"
		}
	}
	return a.front == opposite[b.front] && (joint == "rollable" || a.top == b.top) && a.target == b.name
}

// rotDir is Rotation.rotate(Direction) for ordinals NONE, CW90, CW180, CCW90.
func rotDir(d string, rot int) string {
	ring := []string{"north", "east", "south", "west"}
	for i, r := range ring {
		if r == d {
			return ring[(i+[]int{0, 1, 2, 3}[rot])%4]
		}
	}
	return d
}

// rotPos is StructureTemplate.transform with pivot 0 and no mirror.
func rotPos(p [3]int, rot int) [3]int {
	switch rot {
	case 1:
		return [3]int{-p[2], p[1], p[0]}
	case 2:
		return [3]int{-p[0], p[1], -p[2]}
	case 3:
		return [3]int{p[2], p[1], -p[0]}
	}
	return p
}

var rotationNames = [4]string{"NONE", "CLOCKWISE_90", "CLOCKWISE_180", "COUNTERCLOCKWISE_90"}

// RotationName is the saved name of a Rotation ordinal.
func RotationName(r int) string { return rotationNames[r&3] }

func shuffledCopy[T any](in []T, r *worldgen.JavaRandom) []T {
	out := append([]T(nil), in...)
	for i := len(out); i > 1; i-- {
		k := int(r.NextInt(int32(i)))
		out[i-1], out[k] = out[k], out[i-1]
	}
	return out
}

var featureJigsaw = jJigsaw{front: "down", top: "south", name: "minecraft:bottom", target: "minecraft:empty", pool: "minecraft:empty", joint: "rollable"}

// shuffledJigsaws is StructurePoolElement.getShuffledJigsawBlocks.
func (pl *jigsawPlacer) shuffledJigsaws(el *jElement, pos [3]int, rot int) ([]jJigsaw, error) {
	switch el.kind {
	case "feature":
		j := featureJigsaw
		j.pos = pos
		return []jJigsaw{j}, nil
	case "list":
		if len(el.list) == 0 {
			return nil, nil
		}
		return pl.shuffledJigsaws(el.list[0], pos, rot)
	case "single":
		t, err := pl.template(el.location)
		if err != nil {
			return nil, err
		}
		out := make([]jJigsaw, len(t.jigsaws))
		for i, j := range t.jigsaws {
			j.pos = add(rotPos(j.pos, rot), pos)
			j.front, j.top = rotDir(j.front, rot), rotDir(j.top, rot)
			out[i] = j
		}
		out = shuffledCopy(out, pl.rnd)
		sort.SliceStable(out, func(a, b int) bool { return out[a].selPrio > out[b].selPrio })
		return out, nil
	}
	return nil, nil
}

func (pl *jigsawPlacer) bbox(el *jElement, pos [3]int, rot int) (worldgen.Box, error) {
	switch el.kind {
	case "single":
		t, err := pl.template(el.location)
		if err != nil {
			return worldgen.Box{}, err
		}
		a := rotPos([3]int{}, rot)
		b := rotPos([3]int{t.size[0] - 1, t.size[1] - 1, t.size[2] - 1}, rot)
		return worldgen.Box{min(a[0], b[0]) + pos[0], min(a[1], b[1]) + pos[1], min(a[2], b[2]) + pos[2], max(a[0], b[0]) + pos[0], max(a[1], b[1]) + pos[1], max(a[2], b[2]) + pos[2]}, nil
	case "list":
		var out worldgen.Box
		for i, e := range el.list {
			b, err := pl.bbox(e, pos, rot)
			if err != nil {
				return out, err
			}
			if i == 0 {
				out = b
				continue
			}
			for c := 0; c < 3; c++ {
				out[c], out[c+3] = min(out[c], b[c]), max(out[c+3], b[c+3])
			}
		}
		return out, nil
	}
	return worldgen.Box{pos[0], pos[1], pos[2], pos[0], pos[1], pos[2]}, nil
}

func (pl *jigsawPlacer) maxSize(p *jPool) int {
	if p.maxSize >= 0 {
		return p.maxSize
	}
	m := 0
	for _, e := range p.templates {
		if e.kind == "empty" {
			continue
		}
		if b, err := pl.bbox(e, [3]int{}, 0); err == nil {
			m = max(m, b[4]-b[1]+1)
		}
	}
	p.maxSize = m
	return m
}

func (pl *jigsawPlacer) template(id string) (*jTemplate, error) {
	if t, ok := pl.templates[id]; ok {
		return t, nil
	}
	tt, err := LoadTemplate(pl.store, id)
	if err != nil {
		var pe *PreviewError
		if !errors.As(err, &pe) || pe.Kind != KindMissingTemplate {
			return nil, err
		}
		// StructureTemplateManager.getOrCreate hands out an empty template
		// (size 0) for a missing id; vanilla pools do reference some.
		tt = &Template{ID: id}
	}
	t := &jTemplate{size: tt.Size}
	for _, c := range tt.Jigsaw {
		front, top, _ := strings.Cut(c.Orientation, "_")
		t.jigsaws = append(t.jigsaws, jJigsaw{pos: [3]int{c.X, c.Y, c.Z}, front: front, top: top, name: normName(c.Name), target: normName(c.Target), pool: c.Pool, joint: c.Joint, placePrio: c.PlacementPriority, selPrio: c.SelectionPriority})
	}
	sort.SliceStable(t.jigsaws, func(a, b int) bool {
		x, y := t.jigsaws[a].pos, t.jigsaws[b].pos
		if x[1] != y[1] {
			return x[1] < y[1]
		}
		if x[0] != y[0] {
			return x[0] < y[0]
		}
		return x[2] < y[2]
	})
	pl.templates[id] = t
	return t, nil
}

// pool returns nil, nil for a pool the registry does not have.
func (pl *jigsawPlacer) pool(id string) (*jPool, error) {
	if p, ok := pl.pools[id]; ok {
		return p, nil
	}
	if pl.missing[id] {
		return nil, nil
	}
	if id == "minecraft:empty" {
		p := &jPool{id: id, fallback: id, maxSize: -1}
		pl.pools[id] = p
		return p, nil
	}
	ns, name := Namespaced(id)
	data, err := pl.store.Read(DataPath(ns, "worldgen/template_pool", name, ".json"))
	if err != nil {
		pl.missing[id] = true
		return nil, nil
	}
	var raw struct {
		Fallback string `json:"fallback"`
		Elements []struct {
			Weight  int             `json:"weight"`
			Element json.RawMessage `json:"element"`
		} `json:"elements"`
	}
	if err := json.Unmarshal(lenientJSON(data), &raw); err != nil {
		return nil, fmt.Errorf("structure3d: pool %s: %w", id, err)
	}
	p := &jPool{id: id, fallback: raw.Fallback, maxSize: -1}
	if p.fallback == "" {
		p.fallback = "minecraft:empty"
	}
	for _, e := range raw.Elements {
		el, err := parsePoolElement(e.Element, "")
		if err != nil {
			return nil, fmt.Errorf("structure3d: pool %s: %w", id, err)
		}
		p.raw = append(p.raw, weighted{el, e.Weight})
		for n := 0; n < e.Weight; n++ {
			p.templates = append(p.templates, el)
		}
	}
	pl.pools[id] = p
	if err := pl.applyPoolEdits(p); err != nil {
		return nil, err
	}
	return p, nil
}

func parsePoolElement(raw json.RawMessage, inherit string) (*jElement, error) {
	var e struct {
		Type       string            `json:"element_type"`
		Name       string            `json:"name"`
		MaxCount   int               `json:"max_count"`
		MinDepth   *int              `json:"min_required_depth"`
		MaxDepth   *int              `json:"max_possible_depth"`
		Priority   bool              `json:"is_priority"`
		IgnoreBnds bool              `json:"ignore_bounds"`
		Condition  json.RawMessage   `json:"condition"`
		Deadend    string            `json:"deadend_pool"`
		Modifiers  []json.RawMessage `json:"modifiers"`
		Location   json.RawMessage   `json:"location"`
		Projection string            `json:"projection"`
		Elements   []json.RawMessage `json:"elements"`
		Processors json.RawMessage   `json:"processors"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, err
	}
	proj := e.Projection
	if inherit != "" {
		proj = inherit
	}
	el := &jElement{rigid: proj != "terrain_matching"}
	typ := normID(e.Type)
	switch typ {
	case "yungsapi:yung_single_element":
		if len(e.Condition) > 0 || e.Deadend != "" || len(e.Modifiers) > 0 {
			return nil, &UnsupportedError{Kind: KindUnsupportedPiece, Reason: "yungsapi:yung_single_element condition/deadend_pool/modifiers"}
		}
		el.yung, el.minDepth, el.maxDepth, el.priority, el.ignoreBounds = true, e.MinDepth, e.MaxDepth, e.Priority, e.IgnoreBnds
		el.name, el.maxCount = e.Name, e.MaxCount
		typ = "minecraft:single_pool_element"
	case "yungsapi:max_count_single_element", "yungsapi:max_count_legacy_single_element", "yungsapi:max_count_feature_element", "yungsapi:max_count_list_element":
		el.name, el.maxCount = e.Name, e.MaxCount
		typ = "minecraft:" + strings.TrimSuffix(strings.TrimPrefix(typ, "yungsapi:max_count_"), "_element") + "_pool_element"
	}
	switch typ {
	case "minecraft:single_pool_element", "minecraft:legacy_single_pool_element":
		el.kind = "single"
		var loc string
		if json.Unmarshal(e.Location, &loc) != nil {
			return nil, &UnsupportedError{Kind: KindUnsupportedPiece, Reason: "inline template in pool element"}
		}
		el.location = normID(loc)
		el.processors = e.Processors
		el.legacy = typ == "minecraft:legacy_single_pool_element"
	case "minecraft:feature_pool_element":
		el.kind = "feature"
	case "minecraft:empty_pool_element":
		el.kind = "empty"
	case "minecraft:list_pool_element":
		el.kind = "list"
		for _, s := range e.Elements {
			c, err := parsePoolElement(s, proj)
			if err != nil {
				return nil, err
			}
			el.list = append(el.list, c)
		}
	default:
		return nil, &UnsupportedError{Kind: KindUnsupportedPiece, Reason: "unsupported pool element " + e.Type}
	}
	return el, nil
}

// normName is ResourceLocation.parse(s).toString() for jigsaw names.
func normName(s string) string {
	if s != "" && !strings.Contains(s, ":") {
		return "minecraft:" + s
	}
	return s
}

func normID(id string) string {
	if id == "" {
		return "minecraft:empty"
	}
	if !strings.Contains(id, ":") {
		return "minecraft:" + id
	}
	return id
}

// sampleHeight is HeightProvider.sample for constant and uniform providers.
func sampleHeight(raw json.RawMessage, r *worldgen.JavaRandom, dim DimHeight) (int, error) {
	if len(raw) == 0 {
		return 0, nil
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		var n int
		if json.Unmarshal(raw, &n) == nil {
			return n, nil
		}
		return 0, fmt.Errorf("structure3d: start_height %s", raw)
	}
	t := ""
	if v, ok := obj["type"]; ok {
		_ = json.Unmarshal(v, &t)
	}
	switch normID(t) {
	case "minecraft:empty": // no type: a bare vertical anchor
		return anchor(raw, dim)
	case "minecraft:constant":
		return anchor(obj["value"], dim)
	case "minecraft:uniform":
		lo, err := anchor(obj["min_inclusive"], dim)
		if err != nil {
			return 0, err
		}
		hi, err := anchor(obj["max_inclusive"], dim)
		if err != nil {
			return 0, err
		}
		if lo > hi {
			return lo, nil
		}
		return int(r.NextInt(int32(hi-lo+1))) + lo, nil
	}
	return 0, &UnsupportedError{Kind: KindUnsupportedPiece, Reason: "start_height type " + t}
}

func anchor(raw json.RawMessage, dim DimHeight) (int, error) {
	var a map[string]int
	if err := json.Unmarshal(raw, &a); err != nil {
		var n int
		if json.Unmarshal(raw, &n) == nil {
			return n, nil
		}
		return 0, fmt.Errorf("structure3d: vertical anchor %s", raw)
	}
	if v, ok := a["absolute"]; ok {
		return v, nil
	}
	if v, ok := a["above_bottom"]; ok {
		return dim.MinY + v, nil
	}
	if v, ok := a["below_top"]; ok {
		return dim.MinY + dim.Height - 1 - v, nil
	}
	return 0, fmt.Errorf("structure3d: vertical anchor %s", raw)
}

func maxDistance(raw json.RawMessage) (h, v int) {
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return n, n
	}
	var o struct {
		Horizontal int  `json:"horizontal"`
		Vertical   *int `json:"vertical"`
	}
	_ = json.Unmarshal(raw, &o)
	if o.Horizontal == 0 {
		o.Horizontal = 80
	}
	if o.Vertical == nil {
		return o.Horizontal, 4064 // MaxDistance default vertical: the codec's max height
	}
	return o.Horizontal, *o.Vertical
}

func padding(raw json.RawMessage) (bottom, top int) {
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return n, n
	}
	var o struct{ Bottom, Top int }
	_ = json.Unmarshal(raw, &o)
	return o.Bottom, o.Top
}

// mthSeed is Mth.getSeed(x, y, z).
func mthSeed(x, y, z int) int64 {
	l := int64(int32(x)*3129871) ^ int64(z)*116129781 ^ int64(y)
	l = l*l*42317861 + l*11
	return l >> 16
}

// resolveAliases is PoolAliasLookup.create(aliases, pos, seed).
func resolveAliases(list []json.RawMessage, seed int64, pos [3]int) (map[string]string, error) {
	out := map[string]string{}
	if len(list) == 0 {
		return out, nil
	}
	r := &worldgen.JavaRandom{}
	r.SetSeed(seed)
	fork := r.NextLong()
	r.SetSeed(mthSeed(pos[0], pos[1], pos[2]) ^ fork)
	var bind func(raw json.RawMessage) error
	bind = func(raw json.RawMessage) error {
		var b struct {
			Type    string `json:"type"`
			Alias   string `json:"alias"`
			Target  string `json:"target"`
			Targets []struct {
				Data   string `json:"data"`
				Weight int    `json:"weight"`
			} `json:"targets"`
			Groups []struct {
				Data   []json.RawMessage `json:"data"`
				Weight int               `json:"weight"`
			} `json:"groups"`
		}
		if err := json.Unmarshal(raw, &b); err != nil {
			return err
		}
		switch normID(b.Type) {
		case "minecraft:direct":
			out[normID(b.Alias)] = normID(b.Target)
		case "minecraft:random":
			total := 0
			for _, t := range b.Targets {
				total += t.Weight
			}
			if total > 0 {
				n := int(r.NextInt(int32(total)))
				for _, t := range b.Targets {
					if n -= t.Weight; n < 0 {
						out[normID(b.Alias)] = normID(t.Data)
						break
					}
				}
			}
		case "minecraft:random_group":
			total := 0
			for _, g := range b.Groups {
				total += g.Weight
			}
			if total > 0 {
				n := int(r.NextInt(int32(total)))
				for _, g := range b.Groups {
					if n -= g.Weight; n < 0 {
						for _, d := range g.Data {
							if err := bind(d); err != nil {
								return err
							}
						}
						break
					}
				}
			}
		default:
			return &UnsupportedError{Kind: KindUnsupportedPiece, Reason: "pool alias type " + b.Type}
		}
		return nil
	}
	for _, raw := range list {
		if err := bind(raw); err != nil {
			return nil, err
		}
	}
	return out, nil
}
