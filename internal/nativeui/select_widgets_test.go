package nativeui

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestDropdownLoaderListFitsTrigger(t *testing.T) {
	for _, lang := range []string{LangZH, LangEN} {
		for _, size := range [][2]int{{1536, 1024}, {900, 600}} {
			t.Run(fmt.Sprintf("%s-%d", lang, size[0]), func(t *testing.T) {
				a, tt := homeTester(t, lang)
				tt.SetSize(size[0], size[1])
				tt.Frame()
				tt.Advance(500 * time.Millisecond)
				label := a.L("instance_rail.filter_by_loader")
				clickUI(t, tt, label)
				tt.Advance(300 * time.Millisecond)
				trigger, ok := tt.Find(label)
				if !ok {
					t.Fatal("loader filter trigger disappeared")
				}
				row, ok := tt.Find(label + " " + a.L("instance_rail.all_loaders"))
				if !ok {
					t.Fatal("loader options did not open")
				}
				snapshotUI(t, tt, fmt.Sprintf("dropdown-loaders-%s-%d", lang, size[0]))
				left, right := row.X-trigger.X, trigger.X+trigger.W-row.X-row.W
				if math.Abs(float64(left-right)) > 1 {
					t.Errorf("short list has an empty scrollbar gutter: left %.2f, right %.2f", left, right)
				}
				forge, ok := tt.Find(label + " Forge")
				if !ok {
					t.Fatal("Forge option missing")
				}
				tt.Move(forge.X+forge.W/2, forge.Y+forge.H/2)
				tt.Advance(250 * time.Millisecond)
				if a.instLoader != "" {
					t.Fatal("hover committed a loader filter")
				}
				snapshotUI(t, tt, fmt.Sprintf("dropdown-loaders-hover-%s-%d", lang, size[0]))
				tt.Key(0, ui.KeyEscape)
				if _, ok := tt.Find(label + " Forge"); ok {
					t.Fatal("Escape left the options interactive")
				}
			})
		}
	}
}

func TestDropdownLongListStaysReachable(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	for _, bottom := range []bool{false, true} {
		t.Run(fmt.Sprintf("bottom-%t", bottom), func(t *testing.T) {
			options := make([]string, 40)
			for i := range options {
				options[i] = fmt.Sprintf("Minecraft 1.%d — 长名称版本选项", i)
			}
			selected := options[0]
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme)
				ui.Column(c).Fill().Padding(16).Background(bg).Children(func() {
					if bottom {
						ui.Box(c).Grow(1)
					}
					selectBox(c, "long-list", "Long list", &selected, options).FillWidth()
				})
			}, 320, 240)
			clickUI(t, tt, "Long list")
			tt.Advance(300 * time.Millisecond)
			first, ok := tt.Find("Long list " + options[0])
			if !ok || first.X < 0 || first.X+first.W > 320 || first.Y < 4 || first.Y+first.H > 236 {
				t.Fatalf("first option outside the window: %+v", first)
			}
			tt.Key(0, ui.KeyEnd)
			tt.Frame()
			tt.Advance(300 * time.Millisecond)
			last, ok := tt.Find("Long list " + options[len(options)-1])
			if !ok || last.Y < 4 || last.Y+last.H > 236 {
				t.Fatalf("keyboard could not reveal the last option: %+v", last)
			}
			snapshotUI(t, tt, fmt.Sprintf("dropdown-long-bottom-%t", bottom))
			tt.Key(0, ui.KeyEnter)
			if selected != options[len(options)-1] {
				t.Fatal("Enter did not commit the highlighted option")
			}
			if !tt.Focused("Long list") {
				t.Fatal("selection lost focus from the trigger")
			}
			// Reusing the same control after narrowing its options must drop
			// the old gutter, even without entrance animation frames.
			options = options[:2]
			selected = options[0]
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			clickUI(t, tt, "Long list")
			trigger, _ := tt.Find("Long list")
			first, _ = tt.Find("Long list " + options[0])
			left, right := first.X-trigger.X, trigger.X+trigger.W-first.X-first.W
			if math.Abs(float64(left-right)) > 1 {
				t.Fatal("a narrowed list retained its previous scrollbar gutter")
			}
			tt.Key(0, ui.KeyEscape)
		})
	}
}

func TestDropdownTooltipShowsOnlyClippedDisplayName(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	selected := "opaque-internal-id"
	display := "Fabric"
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Column(c).Fill().Padding(24).Background(bg).Children(func() {
			selectBoxFull(c, "display-name", "Display name", &selected, []string{selected}, nil, func(string) string { return display }).Width(180)
		})
	}, 360, 300)
	tt.Advance(time.Second)
	trigger, _ := tt.Find("Display name")
	tt.Move(trigger.X+50, trigger.Y+trigger.H/2)
	tt.Advance(650 * time.Millisecond)
	if _, ok := tt.Find("文本提示"); ok {
		t.Error("a fitting display name shows an unnecessary tooltip")
	}
	display = strings.Repeat("中文 Minecraft 版本名称", 3)
	tt.Move(0, 0)
	tt.Frame()
	tt.Frame()
	tt.Move(trigger.X+50, trigger.Y+trigger.H/2)
	tt.Advance(650 * time.Millisecond)
	if _, ok := tt.Find("文本提示"); !ok || !tt.HasText(display) {
		t.Error("clipped display name did not expose its full text")
	}
	snapshotUI(t, tt, "dropdown-truncated-label")
}
