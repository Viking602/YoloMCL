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
	row := ui.Row(c.Key("discover-mc-box")).FillWidth().Height(btnMd*s).Padding(0, space4*s).Font(uiSansFamily()).Radius(radiusControl * s).Clip().AlignItems(ui.Center).Gap(space2 * s)
	var cb ui.ComboboxParts
	chevronClicked := false
	clearClicked := false
	row.Children(func() {
		cb = ui.ComboboxBase(c.Key("discover-mc"), &a.discoverMCText)
		cb.Input.Label(a.tr().DiscoverFilterMCAria).FontSize(typeBody * s).TextColor(ink).Grow(1).Shrink(1).Basis(0).MinWidth(0)
		if a.filterMC != "" {
			clr := tooltip(c, ui.ButtonBase(c.Key("discover-mc-clear")).Label(a.L("discover_versions.clear_version_filter")), allLabel).Size(18*s, 18*s).Radius(radiusPill).Center().Shrink(0)
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
			selectArrow(c, cb.Open(), s)
		}).Clicked() {
			chevronClicked = true
		}
	})
	open := cb.Open()
	styleSelectField(row, s, open || cb.Input.Focused())
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
		selectPopupPanel(c, popupPanel, row)
		popupPanel.TransformOrigin(.5, 0).Transition(ui.ElementTransition{Animation: &motionEnter, Colors: true, Enter: &ui.Motion{Opacity: 0, Y: -6 * s, Scale: .98}})
		selectOptionsScroll(c, "select-scroll-discover-mc", row, len(shown)).Children(func() {
			ui.Column(c).FillWidth().Gap(4 * s).Children(func() {
				for _, opt := range shown {
					option := opt
					isSelected := option == selected
					item, fgCol := selectOptionStyle(cb.Item(option), isSelected, s)
					item.Children(func() {
						selectLabel(c, option, fgCol, s)
						selectCheck(c, isSelected, fgCol, s)
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
