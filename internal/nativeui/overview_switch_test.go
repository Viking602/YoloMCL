package nativeui

import (
	"bytes"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestOverviewRefreshAndModStreamingDoNotRepaintSummary(t *testing.T) {
	a, old := homeTester(t, LangZH)
	a.bgWG.Wait()
	old.Frame()
	a.service = nil // isolate rendering from the fixture's nonexistent root
	r := *a.root()
	a.instStatusKey = r.ID + "|" + r.SelectedVersion
	a.instStatus = launcher.InstanceStatus{Dir: r.Path, Mods: 312, EnabledMods: 310, Saves: 3, ResourcePacks: 10, Shaders: 5, Total: 1000, Sizes: map[string]int64{"mods": 800}}
	a.statusCache = map[string]launcher.InstanceStatus{a.instStatusKey: a.instStatus}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Column(c).Fill().Children(func() { a.instanceOverview(c, *a.root()) })
	}, 1536, 1024)
	tt.Advance(motionNumber.Duration + 50*time.Millisecond)
	before := append([]byte(nil), tt.Image().Pix...)
	for _, n := range []int{0, 30, 90, 199} {
		a.modsLoading = true
		a.mods = make([]launcher.Mod, n)
		a.loadInstanceStatus()
		tt.Frame()
		if !bytes.Equal(before, tt.Image().Pix) {
			t.Fatalf("background progress repainted overview for %d partial mods", n)
		}
		if !tt.HasText("312") || !tt.HasText("310 个已启用") {
			t.Fatal("overview used a partial mod list")
		}
	}
	for _, tab := range a.instTabs(r) {
		if tab.key == "mods" && tab.text != "模组" {
			t.Fatal("tab still shows a changing mod count")
		}
	}
}

func TestOverviewColdAndWarmSwitchKeepPanelGeometry(t *testing.T) {
	a, old := homeTester(t, LangZH)
	a.bgWG.Wait()
	old.Frame()
	a.service = nil
	r := *a.root()
	a.instStatusKey = r.ID + "|" + r.SelectedVersion
	a.instStatus = launcher.InstanceStatus{}
	a.instStatusErr = ""
	a.overviewDisplay = overviewPresentation{}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Column(c).Fill().Children(func() { a.instanceOverview(c, *a.root()) })
	}, 1536, 1024)
	tt.Advance(300 * time.Millisecond)
	panels := map[string]ui.Rect{}
	for _, name := range []string{"最近世界", "最近服务器", "游戏截图", "运行环境", "存储空间"} {
		panels[name], _ = tt.Find(name)
	}
	if !tt.HasText("概览加载占位") || tt.HasText("下一个世界，等你创造") || tt.HasText("暂无连接记录") {
		t.Fatal("cold overview passed through false empty states")
	}
	st := launcher.InstanceStatus{Dir: r.Path, Mods: 93, EnabledMods: 91, Screenshots: 4, Total: 1000, Sizes: map[string]int64{"screenshots": 100}}
	a.instStatus = st
	a.statusCache = map[string]launcher.InstanceStatus{a.instStatusKey: st}
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	for name, want := range panels {
		if got, _ := tt.Find(name); got != want {
			t.Fatalf("summary arrival moved %s: %+v -> %+v", name, want, got)
		}
	}
	// The target already has a summary: no loading frame on a warm switch.
	other := "1.21.8"
	a.statusCache[r.ID+"|"+other] = st
	a.applySelection(r.ID, other)
	a.loadInstanceStatus()
	tt.Frame()
	if tt.HasText("概览加载占位") || !tt.HasText("91 个已启用") {
		t.Fatal("warm switch cleared cached data")
	}
	snapshotUI(t, tt, "overview-warm-switch")
}
