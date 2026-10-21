package nativeui

import (
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestThumbGeometry(t *testing.T) {
	r := ui.Rect{X: 0, Y: 0, W: 300, H: 400}
	g := thumbRect(r, 0, 100000, 0)
	if g.H != scrollThumbMin || g.W != scrollThumbIdle || g.X+g.W != r.W-scrollEdgeMargin {
		t.Fatalf("min length / right inset: %+v", g)
	}
	end := thumbRect(r, 100000, 100000, 1)
	if end.W != scrollThumbHot || end.Y+end.H > r.H-1 {
		t.Fatalf("hot width / end position: %+v", end)
	}
}

// thumbAlpha reads how dark the overlay thumb strip is at (x, y).
func darkness(tt *ui.Tester, x, y float32, scale float32) int {
	img := tt.Image()
	c := img.RGBAAt(int(x*scale), int(y*scale))
	return 765 - int(c.R) - int(c.G) - int(c.B)
}

func TestOverlayScrollbarFades(t *testing.T) {
	for _, sc := range []float32{1, 2} {
		n := map[float32]string{1: "1x", 2: "2x"}[sc]
		a, tt := acctTester(t, "zh-CN", 1536, 1024)
		tt.SetScale(sc)
		p := changePlanFixture()
		a.openPackPlan(p, false)
		a.packExpand = map[string]bool{"overwritten": true, "updated": true, "added": true, "removed": true, "manual": true}
		tt.Advance(600 * time.Millisecond)
		box, ok := tt.Find("活动弹窗")
		if !ok {
			t.Fatal("dialog missing")
		}
		shot := func(name string) {
			if os.Getenv("MCL_NATIVE_QA_OUTPUT") != "" {
				snapshotUI(t, tt, "scroll_"+name+"_"+n)
			}
		}
		sc2, ok := tt.Find("弹窗内容")
		if !ok {
			t.Fatal("dialog scroll missing")
		}
		stripX := sc2.X + sc2.W - scrollEdgeMargin - 2
		midY := sc2.Y + sc2.H/2
		idle := darkness(tt, stripX, midY, sc)
		shot("idle")
		tt.Scroll(box.X+box.W/2, midY, 0, 120)
		tt.Advance(120 * time.Millisecond)
		shot("scrolling")
		tt.Advance(2500 * time.Millisecond)
		after := darkness(tt, stripX, midY, sc)
		if after > idle+3 {
			t.Errorf("%s: thumb should auto-hide (idle %d, after %d)", n, idle, after)
		}
		tt.Move(stripX-40, midY)
		tt.Advance(50 * time.Millisecond)
		tt.Move(stripX, midY)
		tt.Advance(300 * time.Millisecond)
		shot("hover")
		if hov := darkness(tt, stripX, midY, sc); hov < idle+15 {
			t.Errorf("%s: thumb should show near the edge (idle %d, hover %d)", n, idle, hov)
		}
	}
}
