package nativeui

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// TestFontShots renders home, settings, a dialog and mixed CJK/Latin rows
// with the current UI font, at 1x and 2x (MCL_FONT_SHOT=before|after).
func TestFontShots(t *testing.T) {
	tag := os.Getenv("MCL_FONT_SHOT")
	if tag == "" || !shotsOn() {
		t.Skip("screenshots only")
	}
	for _, sc := range []float32{1, 2} {
		for _, page := range []string{"home", "settings", "dialog", "mixed"} {
			a, tt := acctTester(t, "zh-CN", 1280, 800)
			tt.SetScale(sc)
			if page == "mixed" {
				namesFixture(a)
			}
			a.closeDialog()
			switch page {
			case "settings":
				a.goPage("settings")
			case "dialog":
				a.openVersionConfig(a.snapshot.ActiveRoot, a.snapshot.Instances[0].SelectedVersion)
			}
			tt.Frame()
			tt.Advance(700 * time.Millisecond)
			snapshotUI(t, tt, fmt.Sprintf("font_%s_%s_%gx", tag, page, sc))
		}
	}
}
