package nativeui

import (
	"strings"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type undoNotice struct {
	message          string
	undo             func() error
	pending          bool
	changesSelection bool
}

// Undo is published even if the user navigates while the disk operation is in
// flight. Its callback owns an immutable removal receipt, never the new selection.
func (a *App) undoable(key, message string, work func() (func() error, error), refresh func()) {
	if a.pending == nil {
		a.pending = map[string]int{}
	}
	if a.pending[key] > 0 {
		return
	}
	a.pending[key]++
	epoch := a.selectionEpoch.Load()
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		undo, err := work()
		snap := a.service.Snapshot()
		a.post(func() {
			delete(a.pending, key)
			a.applySnapshot(snap, epoch)
			if err != nil {
				a.message, a.errorMessage = a.friendlyErr(err), true
				return
			}
			a.notice(message)
			if undo != nil {
				a.noticeUndo = &undoNotice{message: message, undo: undo, changesSelection: strings.HasPrefix(key, "removeRoot:")}
			}
			if refresh != nil {
				refresh()
			}
		})
	}()
}

func (a *App) undoRemoval() {
	u := a.noticeUndo
	if u == nil || u.pending {
		return
	}
	if u.changesSelection && a.guardDiscard(a.hasUnsavedDraft(), a.undoRemoval) {
		return
	}
	u.pending = true
	epoch := a.selectionEpoch.Load()
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		err := u.undo()
		snap := a.service.Snapshot()
		a.post(func() {
			u.pending = false
			if err != nil {
				a.message, a.errorMessage = a.friendlyErr(err), true
				a.noticeFor, a.noticeRetry = a.message, a.undoRemoval
				return
			}
			if a.noticeUndo == u {
				a.noticeUndo = nil
			}
			a.applySnapshot(snap, epoch)
			a.refreshRemovalView()
			a.notice(a.L("floating.undone"))
		})
	}()
}

func (a *App) removeModUndo(rootID, version, file string) {
	a.undoable("removeMod:"+rootID+":"+file, a.L("floating.mod_removed"), func() (func() error, error) {
		receipt, err := a.service.RemoveModUndo(rootID, version, file)
		return func() error { return a.service.RestoreMod(receipt) }, err
	}, a.loadMods)
}

func (a *App) removeRootUndo(rootID string) {
	a.undoable("removeRoot:"+rootID, a.tr().RemRootSuccess, func() (func() error, error) {
		receipt, err := a.service.RemoveRootUndo(rootID)
		return func() error { return a.service.RestoreRoot(receipt) }, err
	}, a.refreshRemovalView)
}

func (a *App) refreshRemovalView() {
	if a.page == "home" && a.instTab == "settings" {
		return // The editor owns its draft; instanceView loads a newly selected root.
	}
	a.loadSelectedInstance()
}

func (a *App) removeAuthServerUndo(server launcher.AuthServer) {
	a.undoable("deleteAuthServer:"+server.URL, a.tr().AccDeleteServerBtn, func() (func() error, error) {
		err := a.service.DeleteAuthServer(server.URL)
		return func() error { return a.service.RestoreAuthServer(server) }, err
	}, nil)
}

func (a *App) removeAssetUndo(rootID, version, kind, file string) {
	a.undoable("removeAssetPack:"+rootID+":"+file, a.tr().ModsRemoveBtn, func() (func() error, error) {
		receipt, err := a.service.RemoveAssetPackUndo(rootID, version, kind, file)
		return func() error { return a.service.RestoreMod(receipt) }, err
	}, a.loadMods)
}
