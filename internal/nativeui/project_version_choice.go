package nativeui

import (
	"slices"

	"github.com/egoist/mygo/ui"
)

type projectChoiceState struct {
	text string
	open bool
}

// A searchable, scrollable list keeps every API version reachable, including
// snapshots and old releases. Editing text never commits a partial version.
func (a *App) projectMinecraftChoice(c *ui.Context, versions []string) bool {
	s, st := uiScale(c), &a.projectMCChoice
	all := a.L("全部游戏版本", "All game versions")
	selected := a.verFilter.mc
	if selected == "" {
		selected = all
	} else if !slices.Contains(versions, selected) {
		versions = append([]string{selected}, versions...)
	}
	options := append([]string{all}, versions...)
	if !st.open {
		st.text = selected
	}
	wasOpen := st.open
	row := ui.Row(c.Key("project-mc-box")).FillWidth().Height(btnMd*s).Padding(6*s, 12*s).Radius(radiusControl * s).AlignItems(ui.Center).Gap(8 * s)
	var cb ui.ComboboxParts
	arrowClicked := false
	row.Children(func() {
		cb = ui.ComboboxBase(c.Key("project-mc"), &st.text)
		cb.Input.Label(a.L("项目游戏版本筛选", "Project game version filter")).FontSize(typeBody * s).TextColor(ink).Grow(1).Basis(0).MinWidth(0).Tooltip(selected)
		if ui.ButtonBase(c.Key("project-mc-arrow")).Label(a.L("展开游戏版本", "Choose game version")).Size(18*s, 18*s).Center().Shrink(0).Children(func() {
			ui.Icon(c, icons["chevron-down"]).Size(15*s, 15*s).TextColor(muted)
		}).Clicked() {
			arrowClicked = true
		}
	})
	color := fieldColor
	if row.Hovered() || cb.Open() {
		color = fieldHover
	}
	row.Background(row.AnimateColor("bg", color, fastAnim)).Border(1, line)
	if cb.Open() || cb.Input.Focused() {
		row.Border(1, accent.Alpha(.35))
	}
	if arrowClicked {
		cb.SetOpen(!wasOpen)
		if !wasOpen {
			cb.Input.Focus()
			cb.Input.SetTextSelection(0, len([]rune(st.text)))
		}
	} else if cb.Input.Clicked() && !wasOpen && cb.Open() && !cb.Filtering() {
		cb.Input.SetTextSelection(0, len([]rune(st.text)))
	}
	shown := options
	if cb.Filtering() {
		shown = matchingVersionOptions(options, st.text)
	}
	cb.Popup(func(p ui.Element) {
		p.AttachTo(row, ui.AnchorBottomStart, ui.AnchorTopStart).Width(row.LayoutBounds().W)
		popover(p.Margin(4*s, 0, 0, 0).Padding(6*s), 14*s, s).Transition(heroSwap(c))
		scrollArea(c, "project-mc-options").Label("游戏版本选项").MaxHeight(250*s).FillWidth().Padding(0, scrollGutter*s, 0, 0).Children(func() {
			ui.Column(c).FillWidth().Gap(2 * s).Children(func() {
				if len(shown) == 0 {
					ui.Text(c, a.L("没有匹配的游戏版本", "No matching game version")).FontSize(typeFootnote * s).TextColor(muted).Padding(10 * s)
				}
				for _, opt := range shown {
					item := cb.Item(opt).Label("游戏版本选项 "+opt).FillWidth().Padding(8*s, 12*s).Radius(8 * s).AlignItems(ui.Center).Gap(8 * s)
					fg := ink
					if item.Highlighted() || opt == selected {
						item.Background(accentSoft)
						fg = accent
					}
					item.Children(func() {
						ui.Text(c, opt).FontSize(typeCallout * s).TextColor(fg).Grow(1).Basis(0).MinWidth(0).Tooltip(opt)
						if opt == selected {
							ui.Icon(c, icons["check"]).Size(12*s, 12*s).TextColor(fg).Shrink(0)
						}
					})
				}
			})
		})
	})
	changed := false
	if chosen, ok := cb.Chosen(); ok {
		next := chosen
		if next == all {
			next = ""
		}
		changed = a.verFilter.mc != next
		a.verFilter.mc, st.text = next, chosen
		cb.Input.SetTextSelection(0, len([]rune(chosen)))
	} else if !cb.Input.Focused() && !cb.Open() {
		st.text = selected
	}
	st.open = cb.Open()
	return changed
}
