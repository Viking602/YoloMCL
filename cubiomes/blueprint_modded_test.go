package cubiomes

import (
	"context"
	"testing"
)

func TestBlueprintDeclinedSliceRetriesRemainingProviders(t *testing.T) {
	base := testGenerator(t, Overworld)
	declined := BlueprintModdedSlice{ID: "test:declined", Weight: 1, Provider: BlueprintModdedProvider{
		Type:     "blueprint:overlay",
		Overlays: []BlueprintModdedOverlay{{Matches: []string{"test:absent"}, Biome: "test:unused"}},
	}}
	accepted := BlueprintModdedSlice{ID: "test:accepted", Weight: 1, Provider: BlueprintModdedProvider{
		Type:     "blueprint:overlay",
		Overlays: []BlueprintModdedOverlay{{Matches: base.Biomes(), Biome: "test:replacement"}},
	}}
	for _, exhaust := range []bool{false, true} {
		slices := []BlueprintModdedSlice{declined, accepted}
		if exhaust {
			slices[1] = declined
		}
		g, err := WithBlueprintModded(context.Background(), base, BlueprintModdedConfig{
			Size: 1, DimID: "minecraft:overworld", Seed: 42, Slices: slices,
		})
		if err != nil {
			t.Fatal(err)
		}
		sampler, original := g.NewSampler(), base.NewSampler()
		source := g.custom.(*blueprintModded)
		retried := 0
		for x := -32; x < 32; x++ {
			if floorMod64(source.positionalRandom(x, -x), 2) != 0 {
				continue
			}
			retried++
			want := "test:replacement"
			if exhaust {
				want, _ = original.NoiseBiome(x*4, 64, -x*4)
			}
			got, ok := sampler.NoiseBiome(x*4, 64, -x*4)
			if !ok || got != want {
				t.Fatalf("exhaust=%v x=%d: biome=%q ok=%v, want %q", exhaust, x, got, ok, want)
			}
		}
		if retried == 0 {
			t.Fatal("no coordinates selected the declining provider")
		}
	}
}
