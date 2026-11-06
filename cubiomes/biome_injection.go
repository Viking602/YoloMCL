package cubiomes

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
)

// BiomeInjection and BiomeRegion describe declarative Lithostitched rules.
// The caller expands registry tags and supplies the effective ordered resources.
type BiomeInjection struct {
	ID, Type, Region, Replacement string
	Priority                      int
	Targets                       []string
	Parameters                    map[string]json.RawMessage
}
type BiomeRegion struct {
	ID     string
	Weight int
	Biomes []string
}
type BiomeInjectionConfig struct {
	Rules         []BiomeInjection
	Regions       []BiomeRegion
	RegionDensity json.RawMessage
	LegacyRandom  bool
}
type biomeCondition struct {
	axis     int
	node     *densityNode
	min, max float64
}
type biomeRule struct {
	kind, region string
	replacement  BiomeID
	targets      map[BiomeID]bool
	conditions   []biomeCondition
}
type biomeRegionWeight struct {
	id           string
	start, total int
}
type biomeInjection struct {
	rules         []biomeRule
	regions       map[BiomeID][]biomeRegionWeight
	regionDensity *densityNode
	nodes         int
}

// WithBiomeInjections composes injectors AFTER the original biome resolver.
// Unknown rule types fail; they are never treated as no-ops.
func WithBiomeInjections(ctx context.Context, base *Generator, docs map[string]Document, cfg BiomeInjectionConfig) (*Generator, error) {
	if len(cfg.Rules) == 0 {
		return base, nil
	}
	if base == nil || base.version != "1.21.1" || base.climate == nil || base.injection != nil || len(cfg.Rules) > 1024 || len(cfg.Regions) > 1024 {
		return nil, errors.New("invalid biome injection source")
	}
	g := *base
	g.names = append([]string(nil), base.names...)
	g.biomes = append([]string(nil), base.biomes...)
	ids := map[string]BiomeID{}
	for i, name := range g.names {
		if name != "" {
			ids[name] = BiomeID(i)
		}
	}
	add := func(name string) (BiomeID, error) {
		if id, ok := ids[name]; ok {
			return id, nil
		}
		if name == "" || len(g.names) > math.MaxUint16 {
			return 0, errors.New("invalid injected biome")
		}
		id := BiomeID(len(g.names))
		ids[name] = id
		g.names = append(g.names, name)
		g.biomes = append(g.biomes, name)
		return id, nil
	}
	in := &biomeInjection{regions: map[BiomeID][]biomeRegionWeight{}}
	c := newDensityCompiler(ctx, docs, g.seed, cfg.LegacyRandom)
	c.version = g.version
	rules := append([]BiomeInjection(nil), cfg.Rules...)
	sort.SliceStable(rules, func(i, j int) bool { return rules[i].Priority < rules[j].Priority })
	// A later stage may target a biome introduced by an earlier stage even
	// when its declaration appears first in the registry.
	for _, rule := range rules {
		if _, err := add(rule.Replacement); err != nil {
			return nil, err
		}
	}
	needsRegion := false
	for _, rule := range rules {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		r := biomeRule{kind: rule.Type, region: rule.Region, targets: map[BiomeID]bool{}}
		switch rule.Type {
		case "replace_partially", "replace_fully", "force_placement":
		default:
			return nil, &CompileError{"biome_source_unsupported", "Lithostitched injector: " + rule.Type}
		}
		var err error
		r.replacement, err = add(rule.Replacement)
		if err != nil {
			return nil, err
		}
		for _, name := range rule.Targets {
			if id, ok := ids[name]; ok {
				r.targets[id] = true
			}
		}
		needsRegion = needsRegion || rule.Region != ""
		for key, raw := range rule.Parameters {
			b := biomeCondition{axis: -1}
			var bounds struct{ Min, Max *float64 }
			var interval struct {
				Min *float64 `json:"min_inclusive"`
				Max *float64 `json:"max_inclusive"`
			}
			if number, err := densityNumber(raw); err == nil {
				bounds.Min, bounds.Max = &number, &number
			} else if json.Unmarshal(raw, &interval) == nil {
				bounds.Min, bounds.Max = interval.Min, interval.Max
			}
			if bounds.Min == nil || bounds.Max == nil || *bounds.Min > *bounds.Max {
				return nil, errors.New("invalid injection interval")
			}
			b.min, b.max = *bounds.Min, *bounds.Max
			for i, name := range []string{"temperature", "humidity", "continentalness", "erosion", "depth", "weirdness"} {
				if key == name {
					b.axis = i
					break
				}
			}
			if b.axis < 0 {
				ref, _ := json.Marshal(key)
				b.node, err = c.compile(ref, 0)
				if err != nil {
					return nil, err
				}
			}
			r.conditions = append(r.conditions, b)
		}
		in.rules = append(in.rules, r)
	}
	if needsRegion {
		if len(cfg.RegionDensity) > 0 {
			var err error
			in.regionDensity, err = c.compile(cfg.RegionDensity, 0)
			if err != nil {
				return nil, err
			}
		}
		regions := append([]BiomeRegion(nil), cfg.Regions...)
		sort.SliceStable(regions, func(i, j int) bool {
			a, b := regions[i].ID, regions[j].ID
			an, ap, _ := strings.Cut(a, ":")
			bn, bp, _ := strings.Cut(b, ":")
			if ap != bp {
				return ap < bp
			}
			return an < bn
		})
		for _, region := range regions {
			if region.Weight < 0 {
				return nil, errors.New("negative region weight")
			}
			if region.Weight == 0 {
				continue
			}
			for _, name := range region.Biomes {
				id, ok := ids[name]
				if !ok {
					continue
				}
				list := in.regions[id]
				total := 0
				if len(list) > 0 {
					total = list[len(list)-1].total
				}
				if int64(total)+int64(region.Weight) > math.MaxInt32 {
					return nil, errors.New("region weight overflow")
				}
				in.regions[id] = append(list, biomeRegionWeight{region.ID, total + 1, total + region.Weight})
			}
		}
	}
	in.nodes = c.nodes
	g.injection = in
	return &g, nil
}

func (s *Sampler) injectBiome(id BiomeID, p *climatePoint, x, y, z int) BiomeID {
	in := s.g.injection
	if in == nil {
		return id
	}
	eligible, needsRegion := false, false
	for i := range in.rules {
		r := &in.rules[i]
		if r.kind == "force_placement" || r.targets[id] {
			eligible = true
			needsRegion = needsRegion || r.region != ""
		}
	}
	if !eligible {
		return id
	}
	// Keep a separate density cache: a raster may retrieve climate values from
	// an older quart cell while the main density scratch points elsewhere.
	x, y, z = (x>>2)<<2, (y>>2)<<2, (z>>2)<<2
	if s.injection.serial == 0 || s.injection.x != float64(x) || s.injection.y != float64(y) || s.injection.z != float64(z) {
		s.injection.at(x, y, z)
	}
	region := "lithostitched:error/no_regions"
	if list := in.regions[id]; needsRegion && len(list) > 0 && in.regionDensity != nil {
		value := int(max(0, min(1, in.regionDensity.eval(&s.injection)))*float64(list[len(list)-1].total) + 1)
		for _, e := range list {
			if value >= e.start {
				region = e.id
			} else {
				break
			}
		}
	}
	matches := func(r *biomeRule) bool {
		if r.region != "" && r.region != region {
			return false
		}
		for _, c := range r.conditions {
			var value float64
			if c.axis >= 0 {
				value = float64(p[c.axis]) / 10000
			} else {
				value = c.node.eval(&s.injection)
			}
			if !(value >= c.min && value <= c.max) {
				return false
			}
		}
		return true
	}
	for i := range in.rules {
		r := &in.rules[i]
		if r.kind == "force_placement" && matches(r) {
			return r.replacement
		}
	}
	for i := range in.rules {
		r := &in.rules[i]
		if r.kind == "replace_partially" && r.targets[id] && matches(r) {
			id = r.replacement
			break
		}
	}
	for i := range in.rules {
		r := &in.rules[i]
		if r.kind == "replace_fully" && r.targets[id] {
			return r.replacement
		}
	}
	return id
}

func (in *biomeInjection) withSeed(seed int64) *biomeInjection {
	next := *in
	next.rules = append([]biomeRule(nil), in.rules...)
	r := seedBinder{seed: seed, nodes: make([]densityNode, in.nodes), seen: make([]bool, in.nodes), noises: map[*normalNoise]*normalNoise{}}
	next.regionDensity = r.node(in.regionDensity)
	for i := range next.rules {
		next.rules[i].conditions = append([]biomeCondition(nil), in.rules[i].conditions...)
		for j := range next.rules[i].conditions {
			next.rules[i].conditions[j].node = r.node(in.rules[i].conditions[j].node)
		}
	}
	return &next
}
