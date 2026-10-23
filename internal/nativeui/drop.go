package nativeui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func (a *App) inspectPackFile(path string) {
	a.packPreview = nil
	a.packErr = ""
	a.form.Path = path
	if _, err := a.service.InspectPack(path); err != nil {
		a.packErr = a.friendlyErr(err)
	}
	if preview, err := a.service.InspectPack(path); err == nil {
		a.packPreview = &preview
		if a.form.Name == "" {
			name := preview.Name
			if preview.Version != "" && !strings.Contains(name, preview.Version) {
				name += "-" + preview.Version
			}
			a.form.Name = cleanName(name)
		}
	}
	if a.form.Name == "" {
		a.form.Name = cleanName(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	}
	if a.form.Parent == "" {
		if r := a.root(); r != nil {
			p := r.Path
			if strings.EqualFold(filepath.Base(p), ".minecraft") {
				p = filepath.Dir(p)
			}
			a.form.Parent = filepath.Dir(p)
		} else {
			a.form.Parent = filepath.Dir(path)
		}
	}
}

func (a *App) handleDroppedFiles(paths []string) {
	if len(paths) == 0 {
		return
	}
	first := paths[0]
	if st, err := os.Stat(first); err == nil && st.IsDir() {
		a.openDialog("add")
		a.form.Path = first
		return
	}
	ext := strings.ToLower(filepath.Ext(first))
	switch ext {
	case ".mrpack":
		if a.offerPackUpdate(first) {
			return
		}
		a.openDialog("import")
		a.inspectPackFile(first)
	case ".zip":
		kind, err := a.service.ArchiveKind(first)
		if err != nil {
			a.message, a.errorMessage = a.friendlyErr(err), true // never guess
			return
		}
		if kind == "modpack" {
			if a.offerPackUpdate(first) {
				return
			}
			a.openDialog("import")
			a.inspectPackFile(first)
			return
		}
		if r := a.root(); r == nil {
			a.message, a.errorMessage = a.tr().DropNeedRoot, true
			return
		} else {
			a.execute("addLocalAssetPacks", map[string]any{"rootId": r.ID, "kind": kind, "paths": paths}, func(json.RawMessage) {
				a.addonTab = kind
				a.goPage("mods")
				a.loadMods()
			})
			return
		}
	case ".jar":
		if r := a.root(); r != nil {
			var jars []string
			for _, p := range paths {
				if strings.EqualFold(filepath.Ext(p), ".jar") {
					jars = append(jars, p)
				}
			}
			if len(jars) > 0 {
				a.execute("addLocalMods", map[string]any{"rootId": r.ID, "paths": jars}, func(json.RawMessage) {
					a.addonTab = "mods"
					a.goPage("mods")
					a.loadMods()
				})
			}
		} else {
			a.openDialog("add")
		}
	}
}
