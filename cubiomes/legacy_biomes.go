package cubiomes

// Historical biome identifiers follow Cubitect/cubiomes (MIT). See data/LICENSE-cubiomes.txt.
const (
	mcUNDEF                            = 0
	mcB1_7                             = 1
	mcB1_8                             = 2
	mc1_0_0                            = 3
	mc1_0                              = 3
	mc1_1_0                            = 4
	mc1_1                              = 4
	mc1_2_5                            = 5
	mc1_2                              = 5
	mc1_3_2                            = 6
	mc1_3                              = 6
	mc1_4_7                            = 7
	mc1_4                              = 7
	mc1_5_2                            = 8
	mc1_5                              = 8
	mc1_6_4                            = 9
	mc1_6                              = 9
	mc1_7_10                           = 10
	mc1_7                              = 10
	mc1_8_9                            = 11
	mc1_8                              = 11
	mc1_9_4                            = 12
	mc1_9                              = 12
	mc1_10_2                           = 13
	mc1_10                             = 13
	mc1_11_2                           = 14
	mc1_11                             = 14
	mc1_12_2                           = 15
	mc1_12                             = 15
	mc1_13_2                           = 16
	mc1_13                             = 16
	mc1_14_4                           = 17
	mc1_14                             = 17
	mc1_15_2                           = 18
	mc1_15                             = 18
	mc1_16_1                           = 19
	mc1_16_5                           = 20
	mc1_16                             = 20
	mc1_17_1                           = 21
	mc1_17                             = 21
	mc1_18_2                           = 22
	mc1_18                             = 22
	mc1_19_2                           = 23
	mc1_19_4                           = 24
	mc1_19                             = 24
	mc1_20_6                           = 25
	mc1_20                             = 25
	mc1_21_1                           = 26
	mc1_21_3                           = 27
	mc1_21WD                           = 28
	mc1_21                             = 28
	mcNEWEST                           = 28
	biomeNone                          = -1
	biomeOcean                         = 0
	biomePlains                        = 1
	biomeDesert                        = 2
	biomeMountains                     = 3
	biomeForest                        = 4
	biomeTaiga                         = 5
	biomeSwamp                         = 6
	biomeRiver                         = 7
	biomeNetherWastes                  = 8
	biomeTheEnd                        = 9
	biomeFrozenOcean                   = 10
	biomeFrozenRiver                   = 11
	biomeSnowyTundra                   = 12
	biomeSnowyMountains                = 13
	biomeMushroomFields                = 14
	biomeMushroomFieldShore            = 15
	biomeBeach                         = 16
	biomeDesertHills                   = 17
	biomeWoodedHills                   = 18
	biomeTaigaHills                    = 19
	biomeMountainEdge                  = 20
	biomeJungle                        = 21
	biomeJungleHills                   = 22
	biomeJungleEdge                    = 23
	biomeDeepOcean                     = 24
	biomeStoneShore                    = 25
	biomeSnowyBeach                    = 26
	biomeBirchForest                   = 27
	biomeBirchForestHills              = 28
	biomeDarkForest                    = 29
	biomeSnowyTaiga                    = 30
	biomeSnowyTaigaHills               = 31
	biomeGiantTreeTaiga                = 32
	biomeGiantTreeTaigaHills           = 33
	biomeWoodedMountains               = 34
	biomeSavanna                       = 35
	biomeSavannaPlateau                = 36
	biomeBadlands                      = 37
	biomeWoodedBadlandsPlateau         = 38
	biomeBadlandsPlateau               = 39
	biomeSmallEndIslands               = 40
	biomeEndMidlands                   = 41
	biomeEndHighlands                  = 42
	biomeEndBarrens                    = 43
	biomeWarmOcean                     = 44
	biomeLukewarmOcean                 = 45
	biomeColdOcean                     = 46
	biomeDeepWarmOcean                 = 47
	biomeDeepLukewarmOcean             = 48
	biomeDeepColdOcean                 = 49
	biomeDeepFrozenOcean               = 50
	biomeSeasonalForest                = 51
	biomeRainforest                    = 52
	biomeShrubland                     = 53
	biomeTheVoid                       = 127
	biomeSunflowerPlains               = 129
	biomeDesertLakes                   = 130
	biomeGravellyMountains             = 131
	biomeFlowerForest                  = 132
	biomeTaigaMountains                = 133
	biomeSwampHills                    = 134
	biomeIceSpikes                     = 140
	biomeModifiedJungle                = 149
	biomeModifiedJungleEdge            = 151
	biomeTallBirchForest               = 155
	biomeTallBirchHills                = 156
	biomeDarkForestHills               = 157
	biomeSnowyTaigaMountains           = 158
	biomeGiantSpruceTaiga              = 160
	biomeGiantSpruceTaigaHills         = 161
	biomeModifiedGravellyMountains     = 162
	biomeShatteredSavanna              = 163
	biomeShatteredSavannaPlateau       = 164
	biomeErodedBadlands                = 165
	biomeModifiedWoodedBadlandsPlateau = 166
	biomeModifiedBadlandsPlateau       = 167
	biomeBambooJungle                  = 168
	biomeBambooJungleHills             = 169
	biomeSoulSandValley                = 170
	biomeCrimsonForest                 = 171
	biomeWarpedForest                  = 172
	biomeBasaltDeltas                  = 173
	biomeDripstoneCaves                = 174
	biomeLushCaves                     = 175
	biomeMeadow                        = 177
	biomeGrove                         = 178
	biomeSnowySlopes                   = 179
	biomeJaggedPeaks                   = 180
	biomeFrozenPeaks                   = 181
	biomeStonyPeaks                    = 182
	biomeDeepDark                      = 183
	biomeMangroveSwamp                 = 184
	biomeCherryGrove                   = 185
	biomePaleGarden                    = 186
)

var legacyBiomeNames = [256]string{
	0:   "minecraft:ocean",
	1:   "minecraft:plains",
	2:   "minecraft:desert",
	3:   "minecraft:mountains",
	4:   "minecraft:forest",
	5:   "minecraft:taiga",
	6:   "minecraft:swamp",
	7:   "minecraft:river",
	8:   "minecraft:nether_wastes",
	9:   "minecraft:the_end",
	10:  "minecraft:frozen_ocean",
	11:  "minecraft:frozen_river",
	12:  "minecraft:snowy_tundra",
	13:  "minecraft:snowy_mountains",
	14:  "minecraft:mushroom_fields",
	15:  "minecraft:mushroom_field_shore",
	16:  "minecraft:beach",
	17:  "minecraft:desert_hills",
	18:  "minecraft:wooded_hills",
	19:  "minecraft:taiga_hills",
	20:  "minecraft:mountain_edge",
	21:  "minecraft:jungle",
	22:  "minecraft:jungle_hills",
	23:  "minecraft:jungle_edge",
	24:  "minecraft:deep_ocean",
	25:  "minecraft:stone_shore",
	26:  "minecraft:snowy_beach",
	27:  "minecraft:birch_forest",
	28:  "minecraft:birch_forest_hills",
	29:  "minecraft:dark_forest",
	30:  "minecraft:snowy_taiga",
	31:  "minecraft:snowy_taiga_hills",
	32:  "minecraft:giant_tree_taiga",
	33:  "minecraft:giant_tree_taiga_hills",
	34:  "minecraft:wooded_mountains",
	35:  "minecraft:savanna",
	36:  "minecraft:savanna_plateau",
	37:  "minecraft:badlands",
	38:  "minecraft:wooded_badlands_plateau",
	39:  "minecraft:badlands_plateau",
	40:  "minecraft:small_end_islands",
	41:  "minecraft:end_midlands",
	42:  "minecraft:end_highlands",
	43:  "minecraft:end_barrens",
	44:  "minecraft:warm_ocean",
	45:  "minecraft:lukewarm_ocean",
	46:  "minecraft:cold_ocean",
	47:  "minecraft:deep_warm_ocean",
	48:  "minecraft:deep_lukewarm_ocean",
	49:  "minecraft:deep_cold_ocean",
	50:  "minecraft:deep_frozen_ocean",
	51:  "minecraft:seasonal_forest",
	52:  "minecraft:rainforest",
	53:  "minecraft:shrubland",
	127: "minecraft:the_void",
	129: "minecraft:sunflower_plains",
	130: "minecraft:desert_lakes",
	131: "minecraft:gravelly_mountains",
	132: "minecraft:flower_forest",
	133: "minecraft:taiga_mountains",
	134: "minecraft:swamp_hills",
	140: "minecraft:ice_spikes",
	149: "minecraft:modified_jungle",
	151: "minecraft:modified_jungle_edge",
	155: "minecraft:tall_birch_forest",
	156: "minecraft:tall_birch_hills",
	157: "minecraft:dark_forest_hills",
	158: "minecraft:snowy_taiga_mountains",
	160: "minecraft:giant_spruce_taiga",
	161: "minecraft:giant_spruce_taiga_hills",
	162: "minecraft:modified_gravelly_mountains",
	163: "minecraft:shattered_savanna",
	164: "minecraft:shattered_savanna_plateau",
	165: "minecraft:eroded_badlands",
	166: "minecraft:modified_wooded_badlands_plateau",
	167: "minecraft:modified_badlands_plateau",
	168: "minecraft:bamboo_jungle",
	169: "minecraft:bamboo_jungle_hills",
	170: "minecraft:soul_sand_valley",
	171: "minecraft:crimson_forest",
	172: "minecraft:warped_forest",
	173: "minecraft:basalt_deltas",
	174: "minecraft:dripstone_caves",
	175: "minecraft:lush_caves",
	177: "minecraft:meadow",
	178: "minecraft:grove",
	179: "minecraft:snowy_slopes",
	180: "minecraft:jagged_peaks",
	181: "minecraft:frozen_peaks",
	182: "minecraft:stony_peaks",
	183: "minecraft:deep_dark",
	184: "minecraft:mangrove_swamp",
	185: "minecraft:cherry_grove",
	186: "minecraft:pale_garden",
}

func legacyCategory(mc int, id int32) int32 {
	switch id {
	case 16, 26:
		return 16
	case 2, 17, 130:
		return 2
	case 3, 20, 34, 131, 162:
		return 3
	case 4, 18, 27, 28, 29, 132, 155, 156, 157:
		return 4
	case 12, 13, 140:
		return 12
	case 21, 22, 23, 149, 151, 168, 169:
		return 21
	case 37, 165, 166, 167:
		return 37
	case 38, 39:
		if mc <= mc1_15 {
			return biomeBadlands
		}
		return biomeBadlandsPlateau
	case 14, 15:
		return 14
	case 25:
		return 25
	case 0, 10, 24, 44, 45, 46, 47, 48, 49, 50:
		return 0
	case 1, 129:
		return 1
	case 7, 11:
		return 7
	case 35, 36, 163, 164:
		return 35
	case 6, 134:
		return 6
	case 5, 19, 30, 31, 32, 33, 133, 158, 160, 161:
		return 5
	case 8, 170, 171, 172, 173:
		return 8
	}
	return -1
}

func legacyMutated(mc int, id int32) int32 {
	switch id {
	case 1:
		return 129
	case 2:
		return 130
	case 3:
		return 131
	case 4:
		return 132
	case 5:
		return 133
	case 6:
		return 134
	case 12:
		return 140
	case 21:
		return 149
	case 23:
		return 151
	case 29:
		return 157
	case 30:
		return 158
	case 32:
		return 160
	case 33:
		return 161
	case 34:
		return 162
	case 35:
		return 163
	case 36:
		return 164
	case 37:
		return 165
	case 38:
		return 166
	case 39:
		return 167
	case biomeBirchForest:
		if mc >= mc1_9 && mc <= mc1_10 {
			return biomeTallBirchHills
		}
		return biomeTallBirchForest
	case biomeBirchForestHills:
		if mc >= mc1_9 && mc <= mc1_10 {
			return -1
		}
		return biomeTallBirchHills
	}
	return -1
}
func legacySimilar(mc int, a, b int32) bool {
	if a == b {
		return true
	}
	if mc <= mc1_15 && (a == biomeWoodedBadlandsPlateau || a == biomeBadlandsPlateau) {
		return b == biomeWoodedBadlandsPlateau || b == biomeBadlandsPlateau
	}
	return legacyCategory(mc, a) == legacyCategory(mc, b)
}
func legacyOcean(id int32) bool {
	return id >= 0 && id < 64 && (uint64(1)<<uint(id))&((1<<0)|(1<<10)|(1<<24)|(1<<44)|(1<<45)|(1<<46)|(1<<47)|(1<<48)|(1<<49)|(1<<50)) != 0
}
func legacyShallow(id int32) bool {
	return id >= 0 && id < 64 && (uint64(1)<<uint(id))&((1<<0)|(1<<10)|(1<<44)|(1<<45)|(1<<46)) != 0
}
func legacyDeep(id int32) bool { return legacyOcean(id) && !legacyShallow(id) }
func legacySnowy(id int32) bool {
	switch id {
	case 10, 11, 12, 13, 26, 30, 31, 140, 158:
		return true
	}
	return false
}
func legacyMesa(id int32) bool {
	switch id {
	case 37, 38, 39, 165, 166, 167:
		return true
	}
	return false
}
func legacyDeepen(id int32) int32 {
	switch id {
	case 0:
		return 24
	case 44:
		return 47
	case 45:
		return 48
	case 46:
		return 49
	case 10:
		return 50
	}
	return 24
}
