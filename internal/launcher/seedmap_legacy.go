package launcher

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/Viking602/YoloMCL/cubiomes"
)

func seedMapLegacySavedDimensions(version string, level LevelInfo) map[string]json.RawMessage {
	name := strings.ToLower(level.LegacyGenerator)
	if name == "" && strings.HasPrefix(version, "b1.") {
		name = "default"
	}
	if level.LegacyGeneratorOptions != "" || (name != "default" && name != "largebiomes" && name != "amplified" && name != "default_1_1") {
		return nil
	}
	docs, err := cubiomes.BuiltinDocuments(version)
	if err != nil {
		return nil
	}
	for _, doc := range docs {
		if doc.Kind != "world_preset" || doc.ID != "minecraft:normal" {
			continue
		}
		var preset struct{ Dimensions map[string]json.RawMessage }
		if json.Unmarshal(doc.Data, &preset) != nil {
			return nil
		}
		var overworld struct {
			Generator struct {
				Type     string          `json:"type"`
				Settings json.RawMessage `json:"settings"`
				Source   map[string]any  `json:"biome_source"`
			} `json:"generator"`
		}
		if json.Unmarshal(preset.Dimensions["minecraft:overworld"], &overworld) != nil || overworld.Generator.Source["type"] != "minecraft:vanilla_layered" {
			return nil
		}
		overworld.Generator.Source["large_biomes"] = name == "largebiomes"
		minor := 0
		if parts := strings.Split(version, "."); len(parts) > 1 {
			minor, _ = strconv.Atoi(parts[1])
		}
		if name == "default_1_1" || (name == "default" && level.HasLegacyGeneratorVersion && level.LegacyGeneratorVersion == 0 && minor >= 2) {
			overworld.Generator.Source["legacy_biome_init_layer"] = true
		}
		b, err := json.Marshal(overworld)
		if err != nil {
			return nil
		}
		preset.Dimensions["minecraft:overworld"] = b
		return preset.Dimensions
	}
	return nil
}
