package cubiomes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

const maxDensityNodes = 4096

type densityNode struct {
	id         int
	single     bool
	kind       string
	a, b, c    *densityNode
	p, q, r, s float64
	noise      *normalNoise
	spline     *densitySpline
	end        *endIslands
	cellular   *cellularNoise
	blended    *blendedNoise
}

type densitySpline struct {
	coordinate *densityNode
	points     []densitySplinePoint
	constant   float32
}

type densitySplinePoint struct {
	location, derivative float32
	value                *densitySpline
}

// Evaluation scratch belongs to a sampling job, never to the shared compiler
// output. Memoizing the DAG avoids re-evaluating noise through nested splines.
type densitySample struct {
	x, y, z   float64
	serial    uint32
	next      uint32 // last issued serial; evalAt restores serial but never reuses one
	interp    map[interpKey]float64
	interp64  map[uint64]float64 // packed interpKey, the common case
	seen      []uint32
	values    []float64
	endCache  *endCache
	endHeight EndIslandHeight
}

func (p *densitySample) at(x, y, z int) {
	p.x, p.y, p.z = float64(x), float64(y), float64(z)
	p.serial = p.bump()
}

func (p *densitySample) bump() uint32 {
	p.next++
	if p.next == 0 {
		clear(p.seen)
		p.next++
	}
	return p.next
}

// evalAt evaluates n at another position (a cell corner or a flat-cache
// quart) without disturbing the memo of the current position.
func (p *densitySample) evalAt(n *densityNode, x, y, z float64) float64 {
	ox, oy, oz, os := p.x, p.y, p.z, p.serial
	p.x, p.y, p.z, p.serial = x, y, z, p.bump()
	v := n.eval(p)
	p.x, p.y, p.z, p.serial = ox, oy, oz, os
	return v
}

func (n *densityNode) eval(p *densitySample) float64 {
	if p.seen[n.id] == p.serial {
		return p.values[n.id]
	}
	if n.single {
		v := n.evalLatest(p)
		p.seen[n.id], p.values[n.id] = p.serial, v
		return v
	}
	v := n.p
	switch n.kind {
	case "lithostitched:fast_noise":
		v = n.cellular.sample(p.x*n.p+n.a.eval(p), p.y*n.q+n.b.eval(p), p.z*n.p+n.c.eval(p))
	case "noise":
		v = n.noise.sample(p.x*n.p, p.y*n.q, p.z*n.p)
	case "end_islands":
		v = (float64(n.end.height(int(p.x)/8, int(p.z)/8, p)) - 8) / 128
	case "interpolated":
		v = n.interpolate(p)
	case "flat_cache":
		// NoiseChunk.FlatCache fills quart columns at y 0.
		v = n.flatCache(p)
	case "old_blended_noise":
		v = n.blended.compute(p.x, p.y, p.z)
	case "weird_scaled_sampler":
		d := weirdRarity(n.a.eval(p), n.q == 2)
		v = d * math.Abs(n.noise.sample(p.x/d, p.y/d, p.z/d))
	case "shifted_noise":
		v = n.noise.sample(p.x*n.p+n.a.eval(p), p.y*n.q+n.b.eval(p), p.z*n.p+n.c.eval(p))
	case "shift":
		v = n.noise.sample(p.x*.25, p.y*.25, p.z*.25) * 4
	case "shift_a":
		v = n.noise.sample(p.x*.25, 0, p.z*.25) * 4
	case "shift_b":
		v = n.noise.sample(p.z*.25, p.x*.25, 0) * 4
	case "add":
		v = n.a.eval(p) + n.b.eval(p)
	case "mul":
		v = n.a.eval(p)
		if v != 0 {
			v *= n.b.eval(p)
		}
	case "min":
		v = math.Min(n.a.eval(p), n.b.eval(p))
	case "max":
		v = math.Max(n.a.eval(p), n.b.eval(p))
	case "abs":
		v = math.Abs(n.a.eval(p))
	case "invert":
		v = 1 / n.a.eval(p)
	case "square":
		v = n.a.eval(p)
		v *= v
	case "cube":
		v = n.a.eval(p)
		v = v * v * v
	case "half_negative":
		v = n.a.eval(p)
		if v < 0 {
			v *= .5
		}
	case "quarter_negative":
		v = n.a.eval(p)
		if v < 0 {
			v *= .25
		}
	case "squeeze":
		v = max(-1, min(1, n.a.eval(p)))
		v = v/2 - v*v*v/24
	case "clamp":
		v = max(n.p, min(n.q, n.a.eval(p)))
	case "y_clamped_gradient":
		v = noiseLerp(max(0, min(1, (p.y-n.p)/(n.q-n.p))), n.r, n.s)
	case "range_choice":
		v = n.a.eval(p)
		if v >= n.p && v < n.q {
			v = n.b.eval(p)
		} else {
			v = n.c.eval(p)
		}
	case "find_top_surface":
		v = n.findTopSurface(p)
	case "spline":
		v = float64(n.spline.eval(p))
	case "terrain_shaper_spline":
		v = float64(n.spline.eval(p) + float32(n.p))
	case "folded_float":
		w := float32(n.a.eval(p))
		folded := float32(math.Abs(float64(w)))
		folded = float32(math.Abs(float64(folded - float32(.6666667))))
		v = float64(-(folded - float32(.33333334)) * 3)
	}
	p.seen[n.id], p.values[n.id] = p.serial, v
	return v
}

func (s *densitySpline) eval(p *densitySample) float32 {
	if s.coordinate == nil {
		return s.constant
	}
	x := float32(s.coordinate.eval(p))
	i := sort.Search(len(s.points), func(i int) bool { return x < s.points[i].location }) - 1
	if i < 0 || i == len(s.points)-1 {
		point := s.points[max(0, i)]
		v := point.value.eval(p)
		if point.derivative == 0 {
			return v
		}
		return v + float32(point.derivative*(x-point.location))
	}
	a, b := s.points[i], s.points[i+1]
	span := b.location - a.location
	t := (x - a.location) / span
	v0, v1 := a.value.eval(p), b.value.eval(p)
	p0, p1 := float32(a.derivative*span)-(v1-v0), float32(-b.derivative*span)+(v1-v0)
	// Java rounds every float operation. Explicit conversions on products
	// prevent ARM64 from fusing their subsequent additions/subtractions.
	base := v0 + float32(t*(v1-v0))
	curve := p0 + float32(t*(p1-p0))
	return base + float32(float32(t*(1-t))*curve)
}

type densityCompiler struct {
	ctx          context.Context
	docs         map[string]Document
	seed         int64
	version      string
	legacy       bool
	factory      noisePositional
	refs         map[string]*densityNode
	active       map[string]bool
	intern       map[string]*densityNode
	noises       map[string]*normalNoise
	shaper       map[string]json.RawMessage
	nodes, work  int
	noiseOctaves int
	end          *endIslands
	// terrain compiles final_density: interpolated and flat_cache keep their
	// NoiseChunk meaning instead of being unwrapped for climate sampling.
	terrain      bool
	cellW, cellH int
}

func (c *densityCompiler) nodeLimit() int {
	if c.terrain {
		return 4 * maxDensityNodes
	}
	return maxDensityNodes
}

// CompileError identifies a worldgen codec that cannot be evaluated in Go.
type CompileError struct{ Code, Detail string }

func (e *CompileError) Error() string { return e.Detail }

func newDensityCompiler(ctx context.Context, docs map[string]Document, seed int64, legacy bool) *densityCompiler {
	return &densityCompiler{ctx: ctx, docs: docs, seed: seed, legacy: legacy, factory: newNoiseRandom(seed, legacy).forkPositional(), refs: map[string]*densityNode{}, active: map[string]bool{}, intern: map[string]*densityNode{}, noises: map[string]*normalNoise{}}
}

func seedMapID(id string) string {
	if id != "" && !strings.Contains(id, ":") {
		return "minecraft:" + id
	}
	return id
}

func (c *densityCompiler) budget(depth int) error {
	if err := c.ctx.Err(); err != nil {
		return err
	}
	c.work++
	work := 32768
	if c.terrain {
		work = 4 * 32768
	}
	if depth > 128 || c.work > work || c.nodes >= c.nodeLimit() {
		return errors.New("density graph exceeds evaluation limits")
	}
	return nil
}

func densityNumber(raw json.RawMessage) (float64, error) {
	var v float64
	if len(raw) == 0 || string(raw) == "null" || json.Unmarshal(raw, &v) != nil || math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e7 {
		return 0, errors.New("invalid or missing density number")
	}
	return v, nil
}

func (c *densityCompiler) compile(raw json.RawMessage, depth int) (*densityNode, error) {
	if err := c.budget(depth); err != nil {
		return nil, err
	}
	if n := c.intern[string(raw)]; n != nil {
		return n, nil
	}
	var ref string
	if json.Unmarshal(raw, &ref) == nil && ref != "" {
		ref = seedMapID(ref)
		if n := c.refs[ref]; n != nil {
			return n, nil
		}
		if c.active[ref] {
			return nil, fmt.Errorf("cyclic density reference %s", ref)
		}
		doc, ok := c.docs["density_function/"+ref]
		if !ok {
			return nil, fmt.Errorf("missing density function %s", ref)
		}
		c.active[ref] = true
		n, err := c.compile(doc.Data, depth+1)
		delete(c.active, ref)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ref, err)
		}
		c.refs[ref] = n
		return n, nil
	}
	n := &densityNode{}
	if v, err := densityNumber(raw); err == nil {
		n.p = v
	} else {
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil || fields == nil {
			return nil, errors.New("invalid density function")
		}
		var kind string
		if json.Unmarshal(fields["type"], &kind) != nil {
			return nil, errors.New("missing density type")
		}
		kind = seedMapID(kind)
		if !strings.HasPrefix(kind, "minecraft:") && kind != "lithostitched:fast_noise" {
			return nil, &CompileError{"density_unsupported", kind}
		}
		n.kind = strings.TrimPrefix(kind, "minecraft:")
		var e error
		arg := func(key string) *densityNode {
			if e != nil {
				return nil
			}
			v, err := c.compile(fields[key], depth+1)
			e = err
			return v
		}
		num := func(key string) float64 {
			if e != nil {
				return 0
			}
			v, err := densityNumber(fields[key])
			e = err
			return v
		}
		switch n.kind {
		case "lithostitched:fast_noise":
			e = c.compileCellular(fields, n, depth)
		case "interpolated", "flat_cache":
			if !c.terrain {
				return c.compile(fields["argument"], depth+1)
			}
			n.a = arg("argument")
			n.p, n.q = float64(c.cellW), float64(c.cellH)
		case "beardifier":
			// Structure terrain adaptation; no structure pieces are known here.
			n.p = 0
		case "old_blended_noise":
			n.blended = c.compileBlended(fields)
		case "weird_scaled_sampler":
			n.a = arg("input")
			n.noise, e = c.compileNoise(fields["noise"])
			var mapper string
			json.Unmarshal(fields["rarity_value_mapper"], &mapper)
			switch mapper {
			case "type_1":
				n.q = 1
			case "type_2":
				n.q = 2
			default:
				e = errors.New("unknown rarity_value_mapper " + mapper)
			}
		case "cache_2d", "cache_once", "cache_all_in_cell", "blend_density":
			// RandomState's climate sampler unwraps markers. New chunks have no
			// blending data; this is not a surface/terrain interpolation engine.
			return c.compile(fields["argument"], depth+1)
		case "constant":
			n.p = num("argument")
		case "end_islands":
			if c.end == nil {
				c.end = newEndIslands(c.seed)
			}
			n.end = c.end
		case "blend_alpha":
			n.p = 1
		case "blend_offset":
			n.p = 0
		case "add", "mul", "min", "max":
			n.a, n.b = arg("argument1"), arg("argument2")
		case "abs", "invert", "square", "cube", "half_negative", "quarter_negative", "squeeze":
			n.a = arg("argument")
		case "clamp":
			n.a, n.p, n.q = arg("input"), num("min"), num("max")
			if n.p > n.q {
				e = errors.New("inverted clamp")
			}
		case "y_clamped_gradient":
			n.p, n.q, n.r, n.s = num("from_y"), num("to_y"), num("from_value"), num("to_value")
			if n.p >= n.q {
				e = errors.New("invalid gradient heights")
			}
		case "range_choice":
			n.a, n.b, n.c = arg("input"), arg("when_in_range"), arg("when_out_of_range")
			n.p, n.q = num("min_inclusive"), num("max_exclusive")
		case "interval_select":
			return c.compileIntervalSelect(raw, fields, depth)
		case "find_top_surface":
			n.a, n.b = arg("density"), arg("upper_bound")
			n.p, n.q = num("lower_bound"), num("cell_height")
			if e == nil && (n.p != math.Trunc(n.p) || n.q != math.Trunc(n.q) || n.q <= 0) {
				e = errors.New("invalid find_top_surface bounds or cell height")
			}
		case "noise", "shifted_noise", "shift", "shift_a", "shift_b":
			field := "noise"
			if n.kind == "shift" || n.kind == "shift_a" || n.kind == "shift_b" {
				field = "argument"
			}
			n.noise, e = c.compileNoise(fields[field])
			if n.kind == "noise" || n.kind == "shifted_noise" {
				n.p, n.q = num("xz_scale"), num("y_scale")
			}
			if n.kind == "shifted_noise" {
				n.a, n.b, n.c = arg("shift_x"), arg("shift_y"), arg("shift_z")
			}
		case "spline":
			n.spline, e = c.compileSpline(fields["spline"], depth+1, nil)
		case "terrain_shaper_spline":
			var spline string
			if json.Unmarshal(fields["spline"], &spline) != nil || len(c.shaper[spline]) == 0 {
				return nil, errors.New("missing legacy terrain shaper spline")
			}
			n.a, n.b, n.c = arg("continentalness"), arg("erosion"), arg("weirdness")
			if e != nil {
				return nil, e
			}
			folded := &densityNode{id: c.nodes, kind: "folded_float", a: n.c}
			c.nodes++
			n.spline, e = c.compileSpline(c.shaper[spline], depth+1, map[string]*densityNode{"continents": n.a, "erosion": n.b, "weirdness": n.c, "ridges": folded})
			if spline == "offset" {
				n.p = float64(float32(-.50375))
			}
		default:
			return nil, &CompileError{"density_unsupported", kind}
		}
		if e != nil {
			return nil, e
		}
	}
	if c.nodes >= c.nodeLimit() {
		return nil, errors.New("density graph exceeds evaluation limits")
	}
	n.id = c.nodes
	c.nodes++
	c.intern[string(raw)] = n
	return n, nil
}

func (c *densityCompiler) compileNoise(raw json.RawMessage) (*normalNoise, error) {
	var id string
	if json.Unmarshal(raw, &id) != nil || id == "" {
		return nil, errors.New("noise must reference a registry ID")
	}
	id = seedMapID(id)
	if n := c.noises[id]; n != nil {
		return n, nil
	}
	var data struct {
		FirstOctave *int      `json:"firstOctave"`
		Amplitudes  []float64 `json:"amplitudes"`
		BaseOctave  *int      `json:"base_octave"`
		OctaveCount *int      `json:"octave_count"`
	}
	doc, ok := c.docs["noise/"+id]
	if !ok || json.Unmarshal(doc.Data, &data) != nil {
		return nil, fmt.Errorf("invalid or missing noise %s", id)
	}
	if data.FirstOctave == nil && latestFloatRelease(c.version) && data.BaseOctave != nil && data.OctaveCount != nil {
		return nil, &CompileError{Code: "density_unsupported", Detail: "26.3 noise normalization: " + id}
	}
	if data.FirstOctave == nil || len(data.Amplitudes) == 0 || len(data.Amplitudes) > 64 || *data.FirstOctave < -64 || *data.FirstOctave > 32 {
		return nil, fmt.Errorf("invalid or missing noise %s", id)
	}
	for _, a := range data.Amplitudes {
		if math.Abs(a) > 1e4 {
			return nil, fmt.Errorf("noise amplitude limit: %s", id)
		}
	}
	r, legacyNether := c.factory.fromHash(noiseSeedID(id)), false
	if c.legacy {
		alias := latestLegacyNoiseAlias(c.version, id)
		switch alias {
		case "minecraft:temperature", "minecraft:vegetation":
			seed := c.seed
			if alias == "minecraft:vegetation" {
				seed++
			}
			r, legacyNether = newNoiseRandom(seed, true), true
			first := -7
			data.FirstOctave, data.Amplitudes = &first, []float64{1, 1}
		case "minecraft:offset":
			first := 0
			data.FirstOctave, data.Amplitudes = &first, []float64{0}
		}
	}
	for _, amplitude := range data.Amplitudes {
		if amplitude != 0 {
			c.noiseOctaves += 2
		}
	}
	if c.noiseOctaves > 2048 && !c.terrain || c.noiseOctaves > 8192 {
		return nil, errors.New("climate noise octave budget exceeded")
	}
	n := newNormalNoise(r, *data.FirstOctave, data.Amplitudes, legacyNether)
	n.spec = &noiseSpec{id: id, first: *data.FirstOctave, amplitudes: data.Amplitudes, legacy: c.legacy, legacyNether: legacyNether}
	c.noises[id] = n
	return n, nil
}

func (c *densityCompiler) compileSpline(raw json.RawMessage, depth int, coordinates map[string]*densityNode) (*densitySpline, error) {
	if err := c.budget(depth); err != nil {
		return nil, err
	}
	if v, err := densityNumber(raw); err == nil {
		return &densitySpline{constant: float32(v)}, nil
	}
	var data struct {
		Coordinate json.RawMessage `json:"coordinate"`
		Points     []struct {
			Location, Derivative *float32
			Value                json.RawMessage
		} `json:"points"`
	}
	if json.Unmarshal(raw, &data) != nil || len(data.Points) == 0 || len(data.Points) > 256 {
		return nil, errors.New("invalid spline points")
	}
	s := &densitySpline{}
	var err error
	var coordinate string
	if coordinates != nil && json.Unmarshal(data.Coordinate, &coordinate) == nil {
		s.coordinate = coordinates[coordinate]
		if s.coordinate == nil {
			return nil, errors.New("invalid terrain shaper coordinate")
		}
	} else {
		s.coordinate, err = c.compile(data.Coordinate, depth+1)
	}
	if err != nil {
		return nil, err
	}
	for i, p := range data.Points {
		// The serialized CubicSpline codec accepts equal locations. They encode
		// different left/right derivatives (used by Tectonic on 1.21.1 too).
		// eval's upper-bound search selects the last equal knot, avoiding a
		// zero-width interval without changing the pack's piecewise function.
		if p.Location == nil || p.Derivative == nil || (i > 0 && *p.Location < s.points[i-1].location) || math.Abs(float64(*p.Location)) > 1e7 || math.Abs(float64(*p.Derivative)) > 1e7 {
			return nil, errors.New("invalid spline knot")
		}
		v, err := c.compileSpline(p.Value, depth+1, coordinates)
		if err != nil {
			return nil, err
		}
		s.points = append(s.points, densitySplinePoint{*p.Location, *p.Derivative, v})
	}
	return s, nil
}
