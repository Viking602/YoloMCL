package nativeui

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// Use opaque, square-cornered sources. Already-rounded test artwork conceals
// missing image clipping, especially when an icon arrives after its card.
func TestDiscoverIconRoundsActualImage(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"bitmap", "svg"} {
		for _, dims := range [][2]int{{44, 44}, {88, 44}, {44, 88}} {
			t.Run(fmt.Sprintf("%s-%dx%d", format, dims[0], dims[1]), func(t *testing.T) {
				pixels := image.NewRGBA(image.Rect(0, 0, dims[0], dims[1]))
				draw.Draw(pixels, pixels.Bounds(), image.NewUniform(color.RGBA{20, 100, 210, 255}), image.Point{}, draw.Src)
				var source ui.ImageSource = ui.NewBitmap(pixels)
				if format == "svg" {
					source = ui.MustParseSVG([]byte(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d"><rect width="100%%" height="100%%" fill="#1464d2"/></svg>`, dims[0], dims[1])))
				}
				const raw = "fixture://square-cornered-icon"
				a := &App{images: map[string]ui.ImageSource{}, imagePending: map[string]bool{raw: true}}
				state := &discoverCardMotion{progress: 1}
				project := launcher.Project{Title: "Rounded icon", Icon: raw}
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(theme)
					ui.Box(c).Fill().Background(panel).Padding(20).Children(func() { a.discoverIcon(c, project, state) })
				}, 400, 180)
				tt.Advance(500 * time.Millisecond)
				a.images[raw] = source
				tt.Frame()
				tt.Advance(60 * time.Millisecond)
				if state.iconProgress <= 0 || state.iconProgress >= 1 {
					t.Fatal("late icon no longer animates locally")
				}
				tt.Advance(400 * time.Millisecond)
				box, ok := tt.Find("项目封面 " + project.Title)
				if !ok {
					t.Fatal("icon disappeared")
				}
				scale := min(box.W/float32(dims[0]), box.H/float32(dims[1]))
				w, h := float32(dims[0])*scale, float32(dims[1])*scale
				x, y := box.X+(box.W-w)/2, box.Y+(box.H-h)/2
				// Check the image's four corners, not only the surrounding tile.
				for _, pt := range [][2]float32{{x + 1, y + 1}, {x + w - 2, y + 1}, {x + 1, y + h - 2}, {x + w - 2, y + h - 2}} {
					c := tt.Image().RGBAAt(int(math.Round(float64(pt[0]))), int(math.Round(float64(pt[1]))))
					if c.R < 240 || c.G < 240 || c.B < 240 {
						t.Fatalf("square image corner remains at %v: %v", pt, c)
					}
				}
				center := tt.Image().RGBAAt(int(box.X+box.W/2), int(box.Y+box.H/2))
				if center.B < 180 || center.R > 50 {
					t.Fatalf("rounding lost the image: %v", center)
				}
				snapshotUI(t, tt, fmt.Sprintf("discover-icon-%s-%dx%d", format, dims[0], dims[1]))
			})
		}
	}
}
