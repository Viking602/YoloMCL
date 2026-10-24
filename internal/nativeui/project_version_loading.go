package nativeui

import (
	"context"
	"time"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func (a *App) openProject(p launcher.Project) {
	a.cancelProjectRequests()
	a.dialogGen++
	a.project = &p
	a.projectVersions = nil
	a.versionList = projectVersionListState{}
	a.projLoading, a.projShowAll = false, false
	a.verFilter = verFilter{}
	a.projectMCChoice = projectChoiceState{}
	a.plan, a.planKey, a.planJob, a.planReplace, a.planErr, a.planOff = nil, "", "", "", "", nil
	a.planLoading, a.planFinished = false, false
	a.message, a.errorMessage = "", false
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
	a.reloadProjectVersions()
}

type projectVersionsFunc func(context.Context, string, string, string, string) ([]launcher.ProjectVersion, error)

type projectVersionRequest struct {
	revision uint64
	cancel   context.CancelFunc
	err      string
	fetch    projectVersionsFunc
}

func (a *App) cancelProjectRequests() {
	f := &a.projectFetch
	f.revision++
	if f.cancel != nil {
		f.cancel()
		f.cancel = nil
	}
	a.planRevision++
	if a.planCancel != nil {
		a.planCancel()
		a.planCancel = nil
	}
}

// Fetch the catalog once. Instance compatibility is a local filter, so it
// cannot erase the choices needed to browse other Minecraft versions/loaders.
func (a *App) reloadProjectVersions() {
	p, f := a.project, &a.projectFetch
	if p == nil {
		return
	}
	fetch := f.fetch
	if fetch == nil && a.service != nil {
		fetch = a.service.Versions
	}
	if fetch == nil {
		return
	}
	if f.cancel != nil {
		f.cancel()
	}
	f.revision++
	rev, src, id := f.revision, p.Source, p.ID
	ctx, cancel := context.WithTimeout(a.ctx, 90*time.Second)
	f.cancel, f.err = cancel, ""
	a.projLoading, a.form.VersionID = true, ""
	a.clearPlan()
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		defer cancel()
		vs, err := fetch(ctx, src, id, "", "")
		a.post(func() {
			if f.revision != rev || a.project == nil || a.project.Source != src || a.project.ID != id || !a.dialogOpen {
				return
			}
			a.projLoading, f.cancel = false, nil
			if err != nil {
				f.err = a.friendlyErr(err)
				return
			}
			a.projectVersions = vs
		})
	}()
}
