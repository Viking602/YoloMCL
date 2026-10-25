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
				version += a.L("（本地修改）", " (modified)")
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
		return a.L("正在检查 GitHub 更新…", "Checking GitHub for updates…")
	case "current":
		return a.L("已是最新正式版本", "You're on the latest stable release")
	case "available":
		if state.err != nil {
			if a.launcherUpdateRequired() {
				return a.L("更新未完成，请重试。此版本仍处于停用状态", "Update failed. Retry to update; this version remains retired")
			}
			return a.L("更新未完成，可以重试；当前版本仍可使用", "Update failed. You can retry; the current version is still usable")
		}
		return a.L("新版本 ", "New release ") + "v" + strings.TrimPrefix(state.release.version, "v")
	case "installing":
		if state.total > 0 {
			return fmt.Sprintf(a.L("正在下载并校验更新 %d%%", "Downloading and verifying %d%%"), min(100, 100*state.downloaded/state.total))
		}
		return a.L("正在下载并校验更新…", "Downloading and verifying update…")
	case "ready":
		if a.launcherRestartBlocked() {
			return a.L("更新已准备好，游戏和任务结束后可重启", "Update ready. Finish active games and tasks before restarting")
		}
		return a.L("更新已准备好，重启后生效", "Update ready. Restart to use the new version")
	case "disabled":
		return a.L("此构建或安装位置不支持原位更新，可从 GitHub 下载正式版", "This build or location cannot update in place. Download a release from GitHub")
	case "failed":
		if a.launcherUpdateRequired() {
			return a.L("无法连接更新服务。请检查网络后重试，或从发布页面下载新版", "Can't reach the update service. Retry when connected or download from the release page")
		}
		return a.L("无法检查更新，请检查网络后重试", "Couldn't check for updates. Check your connection and retry")
	default:
		return a.L("通过 GitHub 获取正式版本更新", "Stable release updates from GitHub")
	}
}

func (a *App) launcherUpdateButton(c *ui.Context) {
	s := uiScale(c)
	switch a.launcherUpdate.phase {
	case "disabled":
		if fixedButton(c, "launcher-download", a.L("下载正式版", "Get release"), "download", 0, btnSm*s, false, false) {
			c.OpenURL(buildinfo.ReleasesURL + "/latest")
		}
	case "available", "installing":
		if fixedButton(c, "launcher-install", a.L("查看更新", "View update"), "download", 0, btnSm*s, true, false) {
			a.openDialog("launcherUpdate")
		}
	case "ready":
		if fixedButton(c, "launcher-restart", a.L("重启启动器", "Restart launcher"), "refresh", 0, btnSm*s, true, a.launcherRestartBlocked()) {
			mygo.App.Relaunch()
		}
	default:
		if fixedButton(c, "launcher-check", a.L("检查更新", "Check for updates"), "refresh", 0, btnSm*s, false, a.launcherUpdate.phase == "checking") {
			a.checkLauncherUpdates()
		}
	}
}
