package nativeui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/buildinfo"
)

func (a *App) aboutSettings(c *ui.Context) {
	s := uiScale(c)
	info := buildinfo.Current()
	glass(ui.Column(c.Key("关于 YoloMCL 面板")).Label("关于 YoloMCL 面板").FillWidth().Padding(space3*s, settingPadX*s).Gap(12*s), radiusCard*s-4*s, s).Children(func() {
		ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
			brandMark(c, 30*s)
			ui.Text(c, "YoloMCL").Font("MCL Pixel").FontSize(typeTitle2 * s).TextColor(ink).NoWrap().Shrink(0)
			version := info.Version
			if info.Dirty {
				version += a.L("about.modified")
			}
			label(c, version).Label("launcher-version").FontSize(typeCaption * s).NoWrap().Shrink(0)
			ui.Box(c).Grow(1)
			if iconOnlyButton(c, "about-github", "github", "GitHub / "+buildinfo.Repository, btnSm*s, false) {
				c.OpenURL(buildinfo.RepositoryURL)
			}
			if fixedButton(c, "about-commit", info.ShortCommit(), "", 0, btnSm*s, false, info.Commit == "") {
				c.OpenURL(buildinfo.RepositoryURL + "/commit/" + info.Commit)
			}
			if fixedButton(c, "open-data", a.tr().SettingsOpenDataBtn, "folder", 0, btnSm*s, false, false) {
				a.executeDialog(func() error { return openPath(a.snapshot.DataDir) })
			}
		})
		ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
			label(c, a.launcherUpdateText()).Label("launcher-update-status").Grow(1).MinWidth(0).FontSize(typeCaption * s)
			a.launcherUpdateButton(c)
		})
		if state := a.launcherUpdate; state.phase == "installing" && state.total > 0 {
			ui.Progress(c, float64(state.downloaded)/float64(state.total))
		}
	})
}

func (a *App) launcherUpdateText() string {
	state := a.launcherUpdate
	switch state.phase {
	case "checking":
		return a.L("about.checking_github_for_updates")
	case "current":
		return a.L("about.you_re_on_the_latest_stable_release")
	case "available":
		if state.err != nil {
			if a.launcherUpdateRequired() {
				return a.L("about.update_failed_retry_to_update_this_version_remains")
			}
			return a.L("about.update_failed_you_can_retry_the_current_version")
		}
		return a.L("about.new_release") + "v" + strings.TrimPrefix(state.release.version, "v")
	case "installing":
		if state.total > 0 {
			return fmt.Sprintf(a.L("about.downloading_and_verifying"), min(100, 100*state.downloaded/state.total))
		}
		return a.L("about.downloading_and_verifying_update")
	case "ready":
		if a.launcherRestartBlocked() {
			return a.L("about.update_ready_finish_active_games_and_tasks_before")
		}
		return a.L("about.update_ready_restart_to_use_the_new_version")
	case "disabled":
		return a.L("about.this_build_or_location_cannot_update_in_place")
	case "failed":
		if a.launcherUpdateRequired() {
			return a.L("about.can_t_reach_the_update_service_retry_when")
		}
		return a.L("about.couldn_t_check_for_updates_check_your_connection")
	default:
		return a.L("about.stable_release_updates_from_github")
	}
}

func (a *App) launcherUpdateButton(c *ui.Context) {
	s := uiScale(c)
	switch a.launcherUpdate.phase {
	case "disabled":
		if fixedButton(c, "launcher-download", a.L("about.get_release"), "download", 0, btnSm*s, false, false) {
			c.OpenURL(buildinfo.ReleasesURL + "/latest")
		}
	case "available", "installing":
		if fixedButton(c, "launcher-install", a.L("about.view_update"), "download", 0, btnSm*s, true, false) {
			a.openDialog("launcherUpdate")
		}
	case "ready":
		if fixedButton(c, "launcher-restart", a.L("about.restart_launcher"), "refresh", 0, btnSm*s, true, a.launcherRestartBlocked()) {
			mygo.App.Relaunch()
		}
	default:
		if fixedButton(c, "launcher-check", a.L("about.check_for_updates"), "refresh", 0, btnSm*s, false, a.launcherUpdate.phase == "checking") {
			a.checkLauncherUpdates()
		}
	}
}
