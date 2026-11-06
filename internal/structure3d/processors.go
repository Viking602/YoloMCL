package structure3d

import (
	"encoding/json"
	"strings"

	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// Structure processors (1.20.1 templatesystem), ported from the srg jar.
// Each runs on the unrotated template state at its final world position;
// the caller rotates the result. A nil result drops the block.

// procCtx is what a processor may read.
type procCtx struct {
	store    *Store
	pivot    [3]int
	height   HeightFunc                      // surface Y; WORLD_SURFACE_WG height is height+1
	existing func([3]int) (BlockState, bool) // blocks placed so far
	tags     map[string]map[string]bool
	approx   bool // a ≈ processor ran (unknown terrain, custom code, loc predicates)
	gravity  bool
	sea      int                      // sea level, 0 = none
	place    func([3]int, BlockState) // direct chunk writes (YUNG legs)
}

type procBlock struct {
	local [3]int // template-local position (original)
	world [3]int
	state BlockState
	nbt   map[string]any
}

type processor interface {
	process(c *procCtx, b procBlock) *procBlock
}

func procRandom(p [3]int) *worldgen.JavaRandom {
	r := &worldgen.JavaRandom{}
	r.SetSeed(mthSeed(p[0], p[1], p[2]))
	return r
}

// loadProcessors resolves a pool element's "processors" field: a
// processor_list id, an inline {"processors":[...]} or a bare list.
func loadProcessors(store *Store, raw json.RawMessage) ([]processor, bool, error) {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false, nil
	}
	var id string
	if json.Unmarshal(raw, &id) == nil {
		ns, name := Namespaced(normID(id))
		if ns == "minecraft" && name == "empty" {
			return nil, false, nil
		}
		data, err := store.Read(DataPath(ns, "worldgen/processor_list", name, ".json"))
		if err != nil {
			return nil, true, nil // unknown list: place unprocessed (≈)
		}
		raw = lenientJSON(data)
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		var o struct {
			Processors []json.RawMessage `json:"processors"`
		}
		if err := json.Unmarshal(raw, &o); err != nil {
			return nil, true, nil
		}
		list = o.Processors
	}
	var out []processor
	approx := false
	for _, p := range list {
		pr, ok := parseProcessor(p)
		if !ok {
			approx = true
			continue
		}
		if pr != nil {
			out = append(out, pr)
		}
	}
	return out, approx, nil
}

func parseProcessor(raw json.RawMessage) (processor, bool) {
	var h struct {
		Type      string            `json:"processor_type"`
		Rules     []json.RawMessage `json:"rules"`
		Integrity *float32          `json:"integrity"`
		Rottable  json.RawMessage   `json:"rottable_blocks"`
		Mossiness float32           `json:"mossiness"`
		Heightmap string            `json:"heightmap"`
		Offset    int               `json:"offset"`
		Blocks    []json.RawMessage `json:"blocks"`
	}
	if json.Unmarshal(raw, &h) != nil {
		return nil, false
	}
	switch normID(h.Type) {
	case "minecraft:nop":
		return nil, true
	case "minecraft:rule":
		rp := &ruleProc{}
		for _, r := range h.Rules {
			rule, ok := parseRule(r)
			if !ok {
				return nil, false
			}
			rp.rules = append(rp.rules, rule)
		}
		return rp, true
	case "minecraft:block_rot":
		in := float32(1)
		if h.Integrity != nil {
			in = *h.Integrity
		}
		var tag string
		if len(h.Rottable) > 0 {
			if json.Unmarshal(h.Rottable, &tag) != nil {
				var ids []string
				json.Unmarshal(h.Rottable, &ids)
				return &blockRotProc{integrity: in, list: ids, hasSet: true}, true
			}
			return &blockRotProc{integrity: in, tag: tag, hasSet: true}, true
		}
		return &blockRotProc{integrity: in}, true
	case "minecraft:block_age":
		return &blockAgeProc{mossiness: h.Mossiness}, true
	case "minecraft:gravity":
		return &gravityProc{heightmap: h.Heightmap, offset: h.Offset}, true
	case "minecraft:blackstone_replace":
		return blackstoneProc{}, true
	case "minecraft:jigsaw_replacement":
		return jigsawReplaceProc{}, true
	case "minecraft:block_ignore":
		ig := ignoreProc{}
		for _, b := range h.Blocks {
			var st struct{ Name string }
			if json.Unmarshal(b, &st) == nil && st.Name != "" {
				ig[normID(st.Name)] = true
			}
		}
		return ig, true
	}
	// capped, protected_blocks, lava_submerged_block and mod processors: pass through (≈).
	return parseYungProcessor(normID(h.Type), raw)
}

// ---- rule processor ----

type ruleTest struct {
	typ   string
	block string
	state *BlockState
	tag   string
	prob  float32
}

type posTest struct {
	typ        string
	minC, maxC float32
	minD, maxD int
	axis       string
}

type procRule struct {
	input, loc ruleTest
	pos        posTest
	out        BlockState
}

type ruleProc struct{ rules []procRule }

func parseRuleTest(raw json.RawMessage) (ruleTest, bool) {
	var t struct {
		Type        string          `json:"predicate_type"`
		Block       string          `json:"block"`
		BlockState  json.RawMessage `json:"block_state"`
		Tag         string          `json:"tag"`
		Probability float32         `json:"probability"`
	}
	if len(raw) == 0 {
		return ruleTest{typ: "minecraft:always_true"}, true
	}
	if json.Unmarshal(raw, &t) != nil {
		return ruleTest{}, false
	}
	rt := ruleTest{typ: normID(t.Type), block: normID(t.Block), tag: t.Tag, prob: t.Probability}
	if len(t.BlockState) > 0 {
		st := parseStateJSON(t.BlockState)
		rt.state = &st
	}
	switch rt.typ {
	case "minecraft:always_true", "minecraft:block_match", "minecraft:blockstate_match", "minecraft:tag_match",
		"minecraft:random_block_match", "minecraft:random_blockstate_match":
		return rt, true
	}
	return rt, false
}

func parseStateJSON(raw json.RawMessage) BlockState {
	var s struct {
		Name       string            `json:"Name"`
		Properties map[string]string `json:"Properties"`
	}
	json.Unmarshal(raw, &s)
	if s.Properties == nil {
		s.Properties = map[string]string{}
	}
	return BlockState{Name: normID(s.Name), Properties: s.Properties}
}

func parseRule(raw json.RawMessage) (procRule, bool) {
	var r struct {
		In  json.RawMessage `json:"input_predicate"`
		Loc json.RawMessage `json:"location_predicate"`
		Pos json.RawMessage `json:"position_predicate"`
		Out json.RawMessage `json:"output_state"`
	}
	if json.Unmarshal(raw, &r) != nil {
		return procRule{}, false
	}
	in, ok1 := parseRuleTest(r.In)
	loc, ok2 := parseRuleTest(r.Loc)
	pr := procRule{input: in, loc: loc, out: parseStateJSON(r.Out), pos: posTest{typ: "minecraft:always_true"}}
	if len(r.Pos) > 0 {
		var p struct {
			Type string  `json:"predicate_type"`
			MinC float32 `json:"min_chance"`
			MaxC float32 `json:"max_chance"`
			MinD int     `json:"min_dist"`
			MaxD int     `json:"max_dist"`
			Axis string  `json:"axis"`
		}
		if json.Unmarshal(r.Pos, &p) != nil {
			return pr, false
		}
		if p.Axis == "" {
			p.Axis = "y"
		}
		pr.pos = posTest{typ: normID(p.Type), minC: p.MinC, maxC: p.MaxC, minD: p.MinD, maxD: p.MaxD, axis: p.Axis}
		switch pr.pos.typ {
		case "minecraft:always_true", "minecraft:linear_pos", "minecraft:axis_aligned_linear_pos":
		default:
			return pr, false
		}
	}
	return pr, ok1 && ok2
}

func (c *procCtx) inTag(tag, block string) bool {
	tag = normID(strings.TrimPrefix(tag, "#"))
	m, ok := c.tags[tag]
	if !ok {
		m = map[string]bool{}
		c.tags[tag] = m
		var walk func(t string, depth int)
		walk = func(t string, depth int) {
			if depth > 8 {
				return
			}
			ns, name := Namespaced(t)
			vals := c.store.ReadTagValues(DataPath(ns, "tags/blocks", name, ".json"))
			vals = append(vals, c.store.ReadTagValues(DataPath(ns, "tags/block", name, ".json"))...)
			for _, v := range vals {
				if strings.HasPrefix(v, "#") {
					walk(normID(v[1:]), depth+1)
				} else {
					m[normID(v)] = true
				}
			}
		}
		walk(tag, 0)
	}
	return m[block]
}

func sameState(a BlockState, b BlockState) bool {
	if a.Name != b.Name {
		return false
	}
	// Codec states fill defaults we do not know; compare listed properties only.
	for k, v := range b.Properties {
		if a.Properties[k] != v {
			return false
		}
	}
	return true
}

func (t ruleTest) test(c *procCtx, st BlockState, r *worldgen.JavaRandom) bool {
	switch t.typ {
	case "minecraft:always_true":
		return true
	case "minecraft:block_match":
		return st.Name == t.block
	case "minecraft:blockstate_match":
		return t.state != nil && sameState(st, *t.state)
	case "minecraft:tag_match":
		return c.inTag(t.tag, st.Name)
	case "minecraft:random_block_match":
		return st.Name == t.block && r.NextFloat() < t.prob
	case "minecraft:random_blockstate_match":
		return t.state != nil && sameState(st, *t.state) && r.NextFloat() < t.prob
	}
	return false
}

func clampedLerp(a, b, t float32) float32 {
	if t < 0 {
		return a
	}
	if t > 1 {
		return b
	}
	return a + t*(b-a)
}

func (p posTest) test(local, world, pivot [3]int, r *worldgen.JavaRandom) bool {
	switch p.typ {
	case "minecraft:linear_pos":
		d := abs(world[0]-pivot[0]) + abs(world[1]-pivot[1]) + abs(world[2]-pivot[2])
		f := r.NextFloat()
		return f <= clampedLerp(p.minC, p.maxC, float32(d-p.minD)/float32(p.maxD-p.minD))
	case "minecraft:axis_aligned_linear_pos":
		ax := map[string]int{"x": 0, "y": 1, "z": 2}[p.axis]
		d := abs(world[ax] - pivot[ax])
		f := r.NextFloat()
		return f <= clampedLerp(p.minC, p.maxC, float32(d-p.minD)/float32(p.maxD-p.minD))
	}
	return true
}

func (rp *ruleProc) process(c *procCtx, b procBlock) *procBlock {
	r := procRandom(b.world)
	ex, ok := c.existing(b.world)
	if !ok {
		ex = BlockState{Name: "minecraft:air"}
	}
	for _, rule := range rp.rules {
		if rule.loc.typ != "minecraft:always_true" && !ok {
			c.approx = true // world terrain under the structure is unknown
		}
		if rule.input.test(c, b.state, r) && rule.loc.test(c, ex, r) && rule.pos.test(b.local, b.world, c.pivot, r) {
			b.state = rule.out
			return &b
		}
	}
	return &b
}

// ---- block_rot ----

type blockRotProc struct {
	integrity float32
	hasSet    bool
	tag       string
	list      []string
}

func (p *blockRotProc) process(c *procCtx, b procBlock) *procBlock {
	if p.hasSet {
		in := false
		if p.tag != "" {
			in = c.inTag(p.tag, b.state.Name)
		}
		for _, id := range p.list {
			if normID(id) == b.state.Name {
				in = true
			}
		}
		if !in {
			return &b
		}
	}
	if procRandom(b.world).NextFloat() <= p.integrity {
		return &b
	}
	return nil
}

// ---- block_age ----

type blockAgeProc struct{ mossiness float32 }

var horizontalFacings = [4]string{"north", "east", "south", "west"}

func randomStairs(r *worldgen.JavaRandom, name string) BlockState {
	f := horizontalFacings[r.NextInt(4)]
	h := [2]string{"top", "bottom"}[r.NextInt(2)]
	return BlockState{Name: name, Properties: map[string]string{"facing": f, "half": h, "shape": "straight", "waterlogged": "false"}}
}

func plain(name string) BlockState { return BlockState{Name: name, Properties: map[string]string{}} }

func (p *blockAgeProc) process(c *procCtx, b procBlock) *procBlock {
	r := procRandom(b.world)
	st := b.state
	var out *BlockState
	switch {
	case st.Name == "minecraft:stone_bricks" || st.Name == "minecraft:stone" || st.Name == "minecraft:chiseled_stone_bricks":
		if r.NextFloat() >= 0.5 {
			return &b
		}
		a := []BlockState{plain("minecraft:cracked_stone_bricks"), randomStairs(r, "minecraft:stone_brick_stairs")}
		m := []BlockState{plain("minecraft:mossy_stone_bricks"), randomStairs(r, "minecraft:mossy_stone_brick_stairs")}
		var s BlockState
		if r.NextFloat() < p.mossiness {
			s = m[r.NextInt(2)]
		} else {
			s = a[r.NextInt(2)]
		}
		out = &s
	case c.inTag("minecraft:stairs", st.Name):
		if r.NextFloat() >= 0.5 {
			return &b
		}
		ms := BlockState{Name: "minecraft:mossy_stone_brick_stairs", Properties: map[string]string{}}
		for _, k := range []string{"facing", "half"} {
			if v, ok := st.Properties[k]; ok {
				ms.Properties[k] = v
			}
		}
		nonMossy := []BlockState{plain("minecraft:stone_slab"), plain("minecraft:stone_brick_slab")}
		mossy := []BlockState{ms, plain("minecraft:mossy_stone_brick_slab")}
		var s BlockState
		if r.NextFloat() < p.mossiness {
			s = mossy[r.NextInt(2)]
		} else {
			s = nonMossy[r.NextInt(2)]
		}
		out = &s
	case c.inTag("minecraft:slabs", st.Name):
		if r.NextFloat() < p.mossiness {
			s := plain("minecraft:mossy_stone_brick_slab")
			out = &s
		}
	case c.inTag("minecraft:walls", st.Name):
		if r.NextFloat() < p.mossiness {
			s := plain("minecraft:mossy_stone_brick_wall")
			out = &s
		}
	case st.Name == "minecraft:obsidian":
		if r.NextFloat() < 0.15 {
			s := plain("minecraft:crying_obsidian")
			out = &s
		}
	}
	if out != nil {
		b.state = *out
	}
	return &b
}

// ---- gravity ----

type gravityProc struct {
	heightmap string
	offset    int
}

func (p *gravityProc) process(c *procCtx, b procBlock) *procBlock {
	c.gravity = true
	if c.height == nil {
		c.approx = true
		return &b
	}
	h := c.height(b.world[0], b.world[2]) + 1 // WORLD_SURFACE_WG / OCEAN_FLOOR_WG first free
	b.world[1] = h + p.offset + b.local[1]
	return &b
}

// ---- blackstone_replace ----

type blackstoneProc struct{}

var blackstoneTable = map[string]string{
	"cobblestone": "blackstone", "mossy_cobblestone": "blackstone", "stone": "polished_blackstone",
	"stone_bricks": "polished_blackstone_bricks", "mossy_stone_bricks": "polished_blackstone_bricks",
	"cobblestone_stairs": "blackstone_stairs", "mossy_cobblestone_stairs": "blackstone_stairs",
	"stone_stairs": "polished_blackstone_stairs", "stone_brick_stairs": "polished_blackstone_brick_stairs",
	"mossy_stone_brick_stairs": "polished_blackstone_brick_stairs", "cobblestone_slab": "blackstone_slab",
	"mossy_cobblestone_slab": "blackstone_slab", "smooth_stone_slab": "polished_blackstone_slab",
	"stone_slab": "polished_blackstone_slab", "stone_brick_slab": "polished_blackstone_brick_slab",
	"mossy_stone_brick_slab": "polished_blackstone_brick_slab", "stone_brick_wall": "polished_blackstone_brick_wall",
	"mossy_stone_brick_wall": "polished_blackstone_brick_wall", "cobblestone_wall": "blackstone_wall",
	"mossy_cobblestone_wall": "blackstone_wall", "chiseled_stone_bricks": "chiseled_polished_blackstone",
	"cracked_stone_bricks": "cracked_polished_blackstone_bricks", "iron_bars": "chain",
}

func (blackstoneProc) process(c *procCtx, b procBlock) *procBlock {
	to, ok := blackstoneTable[strings.TrimPrefix(b.state.Name, "minecraft:")]
	if !ok || !strings.HasPrefix(b.state.Name, "minecraft:") {
		return &b
	}
	ns := plain("minecraft:" + to)
	for _, k := range []string{"facing", "half", "type"} {
		if v, ok := b.state.Properties[k]; ok {
			ns.Properties[k] = v
		}
	}
	b.state = ns
	return &b
}

// ---- jigsaw_replacement / block_ignore ----

type jigsawReplaceProc struct{}

func (jigsawReplaceProc) process(c *procCtx, b procBlock) *procBlock {
	if b.state.Name != "minecraft:jigsaw" {
		return &b
	}
	fs, _ := b.nbt["final_state"].(string)
	if fs == "" {
		fs = "minecraft:air"
	}
	st := parseStateString(fs)
	if st.Name == "minecraft:structure_void" {
		return nil
	}
	b.state = st
	return &b
}

// parseStateString reads "ns:id[k=v,...]".
func parseStateString(s string) BlockState {
	st := BlockState{Properties: map[string]string{}}
	name := s
	if i := strings.IndexByte(s, '['); i >= 0 {
		name = s[:i]
		for _, kv := range strings.Split(strings.TrimSuffix(s[i+1:], "]"), ",") {
			if k, v, ok := strings.Cut(kv, "="); ok {
				st.Properties[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
	}
	if j := strings.IndexByte(name, '{'); j >= 0 {
		name = name[:j]
	}
	st.Name = normID(strings.TrimSpace(name))
	return st
}

type ignoreProc map[string]bool

func (p ignoreProc) process(c *procCtx, b procBlock) *procBlock {
	if p[b.state.Name] {
		return nil
	}
	return &b
}

// pieceProcessors is SinglePoolElement.getSettings: structure-block ignore,
// jigsaw replacement, the element's list, then the projection's processors;
// legacy elements swap the first ignore for STRUCTURE_AND_AIR at the end.
func pieceProcessors(list []processor, legacy, terrainMatching bool) []processor {
	out := []processor{}
	if !legacy {
		out = append(out, ignoreProc{"minecraft:structure_block": true})
	}
	out = append(out, jigsawReplaceProc{})
	out = append(out, list...)
	if terrainMatching {
		out = append(out, &gravityProc{heightmap: "WORLD_SURFACE_WG", offset: -1})
	}
	if legacy {
		out = append(out, ignoreProc{"minecraft:structure_block": true, "minecraft:air": true})
	}
	return out
}
