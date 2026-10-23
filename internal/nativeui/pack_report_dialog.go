package nativeui

import (
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func (a *App) packDialog(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	r := a.packReport
	if r == nil || r.Pack == nil {
		return
	}
	ui.Text(c, r.Pack.Name+" / "+r.Pack.Version).FontSize(typeTitle * s).Bold().TextColor(ink).SingleLine()
	if r.Latest != nil {
		badge(c, t.PackUpdateFoundPrefix+r.Latest.Number, accent)
		if button(c, "pack-update-install", t.PackUpdateInstallBtn, "download", true, a.dialogPending()) {
			p := launcher.Project{ID: r.Pack.ProjectID, Source: r.Pack.Source, Title: r.Pack.Name, Type: "modpack"}
			v := *r.Latest
			a.project = &p
			a.projectVersions = []launcher.ProjectVersion{v}
			a.form = formState{Name: cleanName(p.Title + "-" + v.Number), VersionID: v.ID}
			a.dialog = "project"
		}
	}
	for _, f := range r.Files {
		ui.Row(c).Gap(14 * s).AlignItems(ui.Center).Children(func() {
			pathLabel(c, f.Path, 13*s)
			status := a.packFileStatusText(f.Status)
			color := muted
			if f.Status == "missing" || f.Status == "modified" {
				color = accent
			}
			badge(c, status, color)
		})
	}
	for _, manual := range r.Pack.Manual {
		m := manual
		label(c, t.PackManualPrefix+m.Name)
		if button(c, "manual-"+m.FileID, t.PackManualOpenBtn, "arrow", false, false) {
			c.OpenURL(m.URL)
		}
	}
}
