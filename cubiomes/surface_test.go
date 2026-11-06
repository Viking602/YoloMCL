package cubiomes

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"testing"
)

func TestSurfaceReference(t *testing.T) {
	f, err := os.Open("testdata/surface.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	var s *Sampler
	var last Config
	differences := 0
	worst := 0.0
	for _, row := range rows[1:] {
		seed, _ := strconv.ParseInt(row[2], 10, 64)
		cfg := Config{Version: row[0], Seed: seed, LargeBiomes: row[1] == "1"}
		if s == nil || cfg != last {
			g, err := New(context.Background(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			if !g.SupportsSurface() {
				t.Fatalf("surface not detected: %+v", cfg)
			}
			s, last = g.NewSampler(), cfg
		}
		x, _ := strconv.Atoi(row[3])
		z, _ := strconv.Atoi(row[4])
		want, _ := strconv.ParseFloat(row[5], 32)
		got, ok := s.EstimateSurfaceHeight(x, z)
		delta := math.Abs(float64(got) - want)
		if got != float32(want) {
			differences++
			worst = max(worst, delta)
		}
		// Upstream's hard-coded float spline offset and Minecraft's JSON
		// double graph can straddle one quantization unit at distant points.
		// Preserve Minecraft evaluation, with an explicit 1/76-block bound.
		if !ok || delta > 1.0/76+.00002 {
			t.Fatalf("%+v (%d,%d): got %.9g want %.9g", cfg, x, z, got, want)
		}
	}
	if len(rows) != 5881 {
		t.Fatalf("reference coverage: %d", len(rows)-1)
	}
	t.Logf("%d samples, %d exact, %d within one depth quantum; max height difference %.8g blocks", len(rows)-1, len(rows)-1-differences, differences, worst)
}

func TestSurfaceRangeMatchesScalar(t *testing.T) {
	for _, version := range []string{"1.18.2", "1.21.1", "26.3", "bop"} {
		t.Run(version, func(t *testing.T) {
			var g *Generator
			if version == "bop" {
				g = terraBlenderTestGenerator(t, Overworld, 42)
			} else {
				var err error
				g, err = New(context.Background(), Config{Version: version, Seed: 42})
				if err != nil {
					t.Fatal(err)
				}
			}
			if !g.SupportsSurface() {
				t.Fatal("missing surface")
			}
			s, oracle := g.NewSampler(), g.NewSampler()
			for _, step := range []int{1, 2, 4, 8, 32} {
				r := SurfaceRange{X: -137, Z: -257, SizeX: 67, SizeZ: 59, Step: step, SeaLevel: 63, Voronoi: step < 4}
				out := make([]SurfaceSample, r.SizeX*r.SizeZ)
				if err := s.GenerateSurface(context.Background(), r, out); err != nil {
					t.Fatal(err)
				}
				oracle.Reset()
				for i, p := range out {
					x, z := r.X+(i%r.SizeX)*step, r.Z+(i/r.SizeX)*step
					// Independent four-corner formula; only uses the original Climate API.
					heights := [4]float64{}
					for corner := range 4 {
						climate, ok := oracle.Climate(((x>>2)+corner%2)*4, 0, ((z>>2)+corner/2)*4)
						if !ok {
							t.Fatal("climate")
						}
						heights[corner] = float64(float32(float64(climate[4]) / 76))
					}
					h := float32(noiseLerp(float64(z&3)/4, noiseLerp(float64(x&3)/4, heights[0], heights[1]), noiseLerp(float64(x&3)/4, heights[2], heights[3])))
					y := int(math.Floor(max(float64(h), float64(r.SeaLevel)))) + 1
					if r.Voronoi {
						x, y, z = oracle.voronoi(x, y, z)
						x, y, z = x*4, y*4, z*4
					}
					id, ok := oracle.NextNoiseBiomeID(x, y, z)
					if !ok || p.Height != h || p.Biome != id {
						t.Fatalf("step %d pixel %d: got %+v want %g/%d", step, i, p, h, id)
					}
				}
			}
		})
	}
}

func TestSurfaceBoundsAndReuse(t *testing.T) {
	ctx := context.Background()
	for _, cfg := range []Config{{Version: "1.17.1"}, {Version: "1.21.1", Dimension: Nether}, {Version: "26.3", Dimension: End}} {
		g, err := New(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if g.SupportsSurface() {
			t.Fatalf("fabricated surface for %+v", cfg)
		}
		if _, ok := g.NewSampler().EstimateSurfaceHeight(0, 0); ok {
			t.Fatal("unsupported height")
		}
	}
	g, err := New(ctx, Config{Version: "26.3", Seed: 42})
	if err != nil {
		t.Fatal(err)
	}
	s := g.NewSampler()
	r := SurfaceRange{X: -128, Z: -128, SizeX: 128, SizeZ: 128, Step: 1, SeaLevel: 63, Voronoi: true}
	out := make([]SurfaceSample, 128*128)
	if err := s.GenerateSurface(ctx, r, out); err != nil {
		t.Fatal(err)
	}
	if n := testing.AllocsPerRun(5, func() {
		r.X += 128
		if err := s.GenerateSurface(ctx, r, out); err != nil {
			panic(err)
		}
	}); n != 0 {
		t.Fatalf("warm allocations: %g", n)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := s.GenerateSurface(cancelled, r, out); err != context.Canceled {
		t.Fatal(err)
	}
	for _, bad := range []SurfaceRange{{SizeX: 1 << 30, SizeZ: 1 << 30, Step: 1}, {X: math.MaxInt32, SizeX: 2, SizeZ: 2, Step: 4}, {SizeX: 2, SizeZ: 2, Step: 0}} {
		if s.GenerateSurface(ctx, bad, out) == nil {
			t.Fatal("invalid range accepted")
		}
	}
}

func TestSurfaceCustomGraphAndReseed(t *testing.T) {
	ctx := context.Background()
	documents, err := BuiltinDocuments("1.21.1")
	if err != nil {
		t.Fatal(err)
	}
	docs := map[string]Document{}
	for _, d := range documents {
		docs[d.Kind+"/"+d.ID] = d
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(docs["noise_settings/minecraft:overworld"].Data, &settings); err != nil {
		t.Fatal(err)
	}
	var router map[string]json.RawMessage
	if err := json.Unmarshal(settings["noise_router"], &router); err != nil {
		t.Fatal(err)
	}
	router["temperature"] = json.RawMessage(`{"type":"minecraft:noise","noise":"minecraft:temperature","xz_scale":1,"y_scale":1}`)
	router["depth"] = json.RawMessage(`{"type":"minecraft:add","argument1":"minecraft:overworld/depth","argument2":0.25}`)
	settings["noise_router"], _ = json.Marshal(router)
	raw, _ := json.Marshal(settings)
	def := Definition{Version: "1.21.1", Seed: 42, Settings: raw, BiomeSource: json.RawMessage(`{"type":"minecraft:multi_noise","preset":"minecraft:overworld"}`)}
	g, err := Compile(ctx, docs, def)
	if err != nil {
		t.Fatal(err)
	}
	if !g.SupportsSurface() {
		t.Fatal("datapack depth override was not recognized")
	}
	for _, seed := range []int64{42, -1, math.MinInt64} {
		g, err = g.WithSeed(ctx, seed)
		if err != nil {
			t.Fatal(err)
		}
		s, oracle := g.NewSampler(), g.NewSampler()
		vanilla, err := New(ctx, Config{Version: "1.21.1", Seed: seed})
		if err != nil {
			t.Fatal(err)
		}
		for _, xz := range [][2]int{{-324, 512}, {0, 0}, {7475000, -7475000}} {
			h, ok := s.EstimateSurfaceHeight(xz[0]*4, xz[1]*4)
			if !ok {
				t.Fatal("height")
			}
			base, _ := vanilla.NewSampler().EstimateSurfaceHeight(xz[0]*4, xz[1]*4)
			if math.Abs(float64(h-base)-2500.0/76) > .02 {
				t.Fatal("depth override lost")
			}
			for _, y := range []int{-16, 0, 16, 64, -16} {
				got, ok := s.surfaceTarget(xz[0], y, xz[1])
				want, valid := oracle.Climate(xz[0]*4, y*4, xz[1]*4)
				if !ok || !valid || [6]int64(got[:6]) != want {
					t.Fatalf("column cache reused Y-dependent noise at %v/%d", xz, y)
				}
			}
		}
	}
}

func BenchmarkSurfaceRange(b *testing.B) {
	for _, step := range []int{1, 2, 4} {
		b.Run(fmt.Sprint(step), func(b *testing.B) {
			g, err := New(context.Background(), Config{Version: "1.21.1", Seed: 42})
			if err != nil {
				b.Fatal(err)
			}
			s := g.NewSampler()
			r := SurfaceRange{X: -1296, Z: 2048, SizeX: 128, SizeZ: 128, Step: step, SeaLevel: 63, Voronoi: step < 4}
			out := make([]SurfaceSample, 128*128)
			_ = s.GenerateSurface(context.Background(), r, out)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				r.X += 128 * step
				if err := s.GenerateSurface(context.Background(), r, out); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestSurfaceDepthCaps(t *testing.T) {
	ctx := context.Background()
	documents, err := BuiltinDocuments("1.21.1")
	if err != nil {
		t.Fatal(err)
	}
	docs := map[string]Document{}
	for _, d := range documents {
		docs[d.Kind+"/"+d.ID] = d
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(docs["noise_settings/minecraft:overworld"].Data, &settings); err != nil {
		t.Fatal(err)
	}
	var router map[string]json.RawMessage
	if err := json.Unmarshal(settings["noise_router"], &router); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, depth string
		supported   bool
	}{
		{"upper", `{"type":"min","argument1":"minecraft:overworld/depth","argument2":{"type":"range_choice","input":{"type":"y_clamped_gradient","from_y":-2048,"to_y":2048,"from_value":-2048,"to_value":2048},"min_inclusive":32,"max_exclusive":2048,"when_in_range":0.05,"when_out_of_range":0.1}}`, true},
		{"lower", `{"type":"max","argument1":-0.1,"argument2":"minecraft:overworld/depth"}`, true},
		{"clamp", `{"type":"clamp","input":"minecraft:overworld/depth","min":-0.1,"max":0.1}`, true},
		{"range_cap", `{"type":"range_choice","input":"minecraft:y","min_inclusive":32,"max_exclusive":2048,"when_in_range":{"type":"clamp","input":{"type":"add","argument1":0,"argument2":"minecraft:overworld/depth"},"min":-64,"max":0.8},"when_out_of_range":"minecraft:overworld/depth"}`, true},
		{"range_different_depth", `{"type":"range_choice","input":"minecraft:y","min_inclusive":32,"max_exclusive":2048,"when_in_range":{"type":"add","argument1":1,"argument2":"minecraft:overworld/depth"},"when_out_of_range":"minecraft:overworld/depth"}`, false},
		{"zero", `{"type":"min","argument1":0,"argument2":"minecraft:overworld/depth"}`, false},
		{"crossing", `{"type":"min","argument1":"minecraft:overworld/depth","argument2":{"type":"range_choice","input":"minecraft:overworld/depth","min_inclusive":0,"max_exclusive":1,"when_in_range":0.1,"when_out_of_range":-0.1}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router["depth"] = json.RawMessage(tc.depth)
			settings["noise_router"], _ = json.Marshal(router)
			raw, _ := json.Marshal(settings)
			g, err := Compile(ctx, docs, Definition{Version: "1.21.1", Seed: 42, Settings: raw, BiomeSource: json.RawMessage(`{"type":"minecraft:multi_noise","preset":"minecraft:overworld"}`)})
			if err != nil {
				t.Fatal(err)
			}
			if g.SupportsSurface() != tc.supported {
				t.Fatalf("cap support=%v", g.SupportsSurface())
			}
			if !tc.supported {
				return
			}
			for _, seed := range []int64{42, -1, math.MinInt64} {
				g, err = g.WithSeed(ctx, seed)
				if err != nil {
					t.Fatal(err)
				}
				base, err := New(ctx, Config{Version: "1.21.1", Seed: seed})
				if err != nil {
					t.Fatal(err)
				}
				s, oracle, vanilla := g.NewSampler(), g.NewSampler(), base.NewSampler()
				for _, xz := range [][2]int{{0, 0}, {-1296, 2048}, {29899992, -29899992}} {
					h, ok := s.EstimateSurfaceHeight(xz[0], xz[1])
					want, valid := vanilla.EstimateSurfaceHeight(xz[0], xz[1])
					if !ok || !valid || h != want {
						t.Fatalf("cap flattened or reseeded wrong terrain: %g want %g", h, want)
					}
					out := make([]SurfaceSample, 1)
					if err := s.GenerateSurface(ctx, SurfaceRange{X: xz[0], Z: xz[1], SizeX: 1, SizeZ: 1, Step: 4, SeaLevel: 63}, out); err != nil {
						t.Fatal(err)
					}
					y := int(math.Floor(max(63, float64(h)))) + 1
					biome, ok := oracle.NoiseBiomeID(xz[0], y, xz[1])
					if !ok || out[0].Biome != biome {
						t.Fatal("surface biome bypassed original capped climate")
					}
					climate, _ := oracle.Climate(xz[0], 0, xz[1])
					if tc.name != "range_cap" && (tc.name != "lower" && climate[4] > 1000 || tc.name != "upper" && climate[4] < -1000) {
						t.Fatal("climate cap lost")
					}
				}
			}
		})
	}
}
