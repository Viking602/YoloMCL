package launcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func inventoryFixture(t *testing.T, s *Service) Root {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".minecraft")
	writeTestFile(t, filepath.Join(path, "versions", "pack", "pack.json"), []byte(`{"id":"pack","mainClass":"net.minecraft.client.main.Main","clientVersion":"1.21.1"}`))
	writeTestFile(t, filepath.Join(path, "versions", "pack", "mods", "one.jar"), []byte("one"))
	r, err := s.AddRoot(path, "pack")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestInventoryReusesSnapshotsAndRestarts(t *testing.T) {
	s := testService(t)
	r := inventoryFixture(t, s)
	first := s.Snapshot().Instances[0]
	entry := s.inventoryEntry(r.Path)
	data := entry.data
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() { _ = s.Snapshot() })
	}
	wg.Wait()
	if entry.data != data {
		t.Fatal("unchanged snapshots rebuilt the inventory")
	}
	if first.ModCount != 1 || len(first.Versions) != 1 {
		t.Fatalf("incomplete index: %+v", first)
	}
	// A UI's optimistic selection/config change cannot edit the cached list.
	first.Versions[0].ID = "edited by caller"
	if s.Snapshot().Instances[0].Versions[0].ID != "pack" {
		t.Fatal("snapshot aliases cached versions")
	}
	loaded := &Service{dir: s.dir}
	cached := loaded.inventoryEntry(r.Path)
	if cached.data == nil || cached.data.modCount("pack").Total != 1 {
		t.Fatal("startup did not restore the persisted inventory")
	}
	if !inventoryUnchanged(r.Path, cached.data) {
		t.Fatal("unchanged inventory failed validation")
	}
}

func TestInventoryInvalidatesOnMutationAndExternalChanges(t *testing.T) {
	s := testService(t)
	r := inventoryFixture(t, s)
	_ = s.Snapshot()
	if err := s.ToggleMod(r.ID, "one.jar"); err != nil {
		t.Fatal(err)
	}
	_ = s.Snapshot()
	entry := s.inventoryEntry(r.Path)
	if got := entry.data.modCount("pack"); got.Total != 1 || got.Enabled != 0 {
		t.Fatalf("toggle did not invalidate counts: %+v", got)
	}
	if err := s.RemoveMod(r.ID, "one.jar.disabled"); err != nil {
		t.Fatal(err)
	}
	if s.Snapshot().Instances[0].ModCount != 0 {
		t.Fatal("remove did not invalidate inventory")
	}
	old := entry.data
	writeTestFile(t, filepath.Join(r.Path, "versions", "pack", "mods", "two.jar"), []byte("two"))
	if inventoryUnchanged(r.Path, old) {
		t.Fatal("external mod addition was missed")
	}
	entry.mu.Lock()
	entry.checked = time.Time{}
	entry.mu.Unlock()
	data := s.refreshInventory(r, entry)
	if data.modCount("pack").Enabled != 1 {
		t.Fatal("external refresh kept old counts")
	}
	// A partially written version directory must still be monitored.
	if err := os.MkdirAll(filepath.Join(r.Path, "versions", "later"), 0755); err != nil {
		t.Fatal(err)
	}
	s.invalidateInventory(r.ID)
	_ = s.Snapshot()
	writeTestFile(t, filepath.Join(r.Path, "versions", "later", "later.json"), []byte(`{"id":"later","mainClass":"net.minecraft.client.main.Main"}`))
	if inventoryUnchanged(r.Path, entry.data) {
		t.Fatal("completion of a partial installation was missed")
	}
	entry.mu.Lock()
	entry.checked = time.Time{}
	entry.mu.Unlock()
	if len(s.refreshInventory(r, entry).Versions) != 2 {
		t.Fatal("completed installation not indexed")
	}
}

func TestInventoryJobCompletionInvalidates(t *testing.T) {
	s := testService(t)
	r := inventoryFixture(t, s)
	_ = s.Snapshot()
	id, err := s.startJob("inventory", r.ID, func(context.Context, progress) error {
		return os.WriteFile(filepath.Join(r.Path, "versions", "pack", "mods", "two.jar"), []byte("two"), 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, s, id); j.State != "completed" {
		t.Fatal(j.State)
	}
	if s.Snapshot().Instances[0].ModCount != 2 {
		t.Fatal("completed job still has stale inventory")
	}
}

func BenchmarkCachedSnapshot(b *testing.B) {
	s := &Service{dir: b.TempDir(), jobs: map[string]*jobState{}, running: map[string]*runningGame{}}
	path := b.TempDir()
	r := Root{ID: "root", Path: path}
	d := &inventoryData{Path: path, Available: true, Directories: map[string]string{}, Mods: map[string]inventoryMods{}}
	for i := range 100 {
		id := fmt.Sprint(i)
		d.Versions = append(d.Versions, VersionSummary{ID: id, Minecraft: "1.21.1", Loader: "neoforge"})
		d.Directories[id] = id
		d.Mods[id] = inventoryMods{Total: 300, Enabled: 300}
	}
	s.cfg.Roots = []Root{r}
	s.inventories = map[string]*inventoryEntry{path: {data: d, checked: time.Now().Add(time.Hour)}}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = s.Snapshot()
	}
}

func TestVersionDirUsesPublishedInventory(t *testing.T) {
	s := testService(t)
	r := inventoryFixture(t, s)
	writeTestFile(t, filepath.Join(r.Path, "mods", "shared.jar"), []byte("shared"))
	writeTestFile(t, filepath.Join(r.Path, "versions", "two", "two.json"), []byte(`{"id":"two","mainClass":"net.minecraft.client.main.Main"}`))
	s.invalidateInventory(r.ID)
	_ = s.Snapshot()

	probe := func(version string) string { return Root{Path: r.Path, SelectedVersion: version}.VersionDir() }
	two, err := s.rootAt(r.ID, "two")
	if err != nil {
		t.Fatal(err)
	}
	if two.gameDir == "" || two.VersionDir() != r.Path || two.VersionDir() != probe("two") {
		t.Fatalf("shared folder not served from the inventory: %q", two.VersionDir())
	}

	// Creating the isolation folder invalidates the inventory, which must
	// disable the cached directory until the next scan.
	isolated := filepath.Join(r.Path, "versions", "two")
	if err := os.MkdirAll(filepath.Join(isolated, "mods"), 0755); err != nil {
		t.Fatal(err)
	}
	s.invalidateInventory(r.ID)
	two, _ = s.rootAt(r.ID, "two")
	if two.gameDir != "" || filepath.Clean(two.VersionDir()) != filepath.Clean(isolated) {
		t.Fatalf("stale directory after invalidation: %q", two.VersionDir())
	}
	_ = s.Snapshot()
	two, _ = s.rootAt(r.ID, "two")
	if two.gameDir == "" || two.VersionDir() != probe("two") || filepath.Clean(two.VersionDir()) != filepath.Clean(isolated) {
		t.Fatalf("rescanned directory differs from probing: %q vs %q", two.VersionDir(), probe("two"))
	}

	// A cached directory belongs to the version it was resolved for.
	two.SelectedVersion = "pack"
	if two.VersionDir() != probe("pack") {
		t.Fatalf("directory leaked across versions: %q", two.VersionDir())
	}
}
