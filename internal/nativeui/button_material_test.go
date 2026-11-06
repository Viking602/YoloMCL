package nativeui

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

var materialNames = [...]string{"Primary", "Secondary", "Tertiary", "Ghost", "Text"}

// Drive the real ButtonBase: the material must never move its neighbours,
// swallow keyboard actions, activate a disabled action or keep rendering idle.
func TestButtonMaterialInteraction(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	for variant, name := range materialNames {
		for _, height := range []float32{btnMd, btnLg} {
			t.Run(fmt.Sprintf("%s/%.1f", name, height), func(t *testing.T) {
				clicks, disabledClicks := 0, 0
				// Callers pass scaled heights; a 1536-wide window is scale 1.
				scale := uiDensity
				height := height * scale
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(theme)
					ui.Column(c).Fill().Padding(24).Gap(30).Background(bg).Children(func() {
						if fixedVariantButton(c, "action", name, "plus", 0, height, buttonVariant(variant), false) {
							clicks++
						}
						if fixedVariantButton(c, "disabled", "Disabled", "", 0, height, buttonVariant(variant), true) {
							disabledClicks++
						}
					})
				}, 1536, 1024)
				tt.Advance(200 * time.Millisecond)
				rest := addonFilterBounds(t, tt, name)
				neighbour := addonFilterBounds(t, tt, "Disabled")
				if math.Abs(float64(rest.H-height)) > .01 {
					t.Fatalf("size changed: %+v", rest)
				}
				x, y := rest.X+rest.W/2, rest.Y+rest.H/2
				tt.Move(x, y)
				tt.Advance(200 * time.Millisecond)
				tt.Press(x, y)
				tt.Advance(60 * time.Millisecond)
				middle := addonFilterBounds(t, tt, name)
				tt.Advance(150 * time.Millisecond)
				pressed := addonFilterBounds(t, tt, name)
				u := scale // material lengths scale with the UI, not the height
				if math.Abs(float64(pressed.W-rest.W)) > .01 || math.Abs(float64(pressed.H-rest.H)) > .01 || math.Abs(float64(pressed.Y-rest.Y-u)) > .02 || middle.Y < rest.Y || middle.Y > pressed.Y {
					t.Fatalf("press must animate visually without resizing: rest=%+v middle=%+v pressed=%+v", rest, middle, pressed)
				}
				if got := addonFilterBounds(t, tt, "Disabled"); got != neighbour {
					t.Fatalf("pressed button moved its neighbour: %+v -> %+v", neighbour, got)
				}
				if variant < int(buttonGhost) {
					img := tt.Image()
					top := img.RGBAAt(int(pressed.X+pressed.W/2), int(pressed.Y+6*u))
					bottom := img.RGBAAt(int(pressed.X+pressed.W/2), int(pressed.Y+pressed.H-6*u))
					if int(top.R)+int(top.G)+int(top.B) >= int(bottom.R)+int(bottom.G)+int(bottom.B) {
						t.Fatalf("pressed face must be darker at the top: %v / %v", top, bottom)
					}
				}
				snapshotUI(t, tt, fmt.Sprintf("button-%s-%.0f-pressed", name, height))
				tt.Release(x, y)
				tt.Advance(200 * time.Millisecond)
				if clicks != 1 || addonFilterBounds(t, tt, name) != rest {
					t.Fatalf("release did not restore the button and activate once: clicks=%d", clicks)
				}
				tt.Key(0, ui.KeyEnter)
				if clicks != 2 {
					t.Fatal("native Enter activation lost", clicks)
				}
				tt.Key(0, ui.KeyTab)
				if !tt.Focused("Disabled") {
					t.Fatal("disabled reason is no longer keyboard reachable")
				}
				tt.Key(0, ui.KeyEnter)
				clickUI(t, tt, "Disabled")
				if disabledClicks != 0 {
					t.Fatal("disabled action fired")
				}
				tt.Key(0, ui.KeyTab)
				if !tt.Focused(name) {
					t.Fatal("keyboard traversal did not return to button")
				}
				tt.Move(1500, 1000)
				tt.Advance(300 * time.Millisecond)
				if tt.FramePending() {
					t.Fatal("settled controls keep requesting frames")
				}
				snapshotUI(t, tt, fmt.Sprintf("button-%s-%.0f-focus", name, height))
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				tt.Move(x, y)
				tt.Press(x, y)
				first := addonFilterBounds(t, tt, name)
				if math.Abs(float64(first.Y-rest.Y-u)) > .02 {
					t.Fatal("reduced motion did not immediately apply press offset")
				}
				tt.Advance(200 * time.Millisecond)
				// MyGo retains colour transitions in reduced-motion mode;
				// geometry snaps immediately, and every transition must settle.
				if tt.FramePending() || first != addonFilterBounds(t, tt, name) {
					t.Fatal("reduced motion still moves button or schedules idle frames")
				}
			})
		}
	}
}

func TestButtonMaterialGallery(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Column(c).Fill().Padding(40).Gap(25).Background(bg).Font(uiSansFamily()).Children(func() {
			ui.Text(c, "YoloMCL / Controls").FontSize(25).FontWeight(600).TextColor(ink)
			ui.Text(c, "Blue accents · sm 28 · md 32 · lg 36 · disabled").FontSize(14).TextColor(muted)
			for variant, name := range materialNames {
				ui.Row(c).Gap(30).AlignItems(ui.Center).Children(func() {
					ui.Text(c, name).Width(100).FontSize(14).TextColor(muted)
					fixedVariantButton(c, name+"-sm", "Continue", "plus", 0, btnSm*uiScale(c), buttonVariant(variant), false)
					fixedVariantButton(c, name+"-md", "Continue", "plus", 150, btnMd*uiScale(c), buttonVariant(variant), false)
					fixedVariantButton(c, name+"-lg", "Continue", "plus", 175, btnLg*uiScale(c), buttonVariant(variant), false)
					fixedVariantButton(c, name+"-disabled", "Continue", "plus", 150, btnMd*uiScale(c), buttonVariant(variant), true)
				})
			}
		})
	}, 820, 530)
	tt.Advance(300 * time.Millisecond)
	snapshotUI(t, tt, "button-materials")
}
