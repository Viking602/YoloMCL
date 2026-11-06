package nativeui

import (
	"bytes"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestGameSettingsUnderlineTabsFollowClicksKeysAndResize(t *testing.T) {
	a, tt, _ := worldsTester(t)
	a.openKeyBindings("", "")
	waitUI(t, tt, func() bool { return a.keyBindings.view != nil && !a.dialogPending() })
	g := a.keyBindings
	g.backupsLoaded = true
	tt.Advance(400 * time.Millisecond)
	start := addonFilterBounds(t, tt, a.L("game_options.tab_indicator"))
	g.query, g.options["maxFps"] = "maxFps", "180"
	clickUI(t, tt, a.L("game_options.tab.controls"))
	tt.Advance(65 * time.Millisecond)
	mid := addonFilterBounds(t, tt, a.L("game_options.tab_indicator"))
	controls := addonFilterBounds(t, tt, a.L("game_options.tab.controls"))
	if mid.X <= start.X || mid.X >= controls.X || g.tab != "controls" || g.query != "" {
		t.Fatal("selection, filter reset and moving indicator disagree", start, mid, controls)
	}
	clickUI(t, tt, a.L("game_options.tab.audio"))
	clickUI(t, tt, a.L("game_options.tab.keys"))
	tt.Advance(400 * time.Millisecond)
	if got := addonFilterBounds(t, tt, a.L("game_options.tab_indicator")); got != start || g.options["maxFps"] != "180" {
		t.Fatal("rapid tab changes lost their destination or draft", got)
	}
	tt.Key(0, ui.KeyRight)
	if g.tab != "video" {
		t.Fatal("arrow key did not select the next game settings tab", g.tab)
	}
	tt.Key(0, ui.KeyEnd)
	if g.tab != "backups" || !tt.HasText(a.L("game_options.backup.empty")) {
		t.Fatal("End did not select and render backups", g.tab)
	}
	tt.Key(0, ui.KeyHome)
	if g.tab != "keys" {
		t.Fatal("Home did not select the first tab", g.tab)
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	for _, lang := range []string{LangZH, LangEN} {
		a.OverrideLanguage(lang)
		tt.Frame()
		clickUI(t, tt, a.L("game_options.tab.controls"))
		for _, size := range [][2]int{{900, 540}, {1536, 1024}} {
			tt.SetSize(size[0], size[1])
			line := addonFilterBounds(t, tt, a.L("game_options.tab_indicator"))
			tab := addonFilterBounds(t, tt, a.L("game_options.tab.controls"))
			if line.W <= 0 || line.X < tab.X || line.X+line.W > tab.X+tab.W || math.Abs(float64(line.X+line.W/2-tab.X-tab.W/2)) > .1 {
				t.Fatal("indicator lost its tab after resize", line, tab)
			}
		}
	}
}

func TestSkeletonStopsFramesWhenHiddenOrMotionReduced(t *testing.T) {
	loading := true
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(20).Children(func() {
			if loading {
				skeleton(c, "load").Label("placeholder").Height(50)
			} else {
				ui.Text(c, "Loaded")
			}
		})
	}, 500, 160)
	region := addonFilterBounds(t, tt, "placeholder")
	before := overviewRegionPixels(tt.Image(), region)
	tt.Advance(450 * time.Millisecond)
	if !tt.FramePending() || bytes.Equal(before, overviewRegionPixels(tt.Image(), region)) {
		t.Fatal("visible skeleton does not shimmer")
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	before = overviewRegionPixels(tt.Image(), region)
	tt.Advance(500 * time.Millisecond)
	if tt.FramePending() || !bytes.Equal(before, overviewRegionPixels(tt.Image(), region)) {
		t.Fatal("reduced motion still animates the skeleton")
	}
	tt.SetPreferences(ui.Preferences{})
	loading = false
	tt.Frame()
	if tt.FramePending() || tt.HasText("placeholder") || !tt.HasText("Loaded") {
		t.Fatal("unmounted skeleton left rendering work behind")
	}
}

func TestGameSettingsSkeletonLoadingErrorReloadAndBackups(t *testing.T) {
	a, tt, _ := worldsTester(t)
	a.openKeyBindings("", "")
	waitUI(t, tt, func() bool { return a.keyBindings.view != nil && !a.dialogPending() })
	a.bgWG.Wait()
	tt.Frame()
	g := a.keyBindings
	view := g.view
	setPending := func(method string, on bool) {
		if on {
			a.pending[method] = 1
			a.pendingRequests["placeholder-test"] = pendingRequest{method: method, dialog: a.dialog, dialogGen: a.dialogGen}
		} else {
			delete(a.pending, method)
			delete(a.pendingRequests, "placeholder-test")
		}
	}
	for _, lang := range []string{LangZH, LangEN} {
		a.OverrideLanguage(lang)
		for _, pack := range []bool{false, true} {
			g.rootID = ""
			if pack {
				g.rootID = a.root().ID
			}
			g.view = nil
			setPending("keyBindings", true)
			for _, size := range [][2]int{{900, 540}, {1536, 1024}} {
				tt.SetSize(size[0], size[1])
				tt.Advance(400 * time.Millisecond)
				placeholder := addonFilterBounds(t, tt, "游戏设置加载占位")
				footer := addonFilterBounds(t, tt, "按钮文字 keybindings-reload")
				if placeholder.Y+placeholder.H > footer.Y || footer.Y+footer.H > float32(size[1]) || !tt.HasText(a.L("keybindings.loading")) {
					t.Fatal("loading placeholder covers the footer or lost its status", placeholder, footer)
				}
				snapshotUI(t, tt, fmt.Sprintf("settings-skeleton-%s-%t-%d", lang, pack, size[0]))
			}
			setPending("keyBindings", false)
			tt.Frame()
			if tt.HasText("游戏设置加载占位") || !tt.HasText("按钮文字 keybindings-retry") {
				t.Fatal("failed read retained its loading animation instead of retry")
			}
		}
	}
	g.rootID, g.view = "", view
	g.options["maxFps"] = "180"
	setPending("keyBindings", true)
	tt.Frame()
	if tt.HasText("游戏设置加载占位") || !tt.HasText(a.L("keybindings.search")) || g.options["maxFps"] != "180" {
		t.Fatal("refresh hid loaded settings or cleared a draft")
	}
	setPending("keyBindings", false)
	g.tab, g.backupsLoaded = "backups", false
	setPending("gameSettingsBackups", true)
	tt.Frame()
	if !tt.HasText("备份加载占位") {
		t.Fatal("backup read has no placeholder")
	}
	setPending("gameSettingsBackups", false)
	g.backupsLoaded = true
	tt.Frame()
	if tt.HasText("备份加载占位") || !tt.HasText(a.L("game_options.backup.empty")) {
		t.Fatal("completed backup read retained its placeholder")
	}
}

func TestStorageBarHoverNavigationAndLoading(t *testing.T) {
	a, old := homeTester(t, LangZH)
	a.bgWG.Wait()
	old.Frame()
	a.service = nil
	a.contentSwaps = nil
	st := launcher.InstanceStatus{Dir: a.root().Path, Total: 1000, Sizes: map[string]int64{
		"mods": 400, "resourcepacks": 200, "shaderpacks": 100, "saves": 100, "screenshots": 100,
	}}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(20).Children(func() { a.ovDir(c, st, *a.root(), false) })
	}, 800, 540)
	tt.Advance(400 * time.Millisecond)
	mods := addonFilterBounds(t, tt, "存储占比 mods")
	resources := addonFilterBounds(t, tt, "存储占比 resourcepacks")
	normal := tt.Image().At(int(resources.X+resources.W/2), int(resources.Y+resources.H/2))
	tt.Move(mods.X+mods.W/2, mods.Y+mods.H/2)
	tt.Advance(time.Second)
	if !tt.HasText(a.L("storage.label.mods")+"  "+fmtSize(400)+"  40%") || normal == tt.Image().At(int(resources.X+resources.W/2), int(resources.Y+resources.H/2)) {
		t.Fatal("segment hover lost its size/percentage or category highlight")
	}
	snapshotUI(t, tt, "storage-segment-tooltip")
	tt.Move(0, 0)
	tt.Advance(400 * time.Millisecond)
	row := addonFilterBounds(t, tt, a.L("storage.label.mods"))
	tt.Move(row.X+row.W/2, row.Y+row.H/2)
	if normal == tt.Image().At(int(resources.X+resources.W/2), int(resources.Y+resources.H/2)) {
		t.Fatal("legend hover did not highlight the corresponding bar segment")
	}
	a.overviewHolding = true
	clickUI(t, tt, "存储占比 mods")
	if a.instTab != "overview" {
		t.Fatal("outgoing instance storage allowed navigation")
	}
	a.overviewHolding = false
	clickUI(t, tt, "存储占比 mods")
	if a.instTab != "mods" {
		t.Fatal("storage segment did not open its category")
	}
	st.StoragePending = true
	tt.Frame()
	if tt.HasText(a.L("overview.calculating")) {
		t.Fatal("refresh discarded the previous storage distribution")
	}
	st.Total, st.Sizes = 0, nil
	tt.Frame()
	if !tt.HasText(a.L("overview.calculating")) || tt.HasText("存储占比 mods") {
		t.Fatal("cold storage scan was presented as measured capacity")
	}
	st.StoragePending = false
	tt.Frame()
	if tt.HasText(a.L("overview.calculating")) || tt.HasText("存储占比 other") {
		t.Fatal("empty storage shows an endless scan or a fabricated segment")
	}
}
