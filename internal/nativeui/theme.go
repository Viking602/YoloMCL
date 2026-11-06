package nativeui

import "github.com/egoist/mygo/ui"

// Light desktop chrome: paper surfaces, blue accents and quiet separators.
var (
	bg            = ui.Hex("#f3f4f6")
	panel         = ui.Hex("#ffffff")
	raised        = ui.Hex("#f8f9fa")
	line          = ui.RGBA(15, 23, 42, .10)
	lineStrong    = ui.RGBA(15, 23, 42, .18)
	glassEdge     = ui.RGBA(255, 255, 255, .9)
	ink           = ui.Hex("#222833")
	inkSecondary  = ui.Hex("#424956")
	muted         = ui.Hex("#626b78")
	accent        = ui.Hex("#007aff") // Apple System Blue
	accentIndigo  = ui.Hex("#5e5ce6") // Apple System Indigo
	accentViolet  = ui.Hex("#af52de") // Apple System Purple
	accentHover   = ui.Hex("#0066d6")
	accentSoft    = ui.RGBA(0, 122, 255, .10)
	blue          = ui.Hex("#0a84ff")
	danger        = ui.Hex("#ff3b30") // Apple System Red
	dangerSoft    = ui.RGBA(255, 59, 48, .10)
	fieldColor    = ui.Hex("#f0f1f3")
	fieldHover    = ui.Hex("#e7e9ed")
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
	theme.Radius = radiusControl
	theme.Spacing = space2
	theme.Focus = accent.Alpha(.25)
	theme.Selection = selectedColor
	return theme
}

// Shared blue-to-indigo accent for primary actions and selected items.
func appleAccentGradient(hovered bool) ui.LinearGradient {
	if hovered {
		return ui.LinearGradient{From: buttonMaterials[buttonPrimary].hoverTop, To: buttonMaterials[buttonPrimary].hoverBottom, Angle: 120}
	}
	return ui.LinearGradient{From: buttonMaterials[buttonPrimary].top, To: buttonMaterials[buttonPrimary].bottom, Angle: 120}
}

// tintGradient is the two-tone squircle fill of a colored row or header icon.
func tintGradient(from, to ui.Color) ui.LinearGradient {
	return ui.LinearGradient{From: from, To: to, Angle: 135}
}

// Existing surface helper uses opaque paper with a quiet contact edge.
func glass(e ui.Element, radius, s float32) ui.Element {
	return e.Radius(radius).Background(panel).Border(s, line).Shadow(0, s, 2*s, 0, ui.RGBA(0, 0, 0, .04))
}

// popover styles floating layers (dialogs, menus) that sit above content
// and must stay legible over anything.
func popover(e ui.Element, radius, s float32) ui.Element {
	return e.Radius(radius).Background(panel).Border(s, lineStrong).
		Shadow(0, s, 2*s, 0, ui.RGBA(0, 0, 0, .08)).Shadow(0, 8*s, 16*s, -5*s, ui.RGBA(0, 0, 0, .14))
}
