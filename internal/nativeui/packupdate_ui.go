package nativeui

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// packHasUpdate reports whether the cached online check found a newer
// release for the version the pack installed.
func packHasUpdate(r launcher.Instance, versionID string) bool {
	if r.Pack == nil || r.PackUpdate == nil || !r.PackUpdate.HasUpdate || r.PackUpdate.Latest == nil {
		return false
	}
	return r.Pack.GameVersion == "" || versionID == "" || r.Pack.GameVersion == versionID
}

// packSection is the 整合包 block at the bottom of the version 设置 dialog.
func (a *App) packSection(c *ui.Context, r launcher.Instance) {
	a.packExportSection(c, r)
	if r.Pack == nil {
		return
	}
	s := uiScale(c)
	t := a.tr()
	p := r.Pack
	ui.Box(c).FillWidth().Height(1).Background(line)
	ui.Column(c.Key("pack-section")).Label("整合包设置").FillWidth().Gap(10*s).Padding(16*s).Radius(12*s).Background(raised).Border(1, line).Children(func() {
		ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(36*s, 36*s).Radius(10*s).Background(panel).Border(1, line).Center().Shrink(0).Children(func() {
				if p.Source == "modrinth" {
					ui.Image(c, modrinthLogo).Size(22*s, 22*s).Fit(ui.Contain)
				} else if p.Source == "curseforge" {
					ui.Image(c, curseforgeLogo).Size(22*s, 22*s).Fit(ui.Contain)
				} else {
					ui.Icon(c, icons["layers"]).Size(20*s, 20*s).TextColor(accent)
				}
			})
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(2 * s).Children(func() {
				ui.Text(c, t.PackSecTitle+"   "+p.Name).FontSize(typeSubhead * s).Bold().TextColor(ink).SingleLine().FillWidth().MinWidth(0)
				label(c, t.PackSecCurrent+" "+p.Version+"   "+p.Minecraft).FontSize(typeFootnote * s).SingleLine()
			})
			switch {
			case packHasUpdate(r, ""):
				badge(c, t.PackUpdateBadge+"   "+r.PackUpdate.Latest.Number, warning)
			case r.PackUpdate != nil && r.PackUpdate.Latest != nil:
				badge(c, t.PackSecUpToDate, success)
			case p.ProjectID == "":
				badge(c, t.PackSecNoOrigin, muted)
			default:
				badge(c, t.PackSecNotChecked, muted)
			}
		})
		if r.Running {
			label(c, t.PackSecRunning).TextColor(danger)
		}
		ui.Row(c).FillWidth().Wrap().Gap(8 * s).AlignItems(ui.Center).Children(func() {
			blocked := a.instancePending("checkPackUpdate", "planPackUpdate", "rollbackPack") || r.Running
			if p.ProjectID != "" && (p.Source == "modrinth" || p.Source == "curseforge") && button(c, "pack-check", t.PackCheckBtn, "refresh", false, blocked) {
				a.execute("checkPackUpdate", map[string]string{"rootId": r.ID}, func(b json.RawMessage) {
					info := decode[launcher.PackUpdateInfo](b)
					if info.HasUpdate && info.Latest != nil {
						a.notice(t.PackFoundNotice + info.Latest.Number)
					} else {
						a.notice(t.PackUpToDateNotice)
					}
				})
			}
			if packHasUpdate(r, "") && button(c, "pack-update", t.PackUpdateToPrefix+r.PackUpdate.Latest.Number, "download", true, blocked) {
				a.planPackUpdate(r.ID, "", r.PackUpdate.Latest.ID)
			}
			if (p.Source == "modrinth" || p.Source == "curseforge") && button(c, "pack-from-file", t.PackFromFileBtn, "folder", false, blocked) {
				id := r.ID
				a.chooseFiles("pack", func(paths []string) { a.planPackUpdate(id, paths[0], "") })
			}
			if len(r.PackBackups) > 0 {
				b := r.PackBackups[0]
				if button(c, "pack-rollback", t.PackRollbackPrefix+b.FromVersion, "refresh", false, blocked) {
					a.execute("rollbackPack", map[string]string{"rootId": r.ID, "id": b.ID}, a.taskAdded)
				}
			}
		})
	})
}

func (a *App) planPackUpdate(rootID, path, versionID string) {
	a.notice(a.tr().PackCheckingNotice)
	a.execute("planPackUpdate", map[string]string{"rootId": rootID, "path": path, "versionId": versionID}, func(b json.RawMessage) {
		plan := decode[launcher.PackUpdatePlan](b)
		a.message = ""
		a.openPackPlan(&plan, false)
	})
}

func (a *App) openPackPlan(plan *launcher.PackUpdatePlan, report bool) {
	a.packPlan = plan
	a.packPlanReport = report
	a.packExpand = map[string]bool{"overwritten": true} // only the warning group starts open
	a.dialog = "packUpdate"
	a.dialogShown = "packUpdate"
	a.dialogOpen = true
}

// offerPackUpdate asks 更新 vs 导入为新目录 when a dropped pack matches the
// pack installed in the current directory.
func (a *App) offerPackUpdate(path string) bool {
	r := a.root()
	if r == nil || r.Pack == nil || (r.Pack.Source != "modrinth" && r.Pack.Source != "curseforge") {
		return false
	}
	preview, err := a.service.InspectPack(path)
	if err != nil || preview.Source != r.Pack.Source || !strings.EqualFold(strings.TrimSpace(preview.Name), strings.TrimSpace(r.Pack.Name)) {
		return false
	}
	a.openDialog("packDrop")
	a.packDropPath = path
	a.packPreview = &preview
	return true
}

func (a *App) packDropDialog(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	r := a.root()
	if r == nil || r.Pack == nil {
		return
	}
	label(c, fmt.Sprintf(t.PackDropDesc, r.Name, r.Pack.Version))
	if p := a.packPreview; p != nil {
		ui.Row(c).FillWidth().Padding(12*s, 14*s).Radius(12*s).Background(accentSoft).Border(1, blue).Gap(10 * s).AlignItems(ui.Center).Children(func() {
			ui.Image(c, packLogo(p.Source)).Size(22*s, 22*s).Fit(ui.Contain)
			ui.Text(c, p.Name).FontSize(typeBody * s).Bold().TextColor(ink).SingleLine().Shrink(1).MinWidth(0)
			badge(c, r.Pack.Version, muted)
			ui.Icon(c, icons["arrow"]).Size(14*s, 14*s).TextColor(muted)
			badge(c, p.Version, accent)
		})
	}
	if r.Running {
		label(c, t.PackSecRunning).TextColor(danger)
	}
	ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
		if buttonWhy(c, "pack-drop-update", t.PackDropUpdateBtn, "download", true, whyOf(a.instancePending("planPackUpdate"), t.WhyBusy, r.Running, t.WhyRunning), true) {
			a.planPackUpdate(r.ID, a.packDropPath, "")
		}
		if button(c, "pack-drop-import", t.PackDropImportBtn, "plus", false, false) {
			path := a.packDropPath
			a.openDialog("import")
			a.inspectPackFile(path)
		}
	})
}

var htmlTag = regexp.MustCompile(`(?s)<[^>]*>`)

func plainChangelog(s string) string {
	if strings.Contains(s, "</") || strings.Contains(s, "<br") {
		s = regexp.MustCompile(`(?i)<br\s*/?>|</p>|</li>|</h[1-6]>`).ReplaceAllString(s, "\n")
		s = regexp.MustCompile(`(?i)<li[^>]*>`).ReplaceAllString(s, "- ")
		s = htmlTag.ReplaceAllString(s, "")
		s = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", "\"", "&#39;", "'").Replace(s)
	}
	lines := []string{}
	for _, l := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		l = strings.TrimSpace(strings.TrimLeft(l, "#"))
		l = strings.ReplaceAll(strings.ReplaceAll(l, "**", ""), "__", "")
		if strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "* ") {
			l = "- " + l[2:]
		}
		lines = append(lines, l)
	}
	out := strings.TrimSpace(regexp.MustCompile(`\n{3,}`).ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
	if r := []rune(out); len(r) > 3000 {
		out = string(r[:3000]) + "…"
	}
	return out
}

func (a *App) packUpdateDialog(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	p := a.packPlan
	if p == nil {
		return
	}
	ui.Row(c).FillWidth().Padding(14*s, 16*s).Radius(12*s).Background(accentSoft).Border(1, blue).Gap(12 * s).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(40*s, 40*s).Radius(10*s).Background(panel).Border(1, line).Center().Shrink(0).Children(func() {
			ui.Image(c, packLogo(a.packPlanSource())).Size(24*s, 24*s).Fit(ui.Contain)
		})
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4 * s).Children(func() {
			ui.Text(c, p.Name).FontSize(typeSubhead * s).Bold().TextColor(ink).SingleLine()
			if p.GameChanged {
				label(c, a.gameChangeText(p.OldGame, p.NewGame)).FontSize(typeFootnote * s).TextColor(warningInk).SingleLine()
			} else {
				label(c, prettyGame(p.NewGame)).FontSize(typeFootnote * s).SingleLine()
			}
		})
		badge(c, p.OldVersion, muted)
		ui.Icon(c, icons["arrow"]).Size(14*s, 14*s).TextColor(muted)
		badge(c, p.NewVersion, accent)
	})
	if p.GameChanged && !a.packPlanReport {
		ui.Row(c.Key("pack-game-warn")).Label("游戏版本变更提示").FillWidth().Padding(12*s, 14*s).Gap(12*s).Radius(12*s).Background(amberSoft).Border(1, amberLine).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons["info"]).Size(18*s, 18*s).TextColor(warningInk).Shrink(0)
			ui.Text(c, fmt.Sprintf(t.PackGameWarn, prettyGame(p.NewGame), prettyGame(p.OldGame))).Grow(1).Basis(0).MinWidth(0).FontSize(typeFootnote * s).TextColor(warningInk)
			if button(c, "pack-open-dir", t.PackOpenDirBtn, "folder", false, false) {
				a.openFolder(p.RootID)
			}
		})
	}
	// counts
	ui.Row(c).FillWidth().Wrap().Gap(8 * s).AlignItems(ui.Center).Children(func() {
		badge(c, fmt.Sprintf("%s %d", t.PackChAdded, len(p.Added)), success)
		badge(c, fmt.Sprintf("%s %d", t.PackChUpdated, len(p.Updated)), accent)
		badge(c, fmt.Sprintf("%s %d", t.PackChRemoved, len(p.Removed)), danger)
		badge(c, fmt.Sprintf("%s %d", t.PackChOverwritten, len(p.Overwritten)), warning)
		tagChip(c, fmt.Sprintf("%s%d", t.PackUnchangedPrefix, p.Unchanged))
		if p.DownloadBytes > 0 {
			tagChip(c, t.PackDownloadPrefix+sizeText(p.DownloadBytes))
		}
	})
	if !a.packPlanReport {
		ui.Column(c).FillWidth().Gap(6*s).Padding(12*s, 14*s).Radius(12*s).Background(raised).Border(1, line).Children(func() {
			label(c, t.PackChangelog).FontWeight(600)
			text := plainChangelog(p.Changelog)
			if text == "" {
				text = t.PackNoChangelog
			}
			ui.Text(c, text).FillWidth().FontSize(typeCallout * s).TextColor(inkSecondary)
		})
	}
	if len(p.Added)+len(p.Updated)+len(p.Removed)+len(p.Overwritten) == 0 {
		label(c, t.PackNothing)
	}
	a.packChangeGroups(c, p)
	if a.packPlanReport {
		return
	}
	ui.Row(c).FillWidth().Padding(10*s, 14*s).Radius(12 * s).Background(fieldColor).Children(func() {
		ui.Text(c, t.PackBackupNote).FillWidth().FontSize(typeFootnote * s).TextColor(inkSecondary)
	})
}

// packJobButtons: 查看变更 / 回滚 / 启动 on a finished 更新整合包 task.
func (a *App) packJobButtons(c *ui.Context, j launcher.Job) {
	t := a.tr()
	r := a.rootByID(j.ResultRoot)
	if button(c, "job-changes-"+j.ID, t.TaskChangesBtn, "layers", false, a.pendingAt("packUpdateReport", "id", j.ResultRef)) {
		a.execute("packUpdateReport", map[string]string{"rootId": j.ResultRoot, "id": j.ResultRef}, func(b json.RawMessage) {
			plan := decode[launcher.PackUpdatePlan](b)
			a.openPackPlan(&plan, true)
		})
	}
	for _, b := range r.PackBackups {
		if b.ID == j.ResultRef {
			if button(c, "job-rollback-"+j.ID, t.TaskRollbackBtn, "refresh", false, a.pendingAt("rollbackPack", "rootId", r.ID) || r.Running) {
				a.execute("rollbackPack", map[string]string{"rootId": j.ResultRoot, "id": j.ResultRef}, a.taskAdded)
			}
		}
	}
	if button(c, "job-launch-"+j.ID, t.TaskLaunchBtn, "play", true, a.pendingAt("launch", "rootId", j.ResultRoot)) {
		a.launchRoot(j.ResultRoot)
	}
}

// packUpdateFooter sits under the dialog's scroll area so 更新整合包 is
// always visible.
func (a *App) packUpdateFooter(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	p := a.packPlan
	if p == nil || a.packPlanReport {
		return
	}
	ui.Box(c).FillWidth().Height(1).Background(line)
	r := a.rootByID(p.RootID)
	running := r != nil && r.Running
	if running {
		label(c, t.PackSecRunning).TextColor(danger)
	}
	ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icons["refresh"]).Size(12*s, 12*s).TextColor(muted).Shrink(0)
		ui.Text(c, a.L("packupdate_ui.backed_up_first_you_can_roll_back")).FontSize(typeFootnote * s).TextColor(muted).SingleLine().Grow(1).Shrink(1).MinWidth(0)
		if button(c, "pack-cancel", t.PackCancelBtn, "", false, a.pendingOp("applyPackUpdate")) {
			go a.service.DiscardPackUpdate(p.ID)
			a.closeDialog()
		}
		if buttonWhy(c, "pack-apply", t.PackApplyBtn, "download", true, whyOf(a.pendingOp("applyPackUpdate"), t.WhyBusy, running, t.WhyRunning), true) {
			a.execute("applyPackUpdate", map[string]string{"id": p.ID}, a.taskAdded)
		}
	})
}

func packLogo(source string) *ui.SVG {
	if source == "curseforge" {
		return curseforgeLogo
	}
	return modrinthLogo
}

func (a *App) packPlanSource() string {
	if a.packPlan != nil {
		if r := a.rootByID(a.packPlan.RootID); r != nil && r.Pack != nil {
			return r.Pack.Source
		}
	}
	return "modrinth"
}

// prettyGame turns "1.21.1   fabric 0.19.3" into "1.21.1  Fabric 0.19.3".
func prettyGame(g string) string {
	f := strings.Fields(g)
	for i, w := range f {
		if l := loaderLabel(w); l != "" {
			f[i] = l
		}
	}
	return strings.Join(f, "  ")
}

// gameChangeText names only what changes: the loader when Minecraft stays.
func (a *App) gameChangeText(old, new string) string {
	of, nf := strings.Fields(old), strings.Fields(new)
	if len(of) == 3 && len(nf) == 3 && of[0] == nf[0] && of[1] == nf[1] {
		return fmt.Sprintf(a.L("packupdate_ui.loader_changes"), loaderLabel(nf[1]), of[2], nf[2])
	}
	return a.L("packupdate_ui.game_changes") + prettyGame(old) + " → " + prettyGame(new)
}
