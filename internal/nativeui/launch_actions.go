package nativeui

import (
	"encoding/json"

	"github.com/egoist/mygo"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func (a *App) openFolder(id string) {
	p, e := a.service.RootPath(id)
	if e == nil {
		e = mygo.Shell.OpenPath(p)
	}
	if e != nil {
		a.message = a.friendlyErr(e)
		a.errorMessage = true
	}
}

func (a *App) launch() {
	r := a.root()
	if r == nil {
		a.openDialog("add")
		return
	}
	if a.account() == nil {
		if len(a.snapshot.Accounts) == 0 {
			a.openDialog("accountAdd")
		} else {
			a.openDialog("account")
		}
		return
	}
	if r.SelectedVersion == "" {
		a.openDialog("install")
		return
	}
	rootID, accountID := r.ID, a.snapshot.ActiveAccount
	start := func() {
		a.execute("launch", map[string]string{"rootId": rootID, "accountId": accountID}, func(json.RawMessage) { a.notice(a.tr().InstallLaunchPreparing) })
	}
	if a.saveLaunchDraft(*r, start) {
		return
	}
	start()
}

// Launching from settings applies the visible runtime first. This also covers
// opening the account chooser for another copy of an already running game.
func (a *App) saveLaunchDraft(r launcher.Instance, start func()) bool {
	if a.instancePending("updateVersion") {
		return true
	}
	if a.versionDraftRoot == r.ID && a.form.ConfigVersion == r.SelectedVersion && a.versionDraftDirty() {
		a.saveVersionConfigThen(r.ID, start)
		return true
	}
	return false
}

// launchRoot makes a directory current (keeping its selected version) and
// launches it, as 「启动」 on a finished import task does.
func (a *App) launchRoot(id string) {
	if a.snapshot.ActiveRoot == id {
		a.launch()
		return
	}
	a.execute("select", map[string]string{"rootId": id}, func(json.RawMessage) {
		a.snapshot = a.service.Snapshot()
		a.launch()
	})
}
