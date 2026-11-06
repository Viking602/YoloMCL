package nativeui

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

const (
	selectRowHeight float32 = 36
	selectMaxHeight float32 = 248
	selectIconSize  float32 = 20
)

func selectBox(c *ui.Context, key, labelText string, selected *string, options []string) ui.Element {
	return selectBoxWithIcons(c, key, labelText, selected, options, nil)
}

func selectBoxWithIcons(c *ui.Context, key, labelText string, selected *string, options []string, iconFn func(string) (string, ui.Color)) ui.Element {
	return selectBoxFull(c, key, labelText, selected, options, iconFn, nil)
}

// selectBoxFull renders option values through display (when set) so long
// internal ids can be shown as short human titles. Text always ellipsizes.
func selectBoxFull(c *ui.Context, key, labelText string, selected *string, options []string, iconFn func(string) (string, ui.Color), display func(string) string) ui.Element {
	return selectBoxImpl(c, key, labelText, selected, options, iconFn, display, nil, nil)
}

// selectBoxImpl optionally draws a custom leading icon per option.
func selectBoxImpl(c *ui.Context, key, labelText string, selected *string, options []string, iconFn func(string) (string, ui.Color), display func(string) string, drawIcon func(string), trailing func(opt string, onAccent bool)) ui.Element {
	s := uiScale(c)
	show := func(v string) string {
		if display != nil {
			return display(v)
		}
		return v
	}
	ctx := c
	if key != "" {
		ctx = c.Key(key)
	}
	sel := ui.SelectBase(ctx, selected)
	if !sel.Open() {
		sel.Trigger.Jump("popup-reveal", 0)
	}
	styleSelectField(sel.Trigger, s, sel.Open())
	if labelText != "" {
		sel.Trigger.Label(labelText)
	}
	sel.Trigger.Height(btnMd*s).Padding(0, space4*s).Font(uiSansFamily()).Clip().AlignItems(ui.Center).Gap(space2 * s).Children(func() {
		selectLeadingIcon(c, *selected, s, iconFn, drawIcon)
		selectLabel(c, show(*selected), ink, s)
		if trailing != nil {
			trailing(*selected, false)
		}
		selectArrow(c, sel.Open(), s)
	})
	sel.Popup(func(popupPanel ui.Element) {
		selectPopupPanel(c, popupPanel, sel.Trigger)
		// Keep the panel and text at their final size throughout the entrance.
		p := sel.Trigger.AnimateTo("popup-reveal", 1, motionEnter)
		popupPanel.Opacity(p).Translate(0, -6*s*(1-p))
		selectOptionsScroll(c, "select-scroll-"+key+"-"+labelText, sel.Trigger, len(options)).Children(func() {
			ui.Column(c).FillWidth().MinWidth(0).Gap(space1 * s).Children(func() {
				for _, opt := range options {
					option := opt
					isSelected := option == *selected
					item, fgCol := selectOptionStyle(sel.Item(option).Label(labelText+" "+show(option)), isSelected, s)
					item.Children(func() {
						selectLeadingIcon(c, option, s, iconFn, drawIcon)
						selectLabel(c, show(option), fgCol, s)
						if trailing != nil {
							trailing(option, false)
						}
						selectCheck(c, isSelected, fgCol, s)
					})
				}
			})
		})
	})
	return sel.Trigger
}

// Choice fields share a quiet paper surface and blue focus edge across
// settings and filters. Their bounds stay still while the pointer presses.
func styleSelectField(field ui.Element, s float32, active bool) {
	fill, edge := panel, line
	if field.Hovered() {
		fill, edge = raised, lineStrong
	}
	if active || field.Focused() {
		fill, edge = panel, accent.Alpha(.4)
	}
	if field.Pressed() {
		fill = fieldColor
	}
	field.Radius(radiusControl*s).
		Background(field.AnimateColor("select-surface", fill, fastAnim)).
		Border(s, field.AnimateColor("select-edge", edge, fastAnim))
}

// The trigger and options share icon, type and trailing columns. A check's
// space stays reserved so choosing an option never changes its text measure.
func selectLeadingIcon(c *ui.Context, value string, s float32, iconFn func(string) (string, ui.Color), drawIcon func(string)) {
	if iconFn == nil && drawIcon == nil {
		return
	}
	ui.Row(c).Size(selectIconSize*s, selectIconSize*s).Shrink(0).Center().Children(func() {
		if drawIcon != nil {
			drawIcon(value)
		}
		if iconFn != nil {
			ic, col := iconFn(value)
			if loaderLogos[strings.TrimPrefix(ic, "loader-")] != nil {
				loaderLogo(c, value, selectIconSize*s)
			} else if icons[ic] != nil {
				ui.Icon(c, icons[ic]).Size(space4*s, space4*s).TextColor(col).Shrink(0)
			}
		}
	})
}

func selectArrow(c *ui.Context, open bool, s float32) {
	angle := float32(0)
	if open {
		angle = 180
	}
	ui.Icon(c, icons["chevron-down"]).Size(space4*s, space4*s).TextColor(muted).Shrink(0).Rotate(angle)
}

func selectLabel(c *ui.Context, text string, color ui.Color, s float32) ui.Element {
	font := ui.Font{Family: uiSansFamily(), Size: typeCallout * s, Weight: 400}
	e := ui.Text(c, text).Font(font.Family).FontSize(font.Size).FontWeight(font.Weight).TextColor(color).Grow(1).Shrink(1).Basis(0).MinWidth(0).SingleLine()
	return textTooltip(c, e, text, "", font, 1)
}

func selectCheck(c *ui.Context, selected bool, color ui.Color, s float32) {
	ui.Box(c).Size(space4*s, space4*s).Shrink(0).Center().Children(func() {
		if selected {
			ui.Icon(c, icons["check"]).Size(space4*s, space4*s).TextColor(color).Shrink(0)
		}
	})
}

func selectOptionStyle(item ui.Element, selected bool, s float32) (ui.Element, ui.Color) {
	bg, fg, edge := ui.Transparent, ink, ui.Transparent
	if selected {
		bg, fg = accentSoft, accent
	} else if item.Highlighted() {
		bg = accent.Alpha(.05)
	}
	if item.Highlighted() {
		edge = accent.Alpha(.20)
	}
	return item.FillWidth().MinWidth(0).Height(selectRowHeight*s).Padding(0, space2*s).
		Font(uiSansFamily()).Radius(nestedRadius(radiusPanel, space2)*s).AlignItems(ui.Center).Gap(space2*s).
		Background(item.AnimateColor("choice-bg", bg, motionFast)).Border(s, edge), fg
}

func selectPopupPanel(c *ui.Context, popup, trigger ui.Element) {
	s := uiScale(c)
	w, _ := c.Size()
	width := min(trigger.LayoutBounds().W, max(1, w-2*space1*s))
	popover(popup.AttachTo(trigger, ui.AnchorBottomStart, ui.AnchorTopStart).Width(width).MinWidth(0).
		Margin(space1*s, 0, 0, 0).Padding(space2*s).Font(uiSansFamily()), radiusPanel*s, s)
}

// Every option has a fixed, single-line height, so reserve the thumb's strip
// only when those rows need to scroll. Computing this before layout also
// removes the gutter immediately when a search makes a long list short.
func selectOptionsScroll(c *ui.Context, key string, trigger ui.Element, count int) ui.Element {
	s := uiScale(c)
	_, h := c.Size()
	r := trigger.LayoutBounds()
	room := max(r.Y, h-r.Y-r.H) - space1*s - space2*s - 2*(space2+1)*s
	height := max(1, min(selectMaxHeight*s, room))
	contentHeight := (float32(count)*selectRowHeight + float32(max(0, count-1))*space1) * s
	gutter := float32(0)
	if contentHeight > height+.5 {
		gutter = scrollGutter * s
	}
	return scrollArea(c, key).FillWidth().MinWidth(0).MaxHeight(height).Padding(0, gutter, 0, 0)
}

func loaderSelectBox(c *ui.Context, key, labelText string, selected *string, options []string) ui.Element {
	return selectBoxWithIcons(c, key, labelText, selected, options, func(opt string) (string, ui.Color) {
		return loaderIconName(opt), loaderTone(opt)
	})
}
