package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Viking602/YoloMCL/cubiomes"
	"github.com/Viking602/YoloMCL/cubiomes/modjar"
)

func TestCapturedRegionsUseSharedModernEngine(t *testing.T) {
	for _, version := range []string{"1.21.1", "26.3"} {
		for _, id := range []string{"arbitrary_one", "arbitrary_two"} {
			snapshot, err := WorldgenIngest(context.Background(), WorldgenInput{Minecraft: version})
			if err != nil {
				t.Fatal(err)
			}
			artifacts, err := cubiomes.BuiltinTerraBlenderArtifacts(version, "")
			if err != nil {
				t.Fatal(err)
			}
			for _, artifact := range artifacts {
				snapshot.Mods = append(snapshot.Mods, WorldgenMod{ID: artifact.ID, Version: artifact.Version, Loader: "fabric", SHA512: artifact.SHA512})
			}
			snapshot.Mods = append(snapshot.Mods, WorldgenMod{ID: id, TerraBlender: true})
			snapshot.Documents = append(snapshot.Documents, WorldgenDocument{Kind: "dimension_type_tag", ID: "terrablender:overworld_regions", Data: json.RawMessage(`{"values":["minecraft:overworld"]}`)})
			snapshot.ModConfigs = map[string][]byte{"terrablender.toml": []byte("[general]\nvanilla_overworld_region_weight=0\n")}
			snapshot.RegionPrograms = []WorldgenRegionProgram{{ModID: id, Result: &modjar.Result{Regions: []modjar.Registration{{ID: id + ":region", Dimension: cubiomes.Overworld, Weight: 1, Parameters: []cubiomes.TerraBlenderParameter{{Biome: "minecraft:desert"}}}}}}}
			rehashNativeModSnapshot(t, snapshot)
			sampler := testSeedMapSampler(t, snapshot, 0)
			for _, pos := range [][3]int{{0, 64, 0}, {-4097, -32, 8191}, {12000, 200, -4000}} {
				biome, ok := sampler.BiomeAt("minecraft:overworld", pos[0], pos[1], pos[2])
				if !ok || biome != "minecraft:desert" {
					t.Fatalf("%s / %s: %s %v %+v", version, id, biome, ok, sampler.Coverage("minecraft:overworld"))
				}
			}
		}
	}
}

func TestLegacyTerraBlenderDoesNotRequireBOPOrHookEnd(t *testing.T) {
	ctx := context.Background()
	snapshot, err := WorldgenIngest(ctx, WorldgenInput{Minecraft: "1.20.1"})
	if err != nil {
		t.Fatal(err)
	}
	vanilla := testSeedMapSampler(t, snapshot, 42)
	snapshot.Mods = []WorldgenMod{{ID: "terrablender", Version: "3.0.1.10", Loader: "forge", SHA512: terraBlender1201SHA512}, {ID: "ae2things", Version: "1", Loader: "forge"}}
	for tag, dim := range map[string]string{"overworld_regions": "overworld", "nether_regions": "the_nether"} {
		snapshot.Documents = append(snapshot.Documents, WorldgenDocument{Kind: "dimension_type_tag", ID: "terrablender:" + tag, Data: json.RawMessage(`{"values":["minecraft:` + dim + `"]}`)})
	}
	rehashNativeModSnapshot(t, snapshot)
	modded := testSeedMapSampler(t, snapshot, 42)
	for _, dim := range []string{"minecraft:overworld", "minecraft:the_nether", "minecraft:the_end"} {
		if c := modded.Coverage(dim); c.Opaque || c.Terrain == "candidates" {
			t.Fatalf("unrelated profile disabled %s: %+v", dim, c)
		}
		for _, pos := range [][3]int{{0, 64, 0}, {-15000, 20, 9000}, {8192, 128, -8192}} {
			got, ok := modded.BiomeAt(dim, pos[0], pos[1], pos[2])
			want, wok := vanilla.BiomeAt(dim, pos[0], pos[1], pos[2])
			if !ok || !wok || got != want {
				t.Fatalf("framework without regions changed %s: %s / %s", dim, got, want)
			}
		}
	}
	snapshot.RegionPrograms = []WorldgenRegionProgram{{ModID: "unsupported", Error: "unbound region API"}}
	rehashNativeModSnapshot(t, snapshot)
	unsupported := testSeedMapSampler(t, snapshot, 42)
	// A failed provider drops only its own regions: the overworld still
	// resolves but is flagged approximate; the End is untouched.
	over := unsupported.Coverage("minecraft:overworld")
	flagged := false
	for _, is := range over.Issues {
		flagged = flagged || is.Code == "region_skipped"
	}
	if over.Opaque || !flagged {
		t.Fatalf("failed region provider must be skipped and flagged: %+v", over)
	}
	for _, is := range unsupported.Coverage("minecraft:the_end").Issues {
		if is.Code == "region_skipped" {
			t.Fatal("regional adapter failure leaked into the End")
		}
	}
	if unsupported.Coverage("minecraft:the_end").Opaque {
		t.Fatal("regional adapter failure leaked into the End")
	}
}

func TestWorldgenManifestVersion(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "arbitrary.jar")
	writeTestFile(t, jar, zipBytes(t, map[string][]byte{
		"META-INF/mods.toml":   []byte("[[mods]]\nmodId='terrablender'\nversion='${file.jarVersion}'\n"),
		"META-INF/MANIFEST.MF": []byte("Manifest-Version: 1.0\r\nImplementation-Version: 3.0.\r\n 1.10\r\n\r\nName: another\r\nImplementation-Version: wrong\r\n"),
	}))
	snap, err := WorldgenIngest(context.Background(), WorldgenInput{Minecraft: "1.20.1", Sources: []WorldgenSource{{jar, "mod:arbitrary"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Mods) != 1 || snap.Mods[0].Version != "3.0.1.10" {
		t.Fatalf("manifest substitution: %+v", snap.Mods)
	}
}

func TestWorldgenInstalledRegistrationAndConfig(t *testing.T) {
	root := os.Getenv("MCL_REGISTRATION_INSTANCE")
	if root == "" {
		t.Skip("MCL_REGISTRATION_INSTANCE points to an installed 1.20.1 TB/YCB instance")
	}
	ctx := context.Background()
	input := WorldgenInput{Minecraft: "1.20.1", HasMods: true}
	for _, name := range []string{"TerraBlender-forge-1.20.1-3.0.1.10.jar", "YungsCaveBiomes-1.20.1-Forge-2.0.5.jar"} {
		input.Sources = append(input.Sources, WorldgenSource{filepath.Join(root, "mods", name), "mod:" + name})
	}
	initial, err := WorldgenIngest(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(initial.RegionPrograms) != 1 || initial.RegionPrograms[0].Error != "" || len(initial.RegionPrograms[0].Result.Regions) != 1 {
		t.Fatalf("registration ingest: %+v", initial.RegionPrograms)
	}
	s := testSeedMapSampler(t, initial, 0)
	if !slices.ContainsFunc(s.Biomes("minecraft:overworld"), func(b SeedMapBiome) bool { return b.ID == "yungscavebiomes:frosted_caves" }) || s.Coverage("minecraft:the_end").Opaque {
		t.Fatalf("native parameters did not reach the sampler: biomes=%v coverage=%+v", s.Biomes("minecraft:overworld"), s.Coverage("minecraft:overworld"))
	}
	name := ""
	input.ReadModConfig = func(n string) ([]byte, error) {
		name = n
		return []byte("[\"YUNG's Cave Biomes\".\"Frosted Caves\".\"Biome Noise Parameters\"]\n\"Temperature Min\"=-0.8\n"), nil
	}
	changed, err := WorldgenIngest(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if name != "YungsCaveBiomes-forge-1_20_1.toml" || len(changed.ModConfigs[name]) == 0 || changed.Hash == initial.Hash {
		t.Fatal("dynamically discovered configuration omitted from snapshot/hash")
	}
	params := changed.RegionPrograms[0].Result.Regions[0].Parameters
	if params[len(params)-2].Space[0][0] != -8000 {
		t.Fatal("changed config did not affect native recipe")
	}
	input.ReadModConfig = func(string) ([]byte, error) { return nil, errors.New("fixture unavailable") }
	broken, err := WorldgenIngest(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	failed := testSeedMapSampler(t, broken, 0)
	if !failed.Coverage("minecraft:overworld").Opaque || failed.Coverage("minecraft:the_end").Opaque {
		t.Fatal("unreadable region configuration reused defaults or disabled unrelated dimension")
	}
}
