package nativeui

import (
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type modDisableState struct {
	rootID, version, file, name string
	sha1, iconURL               string
}

func (a *App) toggleModFn(m launcher.Mod) func() {
	return func() {
		r := a.root()
		if r == nil {
			return
		}
		if m.Enabled {
			name, _ := a.itemNames(a.modItem(m))
			iconURL := a.modIconURLs[m.SHA1]
			for _, entry := range a.addonEntries() {
				if entry.mod != nil && entry.it.file == m.File {
					name, _ = a.itemNames(entry.it)
					iconURL = entry.it.iconURL
					break
				}
			}
			a.openDialog("disableMod")
			a.modDisable = modDisableState{rootID: r.ID, version: r.SelectedVersion, file: m.File, name: name, sha1: m.SHA1, iconURL: iconURL}
			return
		}
		id, file := r.ID, m.File
		a.toggleAsync("mods", file, func() error { return a.service.ToggleMod(id, file) })
	}
}

func (a *App) modDisableReady() bool {
	r, target := a.root(), a.modDisable
	if !a.dialogOpen || a.dialog != "disableMod" || r == nil || r.ID != target.rootID || r.SelectedVersion != target.version || a.selectionPending || a.modsLoading {
		return false
	}
	for _, mod := range a.mods {
		if mod.File == target.file && mod.Enabled {
			return true
		}
	}
	return false
}

func (a *App) disableModDialog(c *ui.Context) {
	s, target := uiScale(c), a.modDisable
	iconURL := target.iconURL
	if iconURL == "" {
		iconURL = a.modIconURLs[target.sha1] // metadata can arrive while the dialog is open
	}
	icon := a.addonIcon(target.rootID, target.version, "mod", target.file, iconURL, target.sha1)
	ui.Column(c).Font(uiSansFamily()).FillWidth().Gap(18 * s).Children(func() {
		ui.Row(c).FillWidth().Padding(14*s, 16*s).Gap(12 * s).Radius(12 * s).Background(raised).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(40*s, 40*s).Radius(10 * s).Clip().Background(panel).Shrink(0).Center().Children(func() {
				if icon != nil {
					ui.Image(c, icon).Label("禁用模组图标").Size(40*s, 40*s).Fit(ui.Contain)
				} else {
					letterTile(c, target.name, 40*s, false)
				}
			})
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(5 * s).Children(func() {
				tooltip(c, ui.Text(c, target.name).FillWidth().FontSize(typeHeadline*s).FontWeight(600).TextColor(ink).MaxLines(2), target.name)
				tooltip(c, ui.Text(c, target.file).FillWidth().FontSize(typeCaption*s).TextColor(muted).SingleLine(), target.file)
			})
		})
		ui.Text(c, a.L("mod_disable.this_mod_won_t_load_the_next_time")).FillWidth().FontSize(typeCallout * s).TextColor(muted)
		ui.Row(c).FillWidth().Gap(8 * s).Justify(ui.End).Children(func() {
			if button(c, "cancel-disable-mod", a.L("install_dialog.cancel"), "", false, !a.dialogOpen) {
				a.closeDialog()
			}
			if button(c, "confirm-disable-mod", a.L("mod_disable.disable_mod"), "", true, !a.modDisableReady()) {
				a.closeDialog()
				a.toggleAsync("mods", target.file, func() error { return a.service.ToggleMod(target.rootID, target.file) })
			}
		})
	})
}
