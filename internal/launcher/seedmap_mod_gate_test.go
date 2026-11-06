package launcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSeedMapTerrainModMetadata(t *testing.T) {
	for _, tc := range []struct {
		name    string
		files   map[string][]byte
		blocked bool
	}{
		{"client-ui", map[string][]byte{"fabric.mod.json": []byte(`{"id":"map_ui","environment":"client","depends":{"fabric-api":"*"}}`)}, false},
		{"shader-multiline-description", map[string][]byte{"fabric.mod.json": []byte("{\"id\":\"euphoria_patcher\",/* loader comment */\"environment\":\"client\",\"description\":\"Shader patcher\nsettings\"}")}, false},
		{"performance", map[string][]byte{"fabric.mod.json": []byte(`{"id":"lithium","version":"1"}`)}, false},
		{"unknown-biomes", map[string][]byte{"fabric.mod.json": []byte(`{"id":"new_mod"}`), "data/example/worldgen/biome/new.json": []byte(`{}`)}, true},
		{"optional-terrain-pack", map[string][]byte{"resourcepacks/optional/data/minecraft/worldgen/noise_settings/overworld.json": []byte(`{}`)}, true},
		{"fabric-code-generator", map[string][]byte{"fabric.mod.json": []byte(`{"id":"bigglobe","version":"1"}`)}, true},
		{"fabric-region-api", map[string][]byte{"fabric.mod.json": []byte(`{"id":"custom_biomes","entrypoints":{"terrablender":["example.Init"]}}`)}, true},
		{"quilt-generator", map[string][]byte{"quilt.mod.json": []byte(`{"schema_version":1,"quilt_loader":{"id":"terra","version":"1"}}`)}, true},
		{"forge-generator", map[string][]byte{"META-INF/mods.toml": []byte("[[mods]]\nmodId='terraforged'\nversion='1'\n")}, true},
		{"neoforge-api-dependency", map[string][]byte{"META-INF/neoforge.mods.toml": []byte("[[mods]]\nmodId='unknown_biomes'\nversion='1'\n[[dependencies.unknown_biomes]]\nmodId='terrablender'\ntype='required'\n")}, true},
		{"optional-api-dependency", map[string][]byte{"META-INF/mods.toml": []byte("[[mods]]\nmodId='ui_mod'\n[[dependencies.ui_mod]]\nmodId='terrablender'\nmandatory=false\n")}, false},
		{"legacy-generator", map[string][]byte{"mcmod.info": []byte(`{"modList":[{"modid":"rtg","version":"1"}]}`)}, true},
		{"structure-only", map[string][]byte{"data/example/worldgen/structure/new.json": []byte(`{}`), "data/example/worldgen/structure_set/new.json": []byte(`{}`)}, false},
		{"ore-only", map[string][]byte{"data/example/worldgen/configured_feature/ore.json": []byte(`{}`)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTestFile(t, filepath.Join(dir, "arbitrarily-renamed.jar"), zipBytes(t, tc.files))
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			if got := seedMapTerrainMod(root, "arbitrarily-renamed.jar"); got != tc.blocked {
				t.Fatalf("terrain gate = %v, want %v", got, tc.blocked)
			}
		})
	}
}

func TestSeedMapTerrainModNestedArchive(t *testing.T) {
	dir := t.TempDir()
	nested := zipBytes(t, map[string][]byte{"fabric.mod.json": []byte(`{"id":"tectonic","version":"1"}`)})
	writeTestFile(t, filepath.Join(dir, "wrapper.jar"), zipBytes(t, map[string][]byte{
		"fabric.mod.json":     []byte(`{"id":"wrapper","jars":[{"file":"bundled/terrain.jar"}]}`),
		"bundled/terrain.jar": nested,
	}))
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if !seedMapTerrainMod(root, "wrapper.jar") {
		t.Fatal("nested terrain mod was missed")
	}
}

func TestSeedMapTerrainGateTracksInstanceMods(t *testing.T) {
	s := testService(t)
	r := newTestRoot(t, s, "Terrain gate")
	writeTestFile(t, filepath.Join(r.Path, "versions", "one", "one.json"), []byte(`{"id":"one","mainClass":"Main"}`))
	writeTestFile(t, filepath.Join(r.Path, "versions", "two", "two.json"), []byte(`{"id":"two","mainClass":"Main"}`))
	jar := filepath.Join(r.Path, "versions", "one", "mods", "renamed.jar")
	writeTestFile(t, jar, zipBytes(t, map[string][]byte{"fabric.mod.json": []byte(`{"id":"tectonic"}`)}))
	if err := s.Select(r.ID, "", "one"); err != nil {
		t.Fatal(err)
	}
	s.invalidateInventory(r.ID)
	check := func(want bool) {
		t.Helper()
		if err := s.CheckSeedMapMods(context.Background(), r.ID, "one"); err != nil {
			t.Fatal(err)
		}
		for _, v := range s.Snapshot().Instances[0].Versions {
			if v.SeedMapDisabled != (v.ID == "one" && want) {
				t.Fatalf("gate leaked between instances: %+v", v)
			}
		}
	}
	check(true)
	if err := s.ToggleMod(r.ID, "renamed.jar"); err != nil {
		t.Fatal(err)
	}
	check(false)
	if err := s.ToggleMod(r.ID, "renamed.jar.disabled"); err != nil {
		t.Fatal(err)
	}
	check(true)
	old := s.inventoryEntry(r.Path).data
	writeTestFile(t, jar, zipBytes(t, map[string][]byte{"fabric.mod.json": []byte(`{"id":"ui_only","version":"2"}`)}))
	if inventoryUnchanged(r.Path, old) {
		t.Fatal("replacing an existing jar did not invalidate the gate")
	}
	ageInventoryCheck(s, r, time.Minute)
	s.RefreshInventories(context.Background())
	check(false)
	if err := os.Remove(jar); err != nil {
		t.Fatal(err)
	}
	s.invalidateInventory(r.ID)
	check(false)
}

func TestSeedMapTerrainCheckStaysOutOfSnapshot(t *testing.T) {
	s := testService(t)
	r := newTestRoot(t, s, "Deferred terrain gate")
	writeTestFile(t, filepath.Join(r.Path, "versions", "pack", "pack.json"), []byte(`{"id":"pack","mainClass":"Main"}`))
	writeTestFile(t, filepath.Join(r.Path, "versions", "pack", "mods", "unreadable.jar"), []byte("not a ZIP"))
	s.invalidateInventory(r.ID)
	v := s.Snapshot().Instances[0].Versions[0]
	if !v.SeedMapDisabled || !v.SeedMapCheckPending || len(s.inventoryEntry(r.Path).data.TerrainMods) != 0 {
		t.Fatal("snapshot read jar contents or exposed an unchecked map")
	}
	if err := s.CheckSeedMapMods(context.Background(), r.ID, "pack"); err != nil {
		t.Fatal(err)
	}
	v = s.Snapshot().Instances[0].Versions[0]
	if !v.SeedMapDisabled || v.SeedMapCheckPending {
		t.Fatal("an unreadable enabled mod was treated as unmodified terrain")
	}
}
