package nativeui

import (
	"github.com/egoist/mygo/ui"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"
)

// Lobe UI light design tokens: crisp neutral surfaces, hairline borders,
// monochrome primary actions, and restrained semantic accents.
var (
	lobeLight = lobetheme.Light()

	bg            = lobeLight.BgLayout
	panel         = lobeLight.BgContainer
	raised        = lobeLight.PrimaryBg
	line          = lobeLight.BorderSecondary
	lineStrong    = lobeLight.Border
	glassEdge     = lobeLight.BorderSecondary
	ink           = lobeLight.Text
	inkSecondary  = lobeLight.TextSecondary
	muted         = lobeLight.TextTertiary
	lobePrimary   = lobeLight.Primary
	accent        = lobeLight.Info
	accentIndigo  = ui.Hex("#4f46e5")
	accentViolet  = ui.Hex("#9333ea")
	accentSoft    = lobeLight.InfoFillTertiary
	blue          = lobeLight.Info
	danger        = lobeLight.Error
	dangerSoft    = lobeLight.ErrorFillTertiary
	fieldColor    = ui.Hex("#f1f1f2")
	fieldHover    = ui.Hex("#e7e7e9")
	selectedColor = lobeLight.FillSecondary
	success       = lobeLight.Success
	successInk    = lobeLight.SuccessText
	successSoft   = lobeLight.SuccessFillTertiary
	warning       = lobeLight.Warning
	warningInk    = lobeLight.WarningText
	theme         *ui.Theme
	fastAnim      = motionFast
)

// newTheme is called with asset initialization, after fonts are registered.
func newTheme() *ui.Theme {
	t := lobeLight.Theme()
	t.Background = bg
	t.Surface = fieldColor
	t.SurfaceHover = fieldHover
	t.SurfacePressed = selectedColor
	t.Border = lineStrong
	t.Text = ink
	t.TextMuted = muted
	t.Accent = lobePrimary
	t.AccentHover = lobeLight.PrimaryHover
	t.AccentPressed = lobeLight.PrimaryActive
	t.AccentText = lobeLight.PrimaryForeground
	t.Danger = danger
	t.Success = successInk
	t.Warning = warningInk
	t.Font = uiFontFamily(titlebarOS())
	t.FontSize = 14.5
	t.Radius = radiusControl
	t.Spacing = space2
	t.Focus = accent
	t.Selection = accentSoft
	return t
}

// tintGradient is a subtle two-tone fill for square badges or icon tiles.
func tintGradient(from, to ui.Color) ui.LinearGradient {
	return ui.LinearGradient{From: from, To: to, Angle: 135}
}

// glass styles elevated paper cards using Lobe UI's crisp container surface.
func glass(e ui.Element, radius, s float32) ui.Element {
	return e.Radius(radius).Background(panel).Border(s, line).Shadow(0, s, 2*s, 0, ui.RGBA(0, 0, 0, .03))
}

// popover styles floating layers (dialogs, menus, dropdowns) above content
// with Lobe UI's crisp hairline border and layered neutral elevation.
func popover(e ui.Element, radius, s float32) ui.Element {
	return e.Radius(radius).Background(panel).Border(s, lineStrong).
		Shadow(0, s, 2*s, 0, ui.RGBA(0, 0, 0, .05)).
		Shadow(0, 8*s, 20*s, -4*s, ui.RGBA(0, 0, 0, .10))
}
