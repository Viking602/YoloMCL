package cubiomes

import (
	"context"
	"errors"
	"math"
)

// SurfaceRange samples an estimated surface in Z/X order at block coordinates.
// SeaLevel selects the biome at the water surface in submerged columns.
type SurfaceRange struct {
	X, Z, SizeX, SizeZ, Step, SeaLevel int
	Voronoi                            bool
}

// SurfaceSample contains a terrain estimate and the biome at that height (or
// SeaLevel + 1 underwater). Height is not a generated Minecraft heightmap:
// caves, overhangs, structures, surface blocks and final density are not sampled.
type SurfaceSample struct {
	Height float32
	Biome  BiomeID
}

// SupportsSurface reports whether the depth graph has the Overworld vertical
// gradient used by cubiomes' modern mapApproxHeight estimator. Other sources
// remain usable through GenerateIDs; zero depth must not become a flat ocean.
func (g *Generator) SupportsSurface() bool { return g.surface != nil }

// A positive upper cap (or negative lower cap) leaves the depth zero crossing
// unchanged. For height estimation, use its underlying terrain gradient;
// climate evaluation still uses the complete original graph. Tectonic 3.0.29
// uses min(tectonic:depth, range_choice(y, .8, 1.3)) for biome depth alone.
func surfaceHeightRoot(root *densityNode) *densityNode {
	if root == nil {
		return nil
	}
	if root.kind == "add" {
		if (root.a.kind == "constant" || root.a.kind == "") && root.a.p == 0 {
			return surfaceHeightRoot(root.b)
		}
		if (root.b.kind == "constant" || root.b.kind == "") && root.b.p == 0 {
			return surfaceHeightRoot(root.a)
		}
	}
	if surfaceDepth(root) {
		return root
	}
	switch root.kind {
	case "min", "max":
		sign := 1
		if root.kind == "max" {
			sign = -1
		}
		if surfaceCapSign(root.b) == sign {
			return surfaceHeightRoot(root.a)
		}
		if surfaceCapSign(root.a) == sign {
			return surfaceHeightRoot(root.b)
		}
	case "clamp":
		if root.p < 0 && root.q > 0 {
			return surfaceHeightRoot(root.a)
		}
	case "range_choice":
		// Older Tectonic packs cap depth only above a given Y. Both branches
		// retain the same underlying zero crossing after removing safe caps.
		a, b := surfaceHeightRoot(root.b), surfaceHeightRoot(root.c)
		if a != nil && a == b {
			return a
		}
	}
	return nil
}

// Prove a bound stays strictly on one side of zero, without evaluating a seed
// or guessing the range of noise. In a range_choice the input may depend on Y;
// both output branches must have the same proven sign.
func surfaceCapSign(n *densityNode) int {
	return surfaceCapSignAt(n, make(map[*densityNode]int))
}

func surfaceCapSignAt(n *densityNode, memo map[*densityNode]int) (sign int) {
	if n == nil {
		return 0
	}
	if sign, ok := memo[n]; ok {
		return sign
	}
	defer func() { memo[n] = sign }()
	value := func(v float64) int {
		if n.single {
			v = float64(float32(v))
		}
		if v > 0 {
			return 1
		}
		if v < 0 {
			return -1
		}
		return 0
	}
	switch n.kind {
	case "", "constant":
		return value(n.p)
	case "range_choice":
		if a, b := surfaceCapSignAt(n.b, memo), surfaceCapSignAt(n.c, memo); a == b {
			return a
		}
	case "clamp":
		if value(n.p) > 0 {
			return 1
		}
		if value(n.q) < 0 {
			return -1
		}
	case "y_clamped_gradient":
		if a, b := value(n.r), value(n.s); a == b {
			return a
		}
	}
	return 0
}

// surfaceDepth accepts a canonical -1/128 Y gradient plus any Y-independent
// graph. This includes datapack spline/noise changes without assuming arbitrary
// 3D custom depth functions are meaningful surface height estimators.
func surfaceDepth(root *densityNode) bool {
	type result struct {
		slope float64
		ok    bool
	}
	memo := map[*densityNode]result{}
	var node func(*densityNode) result
	var spline func(*densitySpline) bool
	spline = func(s *densitySpline) bool {
		if s == nil || s.coordinate == nil {
			return true
		}
		r := node(s.coordinate)
		if !r.ok || r.slope != 0 {
			return false
		}
		for _, p := range s.points {
			if !spline(p.value) {
				return false
			}
		}
		return true
	}
	node = func(n *densityNode) result {
		if n == nil {
			return result{ok: true}
		}
		if r, ok := memo[n]; ok {
			return r
		}
		a, b, c := node(n.a), node(n.b), node(n.c)
		r := result{ok: a.ok && b.ok && c.ok && a.slope == 0 && b.slope == 0 && c.slope == 0}
		switch n.kind {
		case "y_clamped_gradient":
			r = result{(n.s - n.r) / (n.q - n.p), n.p <= 0 && n.q >= 0 && n.q > n.p}
		case "add":
			r = result{a.slope + b.slope, a.ok && b.ok}
		case "noise", "shifted_noise":
			r.ok = r.ok && n.q == 0
		case "shift":
			r.ok = false
		case "spline", "terrain_shaper_spline":
			r.ok = r.ok && spline(n.spline)
		}
		memo[n] = r
		return r
	}
	r := node(root)
	return r.ok && r.slope == -1.0/128
}

// Direct-mapped caches are bounded regardless of viewport size or navigation.
// Only coordinate-dependent climate vectors are cached, never winning biomes;
// every raster pixel still participates in history-dependent tree lookup.
type surfaceScratch struct {
	heights  [4096]surfaceHeightCell
	climates [4096]surfaceClimateCell
}

// Height and surface biome queries often share X/Z but use different Y. Keep
// already evaluated column noise and invalidate exactly the nodes depending on
// Y. This uses dependency analysis, never a rounded/linear climate substitute.
func surfaceYNodes(roots [6]*densityNode) []int {
	memo := map[*densityNode]bool{}
	var ids []int
	var node func(*densityNode) bool
	var spline func(*densitySpline) bool
	spline = func(s *densitySpline) bool {
		if s == nil || s.coordinate == nil {
			return false
		}
		dependent := node(s.coordinate)
		for _, p := range s.points {
			if spline(p.value) {
				dependent = true
			}
		}
		return dependent
	}
	node = func(n *densityNode) bool {
		if n == nil {
			return false
		}
		if dependent, ok := memo[n]; ok {
			return dependent
		}
		a, b, c, sp := node(n.a), node(n.b), node(n.c), spline(n.spline)
		dependent := a || b || c || sp
		switch n.kind {
		case "y_clamped_gradient":
			dependent = n.r != n.s
		case "noise", "shifted_noise":
			dependent = dependent || n.q != 0
		case "shift":
			dependent = true
		}
		memo[n] = dependent
		if dependent {
			ids = append(ids, n.id)
		}
		return dependent
	}
	for _, root := range roots {
		node(root)
	}
	return ids
}

func (s *Sampler) surfaceTarget(x, y, z int) (climatePoint, bool) {
	p := &s.scratch
	if p.serial != 0 && p.x == float64(x*4) && p.z == float64(z*4) {
		if p.y != float64(y*4) {
			for _, id := range s.g.surfaceY {
				p.seen[id] = 0
			}
			p.y = float64(y * 4)
		}
	} else {
		p.at(x*4, y*4, z*4)
	}
	return s.g.climate.targetAt(p)
}

type surfaceHeightCell struct {
	x, z   int
	height float32
	valid  bool
}

type surfaceClimateCell struct {
	x, y, z int
	point   climatePoint
	region  int
	valid   bool
}

func surfaceHash(x, y, z int) uint32 {
	h := uint32(x)*0x9e3779b1 ^ uint32(z)*0x85ebca77 ^ uint32(y)*0xc2b2ae3d
	h ^= h >> 16
	return h & 4095
}

func (s *Sampler) quartHeight(x, z int) (float32, bool) {
	cell := &s.surface.heights[surfaceHash(x, 0, z)]
	if cell.valid && cell.x == x && cell.z == z {
		return cell.height, true
	}
	s.scratch.at(x*4, 0, z*4)
	depth := s.g.surface.eval(&s.scratch)
	if math.IsNaN(depth) || math.IsInf(depth, 0) || math.Abs(depth) > 10000 {
		return 0, false
	}
	// Cubitect/cubiomes generator.c mapApproxHeight (MIT): quantized depth / 76.
	h := float32(float64(climateQuantize(depth)) / 76)
	*cell = surfaceHeightCell{x: x, z: z, height: h, valid: true}
	return h, true
}

// EstimateSurfaceHeight interpolates the four surrounding quart height samples.
// At quart coordinates it uses the cubiomes modern depth estimator, evaluated
// with the selected Minecraft version's native density graph and precision.
// It does not reset biome lookup history. Coordinates must fit Java int32.
func (s *Sampler) EstimateSurfaceHeight(x, z int) (float32, bool) {
	if s.g.surface == nil || int64(x) < math.MinInt32+4 || int64(x) > math.MaxInt32-4 || int64(z) < math.MinInt32+4 || int64(z) > math.MaxInt32-4 {
		return 0, false
	}
	if s.surface == nil {
		s.surface = new(surfaceScratch)
	}
	qx, qz := x>>2, z>>2
	a, ok := s.quartHeight(qx, qz)
	if !ok {
		return 0, false
	}
	b, c, d := a, a, a
	if x&3 != 0 {
		b, ok = s.quartHeight(qx+1, qz)
		if !ok {
			return 0, false
		}
	}
	if z&3 != 0 {
		c, ok = s.quartHeight(qx, qz+1)
		if !ok {
			return 0, false
		}
		d = c
		if x&3 != 0 {
			d, ok = s.quartHeight(qx+1, qz+1)
			if !ok {
				return 0, false
			}
		}
	}
	fx, fz := float64(x&3)/4, float64(z&3)/4
	return float32(noiseLerp(fz, noiseLerp(fx, float64(a), float64(b)), noiseLerp(fx, float64(c), float64(d)))), true
}

// GenerateSurface resets tree history once, preserves ties in raster order and
// checks cancellation every 64 pixels. Repeated calls reuse bounded scratch.
// Output may be partially written on error. No surface is fabricated for an
// unsupported biome source; callers can render its existing biome slice.
func (s *Sampler) GenerateSurface(ctx context.Context, r SurfaceRange, dst []SurfaceSample) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	const maxCells = 1 << 20
	if s.g.surface == nil {
		return errors.New("surface estimation is unavailable for this depth graph")
	}
	if r.SizeX < 1 || r.SizeZ < 1 || r.SizeX > maxCells || r.SizeZ > maxCells/r.SizeX || len(dst) < r.SizeX*r.SizeZ || r.Step < 1 || r.SeaLevel < -2048 || r.SeaLevel > 2047 {
		return errors.New("invalid surface range")
	}
	valid := func(start, count int) bool {
		return int64(start) >= math.MinInt32+4 && int64(start) <= math.MaxInt32-4 && (count == 1 || int64(r.Step) <= (math.MaxInt32-4-int64(start))/int64(count-1))
	}
	if !valid(r.X, r.SizeX) || !valid(r.Z, r.SizeZ) {
		return errors.New("surface coordinates exceed Java integer range")
	}
	s.Reset()
	for iz := range r.SizeZ {
		for ix := range r.SizeX {
			i := iz*r.SizeX + ix
			if i&63 == 0 && ctx.Err() != nil {
				return ctx.Err()
			}
			x, z := r.X+ix*r.Step, r.Z+iz*r.Step
			h, ok := s.EstimateSurfaceHeight(x, z)
			if !ok {
				return errors.New("invalid surface height")
			}
			y := int(math.Floor(max(float64(h), float64(r.SeaLevel)))) + 1
			if r.Voronoi {
				x, y, z = s.voronoi(x, y, z)
			} else {
				x, y, z = x>>2, y>>2, z>>2
			}
			cell := &s.surface.climates[surfaceHash(x, y, z)]
			if !cell.valid || cell.x != x || cell.y != y || cell.z != z {
				p, ok := s.surfaceTarget(x, y, z)
				if !ok {
					return errors.New("invalid surface climate")
				}
				*cell = surfaceClimateCell{x: x, y: y, z: z, point: p, valid: true}
				if s.tb != nil {
					cell.region = s.tb.region(s.g.tb, len(s.tb.layers)-1, int32(x), int32(z))
				}
			}
			var id BiomeID
			if s.tb != nil {
				id = s.terraBlenderClimate(&cell.point, cell.region)
			} else {
				c := s.g.climate
				s.previous = c.root.search(&cell.point, s.previous)
				id = s.g.ids[c.root.nodes[s.previous].biome]
			}
			if s.g.injection != nil {
				id = s.injectBiome(id, &cell.point, x*4, y*4, z*4)
			}
			dst[i] = SurfaceSample{Height: h, Biome: id}
		}
	}
	return ctx.Err()
}
