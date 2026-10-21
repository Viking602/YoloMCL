package nativeui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// One motion system for the whole UI. Every animated value goes through
// AnimateTo / Transition, so an interrupted animation continues from where
// it is instead of jumping; the toolkit honours the desktop's reduce-motion
// preference (values snap, nothing slides).
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
	motionEnter = ui.Animation{Duration: motionDurEnter, Ease: easeOutCubic}
	motionExit  = ui.Animation{Duration: motionDurExit, Ease: easeInCubic}
	motionFast  = ui.Animation{Duration: motionDurFast, Ease: easeOutCubic}
	motionMove  = ui.Animation{Duration: motionDurMove, Ease: easeStandard}
)

// navOrder is the top navigation order, used to slide pages in the
// direction of travel.
var navOrder = []string{"home", "mods", "discover", "wardrobe", "tasks", "settings"}

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
