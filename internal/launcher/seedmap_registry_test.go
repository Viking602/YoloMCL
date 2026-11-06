package launcher

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSeedMapStructuresFollowEnabledSourcesAndDimensions(t *testing.T) {
	s, root, _ := worldFixture(t)
	mod := filepath.Join(root.Path, "mods", "atlas.jar")
	files := map[string][]byte{
		"fabric.mod.json":                             []byte(`{"id":"atlas","version":"1"}`),
		"data/minecraft/dimension/overworld.json":     []byte(`{"generator":{"type":"minecraft:noise","biome_source":{"type":"minecraft:fixed","biome":"minecraft:plains"}}}`),
		"data/atlas/dimension/depths.json":            []byte(`{"generator":{"type":"minecraft:noise","biome_source":{"type":"minecraft:fixed","biome":"atlas:depths"}}}`),
		"data/atlas/tags/worldgen/biome/surface.json": []byte(`{"values":["minecraft:plains"]}`),
	}
	add := func(id, biomes string) {
		files["data/independent/worldgen/structure/"+id+".json"] = []byte(`{"biomes":` + biomes + `}`)
		files["data/independent/worldgen/structure_set/"+id+".json"] = []byte(`{"structures":[{"structure":"independent:` + id + `","weight":1}],"placement":{"type":"minecraft:random_spread","spacing":16,"separation":4,"salt":1}}`)
	}
	add("observatory", `"#atlas:surface"`)
	add("cavern", `"atlas:depths"`)
	add("shared", `["minecraft:plains","atlas:depths"]`)
	add("disabled", `[]`)
	writeTestFile(t, mod, zipBytes(t, files))
	writeTestFile(t, filepath.Join(root.Path, "mods", "not-enabled.jar.disabled"), zipBytes(t, map[string][]byte{
		"data/disabled/worldgen/structure_set/towers.json": []byte(seedMapTowerJSON),
	}))
	load := func() *LocalSampler {
		t.Helper()
		snapshot, _, err := s.SeedMapWorldgen(context.Background(), root.ID, "", "")
		if err != nil {
			t.Fatal(err)
		}
		return testSeedMapSampler(t, snapshot, 42)
	}
	ids := func(sampler *LocalSampler, dimension string) []string {
		var ids []string
		for _, structure := range sampler.Structures(dimension) {
			ids = append(ids, structure.ID)
		}
		return ids
	}
	sampler := load()
	if got := ids(sampler, "minecraft:overworld"); !slices.Equal(got, []string{"independent:observatory", "independent:shared"}) {
		t.Fatalf("Overworld list did not follow installed registry: %v", got)
	}
	if got := ids(sampler, "atlas:depths"); !slices.Equal(got, []string{"independent:cavern", "independent:shared"}) {
		t.Fatalf("custom dimension list: %v", got)
	}
	if _, err := sampler.FindStructures(context.Background(), SeedMapSearch{Dimension: "minecraft:overworld", Structure: "independent:cavern", Radius: 4096, Limit: 8, Y: 64}); err == nil {
		t.Fatal("search accepted a structure excluded from this dimension")
	}
	tile, err := sampler.Sample(context.Background(), SeedMapTileKey{Dimension: "minecraft:overworld", BlocksPerPixel: 4, Y: 64})
	if err != nil || len(tile.Hits) == 0 {
		t.Fatalf("dynamic placements did not reach the map: %v", err)
	}
	for _, hit := range tile.Hits {
		if !slices.Contains(ids(sampler, "minecraft:overworld"), hit.ID) {
			t.Fatalf("map and sidebar disagree: %s", hit.ID)
		}
	}
	// A pack can introduce another namespace without an identically named mod.
	pack := filepath.Join(root.Path, "datapacks", "custom.zip")
	writeTestFile(t, pack, zipBytes(t, map[string][]byte{
		"data/pack/worldgen/structure_set/towers.json": []byte(seedMapTowerJSON),
		"data/pack/worldgen/structure/tower.json":      json.RawMessage(`{"biomes":"minecraft:plains"}`),
	}))
	if !slices.Contains(ids(load(), "minecraft:overworld"), "pack:towers") {
		t.Fatal("new datapack structure was absent")
	}
	if err := s.ToggleMod(root.ID, "atlas.jar"); err != nil {
		t.Fatal(err)
	}
	if got := ids(load(), "minecraft:overworld"); !slices.Equal(got, []string{"pack:towers"}) {
		t.Fatalf("disabled mod's structures survived: %v", got)
	}
	if err := s.ToggleMod(root.ID, "atlas.jar.disabled"); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(ids(load(), "minecraft:overworld"), "independent:observatory") {
		t.Fatal("re-enabled mod's structure did not return")
	}
	if err := os.Remove(mod); err != nil {
		t.Fatal(err)
	}
	if got := ids(load(), "minecraft:overworld"); !slices.Equal(got, []string{"pack:towers"}) {
		t.Fatalf("removed mod's structures survived: %v", got)
	}
}
