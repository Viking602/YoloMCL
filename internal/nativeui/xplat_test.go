package nativeui

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestTitlebarPerPlatform forces each platform's chrome: macOS leaves the
// traffic lights' room at the left and draws no window buttons of its own;
// Windows/Linux keep min/max/close at the right.
func TestTitlebarPerPlatform(t *testing.T) {
	for _, goos := range []string{"darwin", "windows", "linux"} {
		for _, sc := range []float32{1, 2} {
			t.Setenv("MCL_TITLEBAR_OS", goos)
			a, tt := acctTester(t, "zh-CN", 1280, 800)
			tt.SetScale(sc)
			a.closeDialog()
			tt.Frame()
			tt.Advance(400 * time.Millisecond)
			brand, ok := tt.Find(a.tr().BrandAria)
			if !ok {
				t.Fatal("no brand")
			}
			_, hasClose := tt.Find(a.tr().WinClose)
			if goos == "darwin" {
				if hasClose {
					t.Errorf("%s: custom window buttons drawn on macOS", goos)
				}
				if brand.X < macTrafficInset {
					t.Errorf("%s %gx: brand at %v overlaps traffic lights", goos, sc, brand.X)
				}
			} else {
				if !hasClose {
					t.Errorf("%s: window buttons missing", goos)
				}
				if brand.X > 40*sc {
					t.Errorf("%s: brand inset %v", goos, brand.X)
				}
			}
			if shotsOn() {
				snapshotUI(t, tt, fmt.Sprintf("xplat_%s_%gx", goos, sc))
			}
		}
	}
	_ = ui.Rect{}
}

func TestUIFontFamilyPerOS(t *testing.T) {
	for goos, want := range map[string]string{"darwin": "PingFang SC", "windows": "Microsoft YaHei UI", "linux": "Noto Sans CJK SC"} {
		if f := uiFontFamily(goos); f[:len(uiMonoFamily)] != uiMonoFamily || !contains(f, want) {
			t.Errorf("%s: %q", goos, f)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
