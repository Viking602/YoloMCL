package cubiomes

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

// Each entry has its own hash-verified official registry data and independent
// Java oracle. Never infer compatibility from a release's numeric prefix.
func latestRelease(version string) bool {
	return version == "26.1" || version == "26.1.1" || version == "26.1.2" || version == "26.2" || version == "26.3"
}

// The legacy Nether channels became separately named registry entries in
// 26.1. Their direct world-seed binding must not leak to arbitrary noises with
// the formerly special Overworld IDs in a latest-version datapack.
func latestLegacyNoiseAlias(version, id string) string {
	if !latestRelease(version) && !latestFloatRelease(version) {
		return id
	}
	switch id {
	case "minecraft:nether/temperature":
		return "minecraft:temperature"
	case "minecraft:nether/vegetation":
		return "minecraft:vegetation"
	default:
		return ""
	}
}

var latestBuiltinRecords sync.Map

// Installed parameter-list records name Java builtins with {"preset": ...}.
// Resolve those builtins from the exact release's verified parameter lists,
// independently of registry overrides (which may themselves name a builtin).
func latestVanillaClimateTree(ctx context.Context, version, preset string) (*climateTree, []string, error) {
	key := version + "/" + preset
	if value, ok := vanillaTrees.Load(key); ok {
		tree := value.(*climateTree)
		return tree, tree.biomes, nil
	}
	docs, _, err := latestBuiltinDocuments(version)
	if err != nil {
		return nil, nil, err
	}
	for _, doc := range docs {
		if doc.Kind != "multi_noise_biome_source_parameter_list" || doc.ID != preset {
			continue
		}
		var data struct {
			Biomes json.RawMessage `json:"biomes"`
		}
		if err := json.Unmarshal(doc.Data, &data); err != nil {
			return nil, nil, err
		}
		tree, _, err := parseClimateList(ctx, data.Biomes, climateTreeOrder(version))
		if err != nil {
			return nil, nil, err
		}
		value, _ := vanillaTrees.LoadOrStore(key, tree)
		tree = value.(*climateTree)
		return tree, tree.biomes, nil
	}
	return nil, nil, fmt.Errorf("unknown biome preset %s", preset)
}

func latestBuiltinDocuments(version string) ([]Document, bool, error) {
	if !latestRelease(version) {
		return nil, false, nil
	}
	value, _ := latestBuiltinRecords.LoadOrStore(version, &builtinRecord{})
	record := value.(*builtinRecord)
	record.once.Do(func() { record.docs, record.err = loadLatestBuiltinDocuments(version) })
	if record.err != nil {
		return nil, true, record.err
	}
	out := make([]Document, len(record.docs))
	for i, doc := range record.docs {
		out[i] = doc
		out[i].Data = append(json.RawMessage(nil), doc.Data...)
	}
	return out, true, nil
}

func loadLatestBuiltinDocuments(version string) ([]Document, error) {
	b, err := readSeedMapBuiltin("latest-" + version + ".json")
	if err != nil {
		return nil, err
	}
	var registries map[string]map[string]json.RawMessage
	if err := json.Unmarshal(b, &registries); err != nil {
		return nil, err
	}
	var docs []Document
	for kind, entries := range registries {
		for id, data := range entries {
			docs = append(docs, Document{Kind: kind, ID: id, Source: "builtin:" + version, Data: data})
		}
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Kind+"/"+docs[i].ID < docs[j].Kind+"/"+docs[j].ID })
	return docs, nil
}
