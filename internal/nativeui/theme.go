package nativeui

import "github.com/egoist/mygo/ui"

// Apple-inspired light palette: aurora canvas, frosted white surfaces, system accents.
var (
	bg            = ui.Hex("#f1f4fb")
	panel         = ui.Hex("#ffffff")
	raised        = ui.RGBA(246, 248, 252, .9)
	line          = ui.RGBA(60, 72, 110, .10)
	lineStrong    = ui.RGBA(60, 72, 110, .18)
	glassEdge     = ui.RGBA(255, 255, 255, .9)
	ink           = ui.Hex("#1d1d1f")
	inkSecondary  = ui.Hex("#3a3a44")
	muted         = ui.Hex("#5f6170") // ≥4.5:1 on frosted white over the aurora
	accent        = ui.Hex("#007aff") // Apple System Blue
	accentIndigo  = ui.Hex("#5e5ce6") // Apple System Indigo
	accentViolet  = ui.Hex("#af52de") // Apple System Purple
	accentHover   = ui.Hex("#0066d6")
	accentSoft    = ui.RGBA(0, 122, 255, .10)
	blue          = ui.Hex("#0a84ff")
	danger        = ui.Hex("#ff3b30") // Apple System Red
	dangerSoft    = ui.RGBA(255, 59, 48, .10)
	fieldColor    = ui.RGBA(118, 128, 160, .10)
	fieldHover    = ui.RGBA(118, 128, 160, .17)
	selectedColor = ui.RGBA(0, 122, 255, .14)
	success       = ui.Hex("#34c759") // Apple System Green
	successInk    = ui.Hex("#15803d")
	successSoft   = ui.RGBA(52, 199, 89, .12)
	warning       = ui.Hex("#ff9500") // Apple System Orange
	warningInk    = ui.Hex("#b45309")
	theme         *ui.Theme
	fastAnim      = motionFast
)

// newTheme is called with asset initialization, after fonts are registered.
func newTheme() *ui.Theme {
	theme := ui.LightTheme()
	theme.Background = bg
	theme.Surface = fieldColor
	theme.SurfaceHover = fieldHover
	theme.SurfacePressed = selectedColor
	theme.Border = line
	theme.Text = ink
	theme.TextMuted = muted
	theme.Accent = accent
	theme.AccentHover = accentHover
	theme.AccentPressed = ui.Hex("#0051a8")
	theme.AccentText = ui.Hex("#ffffff")
	theme.Danger = danger
	theme.Success = successInk
	theme.Warning = warningInk
	theme.Font = uiFontFamily(titlebarOS())
	theme.FontSize = 14.5
	theme.Radius = 9
	theme.Spacing = 16.0 / 3
	theme.Focus = ui.RGBA(0, 122, 255, .25)
	return theme
}

// Apple Oklab gradient helpers.
func appleAccentGradient(hovered bool) ui.LinearGradient {
	if hovered {
		return ui.LinearGradient{From: ui.Hex("#2b95ff"), To: ui.Hex("#7471f0"), Angle: 120}
	}
	return ui.LinearGradient{From: ui.Hex("#0a84ff"), To: accentIndigo, Angle: 120}
}

// tintGradient is the two-tone squircle fill of a colored row or header icon.
func tintGradient(from, to ui.Color) ui.LinearGradient {
	return ui.LinearGradient{From: from, To: to, Angle: 135}
}

// glass styles e as a frosted surface: translucent white over the aurora
// canvas, a bright hairline edge and a soft two-layer ambient shadow.
func glass(e ui.Element, radius, s float32) ui.Element {
	return e.Radius(radius).LinearGradient(ui.LinearGradient{From: ui.RGBA(255, 255, 255, .82), To: ui.RGBA(255, 255, 255, .66), Angle: 180}).
		Border(1, glassEdge).Shadow(0, 12*s, 32*s, -10*s, ui.RGBA(40, 52, 110, .14)).Shadow(0, 1*s, 3*s, 0, ui.RGBA(40, 52, 110, .05))
}

// popover styles floating layers (dialogs, menus) that sit above content
// and must stay legible over anything.
func popover(e ui.Element, radius, s float32) ui.Element {
	return e.Radius(radius).LinearGradient(ui.LinearGradient{From: ui.RGBA(255, 255, 255, 1), To: ui.RGBA(248, 249, 253, 1), Angle: 180}).
		Border(1, glassEdge).Shadow(0, 24*s, 60*s, -12*s, ui.RGBA(30, 40, 90, .28)).Shadow(0, 0, 0, 1, ui.RGBA(30, 40, 90, .06))
}
