package nativeui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

func (a *App) loadVersionConfigForm(versionID string) {
	a.form.ConfigVersion = versionID
	a.form.VerJava = ""
	a.form.VerMemory = "0"
	a.form.VerJVM = ""
	a.form.VerName = ""
	a.form.VerMemAuto = true
	if r := a.root(); r != nil {
		a.form.VerMemAuto, _ = r.MemoryPlan(versionID, a.settings)
		if !a.form.VerMemAuto {
			_, mb := r.MemoryPlan(versionID, a.settings)
			a.form.VerMemory = fmt.Sprint(mb)
		}
	}
	if r := a.root(); r != nil && r.VersionConfigs != nil {
		if vc, ok := r.VersionConfigs[versionID]; ok {
			a.form.VerJava = vc.JavaPath
			if vc.MemoryMB > 0 {
				a.form.VerMemory = fmt.Sprint(vc.MemoryMB)
			}
			a.form.VerJVM = vc.JVMArgs
			a.form.VerName = vc.DisplayName
		}
	}
}

func (a *App) openVersionConfig(rootID, versionID string) {
	if rootID != "" && rootID != a.snapshot.ActiveRoot {
		_ = a.service.Select(rootID, "", versionID)
		a.snapshot = a.service.Snapshot()
	}
	a.openDialog("version")
	if versionID != "" {
		a.loadVersionConfigForm(versionID)
	}
}

func (a *App) openDialog(name string) {
	a.dialogGen++
	t := a.tr()
	a.dialog = name
	a.dialogOpen = true
	a.packPreview = nil
	a.form = formState{Loader: t.LoaderVanilla, Target: t.InstallTargetNew, Memory: "0", VerMemory: "0"}
	a.message = ""
	switch name {
	case "install":
		if a.root() != nil {
			a.form.Target = t.InstallTargetCurrent
		}
		a.versionQuery = ""
		a.versionKind = t.InstallKindRelease
		if a.settings.ShowSnapshots {
			a.versionKind = t.InstallKindAll
		}
		a.loadManifest()
	case "import":
		if r := a.root(); r != nil {
			p := r.Path
			if strings.EqualFold(filepath.Base(p), ".minecraft") {
				p = filepath.Dir(p)
			}
			a.form.Parent = filepath.Dir(p)
		}
	case "root":
		a.ensureJavaScan()
		if r := a.root(); r != nil {
			a.form.Name = r.Name
			a.form.Java = r.JavaPath
			a.form.Memory = fmt.Sprint(r.MemoryMB)
			a.form.MemAuto = r.MemoryMB <= 0
			if a.form.MemAuto {
				a.form.Memory = "4096"
			}
			a.form.JVM = r.JVMArgs
			ver := r.SelectedVersion
			if ver == "" && len(r.Versions) > 0 {
				ver = r.Versions[0].ID
			}
			a.loadVersionConfigForm(ver)
		}
	case "account":
		a.accountAdding, a.serverEdit, a.serverCustom = false, nil, false
	case "version":
		a.ensureJavaScan()
		if r := a.root(); r != nil {
			ver := r.SelectedVersion
			if ver == "" && len(r.Versions) > 0 {
				ver = r.Versions[0].ID
			}
			a.loadVersionConfigForm(ver)
		}
	case "logs":
		a.loadLogs()
	}
}

func (a *App) closeDialog() {
	a.dialogGen++
	if a.device != nil {
		a.service.CancelMicrosoft(a.device.ID)
	}
	a.device = nil
	a.profiles = nil
	a.dialog = ""
	a.dialogOpen = false
	a.form.Password = ""
}

func (a *App) executeDialog(fn func() error) {
	if a.win == nil {
		return
	}
	go func() {
		if e := fn(); e != nil {
			a.post(func() { a.message = a.friendlyErr(e); a.errorMessage = true })
		}
	}()
}

func (a *App) chooseFolder(target *string) {
	t := a.tr()
	a.executeDialog(func() error {
		paths, e := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: t.DialogChooseFolder, Directory: true, ShowHiddenFiles: true})
		a.post(func() {
			if e == nil && len(paths) > 0 {
				*target = paths[0]
			}
		})
		return e
	})
}

func (a *App) chooseFiles(kind string, done func([]string)) {
	t := a.tr()
	filters := []mygo.FileFilter{{Name: t.DiscoverTypeModpack, Extensions: []string{"mrpack", "zip"}}}
	multiple := false
	switch kind {
	case "mods":
		filters = []mygo.FileFilter{{Name: t.DiscoverTypeMod, Extensions: []string{"jar"}}}
		multiple = true
	case "zip":
		filters = []mygo.FileFilter{{Name: "ZIP Archive", Extensions: []string{"zip"}}}
		multiple = true
	case "png":
		filters = []mygo.FileFilter{{Name: "PNG", Extensions: []string{"png"}}}
	case "java":
		filters = []mygo.FileFilter{{Name: "Java", Extensions: []string{"exe"}}}
	}
	a.executeDialog(func() error {
		paths, e := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Filters: filters, Multiple: multiple, Title: t.DialogBrowseBtn})
		if e == nil && len(paths) > 0 {
			a.post(func() { done(paths) })
		}
		return e
	})
}

// closeDialogNow closes the dialog without its exit animation (used when
// the page underneath changes at the same moment).
func (a *App) closeDialogNow() {
	a.closeDialog()
	a.dialogShown = ""
}
