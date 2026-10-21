package nativeui

import "github.com/egoist/mygo/ui"

// Design tokens shared by every screen. Values are DIPs on the 1536×1024
// reference canvas and are multiplied by uiScale at the call site, so one
// scale keeps spacing, radii and type in proportion at every window size.
const (
	// Spacing follows a 4-DIP grid.
	space1 float32 = 4
	space2 float32 = 8
	space3 float32 = 12
	space4 float32 = 16
	space5 float32 = 20
	space6 float32 = 24
	space8 float32 = 32

	// Corner radii: controls < rows < cards < floating chrome.
	radiusControl float32 = 12
	radiusRow     float32 = 14
	radiusCard    float32 = 22
	radiusChrome  float32 = 24

	// Type scale (Apple-like): display, title, headline, body, callout, caption.
	typeDisplay  float32 = 28
	typeTitle    float32 = 19
	typeHeadline float32 = 16
	typeBody     float32 = 15
	typeCallout  float32 = 14
	typeCaption  float32 = 12

	// Page chrome: one gutter for bar, page header and content so their
	// edges align, and one top margin under the bar for every page header.
	pageGutter    float32 = 32 // horizontal page padding (8-grid)
	pageTop       float32 = 28 // bar → page title
	pageHeaderGap float32 = 24 // page header → first content
	barHeight     float32 = 56 // title bar + navigation

	radiusPill float32 = 999
)

// Elevation: two soft shadows only. Hierarchy comes from size, weight and
// color; shadows just lift the few floating things.
func elevate1(e ui.Element, s float32) ui.Element {
	return e.Shadow(0, 1*s, 3*s, 0, ui.RGBA(30, 40, 90, .08))
}
func elevate2(e ui.Element, s float32) ui.Element {
	return e.Shadow(0, 4*s, 14*s, -2*s, ui.RGBA(30, 40, 90, .12))
}

// Pastel aurora tones from the README: pink, purple and cyan over a cool white.
var (
	auroraPink   = ui.RGBA(255, 105, 180, .14)
	auroraViolet = ui.RGBA(175, 82, 222, .16)
	auroraCyan   = ui.RGBA(48, 190, 220, .15)
	auroraBlue   = ui.RGBA(10, 132, 255, .16)
)
