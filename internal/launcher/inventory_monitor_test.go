package launcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type testInventoryWatch struct {
	notified bool
	err      error
	closed   int
}

func (w *testInventoryWatch) changed() (bool, error) {
	changed := w.notified
	w.notified = false
	return changed, w.err
}

func (w *testInventoryWatch) close() { w.closed++ }

func ageInventoryCheck(s *Service, r Root, age time.Duration) time.Time {
	entry := s.inventoryEntry(r.Path)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.checked = time.Now().Add(-age)
	return entry.checked
}

func TestInventoryMonitorKeepsNotificationsUntilValidation(t *testing.T) {
	s := testService(t)
	r := inventoryFixture(t, s)
	w := &testInventoryWatch{}
	m := s.NewInventoryMonitor()
	t.Cleanup(m.Close)
	m.open = func(string) inventoryChangeWatch { return w }
	ageInventoryCheck(s, r, inventoryInterval)
	m.Refresh(context.Background())
	entry := s.inventoryEntry(r.Path)
	checked := ageInventoryCheck(s, r, 10*time.Second)
	m.Refresh(context.Background())
	if !entry.checked.Equal(checked) {
		t.Fatal("unchanged directory was probed again")
	}
	writeTestFile(t, filepath.Join(r.Path, "versions", "pack", "mods", "two.jar"), []byte("two"))
	ageInventoryCheck(s, r, 0)
	w.notified = true
	m.Refresh(context.Background()) // notification arrives inside scan coalescing window
	if entry.data.modCount("pack").Total != 1 {
		t.Fatal("early notification bypassed scan coalescing")
	}
	ageInventoryCheck(s, r, inventoryInterval)
	m.Refresh(context.Background()) // the OS no longer has a pending event
	if entry.data.modCount("pack").Total != 2 {
		t.Fatal("notification was lost before validation could run")
	}
	writeTestFile(t, filepath.Join(r.Path, "versions", "pack", "mods", "three.jar"), []byte("three"))
	ageInventoryCheck(s, r, inventoryVerifyInterval)
	m.Refresh(context.Background())
	if entry.data.modCount("pack").Total != 3 {
		t.Fatal("periodic verification missed a dropped notification")
	}
}

func TestInventoryMonitorValidatesPersistedCacheAndExplicitChanges(t *testing.T) {
	s := testService(t)
	r := inventoryFixture(t, s)
	_ = s.Snapshot()
	writeTestFile(t, filepath.Join(r.Path, "versions", "pack", "mods", "two.jar"), []byte("two"))
	loaded, err := New(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	m := loaded.NewInventoryMonitor()
	t.Cleanup(m.Close)
	m.open = func(string) inventoryChangeWatch { return &testInventoryWatch{} }
	m.Refresh(context.Background())
	if got := loaded.Snapshot().Instances[0].ModCount; got != 2 {
		t.Fatalf("persisted startup cache not validated: %d", got)
	}
	if err := loaded.ToggleMod(r.ID, "two.jar"); err != nil {
		t.Fatal(err)
	}
	m.Refresh(context.Background())
	if got := loaded.inventoryEntry(r.Path).data.modCount("pack"); got.Total != 2 || got.Enabled != 1 {
		t.Fatalf("explicit mutation waited for a filesystem event: %+v", got)
	}
}

func TestInventoryMonitorFallbackAndLifecycle(t *testing.T) {
	s := testService(t)
	r := inventoryFixture(t, s)
	first, replacement := &testInventoryWatch{}, &testInventoryWatch{}
	m := s.NewInventoryMonitor()
	t.Cleanup(m.Close)
	opened := 0
	m.open = func(string) inventoryChangeWatch {
		opened++
		if opened == 1 {
			return first
		}
		return replacement
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.Refresh(ctx)
	if opened != 0 {
		t.Fatal("cancelled maintenance opened a watcher")
	}
	m.Refresh(context.Background())
	first.err = os.ErrNotExist
	m.Refresh(context.Background())
	if first.closed != 1 || opened != 2 {
		t.Fatal("broken notification handle was not closed and replaced")
	}
	s.mu.Lock()
	s.cfg.Roots = nil
	s.mu.Unlock()
	m.Refresh(context.Background())
	m.Close()
	if replacement.closed != 1 || len(m.watches) != 0 {
		t.Fatal("removed root retained a watcher")
	}
	s.mu.Lock()
	s.cfg.Roots = []Root{r}
	s.mu.Unlock()
	m.open = func(string) inventoryChangeWatch { return nil }
	writeTestFile(t, filepath.Join(r.Path, "versions", "pack", "mods", "two.jar"), []byte("two"))
	ageInventoryCheck(s, r, inventoryInterval)
	m.Refresh(context.Background())
	if got := s.Snapshot().Instances[0].ModCount; got != 2 {
		t.Fatalf("unsupported notification volume stopped refreshing: %d", got)
	}
}

func TestInventoryCheckReportsItsOwnValidation(t *testing.T) {
	s := testService(t)
	r := inventoryFixture(t, s)
	entry := s.inventoryEntry(r.Path)
	ageInventoryCheck(s, r, inventoryInterval)
	start, results := make(chan struct{}), make(chan bool, 2)
	for range 2 {
		go func() {
			<-start
			_, validated := s.checkInventory(r, entry)
			results <- validated
		}()
	}
	close(start)
	validated := 0
	for range 2 {
		if <-results {
			validated++
		}
	}
	if validated != 1 {
		t.Fatalf("coalesced check reported another caller's validation: %d", validated)
	}
}
