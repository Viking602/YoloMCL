package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Viking602/YoloMCL/cubiomes"
)

func bindLithostitchedWorldgen(ctx context.Context, snapshot *WorldgenSnapshot, dimension string, definition seedMapDimension, base *cubiomes.Generator) (*cubiomes.Generator, error) {
	docs := map[string]WorldgenDocument{}
	native := map[string]cubiomes.Document{}
	var injectors []WorldgenDocument
	for _, doc := range snapshot.Documents {
		docs[doc.Kind+"/"+doc.ID] = doc
		native[doc.Kind+"/"+doc.ID] = cubiomes.Document{Kind: doc.Kind, ID: doc.ID, Data: doc.Data}
		if doc.Kind == "lithostitched_biome_injector" {
			injectors = append(injectors, doc)
		}
	}
	if len(injectors) == 0 {
		return base, nil
	}
	version := ""
	for _, mod := range snapshot.Mods {
		if mod.ID == "lithostitched" {
			version = mod.Version
		}
	}
	if version == "" {
		return base, nil
	}
	cfg := cubiomes.BiomeInjectionConfig{}
	var settings struct {
		Legacy bool `json:"legacy_random_source"`
	}
	raw := definition.settings
	var ref string
	if json.Unmarshal(raw, &ref) == nil {
		raw = docs["noise_settings/"+seedMapID(ref)].Data
	}
	if json.Unmarshal(raw, &settings) == nil {
		cfg.LegacyRandom = settings.Legacy
	}
	expand := func(raw json.RawMessage) ([]string, error) {
		budget := 32768
		values, ok := resolveSeedMapBiomes(ctx, docs, raw, map[string]bool{}, &budget)
		if !ok {
			return nil, fmt.Errorf("invalid Lithostitched biome set: %s", raw)
		}
		out := make([]string, 0, len(values))
		for value := range values {
			out = append(out, value)
		}
		sort.Strings(out)
		return out, nil
	}
	for _, doc := range injectors {
		var data struct {
			Type, Dimension, Region, Replacement, Biome string
			Priority                                    int
			Targets                                     json.RawMessage
			Parameters                                  map[string]json.RawMessage
			LoadPredicate                               json.RawMessage `json:"predicate"`
		}
		data.Priority = 1000
		if err := json.Unmarshal(doc.Data, &data); err != nil {
			return nil, err
		}
		if seedMapID(data.Dimension) != dimension {
			continue
		}
		if len(data.LoadPredicate) > 0 && string(data.LoadPredicate) != "null" {
			return nil, fmt.Errorf("lithostitched load predicate: %s", doc.ID)
		}
		rule := cubiomes.BiomeInjection{ID: doc.ID, Type: strings.TrimPrefix(data.Type, "lithostitched:"), Priority: data.Priority, Parameters: data.Parameters, Region: data.Region, Replacement: data.Replacement}
		if !strings.HasPrefix(data.Type, "lithostitched:") {
			return nil, fmt.Errorf("unknown biome injector: %s", data.Type)
		}
		if rule.Type == "force_placement" {
			rule.Replacement = data.Biome
		} else {
			var err error
			rule.Targets, err = expand(data.Targets)
			if err != nil {
				return nil, err
			}
		}
		if rule.Replacement != "" {
			rule.Replacement = seedMapID(rule.Replacement)
		}
		if rule.Region != "" {
			rule.Region = seedMapID(rule.Region)
		}
		cfg.Rules = append(cfg.Rules, rule)
	}
	if len(cfg.Rules) == 0 {
		return base, nil
	}
	if snapshot.Minecraft != "1.21.1" || version != "1.7.13" && version != "1.7.13-fabric-21.1" {
		return nil, fmt.Errorf("lithostitched biome injector version: %s", version)
	}
	for _, doc := range snapshot.Documents {
		if doc.Kind != "lithostitched_region" {
			continue
		}
		var data struct {
			Dimension string
			Biomes    json.RawMessage
			Weight    int
		}
		if err := json.Unmarshal(doc.Data, &data); err != nil {
			return nil, err
		}
		if seedMapID(data.Dimension) != dimension {
			continue
		}
		biomes, err := expand(data.Biomes)
		if err != nil {
			return nil, err
		}
		cfg.Regions = append(cfg.Regions, cubiomes.BiomeRegion{ID: doc.ID, Weight: data.Weight, Biomes: biomes})
	}
	_, path, _ := strings.Cut(dimension, ":")
	if _, ok := docs["density_function/lithostitched:region/"+path]; ok {
		cfg.RegionDensity, _ = json.Marshal("lithostitched:region/" + path)
	}
	return cubiomes.WithBiomeInjections(ctx, base, native, cfg)
}
