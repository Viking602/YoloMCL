package nativeui

import (
	"encoding/json"

	"github.com/egoist/mygo"
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
		a.openDialog("account")
		return
	}
	if r.SelectedVersion == "" {
		a.openDialog("install")
		return
	}
	a.execute("launch", map[string]string{"rootId": r.ID, "accountId": a.snapshot.ActiveAccount}, func(json.RawMessage) { a.notice(a.tr().InstallLaunchPreparing) })
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
