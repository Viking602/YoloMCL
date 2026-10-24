package nativeui

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func addonFilterTester(t *testing.T, list bool, language string) (*App, *ui.Tester) {
	t.Helper()
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	a := &App{
		snapshot: launcher.Snapshot{ActiveRoot: "r", Instances: []launcher.Instance{{Root: launcher.Root{ID: "r", SelectedVersion: "pack"}}}},
		addonTab: "mods", instTab: "mods", addonListMode: list, modsLoading: true,
		images: map[string]ui.ImageSource{}, imagePending: map[string]bool{}, imageRetry: map[string]time.Time{},
		gameStates: map[string]launcher.GameOptions{"r|pack": {}},
	}
	a.settings.Language = language
	for i := range 371 {
		file := fmt.Sprintf("mod-%03d.jar", i)
		m := launcher.Mod{File: file, Name: fmt.Sprintf("Mod %03d", i), Enabled: true, Version: "1.21.1", Size: 12500,
			Description: "Explore, build and discover new adventures in your world."}
		if i < 148 {
			m.Latest = &launcher.ProjectVersion{ID: "next", Name: "1.21.2"}
		}
		a.mods = append(a.mods, m)
		a.imageRetry["addon:r:pack:mod:"+file] = time.Now()
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		scroll := ui.Scroll(c).Fill().Padding(24).Background(chromeBg)
		a.tabViewBounds = scroll.LayoutBounds()
		scroll.Children(func() {
			ui.Column(c).FillWidth().Gap(20).Children(func() { a.addonTabBody(c) })
		})
	}, 1100, 760)
	tt.Advance(400 * time.Millisecond)
	return a, tt
}

func addonFilterBounds(t *testing.T, tt *ui.Tester, label string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("missing %s", label)
	}
	return r
}

func TestAddonFilterSlidesAndRefreshesResults(t *testing.T) {
	for _, list := range []bool{false, true} {
		t.Run(fmt.Sprint(list), func(t *testing.T) {
			a, tt := addonFilterTester(t, list, LangZH)
			start := addonFilterBounds(t, tt, "筛选选中背景")
			rest := addonFilterBounds(t, tt, "筛选结果")
			mode := "grid"
			if list {
				mode = "list"
			}
			snapshotUI(t, tt, "addon-filter-"+mode+"-all")
			clickUI(t, tt, "可更新")
			tt.Advance(65 * time.Millisecond)
			mid := addonFilterBounds(t, tt, "筛选选中背景")
			target := addonFilterBounds(t, tt, "可更新")
			body := addonFilterBounds(t, tt, "筛选结果")
			if mid.X <= start.X || mid.X >= target.X || body.Y <= rest.Y || body.W != rest.W {
				t.Fatalf("expected a sliding indicator and native-size result entrance: start %+v mid %+v target %+v body %+v rest %+v", start, mid, target, body, rest)
			}
			if a.addonFilterKind != "update" || !tt.HasText("Mod 000") || tt.HasText("Mod 148") {
				t.Fatal("filter did not immediately display the latest results")
			}
			snapshotUI(t, tt, "addon-filter-"+mode+"-065ms")
			tt.Advance(300 * time.Millisecond)
			end := addonFilterBounds(t, tt, "筛选选中背景")
			if math.Abs(float64(end.X-target.X)) > .1 || math.Abs(float64(end.W-target.W)) > .1 || a.addonFilterMotion.progress != 1 {
				t.Fatalf("filter did not settle on the selected button: %+v %+v", end, target)
			}
			if body := addonFilterBounds(t, tt, "筛选结果"); body.Y != rest.Y {
				t.Fatalf("result entrance changed the resting layout: %+v %+v", body, rest)
			}
			clickUI(t, tt, "可更新")
			if a.addonFilterMotion.progress != 1 {
				t.Fatal("clicking the current filter replayed the entrance")
			}
			a.mods[0].Latest = nil
			tt.Frame()
			if a.addonFilterMotion.progress != 1 || tt.HasText("Mod 000") || !tt.HasText("Mod 001") {
				t.Fatal("background metadata update replayed motion or left stale results")
			}
			if !list {
				n := 0
				for _, text := range tt.Texts() {
					if strings.HasPrefix(text, "Mod ") {
						n++
					}
				}
				if n > 40 {
					t.Fatalf("animated filtering built %d cards instead of virtualizing", n)
				}
			}
			snapshotUI(t, tt, "addon-filter-"+mode+"-updates")
		})
	}
}

func TestAddonFilterRapidChangesAndReducedMotion(t *testing.T) {
	a, tt := addonFilterTester(t, false, "en")
	clickUI(t, tt, "Updates")
	tt.Advance(50 * time.Millisecond)
	mid := addonFilterBounds(t, tt, "筛选选中背景")
	progress := a.addonFilterMotion.progress
	clickUI(t, tt, "Enabled")
	clickUI(t, tt, "All")
	if now := addonFilterBounds(t, tt, "筛选选中背景"); math.Abs(float64(mid.X-now.X)) > .1 || a.addonFilterMotion.progress < progress {
		t.Fatal("rapid selection jumped the indicator or restarted the result fade")
	}
	tt.Advance(300 * time.Millisecond)
	end, target := addonFilterBounds(t, tt, "筛选选中背景"), addonFilterBounds(t, tt, "All")
	if end.X != target.X || a.addonFilterKind != "" || a.addonFilterMotion.progress != 1 {
		t.Fatal("rapid selection did not settle on the latest filter")
	}
	clickUI(t, tt, "Updates")
	tt.Advance(35 * time.Millisecond)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	end, target = addonFilterBounds(t, tt, "筛选选中背景"), addonFilterBounds(t, tt, "Updates")
	if end.X != target.X || a.addonFilterMotion.progress != 1 {
		t.Fatal("reduced motion did not finish an in-progress switch")
	}
	a.mods[0].Enabled = false
	tt.Frame()
	clickUI(t, tt, "Disabled")
	end, target = addonFilterBounds(t, tt, "筛选选中背景"), addonFilterBounds(t, tt, "Disabled")
	if end.X != target.X || end.W != target.W || a.addonFilterMotion.progress != 1 || !tt.HasText("Mod 000") || tt.HasText("Mod 001") {
		t.Fatal("new disabled filter did not snap to its latest result with reduced motion")
	}
	clickUI(t, tt, "Updates")
	for i := range a.mods {
		a.mods[i].Latest = nil
	}
	tt.Frame()
	if !tt.HasText("No matching mods") {
		t.Fatal("a filter becoming empty did not show its empty state")
	}
	clickUI(t, tt, "Clear filters")
	tt.Frame()
	end, target = addonFilterBounds(t, tt, "筛选选中背景"), addonFilterBounds(t, tt, "All")
	if end.X != target.X || a.addonFilterKind != "" || !tt.HasText("Mod 001") {
		t.Fatal("clearing an empty filter left its indicator or results behind")
	}
}
