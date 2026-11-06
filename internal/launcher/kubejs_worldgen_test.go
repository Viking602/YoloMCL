package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanKubeJSWorldgen(t *testing.T) {
	dir := t.TempDir()
	js := `// WorldgenEvents.remove(e => e.removeFeatureById('x:commented'))
WorldgenEvents.remove(event => {
  event.removeFeatureById('underground_ores', ['minecraft:ore_diamond', "minecraft:ore_gold"])
  event.removeOres(props => { props.blocks = ['minecraft:coal_ore'] })
})
WorldgenEvents.add(event => {
  event.addOre(ore => { ore.id = 'kubejs:glowstone_test' })
})
`
	p := filepath.Join(dir, "kubejs", "startup_scripts")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "wg.js"), []byte(js), 0o644); err != nil {
		t.Fatal(err)
	}
	k := ScanKubeJSWorldgen(dir)
	if k == nil {
		t.Fatal("nil scan")
	}
	if len(k.RemovedFeatures) != 2 || k.RemovedFeatures[0] != "minecraft:ore_diamond" {
		t.Fatalf("removed %v", k.RemovedFeatures)
	}
	if k.AddedOres != 1 || len(k.Approximate) != 1 {
		t.Fatalf("ores %d approx %v", k.AddedOres, k.Approximate)
	}
	if ScanKubeJSWorldgen(t.TempDir()) != nil {
		t.Fatal("empty dir should be nil")
	}
}

// TestScanKubeJSWorldgenReal reads real instances named in
// KUBEJS_SCAN_DIRS (os.PathListSeparator separated game dirs).
func TestScanKubeJSWorldgenReal(t *testing.T) {
	dirs := filepath.SplitList(os.Getenv("KUBEJS_SCAN_DIRS"))
	if len(dirs) == 0 || dirs[0] == "" {
		t.Skip("KUBEJS_SCAN_DIRS not set")
	}
	for _, d := range dirs {
		k := ScanKubeJSWorldgen(d)
		if k == nil {
			t.Logf("%s: no worldgen", filepath.Base(d))
			continue
		}
		t.Logf("%s: scripts=%d data=%v removed=%d ores=%d approx=%d %v", filepath.Base(d), k.Scripts, k.DataDir != "", len(k.RemovedFeatures), k.AddedOres, len(k.Approximate), k.Approximate)
	}
}

func TestScanKubeJSWorldgenDataKinds(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "kubejs", "data", "demo", "worldgen", "structure_set")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "x.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	bm := filepath.Join(dir, "kubejs", "data", "demo", "forge", "biome_modifier")
	if err := os.MkdirAll(bm, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bm, "y.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// non-worldgen should be skipped
	other := filepath.Join(dir, "kubejs", "data", "demo", "recipes")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(other, "z.json"), []byte(`{}`), 0o644)
	k := ScanKubeJSWorldgen(dir)
	if k == nil || k.DataWorldgen != 2 {
		t.Fatalf("data worldgen: %+v", k)
	}
	if k.DataKinds["structure_set"] != 1 || k.DataKinds["biome_modifier"] != 1 {
		t.Fatalf("kinds %v", k.DataKinds)
	}
}
