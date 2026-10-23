package nativeui

import (
	"encoding/json"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func (a *App) search() { a.searchAt(0) }

func (a *App) searchAt(offset int) {
	kind := normalizeResourceType(a.resourceType)
	loader := loaderKey(a.filterLoader)
	if loader == "vanilla" || (kind != "mod" && kind != "modpack") {
		loader = ""
	}
	sortKey := normalizeSortKey(a.searchSort)
	sort := ""
	if sortKey == "downloads" || sortKey == "updated" {
		sort = sortKey
	}
	q := launcher.SearchQuery{Source: a.provider, Query: a.query, Type: kind, Minecraft: a.filterMC, Loader: loader, Offset: offset, Sort: sort, Categories: append([]string(nil), a.selectedCats()...)}
	a.startDiscoverRequest(q, false)
}

func (a *App) openProject(p launcher.Project) {
	a.project = &p
	a.projectVersions = nil
	a.projLoading, a.projShowAll = true, false
	a.plan, a.planKey, a.planJob, a.planReplace, a.planErr, a.planOff = nil, "", "", "", "", nil
	if a.planCancel != nil {
		a.planCancel()
	}
	a.projectVersionFilter = ""
	a.dialog = "project"
	a.dialogOpen = true
	a.form = formState{Name: cleanName(p.Title)}
	if r := a.root(); r != nil {
		dir := r.Path
		if p.Type == "modpack" && dir != "" {
			a.form.Parent = dir // the game folder chosen in the sidebar
			a.form.Name = uniqueDirName(dir, cleanName(p.Title))
		}
	}
	mc, loader := "", ""
	if p.Type == "mod" {
		if v := a.gameVersion(); v != nil {
			mc = v.Minecraft
			loader = v.Loader
		}
	} else if p.Type == "resourcepack" || p.Type == "shader" {
		if v := a.gameVersion(); v != nil {
			mc = v.Minecraft
		}
	}
	key := p.Source + p.ID
	a.execute("versions", map[string]string{"source": p.Source, "projectId": p.ID, "minecraft": mc, "loader": loader}, func(b json.RawMessage) {
		if a.project == nil || a.project.Source+a.project.ID != key {
			return
		}
		a.projLoading = false
		a.projectVersions = decode[[]launcher.ProjectVersion](b)
		a.form.VersionID = a.recommendedVersion()
	})
}
