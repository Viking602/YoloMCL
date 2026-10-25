package launcher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"
)

func TestOverviewInputsIgnoreUnchangedCounts(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "mods", "one.jar")
	writeTestFile(t, file, []byte("old bytes"))
	info, err := os.Stat(filepath.Dir(file))
	if err != nil {
		t.Fatal(err)
	}
	first, err := readOverviewInputs(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, file, []byte("replacement JAR contents do not change the overview counters"))
	if err := os.Chtimes(filepath.Dir(file), info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	second, err := readOverviewInputs(context.Background(), dir)
	if err != nil || first.stamp != second.stamp {
		t.Fatalf("JAR contents invalidated summary metadata: %v", err)
	}
	if err := os.Rename(file, file+".disabled"); err != nil {
		t.Fatal(err)
	}
	// The filename itself must detect a toggle even on a filesystem with
	// coarse directory timestamps.
	if err := os.Chtimes(filepath.Dir(file), info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	third, err := readOverviewInputs(context.Background(), dir)
	if err != nil || third.stamp == first.stamp {
		t.Fatalf("mod toggle was missed: %v", err)
	}
}

func TestOverviewInputsDetectInPlaceMetadataChanges(t *testing.T) {
	for _, path := range []string{"screenshots/view.png", "logs/latest.log", "crash-reports/crash.txt", "saves/world/level.dat", "saves/world/icon.png", "backups/world/backup.zip", ".mcl/playstats.json"} {
		t.Run(path, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, filepath.FromSlash(path))
			writeTestFile(t, file, []byte("old"))
			info, err := os.Stat(file)
			if err != nil {
				t.Fatal(err)
			}
			first, err := readOverviewInputs(context.Background(), dir)
			if err != nil {
				t.Fatal(err)
			}
			if path == "backups/world/backup.zip" {
				// Backup counts depend on additions/removals, not archive contents.
				writeTestFile(t, filepath.Join(filepath.Dir(file), "another.zip"), []byte("new"))
				if err := os.Chtimes(filepath.Dir(file), info.ModTime().Add(time.Second), info.ModTime().Add(time.Second)); err != nil {
					t.Fatal(err)
				}
			} else {
				writeTestFile(t, file, []byte("new data"))
				if err := os.Chtimes(file, info.ModTime().Add(time.Second), info.ModTime().Add(time.Second)); err != nil {
					t.Fatal(err)
				}
			}
			second, err := readOverviewInputs(context.Background(), dir)
			if err != nil || second.stamp == first.stamp {
				t.Fatalf("changed summary input stayed cached: %v", err)
			}
		})
	}
}

func TestOverviewInputsCancelDuringEnumeration(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.jar", "b.jar", "c.jar", "d.jar"} {
		writeTestFile(t, filepath.Join(dir, "mods", name), []byte("mod"))
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checking := &overviewCancelContext{Context: ctx, cancel: cancel, remaining: 3}
	if _, err := readOverviewInputs(checking, dir); !errors.Is(err, context.Canceled) {
		t.Fatalf("superseded overview continued enumerating: %v", err)
	}
}

type overviewCancelContext struct {
	context.Context
	cancel    context.CancelFunc
	remaining int
}

func (c *overviewCancelContext) Err() error {
	c.remaining--
	if c.remaining == 0 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestOverviewCacheRejectsLatePublication(t *testing.T) {
	s := &Service{}
	dir := t.TempDir()
	entry := s.statusEntry(dir)
	entry.overviewScan = 2
	s.storeOverview(entry, 2, 22, InstanceStatus{Dir: dir, Mods: 2})
	s.storeOverview(entry, 1, 11, InstanceStatus{Dir: dir, Mods: 1})
	if st, ok := s.cachedOverview(entry, dir, 22); !ok || st.Mods != 2 {
		t.Fatal("late overview replaced newer metadata")
	}
	s.invalidateStatus(dir)
	s.storeOverview(entry, 2, 22, InstanceStatus{Dir: dir, Mods: 2})
	if _, ok := s.cachedOverview(entry, dir, 22); ok {
		t.Fatal("late overview resurrected an invalidated cache")
	}
}

func TestOverviewInputsDoNotEnumerateLinkedDirectories(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	writeTestFile(t, filepath.Join(outside, "external.jar"), []byte("outside"))
	if err := scanTestDirectoryLink(outside, filepath.Join(dir, "mods")); err != nil {
		t.Fatal(err)
	}
	inputs, err := readOverviewInputs(context.Background(), dir)
	if err != nil || len(inputs.entries["mods"]) != 0 {
		t.Fatalf("overview enumerated an outside directory: %+v, %v", inputs.entries["mods"], err)
	}
}

func TestInstanceStorageRefreshSeesNestedChanges(t *testing.T) {
	s := &Service{}
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "saves", "world", "region", "a.mca"), []byte("old"))
	st := InstanceStatus{Dir: dir, RecentWorld: &World{Dir: "world"}, StoragePending: true}
	first, err := s.CompleteInstanceStorage(context.Background(), st)
	if err != nil || first.Total != 3 {
		t.Fatalf("initial size: %d, %v", first.Total, err)
	}
	writeTestFile(t, filepath.Join(dir, "saves", "world", "region", "b.mca"), []byte("newregion"))
	second, err := s.CompleteInstanceStorage(context.Background(), st)
	if err != nil || second.Total != 12 || second.Sizes["saves"] != 12 || second.RecentWorld.Size != 12 {
		t.Fatalf("refreshed size = %+v, want 12; err=%v", second, err)
	}
	if first.Total != 3 || first.RecentWorld.Size != 3 || st.RecentWorld.Size != 0 {
		t.Fatal("refresh mutated an already published summary")
	}
}

func TestInstanceStorageCacheExpiresDespiteRepeatedUse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := &Service{}
		dir := t.TempDir()
		file := filepath.Join(dir, "saves", "world", "region", "a.mca")
		writeTestFile(t, file, []byte("old"))
		st := InstanceStatus{Dir: dir, StoragePending: true}
		if _, err := s.CompleteInstanceStorage(context.Background(), st); err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, file, []byte("newregion"))
		time.Sleep(storageCacheTTL / 2)
		warm, err := s.CachedInstanceStorage(context.Background(), st)
		if err != nil || warm.Total != 3 {
			t.Fatalf("warm navigation did not reuse recent totals: %+v, %v", warm, err)
		}
		time.Sleep(storageCacheTTL / 2)
		fresh, err := s.CachedInstanceStorage(context.Background(), st)
		if err != nil || fresh.Total != 9 {
			t.Fatalf("repeated navigation kept expired totals: %+v, %v", fresh, err)
		}
	})
}

func TestInstanceStorageCacheDetectsVersionFileChanges(t *testing.T) {
	s := &Service{}
	dir := t.TempDir()
	file := filepath.Join(dir, filepath.Base(dir)+".jar")
	writeTestFile(t, file, []byte("old"))
	st := InstanceStatus{Dir: dir}
	if _, err := s.CompleteInstanceStorage(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, file, []byte("replacement"))
	fresh, err := s.CachedInstanceStorage(context.Background(), st)
	if err != nil || fresh.Total != 11 {
		t.Fatalf("changed version jar stayed cached: %+v, %v", fresh, err)
	}
}

func TestInstanceStorageCancellationAndLatePublication(t *testing.T) {
	s := &Service{}
	dir := t.TempDir()
	file := filepath.Join(dir, "mods", "a.jar")
	writeTestFile(t, file, []byte("old"))
	st := InstanceStatus{Dir: dir}
	old, err := s.CompleteInstanceStorage(context.Background(), st)
	if err != nil {
		t.Fatal(err)
	}
	entry := s.statusEntry(dir)
	oldScan := entry.storageScan
	writeTestFile(t, file, []byte("replacement"))
	if _, err = s.CompleteInstanceStorage(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	s.storeStorage(entry, oldScan, storageStamp(dir), old)
	current, err := s.CachedInstanceStorage(context.Background(), st)
	if err != nil || current.Total != 11 {
		t.Fatalf("late scan replaced newer data: %+v, %v", current, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.CachedInstanceStorage(ctx, st); !errors.Is(err, context.Canceled) {
		t.Fatalf("warm cache ignored cancellation: %v", err)
	}
	if _, err := s.CompleteInstanceStorage(ctx, st); !errors.Is(err, context.Canceled) {
		t.Fatalf("explicit refresh ignored cancellation: %v", err)
	}
	s.invalidateStatus(dir)
	s.storeStorage(entry, entry.storageScan, storageStamp(dir), current)
	if _, ok := s.cachedStorage(dir, st); ok {
		t.Fatal("late scan resurrected an invalidated cache")
	}
}

func TestWorldSizeCacheExpiresDespiteRepeatedUse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := &Service{}
		dir := t.TempDir()
		s.storeWorldSize(dir, 42)
		if _, ok := s.cachedWorldSize(dir, true); ok {
			t.Fatal("running game used a cached world size")
		}
		time.Sleep(storageCacheTTL / 2)
		if size, ok := s.cachedWorldSize(dir, false); !ok || size != 42 {
			t.Fatal("fresh world size was not reused")
		}
		time.Sleep(storageCacheTTL / 2)
		if _, ok := s.cachedWorldSize(dir, false); ok {
			t.Fatal("reading a world size extended its validity indefinitely")
		}
	})
}
