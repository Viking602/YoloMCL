package launcher

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestReadPackMcmetaFolderAndTextComponent(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "MyFolder")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(folder, "pack.mcmeta"), []byte(
		`{"pack":{"pack_format":15,"description":[{"text":"Hello "},{"text":"§aWorld","color":"green"}]}}`,
	))
	if got := readPackMcmeta(folder); got != "Hello World" {
		t.Fatalf("folder component desc = %q", got)
	}

	zipPath := filepath.Join(dir, "comp.zip")
	var buf []byte
	{
		f, err := os.Create(zipPath)
		if err != nil {
			t.Fatal(err)
		}
		w := zip.NewWriter(f)
		wf, _ := w.Create("pack.mcmeta")
		_, _ = wf.Write([]byte(`{"pack":{"description":{"text":"Zip §bPack"}}}`))
		_ = w.Close()
		_ = f.Close()
		_ = buf
	}
	if got := readPackMcmeta(zipPath); got != "Zip Pack" {
		t.Fatalf("zip component desc = %q", got)
	}
}

func TestPackDescriptionTextPlain(t *testing.T) {
	if got := packDescriptionText("plain §cX"); got != "plain X" {
		t.Fatalf("%q", got)
	}
	if got := packDescriptionText(map[string]any{"translate": "pack.desc"}); got != "" {
		t.Fatalf("translate-only should be empty, got %q", got)
	}
}
