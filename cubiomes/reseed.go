package cubiomes

import "context"

// WithSeed reuses the validated density graph and biome tree, binding only the
// seed-dependent noise. The original Generator remains immutable and usable.
// Explicit seed fields in historical biome sources keep their saved value.
func (g *Generator) WithSeed(ctx context.Context, seed int64) (*Generator, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if g.seed == seed {
		return g, nil
	}
	if l := g.legacy; l != nil {
		cfg := l.config
		if !l.fixedSeed {
			cfg.Seed = seed
		}
		next, err := newLegacyGenerator(cfg, l.mc)
		if err == nil {
			next.legacy.fixedSeed = l.fixedSeed
			next.seed, next.sha = seed, seedSHA(seed)
		}
		return next, err
	}
	next := *g
	next.seed, next.sha = seed, seedSHA(seed)
	if g.injection != nil {
		next.injection = g.injection.withSeed(seed)
	}
	r := seedBinder{seed: seed, nodes: make([]densityNode, g.NodeCount()), seen: make([]bool, g.NodeCount()), noises: make(map[*normalNoise]*normalNoise)}
	if g.climate != nil {
		climate := *g.climate
		for i, n := range g.climate.router {
			climate.router[i] = r.node(n)
		}
		next.climate = &climate
		next.surface = r.node(g.surface)
	}
	if g.end != nil {
		end := *g.end
		end.erosion = r.node(end.erosion)
		next.end = &end
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &next, nil
}

type noiseSpec struct {
	id                   string
	first                int
	amplitudes           []float64
	legacy, legacyNether bool
}

func (s *noiseSpec) bind(seed int64) *normalNoise {
	r := newNoiseRandom(seed, s.legacy).forkPositional().fromHash(noiseSeedID(s.id))
	if s.legacyNether {
		if s.id == "minecraft:vegetation" || s.id == "minecraft:nether/vegetation" {
			seed++
		}
		r = newNoiseRandom(seed, true)
	}
	n := newNormalNoise(r, s.first, s.amplitudes, s.legacyNether)
	n.spec = s
	return n
}

type seedBinder struct {
	seed   int64
	nodes  []densityNode
	seen   []bool
	noises map[*normalNoise]*normalNoise
	end    *endIslands
}

func (r *seedBinder) node(old *densityNode) *densityNode {
	if old == nil {
		return nil
	}
	n := &r.nodes[old.id]
	if r.seen[old.id] {
		return n
	}
	r.seen[old.id] = true
	*n = *old
	n.a, n.b, n.c = r.node(old.a), r.node(old.b), r.node(old.c)
	if old.cellular != nil {
		value := *old.cellular
		value.seed = int32(r.seed) + value.salt
		n.cellular = &value
	}
	if old.noise != nil {
		n.noise = r.noises[old.noise]
		if n.noise == nil {
			n.noise = old.noise.spec.bind(r.seed)
			r.noises[old.noise] = n.noise
		}
	}
	if old.end != nil {
		if r.end == nil {
			r.end = newEndIslands(r.seed)
		}
		n.end = r.end
	}
	n.spline = r.spline(old.spline)
	return n
}

func (r *seedBinder) spline(old *densitySpline) *densitySpline {
	if old == nil || old.coordinate == nil {
		return old // Constants contain no seed-dependent references.
	}
	s := &densitySpline{coordinate: r.node(old.coordinate), points: make([]densitySplinePoint, len(old.points))}
	for i, p := range old.points {
		s.points[i] = p
		s.points[i].value = r.spline(p.value)
	}
	return s
}
