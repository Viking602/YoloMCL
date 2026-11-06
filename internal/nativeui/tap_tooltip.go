package nativeui

import "github.com/egoist/mygo/ui"

type tapTipState struct{ open, suppressed bool }

// Passive metadata and disabled controls also expose their text on tap.
// Actionable rows keep tooltip(), so their first tap executes the row action.
func tapTooltip(c *ui.Context, anchor ui.Element, text string) ui.Element {
	st := ui.Local(anchor, "tap-tip", func() tapTipState { return tapTipState{} })
	if text == "" {
		st.open = false
		return anchor
	}
	anchor.Description(text).Focusable()
	bounds := anchor.LayoutBounds()
	anchor.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind == ui.InputPointerDown && ev.Button == 0 {
			return true
		}
		if ev.Kind == ui.InputPointerUp && ev.Button == 0 {
			if ev.X >= 0 && ev.Y >= 0 && ev.X < bounds.W && ev.Y < bounds.H {
				st.open, st.suppressed = !st.open, true
			}
			return true
		}
		return false
	})
	if anchor.Shortcut(0, ui.KeyEnter) || anchor.Shortcut(0, ui.KeySpace) {
		st.open, st.suppressed = !st.open, true
	}
	if !st.open && !anchor.Hovered() && !anchor.Focused() {
		st.suppressed = false
	}
	if !st.open && !st.suppressed {
		tooltip(c, anchor, text)
	}
	if st.open {
		ui.PopoverBase(c, anchor, &st.open, func(p ui.Element) {
			p.Role(ui.RoleTooltip)
			tooltipContent(c, anchor, p, text)
		})
	}
	return anchor
}
