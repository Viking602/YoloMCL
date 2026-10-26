package nativeui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestOverviewFitsViewportWithoutScrolling(t *testing.T) {
	for _, lang := range []string{"zh-CN", "en-US"} {
		for _, size := range [][2]int{{1536, 1024}, {1100, 760}, {900, 540}} {
			t.Run(fmt.Sprintf("%s-%dx%d", lang, size[0], size[1]), func(t *testing.T) {
				a, tt := homeTester(t, lang)
				a.bgWG.Wait()
				tt.Frame()
				a.message, a.errorMessage, a.noticeShown = "", false, ""
				r := a.root()
				a.statusGen++
				if a.statusCancel != nil {
					a.statusCancel()
				}
				a.instStatusKey = r.ID + "|" + r.SelectedVersion
				a.instStatus = launcher.InstanceStatus{
					Dir: r.Path, Saves: 4, ResourcePacks: 7, Shaders: 5,
					Total: 350 << 20, Sizes: map[string]int64{"mods": 250 << 20, "saves": 100 << 20},
					RecentWorld:  &launcher.World{Dir: "world", Name: strings.Repeat("Long world name ", 12), Mode: "creative", Size: 100 << 20, LastPlayed: time.Now(), Backups: 2},
					RecentServer: &launcher.InstanceServer{Address: "play.example.test:25565", Confirmed: true, Connected: time.Now()},
					Play:         launcher.PlayStats{Launches: 6, TotalSec: 18500, LastSec: 1450, LastStart: time.Now(), LastExit: "exited"},
				}
				a.recentPing.cache = map[string]serverProbe{"play.example.test:25565": {checked: time.Now(), status: &launcher.ServerStatus{
					Description: "Welcome to our Minecraft world\nBuild and explore together!",
					Version:     "NeoForge 1.21.1", Online: 8, Max: 20, PlayersKnown: true, Latency: 42 * time.Millisecond,
				}}}
				a.statusCache = map[string]launcher.InstanceStatus{a.instStatusKey: a.instStatus}
				tt.SetSize(size[0], size[1])
				tt.Advance(400 * time.Millisecond)
				for _, name := range []string{a.L("overview.installed_mods"), a.L("overview.local_worlds"), a.L("overview.visual_extras"), a.L("overview.play_time")} {
					b, ok := tt.Find(name)
					if !ok || b.W <= 0 || b.X < 0 || b.X+b.W > float32(size[0]) || b.Y+b.H > float32(size[1]) {
						t.Fatalf("stat escaped viewport: %s %+v", name, b)
					}
				}
				{
					for _, name := range []string{a.L("overview.recent_world"), a.L("server_ui.recent_server"), a.L("overview.screenshots"), a.L("overview.runtime"), a.L("overview.storage")} {
						b, ok := tt.Find(name)
						if !ok || b.X+b.W > float32(size[0]) || b.Y+b.H > float32(size[1]) {
							t.Fatalf("dashboard panel escaped viewport: %s %+v", name, b)
						}
					}
				}
				snapshotUI(t, tt, fmt.Sprintf("overview-%s-%d", lang, size[0]))
				before, _ := tt.Find(a.L("overview.storage"))
				viewport, _ := tt.Find("概览固定视区")
				if before.Y+before.H > viewport.Y+viewport.H+1 {
					t.Fatal("overview was clipped instead of fitted")
				}
				if size[0] < 1000 {
					tt.Scroll(780, 430, 0, 3000)
					tt.Advance(400 * time.Millisecond)
					if after, _ := tt.Find(a.L("overview.storage")); after != before {
						t.Fatal("wheel input scrolled the fixed overview")
					}
					b, ok := tt.Find(a.L("java_installed.copy_path"))
					if !ok || b.Y < 42 || b.Y+b.H > float32(size[1]) {
						t.Fatalf("storage actions are unreachable: %+v", b)
					}
					clickUI(t, tt, a.L("java_installed.copy_path"))
					if tt.Clipboard() != r.Path {
						t.Fatal("overview copied the wrong instance path")
					}
					snapshotUI(t, tt, "overview-small-bottom-"+lang)
				} else {
					clickUI(t, tt, a.L("overview.manage_backups"))
					if a.instTab != "worlds" || a.wv.open != "world" {
						t.Fatal("world shortcut lost the selected save")
					}
				}
			})
		}
	}
}

func TestOverviewUnrecordedTimeAndLoadingAreHonest(t *testing.T) {
	for _, lang := range []string{LangZH, LangEN} {
		t.Run(lang, func(t *testing.T) {
			a, tt := homeTester(t, lang)
			a.bgWG.Wait()
			tt.Frame()
			a.service = nil
			a.statusGen++
			a.instStatusErr, a.message, a.noticeShown = "", "", ""
			a.instStatusKey = a.root().ID + "|" + a.root().SelectedVersion
			a.instStatus = launcher.InstanceStatus{Dir: a.root().Path}
			a.contentSwaps = nil
			for _, size := range [][2]int{{1536, 1024}, {900, 540}} {
				tt.SetSize(size[0], size[1])
				tt.Frame()
				tt.Advance(400 * time.Millisecond)
				if !tt.HasText(a.L("overview.no_recorded_sessions")) || !tt.HasText(a.L("overview.tracked_when_launched_here")) {
					t.Fatalf("missing unrecorded play state: %v", tt.Texts())
				}
				var artwork ui.Rect
				for i, item := range [][3]string{
					{"world-empty", a.L("overview.no_worlds_yet"), a.L("overview.recent_world")},
					{"server-empty", a.L("server_ui.no_connections_yet"), a.L("server_ui.recent_server")},
					{"screenshot-empty", a.L("overview.no_screenshots_yet"), a.L("overview.screenshots")},
				} {
					art, ok := tt.Find("空状态插画 " + item[0])
					title, titleOK := tt.Find(item[1])
					panel, _ := tt.Find(item[2])
					if !ok || !titleOK || art.Y+art.H > title.Y || title.Y+title.H > panel.Y+panel.H || title.X < panel.X || title.X+title.W > panel.X+panel.W {
						t.Fatalf("empty content is clipped or overlaps in %s: art %+v title %+v panel %+v", item[0], art, title, panel)
					}
					if i > 0 && (art.Y != artwork.Y || art.H != artwork.H) {
						t.Fatalf("empty cards are not aligned: %+v vs %+v", artwork, art)
					}
					artwork = art
				}
				snapshotUI(t, tt, fmt.Sprintf("empty-overview-%s-%d", lang, size[0]))
			}
			a.instStatus = launcher.InstanceStatus{}
			a.overviewDisplay = overviewPresentation{}
			a.contentSwaps = nil
			tt.Frame()
			if !tt.HasText("概览加载占位") || tt.HasText(a.L("overview.no_worlds_yet")) {
				t.Fatal("unloaded save counts were presented as an empty world list")
			}
		})
	}
}
