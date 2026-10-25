package nativeui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type resolveInstallFunc func(context.Context, string, string, string, string, func([]launcher.PlanItem)) ([]launcher.PlanItem, error)

func (a *App) currentPlanKey() string {
	if a.project == nil || a.root() == nil || a.form.VersionID == "" {
		return ""
	}
	r, p := a.root(), a.project
	return r.ID + "|" + r.SelectedVersion + "|" + p.Source + "|" + p.ID + "|" + a.form.VersionID
}

func (a *App) clearPlan() {
	a.planRevision++
	if a.planCancel != nil {
		a.planCancel()
		a.planCancel = nil
	}
	a.plan, a.planKey, a.planErr, a.planLoading, a.planOff = nil, "", "", false, nil
	a.planErrUI = uiError{}
}

func (a *App) ensurePlan() {
	p := a.project
	r := a.root()
	if !a.dialogOpen {
		return
	}
	if p == nil || p.Type != "mod" || r == nil || a.form.VersionID == "" || a.projLoading {
		if a.planKey != "" {
			a.clearPlan()
		}
		return
	}
	key := a.currentPlanKey()
	if a.planKey == key {
		return
	}
	a.clearPlan()
	resolve := a.resolvePlan
	if resolve == nil && a.service != nil {
		resolve = a.service.ResolveInstall
	}
	if resolve == nil {
		return
	}
	a.planKey, a.planErr, a.planLoading = key, "", true
	a.plan = []launcher.PlanItem{{Key: p.Source + ":" + p.ID, Source: p.Source, ProjectID: p.ID, Title: p.Title, Icon: p.Icon, Role: "target", Status: "resolving"}}
	a.planOff = map[string]bool{}
	ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
	a.planCancel = cancel
	revision := a.planRevision
	root, src, id, ver := r.ID, p.Source, p.ID, a.form.VersionID
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		defer cancel()
		emit := func(items []launcher.PlanItem) {
			a.post(func() {
				if a.planRevision == revision && a.currentPlanKey() == key && a.planKey == key {
					a.plan = items
				}
			})
		}
		items, e := resolve(ctx, root, src, id, ver, emit)
		a.post(func() {
			if a.planRevision != revision || a.currentPlanKey() != key || a.planKey != key {
				return
			}
			a.planLoading, a.planCancel = false, nil
			if e != nil {
				a.planErrUI = a.uiErr(e)
				a.planErr = a.planErrUI.Text()
			}
			if items != nil {
				a.plan = items
			}
		})
	}()
}

// planChecked: missing required rows default on, optional off.
func (a *App) planChecked(it launcher.PlanItem) bool {
	if v, ok := a.planOff[it.Key]; ok {
		return !v
	}
	switch it.Status {
	case "install":
		return it.Role == "target" || it.Role == "required"
	case "disabled":
		return it.Role == "required"
	}
	return false
}

func (a *App) planJobState() *launcher.Job {
	if a.planJob == "" {
		return nil
	}
	for i := range a.snapshot.Jobs {
		if a.snapshot.Jobs[i].ID == a.planJob {
			return &a.snapshot.Jobs[i]
		}
	}
	return nil
}

// planSelection is what one confirm installs.
func (a *App) planSelection() (items []launcher.PlanInstall, enable, skip []string) {
	for _, it := range a.plan {
		on := a.planChecked(it)
		switch {
		case it.Role == "target" && (it.Status == "install"):
			replace := a.planReplace
			if replace == "" {
				replace = it.File
			}
			items = append(items, launcher.PlanInstall{Key: it.Key, Source: it.Source, ProjectID: it.ProjectID, VersionID: it.VersionID, Title: a.planTitle(it), Replace: replace, Depth: 0})
		case it.Status == "install" && on:
			items = append(items, launcher.PlanInstall{Key: it.Key, Source: it.Source, ProjectID: it.ProjectID, VersionID: it.VersionID, Title: a.planTitle(it), Depth: it.Depth})
		case it.Status == "disabled" && on:
			enable = append(enable, it.File)
			skip = append(skip, it.Key)
		default:
			if it.Role != "target" {
				skip = append(skip, it.Key) // present, unchecked or unavailable: never fetched implicitly
			}
		}
	}
	return
}

func (a *App) planTitle(it launcher.PlanItem) string {
	zh := ""
	if it.Source == "curseforge" {
		zh = a.zhNames.Mod(it.Slug, "", "")
	} else {
		zh = a.zhNames.Mod("", it.Slug, "")
	}
	t, _ := a.zhTitle(zh, it.Title)
	return t
}

func (a *App) installDeps(c *ui.Context) {
	s := uiScale(c)
	var deps []launcher.PlanItem
	var target *launcher.PlanItem
	for i := range a.plan {
		if a.plan[i].Role == "target" {
			target = &a.plan[i]
		} else {
			deps = append(deps, a.plan[i])
		}
	}
	ui.Column(c.Key("install-dependencies")).Label("前置区域").FillWidth().Gap(12 * s).Children(func() {
		note := ui.Box(c.Key("installed-mod-note-slot")).FillWidth().Clip()
		noteH := float32(0)
		if target != nil && target.File != "" {
			noteH = 38 * s
		}
		note.Height(note.AnimateTo("height", noteH, motionMove)).Children(func() {
			if noteH > 0 {
				ui.Row(c.Key("installed-mod-note")).FillWidth().Height(38 * s).PaddingX(14 * s).Radius(10 * s).Background(accentSoft).Gap(10 * s).AlignItems(ui.Center).Transition(heroSwap(c)).Children(func() {
					ui.Icon(c, icons["info"]).Size(14*s, 14*s).TextColor(accent).Shrink(0)
					msg := a.L("当前实例已安装此模组", "This mod is already installed")
					if target.Note != "" {
						msg += a.L("，版本 ", ", version ") + target.Note
					}
					tooltip(c, ui.Text(c, msg).FontSize(typeFootnote*s).TextColor(inkSecondary).Grow(1).Basis(0).MinWidth(0).SingleLine(), msg)
				})
			}
		})
		ui.Column(c.Key("dependency-content")).FillWidth().Gap(10 * s).Children(func() {
			ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.L("前置", "Dependencies")).FontSize(typeCallout * s).FontWeight(650).TextColor(ink).Shrink(0)
				if len(deps) > 0 {
					numText(c, fmt.Sprint(len(deps)), typeFootnote*s, muted)
				}
				if a.planLoading {
					statusSpinner(c, 12*s)
				}
				ui.Spacer(c).Grow(1)
				for _, v := range a.projectVersions {
					if v.ID == a.form.VersionID {
						tooltip(c, ui.Text(c, a.L("所选版本 ", "Selected version ")+v.Number).Label("前置所选版本 "+v.Number).FontSize(typeCaption*s).TextColor(muted).SingleLine().Shrink(1).MinWidth(0), v.Number)
						break
					}
				}
			})
			if !a.selectedProjectVersionFits() {
				ui.Row(c.Key("dependency-incompatible")).FillWidth().MinHeight(54 * s).Padding(12 * s).Radius(10 * s).Background(ui.RGBA(255, 149, 0, .08)).Gap(8 * s).AlignItems(ui.Center).Transition(heroSwap(c)).Children(func() {
					ui.Icon(c, icons["info"]).Size(14*s, 14*s).TextColor(warningInk).Shrink(0)
					ui.Text(c, a.L("此版本不兼容当前实例，请选择匹配的游戏版本和加载器。", "This version does not match the instance. Choose a compatible game version and loader.")).FontSize(typeFootnote * s).TextColor(inkSecondary).Grow(1).Basis(0).MinWidth(0)
				})
				return
			}
			if a.planErr != "" && a.planErrUI.Kind == "network" {
				if a.loadFailed(c, "plan-failed", failCard, a.planLoading) {
					a.planKey = ""
				}
				return
			}
			if a.planErr != "" {
				u := a.planErrUI
				ui.Column(c).FillWidth().Padding(12*s, 14*s).Radius(10 * s).Background(ui.RGBA(255, 149, 0, .08)).Gap(8 * s).Children(func() {
					ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons["info"]).Size(13*s, 13*s).TextColor(warningInk).Shrink(0)
						ui.Text(c, u.Title).FontSize(typeFootnote * s).FontWeight(600).TextColor(ink).Grow(1).MinWidth(0)
					})
					if u.Hint != "" {
						ui.Text(c, u.Hint).FontSize(typeCaption * s).TextColor(inkSecondary).FillWidth()
					}
					ui.Row(c).Gap(8 * s).Children(func() {
						if fixedButton(c, "plan-retry", a.L("重试", "Retry"), "refresh", 0, btnSm*s, false, false) {
							a.planKey = ""
						}
						if u.Path != "" && fixedButton(c, "plan-open", a.L("打开文件夹", "Open folder"), "folder", 0, btnSm*s, false, false) {
							c.OpenURL("file:///" + filepath.ToSlash(u.Path))
						}
					})
				})
				return
			}
			// Reserve one row while checking; grow the viewport as rows arrive.
			// Text and icons stay at native size, with only opacity/position animated.
			box := ui.Box(c.Key("dependency-list")).Label("前置列表").FillWidth().Radius(12*s).Border(1, line).Clip()
			h := float32(max(1, len(deps))*55+2) * s
			box.Height(box.AnimateTo("height", h, motionMove)).Children(func() {
				ui.Column(c.Key("dependency-rows-" + a.planKey)).FillWidth().Shrink(0).Transition(heroSwap(c)).Children(func() {
					if len(deps) == 0 {
						key, txt, icon := "empty", a.L("无需前置", "No dependencies"), "check"
						if a.planLoading {
							key, txt = "loading", a.L("正在检查前置", "Checking dependencies")
						}
						ui.Row(c.Key("dependency-" + key)).Label("前置状态").FillWidth().Height(54 * s).PaddingX(14 * s).Gap(10 * s).AlignItems(ui.Center).Transition(heroSwap(c)).Children(func() {
							if a.planLoading {
								statusSpinner(c, 14*s)
							} else {
								ui.Icon(c, icons[icon]).Size(14*s, 14*s).TextColor(successInk)
							}
							ui.Text(c, txt).FontSize(typeFootnote * s).TextColor(muted)
						})
						return
					}
					for i, it := range deps {
						ui.Column(c.Key("dep-slot-" + it.Key)).FillWidth().Shrink(0).Transition(heroSwap(c)).Children(func() {
							if i > 0 {
								ui.Box(c).FillWidth().Height(s).Background(line).Margin(0, 0, 0, 56*s)
							}
							a.depRow(c, it)
						})
					}
				})
			})
		})
	})
}

func (a *App) depRow(c *ui.Context, it launcher.PlanItem) {
	s := uiScale(c)
	job := a.planJobState()
	checkable := it.Status == "install" || it.Status == "disabled"
	on := a.planChecked(it)
	b := ui.ButtonBase(c.Key("dep-" + it.Key)).Label(it.Title).FillWidth().Height(54 * s).PaddingX(14 * s).Gap(12 * s).AlignItems(ui.Center).Disabled(!checkable || job != nil)
	if b.Hovered() && checkable {
		b.Background(ui.RGBA(120, 130, 160, .06))
	}
	if b.Children(func() {
		// checkbox
		ui.Box(c).Size(18*s, 18*s).Radius(5*s).Shrink(0).Border(1.5, map[bool]ui.Color{true: accent, false: ui.RGBA(60, 70, 110, .28)}[on && checkable]).Background(map[bool]ui.Color{true: accent, false: ui.Transparent}[on && checkable]).Opacity(map[bool]float32{true: 1, false: .35}[checkable]).Center().Children(func() {
			if on && checkable {
				ui.Icon(c, icons["check"]).Size(11*s, 11*s).TextColor(panel)
			}
		})
		size := 30 * s
		if img := a.remoteImage(it.Icon); img != nil {
			ui.Box(c).Size(size, size).Radius(size * .22).Clip().Shrink(0).Children(func() { ui.Image(c, img).Size(size, size).Fit(ui.Contain) })
		} else {
			letterTile(c, it.Title, size, false)
		}
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(2 * s).Children(func() {
			ui.Text(c, a.planTitle(it)).FontSize(typeCallout * s).FontWeight(600).TextColor(ink).SingleLine().FillWidth().MinWidth(0)
			sub := it.Version
			if it.Title != a.planTitle(it) {
				sub = strings.TrimSpace(it.Title + "   " + it.Version)
			}
			if sub != "" {
				ui.Text(c, sub).FontSize(typeCaption * s).TextColor(muted).SingleLine().FillWidth().MinWidth(0)
			}
		})
		role := map[string]string{"required": a.L("必需", "Required"), "optional": a.L("可选", "Optional"), "incompatible": a.L("不兼容", "Incompatible")}[it.Role]
		if role != "" {
			quietTag(c, role, inkSecondary)
		}
		a.depStatus(c, it, job)
	}).Clicked() && checkable {
		if a.planOff == nil {
			a.planOff = map[string]bool{}
		}
		a.planOff[it.Key] = on
	}
}

func (a *App) depStatus(c *ui.Context, it launcher.PlanItem, job *launcher.Job) {
	s := uiScale(c)
	if job != nil {
		key := it.Key
		if it.Status == "disabled" {
			key = "enable:" + it.File
		}
		if st, ok := job.Steps[key]; ok {
			switch {
			case st == "running":
				statusSpinner(c, 14*s)
			case st == "done":
				quietTag(c, a.L("完成", "Done"), successInk)
			default:
				quietTag(c, a.L("失败", "Failed"), danger)
				tooltip(c, ui.Box(c).Size(0, 0), classifyError(a.isEN(), strings.TrimPrefix(st, "failed: "), nil).Text())
			}
			return
		}
	}
	switch it.Status {
	case "resolving":
		statusSpinner(c, 14*s)
	case "install":
		if it.Unverified {
			tooltip(c, ui.Box(c).Shrink(0), a.L("模组文件夹暂时无法读取，未能确认是否已安装", "The mods folder could not be read, so presence is unconfirmed")).Children(func() { quietTag(c, a.L("待确认", "Unconfirmed"), warningInk) })
			return
		}
		quietTag(c, a.L("将安装", "Will install"), accent)
	case "installed":
		quietTag(c, a.L("已安装", "Installed"), successInk)
	case "disabled":
		quietTag(c, a.L("已禁用", "Disabled"), warningInk)
	case "none":
		quietTag(c, a.L("无兼容版本", "No compatible file"), danger)
	case "conflict":
		quietTag(c, a.L("冲突", "Conflict"), danger)
	}
}
