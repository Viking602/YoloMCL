package nativeui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestPackSwitchKeepsLatestSelection(t *testing.T) {
	game := t.TempDir()
	for _, id := range []string{"pack-a", "pack-b", "pack-c"} {
		dir := filepath.Join(game, "versions", id)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(`{"id":"`+id+`","inheritsFrom":"1.20.1"}`), 0644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := launcher.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.AddRoot(game, "packs")
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { a.cancel(); a.bgWG.Wait() }()
	tt := ui.NewTester(a.View, 1536, 1024)
	stale, epoch := s.Snapshot(), a.selectionEpoch.Load()
	for _, id := range []string{"pack-b", "pack-a", "pack-c", "pack-b"} {
		a.selectVersion(r.ID, id)
	}
	// A periodic snapshot can finish while a save is pending.
	a.applySnapshot(stale, epoch)
	if a.root().SelectedVersion != "pack-b" {
		t.Fatal("old snapshot undid the user's click")
	}
	waitUI(t, tt, func() bool { return !a.selectionPending })
	// It can also have been queued before the save and arrive after it.
	stale.Jobs = []launcher.Job{{ID: "fresh-job"}}
	a.applySnapshot(stale, epoch)
	if a.root().SelectedVersion != "pack-b" || len(a.snapshot.Jobs) != 1 {
		t.Fatal("late snapshot lost the current pack or fresh background state")
	}
	if snap := s.Snapshot(); snap.Instances[0].SelectedVersion != "pack-b" {
		t.Fatal("rapid pack clicks saved out of order")
	}
}

func TestPendingPackSelectionSurvivesSameEpochSnapshot(t *testing.T) {
	old := launcher.Snapshot{ActiveRoot: "r1", Instances: []launcher.Instance{
		{Root: launcher.Root{ID: "r1", SelectedVersion: "a"}},
		{Root: launcher.Root{ID: "r2", SelectedVersion: "b"}},
	}}
	a := &App{snapshot: old}
	a.snapshot.Instances = append([]launcher.Instance(nil), old.Instances...)
	a.applySelection("r2", "c")
	a.selectionPending = true
	epoch := a.selectionEpoch.Add(1)
	a.applySnapshot(old, epoch) // read before the background save reached disk
	if a.snapshot.ActiveRoot != "r2" || a.root().SelectedVersion != "c" {
		t.Fatal("snapshot read before persistence replaced the pending selection")
	}
}

func TestPackGridWindowCoversScrolledViewport(t *testing.T) {
	view := ui.Rect{Y: 200, H: 600}
	grid := ui.Rect{Y: -3800, H: 100 * 200}
	first, last := addonGridWindow(100, 200, grid, view, 1024)
	if first > 20 || last < 23 || last-first > 8 {
		t.Fatalf("scrolled pack window = %d..%d", first, last)
	}
	grid.Y = view.Y - 100*200 + view.H
	if _, last := addonGridWindow(100, 200, grid, view, 1024); last != 100 {
		t.Fatal("last card is unreachable")
	}
}

// Exercise the first layout too: a grid with fewer than 60 rows can still
// contain hundreds of cards. Building them also starts offscreen icon work.
func TestPackGridColdFrameIsBounded(t *testing.T) {
	for _, n := range []int{80, 199, 496} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			a := &App{}
			entries := make([]addonEntry, n)
			peak := 0
			tt := ui.NewTester(func(c *ui.Context) {
				drawn := 0
				scroll := ui.Scroll(c).Fill()
				a.tabViewBounds = scroll.LayoutBounds()
				scroll.Children(func() {
					a.addonGrid(c, "mod-grid", entries, func(addonEntry) {
						drawn++
						ui.Box(c).Grow(1).Basis(0).FillHeight()
					})
				})
				peak = max(peak, drawn)
			}, 1536, 1024)
			tt.Frame()
			if peak > 40 {
				t.Fatalf("cold pack grid built %d of %d cards for one screen", peak, n)
			}
		})
	}
}

// Render actual cards, including text layout, on a cold grid. Disk/network
// icons are disabled here so this measures UI work independently of scanning.
func BenchmarkPackGridCold(b *testing.B) {
	if err := loadAssets(); err != nil {
		b.Fatal(err)
	}
	a := &App{snapshot: launcher.Snapshot{ActiveRoot: "r", Instances: []launcher.Instance{{Root: launcher.Root{ID: "r", SelectedVersion: "pack"}}}},
		images: map[string]ui.ImageSource{}, imagePending: map[string]bool{}, imageRetry: map[string]time.Time{}}
	entries := make([]addonEntry, 199)
	for i := range entries {
		file := fmt.Sprintf("mod-%03d.jar", i)
		entries[i].it = addonItem{key: file, file: file, kind: "mod", name: file, enabled: true, version: "1.20.1", size: "12 MB", summary: "A mod in a large modpack, with a description for text layout."}
		a.imageRetry["addon:r:pack:mod:"+file] = time.Now()
	}
	b.ReportAllocs()
	for b.Loop() {
		a.addonGridW = nil
		ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme)
			scroll := ui.Scroll(c).Fill()
			a.tabViewBounds = scroll.LayoutBounds()
			scroll.Children(func() {
				a.addonGrid(c, "mod-grid", entries, func(e addonEntry) { a.addonCard(c, e.it, func() {}, nil) })
			})
		}, 1536, 1024)
	}
}
