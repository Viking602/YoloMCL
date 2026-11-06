package launcher

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedMapLevelNBT(t *testing.T, legacy, modern *int64, packs []string, generator string) []byte {
	t.Helper()
	var b bytes.Buffer
	str := func(s string) { b.Write(binary.BigEndian.AppendUint16(nil, uint16(len(s)))); b.WriteString(s) }
	tag := func(kind byte, name string) { b.WriteByte(kind); str(name) }
	long := func(name string, n int64) { tag(4, name); b.Write(binary.BigEndian.AppendUint64(nil, uint64(n))) }
	tag(10, "")
	tag(10, "Data")
	tag(8, "LevelName")
	str("Seed test")
	if modern != nil || generator != "" {
		tag(10, "WorldGenSettings")
		if modern != nil {
			long("seed", *modern)
		}
		if generator != "" {
			tag(10, "dimensions")
			tag(10, "minecraft:overworld")
			tag(10, "generator")
			tag(8, "type")
			str(generator)
			b.Write([]byte{0, 0, 0})
		}
		b.WriteByte(0)
	}
	// Put legacy last to verify modern precedence is independent of tag order.
	if legacy != nil {
		long("RandomSeed", *legacy)
	}
	if packs != nil {
		tag(10, "DataPacks")
		tag(9, "Enabled")
		b.WriteByte(8)
		b.Write(binary.BigEndian.AppendUint32(nil, uint32(len(packs))))
		for _, p := range packs {
			str(p)
		}
		b.WriteByte(0)
	}
	// Wrong-path seeds must not overwrite the actual world's seed.
	tag(10, "Player")
	long("RandomSeed", 12345)
	b.WriteByte(0)
	b.Write([]byte{0, 0})
	return compressedNBT(t, b.Bytes())
}

func TestLevelDatSeedAndWorldgenMetadata(t *testing.T) {
	for _, seed := range []int64{0, -1, -9223372036854775808, 9223372036854775807} {
		for _, modern := range []bool{false, true} {
			legacy, latest := &seed, (*int64)(nil)
			if modern {
				old := int64(17)
				legacy, latest = &old, &seed
			}
			level, err := ParseLevelDat(bytes.NewReader(seedMapLevelNBT(t, legacy, latest, []string{"vanilla", "file/custom.zip"}, "pack:opaque")))
			if err != nil || !level.HasRandomSeed || level.RandomSeed != seed || !level.HasDataPacks || len(level.EnabledDataPacks) != 2 || level.Generators["minecraft:overworld"] != "pack:opaque" {
				t.Fatalf("seed %d modern %v: %+v, %v", seed, modern, level, err)
			}
		}
	}
	level, err := ParseLevelDat(bytes.NewReader(seedMapLevelNBT(t, nil, nil, nil, "")))
	if err != nil || level.HasRandomSeed {
		t.Fatalf("missing seed became zero: %+v, %v", level, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := parseLevelDat(ctx, bytes.NewReader(seedMapLevelNBT(t, nil, nil, nil, ""))); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

const seedMapTowerJSON = `{"structures":[{"structure":"pack:tower","weight":1}],"placement":{"type":"minecraft:random_spread","spacing":20,"separation":5,"salt":12345}}`

func worldgenDoc(t *testing.T, snapshot *WorldgenSnapshot, kind, id string) WorldgenDocument {
	t.Helper()
	for _, doc := range snapshot.Documents {
		if doc.Kind == kind && doc.ID == id {
			return doc
		}
	}
	t.Fatalf("missing %s %s", kind, id)
	return WorldgenDocument{}
}

func hasWorldgenIssue(snapshot *WorldgenSnapshot, code string) bool {
	for _, issue := range snapshot.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func TestWorldgenIngestJSONPriorityCacheAndContent(t *testing.T) {
	dir := t.TempDir()
	mod := filepath.Join(dir, "terrain.jar")
	writeTestFile(t, mod, zipBytes(t, map[string][]byte{
		"data/pack/worldgen/biome/marsh.json":              []byte(`{"temperature":0.7,"downfall":0.9,"effects":{"grass_color":1122867}}`),
		"data/pack/worldgen/noise_settings/overworld.json": []byte(`{"noise":{"min_y":-64,"height":384},"noise_router":{"final_density":"pack:terrain"}}`),
		"data/pack/worldgen/density_function/terrain.json": []byte(`{"type":"minecraft:add","argument1":1,"argument2":{"type":"minecraft:noise","noise":"pack:ridges","xz_scale":0.25,"y_scale":0.1}}`),
		"data/pack/worldgen/structure_set/towers.json":     []byte(seedMapTowerJSON),
		"data/pack/dimension/garden.json":                  []byte(`{"type":"minecraft:overworld","generator":{"type":"minecraft:noise","settings":"pack:overworld","biome_source":{"type":"minecraft:fixed","biome":"pack:marsh"}}}`),
		"data/../worldgen/biome/escape.json":               []byte(`{}`),
	}))
	pack := filepath.Join(dir, "pack")
	biome := filepath.Join(pack, "data/pack/worldgen/biome/marsh.json")
	writeTestFile(t, biome, []byte(`{"effects":{"grass_color":4478310}}`))
	input := WorldgenInput{Minecraft: "1.20.1", CacheDir: filepath.Join(dir, "cache"), HasMods: true, Sources: []WorldgenSource{{mod, "mod:terrain.jar"}, {pack, "datapack:garden"}}}
	snapshot, err := WorldgenIngest(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	doc := worldgenDoc(t, snapshot, "biome", "pack:marsh")
	if doc.Source != "datapack:garden" || !bytes.Contains(doc.Data, []byte("4478310")) {
		t.Fatalf("priority lost: %+v", doc)
	}
	worldgenDoc(t, snapshot, "noise_settings", "pack:overworld")
	worldgenDoc(t, snapshot, "density_function", "pack:terrain")
	if !snapshot.HasMods {
		t.Fatal("mod source metadata lost")
	}
	cached, err := WorldgenIngest(context.Background(), input)
	if err != nil || cached.Hash != snapshot.Hash {
		t.Fatalf("cache miss: %v", err)
	}
	cacheFile := filepath.Join(input.CacheDir, snapshot.Hash+".json")
	writeTestFile(t, cacheFile, []byte(`{"Schema":1,"Hash":"`+snapshot.Hash+`","Documents":[]}`))
	rebuilt, err := WorldgenIngest(context.Background(), input)
	if err != nil || len(rebuilt.Documents) != len(snapshot.Documents) {
		t.Fatalf("corrupt cache used: %v", err)
	}
	info, err := os.Stat(biome)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, biome, []byte(`{"effects":{"grass_color":4478311}}`))
	if err := os.Chtimes(biome, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	changed, err := WorldgenIngest(context.Background(), input)
	if err != nil || changed.Hash == snapshot.Hash {
		t.Fatalf("same-size/mtime content change missed: %v", err)
	}
	input.Minecraft = "1.21.1"
	versioned, err := WorldgenIngest(context.Background(), input)
	if err != nil || versioned.Hash == changed.Hash {
		t.Fatal("Minecraft version omitted from snapshot key")
	}
}

func TestWorldgenIngestInvalidAndBounded(t *testing.T) {
	dir := t.TempDir()
	pack := filepath.Join(dir, "bad.zip")
	writeTestFile(t, pack, zipBytes(t, map[string][]byte{
		"data/minecraft/worldgen/structure_set/villages.json": []byte(`{"placement":`),
		"data/pack/worldgen/density_function/deep.json":       []byte(strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65)),
	}))
	snap, err := WorldgenIngest(context.Background(), WorldgenInput{Minecraft: "1.20.1", Sources: []WorldgenSource{{pack, "datapack:bad"}}})
	if err != nil || !hasWorldgenIssue(snap, "invalid_json") {
		t.Fatalf("invalid JSON coverage: %+v %v", snap, err)
	}
	if doc := worldgenDoc(t, snap, "structure_set", "minecraft:villages"); string(doc.Data) != "null" {
		t.Fatal("invalid override resurrected vanilla villages")
	}
	writeTestFile(t, pack, zipBytes(t, map[string][]byte{"data/pack/worldgen/biome/huge.json": bytes.Repeat([]byte(" "), maxWorldgenJSON+1)}))
	snap, err = WorldgenIngest(context.Background(), WorldgenInput{Sources: []WorldgenSource{{pack, "datapack:huge"}}})
	if err != nil || !hasWorldgenIssue(snap, "source_unreadable") {
		t.Fatalf("oversized archive not reported: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := WorldgenIngest(ctx, WorldgenInput{Sources: []WorldgenSource{{pack, "test"}}}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, name := range []string{"../data/a/worldgen/biome/b.json", "data/a/worldgen/../biome/b.json", "data/a/worldgen/biome/../../b.json", "data\\a\\worldgen\\biome\\b.json"} {
		if _, _, ok := worldgenResource(name); ok {
			t.Fatalf("unsafe resource: %s", name)
		}
	}
}

func TestSeedMapServiceHonorsEnabledPacksWithoutRegions(t *testing.T) {
	s, r, w := worldFixture(t)
	seed := int64(-1900123456789012345)
	writeTestFile(t, filepath.Join(w, "level.dat"), seedMapLevelNBT(t, nil, &seed, []string{"vanilla", "file/b.zip", "file/a.zip"}, "pack:generator"))
	for _, name := range []string{"a", "b", "disabled"} {
		data, _ := json.Marshal(map[string]string{"name": name})
		writeTestFile(t, filepath.Join(w, "datapacks", name+".zip"), zipBytes(t, map[string][]byte{"data/pack/worldgen/biome/test.json": data}))
	}
	writeTestFile(t, filepath.Join(r.Path, "mods", "worldgen.jar.disabled"), zipBytes(t, map[string][]byte{"data/pack/worldgen/biome/disabled.json": []byte(`{}`)}))
	snap, level, err := s.SeedMapWorldgen(context.Background(), r.ID, "", "w1")
	if err != nil || !level.HasRandomSeed || level.RandomSeed != seed {
		t.Fatalf("%+v %v", level, err)
	}
	if doc := worldgenDoc(t, snap, "biome", "pack:test"); doc.Source != "datapack:a.zip" {
		t.Fatalf("enabled order ignored: %s", doc.Source)
	}
	if snap.HasMods {
		t.Fatal("disabled jar ingested")
	}
	// An invalid .mca payload exists in the fixture. Removing it must have no
	// effect on the prediction snapshot; a never-generated world works equally.
	if err := os.Remove(filepath.Join(w, "region", "r.0.0.mca")); err != nil {
		t.Fatal(err)
	}
	without, _, err := s.SeedMapWorldgen(context.Background(), r.ID, "", "w1")
	if err != nil || without.Hash != snap.Hash {
		t.Fatalf("prediction depends on region contents: %v", err)
	}
	worlds, err := s.ListWorlds(r.ID, "")
	if err != nil || len(worlds) != 1 || !worlds[0].HasRandomSeed || worlds[0].RandomSeed != seed {
		t.Fatalf("world summary lost seed: %+v %v", worlds, err)
	}
	if _, _, err := s.SeedMapWorldgen(context.Background(), r.ID, "", "../escape"); err == nil {
		t.Fatal("unsafe world accepted")
	}
}
