package launcher

import "encoding/json"

// CarverFluid is what the Nether carver writes into a carved block. Per the
// 1.21.1 NetherWorldCarver.carveBlock bytecode (javap of the srg client jar):
// a block in the config's replaceable tag becomes lava when
// y <= CarvingContext.getMinGenY()+31, else cave air. The configured
// lava_level and the noise_settings sea level are not read by that class.
type CarverFluid struct {
	LavaY int
	OK    bool
}

// Substance is the carved block at y.
func (f CarverFluid) Substance(y int) TerrainSubstance {
	if y <= f.LavaY {
		return TerrainLava
	}
	return TerrainAir
}

// NetherCarverFluid reports the nether_cave carver's fluid rule for dimension.
func (s *LocalSampler) NetherCarverFluid(dimension string) CarverFluid {
	if s == nil {
		return CarverFluid{}
	}
	return s.dimensions[dimension].carverFluid
}

// compileCarverFluid needs the pack's configured carver to still be of type
// minecraft:nether_cave (a datapack may re-point it to another carver type).
func compileCarverFluid(docs map[string]WorldgenDocument, carver string, minY int) CarverFluid {
	var cc struct {
		Type string `json:"type"`
	}
	doc, ok := docs["configured_carver/"+carver]
	if !ok || json.Unmarshal(doc.Data, &cc) != nil || cc.Type != "minecraft:nether_cave" {
		return CarverFluid{}
	}
	return CarverFluid{LavaY: minY + 31, OK: true}
}
