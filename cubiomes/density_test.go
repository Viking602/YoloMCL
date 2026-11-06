package cubiomes

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestRepeatedSplineKnotsBeforeFloatRelease(t *testing.T) {
	const curve = `{"type":"minecraft:spline","spline":{"coordinate":"test:x","points":[{"location":0,"value":0,"derivative":0},{"location":0,"value":0,"derivative":2},{"location":1,"value":1,"derivative":0},{"location":1,"value":1,"derivative":0}]}}`
	for _, tc := range []struct{ x, want float64 }{{-1, 0}, {0, 0}, {.5, .75}, {1, 1}, {2, 1}} {
		raw, _ := json.Marshal(tc.x)
		c := newDensityCompiler(context.Background(), map[string]Document{"density_function/test:x": {Data: raw}}, 0, false)
		n, err := c.compile(json.RawMessage(curve), 0)
		if err != nil {
			t.Fatal("1.18–1.21 serialized spline rejected", err)
		}
		p := densitySample{seen: make([]uint32, c.nodes), values: make([]float64, c.nodes)}
		p.at(0, 0, 0)
		if got := n.eval(&p); got != tc.want {
			t.Fatalf("x=%g got %g want %g", tc.x, got, tc.want)
		}
		if _, err := c.compile(json.RawMessage(strings.Replace(curve, `"location":1`, `"location":-1`, 1)), 0); err == nil {
			t.Fatal("descending spline knots accepted")
		}
	}
}

func TestNoiseCubiomesReference(t *testing.T) {
	b, err := os.ReadFile("testdata/noise.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Seed             string
		Legacy           bool
		Longs            []string
		Improved, Normal float64
	}
	if err := json.Unmarshal(b, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		seed, err := strconv.ParseInt(f.Seed, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		r := newNoiseRandom(seed, f.Legacy)
		for _, want := range f.Longs {
			if got := strconv.FormatUint(r.nextLong(), 10); got != want {
				t.Fatalf("seed %s legacy %v: long %s want %s", f.Seed, f.Legacy, got, want)
			}
		}
		improved := newImprovedNoise(newNoiseRandom(seed, f.Legacy)).sample(123.25, -.5, 678.75)
		first, amplitudes := -4, []float64{1, 0, .5, 1}
		if f.Legacy {
			first, amplitudes = -7, []float64{1, 1}
		}
		normal := newNormalNoise(newNoiseRandom(seed, f.Legacy), first, amplitudes, f.Legacy).sample(123.25, -.5, 678.75)
		if math.Abs(improved-f.Improved) > 1e-14 || math.Abs(normal-f.Normal) > 1e-14 {
			t.Fatalf("seed %s legacy %v: improved %.17g want %.17g; normal %.17g want %.17g", f.Seed, f.Legacy, improved, f.Improved, normal, f.Normal)
		}
	}
}

func TestNoisePositionalFactories(t *testing.T) {
	// Temperature's MD5 words are also published in cubiomes biomenoise.c.
	f := noisePositional{lo: 1, hi: 2}
	r := f.fromHash("minecraft:temperature")
	if r.lo != 0x5c7e6b29735f0d7e || r.hi != 0xf7d86f1bbc73498a {
		t.Fatalf("noise ID hash: %+v", r)
	}
	// Mth.getSeed vectors include Java int32 multiplication overflow and
	// arithmetic right shift; constants were calculated at fixed integer widths.
	for _, tc := range []struct {
		x, y, z    int32
		lo, legacy uint64
	}{
		{1, 2, 3, 0xffffe15fa16d39f9, 0xe15a7f81df94},
		{-123456, 64, 789012, 0xffffdb3c9d2f3028, 0xdb3943c3d645},
	} {
		r = f.at(tc.x, tc.y, tc.z)
		if r.lo != tc.lo || r.hi != 2 {
			t.Fatalf("positional xoroshiro: %+v", r)
		}
		legacy := noisePositional{lo: 1, legacy: true}.at(tc.x, tc.y, tc.z)
		if legacy.lo != tc.legacy || !legacy.legacy {
			t.Fatalf("positional legacy: %+v", legacy)
		}
	}
}

func TestDensityOperatorsAndSpline(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		y    int
		want float64
	}{
		{`{"type":"add","argument1":2,"argument2":3}`, 0, 5},
		{`{"type":"mul","argument1":-2,"argument2":3}`, 0, -6},
		{`{"type":"min","argument1":-2,"argument2":3}`, 0, -2},
		{`{"type":"max","argument1":-2,"argument2":3}`, 0, 3},
		{`{"type":"abs","argument":-2}`, 0, 2},
		{`{"type":"square","argument":-2}`, 0, 4},
		{`{"type":"cube","argument":-2}`, 0, -8},
		{`{"type":"half_negative","argument":-2}`, 0, -1},
		{`{"type":"quarter_negative","argument":-2}`, 0, -.5},
		{`{"type":"squeeze","argument":2}`, 0, 11.0 / 24},
		{`{"type":"clamp","input":-2,"min":-1,"max":1}`, 0, -1},
		{`{"type":"y_clamped_gradient","from_y":-64,"to_y":320,"from_value":1.5,"to_value":-1.5}`, 64, .5},
		{`{"type":"range_choice","input":1,"min_inclusive":0,"max_exclusive":1,"when_in_range":5,"when_out_of_range":7}`, 0, 7},
		{`{"type":"range_choice","input":0,"min_inclusive":0,"max_exclusive":1,"when_in_range":5,"when_out_of_range":7}`, 0, 5},
		{`{"type":"interpolated","argument":{"type":"flat_cache","argument":{"type":"cache_2d","argument":{"type":"constant","argument":-3}}}}`, 0, -3},
		// Hermite midpoint: lerp(2,4)=3 plus derivative correction .5.
		{`{"type":"spline","spline":{"coordinate":0.5,"points":[{"location":0,"value":2,"derivative":2},{"location":1,"value":4,"derivative":-2}]}}`, 0, 3.5},
		// Nested spline and endpoint extrapolation: inner=5, outer=5+2*2.
		{`{"type":"spline","spline":{"coordinate":3,"points":[{"location":1,"value":{"coordinate":0,"points":[{"location":0,"value":5,"derivative":0}]},"derivative":2}]}}`, 0, 9},
	} {
		c := newDensityCompiler(context.Background(), nil, 0, false)
		n, err := c.compile(json.RawMessage(tc.raw), 0)
		if err != nil {
			t.Fatalf("%s: %v", tc.raw, err)
		}
		p := densitySample{seen: make([]uint32, c.nodes), values: make([]float64, c.nodes)}
		p.at(0, tc.y, 0)
		if got := n.eval(&p); math.Abs(got-tc.want) > 1e-12 {
			t.Fatalf("%s = %g want %g", tc.raw, got, tc.want)
		}
	}
}

func TestDensitySplineJavaFloatRounding(t *testing.T) {
	// Expected raw float bits come from a standalone Java float Hermite spline.
	// Cover interpolation and both extrapolation branches with non-binary knots.
	for _, tc := range []struct {
		x    float32
		bits uint32
	}{{-1.111, 0x3f8fb0ee}, {-.8, 0x3f36872b}, {-.733, 0x3f1d1a2e}, {-.3, 0xbe464902}, {.1, 0xbe3020c5}, {.787, 0x3fa39dec}} {
		s := densitySpline{
			coordinate: &densityNode{id: 0, kind: "constant", p: float64(tc.x)},
			points: []densitySplinePoint{
				{location: -.8, derivative: -1.317, value: &densitySpline{constant: .713}},
				{location: .1, derivative: 2.111, value: &densitySpline{constant: -.172}},
			},
		}
		p := densitySample{seen: make([]uint32, 1), values: make([]float64, 1)}
		p.at(0, 0, 0)
		if got := math.Float32bits(s.eval(&p)); got != tc.bits {
			t.Fatalf("spline at %g: bits %#08x, want %#08x", tc.x, got, tc.bits)
		}
	}
}

func TestDensityRejectsUnsupportedCyclesAndExcessiveWork(t *testing.T) {
	docs := map[string]Document{"density_function/pack:a": {Data: json.RawMessage(`"pack:b"`)}, "density_function/pack:b": {Data: json.RawMessage(`"pack:a"`)}}
	for _, raw := range []string{`"pack:a"`, `"pack:missing"`, `{"type":"pack:java_density"}`, `{"type":"minecraft:weird_scaled_sampler"}`, `{"type":"noise","noise":"missing","xz_scale":1,"y_scale":1}`, `{"type":"spline","spline":{"coordinate":0,"points":[]}}`, strings.Repeat(`{"type":"abs","argument":`, 130) + `1` + strings.Repeat(`}`, 130)} {
		if _, err := newDensityCompiler(context.Background(), docs, 0, false).compile(json.RawMessage(raw), 0); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	c := newDensityCompiler(context.Background(), nil, 0, false)
	c.work = 32768
	if _, err := c.compile(json.RawMessage(`1`), 0); err == nil {
		t.Fatal("work budget ignored")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := newDensityCompiler(ctx, nil, 0, false).compile(json.RawMessage(`1`), 0); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestClimateTreeDistanceAndHistoricalTies(t *testing.T) {
	for _, order := range []int{6, 10} {
		var leaves []*climateNode
		for i := -50; i <= 50; i++ {
			n := &climateNode{Biome: strconv.Itoa(i)}
			n.Space[0] = climateRange{int64(i * 500), int64(i*500 + 100)}
			n.Space[1] = climateRange{int64(i * i), int64(i*i + 20)}
			leaves = append(leaves, n)
		}
		root, err := buildClimateTree(context.Background(), append([]*climateNode(nil), leaves...), order)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, leaf := range leaves {
			ids = append(ids, leaf.Biome)
		}
		compact := compactClimateTree(root, ids)
		previous, last := (*climateNode)(nil), -1
		for i := -25000; i < 25000; i += 97 {
			p := climatePoint{int64(i), 1500}
			want := int64(math.MaxInt64)
			for _, n := range leaves {
				want = min(want, n.distance(p))
			}
			if got := root.search(p, nil).distance(p); got != want {
				t.Fatalf("order %d point %v distance %d want %d", order, p, got, want)
			}
			previous = root.search(p, previous)
			last = compact.search(&p, last)
			if got := compact.biomes[compact.nodes[last].biome]; got != previous.Biome {
				t.Fatalf("compact tree changed historical search: %s != %s", got, previous.Biome)
			}
		}
	}
	a, b := &climateNode{Biome: "a"}, &climateNode{Biome: "b"}
	a.Space[0], b.Space[0] = climateRange{-10, -10}, climateRange{10, 10}
	root, _ := buildClimateTree(context.Background(), []*climateNode{a, b}, 6)
	if root.search(climatePoint{}, nil) != a || root.search(climatePoint{}, b) != b {
		t.Fatal("equal-distance lookup lost stable or previous-leaf tie breaking")
	}
	compact := compactClimateTree(root, []string{"a", "b"})
	last := compact.search(&climatePoint{10}, -1)
	if last = compact.search(&climatePoint{}, last); compact.biomes[compact.nodes[last].biome] != "b" {
		t.Fatal("compact tree lost previous-leaf tie breaking")
	}
}
