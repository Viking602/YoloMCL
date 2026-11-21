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
	// Lobe UI monochrome primary & crisp neutral surfaces with subtle tactile press depth.
	buttonPrimary:   {ui.Hex("#262626"), ui.Hex("#1c1c1c"), ui.Hex("#363636"), ui.Hex("#2b2b2b"), ui.Hex("#1f1f1f"), ui.Hex("#f8f8f8"), .12, .12, .08, .12, .18},
	buttonSecondary: {ui.Hex("#ffffff"), ui.Hex("#fbfbfb"), ui.Hex("#fafafa"), ui.Hex("#f3f3f4"), ui.Hex("#e3e3e3"), ink, .9, .03, .04, .05, .10},
	buttonTertiary:  {ui.Hex("#f3f3f4"), ui.Hex("#ededef"), ui.Hex("#ececee"), ui.Hex("#e5e5e7"), ui.RGBA(0, 0, 0, .06), inkSecondary, .7, .03, .03, .04, .10},
	buttonGhost:     {ui.Transparent, ui.Transparent, ui.RGBA(0, 0, 0, .05), ui.RGBA(0, 0, 0, .06), ui.Transparent, inkSecondary, 0, 0, 0, 0, .08},
	buttonText:      {ui.Transparent, ui.Transparent, ui.Transparent, ui.Transparent, ui.Transparent, accent, 0, 0, 0, 0, 0},
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
	press, hover, visible := float32(0), float32(0), float32(1)
	if hovered {
		top, bottom, hover = m.hoverTop, m.hoverBottom, 1
		if variant == buttonSecondary {
			edge = ui.Hex("#d4d4d4")
		}
	}
	if pressed {
		press = 1
		top, bottom = m.bottom, m.top
		highlight, shade = ui.RGBA(0, 0, 0, m.depression), ui.Transparent
		switch variant {
		case buttonPrimary:
			top, bottom = ui.Hex("#111111"), ui.Hex("#262626")
			highlight = ui.RGBA(0, 0, 0, m.depression)
		case buttonSecondary:
			top, bottom, edge = ui.Hex("#e8e8ea"), ui.Hex("#f7f7f8"), ui.Hex("#cccccc")
		case buttonTertiary:
			top, bottom = ui.Hex("#dfe0e2"), ui.Hex("#eeeeef")
		case buttonGhost:
			top, bottom = ui.RGBA(0, 0, 0, .09), ui.RGBA(0, 0, 0, .07)
			edge = ui.RGBA(0, 0, 0, .06)
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
