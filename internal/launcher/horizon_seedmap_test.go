package launcher

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupHorizonSeedMapRoot(t *testing.T) (*Service, Root, string) {
	t.Helper()
	s := testService(t)
	game := filepath.Join(t.TempDir(), "mcroot")
	buildID := "nightly-2026-11-20-0a875da"
	id := "horizon-" + buildID
	vDir := filepath.Join(game, "versions", id)
	if err := os.MkdirAll(filepath.Join(vDir, "horizon-mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	meta := gameVersion{
		ID:            id,
		ClientVersion: "26.3",
		Type:          "release",
		Horizon: &horizonVersionMeta{
			BuildID:    buildID,
			ReleaseTag: "nightly",
			Minecraft:  "26.3",
			JavaMajor:  25,
		},
	}
	meta.JavaVersion.MajorVersion = 25
	rawMeta, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(vDir, id+".json"), rawMeta)
	// Simulate vanilla 26.3 client jar installed alongside Horizon by horizon-prepare.
	writeTestFile(t, filepath.Join(game, "versions", "26.3", "26.3.jar"), testZip(t, map[string]string{
		"version.json": `{"pack_version":{"data_major":121,"data_minor":0}}`,
		"data/minecraft/worldgen/structure_set/strongholds.json":   `{"structures":[{"structure":"minecraft:stronghold","weight":1}],"placement":{"type":"minecraft:concentric_rings","distance":32,"spread":3,"count":128,"preferred_biomes":"#minecraft:stronghold_biased_to"}}`,
		"data/minecraft/worldgen/structure/village_plains.json":    `{"type":"minecraft:jigsaw","start_pool":"minecraft:village/plains/town_centers"}`,
		"data/minecraft/structure/village/plains/houses/small.nbt": "NBT_PLACEHOLDER",
	}))
	r, err := s.AddRoot(game, "HorizonRoot")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Select(r.ID, "", id); err != nil {
		t.Fatal(err)
	}
	return s, r, id
}

func TestHorizonSeedMapWorldgenSamplerAndAssets(t *testing.T) {
	ctx := context.Background()
	s, r, id := setupHorizonSeedMapRoot(t)

	snap, _, err := s.SeedMapWorldgen(ctx, r.ID, id, "")
	if err != nil {
		t.Fatalf("SeedMapWorldgen(horizon) failed: %v", err)
	}
	if snap.Minecraft != "26.3" || snap.Loader != "horizon" || snap.JavaMajor != 25 {
		t.Fatalf("unexpected horizon snapshot header: mc=%q loader=%q java=%d", snap.Minecraft, snap.Loader, snap.JavaMajor)
	}

	sampler, err := NewLocalSampler(ctx, snap, 0)
	if err != nil {
		t.Fatalf("NewLocalSampler(horizon) failed: %v", err)
	}
	cov := sampler.Coverage("minecraft:overworld")
	if cov.Terrain != "multi_noise" || cov.Opaque {
		t.Fatalf("unexpected overworld coverage: %+v", cov)
	}
	if !sampler.SupportsSurface("minecraft:overworld") {
		t.Fatal("expected Horizon 26.3 overworld to support surface rendering")
	}

	tile, err := sampler.Sample(ctx, SeedMapTileKey{
		X: 0, Y: 64, Z: 0, BlocksPerPixel: 4, Dimension: "minecraft:overworld", Surface: true,
	})
	if err != nil {
		t.Fatalf("Sample(horizon) failed: %v", err)
	}
	if tile == nil || tile.Image == nil || len(tile.Biomes) == 0 {
		t.Fatalf("expected non-empty tile & biomes, got %+v", tile)
	}

	biome, ok := sampler.BiomeAt("minecraft:overworld", 0, 64, 0)
	if !ok || biome == "" {
		t.Fatalf("BiomeAt failed: biome=%q ok=%v", biome, ok)
	}
	if _, ok := sampler.TeleportYAt("minecraft:overworld", 0, 0, 64); !ok {
		t.Fatal("expected TeleportYAt to resolve surface Y for Horizon 26.3")
	}
	if _, err := sampler.EstimateSpawn(ctx); err != nil {
		t.Fatalf("EstimateSpawn failed: err=%v", err)
	}
	searchRes, err := sampler.FindStructures(ctx, SeedMapSearch{
		Dimension: "minecraft:overworld", Structure: "minecraft:villages", X: 0, Y: 64, Z: 0, Radius: 2048, Limit: 16,
	})
	if err != nil || len(searchRes.Hits) == 0 {
		t.Fatalf("FindStructures failed: hits=%d err=%v", len(searchRes.Hits), err)
	}

	store, err := s.StructureAssetStoreWorld(r.ID, id, "")
	if err != nil {
		t.Fatalf("StructureAssetStoreWorld(horizon) failed: %v", err)
	}
	defer store.Close()
	if _, err := store.Read("data/minecraft/worldgen/structure/village_plains.json"); err != nil {
		t.Fatalf("expected Horizon asset store to open vanilla 26.3 client jar, layers=%v err=%v", store.Layers(), err)
	}
}

func makeMinimalLevelDat(t *testing.T, levelName string, seed int64) []byte {
	t.Helper()
	var nbt bytes.Buffer
	writeStr := func(s string) {
		_ = binary.Write(&nbt, binary.BigEndian, uint16(len(s)))
		nbt.WriteString(s)
	}
	// Root compound ""
	nbt.WriteByte(10)
	writeStr("")
	// Compound "Data"
	nbt.WriteByte(10)
	writeStr("Data")
	// String "LevelName"
	nbt.WriteByte(8)
	writeStr("LevelName")
	writeStr(levelName)
	// Long "RandomSeed"
	nbt.WriteByte(4)
	writeStr("RandomSeed")
	_ = binary.Write(&nbt, binary.BigEndian, seed)
	// End "Data"
	nbt.WriteByte(0)
	// End root
	nbt.WriteByte(0)

	var gzBuf bytes.Buffer
	gz := gzip.NewWriter(&gzBuf)
	if _, err := gz.Write(nbt.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return gzBuf.Bytes()
}

func TestHorizonSeedMapContentModsAndDatapacksJSON(t *testing.T) {
	ctx := context.Background()
	s, r, id := setupHorizonSeedMapRoot(t)
	vDir := filepath.Join(r.Path, "versions", id)

	// Enable one content mod and disable another via horizon-mods.txt.
	writeTestFile(t, filepath.Join(vDir, "horizon-mods.txt"), []byte("mod.enabled_pack:true\nmod.disabled_pack:false\n"))
	writeTestFile(t, filepath.Join(vDir, "horizon-mods", "enabled_pack.hmod"), testZip(t, map[string]string{
		"horizon-mod.toml": `id = "enabled_pack"`,
		"data/hdemo/worldgen/structure_set/enabled_towers.json": `{"structures":[{"structure":"hdemo:tower","weight":1}],"placement":{"type":"minecraft:random_spread","spacing":24,"separation":6,"salt":12345}}`,
		"data/hdemo/worldgen/structure/tower.json":              `{"type":"minecraft:jigsaw","start_pool":"hdemo:tower"}`,
	}))
	writeTestFile(t, filepath.Join(vDir, "horizon-mods", "disabled_pack.hmod"), testZip(t, map[string]string{
		"horizon-mod.toml": `id = "disabled_pack"`,
		"data/hdemo/worldgen/structure_set/disabled_towers.json": `{"structures":[{"structure":"hdemo:off","weight":1}],"placement":{"type":"minecraft:random_spread","spacing":24,"separation":6,"salt":54321}}`,
		"data/hdemo/worldgen/structure/off.json":                 `{"type":"minecraft:jigsaw","start_pool":"hdemo:off"}`,
	}))

	// Add a Horizon save with level.dat + datapacks.json (explicit on, default on, explicit off).
	worldDir := filepath.Join(vDir, "saves", "HorizonWorld")
	writeTestFile(t, filepath.Join(worldDir, "level.dat"), makeMinimalLevelDat(t, "Horizon World", 987654321))
	writeTestFile(t, filepath.Join(worldDir, "datapacks.json"), []byte(`{"enabled":["vanilla","file/on.zip"],"disabled":["file/off.zip"]}`))
	writeTestFile(t, filepath.Join(worldDir, "datapacks", "on.zip"), testZip(t, map[string]string{
		"pack.mcmeta": `{"pack":{"pack_format":121}}`,
		"data/dp/worldgen/structure_set/dp_on.json": `{"structures":[{"structure":"dp:on","weight":1}],"placement":{"type":"minecraft:random_spread","spacing":20,"separation":4,"salt":111}}`,
		"data/dp/worldgen/structure/dp_on.json":     `{"type":"minecraft:jigsaw","start_pool":"dp:on"}`,
	}))
	writeTestFile(t, filepath.Join(worldDir, "datapacks", "default_on.zip"), testZip(t, map[string]string{
		"pack.mcmeta": `{"pack":{"pack_format":121}}`,
		"data/dp/worldgen/structure_set/dp_default.json": `{"structures":[{"structure":"dp:default","weight":1}],"placement":{"type":"minecraft:random_spread","spacing":20,"separation":4,"salt":222}}`,
		"data/dp/worldgen/structure/dp_default.json":     `{"type":"minecraft:jigsaw","start_pool":"dp:default"}`,
	}))
	writeTestFile(t, filepath.Join(worldDir, "datapacks", "off.zip"), testZip(t, map[string]string{
		"pack.mcmeta": `{"pack":{"pack_format":121}}`,
		"data/dp/worldgen/structure_set/dp_off.json": `{"structures":[{"structure":"dp:off","weight":1}],"placement":{"type":"minecraft:random_spread","spacing":20,"separation":4,"salt":333}}`,
		"data/dp/worldgen/structure/dp_off.json":     `{"type":"minecraft:jigsaw","start_pool":"dp:off"}`,
	}))

	saves, err := s.SeedMapSaves(ctx, r.ID, id)
	if err != nil || len(saves) != 1 || !saves[0].HasRandomSeed || saves[0].RandomSeed != 987654321 {
		t.Fatalf("SeedMapSaves(horizon) = %+v err=%v", saves, err)
	}

	snap, level, err := s.SeedMapWorldgen(ctx, r.ID, id, "HorizonWorld")
	if err != nil {
		t.Fatalf("SeedMapWorldgen(horizon world) failed: %v", err)
	}
	if !level.HasRandomSeed || level.RandomSeed != 987654321 {
		t.Fatalf("expected save seed 987654321, got %+v", level)
	}
	docIDs := map[string]bool{}
	for _, doc := range snap.Documents {
		docIDs[doc.Kind+"/"+doc.ID] = true
	}
	for _, want := range []string{
		"structure_set/hdemo:enabled_towers",
		"structure_set/dp:dp_on",
		"structure_set/dp:dp_default",
	} {
		if !docIDs[want] {
			t.Errorf("missing expected Horizon worldgen doc %s", want)
		}
	}
	for _, unwanted := range []string{
		"structure_set/hdemo:disabled_towers",
		"structure_set/dp:dp_off",
	} {
		if docIDs[unwanted] {
			t.Errorf("disabled Horizon doc should not be loaded: %s", unwanted)
		}
	}

	store, err := s.StructureAssetStoreWorld(r.ID, id, "HorizonWorld")
	if err != nil {
		t.Fatalf("StructureAssetStoreWorld(horizon world) failed: %v", err)
	}
	defer store.Close()
	for _, want := range []string{
		"data/hdemo/worldgen/structure/tower.json",
		"data/dp/worldgen/structure/dp_on.json",
		"data/dp/worldgen/structure/dp_default.json",
	} {
		if _, err := store.Read(want); err != nil {
			t.Errorf("missing asset %s in Horizon structure store (layers=%v): %v", want, store.Layers(), err)
		}
	}
	for _, unwanted := range []string{
		"data/hdemo/worldgen/structure/off.json",
		"data/dp/worldgen/structure/dp_off.json",
	} {
		if b, err := store.Read(unwanted); err == nil {
			t.Errorf("disabled asset %s should not be loaded (got %s)", unwanted, strings.TrimSpace(string(b)))
		}
	}
}
