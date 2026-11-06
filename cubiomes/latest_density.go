package cubiomes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"strings"
	"sync"
)

func latestFloatRelease(version string) bool { return version == "26.3" }

func (n *densityNode) evalLatest(p *densitySample) float64 {
	v := float32(n.p)
	arg := func(n *densityNode) float32 { return float32(n.eval(p)) }
	switch n.kind {
	case "noise":
		v = n.noise.latestSample(p.x*n.p, p.y*n.q, p.z*n.p)
	case "shifted_noise":
		v = n.noise.latestSample(p.x*n.p+float64(arg(n.a)), p.y*n.q+float64(arg(n.b)), p.z*n.p+float64(arg(n.c)))
	case "shift":
		v = n.noise.latestSample(p.x*.25, p.y*.25, p.z*.25) * 4
	case "shift_a":
		v = n.noise.latestSample(p.x*.25, 0, p.z*.25) * 4
	case "shift_b":
		v = n.noise.latestSample(p.z*.25, p.x*.25, 0) * 4
	case "add":
		v = arg(n.a) + arg(n.b)
	case "mul":
		v = arg(n.a) * arg(n.b)
	case "min":
		v = min(arg(n.a), arg(n.b))
	case "max":
		v = max(arg(n.a), arg(n.b))
	case "abs":
		v = float32(math.Abs(float64(arg(n.a))))
	case "square":
		v = arg(n.a)
		v *= v
	case "cube":
		v = arg(n.a)
		v = float32(v*v) * v
	case "half_negative", "quarter_negative":
		v = arg(n.a)
		if v < 0 {
			if n.kind == "half_negative" {
				v *= .5
			} else {
				v *= .25
			}
		}
	case "clamp":
		v = max(float32(n.p), min(float32(n.q), arg(n.a)))
	case "y_clamped_gradient":
		a := float32(p.y) - float32(n.p)
		b := float32(n.q) - float32(n.p)
		v = latestLerp(max(float32(0), min(float32(1), a/b)), float32(n.r), float32(n.s))
	case "range_choice":
		v = arg(n.a)
		if v >= float32(n.p) && v < float32(n.q) {
			v = arg(n.b)
		} else {
			v = arg(n.c)
		}
	case "squeeze":
		v = max(float32(-1), min(float32(1), arg(n.a)))
		v = v/2 - float32(float32(v*v)*v)/24
	case "spline":
		v = n.spline.eval(p)
	case "end_islands":
		v = (n.end.latestHeight(int(p.x)/8, int(p.z)/8, p) - 8) / 128
	}
	return float64(v)
}

// The 26.3 End replaces the overflowing central island radius with a distance
// function. Far islands keep the same seeded lattice. The bounded factor cache
// is shared with the older implementation, but the radius no longer wraps.
func (n *endIslands) latestHeight(x, z int, p *densitySample) float32 {
	if p.endHeight != nil {
		value, ok := p.endHeight(n.simplex, int32(x), int32(z))
		if !ok {
			return float32(math.NaN())
		}
		return value
	}
	hx, hz, ox, oz := x/2, z/2, x%2, z%2
	height := max(float32(-100), min(float32(80), 100-float32(math.Hypot(float64(x*8), float64(z*8)))))
	if p.endCache == nil {
		p.endCache = &endCache{}
	}
	for dz := -12; dz <= 12; dz++ {
		row, mask := n.factorRow(p.endCache, hx-12, hx+13, hz+dz, false)
		for mask != 0 {
			dx := bits.TrailingZeros64(mask) - 12
			mask &= mask - 1
			a, b := float32(ox-dx*2), float32(oz-dz*2)
			v := 100 - float32(math.Sqrt(float64(a*a+b*b)))*row.values[(hx+dx)&63]
			height = max(height, max(float32(-100), min(float32(80), v)))
		}
	}
	return height
}

var latestNoiseReferences struct {
	sync.Once
	old, current map[string]Document
	err          error
}

func latestReferenceDocuments() (map[string]Document, map[string]Document, error) {
	latestNoiseReferences.Do(func() {
		for _, version := range []string{"26.2", "26.3"} {
			docs, err := loadLatestBuiltinDocuments(version)
			if err != nil {
				latestNoiseReferences.err = err
				return
			}
			m := make(map[string]Document, len(docs))
			for _, d := range docs {
				m[d.Kind+"/"+d.ID] = d
			}
			if version == "26.2" {
				latestNoiseReferences.old = m
			} else {
				latestNoiseReferences.current = m
			}
		}
	})
	return latestNoiseReferences.old, latestNoiseReferences.current, latestNoiseReferences.err
}

func latestEqualJSON(a, b json.RawMessage) bool {
	var aa, bb any
	if json.Unmarshal(a, &aa) != nil || json.Unmarshal(b, &bb) != nil {
		return false
	}
	x, _ := json.Marshal(aa)
	y, _ := json.Marshal(bb)
	return bytes.Equal(x, y)
}

func latestTranslateValue(value any) (any, error) {
	switch v := value.(type) {
	case []any:
		out := make([]any, len(v))
		for i, x := range v {
			y, err := latestTranslateValue(x)
			if err != nil {
				return nil, err
			}
			out[i] = y
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, x := range v {
			y, err := latestTranslateValue(x)
			if err != nil {
				return nil, err
			}
			out[k] = y
		}
		typ, _ := v["type"].(string)
		kind := strings.TrimPrefix(typ, "minecraft:")
		binary := func(kind string, a, b any) any {
			return map[string]any{"type": "minecraft:" + kind, "argument1": a, "argument2": b}
		}
		switch kind {
		case "add", "mul", "min", "max":
			out["argument1"], out["argument2"] = out["left"], out["right"]
		case "sub":
			return binary("add", out["left"], binary("mul", float64(-1), out["right"])), nil
		case "lerp":
			return binary("add", out["first"], binary("mul", out["alpha"], binary("add", out["second"], binary("mul", float64(-1), out["first"])))), nil
		case "abs", "square", "cube", "half_negative", "quarter_negative", "squeeze", "blend_density", "interpolated":
			out["argument"] = out["input"]
		case "cache":
			out["type"], out["argument"] = "minecraft:cache_once", out["input"]
		case "constant":
			out["argument"] = out["value"]
		case "shift", "shift_a", "shift_b":
			out["argument"] = out["noise"]
		case "noise":
			out["type"] = "minecraft:shifted_noise"
			for _, k := range []string{"shift_x", "shift_y", "shift_z"} {
				if _, ok := out[k]; !ok {
					out[k] = float64(0)
				}
			}
		case "gradient":
			if out["axis"] != "y" || out["tiling"] != nil && out["tiling"] != "clamp_to_edge" {
				return nil, &CompileError{Code: "density_function_unsupported", Detail: "26.3 gradient axis or tiling requires runtime sampler"}
			}
			out["type"], out["from_y"], out["to_y"] = "minecraft:y_clamped_gradient", out["from_coordinate"], out["to_coordinate"]
		}
		return out, nil
	default:
		return value, nil
	}
}

func latestTranslateRaw(raw json.RawMessage) (json.RawMessage, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	v, err := latestTranslateValue(value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

func latestCompiler(ctx context.Context, docs map[string]Document, seed int64, legacy bool) (*densityCompiler, error) {
	old, current, err := latestReferenceDocuments()
	if err != nil {
		return nil, err
	}
	converted := make(map[string]Document, len(docs))
	for key, doc := range docs {
		switch doc.Kind {
		case "density_function":
			if doc.ID == "minecraft:end/islands" && latestEqualJSON(doc.Data, current[key].Data) {
				doc.Data = json.RawMessage(`{"type":"minecraft:end_islands"}`)
			} else {
				doc.Data, err = latestTranslateRaw(doc.Data)
				// Unsupported functions outside the six-channel dependency closure
				// do not make a valid biome source unusable.
				if err != nil {
					doc = docs[key]
				}
			}
		case "noise":
			if previous, ok := old[key]; ok && latestEqualJSON(doc.Data, current[key].Data) {
				doc.Data = previous.Data
			} else if legacy, ok := latestLegacyNoiseDefinition(doc.Data); ok {
				doc.Data = legacy
			}
		}
		converted[key] = doc
	}
	c := newDensityCompiler(ctx, converted, seed, legacy)
	c.version = "26.3"
	return c, nil
}

// The explicitly selected legacy normalization retains the former octave
// weights. Unsupported new normalization modes remain compile errors, rather
// than being guessed. Sampling still uses 26.3 single-precision operations.
func latestLegacyNoiseDefinition(raw json.RawMessage) (json.RawMessage, bool) {
	var data struct {
		BaseOctave    int       `json:"base_octave"`
		Count         int       `json:"octave_count"`
		BaseAmplitude *float64  `json:"base_amplitude"`
		Modifiers     []float64 `json:"amplitude_modifiers"`
		Normalize     any       `json:"normalize"`
	}
	if json.Unmarshal(raw, &data) != nil || data.Normalize != "legacy" || data.Count < 1 || data.Count > 32 || data.BaseOctave < -64 || data.BaseOctave > 64 || data.BaseAmplitude != nil && *data.BaseAmplitude != 1 {
		return nil, false
	}
	if data.Modifiers == nil {
		data.Modifiers = make([]float64, data.Count)
		for i := range data.Modifiers {
			data.Modifiers[i] = 1
		}
	}
	if len(data.Modifiers) != data.Count {
		return nil, false
	}
	for _, v := range data.Modifiers {
		if math.IsInf(v, 0) || math.IsNaN(v) {
			return nil, false
		}
	}
	result, err := json.Marshal(map[string]any{"firstOctave": data.BaseOctave, "amplitudes": data.Modifiers})
	return result, err == nil
}

func latestMarkNodes(c *densityCompiler) {
	for _, n := range c.intern {
		n.single = true
	}
	for _, n := range c.refs {
		n.single = true
	}
}

func latestSettings(docs map[string]Document, settings json.RawMessage) (bool, map[string]json.RawMessage, error) {
	var ref string
	if json.Unmarshal(settings, &ref) == nil && ref != "" {
		settings = docs["noise_settings/"+seedMapID(ref)].Data
	}
	var data struct {
		Legacy bool                       `json:"legacy_random_source"`
		Router map[string]json.RawMessage `json:"noise_router"`
	}
	if json.Unmarshal(settings, &data) != nil || data.Router == nil {
		return false, nil, errors.New("invalid or missing latest noise settings")
	}
	return data.Legacy, data.Router, nil
}

func compileLatestClimateRouter(ctx context.Context, docs map[string]Document, seed int64, settings json.RawMessage) (*climateSampler, error) {
	legacy, router, err := latestSettings(docs, settings)
	if err != nil {
		return nil, err
	}
	c, err := latestCompiler(ctx, docs, seed, legacy)
	if err != nil {
		return nil, err
	}
	s := &climateSampler{}
	for i, key := range []string{"temperature", "vegetation", "continents", "erosion", "depth", "ridges"} {
		raw, err := latestTranslateRaw(router[key])
		if err != nil {
			return nil, err
		}
		n, err := c.compile(raw, 0)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		s.router[i] = n
	}
	latestMarkNodes(c)
	s.nodes = c.nodes
	s.hasEnd = c.end != nil
	return s, nil
}

func compileLatestEndBiome(ctx context.Context, docs map[string]Document, seed int64, settings json.RawMessage) (*endBiomeSampler, error) {
	legacy, router, err := latestSettings(docs, settings)
	if err != nil {
		return nil, err
	}
	c, err := latestCompiler(ctx, docs, seed, legacy)
	if err != nil {
		return nil, err
	}
	raw, err := latestTranslateRaw(router["erosion"])
	if err != nil {
		return nil, err
	}
	n, err := c.compile(raw, 0)
	if err != nil {
		return nil, err
	}
	latestMarkNodes(c)
	return &endBiomeSampler{erosion: n, nodes: c.nodes}, nil
}
