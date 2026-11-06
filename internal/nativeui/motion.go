package nativeui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Shared timing for UI transitions. Seed-map tiles use capped paint-only fades
// and the map camera integrates pointer velocity separately. Both honour the
// desktop's reduce-motion preference, as do AnimateTo / Transition below.
const (
	motionDurEnter = 220 * time.Millisecond // things appearing / moving in
	motionDurExit  = 150 * time.Millisecond // things leaving: quicker
	motionDurFast  = 140 * time.Millisecond // hover / press colours
	motionDurMove  = 240 * time.Millisecond // thumbs, sliders, selection
	motionSlide    = 14                     // DIPs a page / form slides
)

// easeOutCubic decelerates into place (enter); easeInCubic accelerates
// away (exit); easeStandard is a gentle ease-out for moving indicators.
func easeOutCubic(t float32) float32 { u := 1 - t; return 1 - u*u*u }
func easeInCubic(t float32) float32  { return t * t * t }

var easeStandard = ui.CubicBezier(.2, 0, 0, 1)

var (
	motionEnter       = ui.Animation{Duration: motionDurEnter, Ease: easeOutCubic}
	motionExit        = ui.Animation{Duration: motionDurExit, Ease: easeInCubic}
	motionFast        = ui.Animation{Duration: motionDurFast, Ease: easeOutCubic}
	motionMove        = ui.Animation{Duration: motionDurMove, Ease: easeStandard}
	motionPack        = ui.Animation{Duration: 260 * time.Millisecond, Ease: ui.CubicBezier(.22, 1, .36, 1)}
	motionDialogEnter = ui.Animation{Duration: 190 * time.Millisecond, Ease: easeOutCubic}
	motionDialogExit  = ui.Animation{Duration: 130 * time.Millisecond, Ease: easeOutCubic}
)

// The hero always shows the current selection. Keeping outgoing titles around
// makes rapid switches overlap. Fade only the incoming artwork and text,
// in fixed slots; the card surface and launch controls stay in place.
func heroSwap(c *ui.Context) ui.ElementTransition {
	if c.Preferences().ReduceMotion {
		return ui.ElementTransition{Colors: true}
	}
	return ui.ElementTransition{
		Animation: &motionFast, Colors: true,
		Enter: &ui.Motion{Opacity: 0},
	}
}

// navOrder is the top navigation order, used to slide pages in the
// direction of travel.
var navOrder = []string{"home", "discover", "wardrobe", "tasks", "settings"}

func navIndex(page string) int {
	for i, p := range navOrder {
		if p == page {
			return i
		}
	}
	return 0
}

// slideTransition crossfades an element with a short slide along dir
// (±1 on the x axis, or y when vertical) — entering from +dir, leaving
// toward −dir, at a smaller distance so the exit does not distract.
func slideTransition(s float32, dir float32, vertical bool) ui.ElementTransition {
	in, out := &ui.Motion{Opacity: 0}, &ui.Motion{Opacity: 0}
	if vertical {
		in.Y, out.Y = dir*motionSlide*s, -dir*motionSlide*.5*s
	} else {
		in.X, out.X = dir*motionSlide*s, -dir*motionSlide*.5*s
	}
	return ui.ElementTransition{Animation: &motionEnter, Position: false, Colors: true, Enter: in, Exit: out}
}
