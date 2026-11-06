package cubiomes

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
)

func TestClimateRecipePreservesOriginalMatchesAndOrder(t *testing.T) {
	a, b, c := [7][2]int64{}, [7][2]int64{{1, 1}}, [7][2]int64{{2, 2}}
	base := []TerraBlenderParameter{{"test:first", a}, {"test:second", a}, {"test:third", b}}
	out, err := RewriteClimateParameters(context.Background(), base, []ClimateParameterRewrite{{a, TerraBlenderParameter{"test:replaced", b}}, {b, TerraBlenderParameter{"", a}}, {a, TerraBlenderParameter{"", c}}}, []TerraBlenderParameter{{"test:tail", a}})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 4 || out[0].Biome != "test:first" || out[1].Biome != "test:second" || out[0].Space != c || out[1].Space != c || out[2].Space != a || out[3].Biome != "test:tail" {
		t.Fatalf("ordered rewrites: %+v", out)
	}
	if base[0].Space != a {
		t.Fatal("rewrote caller's input")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = RewriteClimateParameters(ctx, base, nil, nil); err == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestOrderedVanillaClimateSource(t *testing.T) {
	ctx := context.Background()
	values, err := BuiltinClimateParameters(ctx, "1.20.1", "minecraft:overworld")
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 7593 || fmt.Sprintf("%x", sha256.Sum256(b)) != "713fccd0ec750da13df504264ee45031a46688960082592549e80638d8d5409e" {
		t.Fatal("ordered public vanilla builder output changed")
	}
	values[0].Biome = "test:mutated"
	copy, err := BuiltinClimateParameters(ctx, "1.20.1", "minecraft:overworld")
	if err != nil || copy[0].Biome == "test:mutated" {
		t.Fatal("shared mutable climate table")
	}
}

func TestTerraBlenderLegacyRegistrationClimateOracle(t *testing.T) {
	var fixture struct {
		Parameters []TerraBlenderParameter
		Samples    []struct {
			Climate [6]int64
			Biome   string
		}
	}
	readTerraBlenderFixture(t, "testdata/terrablender-registration-1.20.1.json.gz", &fixture)
	if len(fixture.Parameters) != 7595 || len(fixture.Samples) != 10000 {
		t.Fatal("incomplete independent fixture")
	}
	ctx := context.Background()
	base, err := New(ctx, Config{Version: "1.20.1", Dimension: Overworld})
	if err != nil {
		t.Fatal(err)
	}
	g, err := WithTerraBlender(ctx, base, TerraBlenderConfig{Minecraft: "1.20.1", Version: "3.0.1.10", Dimension: Overworld, RegionSize: 3, Regions: []TerraBlenderRegion{
		{ID: "minecraft:overworld", Base: true, Weight: 10},
		{ID: "yungscavebiomes:overworld", Index: 1, Weight: 3, Parameters: fixture.Parameters},
	}})
	if err != nil {
		t.Fatal(err)
	}
	s := g.NewSampler()
	for i, sample := range fixture.Samples {
		var p climatePoint
		copy(p[:], sample.Climate[:])
		if got := g.BiomeName(s.terraBlenderClimate(&p, 1)); got != sample.Biome {
			t.Fatalf("query %d %v: got %s want %s", i, sample.Climate, got, sample.Biome)
		}
	}
}
