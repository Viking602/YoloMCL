package launcher

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/Viking602/YoloMCL/cubiomes"
)

const terraBlender1201SHA512 = "ef0c98e3d652f420a08a8d4205661ecb61c990ff19fabe04b9a0334b6a1bc671079dde74199a847e25df96f2a8262a35f37a0fca3c8fec95f7139074734ff8f5"

// terraBlender1201AltSHA512 is Forge 3.0.1.7. Every .class file matches 3.0.1.10;
// only the jar manifest version stamp differs. BOP 19.0.x accepts versionRange [3.0.1.7,).
const terraBlender1201AltSHA512 = "1603c7aeb53ba990d0c836a28adb48e9d27e90bfc88f3456e7a62ee737165e631508ed82fd12bedb38dad2c6cb900e8591bff90699c794b79337a33e42fc3288"

// terraBlender1211NeoSHA512 is the NeoForge 4.1.0.8 build. javap -c of all 61 classes shared with
// the pinned Fabric build matches opcode-for-opcode after Mojang/intermediary
// renaming (only ldc vs ldc_w constant-pool widths differ).
const terraBlender1211NeoSHA512 = "d51720b21b4cdaeaf962416575f27209219941b8a0067b0bccab478227f32ad9d91a2b94469925bb7b810ffb446d15b02a5381a5425b3f54739f2f174034870e"

// The framework is versioned once. Consumer identities, configuration names,
// weights and ordered climate recipes come from their own JAR registrations.
func bindTerraBlenderRegistration(ctx context.Context, snapshot *WorldgenSnapshot, dimension cubiomes.Dimension, base *cubiomes.Generator) (*cubiomes.Generator, error) {
	// Report the actual unresolved call before a kernel contract check. Merely
	// recognizing a mod or decoding its entrypoint does not supply valid regions.
	for _, program := range snapshot.RegionPrograms {
		if program.Error != "" {
			// One mod's setup failing drops only that mod's regions; the
			// sampler marks the dimension approximate (region_skipped).
			continue
		}
		if program.Result == nil {
			return nil, fmt.Errorf("missing registration result for %s", program.ModID)
		}
	}
	verified := false
	version, section := "", "general"
	for _, mod := range snapshot.Mods {
		if mod.ID == "terrablender" {
			if verified {
				return nil, fmt.Errorf("duplicate TerraBlender framework")
			}
			if snapshot.Minecraft == "1.20.1" {
				match310 := mod.Version == "3.0.1.10" && mod.SHA512 == terraBlender1201SHA512
				match307 := mod.Version == "3.0.1.7" && mod.SHA512 == terraBlender1201AltSHA512
				if mod.Loader != "forge" || (!match310 && !match307) {
					return nil, fmt.Errorf("TerraBlender does not match the 1.20.1 / 3.0.1.10 framework contract")
				}
				section = "generation_settings"
				verified = true
				version = "3.0.1.10" // class-identical contract id for WithTerraBlender
			} else if artifacts, err := cubiomes.BuiltinTerraBlenderArtifacts(snapshot.Minecraft, ""); err == nil {
				for _, artifact := range artifacts {
					if artifact.ID == "terrablender" && mod.Version == artifact.Version && (mod.Loader == "fabric" && mod.SHA512 == artifact.SHA512 || mod.Loader == "neoforge" && snapshot.Minecraft == "1.21.1" && mod.SHA512 == terraBlender1211NeoSHA512) {
						verified = true
					}
				}
			}
			if version == "" {
				version = mod.Version
			}
		}
	}
	if !verified {
		return nil, fmt.Errorf("TerraBlender kernel contract unavailable for %s / %s", snapshot.Minecraft, version)
	}
	settings, err := parseWorldgenModSettings(snapshot, "terrablender.toml", worldgenModSettings{
		section: {"overworld_region_size": 3, "nether_region_size": 2, "vanilla_overworld_region_weight": 10, "vanilla_nether_region_weight": 10},
	})
	if err != nil {
		return nil, err
	}
	prefix := "overworld"
	if dimension == cubiomes.Nether {
		prefix = "nether"
	}
	values := settings[section]
	cfg := cubiomes.TerraBlenderConfig{Minecraft: snapshot.Minecraft, Version: version, Dimension: dimension, RegionSize: int(values[prefix+"_region_size"]),
		Regions: []cubiomes.TerraBlenderRegion{{ID: "minecraft:" + prefix, Base: true, Weight: int(values["vanilla_"+prefix+"_region_weight"])}}}
	known := make(map[string]bool)
	for i := 0; i < 256; i++ {
		if name := cubiomes.BiomeName(i); name != "" {
			known["minecraft:"+strings.TrimPrefix(name, "minecraft:")] = true
		}
	}
	known["terrablender:deferred_placeholder"] = true
	for _, doc := range snapshot.Documents {
		if doc.Kind == "biome" {
			known[doc.ID] = string(doc.Data) != "null"
		}
	}
	programs := make(map[string]bool)
	ordered, err := forgeOrderRegionPrograms(snapshot)
	if err != nil {
		return nil, err
	}
	// FMLCommonSetupEvent listeners register directly during parallel dispatch;
	// enqueueWork tasks run afterwards from the DeferredWorkQueue. Within each
	// phase the mod order applies (javap: natures_spirit registers in
	// commonSetup, BiomesOPlenty inside enqueueWork, and NS regions come first).
	var deferredRegions []cubiomes.TerraBlenderRegion
	for _, program := range ordered {
		if programs[program.ModID] {
			return nil, fmt.Errorf("duplicate region provider %s", program.ModID)
		}
		programs[program.ModID] = true
		if program.Error != "" || program.Result == nil {
			continue
		}
		for _, region := range program.Result.Regions {
			if region.Dimension != dimension {
				continue
			}
			entry := cubiomes.TerraBlenderRegion{ID: region.ID, Weight: region.Weight}
			for _, parameter := range region.Parameters {
				if known[parameter.Biome] {
					entry.Parameters = append(entry.Parameters, parameter)
				}
			}
			if region.Deferred {
				deferredRegions = append(deferredRegions, entry)
				continue
			}
			entry.Index = len(cfg.Regions)
			cfg.Regions = append(cfg.Regions, entry)
		}
	}
	if weight, ok := quarkTerrablenderRegion(snapshot); ok && dimension == cubiomes.Overworld {
		base, err := cubiomes.BuiltinClimateParameters(ctx, snapshot.Minecraft, "minecraft:overworld")
		if err != nil {
			return nil, err
		}
		entry := cubiomes.TerraBlenderRegion{ID: "quark:biome_provider", Weight: weight}
		for _, parameter := range base {
			if known[parameter.Biome] {
				entry.Parameters = append(entry.Parameters, parameter)
			}
		}
		deferredRegions = append(deferredRegions, entry)
	}
	for _, entry := range deferredRegions {
		entry.Index = len(cfg.Regions)
		cfg.Regions = append(cfg.Regions, entry)
	}
	applyTerraBlenderRegionLog(snapshot, dimension, cfg.Regions)
	for _, p := range skippedRegionPrograms(snapshot) {
		programs[p.ModID] = true
	}
	for _, mod := range snapshot.Mods {
		if mod.TerraBlender && !programs[mod.ID] {
			return nil, fmt.Errorf("missing registration program for %s", mod.ID)
		}
	}
	return cubiomes.WithTerraBlender(ctx, base, cfg)
}

// forgeOrderRegionPrograms mirrors Forge ModSorter for TerraBlender consumers:
// dependency ordering=AFTER/BEFORE edges, with ModsFolderLocator's filename
// sort as the topological tie-break (Comparator.comparing discovery index).
func forgeOrderRegionPrograms(snapshot *WorldgenSnapshot) ([]WorldgenRegionProgram, error) {
	mods := make(map[string]WorldgenMod, len(snapshot.Mods))
	for _, mod := range snapshot.Mods {
		mods[mod.ID] = mod
	}
	var programs []WorldgenRegionProgram
	index := map[string]int{}
	for _, program := range snapshot.RegionPrograms {
		if program.Result == nil {
			continue
		}
		if _, exists := index[program.ModID]; exists {
			return nil, fmt.Errorf("duplicate region provider %s", program.ModID)
		}
		index[program.ModID] = len(programs)
		programs = append(programs, program)
	}
	if len(programs) <= 1 {
		return programs, nil
	}
	edges := make([]map[int]bool, len(programs))
	in := make([]int, len(programs))
	add := func(before, after string) {
		i, okI := index[before]
		j, okJ := index[after]
		if !okI || !okJ || before == after {
			return
		}
		if edges[i] == nil {
			edges[i] = map[int]bool{}
		}
		if !edges[i][j] {
			edges[i][j] = true
			in[j]++
		}
	}
	for _, program := range programs {
		mod := mods[program.ModID]
		for _, dep := range mod.After {
			add(dep, program.ModID) // this mod loads after dep
		}
		for _, dep := range mod.Before {
			add(program.ModID, dep) // this mod loads before dep
		}
	}
	fileRank := func(p WorldgenRegionProgram) string {
		name := p.Source
		if _, file, ok := strings.Cut(name, ":"); ok {
			name = file
		}
		return strings.ToLower(path.Base(name))
	}
	var ordered []WorldgenRegionProgram
	for len(ordered) < len(programs) {
		ready := -1
		for i, n := range in {
			if n != 0 {
				continue
			}
			if ready < 0 || fileRank(programs[i]) < fileRank(programs[ready]) ||
				fileRank(programs[i]) == fileRank(programs[ready]) && programs[i].ModID < programs[ready].ModID {
				ready = i
			}
		}
		if ready < 0 {
			return nil, fmt.Errorf("cross-JAR region registration order contains a cycle")
		}
		in[ready] = -1
		ordered = append(ordered, programs[ready])
		for next := range edges[ready] {
			in[next]--
		}
	}
	return ordered, nil
}

// skippedRegionPrograms lists region providers whose registration failed and
// were left out, so the biome map they feed is approximate.
func skippedRegionPrograms(snapshot *WorldgenSnapshot) []WorldgenRegionProgram {
	var out []WorldgenRegionProgram
	for _, program := range snapshot.RegionPrograms {
		if program.Error != "" {
			out = append(out, program)
		}
	}
	return out
}
