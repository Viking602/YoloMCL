package toolutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFailedWritePreservesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "result.txt")
	if err := WriteFile(path, []byte("previous")); err != nil {
		t.Fatal(err)
	}
	want := errors.New("generation failed")
	err := WriteOutput(path, func(f *os.File) error {
		if _, err := f.WriteString("partial"); err != nil {
			return err
		}
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("got %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "previous" {
		t.Fatalf("lost existing file: %q %v", data, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary file left behind: %v %v", entries, err)
	}
	if err := WriteFile(path, []byte("complete")); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil || string(data) != "complete" {
		t.Fatalf("replacement failed: %q %v", data, err)
	}
}
