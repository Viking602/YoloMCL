package nativeui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestLargeInstanceListBuildsOnlyVisibleRows(t *testing.T) {
	a, _ := homeTester(t, "zh-CN")
	a.bgWG.Wait()
	r := a.root()
	r.Versions = make([]launcher.VersionSummary, 1000)
	for i := range r.Versions {
		r.Versions[i] = launcher.VersionSummary{ID: fmt.Sprintf("virtual-pack-%04d", i), Minecraft: "1.21.1", Loader: "neoforge", LoaderVersion: "21.1.250"}
	}
	r.SelectedVersion = r.Versions[500].ID
	a.instScrollPending = true
	// Exercise the real list and its asynchronous icon requests, without the
	// overview's unrelated filesystem and network work.
	tt := ui.NewTester(func(c *ui.Context) { a.instanceList(c) }, 340, 800)
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	selected, ok := tt.Find("版本卡片 " + r.SelectedVersion)
	picker, _ := tt.Find("目录选择栏")
	if !ok || selected.Y < 0 || selected.Y+selected.H > picker.Y {
		t.Fatalf("selected pack was not in the first viewport: %+v", selected)
	}
	requested := 0
	a.imageMu.Lock()
	for key := range a.imagePending {
		if strings.Contains(key, "virtual-pack-") {
			requested++
		}
	}
	for key := range a.imageRetry {
		if strings.Contains(key, "virtual-pack-") {
			requested++
		}
	}
	a.imageMu.Unlock()
	if requested == 0 || requested > 35 {
		t.Fatalf("first viewport requested %d icons for 1000 packs", requested)
	}
	a.instRows.ScrollToEnd()
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	last, ok := tt.Find("版本卡片 " + r.Versions[999].ID)
	if !ok || last.Y < 0 || last.Y+last.H > picker.Y+.5 {
		t.Fatalf("last pack is missing or clipped after virtual scrolling: %+v", last)
	}
	a.instFilter = "virtual-pack-0001"
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	if _, ok := tt.Find("版本卡片 virtual-pack-0001"); !ok {
		t.Fatal("filtering at the end left the list empty")
	}
}

func TestInstanceNamesKeepDuplicateTitlesDistinct(t *testing.T) {
	a := &App{}
	r := launcher.Instance{Versions: []launcher.VersionSummary{
		{ID: "one", DisplayName: "Shared name", Minecraft: "1.21.1", Loader: "fabric", LoaderVersion: "0.16.14"},
		{ID: "two", DisplayName: "Shared name", Minecraft: "1.21.1", Loader: "fabric", LoaderVersion: "0.16.14"},
		{ID: "other-mc", DisplayName: "Shared name", Minecraft: "1.20.1", Loader: "fabric", LoaderVersion: "0.16.14"},
	}}
	names := a.instanceNames(r)
	if !strings.Contains(names["one"].Sub, "one") || !strings.Contains(names["two"].Sub, "two") || strings.Contains(names["other-mc"].Sub, "other-mc") {
		t.Fatal("duplicate names did not use folder IDs only where necessary")
	}
	r.Versions[1].DisplayName = "Renamed"
	if after := a.instanceNames(r); strings.Contains(after["one"].Sub, "one") || after["two"].Title != "Renamed" {
		t.Fatal("renaming retained stale disambiguation")
	}
}
