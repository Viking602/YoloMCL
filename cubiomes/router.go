package cubiomes

import (
	"context"
	"fmt"
	"math"
)

// NoiseRouter is the immutable, seeded climate portion of RandomState. Unlike
// the vanilla climate lookup it preserves raw doubles and accepts the exact
// block coordinates supplied by a custom biome source. No biome source or
// vanilla climate tree is substituted for the caller's program.
type NoiseRouter struct{ program *Generator }

// NoiseRequirements describes the primitives reached by the six climate
// functions, including functions nested inside spline coordinates and values.
type NoiseRequirements struct{ NormalNoise, EndIslands bool }

func (r *NoiseRouter) Requirements() NoiseRequirements {
	var out NoiseRequirements
	seen := map[*densityNode]bool{}
	var node func(*densityNode)
	var spline func(*densitySpline)
	spline = func(s *densitySpline) {
		if s == nil {
			return
		}
		node(s.coordinate)
		for _, p := range s.points {
			spline(p.value)
		}
	}
	node = func(n *densityNode) {
		if n == nil || seen[n] {
			return
		}
		seen[n] = true
		out.NormalNoise = out.NormalNoise || n.noise != nil
		out.EndIslands = out.EndIslands || n.end != nil
		node(n.a)
		node(n.b)
		node(n.c)
		spline(n.spline)
	}
	for _, n := range r.program.climate.router {
		node(n)
	}
	return out
}

func (s *NoiseRouterSampler) Requirements() NoiseRequirements { return s.router.Requirements() }

// CompileNoiseRouter binds all six climate functions from the effective noise
// settings. BiomeSource is deliberately not interpreted by this operation.
func CompileNoiseRouter(ctx context.Context, docs map[string]Document, def Definition) (*NoiseRouter, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, _, ok := seedMapRelease(def.Version); !ok && !latestRelease(def.Version) {
		return nil, fmt.Errorf("noise router unsupported for %q", def.Version)
	}
	c, err := compileClimateRouter(ctx, docs, def.Seed, def.Settings, def.Version)
	if err != nil {
		return nil, err
	}
	return &NoiseRouter{program: &Generator{seed: def.Seed, version: def.Version, climate: c}}, nil
}

func (r *NoiseRouter) WithSeed(ctx context.Context, seed int64) (*NoiseRouter, error) {
	g, err := r.program.WithSeed(ctx, seed)
	if err != nil {
		return nil, err
	}
	return &NoiseRouter{program: g}, nil
}

// NoiseRouterSampler is local scratch for one worker; the router can be shared.
type NoiseRouterSampler struct {
	router  *NoiseRouter
	scratch densitySample
}

// EndIslandHeight replaces the game's shared end-island height function. The
// supplied Simplex function carries the world's correctly seeded permutation.
// Overrides run on worker-local state and must preserve Java numeric semantics.
type EndIslandHeight func(simplex func(float64, float64) float64, x, z int32) (float32, bool)

func (s *NoiseRouterSampler) SetEndIslandHeight(height EndIslandHeight) { s.scratch.endHeight = height }

func (r *NoiseRouter) NewSampler() *NoiseRouterSampler {
	n := r.program.NodeCount()
	s := &NoiseRouterSampler{router: r, scratch: densitySample{seen: make([]uint32, n), values: make([]float64, n)}}
	if r.program.climate != nil && r.program.climate.hasEnd {
		s.scratch.endCache = &endCache{}
	}
	return s
}

// Compute uses temperature, humidity, continentalness, erosion, depth, weirdness
// as indices 0..5. Coordinates are Java block coordinates, not quart cells.
func (s *NoiseRouterSampler) Compute(channel int, x, y, z int32) (float64, bool) {
	if channel < 0 || channel >= 6 {
		return 0, false
	}
	s.scratch.at(int(x), int(y), int(z))
	value := s.router.program.climate.router[channel].eval(&s.scratch)
	return value, !math.IsNaN(value) && !math.IsInf(value, 0)
}
