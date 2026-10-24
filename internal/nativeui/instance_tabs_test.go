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

func instanceTabsTester(t *testing.T, tab string) (*App, *ui.Tester) {
	t.Helper()
	a, tt := homeTester(t, LangZH)
	a.bgWG.Wait()
	tt.Frame()
	a.service = nil // drive catalog completion without disk or network timing
	a.message, a.errorMessage = "", false
	a.snapshot.Instances[1].Versions = append([]launcher.VersionSummary(nil), a.root().Versions...)
	a.snapshot.Instances[1].SelectedVersion = "1.21.8"
	a.logSess = logSessions{polled: time.Now().Add(time.Hour)}
	a.gameStates = map[string]launcher.GameOptions{}
	a.imageMu.Lock()
	for _, r := range a.snapshot.Instances {
		for _, v := range r.Versions {
			a.imageRetry["version:"+r.ID+":"+v.ID] = time.Now()
			a.gameStates[r.ID+"|"+v.ID] = launcher.GameOptions{}
			for _, kind := range []string{"mods", "resourcepack", "shader"} {
				name := r.ID + "-" + v.ID + "-" + kind
				entry := addonCacheEntry{}
				if kind == "mods" {
					entry.mods = []launcher.Mod{{File: "test.jar", Name: name, Enabled: true}}
					a.imageRetry["addon:"+r.ID+":"+v.ID+":mod:test.jar"] = time.Now()
				} else {
					entry.packs = []launcher.AssetPack{{File: "test.zip", Name: name, Kind: kind}}
					a.imageRetry["addon:"+r.ID+":"+v.ID+":"+kind+":test.zip"] = time.Now()
				}
				if a.addonCache == nil {
					a.addonCache = map[string]addonCacheEntry{}
				}
				a.addonCache[r.ID+"|"+v.ID+"|"+kind] = entry
			}
		}
	}
	a.imageMu.Unlock()
	a.setInstTab(tab)
	a.loadMods()
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	return a, tt
}

func TestInstanceTabsAnimateVersionAndRootSwitches(t *testing.T) {
	for _, tab := range []string{"mods", "resourcepack", "shader", "worlds", "logs", "settings"} {
		t.Run(tab, func(t *testing.T) {
			a, tt := instanceTabsTester(t, tab)
			rest := addonFilterBounds(t, tt, "实例标签内容")
			bar := addonFilterBounds(t, tt, "实例标签")
			hero := addonFilterBounds(t, tt, "主版本卡")
			indicator := addonFilterBounds(t, tt, "实例标签选中线")
			pixels := overviewRegionPixels(tt.Image(), bar)
			// Both a version change and another root with the same version ID
			// must animate. The tab itself and all surrounding chrome stay put.
			for _, root := range []string{"r1", "r2"} {
				a.selectVersion(root, "1.21.8")
				tt.Frame()
				tt.Advance(65 * time.Millisecond)
				mid := addonFilterBounds(t, tt, "实例标签内容")
				if mid.Y <= rest.Y || mid.X != rest.X || mid.W != rest.W {
					t.Fatalf("%s: expected vertical entrance at native width: rest=%+v mid=%+v", root, rest, mid)
				}
				if now := addonFilterBounds(t, tt, "主版本卡"); now != hero {
					t.Fatal("tab animation moved the hero")
				}
				if now := addonFilterBounds(t, tt, "实例标签"); now != bar || !bytes.Equal(pixels, overviewRegionPixels(tt.Image(), bar)) {
					t.Fatal("version switch moved or repainted the tabs")
				}
				if now := addonFilterBounds(t, tt, "实例标签选中线"); now != indicator {
					t.Fatal("version switch moved the selected tab")
				}
				snapshotUI(t, tt, "instance-"+tab+"-"+root+"-065ms")
				tt.Advance(300 * time.Millisecond)
				if end := addonFilterBounds(t, tt, "实例标签内容"); end != rest {
					t.Fatalf("pane did not return to its original viewport: %+v -> %+v", rest, end)
				}
				if tab == "mods" || tab == "resourcepack" || tab == "shader" {
					if !tt.HasText(root + "-1.21.8-" + tab) {
						t.Fatal("transition did not show the selected instance's cached content")
					}
				}
			}
			snapshotUI(t, tt, "instance-"+tab+"-settled")
		})
	}
}

func TestInstanceTabIndicatorSlidesAndRapidSwitchUsesLatestContent(t *testing.T) {
	a, tt := instanceTabsTester(t, "mods")
	buttons := map[string]ui.Rect{}
	for _, tab := range a.instTabs(*a.root()) {
		buttons[tab.text] = addonFilterBounds(t, tt, "实例标签 "+tab.text)
	}
	start := addonFilterBounds(t, tt, "实例标签选中线")
	clickUI(t, tt, "实例标签 设置")
	tt.Advance(65 * time.Millisecond)
	mid := addonFilterBounds(t, tt, "实例标签选中线")
	if mid.X <= start.X || mid.X >= buttons["设置"].X {
		t.Fatalf("indicator jumped rather than sliding: start=%+v mid=%+v", start, mid)
	}
	if _, ok := tt.Find("名称与 Java"); !ok {
		t.Fatal("new tab was not interactive during its entrance")
	}
	for label, want := range buttons {
		if now := addonFilterBounds(t, tt, "实例标签 "+label); now != want {
			t.Fatalf("selecting a tab moved %s: %+v -> %+v", label, want, now)
		}
	}
	snapshotUI(t, tt, "instance-tabs-moving-right")
	progress := a.instTabMotion.progress
	clickUI(t, tt, "实例标签 资源包")
	clickUI(t, tt, "实例标签 模组")
	if a.instTabMotion.progress < progress {
		t.Fatal("rapid navigation restarted the content fade")
	}
	if _, ok := tt.Find("名称与 Java"); ok {
		t.Fatal("outgoing settings remain interactive after switching tabs")
	}
	tt.Advance(300 * time.Millisecond)
	if a.instTab != "mods" || addonFilterBounds(t, tt, "实例标签选中线") != start {
		t.Fatal("rapid switch did not settle on the final tab")
	}
	clickUI(t, tt, "实例标签 设置")
	tt.Advance(300 * time.Millisecond)
	clickUI(t, tt, "实例标签 模组")
	tt.Advance(65 * time.Millisecond)
	body := addonFilterBounds(t, tt, "资源卡片")
	snapshotUI(t, tt, "instance-tabs-moving-left")
	tt.Advance(300 * time.Millisecond)
	end := addonFilterBounds(t, tt, "资源卡片")
	if body.X >= end.X || math.Abs(float64(body.W-end.W)) > .1 {
		t.Fatalf("backward navigation did not slide left at native width: mid=%+v end=%+v", body, end)
	}
}

func TestInstanceTabFirstResultsAnimateButStreamingDoesNot(t *testing.T) {
	a, tt := instanceTabsTester(t, "mods")
	delete(a.addonCache, "r1|1.21.8|mods")
	a.selectVersion("r1", "1.21.8")
	tt.Frame()
	tt.Advance(350 * time.Millisecond)
	if !a.modsLoading || tt.HasText("r1-1.21.8-mods") {
		t.Fatal("cold switch displayed stale results")
	}
	rest := addonFilterBounds(t, tt, "实例标签内容")
	a.mods = []launcher.Mod{{File: "test.jar", Name: "First loaded result", Enabled: true}}
	tt.Frame()
	tt.Advance(65 * time.Millisecond)
	if mid := addonFilterBounds(t, tt, "实例标签内容"); mid.Y <= rest.Y || !tt.HasText("First loaded result") {
		t.Fatal("slow first result appeared without its entrance")
	}
	tt.Advance(300 * time.Millisecond)
	a.mods[0].Description = "Metadata arrived later"
	a.modsLoading = false
	tt.Frame()
	if now := addonFilterBounds(t, tt, "实例标签内容"); now != rest || a.instTabMotion.progress != 1 {
		t.Fatal("streaming or metadata replayed the navigation animation")
	}
}

func TestInstanceTabsReducedMotionAndCompactLabels(t *testing.T) {
	for _, language := range []string{LangZH, LangEN} {
		t.Run(language, func(t *testing.T) {
			a, tt := instanceTabsTester(t, "mods")
			a.OverrideLanguage(language)
			tt.SetSize(900, 600)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			rest := addonFilterBounds(t, tt, "实例标签内容")
			a.selectVersion("r1", "1.21.8")
			tt.Frame()
			if now := addonFilterBounds(t, tt, "实例标签内容"); now != rest {
				t.Fatal("reduced motion still moves content on version changes")
			}
			var right float32
			for _, tab := range a.instTabs(*a.root()) {
				button := addonFilterBounds(t, tt, "实例标签 "+tab.text)
				if button.X < right || button.X+button.W > 900 {
					t.Fatalf("compact tabs overlap or clip: %s %+v", tab.key, button)
				}
				right = button.X + button.W
				clickUI(t, tt, "实例标签 "+tab.text)
				tt.Frame()
				selected := addonFilterBounds(t, tt, "实例标签选中线")
				if selected.X < button.X || selected.X+selected.W > button.X+button.W || a.instTabMotion.progress != 1 {
					t.Fatal("reduced motion did not snap to the selected tab")
				}
				bar := addonFilterBounds(t, tt, "实例标签")
				pixels := overviewRegionPixels(tt.Image(), bar)
				tt.Advance(300 * time.Millisecond)
				if !bytes.Equal(pixels, overviewRegionPixels(tt.Image(), bar)) {
					t.Fatalf("label and indicator did not switch together with reduced motion on %s", tab.key)
				}
			}
			snapshotUI(t, tt, fmt.Sprintf("instance-tabs-compact-%s", language))
		})
	}
}
