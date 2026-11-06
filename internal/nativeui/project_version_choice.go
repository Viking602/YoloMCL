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
	all := a.L("discover_versions.all_game_versions")
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
	row := ui.Row(c.Key("project-mc-box")).FillWidth().Height(btnMd*s).Padding(0, space4*s).Font(uiSansFamily()).Radius(radiusControl * s).AlignItems(ui.Center).Gap(space2 * s)
	var cb ui.ComboboxParts
	arrowClicked := false
	row.Children(func() {
		cb = ui.ComboboxBase(c.Key("project-mc"), &st.text)
		input := cb.Input.Label(a.L("project_version_choice.project_game_version_filter")).FontSize(typeBody * s).TextColor(ink).Grow(1).Basis(0).MinWidth(0)
		textTooltip(c, input, st.text, "", ui.Font{Family: uiSansFamily(), Size: typeBody * s, Weight: 400}, 1)
		if ui.ButtonBase(c.Key("project-mc-arrow")).Label(a.L("project_version_choice.choose_game_version")).Size(18*s, 18*s).Center().Shrink(0).Children(func() {
			selectArrow(c, cb.Open(), s)
		}).Clicked() {
			arrowClicked = true
		}
	})
	styleSelectField(row, s, cb.Open() || cb.Input.Focused())
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
		selectPopupPanel(c, p, row)
		p.Transition(heroSwap(c))
		selectOptionsScroll(c, "project-mc-options", row, len(shown)).Label("游戏版本选项").Children(func() {
			ui.Column(c).FillWidth().Gap(4 * s).Children(func() {
				if len(shown) == 0 {
					ui.Text(c, a.L("project_version_choice.no_matching_game_version")).FontSize(typeFootnote * s).TextColor(muted).Padding(12 * s)
				}
				for _, opt := range shown {
					item, fg := selectOptionStyle(cb.Item(opt).Label("游戏版本选项 "+opt), opt == selected, s)
					item.Children(func() {
						selectLabel(c, opt, fg, s)
						selectCheck(c, opt == selected, fg, s)
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
