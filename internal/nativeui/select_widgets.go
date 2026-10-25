package nativeui

import (
	"strings"

	"github.com/egoist/mygo/ui"
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
func selectBoxImpl(c *ui.Context, key, labelText string, selected *string, options []string, iconFn func(string) (string, ui.Color), display func(string) string, drawIcon func(string), trailing func(opt string, highlighted bool)) ui.Element {
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
	triggerBg := fieldColor
	if sel.Trigger.Hovered() || sel.Open() {
		triggerBg = fieldHover
	}
	if sel.Open() {
		sel.Trigger.Border(1, accent.Alpha(.35))
	}
	if labelText != "" {
		sel.Trigger.Label(labelText)
	}
	sel.Trigger.Height(btnMd*s).Padding(6*s, 12*s).Radius(radiusControl * s).Clip().Background(sel.Trigger.AnimateColor("bg", triggerBg, fastAnim)).AlignItems(ui.Center).Gap(8 * s).Children(func() {
		if drawIcon != nil {
			drawIcon(*selected)
		}
		if iconFn != nil {
			ic, col := iconFn(*selected)
			if loaderLogos[strings.TrimPrefix(ic, "loader-")] != nil {
				loaderLogo(c, *selected, 20*s)
			} else if icons[ic] != nil {
				ui.Icon(c, icons[ic]).Size(16*s, 16*s).TextColor(col).Shrink(0)
			}
		}
		tooltip(c, ui.Text(c, show(*selected)).FontSize(typeBody*s).TextColor(ink).Grow(1).Shrink(1).Basis(0).MinWidth(0).SingleLine(), *selected)
		if trailing != nil {
			trailing(*selected, false)
		}
		ui.Icon(c, icons["chevron-down"]).Size(15*s, 15*s).TextColor(muted).Shrink(0)
	})
	sel.Popup(func(popupPanel ui.Element) {
		popover(popupPanel.Padding(6*s), 14*s, s)
		// Keep the entrance in layout coordinates: transforming this wide,
		// shadowed panel makes the software renderer rasterize every pixel of
		// its shadows and rounded corners again throughout the animation.
		p := sel.Trigger.AnimateTo("popup-reveal", 1, motionEnter)
		popupPanel.Opacity(p).Margin(-6*s*(1-p), 0, 0, 0)
		scrollArea(c, "select-scroll-"+key+"-"+labelText).MaxHeight(250*s).FillWidth().Padding(0, scrollGutter*s, 0, 0).Children(func() {
			ui.Column(c).FillWidth().Gap(2 * s).Children(func() {
				for _, opt := range options {
					option := opt
					item := sel.Item(option).Label(labelText+" "+show(option)).FillWidth().Padding(8*s, 12*s).Radius(9 * s).AlignItems(ui.Center).Gap(8 * s)
					isSelected := option == *selected
					fgCol := ink
					if item.Highlighted() {
						item.LinearGradient(appleAccentGradient(false))
						fgCol = panel
					} else if isSelected {
						item.Background(accentSoft)
						fgCol = accent
					}
					item.Children(func() {
						if drawIcon != nil {
							drawIcon(option)
						}
						if iconFn != nil {
							ic, col := iconFn(option)
							if item.Highlighted() {
								col = panel
							}
							if loaderLogos[strings.TrimPrefix(ic, "loader-")] != nil {
								loaderLogo(c, option, 20*s)
							} else if icons[ic] != nil {
								ui.Icon(c, icons[ic]).Size(16*s, 16*s).TextColor(col).Shrink(0)
							}
						}
						ui.Text(c, show(option)).FontSize(typeCallout * s).TextColor(fgCol).Grow(1).Basis(0).MinWidth(0).SingleLine()
						if trailing != nil {
							trailing(option, item.Highlighted())
						}
						if isSelected {
							ui.Icon(c, icons["check"]).Size(14*s, 14*s).TextColor(fgCol).Shrink(0)
						}
					})
				}
			})
		})
	})
	return sel.Trigger
}

func loaderSelectBox(c *ui.Context, key, labelText string, selected *string, options []string) ui.Element {
	return selectBoxWithIcons(c, key, labelText, selected, options, func(opt string) (string, ui.Color) {
		return loaderIconName(opt), loaderTone(opt)
	})
}
