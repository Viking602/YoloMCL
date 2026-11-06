package launcher

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
)

func TestWorldgenGlobalPackSelection(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	for _, name := range []string{"required/a.zip", "required/z.zip", "optional/disabled.zip"} {
		writeTestFile(t, filepath.Join(root, name), zipBytes(t, map[string][]byte{"pack.mcmeta": []byte(`{"pack":{"pack_format":48,"description":"test"}}`), "data/test/worldgen/biome/test.json": []byte(`{}`)}))
	}
	writeTestFile(t, filepath.Join(root, "config/global_packs.toml"), []byte("[datapacks]\nrequired=['required/']\noptional=['optional/']"))
	var issues []WorldgenIssue
	sources := seedMapGlobalPacks(ctx, root, "1.21.1", "fabric", nil, nil, &issues)
	names := []string{}
	for _, s := range sources {
		names = append(names, s.Name)
	}
	if !slices.Equal(names, []string{"globalpacks:z.zip", "globalpacks:a.zip"}) || len(issues) > 0 {
		t.Fatalf("manual required order: %v %v", names, issues)
	}
	sources = seedMapGlobalPacks(ctx, root, "1.21.1", "fabric", []string{"a.zip", "disabled.zip", "z.zip"}, nil, &issues)
	if len(sources) != 3 || sources[0].Name != "globalpacks:a.zip" || sources[1].Name != "globalpacks:disabled.zip" {
		t.Fatal("saved selection ignored", sources)
	}
	mod := filepath.Join(root, "global.jar")
	writeTestFile(t, mod, zipBytes(t, map[string][]byte{"fabric.mod.json": []byte(`{"id":"globalpacks","version":"21.0.6"}`)}))
	for _, active := range []bool{false, true} {
		input := WorldgenInput{Minecraft: "1.21.1", Loader: "fabric", Sources: sources}
		if active {
			input.Sources = append([]WorldgenSource{{mod, "mod:global.jar"}}, sources...)
		}
		snapshot, err := WorldgenIngest(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, doc := range snapshot.Documents {
			if doc.Kind == "biome" && doc.ID == "test:test" {
				found = true
				if doc.Source != "globalpacks:z.zip" {
					t.Fatal("priority", doc.Source)
				}
			}
		}
		if found != active {
			t.Fatal("configuration activated without its loader")
		}
	}
}

func TestWorldgenPaxiConfigAndOrder(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	for _, name := range []string{"datapacks/base.zip", "config/paxi/datapacks/a.zip", "config/paxi/datapacks/z.zip"} {
		writeTestFile(t, filepath.Join(root, name), zipBytes(t, map[string][]byte{"pack.mcmeta": []byte(`{"pack":{"pack_format":48,"description":"test"}}`)}))
	}
	writeTestFile(t, filepath.Join(root, "config/paxi/datapack_load_order.json"), []byte(`{"loadOrder":["z.zip","a.zip"]}`))
	writeTestFile(t, filepath.Join(root, "config/paxi-neoforge-1_21.toml"), []byte("[General]\n\"Load from base 'datapacks' directory\"=false"))
	var issues []WorldgenIssue
	sources := seedMapGlobalPacks(ctx, root, "1.21.1", "neoforge", nil, nil, &issues)
	if len(sources) != 2 || sources[0].Name != "paxi:z.zip" || sources[1].Name != "paxi:a.zip" || len(issues) != 0 {
		t.Fatalf("Paxi config/order: %v %v", sources, issues)
	}
}

func TestWorldgenLithostitchedDelegateAndLooseRules(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "data/example/lithostitched/biome_injector/overworld.json"), []byte(`{"type":"lithostitched:force_placement","dimension":"minecraft:overworld","biome":"example:injected","parameters":{}}`))
	writeTestFile(t, filepath.Join(root, "data/minecraft/dimension/overworld.json"), []byte(`{"type":"minecraft:overworld","generator":{"type":"minecraft:noise","settings":"minecraft:overworld","biome_source":{"type":"lithostitched:injector","delegate":{"type":"minecraft:multi_noise","preset":"minecraft:overworld"}}}}`))
	mod := filepath.Join(t.TempDir(), "litho.jar")
	writeTestFile(t, mod, zipBytes(t, map[string][]byte{"fabric.mod.json": []byte(`{"id":"lithostitched","version":"1.7.13-fabric-21.1"}`)}))
	snapshot, err := WorldgenIngest(context.Background(), WorldgenInput{Minecraft: "1.21.1", Loader: "fabric", Sources: []WorldgenSource{{mod, "mod:litho.jar"}, {root, "datapack:rules"}}})
	if err != nil {
		t.Fatal(err)
	}
	sampler, err := NewLocalSampler(context.Background(), snapshot, 0)
	if err != nil {
		t.Fatal(err)
	}
	if biome, ok := sampler.BiomeAt("minecraft:overworld", -120, 64, 128); !ok || biome != "example:injected" {
		t.Fatalf("delegate/injectors missing: %s %+v", biome, sampler.Coverage("minecraft:overworld"))
	}
}
