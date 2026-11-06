package nativeui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestVersionTagsAreSuperscripts(t *testing.T) {
	for _, sc := range []float32{1, 2} {
		a, tt := acctTester(t, "zh-CN", 1280, 800)
		tt.SetScale(sc)
		a.closeDialog()
		tt.Frame()
		tt.Advance(500 * time.Millisecond)
		card, ok := tt.Find("主版本卡")
		tag, ok2 := tt.Find("Java 标签 21")
		if !ok || !ok2 {
			t.Fatal("featured card / tag missing")
		}
		// Font line metrics differ across platforms, including fractional DIPs.
		// Keep the badge at its caption size rather than assuming a 20-DIP box.
		s := uiScaleFor(1280, 800) * uiDensity
		_, captionH := ui.ShapeText("Java 21", ui.Font{Family: theme.Font, Size: typeCaption2 * s, Weight: 600}, 0).Size()
		if tag.Y > card.Y+card.H/2 || tag.H > captionH+2*(space1+1)*s+.5 {
			t.Errorf("%.0fx: tag should be small and in the title line: tag %+v card %+v", sc, tag, card)
		}
	}
}
