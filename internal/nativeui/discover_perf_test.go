package nativeui

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestDiscoverLiftKeepsSoftwareDrawingUntransformed(t *testing.T) {
	for _, mode := range []discoverReveal{discoverLift, discoverExpand} {
		t.Run(map[discoverReveal]string{discoverLift: "initial", discoverExpand: "after-filter"}[mode], func(t *testing.T) {
			testDiscoverLift(t, mode)
		})
	}
}

func testDiscoverLift(t *testing.T, mode discoverReveal) {
	t.Helper()
	a := &App{}
	a.discoverMotion.viewport = ui.Rect{W: 600, H: 400}
	if mode == discoverExpand {
		a.discoverMotion.scroll.st.Y = 200
	}
	state := &discoverCardMotion{mode: mode}
	var layout ui.Rect
	var x, y float32
	tt := ui.NewTester(func(c *ui.Context) {
		card := ui.Box(c.Key("card")).Absolute().Left(100).Top(80).Size(300, 196)
		a.animateDiscoverCard(c, card, state, 0, 1)
		layout = card.LayoutBounds()
		x, y = card.LocalToWindow(0, 0)
	}, 600, 400)
	tt.Advance(100 * time.Millisecond)
	if state.progress <= 0 || state.progress >= 1 || layout.Y <= 80 {
		t.Fatal("card did not retain its animated entrance")
	}
	if x != layout.X || y != layout.Y {
		t.Fatal("card lift transformed its subtree instead of moving its layout")
	}
	tt.Advance(400 * time.Millisecond)
	if state.progress != 1 || math.Abs(float64(layout.Y-80)) > .01 {
		t.Fatalf("card kept moving after its reveal ended: progress=%g y=%g", state.progress, layout.Y)
	}
}

// Exercise the software painter, including rounded cards, text and icons,
// while new rows enter the viewport. Network and file reads are excluded.
func BenchmarkDiscoverScroll(b *testing.B) {
	if err := loadAssets(); err != nil {
		b.Fatal(err)
	}
	icon, err := ui.DecodeBitmap(BrandIconPNG())
	if err != nil {
		b.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		scale float32
		mode  discoverReveal
	}{{"1x", 1, discoverLift}, {"1.5x", 1.5, discoverLift}, {"filtered-1.5x", 1.5, discoverExpand}} {
		b.Run(tc.name, func(b *testing.B) {
			a := &App{provider: "modrinth", resourceType: "modpack", images: map[string]ui.ImageSource{"scroll-icon": icon}}
			a.settings.Language = LangZH
			projects := discoverMotionProjects(0, 600)
			for i := range projects {
				projects[i].Icon = "scroll-icon"
			}
			a.acceptDiscoverResults(launcher.SearchQuery{Source: "modrinth", Type: "modpack"}, launcher.SearchResult{Projects: projects, Total: len(projects), Limit: len(projects)})
			a.discoverMotion.mode = tc.mode
			frames := 0
			var start time.Time
			tt := ui.NewTester(func(c *ui.Context) {
				frames++
				c.SetTheme(theme)
				if !start.IsZero() {
					// Cross rows continuously, then repeat without growing the feed.
					y := float32(c.Now().Sub(start).Milliseconds()%20000) * 1.2
					a.discoverMotion.scroll.st.ScrollTo(0, y, ui.Animation{Duration: time.Nanosecond})
					c.AnimationFrame()
				}
				ui.Column(c).Fill().Background(chromeBg).Children(func() { a.discoverView(c) })
			}, 1536, 1024)
			tt.SetScale(tc.scale)
			tt.Advance(time.Second)
			start = tt.Now()
			tt.Frame()
			before := frames
			b.ReportAllocs()
			for b.Loop() {
				tt.Advance(time.Second / 60)
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(frames-before), "ns/frame")
		})
	}
}
