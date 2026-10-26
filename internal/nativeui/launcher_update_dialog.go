package nativeui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/buildinfo"
)

// Optional prompts wait for the user's current dialog. A verified retirement
// takes priority and replaces the interactive shell until a supported restart.
func (a *App) prepareLauncherUpdateDialog() {
	state := &a.launcherUpdate
	if a.dialog == "launcherUpdate" && a.dialogOpen {
		state.prompt = false
	}
	if a.launcherUpdateRequired() {
		if !state.locked {
			state.locked = true
			a.cancelAddonLoad()
			a.cancelInstanceStatus()
			a.cancelServerPing()
			if a.dialog != "launcherUpdate" {
				a.closeDialogNow()
			}
		}
		if a.dialog != "launcherUpdate" || !a.dialogOpen {
			a.openDialog("launcherUpdate")
		}
		state.prompt = false
		return
	}
	state.locked = false
	if state.prompt && !a.dialogOpen && a.dialogShown == "" {
		state.prompt = false
		a.openDialog("launcherUpdate")
	}
}

func (a *App) launcherUpdateGate(c *ui.Context) {
	s := uiScale(c)
	root := ui.Column(c.Key("required-update-shell")).Label("启动器更新限制").Fill().Background(bg)
	root.Children(func() {
		ui.Row(c).FillWidth().Height(42).PaddingX(20 * s).Gap(10 * s).AlignItems(ui.Center).DragWindow().Children(func() {
			brandMark(c, 24*s)
			ui.Text(c, "YoloMCL").FontSize(typeHeadline * s).Bold().TextColor(ink)
		})
	})
	a.dialogView(c, root)
}

func (a *App) launcherUpdateDialog(c *ui.Context) {
	s := uiScale(c)
	state := &a.launcherUpdate
	required := a.launcherUpdateRequired()
	version := ""
	if state.release != nil {
		version = "v" + strings.TrimPrefix(state.release.version, "v")
	} else if state.policy != nil {
		version = "v" + state.policy.Version
	}
	ui.Row(c).FillWidth().Gap(14 * s).AlignItems(ui.Center).Children(func() {
		appleSoftIconBox(c, 44*s, "download", accent, accentIndigo, panel)
		ui.Column(c).Grow(1).MinWidth(0).Gap(5 * s).Children(func() {
			if version != "" {
				ui.Text(c, "YoloMCL "+version).FontSize(typeTitle2 * s).Bold().TextColor(ink)
			}
			label(c, a.L("launcher_update_dialog.current_version")+state.currentVersion).FontSize(typeCaption * s)
		})
	})
	if required {
		label(c, fmt.Sprintf(a.L("launcher_update_dialog.this_version_has_been_retired_update_to_v"), state.policy.MinimumVersion)).TextColor(danger)
	} else if state.phase == "available" {
		label(c, a.L("launcher_update_dialog.a_new_version_is_available_download_and_install"))
	}
	label(c, a.L("launcher_update_dialog.your_accounts_settings_and_game_files_will_be")).FontSize(typeCaption * s)
	if state.phase != "available" || state.err != nil {
		label(c, a.launcherUpdateText()).Label("launcher-update-dialog-status")
	}
	if state.phase == "installing" && state.total > 0 {
		ui.Progress(c, float64(state.downloaded)/float64(state.total))
	}
	if state.phase == "available" && state.release != nil && state.release.install == nil {
		label(c, a.L("launcher_update_dialog.this_installation_cannot_update_in_place_download_the")).FontSize(typeCaption * s)
	}
	ui.Row(c).FillWidth().Wrap().Gap(10 * s).AlignItems(ui.Center).Children(func() {
		if button(c, "launcher-release-notes", a.L("launcher_update_dialog.release_notes"), "external", false, false) {
			c.OpenURL(buildinfo.ReleasesURL + "/latest")
		}
		ui.Spacer(c).Grow(1)
		if required {
			if button(c, "launcher-update-exit", a.L("launcher_update_dialog.exit_launcher"), "cross", false, false) && a.win != nil {
				a.win.Close()
			}
		} else {
			text := a.L("install_dialog.cancel")
			if state.phase == "installing" {
				text = a.L("launcher_update_dialog.download_in_background")
			}
			if button(c, "launcher-update-cancel", text, "", false, false) {
				a.closeDialog()
			}
		}
		switch state.phase {
		case "available":
			if state.release != nil && state.release.install != nil {
				if button(c, "launcher-update-confirm", a.L("launcher_update_dialog.update_now"), "download", true, false) {
					a.installLauncherUpdate()
				}
			} else {
				if button(c, "launcher-update-retry", a.L("launcher_update_dialog.check_again"), "refresh", false, false) {
					a.checkLauncherUpdates()
				}
				if button(c, "launcher-update-download", a.L("launcher_update_dialog.download_release"), "download", true, false) {
					c.OpenURL(buildinfo.ReleasesURL + "/latest")
				}
			}
		case "ready":
			if button(c, "launcher-update-restart", a.L("about.restart_launcher"), "refresh", true, a.launcherRestartBlocked()) {
				mygo.App.Relaunch()
			}
		case "checking", "installing":
			button(c, "launcher-update-busy", a.L("launcher_update_dialog.please_wait"), "", true, true)
		default:
			if button(c, "launcher-update-retry", a.L("launcher_update_dialog.check_again"), "refresh", true, false) {
				a.checkLauncherUpdates()
			}
		}
	})
}
