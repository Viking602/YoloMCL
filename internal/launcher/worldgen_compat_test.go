package launcher

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestWorldgenLargeZip64Inventory(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for i := 0; i < 65536; i++ {
		if _, err := z.CreateHeader(&zip.FileHeader{Name: fmt.Sprintf("assets/art/%d", i), Method: zip.Store}); err != nil {
			t.Fatal(err)
		}
	}
	f, err := z.Create("data/test/worldgen/biome/large.json")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte(`{}`))
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "large.jar")
	writeTestFile(t, path, b.Bytes())
	snapshot, err := WorldgenIngest(context.Background(), WorldgenInput{Sources: []WorldgenSource{{path, "datapack:large"}}})
	if err != nil || hasWorldgenIssue(snapshot, "source_unreadable") {
		t.Fatalf("valid ZIP64 rejected: %v", err)
	}
	worldgenDoc(t, snapshot, "biome", "test:large")
	raw := b.Bytes()
	at := bytes.LastIndex(raw, []byte{'P', 'K', 6, 6})
	if at < 0 {
		t.Fatal("fixture not ZIP64")
	}
	binary.LittleEndian.PutUint64(raw[at+24:at+32], maxWorldgenArchiveEntries+1)
	binary.LittleEndian.PutUint64(raw[at+32:at+40], maxWorldgenArchiveEntries+1)
	writeTestFile(t, path, raw)
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if worldgenZipEnvelope(file, int64(len(raw))) == nil {
		t.Fatal("unbounded ZIP64 accepted")
	}
}

func TestWorldgenModDescriptorsDoNotDiscardResources(t *testing.T) {
	for _, entry := range []struct{ name, raw, loader string }{
		{"fabric.mod.json", "{\"id\":\"addon\",\"version\":\"1\",\"description\":\"line one\nline two\"}", "fabric"},
		{"META-INF/neoforge.mods.toml", "[[mods]]\nmodId='addon'\nversion='1'\n[[dependencies]]\nmodId='ae2'", "neoforge"},
		{"META-INF/neoforge.mods.toml", "[[mods]]\nmodId='addon'\nversion='1'\n[[dependencies.1.21.1]]\nmodId='minecraft'", "neoforge"},
	} {
		t.Run(entry.loader+entry.raw, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "addon.jar")
			writeTestFile(t, path, zipBytes(t, map[string][]byte{entry.name: []byte(entry.raw), "data/addon/worldgen/biome/test.json": []byte(`{}`)}))
			snap, err := WorldgenIngest(context.Background(), WorldgenInput{Loader: entry.loader, Sources: []WorldgenSource{{path, "mod:addon.jar"}}})
			if err != nil || len(snap.Mods) != 1 || hasWorldgenIssue(snap, "source_unreadable") {
				t.Fatalf("metadata blocked resources: %v %+v", err, snap.Issues)
			}
			worldgenDoc(t, snap, "biome", "addon:test")
		})
	}
	if json.Valid(worldgenJSONControls([]byte("{\x01\"x\":1}"))) {
		t.Fatal("control outside string repaired")
	}
}

func TestWorldgenDuplicateUnrelatedModAndDimensionCapacity(t *testing.T) {
	snapshot := nativeModSnapshot(t)
	snapshot.Mods = append(snapshot.Mods, WorldgenMod{ID: "addon", Version: "1"}, WorldgenMod{ID: "addon", Version: "2"})
	for i := 0; i < 40; i++ {
		snapshot.Documents = append(snapshot.Documents, WorldgenDocument{Kind: "dimension", ID: fmt.Sprintf("aaa:planet%d", i), Data: json.RawMessage(`{"type":"minecraft:overworld","generator":{"type":"minecraft:noise","settings":"minecraft:overworld","biome_source":{"type":"minecraft:fixed","biome":"minecraft:plains"}}}`)})
	}
	rehashNativeModSnapshot(t, snapshot)
	sampler, err := NewLocalSampler(context.Background(), snapshot, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, dimension := range []string{"minecraft:overworld", "minecraft:the_nether", "minecraft:the_end"} {
		if _, ok := sampler.BiomeAt(dimension, 0, 64, 0); !ok {
			t.Fatalf("required dimension dropped: %s %+v", dimension, sampler.Coverage(dimension))
		}
	}
}
