package launcher

import "encoding/json"

// injectSupplementariesDynamicTags mirrors Supplementaries' Forge-only
// ServerDynamicResourcesGenerator: when the mod is installed and the
// moonlight-generated biome tag is absent from jar/datapack sources,
// HAS_WAY_SIGNS is published as #minecraft:is_overworld (javap BiomeTags.f_215817_).
// Datapack or jar definitions win; we never overwrite.
func injectSupplementariesDynamicTags(docs map[string]WorldgenDocument, snap *WorldgenSnapshot) {
	if snap == nil || !worldgenHasMod(snap, "supplementaries") {
		return
	}
	const key = "biome_tag/supplementaries:has_way_signs"
	if _, found := docs[key]; found {
		return
	}
	// way_sign structure must exist; skip if a pack removed it.
	if _, found := docs["structure/supplementaries:way_sign"]; !found {
		return
	}
	data := json.RawMessage(`{"replace":false,"values":["#minecraft:is_overworld"]}`)
	docs[key] = WorldgenDocument{
		ID:     "supplementaries:has_way_signs",
		Kind:   "biome_tag",
		Source: "synthetic:supplementaries_generated",
		Data:   data,
	}
}
