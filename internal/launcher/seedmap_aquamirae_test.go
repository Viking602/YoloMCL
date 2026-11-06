package launcher

import (
	"encoding/json"
	"testing"
)

func TestAquamiraeConfigKeyFeed(t *testing.T) {
	doc := WorldgenDocument{
		ID: "aquamirae:pirate_ship",
		Data: []byte(`{
			"structures":[{"structure":"aquamirae:pirate_ship","weight":1}],
			"placement":{
				"type":"aquamirae:configurable_random_spread",
				"salt":864903,
				"config":{
					"spacing":"@PIRATE_SHIP_PLACEMENT_SPACING",
					"separation":"@PIRATE_SHIP_PLACEMENT_SEPARATION"
				}
			}
		}`),
	}
	p, ok := parseSeedMapPlacement(doc, nil)
	if !ok {
		t.Fatal("defaults should parse")
	}
	if p.spacing != 35 || p.separation != 10 || p.salt != 864903 {
		t.Fatalf("got spacing=%d separation=%d salt=%d", p.spacing, p.separation, p.salt)
	}
	toml := []byte("pirate_ship_placement_spacing = 40\npirate_ship_placement_separation = 12\n")
	p, ok = parseSeedMapPlacement(doc, map[string][]byte{aquamiraeCommonTOML: toml})
	if !ok || p.spacing != 40 || p.separation != 12 {
		t.Fatalf("toml override: ok=%v spacing=%d separation=%d", ok, p.spacing, p.separation)
	}
	// computeSpacing: sep+1 > spacing
	raw := json.RawMessage(`{"spacing":"@PIRATE_SHIP_PLACEMENT_SPACING","separation":"@PIRATE_SHIP_PLACEMENT_SEPARATION"}`)
	sp, se, _, ok := aquamiraeParseConfig(raw, map[string][]byte{aquamiraeCommonTOML: []byte("pirate_ship_placement_spacing = 5\npirate_ship_placement_separation = 10\n")})
	if !ok || se != 10 || sp != 11 {
		t.Fatalf("computeSpacing max(sep+1,spacing): sp=%d se=%d ok=%v", sp, se, ok)
	}
}
