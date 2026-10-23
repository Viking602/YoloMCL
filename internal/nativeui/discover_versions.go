package nativeui

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// Version choices reuse the install manifest and the in-memory instance
// inventory. Opening a filter never scans Minecraft folders or Java again.
func (a *App) discoverVersionOptions() []string {
	options := []string{a.L("全部游戏版本", "All game versions")}
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

func (a *App) discoverVersionFilter(c *ui.Context) {
	s := uiScale(c)
	options := a.discoverVersionOptions()
	selected := a.filterMC
	if selected == "" {
		selected = options[0]
	}
	// MyGo's public combobox commits only a chosen option, not each typed
	// character. Mouse and keyboard selection share the same search path.
	box := ui.Combobox(c.Key("discover-mc"), &selected, options).
		Label(a.tr().DiscoverFilterMCAria).FillWidth().Height(btnMd*s).
		Padding(6*s, 12*s).Radius(radiusControl * s).Background(fieldColor)
	if box.Changed() {
		if selected == options[0] {
			selected = ""
		}
		if a.filterMC != selected {
			a.filterMC = selected
			a.search()
		}
	}
	if a.manifestError != "" {
		if button(c, "discover-versions-retry", a.L("重新获取版本列表", "Retry version list"), "refresh", false, a.manifestLoading) {
			a.loadManifest()
		}
	}
}
