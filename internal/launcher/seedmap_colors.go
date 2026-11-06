package launcher

import "image/color"

// A readable categorical palette: water stays blue, dry biomes sandy, forest
// green and End/Nether distinct. These are map colors, not surface textures.
var seedMapVanillaColors = func() map[string]color.RGBA {
	groups := []struct {
		rgb uint32
		ids []string
	}{
		{0x2c638e, []string{"ocean", "cold_ocean", "lukewarm_ocean"}},
		{0x1f416b, []string{"deep_ocean", "deep_cold_ocean", "deep_lukewarm_ocean"}},
		{0x4b97ab, []string{"warm_ocean"}}, {0x83b6ce, []string{"frozen_ocean", "deep_frozen_ocean"}},
		{0x4e8ab0, []string{"river"}}, {0xa0d0df, []string{"frozen_river"}},
		{0x9fbe78, []string{"plains", "sunflower_plains", "meadow"}},
		{0x4c7d4d, []string{"forest", "flower_forest", "birch_forest", "old_growth_birch_forest"}},
		{0x355c49, []string{"dark_forest", "taiga", "old_growth_pine_taiga", "old_growth_spruce_taiga"}},
		{0x478048, []string{"jungle", "bamboo_jungle", "sparse_jungle"}},
		{0x718768, []string{"swamp", "mangrove_swamp"}},
		{0xe8d292, []string{"desert", "beach"}}, {0xc4bd77, []string{"savanna", "savanna_plateau", "windswept_savanna"}},
		{0xbf7955, []string{"badlands", "eroded_badlands", "wooded_badlands"}},
		{0xc8d8db, []string{"snowy_plains", "snowy_beach", "snowy_taiga", "snowy_slopes", "ice_spikes", "grove"}},
		{0x89938b, []string{"windswept_hills", "windswept_forest", "windswept_gravelly_hills", "stony_shore", "stony_peaks"}},
		{0xe0e6df, []string{"jagged_peaks", "frozen_peaks"}}, {0xaca1ba, []string{"mushroom_fields"}},
		{0x8d9673, []string{"lush_caves"}}, {0x9e8977, []string{"dripstone_caves"}}, {0x374b5c, []string{"deep_dark"}},
		{0xa7b4a2, []string{"pale_garden"}}, {0xe2aabe, []string{"cherry_grove"}},
		{0x9b4f45, []string{"nether_wastes"}}, {0x8b756b, []string{"soul_sand_valley"}},
		{0xaa3f51, []string{"crimson_forest"}}, {0x3d8e8a, []string{"warped_forest"}}, {0x676773, []string{"basalt_deltas"}},
		{0xd3cb91, []string{"the_end", "end_highlands"}}, {0xb7b17f, []string{"end_midlands"}},
		{0x777465, []string{"end_barrens"}}, {0x333748, []string{"small_end_islands", "the_void"}},
	}
	result := map[string]color.RGBA{}
	for _, group := range groups {
		for _, id := range group.ids {
			c := group.rgb
			result["minecraft:"+id] = color.RGBA{byte(c >> 16), byte(c >> 8), byte(c), 255}
		}
	}
	// Canonical pre-1.18 and Beta names share the corresponding map palette.
	for historical, modern := range map[string]string{
		"mountains": "windswept_hills", "wooded_mountains": "windswept_forest", "mountain_edge": "windswept_hills", "gravelly_mountains": "windswept_gravelly_hills", "modified_gravelly_mountains": "windswept_gravelly_hills",
		"snowy_tundra": "snowy_plains", "snowy_mountains": "snowy_plains", "snowy_taiga_hills": "snowy_taiga", "snowy_taiga_mountains": "snowy_taiga", "stone_shore": "stony_shore",
		"wooded_hills": "forest", "forest_hills": "forest", "birch_forest_hills": "birch_forest", "tall_birch_forest": "old_growth_birch_forest", "tall_birch_hills": "old_growth_birch_forest", "dark_forest_hills": "dark_forest",
		"taiga_hills": "taiga", "taiga_mountains": "taiga", "giant_tree_taiga": "old_growth_pine_taiga", "giant_tree_taiga_hills": "old_growth_pine_taiga", "giant_spruce_taiga": "old_growth_spruce_taiga", "giant_spruce_taiga_hills": "old_growth_spruce_taiga",
		"desert_hills": "desert", "desert_lakes": "desert", "swamp_hills": "swamp", "mushroom_field_shore": "mushroom_fields", "deep_warm_ocean": "warm_ocean",
		"jungle_hills": "jungle", "jungle_edge": "sparse_jungle", "modified_jungle": "jungle", "modified_jungle_edge": "sparse_jungle", "bamboo_jungle_hills": "bamboo_jungle",
		"shattered_savanna": "windswept_savanna", "shattered_savanna_plateau": "windswept_savanna", "badlands_plateau": "badlands", "modified_badlands_plateau": "badlands", "wooded_badlands_plateau": "wooded_badlands", "modified_wooded_badlands_plateau": "wooded_badlands",
		"seasonal_forest": "forest", "rainforest": "jungle", "shrubland": "plains",
	} {
		result["minecraft:"+historical] = result["minecraft:"+modern]
	}
	return result
}()

// seedMapPaletteOverGrass lists vanilla biomes whose JSON grass_color is a
// plains-like green (cherry_grove 0xB6DB61, pale_garden 0x778272): taking it as
// the map colour hid them among plains and meadows, so the palette wins.
var seedMapPaletteOverGrass = map[string]bool{"minecraft:cherry_grove": true, "minecraft:pale_garden": true}
