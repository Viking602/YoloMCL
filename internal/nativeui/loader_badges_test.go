package nativeui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestLoaderVersionFitsHeroWithSidebar(t *testing.T) {
	for _, tc := range []struct {
		width, height         int
		lang, loader, version string
		scale                 float32
	}{
		{900, 540, "zh-CN", "quilt", "0.29.2-beta.3", 1},
		{900, 800, "en-US", "forge", "10.13.4.1614-1.7.10", 1},
		{1100, 800, "zh-CN", "neoforge", "26.1.2.114", 2},
		{1199, 800, "en-US", "quilt", "0.29.2-beta.3", 2},
		{1200, 800, "zh-CN", "forge", "10.13.4.1614-1.7.10", 1},
		{1536, 1024, "en-US", "fabric", "0.16.14", 2},
	} {
		t.Run(fmt.Sprintf("%s-%d-%s-%gx", tc.lang, tc.width, tc.loader, tc.scale), func(t *testing.T) {
			a, tt := homeTester(t, tc.lang)
			a.bgWG.Wait()
			r := a.root()
			v := launcher.VersionSummary{ID: "loader-fit", DisplayName: "Loader layout fixture", Minecraft: "1.21.1", Loader: tc.loader, LoaderVersion: tc.version}
			r.Versions = []launcher.VersionSummary{v}
			r.SelectedVersion = v.ID
			r.Pack = &launcher.PackLock{Name: v.DisplayName, Source: "curseforge", GameVersion: v.ID}
			r.PackUpdate = &launcher.PackUpdateInfo{HasUpdate: true, Latest: &launcher.ProjectVersion{Number: "2026.10.23-release-long-modpack-update-name"}}
			r.RunningVersions = []string{v.ID}
			a.instLoaded = r.ID
			tt.SetSize(tc.width, tc.height)
			tt.SetScale(tc.scale)
			tt.Frame()
			tt.Advance(500 * time.Millisecond)
			var heroWidth, railWidth float32
			ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme)
				s := uiScale(c)
				heroWidth, _ = c.MeasureText(0, ui.Span{Text: v.LoaderVersion, Font: uiSansFamily(), Size: typeCaption * s, Weight: 500})
				railWidth, _ = c.MeasureText(0, ui.Span{Text: v.LoaderVersion, Size: typeFootnote * s})
			}, tc.width, tc.height)
			for _, item := range []struct {
				label, parent string
				width         float32
			}{
				{"加载器版本 " + v.LoaderVersion, a.L("hero_badges.instance_loader"), heroWidth},
				{"列表加载器版本 " + v.ID, "版本卡片 " + v.ID, railWidth},
			} {
				text, found := tt.Find(item.label)
				parent, parentFound := tt.Find(item.parent)
				if !found || !parentFound || text.W+1 < item.width || text.X < parent.X-.5 || text.X+text.W > parent.X+parent.W+.5 {
					t.Errorf("%s clipped: text=%+v parent=%+v required=%.1f", item.label, text, parent, item.width)
				}
			}
			version, _ := tt.Find("加载器版本 " + v.LoaderVersion)
			history, _ := tt.Find(a.L("hero_badges.play_history"))
			if version.X+version.W > history.X+.5 {
				t.Fatalf("loader version overlaps play history: %+v %+v", version, history)
			}
			snapshotUI(t, tt, fmt.Sprintf("loader-fit-%s-%d-%s", tc.lang, tc.width, tc.loader))
		})
	}
}

func TestRecommendedJavaIsTag(t *testing.T) {
	_, tt := homeTester(t, "zh-CN")
	if _, ok := tt.Find("Java 标签 21"); !ok {
		t.Error("recommended Java should render as a tag")
	}
	for _, txt := range tt.Texts() {
		if strings.Contains(txt, "· Java") {
			t.Errorf("inline Java text remains: %q", txt)
		}
	}
}
