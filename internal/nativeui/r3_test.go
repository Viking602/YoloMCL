package nativeui

import (
	"fmt"
	"testing"
	"time"
)

// At scroll end the last row is fully visible above the launch bar and any
// page notice, at minimum size, 1x and 2x.
func TestR3LastRowVisibleAtScrollEnd(t *testing.T) {
	for _, c := range []struct {
		n     string
		w, h  int
		scale float32
	}{{"min", 900, 540, 1}, {"1x", 1280, 800, 1}, {"2x", 1280, 800, 2}} {
		for _, notice := range []bool{false, true} {
			a, tt := acctTester(t, "en-US", c.w, c.h)
			tt.SetScale(c.scale)
			namesFixture(a)
			a.closeDialog()
			if notice {
				a.message, a.errorMessage = "Download failed: connection timed out", true
				a.noticeFor, a.noticeRetry = a.message, func() {}
			}
			tt.Frame()
			tt.Advance(600 * time.Millisecond)
			for i := 0; i < 30; i++ {
				tt.Scroll(float32(c.w)/2, float32(c.h)/2, 0, 400)
				tt.Advance(50 * time.Millisecond)
			}
			tt.Advance(900 * time.Millisecond)
			vs := a.snapshot.Instances[len(a.snapshot.Instances)-1].Versions
			last, ok := tt.Find("版本卡片 " + vs[len(vs)-1].ID)
			dock, ok2 := tt.Find("启动栏")
			if !ok || !ok2 {
				t.Fatalf("%s: last card/dock missing", c.n)
			}
			limit := dock.Y
			if notice {
				if toast, ok := tt.Find("页面通知"); ok && toast.Y < limit {
					limit = toast.Y
				}
			}
			if last.Y+last.H > limit+0.5 {
				t.Errorf("%s notice=%v: last row bottom %.0f below %.0f", c.n, notice, last.Y+last.H, limit)
			}
			if shotsOn() && c.n == "min" {
				snapshotUI(t, tt, fmt.Sprintf("r3_after_scrollend_notice%v_en_min", notice))
			}
		}
	}
}
