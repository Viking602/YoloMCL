package launcher

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
)

func TestSeedMapServiceKubeJSResources(t *testing.T) {
	s, r, _ := worldFixture(t)
	file := filepath.Join(r.Path, "kubejs/data/example/worldgen/biome/coast.json")
	writeTestFile(t, file, []byte(`{"effects":{"grass_color":123},"features":[[],[],]}`))
	ctx := context.Background()
	before, _, err := s.SeedMapWorldgen(ctx, r.ID, "", "w1")
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range before.Documents {
		if doc.ID == "example:coast" {
			t.Fatal("leftover KubeJS directory activated without the mod")
		}
	}
	writeTestFile(t, filepath.Join(r.Path, "mods/kubejs.jar"), zipBytes(t, map[string][]byte{
		"fabric.mod.json": []byte(`{"id":"kubejs","version":"2101.7.2-build.348"}`),
	}))
	writeTestFile(t, filepath.Join(r.Path, "kubejs/data/example/dimension_type/custom.json"), []byte(`{"height":384}`))
	writeTestFile(t, filepath.Join(r.Path, "kubejs/data/terrablender/tags/dimension_type/overworld_regions.json"), []byte(`{"values":["example:custom"]}`))
	snapshot, _, err := s.SeedMapWorldgen(ctx, r.ID, "", "w1")
	if err != nil || hasWorldgenIssue(snapshot, "invalid_json") || hasWorldgenIssue(snapshot, "source_unreadable") {
		t.Fatalf("instance pack load: %v %+v", err, snapshot.Issues)
	}
	for _, entry := range [][2]string{{"biome", "example:coast"}, {"dimension_type", "example:custom"}, {"dimension_type_tag", "terrablender:overworld_regions"}} {
		if d := worldgenDoc(t, snapshot, entry[0], entry[1]); d.Source != "kubejs:data" {
			t.Fatalf("wrong resource source: %+v", d)
		}
	}
	writeTestFile(t, file, []byte(`{"effects":{"grass_color":456}}`))
	changed, _, err := s.SeedMapWorldgen(ctx, r.ID, "", "w1")
	if err != nil || changed.Hash == snapshot.Hash {
		t.Fatal("edited instance pack reused stale prediction cache", err)
	}
}

func TestWorldgenKubeJS2101PackOrder(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"z.zip", "A.zip", "ignored.ZIP"} {
		writeTestFile(t, filepath.Join(root, "kubejs/data", name), zipBytes(t, nil))
	}
	sources := []WorldgenSource{{"base.jar", "mod:base"}, {"a.zip", "datapack:a.zip"}, {"b.zip", "datapack:b.zip"}, {"folder", "datapack:folder"}}
	var issues []WorldgenIssue
	got := seedMapInstancePacks(context.Background(), root, sources, &issues)
	var names []string
	for _, source := range got {
		names = append(names, source.Name)
	}
	want := []string{"mod:base", "datapack:a.zip", "kubejs:zip:A.zip", "kubejs:zip:z.zip", "kubejs:data", "datapack:b.zip", "datapack:folder"}
	if !slices.Equal(names, want) || len(issues) != 0 {
		t.Fatalf("pack precedence %v, issues %v", names, issues)
	}
}

func TestWorldgenSelectsActiveModDescriptor(t *testing.T) {
	for _, loader := range []string{"neoforge", "forge", "fabric"} {
		t.Run(loader, func(t *testing.T) {
			descriptors := map[string][]byte{
				"META-INF/mods.toml":                    []byte(`[[dependencies.${mod_id}]]`),
				"META-INF/neoforge.mods.toml":           []byte(`[[dependencies.${mod_id}]]`),
				"fabric.mod.json":                       []byte(`{"id":`),
				"data/example/worldgen/biome/test.json": []byte(`{}`),
			}
			if loader == "fabric" {
				descriptors["fabric.mod.json"] = []byte(`{"id":"example","version":"1.2"}`)
			} else {
				name := "META-INF/mods.toml"
				if loader == "neoforge" {
					name = "META-INF/neoforge.mods.toml"
				}
				descriptors[name] = []byte("[[mods]]\nmodId=\"example\"\nversion=\"1.2\"\n")
			}
			jar := filepath.Join(t.TempDir(), "example.jar")
			writeTestFile(t, jar, zipBytes(t, descriptors))
			snapshot, err := WorldgenIngest(context.Background(), WorldgenInput{Minecraft: "1.21.1", Loader: loader, Sources: []WorldgenSource{{jar, "mod:example.jar"}}})
			if err != nil || hasWorldgenIssue(snapshot, "source_unreadable") || len(snapshot.Mods) != 1 || snapshot.Mods[0].Loader != loader {
				t.Fatalf("active descriptor %s: %v %+v", loader, err, snapshot.Mods)
			}
			worldgenDoc(t, snapshot, "biome", "example:test")
		})
	}
}
