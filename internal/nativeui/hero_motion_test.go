package nativeui

import (
	"bytes"
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestHeroHistoryFitsCompactAndWideLayouts(t *testing.T) {
	for _, lang := range []string{"zh-CN", "en-US"} {
		for _, width := range []int{900, 1100, 1536} {
			t.Run(fmt.Sprintf("%s-%d", lang, width), func(t *testing.T) {
				a, tt := heroTester(t)
				a.OverrideLanguage(lang)
				r := *a.root()
				a.instStatusKey = r.ID + "|" + r.SelectedVersion
				a.instStatus = launcher.InstanceStatus{Dir: r.Path, LastLog: time.Date(2026, 11, 28, 18, 30, 0, 0, time.Local)}
				tt.SetSize(width, 1024)
				tt.Frame()
				tt.Advance(400 * time.Millisecond)
				slot, ok := tt.Find(a.L("游玩记录", "Play history"))
				date, found := tt.Find("数字翻牌 hero-history-date")
				if !ok || !found || date.X < slot.X-.5 || date.X+date.W > slot.X+slot.W+.5 || date.Y < slot.Y-.5 || date.Y+date.H > slot.Y+slot.H+.5 {
					t.Fatalf("date does not fit history slot: date=%+v slot=%+v", date, slot)
				}
				if record := a.heroHistory(r); !strings.Contains(record.tip, "2026-11-28 18:30") {
					t.Fatal("full timestamp missing from history tooltip")
				}
				snapshotUI(t, tt, fmt.Sprintf("hero-history-%s-%d", lang, width))
			})
		}
	}
}

func heroTester(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	a, _ := homeTester(t, "zh-CN")
	a.bgWG.Wait()
	// Isolate the hero from the sidebar and page transition when comparing
	// rapid selection changes. Background I/O is intentionally not involved.
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Background(chromeBg).Children(func() { a.instanceHero(c, *a.root()) })
	}, 1536, 1024)
	tt.Advance(400 * time.Millisecond)
	return a, tt
}

func TestHeroRapidSwitchKeepsStableSlots(t *testing.T) {
	a, tt := heroTester(t)
	stable := map[string]ui.Rect{}
	for _, label := range []string{"主版本卡", "版本图标", "版本标题", "游玩记录", "按钮文字 launch"} {
		stable[label], _ = tt.Find(label)
	}
	for i, v := range a.root().Versions {
		a.applySelection("r1", v.ID)
		tt.Frame()
		tt.Advance(25 * time.Millisecond) // interrupt well before each enter ends
		for label, want := range stable {
			if got, ok := tt.Find(label); !ok || got != want {
				t.Fatalf("switch %d moved %s: %+v -> %+v", i, label, want, got)
			}
		}
	}
	snapshotUI(t, tt, "hero-rapid-switch")
}

func TestHeroHistoryIsCompactAndAnchoredAcrossPacks(t *testing.T) {
	a, tt := heroTester(t)
	anchor, _ := tt.Find("游玩记录")
	if anchor.W > 140 {
		t.Fatalf("history badge is too wide: %.0f", anchor.W)
	}
	for _, v := range a.root().Versions {
		a.applySelection("r1", v.ID)
		r := *a.root()
		a.instStatusKey, a.instStatusErr = r.ID+"|"+v.ID, ""
		a.instStatus = launcher.InstanceStatus{Dir: r.Path, Play: launcher.PlayStats{
			Launches: 100, LastStart: time.Date(2026, 10, 20, 18, 30, 0, 0, time.Local), LastSec: 3600, TotalSec: 360000,
		}}
		tt.Frame()
		if now, _ := tt.Find("游玩记录"); now != anchor {
			t.Fatalf("loader or title moved history badge: %+v -> %+v", anchor, now)
		}
		tt.Advance(400 * time.Millisecond)
		if !tt.HasText("上次游玩 10月20日") || tt.HasText("时长") || tt.HasText("累计") {
			t.Fatal("history badge did not keep its compact date-only text")
		}
		snapshotUI(t, tt, "hero-history-"+v.Loader)
	}
	snapshotUI(t, tt, "hero-history-fixed")
}

// The old title must disappear in the very first frame. Pixel-check the title
// strip: transition ghosts are paint
// copies and are intentionally absent from the accessibility/text tree.
func TestHeroSwitchDoesNotRetainOldTitle(t *testing.T) {
	a, tt := heroTester(t)
	a.applySelection("r1", a.root().Versions[len(a.root().Versions)-1].ID)
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	a.applySelection("r1", "1.21.8")
	tt.Frame()
	first := image.NewRGBA(tt.Image().Bounds())
	copy(first.Pix, tt.Image().Pix)
	tt.Advance(400 * time.Millisecond)
	ref := tt.Image()
	// Only look for outgoing title pixels in the empty area to the right of
	// the current short title; any such pixel proves an old copy was kept.
	r, _ := tt.Find("版本标题")
	for y := int(r.Y); y < int(r.Y+32); y++ {
		for x := int(r.X + 300); x < int(r.X+r.W)-10; x++ {
			if first.RGBAAt(x, y) != ref.RGBAAt(x, y) {
				t.Fatalf("outgoing title left a painted ghost at %d,%d", x, y)
			}
		}
	}
}

func TestHeroRespectsReducedMotion(t *testing.T) {
	a, tt := heroTester(t)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	a.applySelection("r1", "neoforge-21.1.264")
	tt.Frame()
	first := append([]byte(nil), tt.Image().Pix...)
	tt.Advance(250 * time.Millisecond)
	if !bytes.Equal(first, tt.Image().Pix) {
		t.Fatal("hero moved or faded despite reduced-motion preference")
	}
}

func TestHeroMetadataArrivalKeepsStableSlots(t *testing.T) {
	a, tt := heroTester(t)
	r := *a.root()
	stable := map[string]ui.Rect{}
	for _, label := range []string{"游玩记录", "按钮文字 launch"} {
		bounds, ok := tt.Find(label)
		if !ok {
			t.Fatalf("missing %s", label)
		}
		stable[label] = bounds
	}
	a.instStatusKey = r.ID + "|" + r.SelectedVersion
	a.instStatus = launcher.InstanceStatus{Dir: r.Path, LastLog: time.Now()}
	tt.Frame()
	for label, want := range stable {
		if got, _ := tt.Find(label); got != want {
			t.Fatalf("history result moved %s: %+v -> %+v", label, want, got)
		}
	}
}
