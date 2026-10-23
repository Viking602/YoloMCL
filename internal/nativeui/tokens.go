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

	// Corner radii: controls < rows < cards < floating chrome.
	radiusControl float32 = 9
	radiusRow     float32 = 12
	radiusCard    float32 = 16
	radiusChrome  float32 = 20

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
	pageGutter    float32 = 32 // horizontal page padding
	pageTop       float32 = 28 // bar → page title
	pageHeaderGap float32 = 24 // page header → first content & between sections
	barHeight     float32 = 44 // title bar + navigation
	radiusPill    float32 = 999
)

func elevate2(e ui.Element, s float32) ui.Element {
	return e.Shadow(0, 4*s, 14*s, -2*s, ui.RGBA(30, 40, 90, .12))
}
