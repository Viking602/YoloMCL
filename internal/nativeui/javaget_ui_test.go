package nativeui

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func javaGetTester(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	a := &App{javaGet: javaGetState{major: 21, source: "mojang"}}
	a.OverrideLanguage(LangZH)
	for _, major := range []int{8, 17, 21, 25} {
		for _, source := range []string{"mojang", "temurin", "zulu"} {
			a.javaGet.offers = append(a.javaGet.offers, launcher.JavaOffer{Major: major, Source: source, Version: fmt.Sprintf("%d.0.1", major)})
		}
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Column(c).Fill().Padding(24).Gap(16).Background(bg).Children(func() { a.javaGetDialog(c) })
	}, 720, 660)
	return a, tt
}

// Clicking a row earlier than the current one used to leave the render's pick
// nil and overwrite the user's click with the first source in the catalog.
func TestJavaDownloadSourceSwitchKeepsClickedChoice(t *testing.T) {
	a, tt := javaGetTester(t)
	for _, source := range []string{"zulu", "temurin", "zulu", "mojang", "temurin", "mojang", "zulu", "temurin"} {
		clickUI(t, tt, javaSourceName(a, source))
		if a.javaGet.source != source {
			t.Fatalf("clicked %s, selection jumped to %s", source, a.javaGet.source)
		}
		tt.Advance(300 * time.Millisecond)
		tt.Frame()
		if a.javaGet.source != source {
			t.Fatalf("repaint overwrote %s with %s", source, a.javaGet.source)
		}
	}
}

func TestJavaDownloadVersionAndCatalogKeepAvailableSource(t *testing.T) {
	a, tt := javaGetTester(t)
	clickUI(t, tt, javaSourceName(a, "zulu"))
	for _, major := range []int{8, 25, 17, 21} {
		clickUI(t, tt, fmt.Sprintf("Java %d", major))
		if a.javaGet.major != major || a.javaGet.source != "zulu" {
			t.Fatalf("changing major lost the available source: Java %d / %s", a.javaGet.major, a.javaGet.source)
		}
	}
	// A refresh may finish after a click and return a differently ordered list.
	a.javaGet.offersBusy = true
	clickUI(t, tt, javaSourceName(a, "temurin"))
	slices.Reverse(a.javaGet.offers)
	a.javaGet.offersBusy = false
	tt.Frame()
	if a.javaGet.source != "temurin" {
		t.Fatal("catalog completion reset a still-available user choice")
	}

	// Fall back only when that source really does not offer the new major.
	a.javaGet.offers = slices.DeleteFunc(a.javaGet.offers, func(o launcher.JavaOffer) bool {
		return o.Major == 17 && o.Source != "mojang"
	})
	tt.Frame()
	clickUI(t, tt, "Java 17")
	if a.javaGet.source != "mojang" {
		t.Fatal("unavailable source did not fall back to an installable offer")
	}
	clickUI(t, tt, javaSourceName(a, "temurin"))
	if a.javaGet.source != "mojang" {
		t.Fatal("an unavailable source became selected")
	}
}
