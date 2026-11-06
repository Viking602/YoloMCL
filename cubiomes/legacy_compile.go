package cubiomes

import (
	"encoding/json"
	"fmt"
)

// Older releases have no world_preset registry. These normalized adapter
// records describe the built-in sources; they are not exported game datapacks.
func legacyBuiltinDocuments(version string, mc int) []Document {
	dimensions := map[string]json.RawMessage{}
	for _, d := range []struct{ id, settings, source string }{
		{"overworld", "overworld", `{"type":"minecraft:vanilla_layered","large_biomes":false}`},
		{"the_nether", "nether", `{"type":"minecraft:multi_noise","preset":"minecraft:nether"}`},
		{"the_end", "end", `{"type":"minecraft:the_end"}`},
	} {
		if d.id == "the_end" && mc < mc1_0 {
			continue
		}
		source := d.source
		if d.id == "the_nether" && mc < mc1_16_1 {
			source = `{"type":"minecraft:fixed","biome":"minecraft:nether_wastes"}`
		}
		if d.id == "the_end" && mc < mc1_9 {
			source = `{"type":"minecraft:fixed","biome":"minecraft:the_end"}`
		}
		dimensions["minecraft:"+d.id] = json.RawMessage(fmt.Sprintf(`{"generator":{"type":"minecraft:noise","settings":"minecraft:%s","biome_source":%s}}`, d.settings, source))
	}
	data, _ := json.Marshal(struct {
		Dimensions map[string]json.RawMessage `json:"dimensions"`
	}{dimensions})
	return []Document{{ID: "minecraft:normal", Kind: "world_preset", Source: "builtin:" + version, Data: data}}
}

func compileLegacy(docs map[string]Document, def Definition, mc int) (*Generator, error) {
	unsupported := func(detail string) (*Generator, error) {
		return nil, &CompileError{Code: "biome_source_unsupported", Detail: detail}
	}
	var source struct {
		Type, Preset string
		Seed         *int64
		Large        bool `json:"large_biomes"`
		OldInit      bool `json:"legacy_biome_init_layer"`
	}
	var fields map[string]json.RawMessage
	var settings string
	if json.Unmarshal(def.BiomeSource, &source) != nil || json.Unmarshal(def.BiomeSource, &fields) != nil || json.Unmarshal(def.Settings, &settings) != nil {
		return unsupported("legacy source requires vanilla noise settings")
	}
	settings = seedMapID(settings)
	if _, exists := docs["noise_settings/"+settings]; exists {
		return unsupported("legacy noise settings override: " + settings)
	}
	cfg := Config{Version: def.Version, Seed: def.Seed}
	allowed := map[string]bool{"type": true, "seed": true}
	switch seedMapID(source.Type) {
	case "minecraft:vanilla_layered":
		if settings != "minecraft:overworld" && settings != "minecraft:large_biomes" && settings != "minecraft:amplified" {
			return unsupported(settings)
		}
		if source.OldInit {
			return unsupported("legacy_biome_init_layer")
		}
		cfg.LargeBiomes = source.Large || settings == "minecraft:large_biomes"
		allowed["large_biomes"], allowed["legacy_biome_init_layer"] = true, true
	case "minecraft:multi_noise":
		if mc < mc1_16_1 || settings != "minecraft:nether" || seedMapID(source.Preset) != "minecraft:nether" {
			return unsupported("legacy multi_noise preset")
		}
		if _, exists := docs["multi_noise_biome_source_parameter_list/minecraft:nether"]; exists {
			return unsupported("legacy Nether preset override")
		}
		cfg.Dimension = Nether
		allowed["preset"] = true
	case "minecraft:the_end":
		if settings != "minecraft:end" {
			return unsupported(settings)
		}
		cfg.Dimension = End
	default:
		return unsupported(source.Type)
	}
	for key := range fields {
		if !allowed[key] {
			return unsupported("legacy biome source field: " + key)
		}
	}
	if source.Seed != nil {
		cfg.Seed = *source.Seed
	}
	g, err := newLegacyGenerator(cfg, mc)
	if err == nil {
		g.sha = seedSHA(def.Seed)
		g.seed = def.Seed
		g.legacy.fixedSeed = source.Seed != nil
	}
	return g, err
}
