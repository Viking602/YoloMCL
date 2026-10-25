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

func TestInstanceListRefreshesChangedValues(t *testing.T) {
	a := &App{}
	a.settings.Language = LangEN
	r := launcher.Instance{Root: launcher.Root{ID: "root"}, Versions: []launcher.VersionSummary{
		{ID: "pack", Minecraft: "1.21.1", Loader: "fabric", LoaderVersion: "0.16.14"},
		{ID: "1.21.1", Minecraft: "1.21.1", Loader: "vanilla"},
	}}
	rows, names := a.instanceListItems(&r)
	if len(rows) != 4 || names["1.21.1"].Title != "Vanilla 1.21.1" {
		t.Fatalf("unexpected initial list: %+v", names)
	}
	r.SelectedVersion = "1.21.1"
	selected, _ := a.instanceListItems(&r)
	if &selected[0] != &rows[0] {
		t.Fatal("changing only selection rebuilt the list")
	}
	r.Versions[0].DisplayName = "Renamed"
	_, names = a.instanceListItems(&r)
	if names["pack"].Title != "Renamed" {
		t.Fatal("in-place rename retained the old title")
	}
	a.settings.Language = LangZH
	rows, names = a.instanceListItems(&r)
	if rows[0].heading != "整合包" || names["1.21.1"].Title != "原版 1.21.1" {
		t.Fatal("language change retained old labels")
	}
	r.Versions[0].DisplayName = ""
	r.Pack = &launcher.PackLock{Name: "Imported pack", Version: "2", GameVersion: "pack", LoaderVersion: "0.17"}
	_, names = a.instanceListItems(&r)
	if names["pack"].Title != "Imported pack" || !strings.Contains(names["pack"].Sub, "v2") {
		t.Fatal("pack metadata did not refresh")
	}
	r.Pack.Version = "3"
	_, names = a.instanceListItems(&r)
	if !strings.Contains(names["pack"].Sub, "v3") {
		t.Fatal("in-place pack update retained an old version")
	}
	a.instFilter = "Imported"
	rows, _ = a.instanceListItems(&r)
	if len(rows) != 2 || rows[1].version.ID != "pack" {
		t.Fatal("name filter did not refresh")
	}
	a.instFilter, a.instLoader = "", "vanilla"
	rows, _ = a.instanceListItems(&r)
	if len(rows) != 2 || rows[1].version.ID != "1.21.1" {
		t.Fatal("loader filter did not refresh")
	}
	a.instLoader = ""
	r.ID = "other-root"
	r.Versions = r.Versions[:1]
	rows, _ = a.instanceListItems(&r)
	if len(rows) != 2 || rows[1].key != "other-root/version/pack" {
		t.Fatal("directory switch retained old rows")
	}
	rows, names = a.instanceListItems(nil)
	if len(rows) != 0 || len(names) != 0 || a.instListData.versions != nil {
		t.Fatal("removing the directory retained its list")
	}
}

// Includes layout, software painting and selection animation, with the same
// visible rows and a growing inventory. Icons are already decoded.
func BenchmarkInstanceListFrames(b *testing.B) {
	if err := loadAssets(); err != nil {
		b.Fatal(err)
	}
	icon, err := ui.DecodeBitmap(BrandIconPNG())
	if err != nil {
		b.Fatal(err)
	}
	for _, n := range []int{60, 500} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			a := &App{images: map[string]ui.ImageSource{}}
			r := launcher.Instance{Root: launcher.Root{ID: "bench"}, Versions: make([]launcher.VersionSummary, n)}
			for i := range r.Versions {
				id := fmt.Sprintf("Pack %03d", i)
				r.Versions[i] = launcher.VersionSummary{ID: id, Loader: "neoforge", LoaderVersion: "21.1.250", Minecraft: "1.21.1"}
				a.images["version:bench:"+id] = icon
			}
			frame := 0
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme)
				c.AnimationFrame()
				ui.Column(c).Fill().Children(func() { a.instanceVirtualList(c, &r) })
			}, 340, 800)
			tt.Advance(time.Second)
			b.ReportAllocs()
			for b.Loop() {
				frame++
				r.SelectedVersion = r.Versions[(frame/15)%10].ID
				tt.Advance(time.Second / 60)
			}
		})
	}
}
