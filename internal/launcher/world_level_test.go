package launcher

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func splitWorldgenNBT(t *testing.T, seed int64) []byte {
	t.Helper()
	var raw bytes.Buffer
	writeSeedMapTestNBT(t, &raw, "", map[string]any{"data": map[string]any{
		"seed": seed,
		"dimensions": map[string]any{"minecraft:overworld": map[string]any{
			"type": "minecraft:overworld", "generator": map[string]any{
				"type": "minecraft:noise", "biome_source": map[string]any{"type": "minecraft:fixed", "biome": "test:marsh"},
			},
		}},
	}})
	return compressedNBT(t, raw.Bytes())
}

func TestSplitWorldMetadataSharedReaders(t *testing.T) {
	s, root, world := worldFixture(t)
	stale := int64(17)
	writeTestFile(t, filepath.Join(world, "level.dat"), seedMapLevelNBT(t, nil, &stale, nil, "pack:stale"))
	for _, seed := range []int64{0, -1, math.MinInt64, math.MaxInt64} {
		writeTestFile(t, filepath.Join(world, worldgenDataPath), splitWorldgenNBT(t, seed))
		level, err := readWorldLevel(context.Background(), world)
		if err != nil || !level.HasRandomSeed || level.RandomSeed != seed || level.Generators["minecraft:overworld"] != "minecraft:noise" {
			t.Fatalf("split metadata seed %d: %+v %v", seed, level, err)
		}
		for _, read := range []func() ([]World, error){
			func() ([]World, error) { return s.SeedMapSaves(context.Background(), root.ID, "") },
			func() ([]World, error) { return s.ListWorlds(root.ID, "") },
		} {
			worlds, err := read()
			if err != nil || len(worlds) != 1 || !worlds[0].HasRandomSeed || worlds[0].RandomSeed != seed {
				t.Fatalf("save list lost split seed %d: %+v %v", seed, worlds, err)
			}
		}
		snapshot, saved, err := s.SeedMapWorldgen(context.Background(), root.ID, "", "w1")
		if err != nil || !saved.HasRandomSeed || saved.RandomSeed != seed {
			t.Fatalf("map lost split seed %d: %+v %v", seed, saved, err)
		}
		sampler := testSeedMapSampler(t, snapshot, saved.RandomSeed)
		if got, ok := sampler.BiomeAt("minecraft:overworld", 0, 64, 0); !ok || got != "test:marsh" || len(sampler.Dimensions()) != 1 {
			t.Fatalf("saved dimension replaced by current preset: %s/%t", got, ok)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readWorldLevel(ctx, world); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled save read: %v", err)
	}
}

func TestSplitWorldMetadataRejectsBadCompanion(t *testing.T) {
	_, _, world := worldFixture(t)
	seed := int64(17)
	writeTestFile(t, filepath.Join(world, "level.dat"), seedMapLevelNBT(t, nil, &seed, nil, "minecraft:noise"))
	for _, data := range [][]byte{[]byte("not gzip"), seedMapLevelNBT(t, nil, nil, nil, ""), splitWorldgenNBT(t, 0)[:12]} {
		writeTestFile(t, filepath.Join(world, worldgenDataPath), data)
		if _, err := readWorldLevel(context.Background(), world); err == nil {
			t.Fatal("corrupt companion fell back to stale embedded metadata")
		}
	}
}

func TestSplitWorldMetadataConfinedToSave(t *testing.T) {
	_, _, world := worldFixture(t)
	outside := filepath.Join(t.TempDir(), "settings.dat")
	writeTestFile(t, outside, splitWorldgenNBT(t, 42))
	path := filepath.Join(world, worldgenDataPath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Skip("symlink creation unavailable", err)
	}
	if _, err := readWorldLevel(context.Background(), world); err == nil {
		t.Fatal("followed companion outside the selected save")
	}
}

func TestLevelDatStructuredWorldSpawn(t *testing.T) {
	var raw bytes.Buffer
	writeSeedMapTestNBT(t, &raw, "", map[string]any{"Data": map[string]any{
		"SpawnX": int32(99), "SpawnY": int32(70), "SpawnZ": int32(99),
		"spawn": map[string]any{"dimension": "test:dimension", "pos": []int32{0, -60, 0}},
	}})
	level, err := ParseLevelDat(bytes.NewReader(compressedNBT(t, raw.Bytes())))
	if err != nil || level.WorldSpawn == nil || *level.WorldSpawn != (SeedMapSpawn{X: 0, Y: -60, Z: 0, Dimension: "test:dimension", Kind: "world"}) {
		t.Fatalf("structured world spawn lost or mixed with legacy axes: %+v %v", level.WorldSpawn, err)
	}
}
