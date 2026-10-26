package nativeui

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// Version choices reuse the install manifest and the in-memory instance
// inventory. Opening a filter never scans Minecraft folders or Java again.
func (a *App) discoverVersionOptions() []string {
	options := []string{a.L("discover_versions.all_game_versions")}
	seen := map[string]bool{"": true}
	add := func(id string) {
		id = strings.TrimSpace(id)
		if !seen[id] {
			seen[id] = true
			options = append(options, id)
		}
	}
	// Keep the current selection available even when offline or when it is
	// an older snapshot hidden by the user's snapshot preference.
	add(a.filterMC)
	if v := a.gameVersion(); v != nil {
		add(v.Minecraft)
	}
	for _, v := range a.minecraftVersions {
		if v.Type == "release" || a.settings.ShowSnapshots {
			add(v.ID)
		}
	}
	for _, root := range a.snapshot.Instances {
		for _, v := range root.Versions {
			add(v.Minecraft)
		}
	}
	return options
}

func matchingVersionOptions(options []string, query string) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return options
	}
	var starts, contains []string
	for _, o := range options {
		switch l := strings.ToLower(o); {
		case strings.HasPrefix(l, q):
			starts = append(starts, o)
		case strings.Contains(l, q):
			contains = append(contains, o)
		}
	}
	return append(starts, contains...)
}

func (a *App) discoverVersionFilter(c *ui.Context) {
	s := uiScale(c)
	allLabel := a.L("discover_versions.all_game_versions")
	selected := a.filterMC
	if selected == "" {
		selected = allLabel
	}
	if !a.discoverMCOpen && a.discoverMCText != selected {
		a.discoverMCText = selected
	}
	wasOpen := a.discoverMCOpen
	row := ui.Row(c.Key("discover-mc-box")).FillWidth().Height(btnMd*s).Padding(6*s, 12*s).Radius(radiusControl * s).Clip().AlignItems(ui.Center).Gap(8 * s)
	var cb ui.ComboboxParts
	chevronClicked := false
	clearClicked := false
	row.Children(func() {
		cb = ui.ComboboxBase(c.Key("discover-mc"), &a.discoverMCText)
		cb.Input.Label(a.tr().DiscoverFilterMCAria).FontSize(typeBody * s).TextColor(ink).Grow(1).Shrink(1).Basis(0).MinWidth(0)
		if a.filterMC != "" {
			clr := tooltip(c, ui.ButtonBase(c.Key("discover-mc-clear")).Label(a.L("discover_versions.clear_version_filter")), allLabel).Size(18*s, 18*s).Radius(999).Center().Shrink(0)
			if clr.Hovered() {
				clr.Background(ui.RGBA(120, 130, 160, .16))
			}
			if clr.Children(func() {
				ui.Icon(c, icons["cross"]).Size(9*s, 9*s).TextColor(muted)
			}).Clicked() {
				clearClicked = true
			}
		}
		arrow := ui.ButtonBase(c.Key("discover-mc-arrow")).Label(a.L("discover_versions.toggle_version_list")).Size(18*s, 18*s).Center().Shrink(0)
		if arrow.Children(func() {
			ui.Icon(c, icons["chevron-down"]).Size(15*s, 15*s).TextColor(muted)
		}).Clicked() {
			chevronClicked = true
		}
	})
	open := cb.Open()
	triggerBg := fieldColor
	if row.Hovered() || cb.Input.Hovered() || open {
		triggerBg = fieldHover
	}
	row.Background(row.AnimateColor("bg", triggerBg, fastAnim))
	if open || cb.Input.Focused() {
		row.Border(1, accent.Alpha(.35))
	}
	if clearClicked {
		a.filterMC = ""
		a.discoverMCText = allLabel
		a.discoverMCOpen = false
		cb.SetOpen(false)
		a.search()
	} else if chevronClicked {
		next := !wasOpen
		cb.SetOpen(next)
		open = next
		if next {
			cb.Input.Focus()
			cb.Input.SetTextSelection(0, len([]rune(a.discoverMCText)))
		}
	} else if cb.Input.Clicked() && !wasOpen && open && !cb.Filtering() {
		cb.Input.SetTextSelection(0, len([]rune(a.discoverMCText)))
	}
	cb.Popup(func(popupPanel ui.Element) {
		// The closed control needs only its selected label, not the entire
		// manifest rebuilt on every animation or scroll frame.
		shown := a.discoverVersionOptions()
		if cb.Filtering() {
			shown = matchingVersionOptions(shown, a.discoverMCText)
		}
		popupPanel.AttachTo(row, ui.AnchorBottomStart, ui.AnchorTopStart).MinWidth(row.LayoutBounds().W)
		popover(popupPanel.Margin(4*s, 0, 0, 0).Padding(6*s), 14*s, s)
		popupPanel.TransformOrigin(.5, 0).Transition(ui.ElementTransition{Animation: &motionEnter, Colors: true, Enter: &ui.Motion{Opacity: 0, Y: -6 * s, Scale: .98}})
		scrollArea(c, "select-scroll-discover-mc").MaxHeight(250*s).FillWidth().Padding(0, scrollGutter*s, 0, 0).Children(func() {
			ui.Column(c).FillWidth().Gap(2 * s).Children(func() {
				for _, opt := range shown {
					option := opt
					item := cb.Item(option).FillWidth().Padding(8*s, 12*s).Radius(9 * s).AlignItems(ui.Center).Gap(8 * s)
					isSelected := option == selected
					fgCol := ink
					if item.Highlighted() {
						item.LinearGradient(appleAccentGradient(false))
						fgCol = panel
					} else if isSelected {
						item.Background(accentSoft)
						fgCol = accent
					}
					item.Children(func() {
						ui.Text(c, option).FontSize(typeCallout * s).TextColor(fgCol).Grow(1).Basis(0).MinWidth(0).SingleLine()
						if isSelected {
							ui.Icon(c, icons["check"]).Size(14*s, 14*s).TextColor(fgCol).Shrink(0)
						}
					})
				}
			})
		})
	})
	if chosen, ok := cb.Chosen(); ok {
		next := chosen
		if next == allLabel {
			next = ""
		}
		a.discoverMCText = chosen
		cb.Input.SetTextSelection(0, len([]rune(chosen)))
		if a.filterMC != next {
			a.filterMC = next
			a.search()
		}
	} else if !cb.Input.Focused() && !cb.Open() && a.discoverMCText != selected {
		a.discoverMCText = selected
	}
	a.discoverMCOpen = cb.Open()
	if a.manifestError != "" {
		if button(c, "discover-versions-retry", a.L("discover_versions.retry_version_list"), "refresh", false, a.manifestLoading) {
			a.loadManifest()
		}
	}
}
