package nativeui

import (
	"math"

	"github.com/egoist/mygo/ui"
)

func buttonWidth(c *ui.Context, text, icon string, fontSize float32) float32 {
	s := uiScale(c)
	w, _ := c.MeasureText(0, ui.Span{Text: text, Font: uiSansFamily(), Size: fontSize, Weight: 600})
	w += 2*btnPad*s + 2 // horizontal padding and the shared one-DIP border
	if icon != "" {
		w += fontSize + btnGap*s
	}
	return float32(math.Ceil(float64(max(w, btnMin*s))))
}

// Both roles use the same border box and tight shadow. Secondary buttons
// keep an opaque tinted face and visible edge even on white cards.
func styleButton(b ui.Element, s float32, primary, disabled bool) {
	hovered := b.Hovered() && !disabled
	pressed := b.Pressed() && !disabled
	if disabled {
		// One disabled look for every button: neutral fill, hairline edge,
		// readable label — never opacity alone (see disabledInk).
		b.Background(disabledFill).Border(1, disabledEdge)
		return
	}
	if primary {
		b.Border(1, ui.RGBA(22, 80, 190, .18))
		if pressed {
			b.LinearGradient(ui.LinearGradient{From: ui.Hex("#0a6fe0"), To: ui.Hex("#4f4dd0"), Angle: 120}).InsetShadow(0, 1*s, 3*s, 0, ui.RGBA(0, 0, 60, .18))
			return
		}
		glow := ui.RGBA(10, 100, 255, .22)
		if hovered {
			glow = ui.RGBA(10, 100, 255, .30)
		}
		b.LinearGradient(appleAccentGradient(hovered)).InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, .28)).Shadow(0, 2*s, 6*s, -2*s, glow)
		return
	}
	targetBg, edge := ui.Hex("#f4f7fc"), ui.Hex("#d1d9e6")
	switch {
	case pressed:
		targetBg, edge = ui.Hex("#dfebfa"), ui.Hex("#7ea8df")
	case hovered:
		targetBg, edge = ui.Hex("#edf4ff"), ui.Hex("#9abce9")
	}
	b.Background(b.AnimateColor("bg", targetBg, fastAnim)).Border(1, b.AnimateColor("edge", edge, fastAnim))
	if pressed {
		b.InsetShadow(0, 1*s, 2*s, 0, ui.RGBA(40, 52, 90, .08))
	} else {
		b.Shadow(0, 1*s, 2*s, -1*s, ui.RGBA(40, 52, 90, .12))
	}
}

func buttonContent(c *ui.Context, key, text, icon string, fontSize float32, fg ui.Color) {
	if icon != "" && icons[icon] != nil {
		ui.Icon(c, icons[icon]).Size(fontSize, fontSize).TextColor(fg).Shrink(0)
	}
	ui.Text(c, text).Label("按钮文字 " + key).FontSize(fontSize).FontWeight(600).TextColor(fg).NoWrap().Shrink(0)
}

func buttonForeground(primary, disabled bool) ui.Color {
	switch {
	case disabled:
		return disabledInk
	case primary:
		return panel
	}
	return inkSecondary
}

// button is the compact standard action button; its width follows the text.
func button(c *ui.Context, key, text, icon string, primary, disabled bool) bool {
	return fixedButton(c, key, text, icon, 0, btnMd*uiScale(c), primary, disabled)
}

// fixedButton is a button of at least width × height, so aligned controls
// in a row share one size.
func fixedButton(c *ui.Context, key, text, icon string, width, height float32, primary, disabled bool) bool {
	s := uiScale(c)
	fg := buttonForeground(primary, disabled)
	font := min(typeCallout*s, max(typeCaption*s, height*.43))
	b := ui.ButtonBase(c.Key(key)).Label(text).Font(uiSansFamily()).Size(max(width, buttonWidth(c, text, icon, font)), height).Shrink(0).PaddingX(btnPad * s).Radius(min(radiusControl*s, height*.30)).TextColor(fg).AlignItems(ui.Center).Justify(ui.Center).Gap(btnGap * s)
	return finishButton(c, b, key, text, icon, font, fg, primary, disabled)
}

// selectBox wraps ui.SelectBase with an Apple-styled rounded trigger and scrollable frosted popup.
// iconOnlyButton is a square secondary button; label stays for a11y/tooltip.
func iconOnlyButton(c *ui.Context, key, icon, label string, size float32, disabled bool) bool {
	s := uiScale(c)
	fg := buttonForeground(false, disabled)
	b := tooltip(c, ui.ButtonBase(c.Key(key)).Label(label), label).Size(size, size).Shrink(0).Radius(min(radiusControl*s, size*.30)).AlignItems(ui.Center).Justify(ui.Center)
	styleButton(b, s, false, disabled)
	return b.Children(func() { ui.Icon(c, icons[icon]).Size(0.45*size, 0.45*size).TextColor(fg) }).Clicked() && !disabled
}

// Disabled controls keep a readable label (about 5:1 on their fill)
// instead of the toolkit's 50% fade.
var (
	disabledFill = ui.Hex("#e8ebf1")
	disabledEdge = ui.RGBA(30, 40, 70, .10)
	disabledInk  = ui.Hex("#596275")
)

// buttonWhy[key] is the reason a button is disabled, set by buttonWhy for
// the frame; shown as its tooltip.
var (
	dialogPrimaryKey string // first enabled primary button in the open dialog
	enterFired       string // primary button Enter activated this frame
	inDialog         bool
)

func finishButton(c *ui.Context, b ui.Element, key, text, icon string, font float32, fg ui.Color, primary, disabled bool) bool {
	s := uiScale(c)
	styleButton(b, s, primary, disabled)
	if disabled {
		// Not ui.Disabled: that halves the opacity. The button still
		// announces itself and keeps focus, but does nothing.
		b.Cursor(ui.CursorDefault)
		if why := pendingWhy; why != "" {
			tooltip(c, b, why)
		}
	}
	pendingWhy = ""
	if primary && !disabled && inDialog && dialogPrimaryKey == "" {
		dialogPrimaryKey = key
	}
	clicked := b.Children(func() { buttonContent(c, key, text, icon, font, fg) }).Clicked()
	if !disabled && enterFired == key {
		enterFired = ""
		return true
	}
	return clicked && !disabled
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
	ui.Row(c).Label("禁用原因 " + why).Gap(5 * s).AlignItems(ui.Center).Shrink(1).MinWidth(0).Children(func() {
		ui.Icon(c, icons["info"]).Size(13*s, 13*s).TextColor(disabledInk).Shrink(0)
		ui.Text(c, why).FontSize(typeFootnote * s).TextColor(disabledInk).SingleLine().Shrink(1).MinWidth(0)
	})
}

// pathLabel shows a path whose end matters most: the folder part shrinks
// (ellipsized) and the last element stays whole; the full path is the
// tooltip.
// dangerButton is the armed second step of a destructive action.
func dangerButton(c *ui.Context, key, text string, height float32, disabled bool) bool {
	s := uiScale(c)
	b := ui.ButtonBase(c.Key(key)).Label(text).Height(height).PaddingX(12 * s).Shrink(0).Radius(min(12*s, height*.32)).AlignItems(ui.Center).Background(danger).AutoFocus()
	return b.Children(func() {
		ui.Text(c, text).FontSize(typeFootnote * s).FontWeight(600).TextColor(panel).NoWrap()
	}).Clicked() && !disabled
}

// Button size tokens: heights by role, and one horizontal padding.
const (
	btnSm  float32 = 30 // row actions (选择, 设为当前)
	btnMd  float32 = 35 // standard action button
	btnLg  float32 = 40 // primary launch CTA
	btnPad float32 = 14
	btnGap float32 = 7
	btnMin float32 = 54 // minimum width of a text button
)

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
