package nativeui

import (
	"testing"
	"time"
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
		if tag.Y > card.Y+card.H/2 || tag.H > 20 { // top in the title line (taller mono title line)
			t.Errorf("%.0fx: tag should be small and in the title line: tag %+v card %+v", sc, tag, card)
		}
	}
}
