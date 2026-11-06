package cubiomes

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// TerraBlenderArtifact identifies an exact artifact used by a bundled profile.
// The SHA-512 covers the complete jar, not only its declared mod version.
type TerraBlenderArtifact struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	Filename string `json:"filename"`
	SHA512   string `json:"sha512"`
}

type terraBlenderProfileDimension struct {
	RegionSize int                  `json:"region_size"`
	Regions    []TerraBlenderRegion `json:"regions"`
}

type terraBlenderProfileEndTable struct {
	Size    int                         `json:"size"`
	Entries []TerraBlenderWeightedBiome `json:"entries"`
}

type terraBlenderProfileData struct {
	Minecraft         string                 `json:"minecraft"`
	Version           string                 `json:"terrablender_version"`
	BOPVersion        string                 `json:"bop_version"`
	RegistryZeroBiome string                 `json:"registry_zero_biome"`
	Artifacts         []TerraBlenderArtifact `json:"artifacts"`
	Dimensions        struct {
		Overworld terraBlenderProfileDimension           `json:"overworld"`
		Nether    terraBlenderProfileDimension           `json:"nether"`
		End       map[string]terraBlenderProfileEndTable `json:"end"`
	} `json:"dimensions"`
	Defaults struct {
		BiomeToggles map[string]bool `json:"biome_toggles"`
	} `json:"defaults"`
}

var bundledTerraBlenderProfiles [4]struct {
	once sync.Once
	data terraBlenderProfileData
	err  error
}

func loadTerraBlenderProfile(minecraft, bopVersion string) (*terraBlenderProfileData, error) {
	version, expectedBOP, index := "26.3.0.0.9", "26.3.0.0.15", 0
	if minecraft == "1.21.1" {
		version, expectedBOP, index = "4.1.0.8", "21.1.0.14", 2
	}
	if minecraft != "26.3" && minecraft != "1.21.1" || (bopVersion != "" && bopVersion != expectedBOP) {
		return nil, fmt.Errorf("no bundled TerraBlender profile for Minecraft %q / Biomes O' Plenty %q", minecraft, bopVersion)
	}
	file := "terrablender-bop-" + minecraft + ".json"
	if bopVersion == "" {
		index, file = index+1, "terrablender-only-"+minecraft+".json"
	}
	p := &bundledTerraBlenderProfiles[index]
	p.once.Do(func() {
		var raw []byte
		raw, p.err = readSeedMapBuiltin(file)
		if p.err == nil {
			p.err = json.Unmarshal(raw, &p.data)
		}
		if p.err == nil && (p.data.Minecraft != minecraft || p.data.Version != version || p.data.BOPVersion != bopVersion) {
			p.err = fmt.Errorf("invalid bundled TerraBlender profile identity")
		}
	})
	return &p.data, p.err
}

// BuiltinTerraBlenderProfile returns an owned region table for the verified
// version-pinned Minecraft/TerraBlender/BOP profile. Its
// parameters assume the default enabled-biome toggles. Region weights and
// sizes may be changed before binding. Base-region parameters always come
// from the supplied Generator, including its effective datapack overrides.
// An empty bopVersion selects the verified TerraBlender-only profile.
func BuiltinTerraBlenderProfile(minecraft, bopVersion string, dimension Dimension) (TerraBlenderConfig, error) {
	p, err := loadTerraBlenderProfile(minecraft, bopVersion)
	if err != nil {
		return TerraBlenderConfig{}, err
	}
	var d terraBlenderProfileDimension
	switch dimension {
	case Overworld:
		d = p.Dimensions.Overworld
	case Nether:
		d = p.Dimensions.Nether
	default:
		return TerraBlenderConfig{}, fmt.Errorf("use BuiltinTerraBlenderEndProfile for End areas")
	}
	regions := append([]TerraBlenderRegion(nil), d.Regions...)
	for i := range regions {
		regions[i].Parameters = append([]TerraBlenderParameter(nil), regions[i].Parameters...)
	}
	return TerraBlenderConfig{Minecraft: p.Minecraft, Version: p.Version, Dimension: dimension, RegionSize: d.RegionSize, Regions: regions}, nil
}

// BuiltinTerraBlenderEndProfile returns owned, ordered End weight tables.
func BuiltinTerraBlenderEndProfile(minecraft, bopVersion string) (TerraBlenderEndConfig, error) {
	p, err := loadTerraBlenderProfile(minecraft, bopVersion)
	if err != nil {
		return TerraBlenderEndConfig{}, err
	}
	table := func(name string) TerraBlenderEndTable {
		t := p.Dimensions.End[name]
		return TerraBlenderEndTable{Size: t.Size, Biomes: append([]TerraBlenderWeightedBiome(nil), t.Entries...)}
	}
	return TerraBlenderEndConfig{Minecraft: p.Minecraft, Version: p.Version, Highlands: table("highlands"), Midlands: table("midlands"), Edge: table("edge"), Islands: table("islands")}, nil
}

// BuiltinTerraBlenderArtifacts returns owned artifact identities for checking
// an installed profile before interpreting its injected biome behavior.
func BuiltinTerraBlenderArtifacts(minecraft, bopVersion string) ([]TerraBlenderArtifact, error) {
	p, err := loadTerraBlenderProfile(minecraft, bopVersion)
	if err != nil {
		return nil, err
	}
	return append([]TerraBlenderArtifact(nil), p.Artifacts...), nil
}

// BuiltinTerraBlenderBiomeToggles returns an owned copy of the supported BOP
// enabled-biome defaults. The WithToggles factories apply their ordered
// fallbacks; deleting boxes does not reproduce BOP's fallback rules.
func BuiltinTerraBlenderBiomeToggles(minecraft, bopVersion string) (map[string]bool, error) {
	p, err := loadTerraBlenderProfile(minecraft, bopVersion)
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(p.Defaults.BiomeToggles))
	for name, enabled := range p.Defaults.BiomeToggles {
		result[name] = enabled
	}
	return result, nil
}

type terraBlenderFallbackData struct {
	Minecraft  string     `json:"minecraft"`
	BOPVersion string     `json:"bop_version"`
	Chains     [][]string `json:"chains"`
	Regions    map[string]struct {
		Chains []int `json:"chains"`
	} `json:"regions"`
}

var bundledTerraBlenderFallbacks [2]struct {
	once sync.Once
	data terraBlenderFallbackData
	err  error
}

func loadTerraBlenderFallbacks(minecraft, bopVersion string) (*terraBlenderFallbackData, error) {
	index := 0
	if minecraft == "1.21.1" {
		index = 1
	}
	p := &bundledTerraBlenderFallbacks[index]
	p.once.Do(func() {
		var raw []byte
		raw, p.err = readSeedMapBuiltin("terrablender-bop-" + minecraft + "-fallbacks.json")
		if p.err == nil {
			p.err = json.Unmarshal(raw, &p.data)
		}
		if p.err == nil && (p.data.Minecraft != minecraft || p.data.BOPVersion != bopVersion) {
			p.err = fmt.Errorf("invalid bundled BOP fallback identity")
		}
	})
	return &p.data, p.err
}

func hasDisabledTerraBlenderBiome(p *terraBlenderProfileData, toggles map[string]bool) bool {
	for flag, enabled := range toggles {
		if _, known := p.Defaults.BiomeToggles[flag]; known && !enabled {
			return true
		}
	}
	return false
}

func terraBlenderBiomeEnabled(biome string, toggles map[string]bool) bool {
	if !strings.HasPrefix(biome, "biomesoplenty:") {
		return true
	}
	enabled, present := toggles[strings.TrimPrefix(biome, "biomesoplenty:")+"_enabled"]
	return !present || enabled
}

// BuiltinTerraBlenderProfileWithToggles applies the verified ordered fallback
// choices for the selected release's BOP biome toggles. Missing toggles default to true and
// unknown keys are ignored, as in BOP. Choices are specific to each climate
// entry: disabling one biome is not a global biome-name substitution.
func BuiltinTerraBlenderProfileWithToggles(minecraft, bopVersion string, dimension Dimension, toggles map[string]bool) (TerraBlenderConfig, error) {
	cfg, err := BuiltinTerraBlenderProfile(minecraft, bopVersion, dimension)
	if err != nil {
		return TerraBlenderConfig{}, err
	}
	p, err := loadTerraBlenderProfile(minecraft, bopVersion)
	if err != nil || !hasDisabledTerraBlenderBiome(p, toggles) {
		return cfg, err
	}
	fallbacks, err := loadTerraBlenderFallbacks(minecraft, bopVersion)
	if err != nil {
		return TerraBlenderConfig{}, err
	}
	for i := range cfg.Regions {
		r := &cfg.Regions[i]
		if r.Base {
			continue
		}
		indices, found := fallbacks.Regions[r.ID]
		if !found || len(indices.Chains) != len(r.Parameters) {
			return TerraBlenderConfig{}, fmt.Errorf("BOP fallback geometry differs for region %q", r.ID)
		}
		for j, index := range indices.Chains {
			if index < 0 || index >= len(fallbacks.Chains) || len(fallbacks.Chains[index]) == 0 {
				return TerraBlenderConfig{}, fmt.Errorf("invalid BOP fallback chain in region %q", r.ID)
			}
			chain := fallbacks.Chains[index]
			r.Parameters[j].Biome = chain[len(chain)-1]
			for _, biome := range chain[:len(chain)-1] {
				if terraBlenderBiomeEnabled(biome, toggles) {
					r.Parameters[j].Biome = biome
					break
				}
			}
		}
	}
	return cfg, nil
}

// BuiltinTerraBlenderEndProfileWithToggles omits disabled BOP End registrations
// while preserving the surviving entry order and weights.
func BuiltinTerraBlenderEndProfileWithToggles(minecraft, bopVersion string, toggles map[string]bool) (TerraBlenderEndConfig, error) {
	cfg, err := BuiltinTerraBlenderEndProfile(minecraft, bopVersion)
	if err != nil {
		return TerraBlenderEndConfig{}, err
	}
	for _, table := range []*TerraBlenderEndTable{&cfg.Highlands, &cfg.Midlands, &cfg.Edge, &cfg.Islands} {
		kept := table.Biomes[:0]
		for _, entry := range table.Biomes {
			if terraBlenderBiomeEnabled(entry.Biome, toggles) {
				kept = append(kept, entry)
			}
		}
		table.Biomes = kept
	}
	return cfg, nil
}
