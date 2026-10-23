package release

import (
	"archive/tar"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSignedReleaseMatrixAndTampering(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	binary := filepath.Join(t.TempDir(), "launcher")
	if err := os.WriteFile(binary, []byte("test launcher"), 0o755); err != nil {
		t.Fatal(err)
	}
	commit := strings.Repeat("a", 40)
	for _, goos := range []string{"windows", "darwin", "linux"} {
		for _, arch := range []string{"amd64", "arm64"} {
			name := "YoloMCL"
			if goos == "windows" {
				name += ".exe"
			} else if goos == "darwin" {
				name = "YoloMCL.app/Contents/MacOS/YoloMCL"
			}
			if err := Package(dir, "v1.0.0", commit, goos+"-"+arch, []File{{Name: name, Path: binary, Mode: 0o755}}, private, time.Unix(1700000000, 0)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := Verify(dir, "v1.0.0", commit, public); err != nil {
		t.Fatal(err)
	}
	if err := Verify(dir, "v1.0.0", strings.Repeat("b", 40), public); err == nil {
		t.Fatal("accepted a different commit")
	}
	archive := filepath.Join(dir, "YoloMCL-1.0.0-linux-arm64.tar.gz")
	f, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	header, err := tr.Next()
	if err != nil || header.Name != "YoloMCL" || header.Mode != 0o755 {
		t.Fatalf("invalid portable archive entry: %+v, %v", header, err)
	}
	if _, err := tr.Next(); err != io.EOF {
		t.Fatalf("archive contains unexpected files: %v", err)
	}
	_ = gz.Close()
	_ = f.Close()
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)/2] ^= 1
	if err := os.WriteFile(archive, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Verify(dir, "v1.0.0", commit, public); err == nil {
		t.Fatal("accepted a tampered update")
	}
}

func TestArchiveRejectsUnsafePaths(t *testing.T) {
	_, private, _ := ed25519.GenerateKey(rand.Reader)
	for _, name := range []string{"../.env", "/etc/passwd", `C:\private`, "app/../config", "./YoloMCL"} {
		if err := Package(t.TempDir(), "v1.0.0", "test", "linux-amd64", []File{{Name: name}}, private, time.Now()); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
}
