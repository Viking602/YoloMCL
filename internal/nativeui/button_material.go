package nativeui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Variants are paint tokens for the existing ButtonBase helpers. Layout,
// keyboard handling, focus rings and disabled actions keep the same path.
type buttonVariant uint8

const (
	buttonPrimary buttonVariant = iota
	buttonSecondary
	buttonTertiary
	buttonGhost
	buttonText
)

type buttonMaterial struct {
	top, bottom, hoverTop, hoverBottom, edge, label ui.Color
	highlight, shade, contact, lift, depression     float32
}

var buttonMaterials = [...]buttonMaterial{
	buttonPrimary:   {blue, accentIndigo, ui.Hex("#2b95ff"), ui.Hex("#7471f0"), ui.RGBA(22, 80, 190, .18), ui.Hex("#ffffff"), .28, .06, .10, .22, .18},
	buttonSecondary: {ui.Hex("#ffffff"), ui.Hex("#f4f7fc"), ui.Hex("#f4f7fc"), ui.Hex("#edf4ff"), ui.Hex("#d1d9e6"), inkSecondary, 1, .06, .06, .09, .12},
	buttonTertiary:  {ui.Hex("#edf4ff"), ui.Hex("#dfebfa"), ui.Hex("#f4f7fc"), ui.Hex("#edf4ff"), ui.RGBA(60, 72, 110, .10), inkSecondary, .85, .06, .05, .08, .16},
	buttonGhost:     {ui.Transparent, ui.Transparent, ui.Hex("#edf4ff"), ui.Hex("#edf4ff"), ui.Transparent, inkSecondary, 0, 0, 0, 0, .14},
	buttonText:      {ui.Transparent, ui.Transparent, ui.Transparent, ui.Transparent, ui.Transparent, accentHover, 0, 0, 0, 0, 0},
}

var (
	buttonTransformMotion = ui.Animation{Duration: 120 * time.Millisecond, Ease: easeOutCubic}
	buttonShadowMotion    = ui.Animation{Duration: 140 * time.Millisecond, Ease: easeOutCubic}
	buttonColorMotion     = ui.Animation{Duration: 160 * time.Millisecond, Ease: easeOutCubic}
)

func primaryButtonVariant(primary bool) buttonVariant {
	if primary {
		return buttonPrimary
	}
	return buttonSecondary
}

func variantForeground(variant buttonVariant, disabled bool) ui.Color {
	if disabled {
		return disabledInk
	}
	return buttonMaterials[variant].label
}

// u is the rendered label size / 14, including the application's UI scale.
// Every physical length uses u; rest and pressed keep the same two inset and
// three outer slots. MyGo paints insets inside this element's border radius.
func styleButtonMaterial(b ui.Element, u float32, variant buttonVariant, disabled bool) {
	m := buttonMaterials[variant]
	hovered, pressed := b.Hovered() && !disabled, b.Pressed() && !disabled
	top, bottom, edge := m.top, m.bottom, m.edge
	highlight, shade := ui.RGBA(255, 255, 255, m.highlight), ui.RGBA(0, 0, 0, m.shade)
	angle, shadow := float32(180), ui.RGBA(0, 0, 0, 1)
	if variant == buttonPrimary {
		angle, shadow = 120, ui.RGBA(10, 100, 255, 1)
		shade = ui.RGBA(0, 0, 60, m.shade)
	}
	press, hover, visible := float32(0), float32(0), float32(1)
	if hovered {
		top, bottom, hover = m.hoverTop, m.hoverBottom, 1
		if variant == buttonSecondary {
			edge = ui.Hex("#9abce9")
		}
	}
	if pressed {
		press = 1
		top, bottom = m.bottom, m.top
		highlight, shade = ui.RGBA(0, 0, 0, m.depression), ui.Transparent
		switch variant {
		case buttonPrimary:
			top, bottom = ui.Hex("#0a6fe0"), ui.Hex("#4f4dd0")
			highlight = ui.RGBA(0, 0, 60, m.depression)
		case buttonSecondary:
			top, bottom, edge = ui.Hex("#dfebfa"), ui.Hex("#f4f7fc"), ui.Hex("#7ea8df")
		case buttonGhost:
			top, bottom = ui.Hex("#dfebfa"), ui.Hex("#edf4ff")
			edge = ui.RGBA(60, 72, 110, .10)
		}
	}
	if disabled {
		visible = 0
		top, bottom, edge = disabledFill, disabledFill, disabledEdge
		highlight, shade = ui.Transparent, ui.Transparent
		if variant == buttonGhost || variant == buttonText {
			top, bottom, edge = ui.Transparent, ui.Transparent, ui.Transparent
		}
		b.Cursor(ui.CursorDefault)
	}
	h := b.AnimateTo("material-hover", hover, buttonShadowMotion)
	p := b.AnimateTo("material-press", press, buttonShadowMotion)
	v := b.AnimateTo("material-shadow", visible, buttonShadowMotion)
	b.Radius(radiusControl*u).Border(u, b.AnimateColor("material-edge", edge, buttonColorMotion)).
		LinearGradient(ui.LinearGradient{From: b.AnimateColor("material-top", top, buttonColorMotion), To: b.AnimateColor("material-bottom", bottom, buttonColorMotion), Angle: angle}).
		Translate(0, u*b.AnimateTo("material-offset", press, buttonTransformMotion))
	b.InsetShadow(0, u, 2*u*p, 0, b.AnimateColor("material-highlight", highlight, buttonShadowMotion)).
		InsetShadow(0, -u, 0, 0, b.AnimateColor("material-shade", shade, buttonShadowMotion))
	// Light keys retain a white line at the lower edge of the pressed well.
	whiteLine := float32(0)
	if variant == buttonSecondary || variant == buttonTertiary || variant == buttonGhost {
		whiteLine = .9
	}
	// MyGo paints outer shadows in insertion order, back to front. Keep the
	// pressed white edge above the contact shadow, as a line in the paper.
	b.Shadow(0, (4+4*h)*u, (10+6*h)*u, (-3-2*h)*u, shadow.Alpha(m.lift*(1-p)*v)).
		Shadow(0, u, (2-p)*u, 0, shadow.Alpha(m.contact*v)).
		Shadow(0, u, 0, 0, ui.RGBA(255, 255, 255, whiteLine*p*v))
}
