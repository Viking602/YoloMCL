package cubiomes

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"
)

const rawRouterFixture = `{"noise_router":{"temperature":0.0000123456789,"vegetation":0,"continents":0,"erosion":{"type":"y_clamped_gradient","from_y":0,"to_y":100,"from_value":0,"to_value":1},"depth":0,"ridges":0}}`

func TestRawRouterPreservesBlockCoordinatesAndPrecision(t *testing.T) {
	r, err := CompileNoiseRouter(context.Background(), nil, Definition{Version: "1.21.1", Settings: json.RawMessage(rawRouterFixture)})
	if err != nil {
		t.Fatal(err)
	}
	s := r.NewSampler()
	if value, ok := s.Compute(0, 0, 0, 0); !ok || value != 0.0000123456789 {
		t.Fatalf("raw precision lost: %.17g %v", value, ok)
	}
	for _, y := range []int32{-1, 1, 3, 4, 99, 101} {
		value, ok := s.Compute(3, 0, y, 0)
		want := min(1., max(0., float64(y)/100))
		if !ok || math.Abs(value-want) > 1e-15 {
			t.Fatalf("y=%d: %g want %g", y, value, want)
		}
	}
	if _, ok := s.Compute(6, 0, 0, 0); ok {
		t.Fatal("invalid channel accepted")
	}
}

type coordinateBiomeSource struct{ fail bool }
type coordinateBiomeSampler struct{ fail bool }

func (coordinateBiomeSource) Biomes() []string { return []string{"test:negative", "test:positive"} }
func (c coordinateBiomeSource) NewBiomeSampler(context.Context, *NoiseRouterSampler) (CustomBiomeSampler, error) {
	return coordinateBiomeSampler(c), nil
}
func (c coordinateBiomeSampler) NoiseBiome(ctx context.Context, x, y, z int32) (string, error) {
	if c.fail {
		return "", errors.New("source rejected input")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if x < 0 {
		return "test:negative", nil
	}
	return "test:positive", nil
}

func TestCustomSourceCoordinatesReseedingAndFailure(t *testing.T) {
	docs := map[string]Document{"biome/test:negative": {ID: "test:negative"}, "biome/test:positive": {ID: "test:positive"}}
	def := Definition{Version: "1.21.1", Settings: json.RawMessage(rawRouterFixture)}
	for _, failure := range []bool{false, true} {
		g, err := CompileCustomBiomeSource(context.Background(), docs, def, coordinateBiomeSource{failure})
		if err != nil {
			t.Fatal(err)
		}
		g, err = g.WithSeed(context.Background(), 42)
		if err != nil {
			t.Fatal(err)
		}
		s := g.NewSampler()
		out := make([]string, 3)
		err = s.Generate(context.Background(), Range{X: -1, SizeX: 3, SizeY: 1, SizeZ: 1, StepXZ: 1, StepY: 1}, out)
		if failure {
			if err == nil || err.Error() != "source rejected input" {
				t.Fatalf("execution failure was hidden: %v", err)
			}
			continue
		}
		if err != nil || out[0] != "test:negative" || out[1] != "test:positive" || out[2] != "test:positive" {
			t.Fatalf("wrong containing quart cells: %v %v", out, err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err = s.Generate(ctx, Range{SizeX: 1, SizeY: 1, SizeZ: 1, StepXZ: 1, StepY: 1}, out); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if _, ok := s.NextNoiseBiome(0, 0, 0); !ok {
			t.Fatal("cancelled request poisoned reusable worker")
		}
	}
	delete(docs, "biome/test:positive")
	if _, err := CompileCustomBiomeSource(context.Background(), docs, def, coordinateBiomeSource{}); err == nil {
		t.Fatal("absent datapack holder accepted")
	}
}

func TestRawEndOverrideUsesSeededNoiseAndRejectsFailedEvaluation(t *testing.T) {
	settings := json.RawMessage(`{"noise_router":{"temperature":0,"vegetation":0,"continents":0,"erosion":{"type":"end_islands"},"depth":0,"ridges":0}}`)
	r, err := CompileNoiseRouter(context.Background(), nil, Definition{Version: "1.21.1", Seed: 7, Settings: settings})
	if err != nil {
		t.Fatal(err)
	}
	s := r.NewSampler()
	calls := 0
	s.SetEndIslandHeight(func(noise func(float64, float64) float64, x, z int32) (float32, bool) {
		calls++
		if x != -1 || z != 2 {
			t.Fatalf("wrong Java division: %d,%d", x, z)
		}
		if noise(.3, .7) != newEndIslands(7).simplex(.3, .7) {
			t.Fatal("wrong seed binding")
		}
		return 24, true
	})
	if got, ok := s.Compute(3, -9, 0, 17); !ok || got != .125 || calls != 1 {
		t.Fatalf("override: %g %v calls=%d", got, ok, calls)
	}
	s.SetEndIslandHeight(func(func(float64, float64) float64, int32, int32) (float32, bool) { return 0, false })
	if _, ok := s.Compute(3, -9, 0, 17); ok {
		t.Fatal("failed override fell back to vanilla")
	}
}

func TestCustomSourceComposesDataDrivenInjections(t *testing.T) {
	ctx := context.Background()
	docs := map[string]Document{"biome/test:negative": {ID: "test:negative"}, "biome/test:positive": {ID: "test:positive"}, "biome/test:replacement": {ID: "test:replacement"}}
	g, err := CompileCustomBiomeSource(ctx, docs, Definition{Version: "1.21.1", Settings: json.RawMessage(rawRouterFixture)}, coordinateBiomeSource{})
	if err != nil {
		t.Fatal(err)
	}
	g, err = WithBiomeInjections(ctx, g, docs, BiomeInjectionConfig{Rules: []BiomeInjection{{ID: "test:replace", Type: "replace_fully", Targets: []string{"test:positive"}, Replacement: "test:replacement"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, seed := range []int64{0, 42} {
		g, err = g.WithSeed(ctx, seed)
		if err != nil {
			t.Fatal(err)
		}
		s := g.NewSampler()
		for _, test := range []struct {
			x    int
			want string
		}{{-1, "test:negative"}, {0, "test:replacement"}, {16, "test:replacement"}} {
			got, ok := s.NextNoiseBiome(test.x, 64, 0)
			if !ok || got != test.want {
				t.Fatalf("seed=%d x=%d: %s %v", seed, test.x, got, ok)
			}
		}
	}
}
