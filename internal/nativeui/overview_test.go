package nativeui

import (
	"fmt"
	"strings"
	"testing"
	"time"

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
				for _, name := range []string{a.L("已安装模组", "Installed mods"), a.L("本地世界", "Local worlds"), a.L("画面扩展", "Visual extras"), a.L("累计游玩", "Play time")} {
					b, ok := tt.Find(name)
					if !ok || b.W <= 0 || b.X < 0 || b.X+b.W > float32(size[0]) || b.Y+b.H > float32(size[1]) {
						t.Fatalf("stat escaped viewport: %s %+v", name, b)
					}
				}
				{
					for _, name := range []string{a.L("最近世界", "Recent world"), a.L("最近服务器", "Recent server"), a.L("游戏截图", "Screenshots"), a.L("运行环境", "Runtime"), a.L("存储空间", "Storage")} {
						b, ok := tt.Find(name)
						if !ok || b.X+b.W > float32(size[0]) || b.Y+b.H > float32(size[1]) {
							t.Fatalf("dashboard panel escaped viewport: %s %+v", name, b)
						}
					}
				}
				snapshotUI(t, tt, fmt.Sprintf("overview-%s-%d", lang, size[0]))
				before, _ := tt.Find(a.L("存储空间", "Storage"))
				viewport, _ := tt.Find("概览固定视区")
				if before.Y+before.H > viewport.Y+viewport.H+1 {
					t.Fatal("overview was clipped instead of fitted")
				}
				if size[0] < 1000 {
					tt.Scroll(780, 430, 0, 3000)
					tt.Advance(400 * time.Millisecond)
					if after, _ := tt.Find(a.L("存储空间", "Storage")); after != before {
						t.Fatal("wheel input scrolled the fixed overview")
					}
					b, ok := tt.Find(a.L("复制路径", "Copy path"))
					if !ok || b.Y < 42 || b.Y+b.H > float32(size[1]) {
						t.Fatalf("storage actions are unreachable: %+v", b)
					}
					clickUI(t, tt, a.L("复制路径", "Copy path"))
					if tt.Clipboard() != r.Path {
						t.Fatal("overview copied the wrong instance path")
					}
					snapshotUI(t, tt, "overview-small-bottom-"+lang)
				} else {
					clickUI(t, tt, a.L("管理备份", "Manage backups"))
					if a.instTab != "worlds" || a.wv.open != "world" {
						t.Fatal("world shortcut lost the selected save")
					}
				}
			})
		}
	}
}

func TestOverviewUnrecordedTimeAndLoadingAreHonest(t *testing.T) {
	a, tt := homeTester(t, "en-US")
	a.statusGen++
	a.instStatusKey = a.root().ID + "|" + a.root().SelectedVersion
	a.instStatus = launcher.InstanceStatus{Dir: a.root().Path}
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	if !tt.HasText("No recorded sessions") || !tt.HasText("Tracked when launched here") || !tt.HasText("Your next world starts here") {
		t.Fatalf("missing empty states: %v", tt.Texts())
	}
	a.instStatus = launcher.InstanceStatus{}
	a.instStatusErr = ""
	a.overviewDisplay = overviewPresentation{}
	a.contentSwaps = nil
	tt.Frame()
	if !tt.HasText("概览加载占位") || tt.HasText("Your next world starts here") {
		t.Fatal("unloaded save counts were presented as an empty world list")
	}
}
