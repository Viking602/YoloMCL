package nativeui

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestOverviewRefreshButtonRecalculatesNestedStorage(t *testing.T) {
	game := t.TempDir()
	dir := filepath.Join(game, "versions", "pack")
	write := func(name, data string) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("pack.json", `{"id":"pack","inheritsFrom":"1.21.1"}`)
	write("saves/world/region/a.mca", "old")
	s, err := launcher.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddRoot(game, "pack"); err != nil {
		t.Fatal(err)
	}
	a, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { a.cancel(); a.bgWG.Wait() }()
	tt := ui.NewTester(a.View, 1536, 1024)
	waitUI(t, tt, func() bool { return !a.statusLoading && a.instStatus.Sizes["saves"] == 3 })
	tt.Advance(400 * time.Millisecond)
	write("saves/world/region/b.mca", "newregion")
	clickUI(t, tt, a.L("overview.refresh_overview"))
	waitUI(t, tt, func() bool { return !a.statusLoading && a.instStatus.Sizes["saves"] == 12 })
	if a.instStatus.RecentWorld == nil || a.instStatus.RecentWorld.Size != 12 {
		t.Fatal("manual refresh did not update the recent world's size")
	}
}

func TestStorageBarFillsWithRealCapacityShares(t *testing.T) {
	a, _ := homeTester(t, "zh-CN")
	for _, sizes := range []map[string]int64{
		{"mods": 400, "resourcepacks": 200, "shaderpacks": 100, "saves": 100, "screenshots": 100},
		{},
	} {
		st := launcher.InstanceStatus{Dir: a.root().Path, Total: 1000, Sizes: sizes}
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Fill().Padding(20).Children(func() { a.ovDir(c, st, *a.root(), false) })
		}, 800, 540)
		bar, _ := tt.Find("容量占比")
		width := float32(0)
		for _, part := range dirParts {
			n := storageBytes(st, part.sub)
			if n == 0 {
				continue
			}
			segment, ok := tt.Find("存储占比 " + part.sub)
			if !ok || math.Abs(float64(segment.W/bar.W)-float64(n)/float64(st.Total)) > .001 {
				t.Fatalf("wrong %s capacity share: %+v / %+v", part.sub, segment, bar)
			}
			width += segment.W
		}
		if math.Abs(float64(width-bar.W)) > .1 || !tt.HasText("其他") {
			t.Fatal("storage bar left an unexplained empty remainder")
		}
	}
}

func TestStorageShareChangeAnimatesWithoutClearingBar(t *testing.T) {
	a, old := homeTester(t, LangZH)
	a.bgWG.Wait()
	old.Frame()
	a.contentSwaps = nil
	st := launcher.InstanceStatus{Dir: a.root().Path, Total: 1000, Sizes: map[string]int64{"mods": 400, "resourcepacks": 200}}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(20).Children(func() { a.ovDir(c, st, *a.root(), false) })
	}, 800, 540)
	tt.Advance(400 * time.Millisecond)
	bar, _ := tt.Find("容量占比")
	before, _ := tt.Find("存储占比 mods")
	st.Sizes = map[string]int64{"mods": 100, "resourcepacks": 500}
	tt.Frame()
	first, _ := tt.Find("存储占比 mods")
	if first != before {
		t.Fatal("storage distribution jumped at the start of a switch")
	}
	tt.Advance(100 * time.Millisecond)
	middle, _ := tt.Find("存储占比 mods")
	width := float32(0)
	for _, part := range dirParts {
		segment, _ := tt.Find("存储占比 " + part.sub)
		width += segment.W
	}
	if math.Abs(float64(width-bar.W)) > .1 {
		t.Fatal("storage animation left an empty remainder")
	}
	tt.Advance(300 * time.Millisecond)
	end, _ := tt.Find("存储占比 mods")
	if !(before.W > middle.W && middle.W > end.W) || math.Abs(float64(end.W/bar.W)-.1) > .001 {
		t.Fatalf("missing storage share transition: %.2f -> %.2f -> %.2f", before.W, middle.W, end.W)
	}
}

func TestOverviewRecentServerCopiesOnlyItsAddress(t *testing.T) {
	a, tt := homeTester(t, "zh-CN")
	a.statusGen++
	a.instStatusKey = a.root().ID + "|" + a.root().SelectedVersion
	for _, pair := range [][2]string{
		{"friends.example.test:25565", "friends.example.test"},
		{"friends.example.test:25566", "friends.example.test:25566"},
		{"[2001:db8::1]:25565", "[2001:db8::1]"},
	} {
		a.recentPing.cache = map[string]serverProbe{pair[0]: {checked: time.Now()}}
		a.instStatus = launcher.InstanceStatus{Dir: a.root().Path, RecentServer: &launcher.InstanceServer{Address: pair[0], Confirmed: true, Connected: time.Now()}}
		tt.Frame()
		tt.Advance(400 * time.Millisecond)
		if !tt.HasText("最近服务器") || !tt.HasText(pair[1]) {
			t.Fatal("missing or incorrect recent multiplayer address")
		}
		clickUI(t, tt, "复制地址")
		if tt.Clipboard() != pair[1] {
			t.Fatal("server action copied the wrong value")
		}
	}
}
