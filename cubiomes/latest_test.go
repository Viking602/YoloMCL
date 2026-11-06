package cubiomes

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"os"
	"strconv"
	"testing"
)

type latestFixture struct {
	Version string
	Cases   []struct {
		Profile, Seed string
		Samples       []struct {
			Position  [3]int
			Climate   [6]int64
			Biome     string
			NextBiome string `json:"next_biome"`
		}
		Grids []struct {
			Region  [6]int
			Hash    string `json:"fnv1a_names"`
			Samples []struct {
				Position [3]int
				Climate  [6]int64
				Biome    string
			}
		}
	}
}

func TestLatestLegacyNoiseWithSeed(t *testing.T) {
	documents, err := BuiltinDocuments("26.3")
	if err != nil {
		t.Fatal(err)
	}
	docs := map[string]Document{}
	for _, d := range documents {
		docs[d.Kind+"/"+d.ID] = d
	}
	for _, amplitudes := range []string{`[1.3,0,1,0,1,1]`, `[1,2,1.4,0,0,0]`, `[0,1,0,0,0,0]`} {
		d := docs["noise/minecraft:temperature"]
		d.Data = json.RawMessage(`{"normalize":"legacy","base_octave":-10,"octave_count":6,"amplitude_modifiers":` + amplitudes + `}`)
		docs["noise/minecraft:temperature"] = d
		def := Definition{Version: "26.3", Seed: 0, Settings: json.RawMessage(`"minecraft:overworld"`), BiomeSource: json.RawMessage(`{"type":"minecraft:multi_noise","preset":"minecraft:overworld"}`)}
		original, err := compileFresh(context.Background(), docs, def)
		if err != nil {
			t.Fatal(err)
		}
		for _, seed := range []int64{-1, 1234567890123456789, math.MinInt64, math.MaxInt64} {
			bound, err := original.WithSeed(context.Background(), seed)
			if err != nil {
				t.Fatal(err)
			}
			def.Seed = seed
			fresh, err := compileFresh(context.Background(), docs, def)
			if err != nil {
				t.Fatal(err)
			}
			a, b := bound.NewSampler(), fresh.NewSampler()
			for i := 0; i < 128; i++ {
				x, z := i*7959-320000, 192812-i*1791
				ac, aok := a.Climate(x, 64, z)
				bc, bok := b.Climate(x, 64, z)
				if !aok || !bok || ac != bc {
					t.Fatalf("seed %d modifier%s at%d,%d: %v != %v", seed, amplitudes, x, z, ac, bc)
				}
			}
		}
	}
}

func TestLatestRepeatedSplineKnots(t *testing.T) {
	const spline = `{"type":"minecraft:spline","spline":{"coordinate":"test:coordinate","points":[{"location":-0.8,"value":0.7,"derivative":0},{"location":-0.8,"value":0.7,"derivative":-1},{"location":-0.5,"value":0,"derivative":-2.5},{"location":-0.5,"value":0,"derivative":0}]}}`
	for _, x := range []float64{-1, -0.8, -0.7, -0.5, 0} {
		raw, _ := json.Marshal(x)
		docs := map[string]Document{"density_function/test:coordinate": {Kind: "density_function", ID: "test:coordinate", Data: raw}}
		c, err := latestCompiler(context.Background(), docs, 0, false)
		if err != nil {
			t.Fatal(err)
		}
		n, err := c.compile(json.RawMessage(spline), 0)
		if err != nil {
			t.Fatal(err)
		}
		latestMarkNodes(c)
		p := densitySample{seen: make([]uint32, c.nodes), values: make([]float64, c.nodes)}
		p.at(0, 0, 0)
		got := n.eval(&p)
		if math.IsNaN(got) || math.IsInf(got, 0) {
			t.Fatalf("nonfinite spline at%g", x)
		}
		if x <= -0.8 && float32(got) != float32(.7) || x >= -0.5 && got != 0 {
			t.Fatalf("wrong duplicate-knot side at%g: %g", x, got)
		}
	}
}

func TestLatestClimateTreeBoundaryParity(t *testing.T) {
	f, err := os.Open("testdata/latest-26.3-tree.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Samples []struct {
			Climate [6]int64
			Biome   string
		}
	}
	if err = json.Unmarshal(b, &fixture); err != nil {
		t.Fatal(err)
	}
	docs, err := loadLatestBuiltinDocuments("26.3")
	if err != nil {
		t.Fatal(err)
	}
	registry := map[string]Document{}
	for _, d := range docs {
		registry[d.Kind+"/"+d.ID] = d
	}
	tree, _, err := resolveClimateSource(context.Background(), registry, json.RawMessage(`{"preset":"minecraft:overworld"}`), "26.3")
	if err != nil {
		t.Fatal(err)
	}
	previous := -1
	for i, p := range fixture.Samples {
		var target climatePoint
		copy(target[:], p.Climate[:])
		previous = tree.search(&target, previous)
		if got := tree.biomes[tree.nodes[previous].biome]; got != p.Biome {
			t.Fatalf("target %d %v: got %s want %s", i, p.Climate, got, p.Biome)
		}
	}
}

func TestLatestJavaParity(t *testing.T) {
	for _, version := range []string{"26.1", "26.1.1", "26.1.2", "26.2", "26.3"} {
		t.Run(version, func(t *testing.T) { testLatestJavaParity(t, version) })
	}
}

func testLatestJavaParity(t *testing.T, version string) {
	path := "testdata/latest-" + version + ".json"
	if dense := os.Getenv("MCL_LATEST_DENSE_ORACLE"); dense != "" && version == "26.3" {
		path = dense
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fixture latestFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	docs, err := loadLatestBuiltinDocuments(version)
	if err != nil {
		t.Fatal(err)
	}
	registry := make(map[string]Document, len(docs))
	for _, d := range docs {
		registry[d.Kind+"/"+d.ID] = d
	}
	for _, example := range fixture.Cases {
		t.Run(example.Profile+"/"+example.Seed, func(t *testing.T) {
			seed, err := strconv.ParseInt(example.Seed, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			source := `{"type":"minecraft:multi_noise","preset":"minecraft:overworld"}`
			if example.Profile == "nether" {
				source = `{"type":"minecraft:multi_noise","preset":"minecraft:nether"}`
			}
			if example.Profile == "end" {
				source = `{"type":"minecraft:the_end"}`
			}
			settings, _ := json.Marshal("minecraft:" + example.Profile)
			g, err := Compile(context.Background(), registry, Definition{Version: version, Seed: seed, Settings: settings, BiomeSource: json.RawMessage(source)})
			if err != nil {
				t.Fatal(err)
			}
			fresh, sequence := g.NewSampler(), g.NewSampler()
			for _, point := range example.Samples {
				p := point.Position
				if example.Profile != "end" {
					got, ok := fresh.Climate(p[0], p[1], p[2])
					if !ok || got != point.Climate {
						t.Fatalf("climate at%v got%v (%v), want%v", p, got, ok, point.Climate)
					}
				}
				got, ok := fresh.NoiseBiome(p[0], p[1], p[2])
				if !ok || got != point.Biome {
					t.Fatalf("biome at%v got%v (%v), want%v", p, got, ok, point.Biome)
				}
				got, ok = sequence.NextNoiseBiome(p[0], p[1], p[2])
				if !ok || got != point.NextBiome {
					t.Fatalf("sequence at%v got%v (%v), want%v", p, got, ok, point.NextBiome)
				}
			}
			for _, grid := range example.Grids {
				sequence.Reset()
				h := fnv.New64a()
				r := grid.Region
				for z := 0; z < r[5]; z++ {
					for x := 0; x < r[4]; x++ {
						got, ok := sequence.NextNoiseBiome(r[0]+x*r[3], r[1], r[2]+z*r[3])
						if !ok {
							t.Fatal("invalid grid sample")
						}
						if len(grid.Samples) > 0 {
							expected := grid.Samples[z*r[4]+x]
							if g.climate != nil {
								climate, ok := fresh.Climate(expected.Position[0], expected.Position[1], expected.Position[2])
								if !ok || climate != expected.Climate {
									t.Fatalf("dense climate at%v got%v want%v", expected.Position, climate, expected.Climate)
								}
							}
							if got != expected.Biome {
								t.Fatalf("dense biome at%v got%s want%s", expected.Position, got, expected.Biome)
							}
						}
						h.Write([]byte(got + "\n"))
					}
				}
				if got := fmt.Sprintf("%x", h.Sum64()); got != grid.Hash {
					t.Errorf("grid%v got%s want%s", r, got, grid.Hash)
				}
			}
		})
	}
}
