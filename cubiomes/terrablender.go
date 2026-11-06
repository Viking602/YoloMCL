package cubiomes

import (
	"context"
	"errors"
	"fmt"
	"math"
)

// TerraBlenderParameter is an already quantized climate box. Axes are
// temperature, humidity, continentalness, erosion, depth, weirdness and offset.
// Ranges are inclusive; the ordered list must retain the mod's registration
// order, including duplicate boxes, because equal-distance ties depend on it.
type TerraBlenderParameter struct {
	Biome string      `json:"biome"`
	Space [7][2]int64 `json:"space"`
}

// TerraBlenderRegion describes one registered region. Index zero is the base
// source, whose parameters come from the supplied Generator, not this table.
type TerraBlenderRegion struct {
	Index      int                     `json:"index"`
	ID         string                  `json:"id"`
	Weight     int                     `json:"weight"`
	Base       bool                    `json:"base"`
	Parameters []TerraBlenderParameter `json:"parameters,omitempty"`
}

// TerraBlenderConfig binds a versioned region table to an existing density
// graph. It does not execute arbitrary mod code or infer region registration.
// The caller must verify the installed mod versions, config and table identity.
type TerraBlenderConfig struct {
	Minecraft  string               `json:"minecraft"`
	Version    string               `json:"terrablender_version"`
	Dimension  Dimension            `json:"-"`
	RegionSize int                  `json:"region_size"`
	Regions    []TerraBlenderRegion `json:"regions"`
}

const terraBlenderDeferred = "terrablender:deferred_placeholder"

type terraBlenderTree struct {
	tree     *climateTree
	ids      []BiomeID
	deferred int
}

type terraBlender struct {
	regionSize    int
	total         int
	weights       []int     // cumulative weights, in registration order
	values        []BiomeID // End tables map ordered choices to canonical biome IDs
	emptyValue    int
	constant      bool
	constantValue int
	trees         []terraBlenderTree
}

// WithTerraBlender adds native positional region selection to a compiled
// climate source. Supported contracts are 3.0.1.10 on Minecraft 1.20.1 and
// 26.3.0.0.9 on Minecraft 26.3. Other versions fail explicitly.
// No JVM, process, network service or mutable input data is retained.
func WithTerraBlender(ctx context.Context, base *Generator, cfg TerraBlenderConfig) (*Generator, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if base == nil || base.climate == nil || base.tb != nil {
		return nil, errors.New("TerraBlender requires an unwrapped multi-noise generator")
	}
	verified := cfg.Minecraft == "26.3" && cfg.Version == "26.3.0.0.9" || cfg.Minecraft == "1.20.1" && cfg.Version == "3.0.1.10" || cfg.Minecraft == "1.21.1" && cfg.Version == "4.1.0.8"
	if !verified || base.version != cfg.Minecraft {
		return nil, fmt.Errorf("unsupported TerraBlender/Minecraft combination %q/%q (generator %q)", cfg.Version, cfg.Minecraft, base.version)
	}
	if cfg.Dimension != Overworld && cfg.Dimension != Nether {
		return nil, errors.New("TerraBlender region lookup requires Overworld or Nether")
	}
	if cfg.RegionSize < 2 || cfg.RegionSize > 6 || len(cfg.Regions) == 0 || len(cfg.Regions) > 256 {
		return nil, errors.New("TerraBlender region size or table exceeds supported limits")
	}
	tb := &terraBlender{regionSize: cfg.RegionSize, weights: make([]int, len(cfg.Regions)), trees: make([]terraBlenderTree, len(cfg.Regions))}
	g := *base
	g.names = append([]string(nil), base.names...)
	g.biomes = append([]string(nil), base.biomes...)
	palette := make(map[string]BiomeID, len(g.names))
	for i, name := range g.names {
		if name != "" {
			palette[name] = BiomeID(i)
		}
	}
	reachable := make(map[string]bool, len(g.biomes))
	for _, name := range g.biomes {
		if name == terraBlenderDeferred {
			return nil, errors.New("TerraBlender base source contains a deferred placeholder")
		}
		reachable[name] = true
	}
	seen := make(map[string]bool, len(cfg.Regions))
	entries := 0
	for i, region := range cfg.Regions {
		if region.Index != i || region.ID == "" || seen[region.ID] || region.Base != (i == 0) || region.Weight < 0 || int64(tb.total)+int64(region.Weight) > math.MaxInt32 {
			return nil, fmt.Errorf("invalid TerraBlender region %d (%q)", i, region.ID)
		}
		seen[region.ID] = true
		tb.total += region.Weight
		tb.weights[i] = tb.total
		t := &tb.trees[i]
		t.deferred = -1
		if i == 0 {
			if len(region.Parameters) != 0 {
				return nil, errors.New("TerraBlender base parameters must come from the supplied generator")
			}
			t.tree, t.ids = base.climate.root, base.ids
			continue
		}
		entries += len(region.Parameters)
		if len(region.Parameters) == 0 || entries > 262144 {
			return nil, fmt.Errorf("TerraBlender region %q has empty or excessive parameters", region.ID)
		}
		nodes := make([]*climateNode, len(region.Parameters))
		biomes := make([]string, 0)
		indices := make(map[string]bool)
		for j, param := range region.Parameters {
			if j&1023 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			if param.Biome == "" {
				return nil, fmt.Errorf("TerraBlender region %q contains an empty biome", region.ID)
			}
			n := &climateNode{Biome: param.Biome}
			for axis, bounds := range param.Space {
				// This bound exceeds valid climate codec ranges while keeping all
				// squared-distance sums safely inside int64, even for custom data.
				if bounds[0] < -100000000 || bounds[1] > 100000000 || bounds[0] > bounds[1] {
					return nil, fmt.Errorf("invalid TerraBlender climate interval in %q", region.ID)
				}
				n.Space[axis] = climateRange{bounds[0], bounds[1]}
			}
			nodes[j] = n
			if !indices[param.Biome] {
				indices[param.Biome] = true
				biomes = append(biomes, param.Biome)
			}
		}
		if len(biomes) > math.MaxUint16 {
			return nil, errors.New("TerraBlender biome palette is too large")
		}
		root, err := buildClimateTree(ctx, nodes, climateTreeOrder(cfg.Minecraft))
		if err != nil {
			return nil, err
		}
		t.tree = compactClimateTree(root, biomes)
		t.ids = make([]BiomeID, len(biomes))
		for j, name := range biomes {
			if name == terraBlenderDeferred {
				t.deferred = j
				continue
			}
			id, ok := palette[name]
			if !ok {
				if len(g.names) > math.MaxUint16 {
					return nil, errors.New("TerraBlender biome palette is too large")
				}
				id = BiomeID(len(g.names))
				palette[name] = id
				g.names = append(g.names, name)
			}
			t.ids[j] = id
			if !reachable[name] {
				reachable[name] = true
				g.biomes = append(g.biomes, name)
			}
		}
	}
	tb.foldConstant()
	g.tb = tb
	return &g, nil
}

type terraBlenderScratch struct {
	layers   []terraBlenderLayer
	previous []int
}

func (tb *terraBlender) newScratch(seed int64) *terraBlenderScratch {
	s := &terraBlenderScratch{previous: make([]int, len(tb.trees))}
	s.reset()
	if tb.constant {
		return s
	}
	s.layers = make([]terraBlenderLayer, 5+tb.regionSize)
	capacity := 32
	for i := range s.layers {
		salt := uint64(1)
		if i > 0 && i < 5 {
			salt = uint64(1999 + i)
		} else if i >= 5 {
			salt = uint64(996 + i)
		}
		_, mixed := legacyLayerSeeds(uint64(seed), salt)
		s.layers[i] = terraBlenderLayer{seed: mixed, cache: make([]terraBlenderCell, capacity)}
		capacity = min(capacity*4, 1024)
	}
	return s
}

// All zoom operators preserve a constant input field. Folding one-output
// tables removes their entire cache allocation, notably three default End
// areas and TerraBlender-only vanilla regions, without changing random state:
// every nonconstant cell starts from its own coordinate-derived random state.
func (tb *terraBlender) foldConstant() {
	if tb.total == 0 {
		tb.constant, tb.constantValue = true, tb.emptyValue
		return
	}
	previous, value := 0, -1
	for i, weight := range tb.weights {
		if weight == previous {
			continue
		}
		previous = weight
		candidate := i
		if tb.values != nil {
			candidate = int(tb.values[i])
		}
		if value >= 0 && value != candidate {
			return
		}
		value = candidate
	}
	tb.constant, tb.constantValue = true, value
}

func (s *terraBlenderScratch) reset() {
	for i := range s.previous {
		s.previous[i] = -1
	}
}

// TerraBlenderRegionIndex reports the selected region at block coordinates.
// This diagnostic does not change climate search history. False means this
// sampler has no TerraBlender configuration or coordinates exceed Java int32.
func (s *Sampler) TerraBlenderRegionIndex(x, z int) (int, bool) {
	if s.tb == nil || int64(x) < math.MinInt32 || int64(x) > math.MaxInt32 || int64(z) < math.MinInt32 || int64(z) > math.MaxInt32 {
		return 0, false
	}
	return s.tb.region(s.g.tb, len(s.tb.layers)-1, int32(x)>>2, int32(z)>>2), true
}

func (s *Sampler) terraBlenderBiome(x, y, z int) (BiomeID, bool) {
	p, ok := s.g.climate.target(&s.scratch, x, y, z)
	if !ok {
		return 0, false
	}
	region := s.tb.region(s.g.tb, len(s.tb.layers)-1, int32(x)>>2, int32(z)>>2)
	return s.terraBlenderClimate(&p, region), true
}

func (s *Sampler) terraBlenderClimate(p *climatePoint, region int) BiomeID {
	t := &s.g.tb.trees[region]
	previous := t.tree.search(p, s.tb.previous[region])
	s.tb.previous[region] = previous
	biome := int(t.tree.nodes[previous].biome)
	if biome == t.deferred {
		t = &s.g.tb.trees[0]
		previous = t.tree.search(p, s.tb.previous[0])
		s.tb.previous[0] = previous
		biome = int(t.tree.nodes[previous].biome)
	}
	return t.ids[biome]
}
