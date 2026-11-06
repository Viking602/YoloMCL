package launcher

import (
	"archive/zip"
	"bytes"
	"context"
	"path/filepath"
	"testing"
)

func TestWorldgenRegistrationSourcesSkipsNonWorldgen(t *testing.T) {
	root := t.TempDir()
	crash := writeWorldgenZip(t, root, "crash.jar", map[string][]byte{
		"fabric.mod.json":             []byte(`{"schemaVersion":1,"id":"crash_assistant","version":"1"}`),
		"crash_assistant.mixins.json": []byte(`{}`),
	})
	biome := writeWorldgenZip(t, root, "biome.jar", map[string][]byte{
		"data/sample/worldgen/biome/x.json": []byte(`{}`),
	})
	paths := map[string]string{"crash": crash, "biome": biome}
	mods := []WorldgenMod{{ID: "crash_assistant", Source: "crash"}, {ID: "sample", Source: "biome"}}
	keep := worldgenRegistrationSources(context.Background(), paths, mods, []bool{false, false})
	if keep["crash"] || !keep["biome"] {
		t.Fatalf("keep=%v", keep)
	}
}

func writeWorldgenZip(t *testing.T, root, name string, files map[string][]byte) string {
	t.Helper()
	path := filepath.Join(root, name)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for n, b := range files {
		f, err := w.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(b); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, buf.Bytes())
	return path
}
