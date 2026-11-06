package nativeui

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func (a *App) keyBindingsDialog(c *ui.Context) {
	g := a.keyBindings
	if g == nil {
		return
	}
	a.applyKeyBindingLabels(g)
	s := uiScale(c)
	if g.tab == "" {
		g.tab = "keys"
	}
	description := "keybindings.global_desc"
	if g.rootID != "" {
		description = "keybindings.pack_desc"
	}
	label(c, a.L(description)).FontSize(typeFootnote * s).FillWidth().MinWidth(0).Shrink(0)
	if g.view == nil {
		if a.dialogPending() {
			a.gameSettingsSkeleton(c, g.rootID != "")
		} else if button(c, "keybindings-retry", a.L("keybindings.reload"), "refresh", false, false) {
			a.loadKeyBindings()
		}
		if !a.dialogPending() {
			ui.Spacer(c).Grow(1)
		}
		return
	}
	tabs := []string{"keys", "video", "audio", "controls"}
	if g.rootID != "" {
		tabs = append(tabs, "other")
	}
	tabs = append(tabs, "backups")
	items := make([]underlineTab, len(tabs))
	for i, tab := range tabs {
		items[i] = underlineTab{tab, a.L("game_options.tab." + tab)}
	}
	tab := underlineTabs(c, "preferences-tabs", items, g.tab, underlineTabStyle{
		label: a.L("game_options.categories"), indicator: a.L("game_options.tab_indicator"), expanded: true,
	})
	if tab != g.tab {
		g.tab, g.query, g.capture, g.manual = tab, "", "", ""
		if tab == "backups" && !g.backupsLoaded {
			a.loadGameSettingsBackups(g)
		}
	}
	if g.tab == "backups" {
		a.gameSettingsBackupsView(c, g)
		return
	}
	if g.rootID != "" && !g.view.HasOptions {
		label(c, a.L("keybindings.empty")).FillWidth().MinWidth(0)
		ui.Spacer(c).Grow(1)
		return
	}
	ui.Row(c).FillWidth().Shrink(0).Gap(8 * s).AlignItems(ui.Center).Children(func() {
		textField(c.Key("keybindings-search"), &g.query).Label(a.L("keybindings.search")).Placeholder(a.L("keybindings.search")).Grow(1).Basis(0).MinWidth(120 * s)
		if g.tab == "keys" && g.rootID != "" {
			ids, names := a.bindingGroups(g)
			selectBoxFull(c, "keybindings-mod", a.L("game_options.mod"), &g.mod, ids, nil, func(id string) string { return names[id] }).Width(190 * s)
			selectBoxFull(c, "keybindings-filter", a.L("keybindings.filter"), &g.scope, []string{"all", "vanilla", "mods", "untranslated"}, nil, func(id string) string { return a.L("keybindings.filter." + id) }).Width(145 * s)
		}
	})
	if g.view.Shared {
		label(c, a.L("keybindings.shared")).FontSize(typeCaption * s).FillWidth().MinWidth(0).Shrink(0)
	}
	ui.Column(c).FillWidth().Grow(1).Basis(0).MinHeight(0).Clip().Children(func() {
		filter := strings.Join([]string{g.tab, g.mod, g.scope, g.query}, "\x00")
		list := scrollArea(c, "preferences-list").Label(a.L("game_options.list")).FillWidth().Grow(1).MinHeight(0).Padding(4*s, 8*s, 8*s, 4*s).Gap(12 * s)
		if filter != g.listFilter {
			scrollTracks["preferences-list"].st.Y = 0
			g.listFilter = filter
			g.capture = ""
		}
		list.Children(func() {
			if g.tab == "keys" {
				a.keyBindingGroups(c, g)
			} else {
				a.gamePreferenceRows(c, g)
			}
		})
	})
	if g.tab == "keys" {
		hint := "game_options.record_hint"
		if g.labelsLoading {
			hint = "game_options.labels_loading"
		} else if g.capture != "" {
			hint = "game_options.recording_hint"
		}
		label(c, a.L(hint)).FontSize(typeCaption * s).FillWidth().MinWidth(0).Shrink(0)
	}
}

func bindingGroupID(row launcher.KeyBindingRow) string {
	if row.Vanilla {
		return "minecraft"
	}
	if row.Namespace == "" {
		return "unassigned"
	}
	return "mod:" + row.Namespace
}

func (a *App) bindingGroups(g *keyBindingsState) ([]string, map[string]string) {
	counts, names := map[string]int{}, map[string]string{"all": a.L("game_options.all_mods")}
	for _, row := range g.view.Rows {
		id := bindingGroupID(row)
		counts[id]++
		switch id {
		case "minecraft":
			names[id] = a.L("game_options.vanilla")
		case "unassigned":
			names[id] = a.L("game_options.unassigned")
		default:
			names[id] = row.ModName
			if names[id] == "" {
				names[id] = row.Namespace
			}
		}
	}
	ids := []string{"all"}
	for id := range counts {
		ids = append(ids, id)
	}
	slices.SortFunc(ids[1:], func(a, b string) int {
		if a == "minecraft" {
			return -1
		}
		if b == "minecraft" {
			return 1
		}
		return strings.Compare(names[a], names[b])
	})
	for id, n := range counts {
		names[id] += fmt.Sprintf(" (%d)", n)
	}
	return ids, names
}

func (a *App) keyBindingGroups(c *ui.Context, g *keyBindingsState) {
	ids, names := a.bindingGroups(g)
	count := 0
	for _, id := range ids[1:] {
		if g.mod != "" && g.mod != "all" && g.mod != id {
			continue
		}
		var rows []launcher.KeyBindingRow
		for _, row := range g.view.Rows {
			if bindingGroupID(row) == id && a.keyBindingMatches(row, g.query, g.scope) {
				rows = append(rows, row)
			}
		}
		if len(rows) == 0 {
			continue
		}
		count += len(rows)
		a.preferenceCard(c, "bindings-"+id, names[id], func() {
			for i, row := range rows {
				if i > 0 {
					ui.Box(c).FillWidth().Height(1).Background(line)
				}
				a.keyBindingRow(c, g, row, a.keyBindingName(row))
			}
		})
	}
	if count == 0 {
		label(c, a.L("keybindings.no_matches"))
	}
}

func (a *App) preferenceCard(c *ui.Context, id, title string, content func()) {
	s := uiScale(c)
	ui.Column(c.Key(id)).FillWidth().Radius(radiusControl*s).Background(ui.Hex("#ffffff")).Border(1, line).Children(func() {
		if title != "" {
			ui.Row(c).FillWidth().Padding(12*s, 16*s).Background(ui.Hex("#f5f7fc")).Radius(radiusControl * s).Children(func() {
				ui.Text(c, title).FontSize(typeCallout * s).FontWeight(650).TextColor(inkSecondary).FillWidth().MinWidth(0)
			})
		}
		content()
	})
}

func (a *App) bindingSource(owned bool, source, value string) string {
	if owned {
		return a.L("game_options.customized")
	}
	name := a.L("keybindings.game")
	if source == "global" {
		name = a.L("keybindings.global")
	}
	if value != "" {
		return fmt.Sprintf(a.L("keybindings.source_value"), name, value)
	}
	return name
}

func (a *App) preferenceReset(c *ui.Context, id, name string, owned bool) bool {
	s := uiScale(c)
	e := ui.ButtonBase(c.Key("binding-inherit-"+id)).Label(a.L("keybindings.inherit")+" "+name).Size(30*s, 30*s).Radius(radiusChip * s).Center().Disabled(!owned || a.dialogPending())
	e = tooltip(c, e, a.L("keybindings.inherit"))
	if e.Hovered() {
		e.Background(accentSoft)
	}
	return e.Children(func() { ui.Icon(c, icons["refresh"]).Size(13*s, 13*s).TextColor(muted) }).Clicked()
}

func (a *App) keyBindingRow(c *ui.Context, g *keyBindingsState, row launcher.KeyBindingRow, name string) {
	s := uiScale(c)
	controlName := name
	if !row.Vanilla && len(row.Names) > 0 {
		controlName += " (" + row.ID + ")"
	}
	value, owned := g.bindings[row.ID]
	if !owned {
		value = row.Inherited
	}
	ui.Column(c.Key("binding-row-"+row.ID)).FillWidth().Padding(12*s, 16*s).Gap(8 * s).Children(func() {
		ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4 * s).Children(func() {
				ui.Text(c, name).FontSize(typeBody * s).FontWeight(550).TextColor(ink).FillWidth().MinWidth(0)
				if !row.Vanilla {
					label(c, row.ID).FontSize(typeCaption * s).FillWidth().MinWidth(0)
				}
				label(c, a.bindingSource(owned, row.Source, a.keyBindingValue(row.Inherited))).FontSize(typeCaption * s).FillWidth().MinWidth(0)
			})
			caption := a.keyBindingValue(value)
			if g.capture == row.ID {
				caption = a.L("game_options.recording")
			}
			e := ui.ButtonBase(c.Key("binding-record-"+row.ID)).Label(a.L("game_options.record")+" "+controlName).Width(140*s).Height(34*s).Radius(radiusChip*s).Background(accentSoft).Border(1, accent.Alpha(.18)).Center().Disabled(a.dialogPending())
			e.HandleInput(func(ev ui.InputEvent) bool { return a.captureBinding(g, row, ev) })
			if e.Children(func() {
				ui.Text(c, caption).FontSize(typeCallout * s).FontWeight(650).TextColor(accent).SingleLine().MinWidth(0)
			}).Clicked() {
				g.capture = row.ID
				e.Focus()
			}
			if button(c, "binding-custom-"+row.ID, a.L("game_options.custom"), "", false, a.dialogPending()) {
				g.capture = ""
				if g.manual == row.ID {
					g.manual = ""
				} else {
					g.manual = row.ID
				}
			}
			if a.preferenceReset(c, row.ID, controlName, owned) {
				delete(g.bindings, row.ID)
				delete(g.inputs, row.ID)
				delete(g.invalid, row.ID)
				g.capture = ""
			}
		})
		if g.manual == row.ID {
			a.manualBinding(c, g, row, controlName, value)
		}
	})
}

func (a *App) manualBinding(c *ui.Context, g *keyBindingsState, row launcher.KeyBindingRow, name, value string) {
	s := uiScale(c)
	ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
		text, ok := g.inputs[row.ID]
		if !ok {
			text = value
		}
		input := textField(c.Key("binding-text-"+row.ID), &text).Label(a.L("game_options.custom_value") + " " + name).Placeholder(a.L("game_options.custom_placeholder")).Grow(1).MinWidth(0).Disabled(a.dialogPending())
		if input.Changed() {
			g.inputs[row.ID] = text
			normal, err := parseBindingText(text)
			if err != nil {
				g.invalid[row.ID] = true
			} else {
				delete(g.invalid, row.ID)
				g.bindings[row.ID] = normal
			}
		}
		base, modifier, _ := strings.Cut(value, ":")
		if modifier == "" {
			modifier = "NONE"
		}
		choices := launcher.KeyBindingChoices()
		if !slices.Contains(choices, base) {
			choices = append(choices, base)
		}
		changed := selectBoxFull(c, "binding-value-"+row.ID, a.L("keybindings.value")+" "+name, &base, choices, nil, a.keyBindingValue).Width(150 * s).Disabled(a.dialogPending()).Changed()
		if g.rootID != "" && (g.view.Modifiers || modifier != "NONE") {
			changed = selectBoxFull(c, "binding-modifier-"+row.ID, a.L("keybindings.modifier")+" "+name, &modifier, []string{"NONE", "CONTROL", "SHIFT", "ALT"}, nil, func(v string) string { return a.L("keybindings.modifier." + v) }).Width(100*s).Disabled(a.dialogPending()).Changed() || changed
		}
		if changed {
			if modifier != "NONE" && base != "key.keyboard.unknown" {
				base += ":" + modifier
			}
			g.bindings[row.ID] = base
			delete(g.inputs, row.ID)
			delete(g.invalid, row.ID)
		}
	})
	if g.invalid[row.ID] {
		label(c, a.L("game_options.invalid_key")).FontSize(typeCaption * s).TextColor(danger)
	}
}

func (a *App) gamePreferenceRows(c *ui.Context, g *keyBindingsState) {
	var rows []launcher.GamePreferenceRow
	for _, row := range g.view.Options {
		name := row.ID
		if row.Label != "" {
			name = a.L(row.Label)
		}
		if row.Group == g.tab && strings.Contains(strings.ToLower(name+" "+row.ID), strings.ToLower(strings.TrimSpace(g.query))) {
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		label(c, a.L("game_options.empty_category"))
		return
	}
	a.preferenceCard(c, "preferences-"+g.tab, a.L("game_options.tab."+g.tab), func() {
		for i, row := range rows {
			if i > 0 {
				ui.Box(c).FillWidth().Height(1).Background(line)
			}
			a.gamePreferenceRow(c, g, row)
		}
	})
}

func (a *App) gamePreferenceRow(c *ui.Context, g *keyBindingsState, row launcher.GamePreferenceRow) {
	s := uiScale(c)
	name := row.ID
	if row.Label != "" {
		name = a.L(row.Label)
	}
	value, owned := g.options[row.ID]
	if !owned {
		value = row.Inherited
	}
	ui.Column(c.Key("preference-row-"+row.ID)).FillWidth().Padding(12*s, 16*s).Gap(8 * s).Children(func() {
		ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4 * s).Children(func() {
				ui.Text(c, name).FontSize(typeBody * s).FontWeight(550).TextColor(ink).FillWidth().MinWidth(0)
				label(c, a.bindingSource(owned, row.Source, "")).FontSize(typeCaption * s).FillWidth().MinWidth(0)
			})
			ui.Row(c).Width(300 * s).Gap(8 * s).AlignItems(ui.Center).Children(func() {
				a.gamePreferenceInput(c, g, row, name, value)
			})
			if a.preferenceReset(c, row.ID, name, owned) {
				delete(g.options, row.ID)
				delete(g.inputs, row.ID)
				delete(g.invalid, row.ID)
			}
		})
		if g.invalid[row.ID] {
			key := "game_options.range"
			if row.Step*optionScale(row) >= 1 {
				key = "game_options.integer_range"
			}
			label(c, fmt.Sprintf(a.L(key), row.Min*optionScale(row)+row.Offset, row.Max*optionScale(row)+row.Offset)).FontSize(typeCaption * s).TextColor(danger)
		}
	})
}

func optionScale(row launcher.GamePreferenceRow) float64 {
	if row.Scale == 0 {
		return 1
	}
	return row.Scale
}

func (a *App) gamePreferenceInput(c *ui.Context, g *keyBindingsState, row launcher.GamePreferenceRow, name, value string) {
	s := uiScale(c)
	control := a.L("game_options.value") + " " + name
	switch row.Kind {
	case "bool":
		on := value == "true"
		ui.Spacer(c).Grow(1)
		if nativeToggle(c, "preference-"+row.ID, control, &on).Disabled(a.dialogPending()).Changed() {
			g.options[row.ID] = strconv.FormatBool(on)
		}
	case "select":
		choices := slices.Clone(row.Choices)
		if !slices.Contains(choices, value) {
			choices = append(choices, value)
		}
		if selectBoxFull(c, "preference-"+row.ID, control, &value, choices, nil, func(v string) string {
			key := "game_options.choice." + row.ID + "." + v
			text := a.L(key)
			if text == key {
				return v
			}
			return text
		}).FillWidth().Disabled(a.dialogPending()).Changed() {
			g.options[row.ID] = value
		}
	case "number":
		n, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			n = row.Min
		}
		sliderValue := math.Max(row.Min, math.Min(row.Max, n))
		slider := ui.Slider(c.Key("preference-slider-"+row.ID), &sliderValue, row.Min, row.Max).Label(name + " " + a.L("game_options.slider")).Step(row.Step).Grow(1).MinWidth(50 * s).Disabled(a.dialogPending())
		if slider.Changed() {
			n = math.Max(row.Min, math.Min(row.Max, sliderValue))
			g.options[row.ID] = strconv.FormatFloat(n, 'f', 6, 64)
			delete(g.inputs, row.ID)
			delete(g.invalid, row.ID)
		}
		input, ok := g.inputs[row.ID]
		if !ok {
			input = a.preferenceNumberText(row, n)
		}
		width := float32(75)
		if row.MaxLabel != "" {
			width = 105
		}
		field := textField(c.Key("preference-input-"+row.ID), &input).Label(control).Width(width * s).Disabled(a.dialogPending())
		if field.Changed() {
			g.inputs[row.ID] = input
			number, err := strconv.ParseFloat(strings.TrimSpace(input), 64)
			if row.MaxLabel != "" && strings.EqualFold(strings.TrimSpace(input), a.L(row.MaxLabel)) {
				number, err = row.Max*optionScale(row)+row.Offset, nil
			}
			whole := row.Step*optionScale(row) >= 1
			validDisplay := !whole || number == math.Trunc(number)
			number = (number - row.Offset) / optionScale(row)
			if err != nil || !validDisplay || math.IsNaN(number) || math.IsInf(number, 0) || number < row.Min-1e-9 || number > row.Max+1e-9 {
				g.invalid[row.ID] = true
			} else {
				g.options[row.ID] = strconv.FormatFloat(math.Max(row.Min, math.Min(row.Max, number)), 'f', 6, 64)
				delete(g.invalid, row.ID)
			}
		}
		if !field.Focused() && !g.invalid[row.ID] {
			delete(g.inputs, row.ID)
		}
		if row.Unit != "" && !(row.MaxLabel != "" && n == row.Max) {
			label(c, a.L("game_options.unit."+row.Unit)).FontSize(typeCaption * s).NoWrap()
		}
	default:
		if textField(c.Key("preference-input-"+row.ID), &value).Label(control).FillWidth().Disabled(a.dialogPending()).Changed() {
			g.options[row.ID] = value
		}
	}
}

func (a *App) preferenceNumberText(row launcher.GamePreferenceRow, n float64) string {
	if row.MaxLabel != "" && n == row.Max {
		return a.L(row.MaxLabel)
	}
	precision := float64(1000)
	if row.Step*optionScale(row) >= 1 {
		precision = 1
	}
	return strconv.FormatFloat(math.Round((n*optionScale(row)+row.Offset)*precision)/precision, 'f', -1, 64)
}
