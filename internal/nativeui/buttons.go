package nativeui

import (
	"math"

	"github.com/egoist/mygo/ui"
)

func buttonWidth(c *ui.Context, text, icon string, bs buttonSize, s float32) float32 {
	w, _ := c.MeasureText(0, ui.Span{Text: text, Font: uiSansFamily(), Size: bs.font * s, Weight: 600})
	w += (2*bs.pad + 2) * s
	if icon != "" {
		w += (bs.icon + bs.gap) * s
	}
	return float32(math.Ceil(float64(max(w, bs.minWidth*s))))
}

// Compatibility entry point for existing primary/secondary callers.
func styleButton(b ui.Element, s float32, primary, disabled bool) {
	styleButtonMaterial(b, s, primaryButtonVariant(primary), disabled)
}

func buttonContent(c *ui.Context, key, text, icon string, fontSize, iconSize float32, fg ui.Color) ui.Element {
	if icon != "" && icons[icon] != nil {
		ui.Icon(c, icons[icon]).Size(iconSize, iconSize).TextColor(fg).Shrink(0)
	}
	return ui.Text(c, text).Label("按钮文字 " + key).FontSize(fontSize).FontWeight(600).TextColor(fg).NoWrap().Shrink(0)
}

func buttonForeground(primary, disabled bool) ui.Color {
	return variantForeground(primaryButtonVariant(primary), disabled)
}

// button is the standard md action button; its width follows the text.
func button(c *ui.Context, key, text, icon string, primary, disabled bool) bool {
	return fixedButton(c, key, text, icon, 0, btnMd*uiScale(c), primary, disabled)
}

// fixedButton is a button of at least width × height, so aligned controls
// in a row share one size.
func fixedButton(c *ui.Context, key, text, icon string, width, height float32, primary, disabled bool) bool {
	return fixedVariantButton(c, key, text, icon, width, height, primaryButtonVariant(primary), disabled)
}

// Additional roles use the same button implementation and interaction contract.
func variantButton(c *ui.Context, key, text, icon string, variant buttonVariant, disabled bool) bool {
	return fixedVariantButton(c, key, text, icon, 0, btnMd*uiScale(c), variant, disabled)
}

// height is scaled (btnSm*s etc.); it snaps to the nearest size token.
func fixedVariantButton(c *ui.Context, key, text, icon string, width, height float32, variant buttonVariant, disabled bool) bool {
	_, clicked := fixedButtonElement(c, key, text, icon, width, height, variant, disabled)
	return clicked
}

func fixedButtonElement(c *ui.Context, key, text, icon string, width, height float32, variant buttonVariant, disabled bool) (ui.Element, bool) {
	s := uiScale(c)
	bs := buttonSizeFor(height / s)
	fg := variantForeground(variant, disabled)
	w := buttonWidth(c, text, icon, bs, s)
	if variant == buttonText {
		w += (bs.icon + bs.gap) * s
	}
	b := ui.ButtonBase(c.Key(key)).Label(text).Font(uiSansFamily()).Size(max(width, w), bs.height*s).Shrink(0).PaddingX(bs.pad * s).TextColor(fg).AlignItems(ui.Center).Justify(ui.Center).Gap(bs.gap * s)
	clicked := finishSizedButton(c, b, key, text, icon, bs, s, fg, variant, disabled)
	return b, clicked
}

// iconOnlyButton is a square tertiary button; label stays for a11y/tooltip.
// size is scaled and snaps to btnSm/btnMd/btnLg.
func iconOnlyButton(c *ui.Context, key, icon, label string, size float32, disabled bool) bool {
	return iconButtonElement(c, key, icon, label, size, disabled).Clicked() && !disabled
}

func iconButtonElement(c *ui.Context, key, icon, label string, size float32, disabled bool) ui.Element {
	s := uiScale(c)
	bs := buttonSizeFor(size / s)
	fg := variantForeground(buttonTertiary, disabled)
	b := tooltip(c, ui.ButtonBase(c.Key(key)).Label(label), label).Size(bs.height*s, bs.height*s).Shrink(0).AlignItems(ui.Center).Justify(ui.Center)
	styleButtonMaterial(b, s, buttonTertiary, disabled)
	return b.Children(func() { ui.Icon(c, icons[icon]).Size(bs.icon*s, bs.icon*s).TextColor(fg) })
}

// Disabled controls keep a readable label (about 5:1 on their fill)
// instead of the toolkit's 50% fade.
var (
	disabledFill = ui.Hex("#e9ebee")
	disabledEdge = ui.RGBA(15, 23, 42, .10)
	disabledInk  = ui.Hex("#59616e")
)

// buttonWhy[key] is the reason a button is disabled, set by buttonWhy for
// the frame; shown as its tooltip.
var (
	dialogPrimaryKey string // first enabled primary button in the open dialog
	enterFired       string // primary button Enter activated this frame
	inDialog         bool
)

func finishButton(c *ui.Context, b ui.Element, key, text, icon string, font float32, fg ui.Color, primary, disabled bool) bool {
	return finishVariantButton(c, b, key, text, icon, font, fg, primaryButtonVariant(primary), disabled)
}

// finishVariantButton is for hand-built buttons that pass a raw label size.
func finishVariantButton(c *ui.Context, b ui.Element, key, text, icon string, font float32, fg ui.Color, variant buttonVariant, disabled bool) bool {
	u := font / 14
	bs := buttonSize{font: 14, icon: 16, gap: btnGap}
	return finishSizedButton(c, b, key, text, icon, bs, u, fg, variant, disabled)
}

func finishSizedButton(c *ui.Context, b ui.Element, key, text, icon string, bs buttonSize, u float32, fg ui.Color, variant buttonVariant, disabled bool) bool {
	font, iconSize := bs.font*u, bs.icon*u
	if variant == buttonPrimary || variant == buttonSecondary {
		styleButton(b, u, variant == buttonPrimary, disabled)
	} else {
		styleButtonMaterial(b, u, variant, disabled)
	}
	// Raw-font callers use u for type and material effects; corner geometry
	// belongs to the window's spacing scale, just like inputs and icon buttons.
	b.Radius(radiusControl * uiScale(c))
	if disabled {
		// Not ui.Disabled: that halves the opacity. The button still
		// announces itself and keeps focus, but does nothing.
		b.Cursor(ui.CursorDefault)
		if why := pendingWhy; why != "" {
			tapTooltip(c, b, why)
		}
	}
	pendingWhy = ""
	if variant == buttonPrimary && !disabled && inDialog && dialogPrimaryKey == "" {
		dialogPrimaryKey = key
	}
	fg = b.AnimateColor("material-label", fg, buttonColorMotion)
	arrow := float32(0)
	if variant == buttonText {
		if b.Hovered() && !disabled {
			arrow = 2 * u
		}
		arrow = b.AnimateTo("material-arrow", arrow, buttonTransformMotion)
	}
	clicked := b.Children(func() {
		label := buttonContent(c, key, text, icon, font, iconSize, fg)
		if variant == buttonPrimary && !disabled {
			slateLabelShadow(label, text, font)
		}
		if variant == buttonText {
			label.Underline()
			ui.Icon(c, icons["arrow"]).Size(iconSize, iconSize).TextColor(fg).Shrink(0).Translate(arrow, 0)
		}
	}).Clicked()
	if !disabled && enterFired == key {
		enterFired = ""
		return true
	}
	return clicked && !disabled
}

func slateLabelShadow(label ui.Element, text string, font float32) {
	label.Draw(func(p *ui.Painter, r ui.Rect) {
		p.RichText(r.X, r.Y-font/14, 0, ui.Span{Text: text, Font: uiSansFamily(), Size: font, Weight: 600, Color: ui.RGBA(0, 0, 0, .28)})
	})
}

var pendingWhy string

// buttonWhy is button with the reason it is disabled: why == "" means
// enabled. The reason shows as a tooltip and, with inline, as a muted hint
// beside the button, so it never depends on hover alone.
func buttonWhy(c *ui.Context, key, text, icon string, primary bool, why string, inline bool) bool {
	pendingWhy = why
	clicked := button(c, key, text, icon, primary, why != "")
	if why != "" && inline {
		whyHint(c, why)
	}
	return clicked
}

func whyHint(c *ui.Context, why string) {
	s := uiScale(c)
	ui.Row(c).Label("禁用原因 " + why).Gap(4 * s).AlignItems(ui.Center).Shrink(1).MinWidth(0).Children(func() {
		ui.Icon(c, icons["info"]).Size(13*s, 13*s).TextColor(disabledInk).Shrink(0)
		ui.Text(c, why).FontSize(typeFootnote * s).TextColor(disabledInk).SingleLine().Shrink(1).MinWidth(0)
	})
}

// whyOf takes (condition, reason) pairs and returns the first reason whose
// condition holds, or "" when none does: the reason a control is disabled.
func whyOf(pairs ...any) string {
	for i := 0; i+1 < len(pairs); i += 2 {
		if on, _ := pairs[i].(bool); on {
			if s, _ := pairs[i+1].(string); s != "" {
				return s
			}
		}
	}
	return ""
}
