package nativeui

import (
	"encoding/json"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func (a *App) root() *launcher.Instance {
	for i := range a.snapshot.Instances {
		if a.snapshot.Instances[i].ID == a.snapshot.ActiveRoot {
			return &a.snapshot.Instances[i]
		}
	}
	return nil
}

func (a *App) account() *launcher.Account {
	for i := range a.snapshot.Accounts {
		if a.snapshot.Accounts[i].ID == a.snapshot.ActiveAccount {
			return &a.snapshot.Accounts[i]
		}
	}
	return nil
}

func (a *App) gameVersion() *launcher.VersionSummary {
	if r := a.root(); r != nil {
		for i := range r.Versions {
			if r.Versions[i].ID == r.SelectedVersion {
				return &r.Versions[i]
			}
		}
	}
	return nil
}

func (a *App) selectRoot(id string) {
	a.selectVersion(id, "")
}

// selectVersion switches the current version in place: the local snapshot
// changes at once (no busy state, so no button turns disabled, and no
// rescan of the version folders), the choice is saved in the background,
// and only a failure rolls the selection back.
func (a *App) selectVersion(rootID, versionID string) {
	prevRoot, prevVersion, ok := a.applySelection(rootID, versionID)
	if !ok {
		a.execute("select", map[string]string{"rootId": rootID, "version": versionID}, func(json.RawMessage) { a.loadSelectedInstance() })
		return
	}
	if prevRoot == rootID && (versionID == "" || prevVersion == versionID) {
		return
	}
	epoch := a.selectionEpoch.Add(1)
	a.resetImageRequests()
	if prevRoot != rootID {
		a.instLoaded, a.instScrollPending = rootID, true
	}
	a.loadSelectedInstance()
	if a.service == nil {
		return
	}
	a.selectionPending = true
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		// Serialize writes and skip requests superseded before they start.
		// A slow disk must not persist an older click after the latest one.
		a.selectionMu.Lock()
		if epoch != a.selectionEpoch.Load() {
			a.selectionMu.Unlock()
			return
		}
		e := a.service.Select(rootID, "", versionID)
		a.selectionMu.Unlock()
		a.post(func() {
			if epoch != a.selectionEpoch.Load() {
				return
			}
			a.selectionPending = false
			a.selectionEpoch.Add(1) // invalidate snapshots begun before the save
			if e == nil {
				return
			}
			a.applySelection(prevRoot, prevVersion)
			a.loadSelectedInstance()
			a.message, a.errorMessage = a.friendlyErr(e), true
		})
	}()
}

// Only load the selected pane. Overview counts and storage need directory
// metadata, not full JAR hashes, nested archives or remote mod update checks.
func (a *App) loadSelectedInstance() {
	a.cancelAddonLoad()
	a.cancelInstanceStatus()
	a.cancelServerPing()
	if a.page == "mods" {
		a.ensureAddons()
	} else if a.page == "home" {
		if a.instTab == "" || a.instTab == "overview" {
			a.loadInstanceStatus()
		} else {
			a.setInstTab(a.instTab)
		}
	}
}

// A snapshot reads cached inventories after reading the saved selection.
// It may arrive after the user has clicked another pack, or after its save.
// Apply fresh jobs/settings while retaining the newer local selection.
func (a *App) applySnapshot(snap launcher.Snapshot, epoch uint64) {
	if a.selectionPending || epoch != a.selectionEpoch.Load() {
		snap.Instances = append([]launcher.Instance(nil), snap.Instances...)
		for i := range snap.Instances {
			r := &snap.Instances[i]
			if current := a.rootByID(r.ID); current != nil {
				if r.SelectedVersion != current.SelectedVersion {
					r.SelectedVersion, r.ModCount = current.SelectedVersion, current.ModCount
				}
				if r.ID == a.snapshot.ActiveRoot {
					snap.ActiveRoot = r.ID
				}
			}
		}
	}
	a.snapshot = snap
}

// applySelection updates the in-memory snapshot and returns what was
// selected before; ok is false when the root is not in the snapshot.
func (a *App) applySelection(rootID, versionID string) (prevRoot, prevVersion string, ok bool) {
	prevRoot = a.snapshot.ActiveRoot
	for i := range a.snapshot.Instances {
		r := &a.snapshot.Instances[i]
		if r.ID == prevRoot {
			prevVersion = r.SelectedVersion
		}
	}
	for i := range a.snapshot.Instances {
		r := &a.snapshot.Instances[i]
		if r.ID == rootID {
			a.snapshot.ActiveRoot = rootID
			if versionID != "" {
				r.SelectedVersion = versionID
			}
			return prevRoot, prevVersion, true
		}
	}
	return prevRoot, prevVersion, false
}

// versionSwapKey identifies what the featured card and the dock show, so
// their text crossfades when the selection changes.
func (a *App) versionSwapKey() string {
	if r := a.root(); r != nil {
		return r.ID + "/" + r.SelectedVersion
	}
	return ""
}

func (a *App) rootByID(id string) *launcher.Instance {
	for i := range a.snapshot.Instances {
		if a.snapshot.Instances[i].ID == id {
			return &a.snapshot.Instances[i]
		}
	}
	return nil
}
