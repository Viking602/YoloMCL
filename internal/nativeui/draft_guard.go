package nativeui

import (
	"reflect"
	"strings"

	"github.com/egoist/mygo"
)

type versionDraft struct {
	form, initial formState
	root          string
}

func versionFields(f formState) formState {
	out := formState{ConfigVersion: f.ConfigVersion, VerName: f.VerName, VerJava: f.VerJava, VerJVM: f.VerJVM, VerMemAuto: f.VerMemAuto}
	if !f.VerMemAuto {
		out.VerMemory = f.VerMemory
	}
	return out
}

func (a *App) versionDraftDirty() bool {
	if a.suspendedVersion != nil {
		d := a.suspendedVersion
		return versionFields(d.form) != versionFields(d.initial)
	}
	return a.page == "home" && a.instTab == "settings" && a.versionDraftRoot == a.snapshot.ActiveRoot && versionFields(a.form) != versionFields(a.versionInitial)
}

func (a *App) dialogDirty() bool {
	if !a.dialogOpen {
		return false
	}
	f, base := a.form, a.dialogInitial
	switch a.dialog {
	case "keyBindings":
		return a.keyBindings != nil && a.keyBindings.dirty()
	case "export":
		return a.packExport != nil && a.packExport.dirty()
	case "root":
		return f.Name != base.Name || f.Java != base.Java || f.JVM != base.JVM || f.MemAuto != base.MemAuto || !f.MemAuto && f.Memory != base.Memory
	case "version":
		return versionFields(f) != versionFields(base)
	case "add":
		return f.Name != base.Name || f.Path != base.Path
	case "import":
		return f.Name != base.Name || f.Path != base.Path || f.Parent != base.Parent
	case "install":
		return f.Name != base.Name || f.Parent != base.Parent || f.Target != base.Target || f.Minecraft != base.Minecraft || f.Loader != base.Loader || f.LoaderVersion != base.LoaderVersion
	case "project":
		return f.Name != base.Name || f.Parent != base.Parent || f.Path != base.Path || a.projectEdited
	case "accountAdd":
		return f.Username != base.Username || f.Password != base.Password || f.Name != base.Name || f.Server != base.Server || a.device != nil || a.serverEditorDirty()
	}
	return false
}

func (a *App) serverEditorDirty() bool {
	return a.serverEdit != nil && (strings.TrimSpace(a.serverEdit.name) != strings.TrimSpace(a.serverEdit.originalName) || strings.TrimSpace(a.serverEdit.url) != strings.TrimSpace(a.serverEdit.originalURL))
}

func (a *App) hasUnsavedDraft() bool {
	return a.dialogDirty() || a.versionDraftDirty() || a.wardrobeDraftDirty()
}

func (a *App) wardrobeDraftDirty() bool {
	return a.page == "wardrobe" && (a.wardrobeSt != nil && len(a.wardrobeSt.pendingSkin) > 0 || a.selectedCape != "" && a.selectedCape != a.activeCapeName())
}

// guardDiscard returns true while a decision is pending. Keeping the action as
// a closure also preserves its original instance/version when selection moves.
func (a *App) guardDiscard(dirty bool, action func()) bool {
	if !dirty || a.discarding {
		return false
	}
	a.confirmAction("discard-draft", a.L("game_options.unsaved"), a.L("floating.discard_hint"), a.L("floating.discard_action"), func() {
		a.discarding = true
		defer func() { a.discarding = false }()
		action()
	})
	return true
}

func (a *App) requestCloseDialog() {
	if !a.guardDiscard(a.dialogDirty(), a.closeDialog) {
		a.closeDialog()
	}
}

func (a *App) restartLauncher() {
	if !a.guardDiscard(a.hasUnsavedDraft(), func() { a.allowClose = true; mygo.App.Relaunch() }) {
		a.allowClose = true
		mygo.App.Relaunch()
	}
}

// A secondary editor can open over instance settings without consuming that
// page's form. Restore both its draft and its comparison baseline on return.
func (a *App) suspendVersionDraft() {
	if a.dialog == "" && a.suspendedVersion == nil && a.page == "home" && a.instTab == "settings" {
		a.suspendedVersion = &versionDraft{a.form, a.versionInitial, a.versionDraftRoot}
	}
}

func (a *App) resumeVersionDraft() {
	if d := a.suspendedVersion; d != nil {
		a.form, a.versionInitial, a.versionDraftRoot = d.form, d.initial, d.root
		a.suspendedVersion = nil
	}
}

func (a *App) instancePageWithDraft(c func()) {
	if d := a.suspendedVersion; d != nil && a.dialogOpen {
		form, initial, root := a.form, a.versionInitial, a.versionDraftRoot
		a.form, a.versionInitial, a.versionDraftRoot = d.form, d.initial, d.root
		c()
		d.form, d.initial = a.form, a.versionInitial
		a.form, a.versionInitial, a.versionDraftRoot = form, initial, root
		return
	}
	c()
}

type exportDraft struct {
	format, name, version, author string
	selected                      []bool
}

func (g *packExportState) draft() exportDraft {
	d := exportDraft{format: g.format, name: g.name, version: g.packVersion, author: g.author}
	for _, e := range g.entries {
		d.selected = append(d.selected, e.Selected)
	}
	return d
}

func (g *packExportState) dirty() bool {
	return g.initial != nil && !reflect.DeepEqual(*g.initial, g.draft())
}
