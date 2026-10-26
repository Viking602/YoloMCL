package nativeui

import (
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

var _ = runtime.GOOS

// recommendedVersion is the newest release compatible with the instance,
// else the newest compatible one.
func (a *App) recommendedVersion() string {
	mc, loader := "", ""
	if v := a.gameVersion(); v != nil {
		mc, loader = v.Minecraft, strings.ToLower(v.Loader)
	}
	p := a.project
	if p != nil && p.Type == "modpack" {
		mc, loader = "", "" // a modpack makes its own instance
	}
	first := ""
	for _, v := range a.projectVersions {
		ok := mc == "" || slices.Contains(v.GameVersions, mc)
		if p != nil && p.Type == "mod" && loader != "" && loader != "vanilla" && len(v.Loaders) > 0 {
			ok = ok && slices.Contains(v.Loaders, loader)
		}
		if !ok {
			continue
		}
		if first == "" {
			first = v.ID
		}
		if v.Type == "" || v.Type == "release" {
			return v.ID
		}
	}
	return first
}

// installDialog is the project / install dialog: header, version list,
// dependencies, and a footer with the target and the actions.
func (a *App) projectInstallDialog(c *ui.Context, maxH float32) {
	s := uiScale(c)
	p := a.project
	if p == nil {
		return
	}
	if j := a.planJobState(); j != nil && j.State != "running" && !a.planFinished {
		a.planFinished = true
		a.loadMods()
	}
	ui.Column(c.Key("install-dialog")).FillWidth().Font(uiSansFamily()).Gap(0).Children(func() {
		a.installHeader(c)
		ui.Box(c).FillWidth().Height(1).Background(line)
		scrollArea(c, "install-body").FillWidth().MinHeight(0).MaxHeight(max(200, maxH-230*s)).Padding(16*s, 24*s, 16*s).Children(func() {
			ui.Column(c).FillWidth().Gap(18 * s).Children(func() {
				if a.message != "" && a.errorMessage {
					a.inlineNotice(c, "error", a.message, "")
				}
				a.installVersions(c)
				a.ensurePlan()
				if p.Type == "mod" && a.form.VersionID != "" {
					a.installDeps(c)
				}
				if p.Type == "modpack" {
					a.packDestFields(c)
				}
			})
		})
		ui.Box(c).FillWidth().Height(1).Background(line)
		a.installFooter(c)
	})
}

func (a *App) installHeader(c *ui.Context) {
	s := uiScale(c)
	p := a.project
	ui.Row(c).FillWidth().Padding(22*s, 20*s, 18*s, 24*s).Gap(16 * s).AlignItems(ui.Start).Children(func() {
		size := 60 * s
		if img := a.remoteImage(p.Icon); img != nil {
			ui.Image(c, img).Size(size, size).Fit(ui.Contain).Radius(size * .22).Shrink(0)
		} else {
			letterTile(c, p.Title, size, false)
		}
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(5 * s).Children(func() {
			title, original := a.zhTitle(a.zhNames.Project(*p), p.Title)
			ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
				ui.Text(c, title).FontSize(typeTitle2 * s).FontWeight(700).TextColor(ink).SingleLine().Shrink(1).MinWidth(0)
				if original != "" {
					ui.Text(c, original).FontSize(typeCallout * s).FontWeight(500).TextColor(muted).SingleLine().Shrink(2).MinWidth(0)
				}
			})
			if p.Description != "" {
				ui.Text(c, strings.Join(strings.Fields(p.Description), " ")).FontSize(typeFootnote * s).TextColor(inkSecondary).FillWidth().MaxLines(2).Ellipsis("…")
			}
			ui.Row(c).FillWidth().Gap(6 * s).AlignItems(ui.Center).Children(func() {
				if logo := sourceLogo(p.Source); logo != nil {
					ui.Row(c).Height(20 * s).PaddingX(7 * s).Gap(5 * s).Radius(6 * s).Background(ui.RGBA(120, 130, 160, .10)).AlignItems(ui.Center).Shrink(0).Children(func() {
						ui.Icon(c, logo).Size(11*s, 11*s)
						ui.Text(c, sourceName(p.Source)).FontSize(typeCaption2 * s).FontWeight(600).TextColor(inkSecondary).NoWrap()
					})
				}
				shown := 0
				for _, cat := range p.Categories {
					if shown >= 3 || isLoaderCategory(cat) {
						continue
					}
					lbl := a.translateCategory(cat)
					if a.isEN() {
						lbl = categoryTitle(cat)
					} else if strings.EqualFold(lbl, cat) {
						continue
					}
					catTag(c, cat, lbl, false)
					shown++
				}
				if p.Downloads > 0 {
					hint := fmt.Sprintf(a.L("install_dialog.total_project_downloads"), exactDownloadCount(p.Downloads))
					tooltip(c, ui.Row(c).Gap(4*s).AlignItems(ui.Center).Shrink(0), hint).Children(func() {
						ui.Icon(c, icons["download"]).Size(10*s, 10*s).TextColor(muted)
						numText(c, downloadCountText(p.Downloads, a.isEN()), typeCaption*s, muted)
					})
				}
			})
		})
		ui.Column(c).Shrink(0).Gap(8 * s).AlignItems(ui.End).Children(func() {
			if iconOnlyButton(c, "close-dialog", "cross", a.tr().DialogCloseBtn, btnSm*s+2*s, false) {
				a.closeDialog()
			}
			if p.URL != "" {
				b := ui.ButtonBase(c.Key("project-site")).Label(a.tr().ProjectWebsiteBtn).Height(24 * s).PaddingX(6 * s).Radius(6 * s).Gap(4 * s).AlignItems(ui.Center)
				fg := muted
				if b.Hovered() {
					fg = accent
				}
				tooltip(c, b, a.L("install_dialog.project_page")).Size(btnSm*s+2*s, btnSm*s+2*s).PaddingX(0).Justify(ui.Center)
				if b.Hovered() {
					b.Background(ui.RGBA(120, 130, 160, .10))
				}
				if b.Children(func() {
					ui.Icon(c, icons["external"]).Size(12*s, 12*s).TextColor(fg)
				}).Clicked() {
					c.OpenURL(p.URL)
				}
			}
		})
	})
}

const verRowH, verRows = 52, 5

func (a *App) installFooter(c *ui.Context) {
	s := uiScale(c)
	p := a.project
	t := a.tr()
	ui.Row(c).FillWidth().Padding(14*s, 24*s).Gap(10 * s).AlignItems(ui.Center).Children(func() {
		r := a.root()
		job := a.planJobState()
		// Reserve the remaining footer width for the target; status messages
		// belong below it so a long download name cannot squeeze it away.
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(6 * s).Children(func() {
			if r != nil && p.Type != "modpack" {
				a.installTarget(c, *r)
			}
			if job != nil && job.State == "running" {
				ui.Row(c).FillWidth().MinWidth(0).Gap(8 * s).AlignItems(ui.Center).Children(func() {
					statusSpinner(c, 14*s)
					tooltip(c, ui.Text(c, job.Message).FontSize(typeFootnote*s).TextColor(inkSecondary).SingleLine().Grow(1).Basis(0).MinWidth(0), job.Message)
					if job.Total > 0 {
						numText(c, fmt.Sprintf("%d/%d", job.Done, job.Total), typeFootnote*s, muted)
					}
				})
			} else if job != nil && job.State == "failed" {
				tooltip(c, ui.Text(c, classifyError(a.isEN(), job.Message, nil).Title).FontSize(typeFootnote*s).TextColor(danger).FillWidth().MinWidth(0).MaxLines(2).Ellipsis("…"), classifyError(a.isEN(), job.Message, nil).Hint)
			} else if job != nil && job.State == "completed" {
				ui.Row(c).FillWidth().Gap(6 * s).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons["check"]).Size(12*s, 12*s).TextColor(successInk)
					ui.Text(c, a.L("install_dialog.installed")).FontSize(typeFootnote * s).TextColor(successInk)
				})
			} else if sum := a.packSummary(); sum != "" {
				ui.Row(c).FillWidth().MinWidth(0).Gap(6 * s).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons["download"]).Size(11*s, 11*s).TextColor(muted)
					numText(c, sum, typeFootnote*s, inkSecondary)
				})
			}
		})
		if job != nil && job.State != "running" {
			if fixedButton(c, "install-done", a.L("install_dialog.done"), "", 0, btnMd*s, true, false) {
				a.closeDialog()
			}
			return
		}
		if fixedButton(c, "install-cancel", a.L("install_dialog.cancel"), "", 0, btnMd*s, false, job != nil) {
			a.closeDialog()
		}
		label, disabled, run := a.installAction()
		if r == nil && p.Type != "modpack" {
			label, disabled = t.ProjectNeedRootHint, true
		}
		if fixedButton(c, "install-confirm", label, "download", 0, btnMd*s, true, disabled || a.pendingOp("installGame", "importPack", "installMod", "installAssetPack", "installPlanned") || job != nil) && run != nil {
			run()
		}
	})
}

func (a *App) installTarget(c *ui.Context, r launcher.Instance) {
	s := uiScale(c)
	v, name := a.gameVersion(), r.Name
	if v != nil {
		name, _ = a.cardText(r, *v)
	}
	w, _ := c.MeasureText(0, ui.Span{Text: name, Font: uiSansFamily(), Size: typeFootnote * s, Weight: 600})
	// A single-line, shrinkable label needs an explicit preferred width;
	// the parent limits it only when the name really exceeds the free space.
	w = float32(math.Ceil(float64(w + (18+7+6*2)*s)))
	ui.Row(c).FillWidth().MinWidth(0).Gap(10 * s).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.L("install_dialog.install_to")).FontSize(typeFootnote * s).TextColor(muted).Shrink(0)
		tooltip(c, ui.Row(c.Key("install-target")).Label("安装目标").Width(w).Height(28*s).PaddingX(6*s).Gap(7*s).Radius(8*s).Background(ui.RGBA(120, 130, 160, .10)).Font(uiSansFamily()).AlignItems(ui.Center).Shrink(1).MinWidth(0), name).Children(func() {
			a.instIcon(c, r, v, 18*s)
			ui.Text(c, name).FontSize(typeFootnote * s).FontWeight(600).TextColor(ink).SingleLine().Grow(1).Basis(0).MinWidth(0)
		})
	})
}

// installAction is the primary button's label, state and action.
func (a *App) installAction() (string, bool, func()) {
	p := a.project
	r := a.root()
	noVer := a.form.VersionID == ""
	switch p.Type {
	case "mod":
		items, enable, skip := a.planSelection()
		n := len(items) + len(enable)
		var target launcher.PlanItem
		for _, it := range a.plan {
			if it.Role == "target" {
				target = it
			}
		}
		lbl := a.L("install_dialog.install")
		if target.File != "" {
			lbl = a.L("install_dialog.reinstall")
			if target.Note != "" && target.Version != "" && !strings.Contains(target.Version, target.Note) {
				lbl = a.L("install_dialog.switch_to_this_version")
			}
		}
		if n > 1 {
			lbl = fmt.Sprintf(a.L("install_dialog.install_items"), n)
		}
		disabled := noVer || r == nil || a.projLoading || a.planLoading || a.planErr != "" || a.planKey != a.currentPlanKey() || !a.selectedProjectVersionFits() || target.Status == "resolving" || target.Status == "none" || n == 0
		for _, it := range a.plan {
			if it.Status == "conflict" || (it.Role == "required" && (it.Status == "none" || it.Status == "resolving")) {
				disabled = true
			}
		}
		return lbl, disabled, func() {
			b, _ := json.Marshal(map[string]any{"items": items, "enable": enable, "skip": skip})
			a.planFinished = false
			a.execute("installPlanned", map[string]string{"rootId": r.ID, "data": string(b)}, func(raw json.RawMessage) {
				a.planJob = decode[string](raw)
			})
		}
	case "resourcepack", "shader":
		lbl := a.L("install_dialog.install")
		return lbl, noVer || r == nil, func() {
			a.execute("installAssetPack", map[string]string{"rootId": r.ID, "kind": p.Type, "source": p.Source, "projectId": p.ID, "versionId": a.form.VersionID}, a.taskAdded)
		}
	}
	_, bad := a.packDestCheck()
	return a.L("discover.download"), noVer || bad || a.form.Parent == "" || a.form.Name == "", func() {
		a.execute("importPack", launcher.ImportRequest{Parent: a.form.Parent, Name: a.form.Name, Source: p.Source, ProjectID: p.ID, VersionID: a.form.VersionID}, a.taskAdded)
	}
}

var writableCache = map[string]bool{}
