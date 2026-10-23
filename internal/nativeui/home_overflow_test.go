package nativeui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// homeTester builds a home page with real-world installer ids plus very long
// custom names, so every card, row and the launch dock has to ellipsize.
func homeTester(t *testing.T, lang string) (*App, *ui.Tester) {
	t.Helper()
	t.Setenv("MCL_DEVELOPMENT", "")
	t.Setenv("CURSEFORGE_API_KEY", "")
	s, e := launcher.New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a, e := New(s)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(a.cancel)
	before := a.settings.Language
	a.SetLanguage(lang)
	if a.settings.Language != before {
		// setLanguage saves in the background and then posts a fresh
		// snapshot; take it now so it cannot land on top of the fixture.
		select {
		case fn := <-a.events:
			fn()
		case <-time.After(5 * time.Second):
			t.Fatal("language save never posted")
		}
	}
	long := "My-Super-Long-Custom-Modpack-Instance-Name-That-Never-Ends-1.21.8-fabric-loader-0.20.1"
	homeFixture = []launcher.Instance{
		{Root: launcher.Root{ID: "r1", Name: "一个非常非常长的游戏目录名称 Very Long Game Directory Name For Testing", Path: filepath.Join(os.TempDir(), strings.Repeat("deep-folder/", 12), ".minecraft"), SelectedVersion: "quilt-loader-0.20.0-beta.9-1.21.8"}, Available: true, ModCount: 12, Versions: []launcher.VersionSummary{
			{ID: "1.21.8", Minecraft: "1.21.8", Loader: "vanilla", JavaVersion: 21},
			{ID: "fabric-loader-0.20.1-1.21.8", Minecraft: "1.21.8", Loader: "fabric"},
			{ID: "quilt-loader-0.20.0-beta.9-1.21.8", Minecraft: "1.21.8", Loader: "quilt", MemoryMB: 8192, JVMArgs: "-XX:+UseG1GC", JavaPath: "/x/java"},
			{ID: "neoforge-21.1.264", Minecraft: "1.21.1", Loader: "neoforge"},
			{ID: "1.20.1-forge-47.4.28", Minecraft: "1.20.1", Loader: "forge"},
			{ID: long, Minecraft: "1.21.8", Loader: "fabric", MemoryMB: 4096},
		}},
		{Root: launcher.Root{ID: "r2", Name: "Second", Path: "/games/second/.minecraft"}, Available: true},
	}
	a.page = "home"
	tt := ui.NewTester(a.View, 1536, 1024)
	tt.Frame()
	a.snapshot.Instances, a.snapshot.ActiveRoot = homeFixture, "r1"
	tt.Frame()
	return a, tt
}

var homeFixture []launcher.Instance

func TestHomeTextNeverOverflows(t *testing.T) {
	for _, lang := range []string{"zh-CN", "en-US"} {
		for _, size := range [][2]int{{1536, 1024}, {1100, 760}, {900, 600}} {
			t.Run(fmt.Sprintf("%s-%dx%d", lang, size[0], size[1]), func(t *testing.T) {
				a, tt := homeTester(t, lang)
				tt.SetSize(size[0], size[1])
				tt.Frame()
				a.snapshot.Instances, a.snapshot.ActiveRoot = homeFixture, "r1"
				tt.Frame()
				var boxes []ui.Rect
				var names []string
				for _, v := range homeFixture[0].Versions {
					r, ok := tt.Find("版本卡片 " + v.ID)
					if !ok {
						continue // scrolled out of the viewport
					}
					boxes, names = append(boxes, r), append(names, v.ID)
				}
				if len(boxes) == 0 {
					t.Fatal("no version tiles rendered")
				}
				bar, ok := tt.Find("目录选择栏")
				if !ok {
					t.Fatal("missing directory picker")
				}
				for _, in := range homeFixture {
					if r, ok := tt.Find("目录 " + in.Name); ok {
						boxes, names = append(boxes, r), append(names, "目录 "+in.Name)
					}
				}
				for _, n := range []string{"主版本卡", "版本列表"} {
					r, ok := tt.Find(n)
					if !ok {
						t.Fatalf("missing %s", n)
					}
					boxes, names = append(boxes, r), append(names, n)
				}
				for _, txt := range tt.Texts() {
					r, _ := tt.Find(txt)
					if r.W <= 0 || strings.HasPrefix(txt, "版本卡片") || strings.HasPrefix(txt, "目录") || txt == "主版本卡" || txt == "版本列表" {
						continue
					}
					inBar := r.Y >= bar.Y-0.5 && r.Y+r.H <= bar.Y+bar.H+0.5
					if !inBar && (r.X < -0.5 || r.X+r.W > float32(size[0])+0.5) {
						t.Errorf("text %q escapes window: %+v", txt, r)
					}
					cx, cy := r.X+r.W/2, r.Y+r.H/2
					for i, b := range boxes {
						if cx > b.X && cx < b.X+b.W && cy > b.Y && cy < b.Y+b.H {
							if r.X < b.X-0.5 || r.X+r.W > b.X+b.W+0.5 {
								t.Errorf("text %q overflows %s: text %+v box %+v", txt, names[i], r, b)
							}
						}
					}
				}
				// No duplicated install entry and no raw installer id as title.
				for _, dup := range []string{a.tr().HomeQuickInstallLoader} {
					if dup != a.tr().HomeInstallBtn && tt.HasText(dup) {
						t.Errorf("duplicated install entry %q", dup)
					}
				}
				// loader name as title, MC version as its own tag, no raw id
				if lang == "zh-CN" && (!tt.HasText("Fabric") || !tt.HasText("Minecraft 1.21.8")) {
					t.Error("expected clean display title")
				}
				if os.Getenv("MCL_NATIVE_QA_OUTPUT") != "" && lang == "zh-CN" {
					snapshotUI(t, tt, fmt.Sprintf("home-overflow-%d", size[0]))
				}
			})
		}
	}
}

// NoWrap keeps text whole even when it overflows, so pairing it with
// Ellipsis silently clips mid-glyph. SingleLine is the ellipsizing form.
func TestNoClippingTextPattern(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		b, _ := os.ReadFile(f)
		if strings.Contains(string(b), "NoWrap().Ellipsis(") && !strings.HasSuffix(f, "_test.go") {
			t.Errorf("%s uses NoWrap().Ellipsis; use SingleLine()", f)
		}
	}
}

// Launch lives in the instance hero; the selected version stays in the
// list (highlighted), so the list never reflows on a switch.
func TestHomeFeaturedCardOwnsLaunch(t *testing.T) {
	for _, lang := range []string{"zh-CN", "en-US"} {
		a, tt := homeTester(t, lang)
		hero, ok := tt.Find("主版本卡")
		launch, ok2 := tt.Find(a.tr().HomeLaunchBtn)
		if !ok || !ok2 || launch.Y < hero.Y || launch.Y+launch.H > hero.Y+hero.H || launch.X+launch.W > hero.X+hero.W {
			t.Errorf("%s: launch must be in the hero: %+v %+v", lang, launch, hero)
		}
		if _, ok := tt.Find("启动栏"); ok {
			t.Errorf("%s: the floating launch dock should be gone", lang)
		}
		if _, ok := tt.Find(a.tr().HomeVersionLabel); ok {
			t.Errorf("%s: version dropdown should be gone", lang)
		}
		for _, id := range []string{"quilt-loader-0.20.0-beta.9-1.21.8", "fabric-loader-0.20.1-1.21.8"} {
			if _, ok := tt.Find("版本卡片 " + id); !ok {
				t.Errorf("%s: version %s missing from the list", lang, id)
			}
		}
	}
}

// manyRoots returns n directories (first = fixture root, others empty).
func manyRoots(n int) []launcher.Instance {
	out := []launcher.Instance{homeFixture[0]}
	for i := 1; i < n; i++ {
		out = append(out, launcher.Instance{Root: launcher.Root{ID: fmt.Sprint("x", i), Name: fmt.Sprintf("目录 Directory %d with a pretty long name", i), Path: fmt.Sprintf("/games/%d/.minecraft", i)}, Available: true})
	}
	return out
}

func TestRootTabsFixedHeightAndVersionsVisible(t *testing.T) {
	a, tt := homeTester(t, "zh-CN")
	tt.SetSize(1060, 720)
	var heights []float32
	for _, n := range []int{2, 6} {
		a.snapshot.Instances, a.snapshot.ActiveRoot = manyRoots(n), "r1"
		tt.Frame()
		bar, ok := tt.Find("目录选择栏")
		if !ok {
			t.Fatal("directory picker missing")
		}
		heights = append(heights, bar.H)
		card, ok := tt.Find("主版本卡")
		tile, ok2 := tt.Find("版本卡片 fabric-loader-0.20.1-1.21.8")
		if !ok || !ok2 || card.Y+card.H > 720 || tile.Y+tile.H > bar.Y {
			t.Errorf("%d dirs: versions not in first screen: card %+v tile %+v picker %+v", n, card, tile, bar)
		}
	}
	if heights[0] != heights[1] {
		t.Errorf("directory picker height must not depend on directory count: %v", heights)
	}
}

func TestEmptyRootShowsInstallHint(t *testing.T) {
	a, tt := homeTester(t, "zh-CN")
	for i := 0; i < 5 && !tt.HasText("此目录还没有版本"); i++ {
		// a background refresh may replace the snapshot; re-apply the fixture
		a.snapshot.Instances, a.snapshot.ActiveRoot = manyRoots(3), "x1"
		tt.Frame()
	}
	if !tt.HasText("此目录还没有版本") {
		t.Error("empty directory hint missing", tt.Texts())
	}
	if _, ok := tt.Find("install-empty-root"); !ok && !tt.HasText(a.tr().HomeInstallBtn) {
		t.Error("install button missing")
	}
}
