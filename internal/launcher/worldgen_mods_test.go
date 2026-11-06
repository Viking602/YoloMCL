package launcher

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Viking602/YoloMCL/cubiomes"
)

func nativeModSnapshot(t testing.TB) *WorldgenSnapshot {
	t.Helper()
	return nativeModSnapshotProfile(t, "26.3.0.0.15")
}

func nativeModSnapshotProfile(t testing.TB, bopVersion string) *WorldgenSnapshot {
	t.Helper()
	snapshot, err := WorldgenIngest(context.Background(), WorldgenInput{Minecraft: "26.3"})
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := cubiomes.BuiltinTerraBlenderArtifacts("26.3", bopVersion)
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range artifacts {
		snapshot.Mods = append(snapshot.Mods, WorldgenMod{ID: artifact.ID, Version: artifact.Version, Loader: "fabric", SHA512: artifact.SHA512})
	}
	for tag, id := range map[string]string{"overworld_regions": "overworld", "nether_regions": "the_nether", "end_regions": "the_end"} {
		snapshot.Documents = append(snapshot.Documents, WorldgenDocument{Kind: "dimension_type_tag", ID: "terrablender:" + tag, Data: json.RawMessage(`{"values":["minecraft:` + id + `"]}`)})
	}
	rehashNativeModSnapshot(t, snapshot)
	return snapshot
}

func rehashNativeModSnapshot(t testing.TB, snapshot *WorldgenSnapshot) {
	t.Helper()
	snapshot.Hash = ""
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	snapshot.Hash = hex.EncodeToString(hash[:])
}

func TestWorldgenNativeModsOracle(t *testing.T) {
	t.Run("BOP", func(t *testing.T) { testWorldgenNativeModsOracle(t, "26.3.0.0.15", "terrablender-bop-26.3.json.gz") })
	t.Run("TerraBlender", func(t *testing.T) { testWorldgenNativeModsOracle(t, "", "terrablender-only-26.3.json.gz") })
}

func testWorldgenNativeModsOracle(t *testing.T, bopVersion, filename string) {
	file, err := os.Open(filepath.Join("../../cubiomes/testdata", filename))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var fixture struct {
		Cases []struct {
			Dimension, Seed string
			Grids           []struct {
				Range [6]int
				Names string `json:"fnv1a_names"`
			}
		}
	}
	if err := json.NewDecoder(reader).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	snapshot := nativeModSnapshotProfile(t, bopVersion)
	ctx := context.Background()
	for _, c := range fixture.Cases {
		t.Run(c.Dimension+"/"+c.Seed, func(t *testing.T) {
			seed, err := strconv.ParseInt(c.Seed, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			sampler, err := NewLocalSampler(ctx, snapshot, seed)
			if err != nil {
				t.Fatal(err)
			}
			id := map[string]string{"overworld": "minecraft:overworld", "nether": "minecraft:the_nether", "end": "minecraft:the_end"}[c.Dimension]
			d := sampler.dimensions[id]
			if d.engine == nil {
				t.Fatalf("native mod adapter not bound: %+v", sampler.Coverage(id))
			}
			for _, grid := range c.Grids {
				r := cubiomes.Range{X: grid.Range[0], Y: grid.Range[1], Z: grid.Range[2], StepXZ: grid.Range[3], SizeX: grid.Range[4], SizeZ: grid.Range[5], SizeY: 1, StepY: 1}
				ids := make([]cubiomes.BiomeID, r.SizeX*r.SizeZ)
				if err := d.engine.NewSampler().GenerateIDs(ctx, r, ids); err != nil {
					t.Fatal(err)
				}
				names := fnv.New64a()
				for _, id := range ids {
					fmt.Fprintln(names, d.palette[id].ID)
				}
				if got := strconv.FormatUint(names.Sum64(), 16); got != grid.Names {
					t.Fatalf("native adapter grid %v: %s, want %s", grid.Range, got, grid.Names)
				}
			}
		})
	}
}

func TestWorldgenNativeModIdentityAndConfig(t *testing.T) {
	ctx := context.Background()
	snapshot := nativeModSnapshot(t)
	baseHash := snapshot.Hash
	snapshot.Mods[0].SHA512 = "changed-code-with-the-same-version"
	rehashNativeModSnapshot(t, snapshot)
	if snapshot.Hash == baseHash {
		t.Fatal("changed injected code reused the same snapshot")
	}
	sampler, err := NewLocalSampler(ctx, snapshot, 42)
	if err != nil {
		t.Fatal(err)
	}
	if sampler.dimensions["minecraft:overworld"].engine != nil {
		t.Fatal("artifact mismatch reused the cached mod generator")
	}
	snapshot = nativeModSnapshot(t)
	snapshot.ModConfigs = map[string][]byte{"terrablender.toml": []byte("[general]\noverworld_region_size=6\nvanilla_overworld_region_weight=0\n")}
	rehashNativeModSnapshot(t, snapshot)
	if snapshot.Hash == baseHash {
		t.Fatal("configuration change reused the same snapshot")
	}
	sampler, err = NewLocalSampler(ctx, snapshot, -1)
	if err != nil || sampler.dimensions["minecraft:overworld"].engine == nil {
		t.Fatalf("valid source configuration rejected: %v", err)
	}
	settings, err := parseWorldgenModSettings(&WorldgenSnapshot{ModConfigs: map[string][]byte{
		"terrablender.toml": []byte("[general]\noverworld_region_size=99\nvanilla_overworld_region_weight=-1\n"),
	}}, "terrablender.toml", worldgenModSettings{"general": {"overworld_region_size": 3, "vanilla_overworld_region_weight": 10}})
	if err != nil || settings["general"]["overworld_region_size"] != 3 || settings["general"]["vanilla_overworld_region_weight"] != 10 {
		t.Fatalf("out-of-range config must use source defaults: %v %v", settings, err)
	}
}

func TestWorldgenNativeModInstalledArtifacts(t *testing.T) {
	dir := os.Getenv("MCL_WORLDGEN_FIXTURES")
	if dir == "" {
		t.Skip("MCL_WORLDGEN_FIXTURES points to the external pinned mod artifacts")
	}
	artifacts, err := cubiomes.BuiltinTerraBlenderArtifacts("26.3", "26.3.0.0.15")
	if err != nil {
		t.Fatal(err)
	}
	input := WorldgenInput{Minecraft: "26.3", HasMods: true}
	for _, artifact := range artifacts {
		input.Sources = append(input.Sources, WorldgenSource{filepath.Join(dir, artifact.Filename), "mod:" + artifact.Filename})
	}
	snapshot, err := WorldgenIngest(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Mods) != 4 {
		t.Fatalf("installed identities: %+v", snapshot.Mods)
	}
	sampler, err := NewLocalSampler(context.Background(), snapshot, 42)
	if err != nil {
		t.Fatal(err)
	}
	for _, dimension := range []string{"minecraft:overworld", "minecraft:the_nether", "minecraft:the_end"} {
		if sampler.dimensions[dimension].engine == nil {
			t.Fatalf("installed native profile not bound: %+v", sampler.Coverage(dimension))
		}
		if _, err := sampler.Sample(context.Background(), SeedMapTileKey{Dimension: dimension, X: 16, Z: -16, Y: 64, BlocksPerPixel: 4}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWorldgenTerraBlenderDimensionTags(t *testing.T) {
	snapshot := nativeModSnapshot(t)
	snapshot.Documents = append(snapshot.Documents,
		WorldgenDocument{Kind: "dimension_type_tag", ID: "pack:types", Data: json.RawMessage(`{"values":["pack:shared"]}`)},
		WorldgenDocument{Kind: "dimension_type_tag", ID: "terrablender:nether_regions", Data: json.RawMessage(`{"values":["#pack:types","minecraft:the_nether"]}`)},
	)
	for _, tc := range []struct {
		name, generator, mode, dimensionType string
		dimension                            cubiomes.Dimension
		applies                              bool
	}{
		{"overworld_type", "minecraft:noise", "multi_noise", "minecraft:overworld", cubiomes.Overworld, true},
		{"untagged_type", "minecraft:noise", "multi_noise", "pack:untagged", 0, false},
		{"nested_tag", "minecraft:noise", "multi_noise", "pack:shared", cubiomes.Nether, true},
		{"end_tag_multi_noise", "minecraft:noise", "multi_noise", "minecraft:the_end", cubiomes.End, false},
		{"end_source_untagged", "minecraft:noise", "the_end", "pack:untagged", cubiomes.End, true},
		{"custom_generator", "pack:custom", "multi_noise", "minecraft:overworld", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dimension, applies, err := terraBlenderDimension(context.Background(), snapshot, seedMapDimension{generator: tc.generator, mode: tc.mode, dimensionType: tc.dimensionType})
			if err != nil || dimension != tc.dimension || applies != tc.applies {
				t.Fatalf("binding %v/%v/%v, want %v/%v", dimension, applies, err, tc.dimension, tc.applies)
			}
		})
	}
}

func TestWorldgenNativeAdaptersIgnoreUnrelatedMods(t *testing.T) {
	snapshot := nativeModSnapshot(t)
	snapshot.Mods = append(snapshot.Mods, WorldgenMod{ID: "custom", Version: "1", Loader: "fabric"})
	rehashNativeModSnapshot(t, snapshot)
	sampler, err := NewLocalSampler(context.Background(), snapshot, 42)
	if err != nil {
		t.Fatal(err)
	}
	if sampler.dimensions["minecraft:overworld"].engine == nil {
		t.Fatal("unrelated mod disabled native prediction")
	}
}

func TestWorldgenUnrelatedModsKeepIndependentDimensions(t *testing.T) {
	ctx := context.Background()
	for _, version := range []string{"1.12.2", "1.21.1", "26.3"} {
		t.Run(version, func(t *testing.T) {
			snapshot, err := WorldgenIngest(ctx, WorldgenInput{Minecraft: version})
			if err != nil {
				t.Fatal(err)
			}
			vanilla, err := NewLocalSampler(ctx, snapshot, 0)
			if err != nil {
				t.Fatal(err)
			}
			snapshot.Mods = []WorldgenMod{{ID: "ae2things", Loader: "neoforge"}}
			snapshot.Documents = append(snapshot.Documents, WorldgenDocument{Kind: "dimension", ID: "custom:world", Data: json.RawMessage(`{"generator":{"type":"custom:generator"}}`)})
			rehashNativeModSnapshot(t, snapshot)
			sampler, err := NewLocalSampler(ctx, snapshot, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, dimension := range []string{"minecraft:overworld", "minecraft:the_nether", "minecraft:the_end"} {
				for _, p := range [][2]int{{0, 0}, {-400, 1200}, {9000, -8192}} {
					want, wantOK := vanilla.BiomeAt(dimension, p[0], 64, p[1])
					got, ok := sampler.BiomeAt(dimension, p[0], 64, p[1])
					if !wantOK || !ok || got != want || sampler.Coverage(dimension).Opaque {
						t.Fatalf("unrelated mod disabled %s at %v: %q/%v, want %q", dimension, p, got, ok, want)
					}
				}
			}
			if coverage := sampler.Coverage("custom:world"); !coverage.Opaque || coverage.Terrain != "candidates" {
				t.Fatalf("unsupported generator was presented as supported: %+v", coverage)
			}
		})
	}
}

func BenchmarkSeedMapModSessionWithSeed(b *testing.B) {
	snapshot := nativeModSnapshot(b)
	ctx := context.Background()
	if _, err := NewLocalSampler(ctx, snapshot, 42); err != nil {
		b.Fatal(err)
	}
	var seed int64
	b.ReportAllocs()
	for b.Loop() {
		if _, err := NewLocalSampler(ctx, snapshot, seed); err != nil {
			b.Fatal(err)
		}
		seed++
	}
}
