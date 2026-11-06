package cubiomes

import (
	"context"
	"encoding/json"
	"testing"
)

func TestBiomeInjectionRasterSurfaceAndSeed(t *testing.T) {
	ctx := context.Background()
	base, err := New(ctx, Config{Version: "1.21.1", Seed: 42})
	if err != nil {
		t.Fatal(err)
	}
	cfg := BiomeInjectionConfig{Rules: []BiomeInjection{{Type: "force_placement", Replacement: "example:forced"}}}
	g, err := WithBiomeInjections(ctx, base, nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []int{1, 4, 16} {
		r := Range{X: -23, Y: 64, Z: -17, SizeX: 17, SizeY: 1, SizeZ: 13, StepXZ: step, StepY: 1, Voronoi: step == 1}
		out := make([]string, 221)
		if err := g.NewSampler().Generate(ctx, r, out); err != nil {
			t.Fatal(err)
		}
		for _, b := range out {
			if b != "example:forced" {
				t.Fatalf("step %d bypassed injection: %s", step, b)
			}
		}
	}
	surface := make([]SurfaceSample, 221)
	if err := g.NewSampler().GenerateSurface(ctx, SurfaceRange{X: -23, Z: -17, SizeX: 17, SizeZ: 13, Step: 1, SeaLevel: 63, Voronoi: true}, surface); err != nil {
		t.Fatal(err)
	}
	for _, sample := range surface {
		if g.BiomeName(sample.Biome) != "example:forced" {
			t.Fatal("surface bypassed injection")
		}
	}
	if b, _ := base.NewSampler().NoiseBiome(0, 64, 0); b == "example:forced" {
		t.Fatal("modified base generator")
	}
	docs := map[string]Document{"lithostitched_fast_noise_config/example:regions": {Data: json.RawMessage(`{"type":"lithostitched:cellular","frequency":0.002,"salt":304658814,"distance_function":"euclidean","return_type":"cell_value","jitter":1}`)}}
	cfg.Rules[0].Region = "example:a"
	cfg.Regions = []BiomeRegion{{"example:a", 1, base.Biomes()}, {"example:b", 1, base.Biomes()}}
	cfg.RegionDensity = json.RawMessage(`{"type":"minecraft:mul","argument1":0.5,"argument2":{"type":"minecraft:add","argument1":1,"argument2":{"type":"lithostitched:fast_noise","config":"example:regions","y_scale":0}}}`)
	g, err = WithBiomeInjections(ctx, base, docs, cfg)
	if err != nil {
		t.Fatal(err)
	}
	next, err := g.WithSeed(ctx, -5116646549048618101)
	if err != nil {
		t.Fatal(err)
	}
	newBase, err := New(ctx, Config{Version: "1.21.1", Seed: -5116646549048618101})
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := WithBiomeInjections(ctx, newBase, docs, cfg)
	if err != nil {
		t.Fatal(err)
	}
	a, b, old := next.NewSampler(), fresh.NewSampler(), g.NewSampler()
	changed, injected := false, false
	for x := -2048; x < 2048; x += 63 {
		for z := -2048; z < 2048; z += 127 {
			got, _ := a.NextNoiseBiome(x, 64, z)
			want, _ := b.NextNoiseBiome(x, 64, z)
			previous, _ := old.NextNoiseBiome(x, 64, z)
			if got != want {
				t.Fatalf("reseed mismatch at %d,%d", x, z)
			}
			changed = changed || got != previous
			injected = injected || got == "example:forced"
		}
	}
	if !changed || !injected {
		t.Fatal("fixture did not exercise seeded regions")
	}
}

func TestBiomeInjectionRangesAndStages(t *testing.T) {
	ctx := context.Background()
	base, err := New(ctx, Config{Version: "1.21.1"})
	if err != nil {
		t.Fatal(err)
	}
	point := [6]int64{3000}
	raw := json.RawMessage(`{"min_inclusive":0.3,"max_inclusive":0.42}`)
	cfg := BiomeInjectionConfig{Rules: []BiomeInjection{{Type: "replace_partially", Targets: base.Biomes(), Replacement: "example:partial", Parameters: map[string]json.RawMessage{"temperature": raw}}, {Type: "replace_fully", Targets: []string{"example:partial"}, Replacement: "example:full"}}}
	g, err := WithBiomeInjections(ctx, base, nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := g.NewSampler()
	id, ok := base.NewSampler().NextNoiseBiomeID(0, 64, 0)
	if !ok {
		t.Fatal("base")
	}
	for _, q := range []int64{2999, 3000, 4200, 4201} {
		point[0] = q
		p := climatePoint{point[0]}
		got := g.BiomeName(s.injectBiome(id, &p, 0, 64, 0))
		want := base.BiomeName(id)
		if q >= 3000 && q <= 4200 {
			want = "example:full"
		}
		if got != want {
			t.Fatalf("range %d: %s != %s", q, got, want)
		}
	}
	cfg.Rules[0], cfg.Rules[1] = cfg.Rules[1], cfg.Rules[0]
	g, err = WithBiomeInjections(ctx, base, nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	p := climatePoint{3500}
	if got := g.BiomeName(g.NewSampler().injectBiome(id, &p, 0, 64, 0)); got != "example:full" {
		t.Fatal("stage order depends on declaration order", got)
	}
	cfg.Rules[0].Type = "unknown"
	if _, err := WithBiomeInjections(ctx, base, nil, cfg); err == nil {
		t.Fatal("unknown injector silently ignored")
	}
}
