package launcher

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/structure3d"
)

func testZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for n, d := range files {
		f, _ := w.Create(n)
		_, _ = f.Write([]byte(d))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Isolated instances keep mods under versions/<id>/mods. The structure store
// must read them there, plus world datapacks on top.
func TestStructureAssetStoreUsesInstanceGameDir(t *testing.T) {
	s := testService(t)
	game := filepath.Join(t.TempDir(), "mcroot")
	vDir := filepath.Join(game, "versions", "Pack")
	writeTestFile(t, filepath.Join(vDir, "Pack.json"), []byte(`{"id":"Pack","inheritsFrom":"1.21.1","mainClass":"x"}`))
	writeTestFile(t, filepath.Join(vDir, "mods", "demo.jar"), testZip(t, map[string]string{
		"fabric.mod.json":                         `{"schemaVersion":1,"id":"demo","version":"1"}`,
		"data/demo/worldgen/structure/hut.json":   `{"type":"minecraft:jigsaw","start_pool":"demo:hut"}`,
		"data/demo/worldgen/structure/tower.json": `{"type":"minecraft:jigsaw","start_pool":"demo:from_mod"}`,
	}))
	writeTestFile(t, filepath.Join(vDir, "saves", "W", "datapacks", "dp.zip"), testZip(t, map[string]string{
		"pack.mcmeta": `{}`,
		"data/demo/worldgen/structure/tower.json": `{"type":"minecraft:jigsaw","start_pool":"demo:from_datapack"}`,
	}))
	if err := os.MkdirAll(filepath.Join(game, "mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	r, err := s.AddRoot(game, "Root")
	if err != nil {
		t.Fatal(err)
	}
	store, err := s.StructureAssetStoreWorld(r.ID, "Pack", "W")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if got, err := ResolvePreviewTarget(store, "demo:hut", ""); err != nil || got != "demo:hut" {
		t.Fatalf("resolve = %q %v", got, err)
	}
	if _, err := store.Read("data/demo/worldgen/structure/hut.json"); err != nil {
		t.Fatalf("mod in versions/Pack/mods not loaded: layers=%v", store.Layers())
	}
	b, err := store.Read("data/demo/worldgen/structure/tower.json")
	if err != nil || !strings.Contains(string(b), "from_datapack") {
		t.Fatalf("world datapack should override mod, got %s %v", b, err)
	}
}

func TestOpenEmbeddedResourcePackAndModOrder(t *testing.T) {
	dir := t.TempDir()
	modsDir := filepath.Join(dir, "mods")
	if err := os.MkdirAll(modsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// dependent.jar requires base; both alphabetical would put base first anyway —
	// use z_base / a_dep so alpha would put dep first; topo must put base first.
	writeTestFile(t, filepath.Join(modsDir, "z_base.jar"), testZip(t, map[string]string{
		"fabric.mod.json":                                      `{"schemaVersion":1,"id":"base","version":"1"}`,
		"assets/base/textures/block/a.png":                     "BASE",
		"resourcepacks/extra/assets/base/textures/block/a.png": "EMBED",
		"resourcepacks/extra/pack.mcmeta":                      `{"pack":{"description":"extra"}}`,
	}))
	writeTestFile(t, filepath.Join(modsDir, "a_dep.jar"), testZip(t, map[string]string{
		"fabric.mod.json":                  `{"schemaVersion":1,"id":"dep","version":"1","depends":{"base":"*"}}`,
		"assets/base/textures/block/a.png": "DEP",
	}))
	writeTestFile(t, filepath.Join(dir, "options.txt"), []byte(
		`resourcePacks:["vanilla","base:extra"]`,
	))

	mods := []Mod{
		{File: "a_dep.jar", ModID: "dep", Enabled: true, Requires: []string{"base"}},
		{File: "z_base.jar", ModID: "base", Enabled: true},
	}
	ordered := sortModsForAssetLayers(mods)
	if len(ordered) != 2 || ordered[0].ModID != "base" || ordered[1].ModID != "dep" {
		t.Fatalf("topo order = %+v", ordered)
	}

	store := &structure3d.Store{}
	defer store.Close()
	for _, m := range ordered {
		_ = store.OpenJar("mod:"+m.File, filepath.Join(modsDir, m.File))
	}
	openStructureResourcePacks(store, dir, ordered, func() []AssetPack { return nil })
	b, err := store.Read("assets/base/textures/block/a.png")
	if err != nil || string(b) != "EMBED" {
		t.Fatalf("embedded overlay got %q %v layers=%v", b, err, store.Layers())
	}
}

func TestSortModsForAssetLayersCycleAlpha(t *testing.T) {
	mods := []Mod{
		{File: "b.jar", ModID: "b", Requires: []string{"a"}},
		{File: "a.jar", ModID: "a", Requires: []string{"b"}},
	}
	got := sortModsForAssetLayers(mods)
	if len(got) != 2 {
		t.Fatal(got)
	}
	// cycle → both remain; alpha by File
	if got[0].File != "a.jar" || got[1].File != "b.jar" {
		t.Fatalf("cycle alpha = %+v", got)
	}
}
