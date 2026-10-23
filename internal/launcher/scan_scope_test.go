package launcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRelativeLauncherDataKeepsManagedJavaInScope(t *testing.T) {
	t.Chdir(t.TempDir())
	s, err := New("launcher-data")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(s.dir) {
		t.Fatal("launcher data root is relative")
	}
	path := javaFixture(t, filepath.Join(s.runtimeDir(), "jdk21"), "21.0.10")
	explicit, roots := s.javaHints()
	items, err := s.scanJavaHints(context.Background(), false, explicit, roots, nil)
	if err != nil || len(items) != 1 || items[0].Path != path {
		t.Fatalf("managed Java escaped the relative data scope: %+v, %v", items, err)
	}
}

func TestMinecraftRuntimeWalkStaysInMinecraftAndLauncher(t *testing.T) {
	s := testService(t)
	r := inventoryFixture(t, s)
	inside := javaFixture(t, filepath.Join(r.Path, "versions", "pack", "runtime", "jdk21"), "21.0.10")
	managed := javaFixture(t, filepath.Join(s.runtimeDir(), "jdk17"), "17.0.15")
	outsideHome := t.TempDir()
	outside := javaFixture(t, outsideHome, "25.0.1")
	javaFixture(t, filepath.Join(filepath.Dir(r.Path), "java"), "26.0.1")
	t.Setenv("JAVA_HOME", outsideHome)
	t.Setenv("JDK_HOME", outsideHome)
	t.Setenv("JRE_HOME", outsideHome)
	t.Setenv("PATH", filepath.Dir(outside))
	// System Java is detected separately as exact paths. The Minecraft
	// runtime walker must still not enumerate parents or unrelated folders.
	for _, deep := range []bool{false, true} {
		explicit, roots := s.javaHints()
		items, err := s.scanJavaHints(context.Background(), deep, explicit, roots, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 2 {
			t.Fatalf("deep=%v: want two scoped runtimes, got %+v", deep, items)
		}
		found := map[string]bool{}
		for _, j := range items {
			found[j.Path] = true
		}
		if !found[inside] || !found[managed] || found[outside] {
			t.Fatalf("scan escaped scope: %+v", items)
		}
	}
	// Explicitly choosing an external executable is still supported; only
	// that executable's metadata is inspected, not its parent or siblings.
	s.mu.Lock()
	s.cfg.Settings.JavaPath = outside
	s.mu.Unlock()
	explicit, roots := s.javaHints()
	items, err := s.scanJavaHints(context.Background(), false, explicit, roots, nil)
	if err != nil || len(items) != 3 {
		t.Fatalf("explicit Java unavailable: %+v, %v", items, err)
	}
}

func TestCleanupLeavesParentsAndUnrelatedFilesAlone(t *testing.T) {
	s := testService(t)
	r := inventoryFixture(t, s)
	keep := []string{
		filepath.Join(filepath.Dir(r.Path), ".mcl-import-user", "keep"),
		filepath.Join(r.Path, "documents", "temp.mcl-part"),
		filepath.Join(r.Path, "versions", "pack", "unrelated", "temp.mcl-part"),
	}
	remove := []string{
		filepath.Join(s.dir, "cache", "temp.mcl-part"),
		filepath.Join(r.Path, "versions", "pack", "mods", "temp.mcl-part"),
		filepath.Join(r.Path, ".mcl", "update-staging", "temp"),
	}
	for _, p := range append(append([]string{}, keep...), remove...) {
		writeTestFile(t, p, []byte("fixture"))
		old := time.Now().Add(-48 * time.Hour)
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(filepath.Join(r.Path, ".mcl", "update-staging"), old, old); err != nil {
		t.Fatal(err)
	}
	fresh := filepath.Join(s.dir, "cache", "new-preview.pack")
	writeTestFile(t, fresh, []byte("active preview"))
	keep = append(keep, fresh)
	s.sweepTemp()
	for _, p := range keep {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("outside cleanup scope: %s", p)
		}
	}
	for _, p := range remove {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("owned temporary file survived: %s", p)
		}
	}
}

func TestScanScopeRejectsEscapingDirectoryLinks(t *testing.T) {
	s := testService(t)
	r := inventoryFixture(t, s)
	outside := t.TempDir()
	javaFixture(t, filepath.Join(outside, "jdk"), "25.0.1")
	keep := filepath.Join(outside, "temp.mcl-part")
	writeTestFile(t, keep, []byte("keep"))
	if err := scanTestDirectoryLink(outside, filepath.Join(r.Path, "runtime")); err != nil {
		t.Skipf("directory links unavailable: %v", err)
	}
	if err := scanTestDirectoryLink(outside, filepath.Join(r.Path, "resourcepacks")); err != nil {
		t.Fatal(err)
	}
	explicit, roots := s.javaHints()
	items, err := s.scanJavaHints(context.Background(), true, explicit, roots, nil)
	if err != nil || len(items) != 0 {
		t.Fatalf("followed runtime link: %+v, %v", items, err)
	}
	s.sweepTemp()
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("cleanup traversed outside link")
	}
	writeTestFile(t, filepath.Join(outside, "external.json"), []byte(`{"id":"external","mainClass":"net.minecraft.client.main.Main"}`))
	if err := scanTestDirectoryLink(outside, filepath.Join(r.Path, "versions", "external")); err != nil {
		t.Fatal(err)
	}
	if versions := scanInventory(r).Versions; len(versions) != 1 || versions[0].ID != "pack" {
		t.Fatalf("indexed an outside version: %+v", versions)
	}
}
