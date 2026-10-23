package launcher

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestLargeJarScanPreservesHashesAndMetadata(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mods", "large.jar")
	// Stored bytes force the bounded reader even though the test data is
	// compressible. Include whitespace and odd boundaries for MurmurHash2.
	payload := bytes.Repeat([]byte{9, 10, 13, 32, 0, 255, 7}, maxInMemoryJar/7+50)
	testJar(t, path, "large", "Large mod", map[string][]byte{"data.jar": payload})
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m Mod
	sha, fp, hasFP, err := readJarContext(context.Background(), path, int64(len(b)), &m)
	want := sha1.Sum(b)
	if err != nil || sha != hex.EncodeToString(want[:]) || !hasFP || fp != refFingerprint(b) || m.ModID != "large" || m.Name != "Large mod" {
		t.Fatalf("streamed jar differs: sha=%s fp=%d hasFP=%v meta=%+v err=%v", sha, fp, hasFP, m, err)
	}
}

func TestStreamFingerprintAcrossReadBoundaries(t *testing.T) {
	rng := rand.New(rand.NewSource(9))
	for _, size := range []int{0, 1, 3, 4, 17, 4097, 65537} {
		data := make([]byte, size)
		_, _ = rng.Read(data)
		filtered := uint32(0)
		for _, v := range data {
			if v != 9 && v != 10 && v != 13 && v != 32 {
				filtered++
			}
		}
		for _, chunk := range []int{1, 3, 7, 65536} {
			fp, err := streamFingerprint(context.Background(), bytes.NewReader(data), filtered, make([]byte, chunk))
			if err != nil || fp != refFingerprint(data) {
				t.Fatalf("size=%d chunk=%d: fp=%d err=%v", size, chunk, fp, err)
			}
		}
	}
}

func TestModScanCancellationStopsObsoletePack(t *testing.T) {
	root := t.TempDir()
	testJar(t, filepath.Join(root, "mods", "one.jar"), "one", "One", nil)
	ctx, cancel := context.WithCancel(context.Background())
	parts := 0
	_, err := scanModsStreamContext(ctx, root, func([]Mod) {
		parts++
		cancel() // user leaves as soon as the initial list is shown
	})
	if !errors.Is(err, context.Canceled) || parts != 1 {
		t.Fatalf("obsolete scan continued: err=%v parts=%d", err, parts)
	}
	if _, err := os.Stat(filepath.Join(root, ".mcl", modCacheFile)); !os.IsNotExist(err) {
		t.Fatal("cancelled scan wrote an incomplete cache")
	}
	if _, _, _, err := readJarContext(ctx, "does-not-exist", 10<<20, &Mod{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled read touched the next jar: %v", err)
	}
}

func BenchmarkColdLargeJar(b *testing.B) {
	path := filepath.Join(b.TempDir(), "large.jar")
	f, err := os.Create(path)
	if err != nil {
		b.Fatal(err)
	}
	z := zip.NewWriter(f)
	w, _ := z.Create("fabric.mod.json")
	_, _ = w.Write([]byte(`{"id":"large","name":"Large mod","version":"1"}`))
	w, _ = z.CreateHeader(&zip.FileHeader{Name: "assets/data.bin", Method: zip.Store})
	chunk := bytes.Repeat([]byte{0, 9, 10, 13, 32, 255, 3}, 10000)
	for i := 0; i < 480; i++ {
		if _, err := w.Write(chunk); err != nil {
			b.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		b.Fatal(err)
	}
	if err := f.Close(); err != nil {
		b.Fatal(err)
	}
	st, _ := os.Stat(path)
	b.SetBytes(st.Size())
	b.ReportAllocs()
	for b.Loop() {
		var m Mod
		if _, _, _, err := readJarOnce(path, st.Size(), &m); err != nil {
			b.Fatal(err)
		}
	}
}
