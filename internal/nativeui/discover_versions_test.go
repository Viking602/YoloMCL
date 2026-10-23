package nativeui

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

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
