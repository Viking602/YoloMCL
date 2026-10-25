package nativeui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func (a *App) importDialog(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	label(c, t.ImportDesc)
	ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
		label(c, t.ImportFileLabel).FontWeight(600)
		ui.Row(c).FillWidth().Gap(10 * s).Children(func() {
			input := tooltip(c, textField(c, &a.form.Path).Label(t.ImportFileLabel).Placeholder(t.ImportFilePlaceholder), t.ImportFileHint).Grow(1).MinWidth(0)
			if input.Changed() && strings.TrimSpace(a.form.Path) != "" {
				a.inspectPackFile(a.form.Path)
			}
			if button(c, "pick-pack", t.DialogBrowseBtn, "folder", false, a.dialogPending()) {
				a.chooseFiles("pack", func(paths []string) {
					a.inspectPackFile(paths[0])
				})
			}
		})
		label(c, t.ImportFileHint).FontSize(typeCaption * s).FillWidth().MinWidth(0)
	})
	if p := a.packPreview; p != nil {
		ui.Row(c).FillWidth().Padding(14*s, 16*s).Radius(12*s).Background(accentSoft).Border(1, blue).Gap(14 * s).AlignItems(ui.Center).Children(func() {
			logo := modrinthLogo
			if p.Source == "curseforge" {
				logo = curseforgeLogo
			}
			ui.Box(c).Size(40*s, 40*s).Radius(10*s).Background(panel).Border(1, line).Center().Shrink(0).Children(func() {
				if p.Source == "multimc" {
					if !loaderLogo(c, p.Loader, 26*s) {
						ui.Icon(c, icons["layers"]).Size(20*s, 20*s).TextColor(accent)
					}
					return
				}
				ui.Image(c, logo).Size(24*s, 24*s).Fit(ui.Contain)
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(4 * s).Children(func() {
				ui.Row(c).Gap(8 * s).AlignItems(ui.Center).Children(func() {
					ui.Text(c, p.Name).FontSize(typeSubhead * s).Bold().TextColor(ink).SingleLine()
					if p.Version != "" {
						badge(c, p.Version, accent)
					}
				})
				meta := fmt.Sprintf("%s   %s", p.Minecraft, a.loaderDisplay(p.Loader))
				if p.LoaderVersion != "" {
					meta += " " + p.LoaderVersion
				}
				if p.Source == "multimc" {
					meta += "   " + t.ImportSourceMultiMC
				} else {
					meta += fmt.Sprintf("   %d files", p.FileCount)
				}
				label(c, meta).FontSize(typeCallout * s).SingleLine()
			})
		})
		if p.Unavailable != "" {
			ui.Row(c).Label("整合包不可用").FillWidth().Padding(10*s, 14*s).Radius(10 * s).Background(dangerSoft).Gap(8 * s).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons["cross"]).Size(14*s, 14*s).TextColor(danger).Shrink(0)
				ui.Text(c, p.Unavailable).FontSize(typeCallout * s).TextColor(danger).Grow(1).MinWidth(0)
			})
		}
	} else if a.packErr != "" && strings.TrimSpace(a.form.Path) != "" {
		ui.Row(c).Label("整合包错误").FillWidth().Padding(10*s, 14*s).Radius(10 * s).Background(dangerSoft).Children(func() {
			ui.Text(c, a.packErr).FontSize(typeCallout * s).TextColor(danger).Grow(1).MinWidth(0)
		})
	}
	a.folderField(c, t.ImportParentLabel, &a.form.Parent)
	field(c, t.ImportNameLabel, &a.form.Name, t.ImportNamePlaceholder)
	label(c, t.ImportNote).FontSize(typeCallout * s)
	if p := a.packPreview; p != nil && p.DownloadMB > 0 && p.Unavailable == "" {
		size := fmt.Sprintf("%d MB", p.DownloadMB)
		if p.DownloadMB >= 1024 {
			size = fmt.Sprintf("%.1f GB", float64(p.DownloadMB)/1024)
		}
		label(c, fmt.Sprintf(t.ImportSizeNote, size)).Label("导入下载大小").FontSize(typeCallout * s).TextColor(accent)
	}
	blocked := a.packPreview == nil || a.packPreview.Unavailable != ""
	if buttonWhy(c, "import-pack-now", t.ImportConfirmBtn, "download", true, whyOf(a.dialogPending(), t.WhyBusy, a.packPreview != nil && a.packPreview.Unavailable != "", a.packPreviewUnavailable(), blocked || a.form.Path == "" || a.form.Parent == "" || a.form.Name == "", t.WhyImportMissing), true) {
		a.execute("importPack", launcher.ImportRequest{Path: a.form.Path, Parent: a.form.Parent, Name: a.form.Name}, a.taskAdded)
	}
}

func (a *App) packPreviewUnavailable() string {
	if a.packPreview == nil {
		return ""
	}
	return a.packPreview.Unavailable
}
