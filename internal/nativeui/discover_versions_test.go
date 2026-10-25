package nativeui

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func BenchmarkDiscoverVersionFilterClosed(b *testing.B) {
	if err := loadAssets(); err != nil {
		b.Fatal(err)
	}
	a := &App{}
	for i := 0; i < 900; i++ {
		a.minecraftVersions = append(a.minecraftVersions, launcher.MinecraftVersion{ID: fmt.Sprintf("1.%d.1", i), Type: "release"})
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		c.AnimationFrame()
		ui.Column(c).Fill().Children(func() { a.discoverVersionFilter(c) })
	}, 340, 800)
	tt.Advance(time.Second)
	b.ReportAllocs()
	for b.Loop() {
		tt.Advance(time.Second / 60)
	}
}

func TestDiscoverVersionDropdownCommitsAndClearsFilter(t *testing.T) {
	for _, source := range []string{"modrinth", "curseforge"} {
		t.Run(source, func(t *testing.T) {
			a, tt, requests := discoverFeedTester(t, source, "modpack")
			a.minecraftVersions = []launcher.MinecraftVersion{{ID: "1.21.1", Type: "release"}, {ID: "1.20.1", Type: "release"}}
			first := nextFeedTestRequest(t, tt, requests)
			replyFeedTest(t, a, tt, first, 0, 18, 100)
			clickUI(t, tt, a.tr().DiscoverFilterMCAria)
			if !tt.HasText("1.21.1") || !tt.HasText("1.20.1") {
				t.Fatal("click did not open version choices")
			}
			snapshotUI(t, tt, "discover-versions-"+source)
			tt.Type("1.20")
			if a.filterMC != "" || len(requests) != 0 {
				t.Fatal("typing an unfinished version submitted a search")
			}
			tt.Key(0, ui.KeyEnter)
			filtered := nextFeedTestRequest(t, tt, requests)
			if filtered.query.Minecraft != "1.20.1" || filtered.query.Offset != 0 {
				t.Fatalf("wrong version filter: %+v", filtered.query)
			}
			replyFeedTest(t, a, tt, filtered, 50, 4, 4)
			clickUI(t, tt, a.tr().DiscoverFilterMCAria)
			clickUI(t, tt, "全部游戏版本")
			cleared := nextFeedTestRequest(t, tt, requests)
			if cleared.query.Minecraft != "" || cleared.query.Offset != 0 {
				t.Fatalf("all versions did not clear the filter: %+v", cleared.query)
			}
		})
	}
}

func TestDiscoverSearchInputButtonAndVersionClearButton(t *testing.T) {
	a, tt, requests := discoverFeedTester(t, "curseforge", "modpack")
	a.minecraftVersions = []launcher.MinecraftVersion{{ID: "26.2", Type: "release"}, {ID: "1.21.1", Type: "release"}}
	first := nextFeedTestRequest(t, tt, requests)
	replyFeedTest(t, a, tt, first, 0, 15, 15)

	// Select 26.2 via the dropdown arrow, then verify the inline clear button resets filterMC.
	clickUI(t, tt, "展开版本列表")
	if !tt.HasText("26.2") || !tt.HasText("1.21.1") {
		t.Fatal("chevron click did not open version choices")
	}
	clickUI(t, tt, "26.2")
	byVer := nextFeedTestRequest(t, tt, requests)
	if byVer.query.Minecraft != "26.2" {
		t.Fatalf("expected Minecraft filter 26.2, got %+v", byVer.query)
	}
	replyFeedTest(t, a, tt, byVer, 0, 15, 15)

	clickUI(t, tt, "清除版本筛选")
	clearedVer := nextFeedTestRequest(t, tt, requests)
	if clearedVer.query.Minecraft != "" {
		t.Fatalf("inline version clear button did not clear filterMC: %+v", clearedVer.query)
	}
	replyFeedTest(t, a, tt, clearedVer, 0, 18, 100)

	// Type in search box and submit via Enter, then re-search while a request is in flight, then clear query.
	typeUI(t, tt, a.tr().DiscoverSearchAria, "all the mods 11")
	tt.Key(0, ui.KeyEnter)
	searched := nextFeedTestRequest(t, tt, requests)
	if searched.query.Query != "all the mods 11" {
		t.Fatalf("Enter in search input did not submit query: %+v", searched.query)
	}
	// While searched is still in flight (a.searchLoading == true), clicking Search must still work and cancel the old request.
	clickUI(t, tt, a.tr().DiscoverSearchBtn)
	reSearched := nextFeedTestRequest(t, tt, requests)
	if searched.ctx.Err() == nil || reSearched.query.Query != "all the mods 11" {
		t.Fatalf("clicking search button during loading did not trigger replacement search: %+v", reSearched.query)
	}
	replyFeedTest(t, a, tt, reSearched, 0, 5, 5)

	clickUI(t, tt, "清空搜索")
	clearedQuery := nextFeedTestRequest(t, tt, requests)
	if clearedQuery.query.Query != "" {
		t.Fatalf("clear search button did not reset query: %+v", clearedQuery.query)
	}
	replyFeedTest(t, a, tt, clearedQuery, 0, 18, 100)
}

func TestDiscoverVersionChoicesWorkOfflineAndKeepCurrentSnapshot(t *testing.T) {
	a := &App{filterMC: "24w14a"}
	a.settings.Language = LangEN
	a.minecraftVersions = []launcher.MinecraftVersion{{ID: "1.21.1", Type: "release"}, {ID: "24w14a", Type: "snapshot"}, {ID: "24w13a", Type: "snapshot"}}
	a.snapshot.Instances = []launcher.Instance{{Root: launcher.Root{ID: "offline"}, Versions: []launcher.VersionSummary{{Minecraft: "1.20.1"}, {Minecraft: "1.21.1"}}}}
	options := a.discoverVersionOptions()
	if !slices.Equal(options, []string{"All game versions", "24w14a", "1.21.1", "1.20.1"}) {
		t.Fatalf("unexpected version choices: %v", options)
	}
	a.minecraftVersions = nil
	if options = a.discoverVersionOptions(); !slices.Contains(options, "1.20.1") || !slices.Contains(options, a.filterMC) {
		t.Fatalf("offline list lost installed versions: %v", options)
	}
}
