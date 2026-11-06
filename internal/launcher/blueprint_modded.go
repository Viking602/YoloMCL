package launcher

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pelletier/go-toml/v2"

	"github.com/Viking602/YoloMCL/cubiomes"
)

// blueprintModdedSource is the saved/runtime biome_source wrapper written by
// Blueprint's ModdedBiomeSlicesManager.
type blueprintModdedSource struct {
	Type                string          `json:"type"`
	OriginalBiomeSource json.RawMessage `json:"original_biome_source"`
}

func parseBlueprintModded(raw json.RawMessage) (original json.RawMessage, ok bool) {
	var src blueprintModdedSource
	if json.Unmarshal(raw, &src) != nil || seedMapID(src.Type) != "blueprint:modded" {
		return nil, false
	}
	if len(src.OriginalBiomeSource) == 0 || string(src.OriginalBiomeSource) == "null" {
		return nil, false
	}
	return src.OriginalBiomeSource, true
}

func blueprintSliceSize(snapshot *WorldgenSnapshot, dimension string) (int, error) {
	const defaultSize = 9 // Blueprint code default when config is absent or non-positive
	raw, exists := snapshot.ModConfigs["blueprint-common.toml"]
	if !exists {
		return defaultSize, nil
	}
	if raw == nil {
		return 0, fmt.Errorf("unreadable worldgen configuration blueprint-common.toml")
	}
	var doc struct {
		Sizes map[string]int64 `toml:"modded_biome_slice_sizes"`
	}
	if err := toml.Unmarshal(raw, &doc); err != nil {
		return 0, fmt.Errorf("invalid blueprint-common.toml: %w", err)
	}
	size := defaultSize
	if v, ok := doc.Sizes["default"]; ok && v > 0 && v <= 32 {
		size = int(v)
	}
	if v, ok := doc.Sizes[dimension]; ok && v > 0 && v <= 32 {
		size = int(v)
	}
	return size, nil
}

func collectBlueprintSlices(snapshot *WorldgenSnapshot, dimension string, known map[string]bool) ([]cubiomes.BlueprintModdedSlice, error) {
	var slices []cubiomes.BlueprintModdedSlice
	for _, doc := range snapshot.Documents {
		if doc.Kind != "modded_biome_slice" || string(doc.Data) == "null" {
			continue
		}
		var body struct {
			Levels   []string `json:"levels"`
			Weight   int      `json:"weight"`
			Provider struct {
				Type     string `json:"type"`
				Overlays []struct {
					MatchesBiomes json.RawMessage `json:"matches_biomes"`
					BiomeSource   struct {
						Type   string          `json:"type"`
						Biome  string          `json:"biome"`
						Biomes json.RawMessage `json:"biomes"`
						Scale  *int            `json:"scale"`
					} `json:"biome_source"`
				} `json:"overlays"`
				Areas           map[string]string `json:"areas"`
				Biomes          json.RawMessage   `json:"biomes"`
				OnlyMapFromArea bool              `json:"only_map_from_areas"`
			} `json:"provider"`
		}
		if err := json.Unmarshal(doc.Data, &body); err != nil {
			return nil, fmt.Errorf("invalid modded biome slice %s: %w", doc.ID, err)
		}
		if body.Weight <= 0 {
			continue
		}
		applies := false
		for _, level := range body.Levels {
			if seedMapID(level) == dimension {
				applies = true
				break
			}
		}
		if !applies {
			continue
		}
		provider := cubiomes.BlueprintModdedProvider{Type: seedMapID(body.Provider.Type)}
		switch provider.Type {
		case "blueprint:original":
		case "blueprint:multi_noise":
			raw, err := resolveBlueprintMultiNoise(body.Provider.Areas, body.Provider.Biomes, body.Provider.OnlyMapFromArea, known)
			if err != nil {
				return nil, fmt.Errorf("modded biome slice %s: %w", doc.ID, err)
			}
			provider.MultiNoise = raw
		case "blueprint:overlay":
			for _, overlay := range body.Provider.Overlays {
				matches, err := decodeBlueprintBiomeList(overlay.MatchesBiomes)
				if err != nil {
					return nil, fmt.Errorf("modded biome slice %s: %w", doc.ID, err)
				}
				biome := seedMapID(overlay.BiomeSource.Biome)
				if biome == "" {
					ids, err := decodeBlueprintBiomeList(overlay.BiomeSource.Biomes)
					if err != nil {
						return nil, fmt.Errorf("modded biome slice %s: %w", doc.ID, err)
					}
					if len(ids) == 1 {
						biome = ids[0]
					} else if len(ids) > 1 {
						return nil, fmt.Errorf("modded biome slice %s: multi-biome overlay sources are not bound", doc.ID)
					}
				}
				if biome == "" || len(matches) == 0 {
					return nil, fmt.Errorf("modded biome slice %s: overlay missing biome or matches", doc.ID)
				}
				scale := 0
				if overlay.BiomeSource.Scale != nil {
					scale = *overlay.BiomeSource.Scale
				}
				provider.Overlays = append(provider.Overlays, cubiomes.BlueprintModdedOverlay{
					Matches: matches, Biome: biome, Scale: scale,
				})
			}
			if len(provider.Overlays) == 0 {
				return nil, fmt.Errorf("modded biome slice %s: empty overlay provider", doc.ID)
			}
		default:
			return nil, fmt.Errorf("modded biome slice %s: unsupported provider %q", doc.ID, provider.Type)
		}
		slices = append(slices, cubiomes.BlueprintModdedSlice{ID: doc.ID, Weight: body.Weight, Provider: provider})
	}
	return slices, nil
}

func decodeBlueprintBiomeList(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var one string
	if json.Unmarshal(raw, &one) == nil {
		if one == "" {
			return nil, nil
		}
		return []string{seedMapID(one)}, nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(many))
	for _, id := range many {
		if id == "" {
			continue
		}
		out = append(out, seedMapID(id))
	}
	return out, nil
}

func applyBlueprintModded(ctx context.Context, snapshot *WorldgenSnapshot, dimension string, seed int64, base *cubiomes.Generator) (*cubiomes.Generator, error) {
	known := map[string]bool{}
	for _, doc := range snapshot.Documents {
		if doc.Kind == "biome" {
			known[doc.ID] = string(doc.Data) != "null"
		}
	}
	for _, name := range base.Biomes() {
		known[name] = true
	}
	slices, err := collectBlueprintSlices(snapshot, dimension, known)
	if err != nil {
		return nil, err
	}
	if len(slices) == 0 {
		return base, nil
	}
	size, err := blueprintSliceSize(snapshot, dimension)
	if err != nil {
		return nil, err
	}
	for _, slice := range slices {
		for _, overlay := range slice.Provider.Overlays {
			if !known[overlay.Biome] {
				return nil, fmt.Errorf("blueprint overlay biome absent from effective registry: %s", overlay.Biome)
			}
		}
	}
	return cubiomes.WithBlueprintModded(ctx, base, cubiomes.BlueprintModdedConfig{
		Size: size, DimID: dimension, Seed: seed, Slices: slices,
	})
}

// resolveBlueprintMultiNoise ports MultiNoiseModdedBiomeProvider's codec
// resolution: an entry whose key is in areas maps to that area biome;
// otherwise only_map_from_areas yields the original-source marker, else the
// registry entry or the marker when absent. A preset-name list is not bound.
func resolveBlueprintMultiNoise(areas map[string]string, raw json.RawMessage, onlyAreas bool, known map[string]bool) (json.RawMessage, error) {
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("multi_noise biomes must be an inline parameter list: %w", err)
	}
	mapped := make(map[string]string, len(areas))
	for k, v := range areas {
		mapped[seedMapID(k)] = seedMapID(v)
	}
	for _, entry := range entries {
		var id string
		if err := json.Unmarshal(entry["biome"], &id); err != nil || id == "" {
			return nil, fmt.Errorf("multi_noise entry missing biome")
		}
		id = seedMapID(id)
		var out string
		if area, ok := mapped[id]; ok {
			if !known[area] {
				return nil, fmt.Errorf("multi_noise area biome absent from effective registry: %s", area)
			}
			out = area
		} else if onlyAreas || !known[id] {
			out = cubiomes.BlueprintOriginalSourceMarker
		} else {
			out = id
		}
		b, _ := json.Marshal(out)
		entry["biome"] = b
	}
	return json.Marshal(entries)
}
