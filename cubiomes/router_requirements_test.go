package cubiomes

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestNoiseRequirementsFollowEffectiveClimateGraph(t *testing.T) {
	docs := map[string]Document{"noise/test:climate": {Data: json.RawMessage(`{"firstOctave":-3,"amplitudes":[1]}`)}}
	for _, tt := range []struct {
		expression string
		want       NoiseRequirements
	}{
		{`0`, NoiseRequirements{}},
		{`{"type":"end_islands"}`, NoiseRequirements{EndIslands: true}},
		{`{"type":"noise","noise":"test:climate","xz_scale":1,"y_scale":0}`, NoiseRequirements{NormalNoise: true}},
		{`{"type":"add","argument1":{"type":"end_islands"},"argument2":{"type":"noise","noise":"test:climate","xz_scale":1,"y_scale":0}}`, NoiseRequirements{NormalNoise: true, EndIslands: true}},
	} {
		raw := fmt.Sprintf(`{"noise_router":{"temperature":%s,"vegetation":0,"continents":0,"erosion":0,"depth":0,"ridges":0}}`, tt.expression)
		r, err := CompileNoiseRouter(context.Background(), docs, Definition{Version: "1.21.1", Settings: json.RawMessage(raw)})
		if err != nil {
			t.Fatal(err)
		}
		if got := r.Requirements(); got != tt.want {
			t.Fatalf("%s: %+v want %+v", tt.expression, got, tt.want)
		}
	}
}
