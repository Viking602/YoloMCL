package cubiomes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
)

type climateRange struct{ Min, Max int64 }
type climatePoint [7]int64

type climateNode struct {
	Space    [7]climateRange
	Biome    string         `json:",omitempty"`
	Children []*climateNode `json:",omitempty"`
}

func climateQuantize(v float64) int64 { return int64(float32(v) * float32(10000)) }

func (n *climateNode) distance(p climatePoint) int64 {
	var sum int64
	for i, r := range n.Space {
		d := max(r.Min-p[i], p[i]-r.Max, 0)
		sum += d * d
	}
	return sum
}

// search uses strict comparisons, stable child order and the previous leaf,
// exactly as Climate.RTree. Equal-distance matches keep the previous result.
func (n *climateNode) search(p climatePoint, previous *climateNode) *climateNode {
	if len(n.Children) == 0 {
		return n
	}
	best, distance := previous, int64(math.MaxInt64)
	if best != nil {
		distance = best.distance(p)
	}
	for _, child := range n.Children {
		if child.distance(p) < distance {
			candidate := child.search(p, best)
			if d := candidate.distance(p); d < distance {
				best, distance = candidate, d
			}
		}
	}
	return best
}

func climateBranch(children []*climateNode) *climateNode {
	n := &climateNode{Children: append([]*climateNode(nil), children...), Space: children[0].Space}
	for _, child := range children[1:] {
		for i, r := range child.Space {
			n.Space[i] = climateRange{min(n.Space[i].Min, r.Min), max(n.Space[i].Max, r.Max)}
		}
	}
	return n
}

func climateAbs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func climateSort(nodes []*climateNode, axis int, absolute bool) {
	sort.SliceStable(nodes, func(i, j int) bool {
		for offset := range 7 {
			d := (axis + offset) % 7
			a, b := (nodes[i].Space[d].Min+nodes[i].Space[d].Max)/2, (nodes[j].Space[d].Min+nodes[j].Space[d].Max)/2
			if absolute {
				a, b = climateAbs(a), climateAbs(b)
			}
			if a != b {
				return a < b
			}
		}
		return false
	})
}

func buildClimateTree(ctx context.Context, nodes []*climateNode, order int) (*climateNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(nodes) == 1 {
		return nodes[0], nil
	}
	if len(nodes) <= order {
		cost := func(n *climateNode) int64 {
			var sum int64
			for _, r := range n.Space {
				sum += climateAbs((r.Min + r.Max) / 2)
			}
			return sum
		}
		sort.SliceStable(nodes, func(i, j int) bool { return cost(nodes[i]) < cost(nodes[j]) })
		return climateBranch(nodes), nil
	}
	// Largest power of the tree order strictly below count; integer arithmetic
	// avoids logarithm rounding at exact bucket boundaries.
	size := 1
	for size*order < len(nodes) {
		size *= order
	}
	var best []*climateNode
	bestCost, bestAxis := int64(math.MaxInt64), 0
	for axis := range 7 {
		climateSort(nodes, axis, false)
		var buckets []*climateNode
		var cost int64
		for start := 0; start < len(nodes); start += size {
			bucket := climateBranch(nodes[start:min(start+size, len(nodes))])
			buckets = append(buckets, bucket)
			for _, r := range bucket.Space {
				cost += climateAbs(r.Max - r.Min)
			}
		}
		if cost < bestCost {
			best, bestCost, bestAxis = buckets, cost, axis
		}
	}
	climateSort(best, bestAxis, true)
	for i, bucket := range best {
		child, err := buildClimateTree(ctx, bucket.Children, order)
		if err != nil {
			return nil, err
		}
		best[i] = child
	}
	return climateBranch(best), nil
}

func parseClimateRange(raw json.RawMessage, offset bool) (climateRange, error) {
	lo, hi := 0.0, 0.0
	if n, err := densityNumber(raw); err == nil {
		lo, hi = n, n
	} else {
		var pair []float64
		if offset || json.Unmarshal(raw, &pair) != nil || len(pair) != 2 {
			return climateRange{}, errors.New("invalid climate interval")
		}
		lo, hi = pair[0], pair[1]
	}
	if lo > hi || lo < -2 || hi > 2 || (offset && (lo < 0 || hi > 1)) {
		return climateRange{}, errors.New("climate interval out of range")
	}
	return climateRange{climateQuantize(lo), climateQuantize(hi)}, nil
}

func parseClimateList(ctx context.Context, raw json.RawMessage, order int) (*climateTree, []string, error) {
	var entries []struct {
		Biome      string                     `json:"biome"`
		Parameters map[string]json.RawMessage `json:"parameters"`
	}
	if json.Unmarshal(raw, &entries) != nil || len(entries) == 0 || len(entries) > 16384 {
		return nil, nil, errors.New("invalid biome parameter list")
	}
	nodes, ids := make([]*climateNode, 0, len(entries)), []string{}
	seen := map[string]bool{}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if entry.Biome == "" {
			return nil, nil, errors.New("missing biome ID")
		}
		n := &climateNode{Biome: seedMapID(entry.Biome)}
		for i, key := range []string{"temperature", "humidity", "continentalness", "erosion", "depth", "weirdness", "offset"} {
			r, err := parseClimateRange(entry.Parameters[key], i == 6)
			if err != nil {
				return nil, nil, fmt.Errorf("%s %s: %w", n.Biome, key, err)
			}
			n.Space[i] = r
		}
		nodes = append(nodes, n)
		if !seen[n.Biome] {
			seen[n.Biome] = true
			ids = append(ids, n.Biome)
		}
	}
	root, err := buildClimateTree(ctx, nodes, order)
	if err != nil {
		return nil, nil, err
	}
	return compactClimateTree(root, ids), ids, nil
}

type climateSampler struct {
	router       [6]*densityNode
	root         *climateTree
	nodes        int
	spawnTargets []*climateNode
	hasEnd       bool
}

func (s *climateSampler) target(p *densitySample, x, y, z int) (climatePoint, bool) {
	// QuartPos.fromBlock is an arithmetic right shift, including negatives.
	p.at((x>>2)<<2, (y>>2)<<2, (z>>2)<<2)
	return s.targetAt(p)
}

func (s *climateSampler) targetAt(p *densitySample) (climatePoint, bool) {
	var target climatePoint
	for i, node := range s.router {
		v := node.eval(p)
		// Bound distance arithmetic for malicious datapacks producing infinities
		// or extreme values. Such samples remain unknown instead of wrapping.
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 10000 {
			return target, false
		}
		target[i] = climateQuantize(v)
	}
	return target, true
}

func compileClimate(ctx context.Context, docs map[string]Document, seed int64, settings, source json.RawMessage, version string) (*climateSampler, []string, error) {
	s, err := compileClimateRouter(ctx, docs, seed, settings, version)
	if err != nil {
		return nil, nil, err
	}
	root, ids, err := resolveClimateSource(ctx, docs, source, version)
	if err != nil {
		return nil, nil, err
	}
	s.root = root
	return s, ids, nil
}

func compileClimateRouter(ctx context.Context, docs map[string]Document, seed int64, settings json.RawMessage, version string) (*climateSampler, error) {
	if latestFloatRelease(version) {
		return compileLatestClimateRouter(ctx, docs, seed, settings)
	}
	var ref string
	if json.Unmarshal(settings, &ref) == nil && ref != "" {
		settings = docs["noise_settings/"+seedMapID(ref)].Data
	}
	var data struct {
		Legacy       bool                         `json:"legacy_random_source"`
		Router       map[string]json.RawMessage   `json:"noise_router"`
		SpawnTargets []map[string]json.RawMessage `json:"spawn_target"`
		Noise        struct {
			Shaper map[string]json.RawMessage `json:"terrain_shaper"`
		} `json:"noise"`
	}
	if json.Unmarshal(settings, &data) != nil || data.Router == nil {
		return nil, errors.New("invalid or missing noise settings")
	}
	c := newDensityCompiler(ctx, docs, seed, data.Legacy)
	c.version = version
	c.shaper = data.Noise.Shaper
	s := &climateSampler{}
	for i, key := range []string{"temperature", "vegetation", "continents", "erosion", "depth", "ridges"} {
		n, err := c.compile(data.Router[key], 0)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		s.router[i] = n
	}
	s.nodes = c.nodes
	s.hasEnd = c.end != nil
	if len(data.SpawnTargets) > 256 {
		return nil, errors.New("spawn target limit")
	}
	for _, definition := range data.SpawnTargets {
		n := &climateNode{}
		for i, key := range []string{"temperature", "humidity", "continentalness", "erosion", "depth", "weirdness", "offset"} {
			r, e := parseClimateRange(definition[key], i == 6)
			if e != nil {
				return nil, e
			}
			n.Space[i] = r
		}
		s.spawnTargets = append(s.spawnTargets, n)
	}
	return s, nil
}

func resolveClimateSource(ctx context.Context, docs map[string]Document, raw json.RawMessage, version string) (*climateTree, []string, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	var data struct {
		Preset string          `json:"preset"`
		Biomes json.RawMessage `json:"biomes"`
	}
	if json.Unmarshal(raw, &data) != nil {
		return nil, nil, errors.New("invalid multi_noise biome source")
	}
	if len(data.Biomes) > 0 {
		return parseClimateList(ctx, data.Biomes, climateTreeOrder(version))
	}
	id := seedMapID(data.Preset)
	if id == "" {
		return nil, nil, errors.New("missing biome preset")
	}
	if doc, ok := docs["multi_noise_biome_source_parameter_list/"+id]; ok {
		// A source's preset is a registry holder; inside a parameter-list
		// definition, preset instead names a Java builtin. It must not resolve
		// the registry again (or turn a recursive override into vanilla data).
		data.Preset, data.Biomes = "", nil
		if json.Unmarshal(doc.Data, &data) != nil {
			return nil, nil, errors.New("invalid biome parameter list")
		}
		if len(data.Biomes) > 0 {
			return parseClimateList(ctx, data.Biomes, climateTreeOrder(version))
		}
		id = seedMapID(data.Preset)
	}
	return vanillaClimateTree(ctx, version, id)
}
