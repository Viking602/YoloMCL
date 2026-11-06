package nativeui

import "github.com/egoist/mygo/ui"

// Design tokens shared by every screen. Values are DIPs on the 1536×1024
// reference canvas and are multiplied by uiScale at the call site, so one
// scale keeps spacing, radii and type in proportion at every window size.
// uiDensity scales the reference canvas; 1.0 keeps full breathing room.
const uiDensity float32 = 1.0

const (
	// Spacing follows a 4-DIP grid with generous room between groups.
	space1 float32 = 4
	space2 float32 = 8
	space3 float32 = 12
	space4 float32 = 16
	space5 float32 = 20
	space6 float32 = 24
	space8 float32 = 32

	// Corner radii on a 4-DIP grid. Nest with nestedRadius(outer, pad):
	// an inner box flush to outer padding p inside radius R gets R−p
	// (floored at radiusMin). Same-level boxes share one token.
	radiusMin     float32 = 4  // floor; never go below
	radiusChip    float32 = 8  // chips, tags, small nested
	radiusControl float32 = 12 // buttons, inputs; independent of label font size
	radiusRow     float32 = 12 // list / selectable rows (same level as controls)
	radiusPanel   float32 = 16 // raised inner panels / sections
	radiusCard    float32 = 16 // cards (same level as panel)
	radiusChrome  float32 = 24 // floating chrome / large overlays
	radiusDialog  float32 = 32 // modal dialogs
	previewPad    float32 = 12 // preview frame: radiusChrome - previewPad = radiusControl

	// Type scale: crisp desktop hierarchy that leaves room around controls.
	typeHero     float32 = 26
	typeDisplay  float32 = 24
	typeTitle1   float32 = 20
	typeTitle2   float32 = 17.5
	typeTitle    float32 = 16
	typeHeadline float32 = 15.5
	typeSubhead  float32 = 15
	typeBody     float32 = 14.5
	typeCallout  float32 = 14
	typeFootnote float32 = 13
	typeCaption  float32 = 12
	typeCaption2 float32 = 11

	// Page chrome: generous outer gutter and vertical rhythm.
	pageGutter    float32 = 32    // horizontal page padding
	pageTop       float32 = 28    // bar → page title
	pageHeaderGap float32 = 24    // page header → first content & between sections
	barHeight     float32 = 48    // title bar + navigation
	radiusPill    float32 = 10000 // fully round sentinel; renderer clamps to half the short edge
)

// nestedRadius returns outer−padding for concentric corners (R−p),
// floored at radiusMin and snapped down onto the 4-DIP grid.
func nestedRadius(outer, pad float32) float32 {
	r := outer - pad
	if r < radiusMin {
		return radiusMin
	}
	return float32(int(r/4)) * 4
}

// MyGo 0.7.4 enables native continuous corners on macOS. Its Windows D3D11
// and Linux GL shaders do not support them, so ordinary radii remain the
// progressive fallback there. Forcing ui/paint.go's private switch would send
// unsupported negative radii to those shaders; it is not a safe enhancement.
// Keep fills, borders, shadows and clips on the same native geometry path.

// iconRadius gives square artwork and icon tiles one shared set of radii,
// instead of a different percentage for each screen. Size is already scaled.
func iconRadius(size, s float32) float32 {
	r := radiusDialog
	switch {
	case size <= 24*s:
		r = radiusMin
	case size <= 36*s:
		r = radiusChip
	case size <= 56*s:
		r = radiusControl
	case size <= 80*s:
		r = radiusCard
	case size <= 112*s:
		r = radiusChrome
	}
	return r * s
}

func elevate2(e ui.Element, s float32) ui.Element {
	return e.Shadow(0, 4*s, 10*s, -3*s, ui.RGBA(0, 0, 0, .09))
}

// Button size tokens (unscaled px; multiply by uiScale). Heights sit on the
// 4 px grid and icon buttons are squares of the same height:
//
//	small  28: toolbars, secondary/row actions, icon close buttons
//	medium 32: primary actions in cards and dialogs, text fields beside them
//	large  36: the hero launch row only
//
// Every size keeps radiusControl (12); nested content follows nestedRadius.
const (
	btnSm  float32 = 28
	btnMd  float32 = 32
	btnLg  float32 = 36
	btnPad float32 = 16 // medium horizontal padding, for hand-built buttons
	btnGap float32 = 8
)

// buttonSize is the label font, horizontal padding, icon size, gap and
// minimum width that go with one button height.
type buttonSize struct {
	height, font, pad, icon, gap, minWidth float32
}

var buttonSizes = [...]buttonSize{
	{height: btnSm, font: 12, pad: 12, icon: 14, gap: 4, minWidth: 56},
	{height: btnMd, font: 13, pad: 16, icon: 16, gap: 8, minWidth: 64},
	{height: btnLg, font: 14, pad: 16, icon: 16, gap: 8, minWidth: 80},
}

// buttonSizeFor snaps an unscaled height to the nearest size token, so a
// stray literal still renders with a token's font and padding.
func buttonSizeFor(height float32) buttonSize {
	best := buttonSizes[0]
	for _, b := range buttonSizes[1:] {
		if abs32(height-b.height) < abs32(height-best.height) {
			best = b
		}
	}
	return best
}
