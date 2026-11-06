package launcher

import (
	"encoding/json"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// aquamiraeConfigurableSpread is ConfigurableStructurePlacement (javap):
// RandomSpreadStructurePlacement with spacing/separation from IntConfigReference
// keys ("@PIRATE_SHIP_PLACEMENT_SPACING") resolved against CommonConfig /
// obscuria/aquamirae-common.toml. computeSpacing = max(separation+1, spacing).
const aquamiraeConfigurableSpread = "aquamirae:configurable_random_spread"

const aquamiraeCommonTOML = "obscuria/aquamirae-common.toml"

// Defaults from CommonConfig.defineInt (javap) when the instance toml is absent.
var aquamiraeIntDefaults = map[string]int{
	"pirate_ship_placement_spacing":       35,
	"pirate_ship_placement_separation":    10,
	"pirate_outpost_placement_spacing":    18,
	"pirate_outpost_placement_separation": 10,
	"pirate_shelter_placement_spacing":    30,
	"pirate_shelter_placement_separation": 20,
}

func aquamiraeConfigDecl() worldgenConfigDecl {
	return worldgenConfigDecl{ModID: "aquamirae", Files: []string{aquamiraeCommonTOML}}
}

// aquamiraeResolveInt turns an IntConfigReference serialized name ("@KEY") or
// a bare JSON number into an int using ModConfigs then CommonConfig defaults.
func aquamiraeResolveInt(raw json.RawMessage, configs map[string][]byte) (int, bool) {
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return n, true
	}
	var s string
	if json.Unmarshal(raw, &s) != nil || s == "" {
		return 0, false
	}
	key := strings.TrimPrefix(s, "@")
	key = strings.ToLower(key)
	if v, ok := aquamiraeTomlInt(configs, key); ok {
		return v, true
	}
	if v, ok := aquamiraeIntDefaults[key]; ok {
		return v, true
	}
	return 0, false
}

func aquamiraeTomlInt(configs map[string][]byte, key string) (int, bool) {
	raw, ok := configs[aquamiraeCommonTOML]
	if !ok || len(raw) == 0 {
		return 0, false
	}
	var doc map[string]any
	if toml.Unmarshal(raw, &doc) != nil {
		return 0, false
	}
	v, ok := doc[key]
	if !ok {
		return 0, false
	}
	switch t := v.(type) {
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	case int:
		return t, true
	default:
		return 0, false
	}
}

// aquamiraeParseConfig reads placement.config {spacing, separation [,spread_type]}.
// Returns spacing after computeSpacing (max(sep+1, spacing)) and separation.
func aquamiraeParseConfig(raw json.RawMessage, configs map[string][]byte) (spacing, separation int, triangular bool, ok bool) {
	var cfg struct {
		Spacing    json.RawMessage `json:"spacing"`
		Separation json.RawMessage `json:"separation"`
		SpreadType string          `json:"spread_type"`
	}
	if json.Unmarshal(raw, &cfg) != nil || len(cfg.Spacing) == 0 || len(cfg.Separation) == 0 {
		return 0, 0, false, false
	}
	sp, ok1 := aquamiraeResolveInt(cfg.Spacing, configs)
	se, ok2 := aquamiraeResolveInt(cfg.Separation, configs)
	if !ok1 || !ok2 {
		return 0, 0, false, false
	}
	if se+1 > sp {
		sp = se + 1
	}
	switch seedMapID(cfg.SpreadType) {
	case "", "minecraft:linear", "linear":
	case "minecraft:triangular", "triangular":
		triangular = true
	default:
		return 0, 0, false, false
	}
	return sp, se, triangular, true
}
