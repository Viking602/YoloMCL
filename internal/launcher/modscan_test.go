package launcher

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func testJar(t *testing.T, path, id, name string, nested map[string][]byte) {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	w, _ := z.Create("fabric.mod.json")
	w.Write([]byte(`{"id":"` + id + `","name":"` + name + `","version":"1.0"}`))
	for n, b := range nested {
		w, _ := z.CreateHeader(&zip.FileHeader{Name: "META-INF/jars/" + n, Method: zip.Store})
		w.Write(b)
	}
	z.Close()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	if e := os.WriteFile(path, buf.Bytes(), 0o644); e != nil {
		t.Fatal(e)
	}
}

func nestedJar(id string) []byte {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	w, _ := z.Create("fabric.mod.json")
	w.Write([]byte(`{"id":"` + id + `"}`))
	z.Close()
	return buf.Bytes()
}

func TestModScanPersistentCacheAndOneRead(t *testing.T) {
	root := t.TempDir()
	for i, n := range []string{"zeta", "alpha", "mid"} {
		var nested map[string][]byte
		if i == 0 {
			nested = map[string][]byte{"lib.jar": nestedJar("inner-lib")}
		}
		testJar(t, filepath.Join(root, "mods", n+".jar"), n, n, nested)
	}
	mods, e := scanMods(root)
	if e != nil || len(mods) != 3 {
		t.Fatal(e, len(mods))
	}
	if mods[0].ModID != "alpha" || mods[2].ModID != "zeta" {
		t.Fatalf("order %v %v %v", mods[0].ModID, mods[1].ModID, mods[2].ModID)
	}
	for _, m := range mods {
		b, _ := os.ReadFile(filepath.Join(root, "mods", m.File))
		if m.Fingerprint != curseFingerprint(b) || m.SHA1 == "" {
			t.Fatalf("%s: fingerprint/sha1 not from the single read", m.File)
		}
	}
	if len(mods[2].Provides) != 1 || mods[2].Provides[0] != "inner-lib" {
		t.Fatalf("stored nested jar ids: %v", mods[2].Provides)
	}
	c := loadModCache(root)
	if len(c) != 3 || c["alpha.jar"].SHA1 != mods[0].SHA1 {
		t.Fatalf("cache %v", c)
	}
	// A fresh process (empty in-memory caches) is served from disk: poison the
	// cached name and see it come back without rehashing.
	e1 := c["alpha.jar"]
	e1.Meta.Name = "FromCache"
	c["alpha.jar"] = e1
	saveModCache(root, c)
	hashCache = sync.Map{}
	again, _ := scanMods(root)
	found := false
	for _, m := range again {
		if m.Name == "FromCache" {
			found = true
		}
	}
	if !found {
		t.Fatal("unchanged jar was rehashed instead of read from the disk cache")
	}
	// A changed jar (new mtime) is re-read.
	p := filepath.Join(root, "mods", "alpha.jar")
	_ = os.Chtimes(p, time.Now().Add(time.Hour), time.Now().Add(time.Hour))
	again, _ = scanMods(root)
	for _, m := range again {
		if m.Name == "FromCache" {
			t.Fatal("changed jar served from a stale cache")
		}
	}
	// Corrupt cache: tolerated and rebuilt.
	_ = os.WriteFile(filepath.Join(root, ".mcl", modCacheFile), []byte("{not json"), 0o644)
	if again, e = scanMods(root); e != nil || len(again) != 3 {
		t.Fatal("corrupt cache broke the scan", e)
	}
	if len(loadModCache(root)) != 3 {
		t.Fatal("cache not rebuilt")
	}
}

func TestModScanStreamKeepsOrder(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"c", "a", "b", "e", "d", "f", "g", "h"} {
		testJar(t, filepath.Join(root, "mods", n+".jar"), n, n, nil)
	}
	var parts [][]Mod
	var mu sync.Mutex
	final, e := scanModsStream(root, func(p []Mod) {
		mu.Lock()
		parts = append(parts, p)
		mu.Unlock()
	})
	if e != nil || len(parts) == 0 {
		t.Fatal("no partial list", e)
	}
	for _, p := range parts {
		if len(p) != len(final) {
			t.Fatal("partial list changed length")
		}
		for i := range p {
			if p[i].File != final[i].File {
				t.Fatalf("row %d moved: %s vs %s", i, p[i].File, final[i].File)
			}
		}
	}
	for _, m := range final {
		if m.Pending {
			t.Fatal("final list has pending rows")
		}
	}
	if !parts[0][0].Pending {
		t.Fatal("first partial should show placeholders")
	}
}

// Isolated version folder; one Modrinth request fails: the check still
// finishes, streams what it found and saves results in the version folder.
func TestCheckModsStepFailureDoesNotAbort(t *testing.T) {
	s := testService(t)
	root := t.TempDir()
	vDir := filepath.Join(root, "versions", "Pack")
	testJar(t, filepath.Join(vDir, "mods", "a.jar"), "a", "A", nil)
	testJar(t, filepath.Join(vDir, "mods", "b.jar"), "b", "B", nil)
	writeTestFile(t, filepath.Join(root, ".mcl", "pack-lock.json"), []byte(`{"minecraft":"1.21.1","loader":"fabric"}`))
	mods, _ := scanMods(vDir)
	known := map[string]mrVersion{mods[0].SHA1: {ID: "v1", ProjectID: "p1"}}
	fakeHTTP(t, s, map[string][]byte{modrinthAPI + "/version_files": jsonBytes(t, known)}) // /update fails
	r := Root{Path: root, SelectedVersion: "Pack"}
	var got []map[string]Mod
	e := s.checkModsIn(context.Background(), r, vDir, func(string, int, int) {}, func(b map[string]Mod) { got = append(got, b) })
	if e != nil {
		t.Fatalf("one failed step aborted the check: %v", e)
	}
	if len(got) == 0 || got[0][mods[0].SHA1].Status != "current" {
		t.Fatalf("partial results: %v", got)
	}
	if c := readChecks(vDir); c[mods[0].SHA1].ProjectID != "p1" {
		t.Fatal("results not saved in the version folder")
	}
	// Cancelled context stops cleanly.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := s.checkModsIn(ctx, r, vDir, func(string, int, int) {}, nil); e == nil {
		t.Fatal("cancelled check should report cancellation")
	}
}
