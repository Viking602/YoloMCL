package modjar

import (
	"context"
	"testing"

	"github.com/Viking602/YoloMCL/cubiomes"
)

// The native builder fed vanilla tables must reproduce the bundled vanilla
// 1.20.1 overworld parameter list exactly, in order.
func TestOverworldBuilderMatchesVanillaParameters(t *testing.T) {
	for _, mojang := range []bool{false, true} {
		checkOverworldBuilder(t, mojang)
	}
}

func checkOverworldBuilder(t *testing.T, mojang bool) {
	v := newMachine(context.Background(), nil)
	h := &bindings{vm: v}
	v.host = h
	self := v.alloc(overworldBuilderClass, nil)
	tables := map[string][][]string{
		"f_187121_": {{"deep_frozen_ocean", "deep_cold_ocean", "deep_ocean", "deep_lukewarm_ocean", "warm_ocean"}, {"frozen_ocean", "cold_ocean", "ocean", "lukewarm_ocean", "warm_ocean"}},
		"f_187122_": {{"snowy_plains", "snowy_plains", "snowy_plains", "snowy_taiga", "taiga"}, {"plains", "plains", "forest", "taiga", "old_growth_spruce_taiga"}, {"flower_forest", "plains", "forest", "birch_forest", "dark_forest"}, {"savanna", "savanna", "forest", "jungle", "jungle"}, {"desert", "desert", "desert", "desert", "desert"}},
		"f_187123_": {{"ice_spikes", "", "snowy_taiga", "", ""}, {"", "", "", "", "old_growth_pine_taiga"}, {"sunflower_plains", "", "", "old_growth_birch_forest", ""}, {"", "", "plains", "sparse_jungle", "bamboo_jungle"}, {"", "", "", "", ""}},
		"f_187124_": {{"snowy_plains", "snowy_plains", "snowy_plains", "snowy_taiga", "snowy_taiga"}, {"meadow", "meadow", "forest", "taiga", "old_growth_spruce_taiga"}, {"meadow", "meadow", "meadow", "meadow", "dark_forest"}, {"savanna_plateau", "savanna_plateau", "forest", "forest", "jungle"}, {"badlands", "badlands", "badlands", "wooded_badlands", "wooded_badlands"}},
		"f_187125_": {{"ice_spikes", "", "", "", ""}, {"cherry_grove", "", "meadow", "meadow", "old_growth_pine_taiga"}, {"cherry_grove", "cherry_grove", "forest", "birch_forest", ""}, {"", "", "", "", ""}, {"eroded_badlands", "eroded_badlands", "", "", ""}},
		"f_201989_": {{"windswept_gravelly_hills", "windswept_gravelly_hills", "windswept_hills", "windswept_forest", "windswept_forest"}, {"windswept_gravelly_hills", "windswept_gravelly_hills", "windswept_hills", "windswept_forest", "windswept_forest"}, {"windswept_hills", "windswept_hills", "windswept_hills", "windswept_forest", "windswept_forest"}, {"", "", "", "", ""}, {"", "", "", "", ""}},
	}
	for name, rows := range tables {
		outer := &array{descriptor: biomeKeyTableDesc}
		for _, row := range rows {
			inner := &array{descriptor: "[Lnet/minecraft/resources/ResourceKey;"}
			for _, id := range row {
				if id == "" {
					inner.values = append(inner.values, nil)
				} else {
					inner.values = append(inner.values, h.vanillaBiomeKey("minecraft:"+id))
				}
			}
			outer.values = append(outer.values, inner)
		}
		if mojang {
			name = obbMojangNames[name]
		}
		self.fields[fieldKey(reference{overworldBuilderClass, name, biomeKeyTableDesc})] = outer
	}
	var got []cubiomes.TerraBlenderParameter
	sink := &consumer{accept: func(x any) {
		pair := objectData(x).(javaPair)
		got = append(got, cubiomes.TerraBlenderParameter{Biome: objectData(pair.right).(string), Space: objectData(pair.left).([7][2]int64)})
	}}
	func() {
		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("addBiomes: %v", p)
			}
		}()
		name := "m_187175_"
		if mojang {
			name = "addBiomes"
		}
		if _, ok := h.overworldBuilderCall(reference{overworldBuilderClass, name, "(Ljava/util/function/Consumer;)V"}, self, []any{sink}); !ok {
			t.Fatal("addBiomes unbound")
		}
	}()
	want, err := cubiomes.BuiltinClimateParameters(context.Background(), "1.20.1", "minecraft:overworld")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("mojang=%v: got %d parameters, want %d", mojang, len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parameter %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}
