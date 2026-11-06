package modjar

import "strings"

// Vanilla OverworldBiomeBuilder (1.20.1 layout, SRG names) rebuilt natively:
// the game class is never loaded, yet TerraBlender regions (BWG and others)
// subclass it and call addBiomes with their own biome tables. Table reads are
// fields of the receiver; picker calls dispatch virtually so subclass
// overrides (TerraBlender's beach/peak/slope pickers) run as in the game.

const (
	overworldBuilderClass = "net/minecraft/world/level/biome/OverworldBiomeBuilder"
	climateParameterClass = "net/minecraft/world/level/biome/Climate$Parameter"
	biomeKeyTableDesc     = "[[Lnet/minecraft/resources/ResourceKey;"
	pickerDesc3           = "(IILnet/minecraft/world/level/biome/Climate$Parameter;)Lnet/minecraft/resources/ResourceKey;"
)

// Vanilla Biomes field names (SRG) to registry paths.
var srgVanillaBiomes = map[string]string{"f_48173_": "the_void", "f_48202_": "plains", "f_48176_": "sunflower_plains", "f_186761_": "snowy_plains", "f_48182_": "ice_spikes", "f_48203_": "desert", "f_48207_": "swamp", "f_220595_": "mangrove_swamp", "f_48205_": "forest", "f_48179_": "flower_forest", "f_48149_": "birch_forest", "f_48151_": "dark_forest", "f_186762_": "old_growth_birch_forest", "f_186763_": "old_growth_pine_taiga", "f_186764_": "old_growth_spruce_taiga", "f_48206_": "taiga", "f_48152_": "snowy_taiga", "f_48157_": "savanna", "f_48158_": "savanna_plateau", "f_186765_": "windswept_hills", "f_186766_": "windswept_gravelly_hills", "f_186767_": "windswept_forest", "f_186768_": "windswept_savanna", "f_48222_": "jungle", "f_186769_": "sparse_jungle", "f_48197_": "bamboo_jungle", "f_48159_": "badlands", "f_48194_": "eroded_badlands", "f_186753_": "wooded_badlands", "f_186754_": "meadow", "f_271432_": "cherry_grove", "f_186755_": "grove", "f_186756_": "snowy_slopes", "f_186757_": "frozen_peaks", "f_186758_": "jagged_peaks", "f_186759_": "stony_peaks", "f_48208_": "river", "f_48212_": "frozen_river", "f_48217_": "beach", "f_48148_": "snowy_beach", "f_186760_": "stony_shore", "f_48166_": "warm_ocean", "f_48167_": "lukewarm_ocean", "f_48170_": "deep_lukewarm_ocean", "f_48174_": "ocean", "f_48225_": "deep_ocean", "f_48168_": "cold_ocean", "f_48171_": "deep_cold_ocean", "f_48211_": "frozen_ocean", "f_48172_": "deep_frozen_ocean", "f_48215_": "mushroom_fields", "f_151784_": "dripstone_caves", "f_151785_": "lush_caves", "f_220594_": "deep_dark", "f_48209_": "nether_wastes", "f_48201_": "warped_forest", "f_48200_": "crimson_forest", "f_48199_": "soul_sand_valley", "f_48175_": "basalt_deltas", "f_48210_": "the_end", "f_48164_": "end_highlands", "f_48163_": "end_midlands", "f_48162_": "small_end_islands", "f_48165_": "end_barrens"}

func q(f float32) int64 { return int64(f * 10000) }

type obbSpan [2]int64

func obbJoin(a, b obbSpan) obbSpan {
	return obbSpan{min(a[0], b[0]), max(a[1], b[1])}
}

var (
	obbFull         = obbSpan{q(-1), q(1)}
	obbTemperatures = [5]obbSpan{{q(-1), q(-0.45)}, {q(-0.45), q(-0.15)}, {q(-0.15), q(0.2)}, {q(0.2), q(0.55)}, {q(0.55), q(1)}}
	obbHumidities   = [5]obbSpan{{q(-1), q(-0.35)}, {q(-0.35), q(-0.1)}, {q(-0.1), q(0.1)}, {q(0.1), q(0.3)}, {q(0.3), q(1)}}
	obbErosions     = [7]obbSpan{{q(-1), q(-0.78)}, {q(-0.78), q(-0.375)}, {q(-0.375), q(-0.2225)}, {q(-0.2225), q(0.05)}, {q(0.05), q(0.45)}, {q(0.45), q(0.55)}, {q(0.55), q(1)}}
	obbFrozen       = obbTemperatures[0]
	obbUnfrozen     = obbJoin(obbTemperatures[1], obbTemperatures[4])
	obbMushroom     = obbSpan{q(-1.2), q(-1.05)}
	obbDeepOcean    = obbSpan{q(-1.05), q(-0.455)}
	obbOcean        = obbSpan{q(-0.455), q(-0.19)}
	obbCoast        = obbSpan{q(-0.19), q(-0.11)}
	obbInland       = obbSpan{q(-0.11), q(0.55)}
	obbNear         = obbSpan{q(-0.11), q(0.03)}
	obbMid          = obbSpan{q(0.03), q(0.3)}
	obbFar          = obbSpan{q(0.3), q(1)}
)

func (h *bindings) vanillaBiomeKey(id string) *object {
	return h.symbol("key:net/minecraft/core/registries/Registries.BIOME\x00"+id, "net/minecraft/resources/ResourceKey", id)
}

// vanillaBiomeStatic binds SRG-named Biomes fields; Mojang names are the
// upper-case registry path.
func (h *bindings) vanillaBiomeStatic(r reference) (any, bool) {
	if r.owner != "net/minecraft/world/level/biome/Biomes" || r.desc != "Lnet/minecraft/resources/ResourceKey;" {
		return nil, false
	}
	if path, ok := srgVanillaBiomes[r.name]; ok {
		return h.vanillaBiomeKey("minecraft:" + path), true
	}
	if strings.ToUpper(r.name) == r.name {
		return h.vanillaBiomeKey("minecraft:" + strings.ToLower(r.name)), true
	}
	return nil, false
}

var obbMojangNames = map[string]string{
	"f_187121_": "OCEANS",
	"f_187122_": "MIDDLE_BIOMES",
	"f_187123_": "MIDDLE_BIOMES_VARIANT",
	"f_187124_": "PLATEAU_BIOMES",
	"f_187125_": "PLATEAU_BIOMES_VARIANT",
	"f_201989_": "SHATTERED_BIOMES",
	"m_187175_": "addBiomes",
	"m_187163_": "pickMiddleBiome",
	"m_187191_": "pickMiddleBiomeOrBadlandsIfHot",
	"m_187211_": "pickMiddleBiomeOrBadlandsIfHotOrSlopeIfCold",
	"m_201990_": "maybePickWindsweptSavannaBiome",
	"m_187222_": "pickShatteredCoastBiome",
	"m_187160_": "pickBeachBiome",
	"m_187172_": "pickBadlandsBiome",
	"m_187233_": "pickPlateauBiome",
	"m_187240_": "pickPeakBiome",
	"m_187244_": "pickSlopeBiome",
	"m_202001_": "pickShatteredBiome",
}

// obbSRGNames maps the Mojang names back to SRG for dispatch in bindings.
var obbSRGNames = func() map[string]string {
	out := map[string]string{}
	for srg, mojang := range obbMojangNames {
		out[mojang] = srg
	}
	return out
}()

type overworldBuilder struct {
	h        *bindings
	self     *object
	consumer any
	// mojang is set when the loader remapped the builder's members to
	// Mojang names (the table fields carry those names on the receiver).
	mojang bool
}

func (b overworldBuilder) n(srg string) string {
	if b.mojang {
		return obbMojangNames[srg]
	}
	return srg
}

func (b overworldBuilder) table(name string) *array {
	x, ok := b.self.fields[fieldKey(reference{overworldBuilderClass, b.n(name), biomeKeyTableDesc})]
	t, isArray := x.(*array)
	if !ok || !isArray || t == nil {
		fail("overworld builder table %s is unavailable", name)
	}
	return t
}

func (b overworldBuilder) cell(name string, i, j int) any {
	row, ok := b.table(name).values[i].(*array)
	if !ok || row == nil {
		fail("overworld builder table %s row %d is unavailable", name, i)
	}
	if x := row.values[j]; x != nil {
		if _, ok := x.(*object); !ok {
			fail("overworld builder table %s holds a non-key value", name)
		}
		return x
	}
	return nil
}

func (b overworldBuilder) param(s obbSpan) *object {
	return b.h.vm.alloc(climateParameterClass, [2]int64(s))
}

func (b overworldBuilder) pick(name string, i, j int, w obbSpan) any {
	return b.h.vm.method(reference{b.self.class, b.n(name), pickerDesc3}, b.self, true, []any{int32(i), int32(j), b.param(w)})
}

func (b overworldBuilder) beach(i, j int) any {
	return b.h.vm.method(reference{b.self.class, b.n("m_187160_"), "(II)Lnet/minecraft/resources/ResourceKey;"}, b.self, true, []any{int32(i), int32(j)})
}

func (b overworldBuilder) windswept(i, j int, w obbSpan, under any) any {
	return b.h.vm.method(reference{b.self.class, b.n("m_201990_"), "(IILnet/minecraft/world/level/biome/Climate$Parameter;Lnet/minecraft/resources/ResourceKey;)Lnet/minecraft/resources/ResourceKey;"}, b.self, true, []any{int32(i), int32(j), b.param(w), under})
}

func (b overworldBuilder) emit(t, hu, c, e, depth, w obbSpan, offset float32, key any) {
	if key == nil {
		fail("overworld builder picked a null biome")
	}
	if _, ok := key.(*object); !ok {
		fail("overworld builder picked an unavailable biome")
	}
	o := q(offset)
	space := [7][2]int64{t, hu, c, e, depth, w, {o, o}}
	point := b.h.vm.alloc("net/minecraft/world/level/biome/Climate$ParameterPoint", space)
	pair := b.h.vm.alloc("com/mojang/datafixers/util/Pair", javaPair{point, key})
	b.h.vm.method(reference{"java/util/function/Consumer", "accept", "(Ljava/lang/Object;)V"}, b.consumer, true, []any{pair})
}

// m_187180_: surface biome at depth 0 and 1.
func (b overworldBuilder) surface(t, hu, c, e, w obbSpan, key any) {
	b.emit(t, hu, c, e, obbSpan{0, 0}, w, 0, key)
	b.emit(t, hu, c, e, obbSpan{q(1), q(1)}, w, 0, key)
}

func (b overworldBuilder) addBiomes() {
	k := b.h.vanillaBiomeKey
	// m_187195_ off-coast
	b.surface(obbFull, obbFull, obbMushroom, obbFull, obbFull, k("minecraft:mushroom_fields"))
	for i, t := range obbTemperatures {
		b.surface(t, obbFull, obbDeepOcean, obbFull, obbFull, b.cell("f_187121_", 0, i))
		b.surface(t, obbFull, obbOcean, obbFull, obbFull, b.cell("f_187121_", 1, i))
	}
	// m_187215_ inland
	slices := []struct {
		lo, hi float32
		kind   byte
	}{
		{-1, -0.93333334, 'm'}, {-0.93333334, -0.7666667, 'h'}, {-0.7666667, -0.56666666, 'p'},
		{-0.56666666, -0.4, 'h'}, {-0.4, -0.26666668, 'm'}, {-0.26666668, -0.05, 'l'},
		{-0.05, 0.05, 'v'}, {0.05, 0.26666668, 'l'}, {0.26666668, 0.4, 'm'},
		{0.4, 0.56666666, 'h'}, {0.56666666, 0.7666667, 'p'}, {0.7666667, 0.93333334, 'h'}, {0.93333334, 1, 'm'},
	}
	for _, s := range slices {
		w := obbSpan{q(s.lo), q(s.hi)}
		switch s.kind {
		case 'p':
			b.peaks(w)
		case 'h':
			b.high(w)
		case 'm':
			b.mid(w)
		case 'l':
			b.low(w)
		case 'v':
			b.valleys(w)
		}
	}
	// m_187226_ underground
	b.emit(obbFull, obbFull, obbSpan{q(0.8), q(1)}, obbFull, obbSpan{q(0.2), q(0.9)}, obbFull, 0, k("minecraft:dripstone_caves"))
	b.emit(obbFull, obbSpan{q(0.7), q(1)}, obbFull, obbFull, obbSpan{q(0.2), q(0.9)}, obbFull, 0, k("minecraft:lush_caves"))
	b.emit(obbFull, obbFull, obbFull, obbJoin(obbErosions[0], obbErosions[1]), obbSpan{q(1.1), q(1.1)}, obbFull, 0, k("minecraft:deep_dark"))
}

func (b overworldBuilder) grid(f func(i, j int, t, hu obbSpan)) {
	for i, t := range obbTemperatures {
		for j, hu := range obbHumidities {
			f(i, j, t, hu)
		}
	}
}

// m_187177_
func (b overworldBuilder) peaks(w obbSpan) {
	e := obbErosions
	b.grid(func(i, j int, t, hu obbSpan) {
		middle := b.pick("m_187163_", i, j, w)
		badlands := b.pick("m_187191_", i, j, w)
		slopeIfCold := b.pick("m_187211_", i, j, w)
		plateau := b.pick("m_187233_", i, j, w)
		shattered := b.pick("m_202001_", i, j, w)
		windswept := b.windswept(i, j, w, shattered)
		peak := b.pick("m_187240_", i, j, w)
		b.surface(t, hu, obbJoin(obbCoast, obbFar), e[0], w, peak)
		b.surface(t, hu, obbJoin(obbCoast, obbNear), e[1], w, slopeIfCold)
		b.surface(t, hu, obbJoin(obbMid, obbFar), e[1], w, peak)
		b.surface(t, hu, obbJoin(obbCoast, obbNear), obbJoin(e[2], e[3]), w, middle)
		b.surface(t, hu, obbJoin(obbMid, obbFar), e[2], w, plateau)
		b.surface(t, hu, obbMid, e[3], w, badlands)
		b.surface(t, hu, obbFar, e[3], w, plateau)
		b.surface(t, hu, obbJoin(obbCoast, obbFar), e[4], w, middle)
		b.surface(t, hu, obbJoin(obbCoast, obbNear), e[5], w, windswept)
		b.surface(t, hu, obbJoin(obbMid, obbFar), e[5], w, shattered)
		b.surface(t, hu, obbJoin(obbCoast, obbFar), e[6], w, middle)
	})
}

// m_187197_
func (b overworldBuilder) high(w obbSpan) {
	e := obbErosions
	b.grid(func(i, j int, t, hu obbSpan) {
		middle := b.pick("m_187163_", i, j, w)
		badlands := b.pick("m_187191_", i, j, w)
		slopeIfCold := b.pick("m_187211_", i, j, w)
		plateau := b.pick("m_187233_", i, j, w)
		shattered := b.pick("m_202001_", i, j, w)
		windswept := b.windswept(i, j, w, middle)
		slope := b.pick("m_187244_", i, j, w)
		peak := b.pick("m_187240_", i, j, w)
		b.surface(t, hu, obbCoast, obbJoin(e[0], e[1]), w, middle)
		b.surface(t, hu, obbNear, e[0], w, slope)
		b.surface(t, hu, obbJoin(obbMid, obbFar), e[0], w, peak)
		b.surface(t, hu, obbNear, e[1], w, slopeIfCold)
		b.surface(t, hu, obbJoin(obbMid, obbFar), e[1], w, slope)
		b.surface(t, hu, obbJoin(obbCoast, obbNear), obbJoin(e[2], e[3]), w, middle)
		b.surface(t, hu, obbJoin(obbMid, obbFar), e[2], w, plateau)
		b.surface(t, hu, obbMid, e[3], w, badlands)
		b.surface(t, hu, obbFar, e[3], w, plateau)
		b.surface(t, hu, obbJoin(obbCoast, obbFar), e[4], w, middle)
		b.surface(t, hu, obbJoin(obbCoast, obbNear), e[5], w, windswept)
		b.surface(t, hu, obbJoin(obbMid, obbFar), e[5], w, shattered)
		b.surface(t, hu, obbJoin(obbCoast, obbFar), e[6], w, middle)
	})
}

func (b overworldBuilder) shores(w obbSpan) {
	k := b.h.vanillaBiomeKey
	e := obbErosions
	b.surface(obbFull, obbFull, obbCoast, obbJoin(e[0], e[2]), w, k("minecraft:stony_shore"))
	b.surface(obbJoin(obbTemperatures[1], obbTemperatures[2]), obbFull, obbJoin(obbNear, obbFar), e[6], w, k("minecraft:swamp"))
	b.surface(obbJoin(obbTemperatures[3], obbTemperatures[4]), obbFull, obbJoin(obbNear, obbFar), e[6], w, k("minecraft:mangrove_swamp"))
}

// m_187217_
func (b overworldBuilder) mid(w obbSpan) {
	e := obbErosions
	b.shores(w)
	b.grid(func(i, j int, t, hu obbSpan) {
		middle := b.pick("m_187163_", i, j, w)
		badlands := b.pick("m_187191_", i, j, w)
		slopeIfCold := b.pick("m_187211_", i, j, w)
		shattered := b.pick("m_202001_", i, j, w)
		plateau := b.pick("m_187233_", i, j, w)
		beach := b.beach(i, j)
		windswept := b.windswept(i, j, w, middle)
		shatteredCoast := b.pick("m_187222_", i, j, w)
		slope := b.pick("m_187244_", i, j, w)
		b.surface(t, hu, obbJoin(obbNear, obbFar), e[0], w, slope)
		b.surface(t, hu, obbJoin(obbNear, obbMid), e[1], w, slopeIfCold)
		if i == 0 {
			b.surface(t, hu, obbFar, e[1], w, slope)
		} else {
			b.surface(t, hu, obbFar, e[1], w, plateau)
		}
		b.surface(t, hu, obbNear, e[2], w, middle)
		b.surface(t, hu, obbMid, e[2], w, badlands)
		b.surface(t, hu, obbFar, e[2], w, plateau)
		b.surface(t, hu, obbJoin(obbCoast, obbNear), e[3], w, middle)
		b.surface(t, hu, obbJoin(obbMid, obbFar), e[3], w, badlands)
		if w[1] < 0 {
			b.surface(t, hu, obbCoast, e[4], w, beach)
			b.surface(t, hu, obbJoin(obbNear, obbFar), e[4], w, middle)
		} else {
			b.surface(t, hu, obbJoin(obbCoast, obbFar), e[4], w, middle)
		}
		b.surface(t, hu, obbCoast, e[5], w, shatteredCoast)
		b.surface(t, hu, obbNear, e[5], w, windswept)
		b.surface(t, hu, obbJoin(obbMid, obbFar), e[5], w, shattered)
		if w[1] < 0 {
			b.surface(t, hu, obbCoast, e[6], w, beach)
		} else {
			b.surface(t, hu, obbCoast, e[6], w, middle)
		}
		if i == 0 {
			b.surface(t, hu, obbJoin(obbNear, obbFar), e[6], w, middle)
		}
	})
}

// m_187228_
func (b overworldBuilder) low(w obbSpan) {
	e := obbErosions
	b.shores(w)
	b.grid(func(i, j int, t, hu obbSpan) {
		middle := b.pick("m_187163_", i, j, w)
		badlands := b.pick("m_187191_", i, j, w)
		slopeIfCold := b.pick("m_187211_", i, j, w)
		beach := b.beach(i, j)
		windswept := b.windswept(i, j, w, middle)
		shatteredCoast := b.pick("m_187222_", i, j, w)
		b.surface(t, hu, obbNear, obbJoin(e[0], e[1]), w, badlands)
		b.surface(t, hu, obbJoin(obbMid, obbFar), obbJoin(e[0], e[1]), w, slopeIfCold)
		b.surface(t, hu, obbNear, obbJoin(e[2], e[3]), w, middle)
		b.surface(t, hu, obbJoin(obbMid, obbFar), obbJoin(e[2], e[3]), w, badlands)
		b.surface(t, hu, obbCoast, obbJoin(e[3], e[4]), w, beach)
		b.surface(t, hu, obbJoin(obbNear, obbFar), e[4], w, middle)
		b.surface(t, hu, obbCoast, e[5], w, shatteredCoast)
		b.surface(t, hu, obbNear, e[5], w, windswept)
		b.surface(t, hu, obbJoin(obbMid, obbFar), e[5], w, middle)
		b.surface(t, hu, obbCoast, e[6], w, beach)
		if i == 0 {
			b.surface(t, hu, obbJoin(obbNear, obbFar), e[6], w, middle)
		}
	})
}

// m_187237_
func (b overworldBuilder) valleys(w obbSpan) {
	k := b.h.vanillaBiomeKey
	e := obbErosions
	stonyOrFrozen, stonyOrRiver := k("minecraft:stony_shore"), k("minecraft:stony_shore")
	if w[1] >= 0 {
		stonyOrFrozen, stonyOrRiver = k("minecraft:frozen_river"), k("minecraft:river")
	}
	frozenRiver, river := k("minecraft:frozen_river"), k("minecraft:river")
	b.surface(obbFrozen, obbFull, obbCoast, obbJoin(e[0], e[1]), w, stonyOrFrozen)
	b.surface(obbUnfrozen, obbFull, obbCoast, obbJoin(e[0], e[1]), w, stonyOrRiver)
	b.surface(obbFrozen, obbFull, obbNear, obbJoin(e[0], e[1]), w, frozenRiver)
	b.surface(obbUnfrozen, obbFull, obbNear, obbJoin(e[0], e[1]), w, river)
	b.surface(obbFrozen, obbFull, obbJoin(obbCoast, obbFar), obbJoin(e[2], e[5]), w, frozenRiver)
	b.surface(obbUnfrozen, obbFull, obbJoin(obbCoast, obbFar), obbJoin(e[2], e[5]), w, river)
	b.surface(obbFrozen, obbFull, obbCoast, e[6], w, frozenRiver)
	b.surface(obbUnfrozen, obbFull, obbCoast, e[6], w, river)
	b.surface(obbJoin(obbTemperatures[1], obbTemperatures[2]), obbFull, obbJoin(obbInland, obbFar), e[6], w, k("minecraft:swamp"))
	b.surface(obbJoin(obbTemperatures[3], obbTemperatures[4]), obbFull, obbJoin(obbInland, obbFar), e[6], w, k("minecraft:mangrove_swamp"))
	b.surface(obbFrozen, obbFull, obbJoin(obbInland, obbFar), e[6], w, frozenRiver)
	b.grid(func(i, j int, t, hu obbSpan) {
		badlands := b.pick("m_187191_", i, j, w)
		b.surface(t, hu, obbJoin(obbMid, obbFar), obbJoin(e[0], e[1]), w, badlands)
	})
}

func climateMax(x any) int64 {
	s, ok := objectData(x).([2]int64)
	if !ok {
		fail("overworld builder weirdness is unavailable")
	}
	return s[1]
}

// overworldBuilderCall binds addBiomes and the vanilla pickers reached by
// inherited dispatch from a subclass (or a direct builder instance).
func (h *bindings) overworldBuilderCall(r reference, receiver any, args []any) (any, bool) {
	if value, ok := h.worldBiomeRegistryCall(r, receiver, args); ok {
		return value, true
	}
	if r.owner != overworldBuilderClass {
		return nil, false
	}
	self, ok := receiver.(*object)
	if !ok || self == nil {
		return nil, false
	}
	b := overworldBuilder{h: h, self: self}
	_, b.mojang = self.fields[fieldKey(reference{overworldBuilderClass, "OCEANS", biomeKeyTableDesc})]
	if srg, ok := obbSRGNames[r.name]; ok {
		r.name = srg
	}
	k := h.vanillaBiomeKey
	ij := func() (int, int) { return int(integer(args[0])), int(integer(args[1])) }
	switch r.name + r.desc {
	case "m_187175_(Ljava/util/function/Consumer;)V":
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "biome consumer")
		}
		b.consumer = args[0]
		b.addBiomes()
		return nil, true
	case "m_187160_(II)Lnet/minecraft/resources/ResourceKey;":
		i, _ := ij()
		switch i {
		case 0:
			return k("minecraft:snowy_beach"), true
		case 4:
			return k("minecraft:desert"), true
		}
		return k("minecraft:beach"), true
	case "m_187172_(ILnet/minecraft/world/level/biome/Climate$Parameter;)Lnet/minecraft/resources/ResourceKey;":
		j := int(integer(args[0]))
		switch {
		case j < 2 && climateMax(args[1]) < 0:
			return k("minecraft:badlands"), true
		case j < 2:
			return k("minecraft:eroded_badlands"), true
		case j < 3:
			return k("minecraft:badlands"), true
		}
		return k("minecraft:wooded_badlands"), true
	case "m_201990_(IILnet/minecraft/world/level/biome/Climate$Parameter;Lnet/minecraft/resources/ResourceKey;)Lnet/minecraft/resources/ResourceKey;":
		i, j := ij()
		if i > 1 && j < 4 && climateMax(args[2]) >= 0 {
			return k("minecraft:windswept_savanna"), true
		}
		return args[3], true
	}
	if r.desc != pickerDesc3 {
		return nil, false
	}
	i, j := ij()
	w := args[2]
	weird := obbSpan(objectData(w).([2]int64))
	switch r.name {
	case "m_187163_":
		if climateMax(w) >= 0 {
			if x := b.cell("f_187123_", i, j); x != nil {
				return x, true
			}
		}
		return b.cell("f_187122_", i, j), true
	case "m_187191_":
		if i == 4 {
			return h.vm.method(reference{self.class, b.n("m_187172_"), "(ILnet/minecraft/world/level/biome/Climate$Parameter;)Lnet/minecraft/resources/ResourceKey;"}, self, true, []any{int32(j), w}), true
		}
		return b.pick("m_187163_", i, j, weird), true
	case "m_187211_":
		if i == 0 {
			return b.pick("m_187244_", i, j, weird), true
		}
		return b.pick("m_187191_", i, j, weird), true
	case "m_187222_":
		under := b.beach(i, j)
		if climateMax(w) >= 0 {
			under = b.pick("m_187163_", i, j, weird)
		}
		return b.windswept(i, j, weird, under), true
	case "m_187233_":
		if climateMax(w) >= 0 {
			if x := b.cell("f_187125_", i, j); x != nil {
				return x, true
			}
		}
		return b.cell("f_187124_", i, j), true
	case "m_187240_":
		switch {
		case i <= 2 && climateMax(w) < 0:
			return k("minecraft:jagged_peaks"), true
		case i <= 2:
			return k("minecraft:frozen_peaks"), true
		case i == 3:
			return k("minecraft:stony_peaks"), true
		}
		return h.vm.method(reference{self.class, b.n("m_187172_"), "(ILnet/minecraft/world/level/biome/Climate$Parameter;)Lnet/minecraft/resources/ResourceKey;"}, self, true, []any{int32(j), w}), true
	case "m_187244_":
		if i >= 3 {
			return b.pick("m_187233_", i, j, weird), true
		}
		if j <= 1 {
			return k("minecraft:snowy_slopes"), true
		}
		return k("minecraft:grove"), true
	case "m_202001_":
		if x := b.cell("f_201989_", i, j); x != nil {
			return x, true
		}
		return b.pick("m_187163_", i, j, weird), true
	}
	return nil, false
}
