package nativeui

import (
	"encoding/json"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// heroPackStatus is the inline update state in the hero: a 更新到 X
// button when newer, 已是最新 otherwise.
func (a *App) heroPackStatus(c *ui.Context, r launcher.Instance) {
	s := uiScale(c)
	u := r.PackUpdate
	switch {
	case packHasUpdate(r, ""):
		text := a.L("overview_pack.update_to") + u.Latest.Number
		b := tooltip(c, ui.ButtonBase(c.Key("hero-pack-update")).Label(text), text+"\n"+a.L("overview_pack.review_and_update")).Height(24*s).MinWidth(0).PaddingX(9*s).Gap(5*s).Radius(7*s).Background(warning.Alpha(.14)).Border(1, warning.Alpha(.30)).AlignItems(ui.Center).Shrink(1)
		if b.Children(func() {
			ui.Icon(c, icons["download"]).Size(12*s, 12*s).TextColor(ui.Hex("#b26a00")).Shrink(0)
			ui.Text(c, text).FontSize(typeFootnote * s).FontWeight(650).TextColor(ui.Hex("#b26a00")).SingleLine().MinWidth(0).Shrink(1)
		}).Clicked() && !a.instancePending("planPackUpdate") && !r.Running {
			a.planPackUpdate(r.ID, "", u.Latest.ID)
		}
	case u != nil && u.Latest != nil:
		heroChip(c, "check", a.L("overview_pack.up_to_date"), false)
	}
}

// playSummary is the one-line recent-run summary.
func (a *App) playSummary(r launcher.Instance, st launcher.InstanceStatus) string {
	p := st.Play
	switch {
	case verRunning(&r):
		return a.L("overview_pack.running")
	case p.Launches > 0:
		out := a.L("overview.played") + agoText(a, p.LastStart) + a.L("overview_pack.for") + durText(a, p.LastSec)
		if p.Launches > 1 {
			out += a.L("overview_pack.total") + durText(a, p.TotalSec)
		}
		if p.LastExit == "crashed" {
			out += a.L("overview_pack.crashed")
		}
		return out
	case !st.LastLog.IsZero():
		return a.L("overview.played") + agoText(a, st.LastLog)
	}
	return a.L("overview_pack.not_launched_yet")
}

// ovPackActions keeps 检查更新 / 从文件更新 / 回滚 in one small row.
func (a *App) ovPackActions(c *ui.Context, r launcher.Instance) {
	s := uiScale(c)
	t := a.tr()
	p := r.Pack
	ui.Box(c).FillWidth().Height(1).Shrink(0).Background(line).Margin(4*s, 0, 6*s, 0)
	ui.Row(c).FillWidth().Height(28 * s).Shrink(0).Gap(12 * s).AlignItems(ui.Center).Children(func() {
		label(c, a.L("overview_pack.modpack")).FontSize(typeCallout * s).NoWrap().Width(kvLabelW * s).Shrink(0)
		txt := p.Version
		if len(r.PackBackups) > 0 {
			txt += a.L("overview_pack.rollback_to") + r.PackBackups[0].FromVersion
		}
		ui.Text(c, txt).FontSize(typeCallout * s).FontWeight(600).TextColor(ink).SingleLine().Grow(1).Basis(0).MinWidth(0)
	})
	ui.Row(c).FillWidth().Shrink(0).Gap(8*s).Padding(4*s, 0, 0, 0).AlignItems(ui.Center).Children(func() {
		blocked := a.instancePending("checkPackUpdate", "planPackUpdate", "rollbackPack") || r.Running
		online := p.ProjectID != "" && (p.Source == "modrinth" || p.Source == "curseforge")
		if online && fixedButton(c, "pack-check", t.PackCheckBtn, "refresh", 0, btnSm*s, false, blocked) && a.overviewActionReady("ov-run") {
			a.execute("checkPackUpdate", map[string]string{"rootId": r.ID}, func(b json.RawMessage) {
				info := decode[launcher.PackUpdateInfo](b)
				if info.HasUpdate && info.Latest != nil {
					a.notice(t.PackFoundNotice + info.Latest.Number)
				} else {
					a.notice(t.PackUpToDateNotice)
				}
			})
		}
		if (p.Source == "modrinth" || p.Source == "curseforge") && fixedButton(c, "pack-from-file", t.PackFromFileBtn, "folder", 0, btnSm*s, false, blocked) && a.overviewActionReady("ov-run") {
			id := r.ID
			a.chooseFiles("pack", func(paths []string) { a.planPackUpdate(id, paths[0], "") })
		}
		if len(r.PackBackups) > 0 {
			b := r.PackBackups[0]
			if fixedButton(c, "pack-rollback", a.L("overview_pack.roll_back"), "refresh", 0, btnSm*s, false, blocked) && a.overviewActionReady("ov-run") {
				a.execute("rollbackPack", map[string]string{"rootId": r.ID, "id": b.ID}, a.taskAdded)
			}
		}
	})
}

// autoCheckPack checks a pack for updates once per launcher run when the
// cached result is missing or older than 6 hours.
func (a *App) autoCheckPack(r launcher.Instance) {
	p := r.Pack
	if p == nil || p.ProjectID == "" || (p.Source != "modrinth" && p.Source != "curseforge") || r.Running {
		return
	}
	if a.packChecked == nil {
		a.packChecked = map[string]bool{}
	}
	if a.packChecked[r.ID] {
		return
	}
	a.packChecked[r.ID] = true
	if u := r.PackUpdate; u != nil {
		if t, e := time.Parse(time.RFC3339, u.CheckedAt); e == nil && time.Since(t) < 6*time.Hour {
			return
		}
	}
	a.execute("checkPackUpdate", map[string]string{"rootId": r.ID}, nil)
}
