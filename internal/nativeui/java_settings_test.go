package nativeui

import (
	"bytes"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestJavaPickerIconsAndSelection(t *testing.T) {
	for _, lang := range []string{LangZH, LangEN} {
		for _, width := range []int{420, 900} {
			t.Run(fmt.Sprintf("%s-%d", lang, width), func(t *testing.T) {
				if err := loadAssets(); err != nil {
					t.Fatal(err)
				}
				a := &App{}
				a.OverrideLanguage(lang)
				a.java = []launcher.JavaInfo{
					{Path: filepath.Join("runtimes", "zulu-21", "bin", "java"), Vendor: "Azul Systems", Version: "21.0.10"},
					{Path: filepath.Join("runtimes", "jdk-21", "bin", "java"), Vendor: "Eclipse Adoptium", Version: "21.0.10"},
					{Path: filepath.Join("portable", "jdk-21", "bin", "java"), Vendor: "Eclipse Adoptium", Version: "21.0.10"},
				}
				manual := filepath.Join("portable", strings.Repeat("long-directory-", 12), "bin", "java")
				options := []string{a.tr().SettingsJavaAuto, "Java 21.0.10 Zulu", "Java 21.0.10 Temurin", "Java 21.0.10 Temurin / portable", manual}
				paths := map[string]string{options[0]: "", options[1]: a.java[0].Path, options[2]: a.java[1].Path, options[3]: a.java[2].Path, manual: manual}
				selected := options[0]
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(theme)
					ui.Column(c).Fill().Padding(24).Background(bg).Children(func() {
						a.javaSelectBox(c, "java-picker-test", "Java picker", &selected, options, paths).FillWidth()
					})
				}, width, 500)
				for i, vendor := range []string{"Zulu", "Temurin", "Temurin", "OpenJDK", ""} {
					clickUI(t, tt, "Java picker")
					tt.Advance(250 * time.Millisecond)
					choice := options[(i+1)%len(options)]
					row, ok := tt.Find("Java picker " + choice)
					if !ok || row.X < 0 || row.X+row.W > float32(width) {
						t.Fatalf("option missing or overflowing window: %+v", row)
					}
					clickUI(t, tt, "Java picker "+choice)
					tt.Advance(250 * time.Millisecond)
					if selected != choice {
						t.Fatalf("selected %q instead of %q", selected, choice)
					}
					icon := "Java 品牌图标 " + vendor
					if vendor == "" {
						icon = "Java 自动选择图标"
					}
					r, ok := tt.Find(icon)
					trigger, _ := tt.Find("Java picker")
					if !ok || r.W <= 0 || r.X < trigger.X || r.X+r.W > trigger.X+trigger.W || r.Y < trigger.Y || r.Y+r.H > trigger.Y+trigger.H {
						t.Fatalf("selected runtime lost its icon: %s %+v", icon, r)
					}
					if _, ok := tt.Find("Java picker " + choice); ok {
						t.Fatal("choosing a runtime left the menu open")
					}
				}
				clickUI(t, tt, "Java picker")
				tt.Advance(250 * time.Millisecond)
				snapshotUI(t, tt, fmt.Sprintf("java-picker-%s-%d", lang, width))
				tt.Key(0, ui.KeyDown)
				tt.Key(0, ui.KeyEnter)
				if paths[selected] != a.java[0].Path {
					t.Fatal("keyboard selection no longer chooses the corresponding executable")
				}
			})
		}
	}
}

func TestJavaPickerOpeningAndDismissal(t *testing.T) {
	a, tt := settingsTester(t)
	path := filepath.Join(t.TempDir(), "temurin-21", "bin", "java")
	a.java = []launcher.JavaInfo{
		{Path: filepath.Join("runtimes", "zulu-26", "bin", "java"), Version: "26.0.2", Vendor: "Azul Systems"},
		{Path: filepath.Join("runtimes", "zulu-25", "bin", "java"), Version: "25.0.2", Vendor: "Azul Systems"},
		{Path: filepath.Join("runtimes", "zulu-21", "bin", "java"), Version: "21.0.10", Vendor: "Azul Systems"},
		{Path: path, Version: "21.0.10", Vendor: "Eclipse Adoptium"},
	}
	tt.Frame()
	tt.Advance(time.Second)
	label := a.tr().SettingsJavaAria
	option := label + " " + a.tr().SettingsJavaAuto
	for _, reduced := range []bool{false, true} {
		tt.SetPreferences(ui.Preferences{ReduceMotion: reduced})
		for _, dismissal := range []string{"escape", "outside"} {
			clickUI(t, tt, label)
			tt.Advance(55 * time.Millisecond)
			mid, ok := tt.Find(option)
			if !ok {
				t.Fatal("opening did not build the choices")
			}
			pixels := overviewRegionPixels(tt.Image(), mid)
			tt.Advance(250 * time.Millisecond)
			end, _ := tt.Find(option)
			if math.Abs(float64(mid.W-end.W)) > .01 || math.Abs(float64(mid.H-end.H)) > .01 {
				t.Fatal("opening rescaled the text and shadowed menu")
			}
			if reduced {
				if mid != end {
					t.Fatal("reduced motion still moves the menu")
				}
			} else if mid.Y >= end.Y || end.Y-mid.Y > 6 || bytes.Equal(pixels, overviewRegionPixels(tt.Image(), end)) {
				t.Fatalf("opening lost its gentle slide and fade: %+v -> %+v", mid, end)
			}
			if tt.FramePending() {
				t.Fatal("settled dropdown keeps requesting animation frames")
			}
			if !reduced && dismissal == "escape" {
				snapshotUI(t, tt, "java-picker-global-open")
			}
			if dismissal == "escape" {
				tt.Key(0, ui.KeyEscape)
			} else {
				tt.ClickAt(1400, 1000)
			}
			if _, ok := tt.Find(option); ok {
				t.Fatal("dismissed dropdown remains interactive")
			}
		}
	}
	clickUI(t, tt, label)
	tt.Key(0, ui.KeyEnd)
	tt.Key(0, ui.KeyEnter)
	waitUI(t, tt, func() bool { return a.service.Snapshot().Settings.JavaPath == path })
	if _, ok := tt.Find("Java 品牌图标 Temurin"); !ok {
		t.Fatal("global Java selection did not retain the vendor icon")
	}
	snapshotUI(t, tt, "java-picker-global-selected")
}

func TestPartialJavaRefreshKeepsKnownInstallations(t *testing.T) {
	known := []launcher.JavaInfo{{Path: "java21/bin/java", Version: "21.0.9"}, {Path: "java17/bin/java", Version: "17.0.15"}}
	if got := mergeJavaScanRows(known, nil); len(got) != 2 {
		t.Fatal("partial refresh blanked Java list")
	}
	found := []launcher.JavaInfo{{Path: known[0].Path, Version: "21.0.10"}, {Path: "java25/bin/java", Version: "25.0.1"}}
	got := mergeJavaScanRows(known, found)
	if len(got) != 3 || got[0].Version != "21.0.10" || got[1].Path != known[1].Path || got[2].Path != found[1].Path {
		t.Fatalf("lost/doubled Java during refresh: %+v", got)
	}
	if known[0].Version != "21.0.9" {
		t.Fatal("mutated previous snapshot")
	}
}
