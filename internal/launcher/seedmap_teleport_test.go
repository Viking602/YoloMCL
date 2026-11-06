package launcher

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/Viking602/YoloMCL/cubiomes"
)

func TestSeedMapTeleportHeight(t *testing.T) {
	for _, tc := range []struct {
		name, density                   string
		minY, height, sea, nearY, wantY int
		wantOK                          bool
	}{
		{"land-above-current-height", `{"type":"minecraft:y_clamped_gradient","from_y":0,"to_y":63,"from_value":1,"to_value":-1}`, 0, 64, 0, 1, 33, true},
		{"water-surface", `{"type":"minecraft:y_clamped_gradient","from_y":0,"to_y":63,"from_value":1,"to_value":-1}`, 0, 64, 40, 1, 41, true},
		{"negative-height", `{"type":"minecraft:y_clamped_gradient","from_y":-64,"to_y":0,"from_value":1,"to_value":-1}`, -64, 128, -64, 64, -31, true},
		{"roofed-floor", `{"type":"minecraft:max","argument1":{"type":"minecraft:y_clamped_gradient","from_y":0,"to_y":31,"from_value":1,"to_value":-1},"argument2":{"type":"minecraft:y_clamped_gradient","from_y":40,"to_y":63,"from_value":-1,"to_value":1}}`, 0, 64, 0, 40, 16, true},
		{"void", `-1`, 0, 64, 0, 40, 0, false},
		{"no-headroom", `1`, 0, 64, 0, 40, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := jsonBytes(t, map[string]any{
				"noise":     map[string]int{"min_y": tc.minY, "height": tc.height, "size_horizontal": 1, "size_vertical": 1},
				"sea_level": tc.sea, "default_fluid": map[string]string{"Name": "minecraft:water"},
				"noise_router": map[string]json.RawMessage{"final_density": json.RawMessage(tc.density)},
			})
			terrain, err := cubiomes.CompileTerrain(context.Background(), nil, cubiomes.Definition{Version: "1.21.1", Settings: settings})
			if err != nil {
				t.Fatal(err)
			}
			sampler := &LocalSampler{dimensions: map[string]seedMapDimension{"test": {
				terrain: terrain, terrainPool: &sync.Pool{New: func() any { return terrain.NewSampler() }},
			}}}
			if y, ok := sampler.TeleportYAt("test", -100, 200, tc.nearY); y != tc.wantY || ok != tc.wantOK {
				t.Fatalf("teleport height = %d, %v; want %d, %v", y, ok, tc.wantY, tc.wantOK)
			}
		})
	}
	if y, ok := (*LocalSampler)(nil).TeleportYAt("missing", 0, 0, 64); ok || y != 0 {
		t.Fatal("missing sampler fabricated a teleport height")
	}
}
