package nativeui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"mcl/internal/launcher"
)

func TestNamingTable(t *testing.T) {
	a, _ := acctTester(t, "zh-CN", 1280, 800)
	pack := launcher.Instance{Pack: &launcher.PackLock{Name: "Fabulously Optimized", Version: "6.5.0", LoaderVersion: "0.16.14", GameVersion: "fabric-loader-0.16.14-1.21.1"}}
	cases := []struct {
		name  string
		r     launcher.Instance
		v     launcher.VersionSummary
		title string
		sub   string
		mcTag bool
	}{
		{"vanilla", launcher.Instance{}, launcher.VersionSummary{ID: "1.21.1", Minecraft: "1.21.1", Loader: "vanilla"}, "原版 1.21.1", "Minecraft", false},
		{"snapshot", launcher.Instance{}, launcher.VersionSummary{ID: "24w14a", Minecraft: "24w14a", Loader: "vanilla", Snapshot: true}, "快照 24w14a", "Minecraft", false},
		{"fabric", launcher.Instance{}, launcher.VersionSummary{ID: "fabric-loader-0.16.14-1.21.1", Minecraft: "1.21.1", Loader: "fabric", LoaderVersion: "0.16.14"}, "Fabric 0.16.14", "", true},
		{"fabric id only", launcher.Instance{}, launcher.VersionSummary{ID: "fabric-loader-0.16.14-1.21.1", Minecraft: "1.21.1", Loader: "fabric"}, "Fabric 0.16.14", "", true},
		{"quilt", launcher.Instance{}, launcher.VersionSummary{ID: "quilt-loader-0.29.2-1.21.1", Minecraft: "1.21.1", Loader: "quilt", LoaderVersion: "0.29.2"}, "Quilt 0.29.2", "", true},
		{"quilt beta", launcher.Instance{}, launcher.VersionSummary{ID: "quilt-loader-0.20.0-beta.9-1.21.8", Minecraft: "1.21.8", Loader: "quilt", LoaderVersion: "0.20.0-beta.9"}, "Quilt 0.20.0-beta.9", "", true},
		{"forge", launcher.Instance{}, launcher.VersionSummary{ID: "1.20.1-forge-47.3.0", Minecraft: "1.20.1", Loader: "forge", LoaderVersion: "47.3.0"}, "Forge 47.3.0", "", true},
		{"neoforge", launcher.Instance{}, launcher.VersionSummary{ID: "neoforge-21.1.77", Minecraft: "1.21.1", Loader: "neoforge", LoaderVersion: "21.1.77"}, "NeoForge 21.1.77", "", true},
		{"other launcher id", launcher.Instance{}, launcher.VersionSummary{ID: "1.21.1-Fabric 0.16.14", Minecraft: "1.21.1", Loader: "fabric", LoaderVersion: "0.16.14"}, "Fabric 0.16.14", "", true},
		{"modpack", pack, launcher.VersionSummary{ID: "fabric-loader-0.16.14-1.21.1", Minecraft: "1.21.1", Loader: "fabric", LoaderVersion: "0.16.14"}, "Fabulously Optimized", "v6.5.0 · Fabric 0.16.14", true},
		{"custom id", launcher.Instance{}, launcher.VersionSummary{ID: "我的生存服", Minecraft: "1.21.1", Loader: "fabric", LoaderVersion: "0.16.14"}, "我的生存服", "Fabric 0.16.14", true},
		{"custom vanilla id", launcher.Instance{}, launcher.VersionSummary{ID: "pvp-1.8.9", Minecraft: "1.8.9", Loader: "vanilla"}, "pvp-1.8.9", "原版", true},
		{"user name wins", pack, launcher.VersionSummary{ID: "fabric-loader-0.16.14-1.21.1", Minecraft: "1.21.1", Loader: "fabric", LoaderVersion: "0.16.14", DisplayName: "周末联机"}, "周末联机", "Fabric 0.16.14", true},
	}
	for _, c := range cases {
		c.r.Versions = []launcher.VersionSummary{c.v}
		n := a.nameOf(c.r, c.v)
		title, sub := a.cardText(c.r, c.v)
		if title != c.title || sub != c.sub || n.MCTag != c.mcTag {
			t.Errorf("%s: got %q / %q / tag=%v, want %q / %q / tag=%v", c.name, title, sub, n.MCTag, c.title, c.sub, c.mcTag)
		}
	}
	// duplicates: same title, same MC -> folder id disambiguates; different MC -> tag does
	r := launcher.Instance{Versions: []launcher.VersionSummary{
		{ID: "fabric-loader-0.16.14-1.21.1", Minecraft: "1.21.1", Loader: "fabric", LoaderVersion: "0.16.14"},
		{ID: "1.21.1-Fabric 0.16.14", Minecraft: "1.21.1", Loader: "fabric", LoaderVersion: "0.16.14"},
		{ID: "fabric-loader-0.16.14-1.20.1", Minecraft: "1.20.1", Loader: "fabric", LoaderVersion: "0.16.14"},
	}}
	t0, s0 := a.cardText(r, r.Versions[0])
	t1, s1 := a.cardText(r, r.Versions[1])
	_, s2 := a.cardText(r, r.Versions[2])
	if t0+s0 == t1+s1 || !strings.Contains(s1, "1.21.1-Fabric 0.16.14") {
		t.Errorf("identical rows: %q %q / %q %q", t0, s0, t1, s1)
	}
	if s2 != "" {
		t.Errorf("different MC needs no suffix: %q", s2)
	}
}

func namesFixture(a *App) {
	long := strings.Repeat("超长的目录名称 Very Long Folder ", 4)
	a.snapshot.Instances = append(a.snapshot.Instances, launcher.Instance{Root: launcher.Root{ID: "names", Name: long, Path: `C:\Users\SomeoneWithALongName\AppData\Roaming\` + long + `\.minecraft`, SelectedVersion: "fabric-loader-0.16.14-1.21.1"}, Available: true,
		Versions: []launcher.VersionSummary{
			{ID: "fabric-loader-0.16.14-1.21.1", Minecraft: "1.21.1", Loader: "fabric", LoaderVersion: "0.16.14", JavaVersion: 21},
			{ID: "1.21.1-Fabric 0.16.14", Minecraft: "1.21.1", Loader: "fabric", LoaderVersion: "0.16.14", JavaVersion: 21},
			{ID: "fabric-loader-0.16.14-1.20.1", Minecraft: "1.20.1", Loader: "fabric", LoaderVersion: "0.16.14", JavaVersion: 17},
			{ID: "neoforge-21.1.77", Minecraft: "1.21.1", Loader: "neoforge", LoaderVersion: "21.1.77", MemoryMB: 4096},
			{ID: "1.20.1-forge-47.3.0", Minecraft: "1.20.1", Loader: "forge", LoaderVersion: "47.3.0"},
			{ID: "quilt-loader-0.29.2-1.21.1", Minecraft: "1.21.1", Loader: "quilt", LoaderVersion: "0.29.2-beta.3"},
			{ID: "我的超级无敌长名字生存服务器整合实例 My Extremely Long Custom Instance Name", Minecraft: "1.21.1", Loader: "fabric", LoaderVersion: "0.16.14", JVMArgs: "-X"},
			{ID: "1.21.1", Minecraft: "1.21.1", Loader: "vanilla", JavaVersion: 21},
			{ID: "24w14a", Minecraft: "24w14a", Loader: "vanilla", Snapshot: true},
		}})
	a.snapshot.ActiveRoot = "names"
}

// Long names, folders and tags never overflow their cards at min size and
// 2x, in Chinese and English.
func TestNamesLongStringsFit(t *testing.T) {
	for _, lang := range []string{"zh-CN", "en-US"} {
		for _, c := range []struct {
			n     string
			w, h  int
			scale float32
		}{{"1x", 1280, 800, 1}, {"2x", 1280, 800, 2}, {"min", 960, 640, 1}} {
			a, tt := acctTester(t, lang, c.w, c.h)
			tt.SetScale(c.scale)
			namesFixture(a)
			a.closeDialog()
			tt.Frame()
			tt.Advance(600 * time.Millisecond)
			for _, v := range a.snapshot.Instances[len(a.snapshot.Instances)-1].Versions {
				card, ok := tt.Find("版本卡片 " + v.ID)
				if !ok {
					continue
				}
				for _, lbl := range []string{"MC 标签 " + v.Minecraft} {
					if r, ok := tt.Find(lbl); ok && r.Y > card.Y && r.Y < card.Y+card.H && r.X+r.W > card.X+card.W+1 {
						t.Errorf("%s %s: tag overflows card %s", lang, c.n, v.ID)
					}
				}
			}
			tab, ok := tt.Find("目录 " + a.snapshot.Instances[len(a.snapshot.Instances)-1].Name)
			if ok && tab.W > 262*uiScaleFor(c.w, c.h)+1 {
				t.Errorf("%s %s: tab too wide %v", lang, c.n, tab.W)
			}
			if shotsOn() {
				snapshotUI(t, tt, fmt.Sprintf("names_home_%s_%s", map[string]string{"zh-CN": "zh", "en-US": "en"}[lang], c.n))
			}
		}
	}
	_ = ui.Rect{}
}
