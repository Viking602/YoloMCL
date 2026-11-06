package launcher

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"testing"
)

const testClimateParameters = `{"biomes":[{"biome":"pack:cold","parameters":{"temperature":-1,"humidity":0,"continentalness":0,"erosion":0,"depth":0,"weirdness":0,"offset":0}},{"biome":"pack:warm","parameters":{"temperature":1,"humidity":0,"continentalness":0,"erosion":0,"depth":0,"weirdness":0,"offset":0}}]}`

func testClimateSettings(temperature string) string {
	return `{"noise_router":{"temperature":` + temperature + `,"vegetation":0,"continents":0,"erosion":0,"depth":0,"ridges":0}}`
}

func testClimateDimension(settings string) string {
	return `{"generator":{"type":"minecraft:noise","settings":` + settings + `,"biome_source":{"type":"minecraft:multi_noise","preset":"pack:climate"}}}`
}

func TestMultiNoiseOverridesTagsAndSavedDimensions(t *testing.T) {
	dir := t.TempDir()
	mod, pack := filepath.Join(dir, "mod.jar"), filepath.Join(dir, "pack")
	writeTestFile(t, mod, zipBytes(t, map[string][]byte{
		"data/pack/dimension/moon.json":                                           []byte(testClimateDimension(`"pack:settings"`)),
		"data/pack/worldgen/noise_settings/settings.json":                         []byte(testClimateSettings(`-1`)),
		"data/pack/worldgen/noise/heat.json":                                      []byte(`{"firstOctave":-4,"amplitudes":[1]}`),
		"data/pack/worldgen/density_function/heat.json":                           []byte(`-1`),
		"data/pack/worldgen/multi_noise_biome_source_parameter_list/climate.json": []byte(testClimateParameters),
		"data/pack/worldgen/structure_set/towers.json":                            []byte(seedMapTowerJSON),
		"data/pack/worldgen/structure/tower.json":                                 []byte(`{"biomes":"#pack:eligible"}`),
		"data/pack/tags/worldgen/biome/eligible.json":                             []byte(`{"values":["pack:cold"]}`),
		"data/pack/worldgen/biome/warm.json":                                      []byte(`{"effects":{"grass_color":16711680}}`),
	}))
	for path, raw := range map[string]string{
		"worldgen/noise_settings/settings": testClimateSettings(`"pack:heat"`),
		"worldgen/density_function/heat":   `{"type":"add","argument1":1,"argument2":{"type":"noise","noise":"pack:heat","xz_scale":1,"y_scale":1}}`,
		"worldgen/noise/heat":              `{"firstOctave":-4,"amplitudes":[0]}`,
		"tags/worldgen/biome/eligible":     `{"values":["pack:warm",{"id":"#pack:optional","required":false}]}`,
	} {
		writeTestFile(t, filepath.Join(pack, "data/pack", path+".json"), []byte(raw))
	}
	input := WorldgenInput{Minecraft: "1.20.1", HasMods: true, Sources: []WorldgenSource{{mod, "mod:terrain"}, {pack, "datapack:override"}}}
	snap, err := WorldgenIngest(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	s := testSeedMapSampler(t, snap, 42)
	if got, ok := s.BiomeAt("pack:moon", -123, 64, 456); !ok || got != "pack:warm" {
		t.Fatalf("override did not drive climate: %s %v", got, ok)
	}
	if c := s.Coverage("pack:moon"); c.Terrain != "multi_noise" {
		t.Fatalf("modded coverage %+v", c)
	}
	key := SeedMapTileKey{Dimension: "pack:moon", X: 1, Z: 1, Y: 64, BlocksPerPixel: 8}
	checkCandidates := func(s *LocalSampler, want bool, confidence string) {
		t.Helper()
		tile, err := s.Sample(context.Background(), key)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, h := range tile.Hits {
			if h.ID == "pack:towers" {
				found = true
				if h.Confidence != confidence {
					t.Fatalf("confidence: %+v", h)
				}
			}
		}
		if found != want {
			t.Fatalf("structure biome filter: found %v want %v", found, want)
		}
	}
	checkCandidates(s, true, "biome_candidate")
	tagPath := filepath.Join(pack, "data/pack/tags/worldgen/biome/eligible.json")
	writeTestFile(t, tagPath, []byte(`{"replace":true,"values":["pack:cold"]}`))
	replaced, err := WorldgenIngest(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if replaced.Hash == snap.Hash {
		t.Fatal("tag replacement omitted from cache key")
	}
	checkCandidates(testSeedMapSampler(t, replaced, 42), false, "")
	writeTestFile(t, tagPath, []byte(`{"replace":true,"values":["#pack:eligible"]}`))
	cyclic, err := WorldgenIngest(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	checkCandidates(testSeedMapSampler(t, cyclic, 42), false, "")
	key.Structure = "pack:towers"
	checkCandidates(testSeedMapSampler(t, cyclic, 42), true, "candidate")

	input.SavedWorld = true
	input.SavedDimensions = map[string]json.RawMessage{"pack:moon": json.RawMessage(testClimateDimension(testClimateSettings(`-1`)))}
	saved, err := WorldgenIngest(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Hash == cyclic.Hash {
		t.Fatal("saved settings omitted from cache key")
	}
	s = testSeedMapSampler(t, saved, 42)
	if ids := s.Dimensions(); len(ids) != 1 || ids[0] != "pack:moon" {
		t.Fatalf("saved dimension set: %v", ids)
	}
	if got, ok := s.BiomeAt("pack:moon", 0, 64, 0); !ok || got != "pack:cold" {
		t.Fatalf("saved inline settings lost: %s", got)
	}
}

func TestMultiNoiseCoverageQuartSamplingAndHeight(t *testing.T) {
	for _, tc := range []struct {
		raw, issue string
		opaque     bool
	}{
		{`{"type":"mod:custom_density"}`, "density_unsupported", true},
		{`"pack:missing"`, "climate_invalid", false},
	} {
		s := testSeedMapSampler(t, &WorldgenSnapshot{Documents: []WorldgenDocument{
			{Kind: "dimension", ID: "pack:moon", Data: json.RawMessage(testClimateDimension(testClimateSettings(tc.raw)))},
			{Kind: "multi_noise_biome_source_parameter_list", ID: "pack:climate", Data: json.RawMessage(testClimateParameters)},
		}}, 0)
		coverage := s.Coverage("pack:moon")
		found := false
		for _, issue := range coverage.Issues {
			found = found || issue.Code == tc.issue
		}
		if coverage.Terrain != "candidates" || coverage.Opaque != tc.opaque || !found {
			t.Fatalf("unsupported data pretended to work: %+v", coverage)
		}
	}
	snap := &WorldgenSnapshot{Documents: []WorldgenDocument{
		{Kind: "dimension", ID: "pack:moon", Data: json.RawMessage(testClimateDimension(testClimateSettings(`{"type":"y_clamped_gradient","from_y":0,"to_y":128,"from_value":-1,"to_value":1}`)))},
		{Kind: "multi_noise_biome_source_parameter_list", ID: "pack:climate", Data: json.RawMessage(testClimateParameters)},
	}}
	s := testSeedMapSampler(t, snap, 0)
	for y, want := range map[int]string{0: "pack:cold", 128: "pack:warm"} {
		if got, ok := s.BiomeAt("pack:moon", 15, y, 15); !ok || got != want {
			t.Fatalf("Y %d: %s", y, got)
		}
		key := SeedMapTileKey{Dimension: "pack:moon", X: 1, Z: 1, Y: y, BlocksPerPixel: 1}
		tile, err := s.Sample(context.Background(), key)
		if err != nil {
			t.Fatal(err)
		}
		for x := 8; x < 12; x++ {
			for z := 8; z < 12; z++ {
				if tile.Image.RGBAAt(x, z) != s.biomeColor(want) {
					t.Fatal("quart raster/color mismatch")
				}
			}
		}
	}
	for i := 0; i < 140; i++ {
		snap.Documents = append(snap.Documents, WorldgenDocument{Kind: "dimension", ID: fmt.Sprintf("pack:extra%02d", i), Data: json.RawMessage(testClimateDimension(testClimateSettings(`0`)))})
	}
	if got := len(testSeedMapSampler(t, snap, 0).Dimensions()); got != 128 {
		t.Fatalf("unbounded dimensions: %d", got)
	}
}

func TestClimatePresetRegistryAndBuiltinNames(t *testing.T) {
	snapshot, err := WorldgenIngest(context.Background(), WorldgenInput{Minecraft: "1.20.1"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Documents = append(snapshot.Documents,
		WorldgenDocument{Kind: "multi_noise_biome_source_parameter_list", ID: "minecraft:overworld", Data: json.RawMessage(`{"biomes":[{"biome":"pack:warm","parameters":{"temperature":0,"humidity":0,"continentalness":0,"erosion":0,"depth":0,"weirdness":0,"offset":0}}]}`)},
		WorldgenDocument{Kind: "multi_noise_biome_source_parameter_list", ID: "pack:climate", Data: json.RawMessage(`{"preset":"minecraft:overworld"}`)},
		WorldgenDocument{Kind: "dimension", ID: "pack:moon", Data: json.RawMessage(testClimateDimension(`"minecraft:overworld"`))},
	)
	s := testSeedMapSampler(t, snapshot, 0)
	for dimension, want := range map[string]string{"minecraft:overworld": "pack:warm", "pack:moon": "minecraft:river"} {
		if got, ok := s.BiomeAt(dimension, 0, 64, 0); !ok || got != want {
			t.Fatalf("preset semantics %s: %s want %s", dimension, got, want)
		}
	}
	snapshot.Documents = append(snapshot.Documents, WorldgenDocument{Kind: "multi_noise_biome_source_parameter_list", ID: "pack:climate", Data: json.RawMessage(`{"preset":"pack:climate"}`)})
	if _, ok := testSeedMapSampler(t, snapshot, 0).BiomeAt("pack:moon", 0, 64, 0); ok {
		t.Fatal("custom preset cycle became a biome")
	}
}

func TestSavedNBTMultiNoiseSourceAndInlineSettings(t *testing.T) {
	var dimension any
	if err := json.Unmarshal([]byte(testClimateDimension(testClimateSettings(`-1`))), &dimension); err != nil {
		t.Fatal(err)
	}
	root := map[string]any{"Data": map[string]any{"WorldGenSettings": map[string]any{"seed": int64(-1), "dimensions": map[string]any{"pack:moon": dimension}}}}
	var b bytes.Buffer
	writeSeedMapTestNBT(t, &b, "", root)
	level, err := ParseLevelDat(bytes.NewReader(compressedNBT(t, b.Bytes())))
	if err != nil {
		t.Fatal(err)
	}
	if level.RandomSeed != -1 || level.Generators["pack:moon"] != "minecraft:noise" {
		t.Fatalf("saved NBT: %+v", level)
	}
	s := testSeedMapSampler(t, &WorldgenSnapshot{SavedWorld: true, SavedDimensions: level.Dimensions, Generators: level.Generators, Documents: []WorldgenDocument{{Kind: "multi_noise_biome_source_parameter_list", ID: "pack:climate", Data: json.RawMessage(testClimateParameters)}}}, level.RandomSeed)
	if got, ok := s.BiomeAt("pack:moon", 0, 64, 0); !ok || got != "pack:cold" {
		t.Fatalf("saved NBT source/settings lost: %s", got)
	}
}

func writeSeedMapTestNBT(t *testing.T, b *bytes.Buffer, name string, value any) {
	t.Helper()
	str := func(s string) { b.Write(binary.BigEndian.AppendUint16(nil, uint16(len(s)))); b.WriteString(s) }
	kind := byte(0)
	switch value.(type) {
	case map[string]any:
		kind = 10
	case string:
		kind = 8
	case int64:
		kind = 4
	case int32:
		kind = 3
	case float64:
		kind = 6
	default:
		t.Fatalf("unexpected NBT test value %T", value)
	}
	b.WriteByte(kind)
	str(name)
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			writeSeedMapTestNBT(t, b, k, v[k])
		}
		b.WriteByte(0)
	case string:
		str(v)
	case int64:
		b.Write(binary.BigEndian.AppendUint64(nil, uint64(v)))
	case int32:
		b.Write(binary.BigEndian.AppendUint32(nil, uint32(v)))
	case float64:
		b.Write(binary.BigEndian.AppendUint64(nil, math.Float64bits(v)))
	}
}
