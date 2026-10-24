package nativeui

import (
	"bytes"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestWardrobeWheelZoomAndOrbit(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	a := &App{preview: newSkinPreview()}
	a.OverrideLanguage(LangZH)
	var scroll ui.ScrollState
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Scroll(c).Fill().Background(chromeBg).TrackScroll(&scroll).Children(func() {
			ui.Column(c).FillWidth().Padding(20).Children(func() {
				ui.Row(c).FillWidth().Children(func() { a.wardrobeStage(c, 540) })
				ui.Box(c).Height(700).FillWidth()
			})
		})
	}, 900, 600)
	p := a.preview
	r, ok := tt.Find(a.tr().WardrobeOrbitAria)
	if !ok || scroll.MaxY <= 0 {
		t.Fatal("missing preview or scrollable page")
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	initial := append([]byte(nil), tt.Image().Pix...)
	tt.Scroll(x, y, 0, -40)
	tt.Advance(65 * time.Millisecond)
	if p.zoom <= 1 || p.zoom >= p.zoomGoal || scroll.Y != 0 {
		t.Fatalf("wheel did not smoothly zoom inside the preview: zoom=%g target=%g scroll=%g", p.zoom, p.zoomGoal, scroll.Y)
	}
	tt.Advance(160 * time.Millisecond)
	if math.Abs(p.zoom-1.1) > .0001 || bytes.Equal(initial, tt.Image().Pix) {
		t.Fatal("wheel zoom did not finish or repaint the character")
	}
	snapshotUI(t, tt, "wardrobe-wheel-zoom")
	clickUI(t, tt, a.tr().WardrobeZoomAria+" −")
	tt.Advance(200 * time.Millisecond)
	if math.Abs(p.zoom-1) > .0001 {
		t.Fatal("zoom button lost the wheel's current zoom")
	}
	tt.Scroll(x, y, 0, .5)
	if p.zoomGoal >= 1 || p.zoomGoal < .99 {
		t.Fatal("fractional touchpad movement was discarded or exaggerated")
	}
	for _, bound := range []struct{ delta, want float64 }{{-10000, 1.4}, {10000, .65}} {
		tt.Scroll(x, y, 0, float32(bound.delta))
		tt.Advance(200 * time.Millisecond)
		tt.Scroll(x, y, 0, float32(bound.delta))
		if math.Abs(p.zoom-bound.want) > .0001 || p.zoomGoal != bound.want || scroll.Y != 0 {
			t.Fatalf("zoom escaped bounds or scrolled the page at a limit: zoom=%g target=%g scroll=%g", p.zoom, p.zoomGoal, scroll.Y)
		}
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Scroll(x, y, 0, -40)
	if math.Abs(p.zoom-p.zoomGoal) > .0001 {
		t.Fatal("reduced motion still animates zoom")
	}
	yaw, pitch := p.yaw, p.pitch
	tt.Press(x, y)
	tt.Move(x+30, y+20)
	tt.Release(x+30, y+20)
	if p.yaw == yaw || p.pitch == pitch {
		t.Fatal("wheel handler intercepted drag rotation")
	}
	goal := p.zoomGoal
	tt.Scroll(800, 300, 0, 100)
	if p.zoomGoal != goal || scroll.Y <= 0 {
		t.Fatal("scrolling outside the preview zoomed the character or stopped page scrolling")
	}
}
