package cubiomes

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"
)

// This fixture calls the official 26.2 RandomState/NoiseRouter APIs. It checks
// complete cave-density graphs and preliminary surface heights, across five
// settings, six seeds and negative/world-border coordinates.
func TestModernTerrainJavaOracle(t *testing.T) {
	f, err := os.Open("testdata/terrain-26.2.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var fixture struct {
		Version string
		Cases   []struct {
			Profile, Seed string
			Samples       []struct {
				Position             [3]int
				Density, Preliminary float64
				SurfaceNoise2D       float64 `json:"surface_noise_2d"`
				SurfaceNoise3D       float64 `json:"surface_noise_3d"`
			}
		}
	}
	if err := json.NewDecoder(z).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	documents, err := BuiltinDocuments(fixture.Version)
	if err != nil {
		t.Fatal(err)
	}
	docs := map[string]Document{}
	for _, d := range documents {
		docs[d.Kind+"/"+d.ID] = d
	}
	for _, tc := range fixture.Cases {
		t.Run(tc.Profile+"/"+tc.Seed, func(t *testing.T) {
			seed, err := strconv.ParseInt(tc.Seed, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			settings := docs["noise_settings/minecraft:"+tc.Profile].Data
			var data struct {
				Legacy bool                       `json:"legacy_random_source"`
				Router map[string]json.RawMessage `json:"noise_router"`
			}
			if err := json.Unmarshal(settings, &data); err != nil {
				t.Fatal(err)
			}
			c := newDensityCompiler(ctx, docs, seed, data.Legacy)
			c.version = fixture.Version
			n, err := c.compile(data.Router["final_density"], 0)
			if err != nil {
				t.Fatal(err)
			}
			p := densitySample{seen: make([]uint32, c.nodes), values: make([]float64, c.nodes)}
			terrain, err := CompileTerrain(ctx, docs, Definition{Version: fixture.Version, Seed: seed, Settings: settings})
			if err != nil {
				t.Fatal(err)
			}
			sampler := terrain.NewSampler()
			sc := &surfaceCompiler{c: c}
			var conditions [2]surfaceCond
			for i := range conditions {
				raw, _ := json.Marshal(map[string]any{"type": "minecraft:noise_threshold", "noise": "minecraft:surface", "min_threshold": -.1, "max_threshold": .1, "is_3d": i == 1})
				conditions[i], err = sc.cond(raw, 0)
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, sample := range tc.Samples {
				pos := sample.Position
				p.at(pos[0], pos[1], pos[2])
				if got := n.eval(&p); math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-sample.Density) > 1e-12 {
					t.Errorf("density %v: got %.17g want %.17g", pos, got, sample.Density)
				}
				if got := sampler.Preliminary(pos[0], pos[2]); float64(got) != sample.Preliminary {
					t.Errorf("preliminary %v: got %d want %g", pos, got, sample.Preliminary)
				}
				for i, noise := range []float64{sample.SurfaceNoise2D, sample.SurfaceNoise3D} {
					want := noise >= -.1 && noise <= .1
					if got := conditions[i](&surfaceCtx{x: pos[0], y: pos[1], z: pos[2]}); got != want {
						t.Errorf("surface noise 3d=%t at %v: got %t want %t (noise %g)", i == 1, pos, got, want, noise)
					}
				}
			}
		})
	}
}

func TestPreliminarySurfaceWithoutAquifers(t *testing.T) {
	for _, tc := range []struct {
		name, router string
		want         int
	}{
		{"legacy", `"initial_density_without_jaggedness":{"type":"y_clamped_gradient","from_y":-64,"to_y":320,"from_value":1,"to_value":-1}`, 48},
		{"modern", `"preliminary_surface_level":-3.25`, -4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := json.RawMessage(`{"noise":{"min_y":-64,"height":384,"size_horizontal":1,"size_vertical":2},"noise_router":{"final_density":1,` + tc.router + `}}`)
			terrain, err := CompileTerrain(context.Background(), nil, Definition{Version: "26.2", Settings: settings})
			if err != nil {
				t.Fatal(err)
			}
			if got := terrain.NewSampler().Preliminary(-1, 7); got != tc.want {
				t.Fatalf("preliminary without aquifers: got %d want %d", got, tc.want)
			}
		})
	}
}
