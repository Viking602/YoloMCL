package launcher

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/Viking602/YoloMCL/cubiomes"
)

// An adapter translates source-verified mod registration/configuration into
// reusable native engine rules. Add adapters here for new injection families;
// TerraBlender-based mods share the engine's weighted region and climate code.
type worldgenNativeAdapter struct {
	ModIDs []string
	Bind   func(context.Context, *WorldgenSnapshot, string, seedMapDimension, *cubiomes.Generator) (*cubiomes.Generator, error)
}

var worldgenNativeAdapters = []worldgenNativeAdapter{
	{[]string{"terrablender"}, bindTerraBlenderWorldgen},
	{[]string{"lithostitched"}, bindLithostitchedWorldgen},
}

type worldgenAdapterCacheEntry struct {
	key string
	gen *cubiomes.Generator
}

var worldgenAdapterCache = struct {
	sync.Mutex
	entries list.List
}{}

func applyNativeWorldgen(ctx context.Context, snapshot *WorldgenSnapshot, dimension string, definition seedMapDimension, seed int64, base *cubiomes.Generator) (*cubiomes.Generator, error) {
	var adapters []worldgenNativeAdapter
	for _, adapter := range worldgenNativeAdapters {
		matched := false
		for _, mod := range snapshot.Mods {
			for _, id := range adapter.ModIDs {
				matched = matched || mod.ID == id
			}
		}
		if matched {
			adapters = append(adapters, adapter)
		}
	}
	if len(adapters) == 0 {
		return base, nil
	}
	key := snapshot.Hash + "/" + dimension
	if snapshot.Hash != "" {
		worldgenAdapterCache.Lock()
		for en := worldgenAdapterCache.entries.Front(); en != nil; en = en.Next() {
			cached := en.Value.(worldgenAdapterCacheEntry)
			if cached.key == key {
				worldgenAdapterCache.entries.MoveToFront(en)
				worldgenAdapterCache.Unlock()
				return cached.gen.WithSeed(ctx, seed)
			}
		}
		worldgenAdapterCache.Unlock()
	}
	for _, adapter := range adapters {
		var err error
		base, err = adapter.Bind(ctx, snapshot, dimension, definition, base)
		if err != nil {
			return nil, &cubiomes.CompileError{Code: "density_unsupported", Detail: strings.Join(adapter.ModIDs, "/") + ": " + err.Error()}
		}
	}
	if snapshot.Hash != "" {
		worldgenAdapterCache.Lock()
		worldgenAdapterCache.entries.PushFront(worldgenAdapterCacheEntry{key, base})
		for worldgenAdapterCache.entries.Len() > 8 {
			worldgenAdapterCache.entries.Remove(worldgenAdapterCache.entries.Back())
		}
		worldgenAdapterCache.Unlock()
	}
	return base, nil
}

func bindTerraBlenderWorldgen(ctx context.Context, snapshot *WorldgenSnapshot, _ string, definition seedMapDimension, base *cubiomes.Generator) (*cubiomes.Generator, error) {
	// Validate source profiles only for dimensions to which this adapter binds.
	// Code in an unrelated mod or dimension is not a world-generation hook.
	dimension, applies, err := terraBlenderDimension(ctx, snapshot, definition)
	if err != nil {
		return nil, err
	}
	if !applies {
		return base, nil
	}
	if snapshot.Minecraft == "1.20.1" {
		return bindTerraBlenderRegistration(ctx, snapshot, dimension, base)
	}
	// Keep existing exact-artifact profiles while their generic API bindings
	// are expanded. Other consumers use their captured programs, not a new
	// per-mod parameter table or a blanket mod-name rejection.
	if generator, profileErr := bindTerraBlenderProfile(ctx, snapshot, dimension, base); profileErr == nil {
		return generator, nil
	} else if len(snapshot.RegionPrograms) == 0 {
		return nil, profileErr
	}
	return bindTerraBlenderRegistration(ctx, snapshot, dimension, base)
}

func bindTerraBlenderProfile(ctx context.Context, snapshot *WorldgenSnapshot, dimension cubiomes.Dimension, base *cubiomes.Generator) (*cubiomes.Generator, error) {
	mods := make(map[string]WorldgenMod, len(snapshot.Mods))
	for _, mod := range snapshot.Mods {
		if mod.TerraBlender && mod.ID != "biomesoplenty" {
			return nil, fmt.Errorf("no verified region registration for %s", mod.ID)
		}
		// Only identities used by this contract can make it ambiguous. An
		// unrelated duplicate selected by the loader does not alter biome rules.
		if mod.ID != "terrablender" && mod.ID != "biomesoplenty" && mod.ID != "glitchcore" && mod.ID != "fabric-api" {
			continue
		}
		if _, exists := mods[mod.ID]; exists {
			return nil, fmt.Errorf("duplicate installed mod %s", mod.ID)
		}
		mods[mod.ID] = mod
	}
	bopVersion := mods["biomesoplenty"].Version
	if _, exists := mods["biomesoplenty"]; exists && bopVersion == "" {
		return nil, fmt.Errorf("Biomes O' Plenty version is missing")
	}
	artifacts, err := cubiomes.BuiltinTerraBlenderArtifacts(snapshot.Minecraft, bopVersion)
	if err != nil {
		return nil, err
	}
	for _, artifact := range artifacts {
		mod := mods[artifact.ID]
		if mod.Loader != "fabric" || mod.Version != artifact.Version || mod.SHA512 != artifact.SHA512 {
			return nil, fmt.Errorf("installed %s does not match source profile %s", artifact.ID, artifact.Version)
		}
	}
	toggles, err := readBOPBiomeToggles(snapshot, bopVersion)
	if err != nil {
		return nil, err
	}
	tb, err := parseWorldgenModSettings(snapshot, "terrablender.toml", worldgenModSettings{
		"general": {"overworld_region_size": 3, "nether_region_size": 2, "vanilla_overworld_region_weight": 10, "vanilla_nether_region_weight": 10},
		"end": {"highlands_biome_size": 4, "midlands_biome_size": 4, "edge_biome_size": 3, "island_biome_size": 2,
			"vanilla_end_highlands_weight": 10, "vanilla_end_midlands_weight": 10, "vanilla_end_barrens_weight": 10, "vanilla_small_end_islands_weight": 10},
	})
	if err != nil {
		return nil, err
	}
	bop, err := parseWorldgenModSettings(snapshot, "biomesoplenty/generation.toml", worldgenModSettings{
		"overworld": {"bop_primary_overworld_region_weight": 10, "bop_secondary_overworld_region_weight": 8, "bop_overworld_rare_region_weight": 2},
		"nether":    {"bop_nether_region_weight": 13, "bop_nether_rare_region_weight": 2},
	})
	if err != nil && bopVersion != "" {
		return nil, err
	}
	if dimension == cubiomes.End {
		cfg, err := cubiomes.BuiltinTerraBlenderEndProfileWithToggles(snapshot.Minecraft, bopVersion, toggles)
		if err != nil {
			return nil, err
		}
		for _, area := range []struct {
			table     *cubiomes.TerraBlenderEndTable
			size, key string
		}{
			{&cfg.Highlands, "highlands_biome_size", "vanilla_end_highlands_weight"},
			{&cfg.Midlands, "midlands_biome_size", "vanilla_end_midlands_weight"},
			{&cfg.Edge, "edge_biome_size", "vanilla_end_barrens_weight"},
			{&cfg.Islands, "island_biome_size", "vanilla_small_end_islands_weight"},
		} {
			area.table.Size = int(tb["end"][area.size])
			area.table.Biomes[0].Weight = int(tb["end"][area.key])
		}
		return cubiomes.WithTerraBlenderEnd(ctx, base, cfg)
	}
	if bop == nil {
		bop = worldgenModSettings{}
	}
	var dim cubiomes.Dimension
	var size string
	var weights map[string]int64
	switch dimension {
	case cubiomes.Overworld:
		dim, size = cubiomes.Overworld, "overworld_region_size"
		weights = map[string]int64{"minecraft:overworld": tb["general"]["vanilla_overworld_region_weight"],
			"biomesoplenty:overworld_primary":   bop["overworld"]["bop_primary_overworld_region_weight"],
			"biomesoplenty:overworld_secondary": bop["overworld"]["bop_secondary_overworld_region_weight"],
			"biomesoplenty:overworld_rare":      bop["overworld"]["bop_overworld_rare_region_weight"]}
	case cubiomes.Nether:
		dim, size = cubiomes.Nether, "nether_region_size"
		weights = map[string]int64{"minecraft:nether": tb["general"]["vanilla_nether_region_weight"],
			"biomesoplenty:nether_common": bop["nether"]["bop_nether_region_weight"],
			"biomesoplenty:nether_rare":   bop["nether"]["bop_nether_rare_region_weight"]}
	default:
		return base, nil
	}
	cfg, err := cubiomes.BuiltinTerraBlenderProfileWithToggles(snapshot.Minecraft, bopVersion, dim, toggles)
	if err != nil {
		return nil, err
	}
	cfg.RegionSize = int(tb["general"][size])
	for i := range cfg.Regions {
		cfg.Regions[i].Weight = int(weights[cfg.Regions[i].ID])
	}
	return cubiomes.WithTerraBlender(ctx, base, cfg)
}

func readBOPBiomeToggles(snapshot *WorldgenSnapshot, version string) (map[string]bool, error) {
	if version == "" {
		return nil, nil
	}
	raw, exists := snapshot.ModConfigs["biomesoplenty/biome_toggles.json"]
	if !exists {
		return nil, nil
	}
	var toggles map[string]bool
	if err := json.Unmarshal(raw, &toggles); err != nil {
		return nil, fmt.Errorf("invalid BOP biome toggles: %w", err)
	}
	return toggles, nil
}

// TerraBlender binds to dimension TYPE tags, not dimension IDs or biome
// presets. End source hooks take priority; an End-tagged multi-noise source
// receives no regional hook. This mirrors LevelUtils.initializeBiomes.
func terraBlenderDimension(ctx context.Context, snapshot *WorldgenSnapshot, d seedMapDimension) (cubiomes.Dimension, bool, error) {
	if d.generator != "minecraft:noise" {
		return 0, false, nil
	}
	if d.mode == "the_end" {
		if snapshot.Minecraft == "1.20.1" {
			return 0, false, nil
		}
		return cubiomes.End, true, nil
	}
	if d.mode != "multi_noise" || d.dimensionType == "" {
		return 0, false, nil
	}
	docs := make(map[string]WorldgenDocument, len(snapshot.Documents))
	for _, doc := range snapshot.Documents {
		docs[doc.Kind+"/"+doc.ID] = doc
	}
	for _, candidate := range []struct {
		tag       string
		dimension cubiomes.Dimension
	}{
		{"end_regions", cubiomes.End}, {"nether_regions", cubiomes.Nether}, {"overworld_regions", cubiomes.Overworld},
	} {
		if snapshot.Minecraft == "1.20.1" && candidate.dimension == cubiomes.End {
			continue
		}
		id := "terrablender:" + candidate.tag
		if _, exists := docs["dimension_type_tag/"+id]; !exists {
			continue
		}
		raw, _ := json.Marshal("#" + id)
		budget := 32768
		members, ok := resolveSeedMapRegistry(ctx, docs, "dimension_type", raw, map[string]bool{}, &budget)
		if !ok {
			return 0, false, fmt.Errorf("invalid dimension type tag %s", id)
		}
		if members[d.dimensionType] {
			return candidate.dimension, candidate.dimension != cubiomes.End, nil
		}
	}
	return 0, false, nil
}
