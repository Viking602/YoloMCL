package nativeui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// uniqueDirName is name, or "name (2)", "name (3)"… so it is free in dir.
func uniqueDirName(dir, name string) string {
	if name == "" {
		return ""
	}
	out := name
	for i := 2; i < 100; i++ {
		if _, e := os.Stat(filepath.Join(dir, out)); os.IsNotExist(e) {
			return out
		}
		out = fmt.Sprintf("%s (%d)", name, i)
	}
	return out
}

// packDestCheck is the hint under the modpack fields: the full path, or a
// friendly reason it cannot be used (bad = true).
func (a *App) packDestCheck() (string, bool) {
	parent, name := strings.TrimSpace(a.form.Parent), strings.TrimSpace(a.form.Name)
	if parent == "" || name == "" {
		return a.L("请填写安装位置和实例名称", "Enter a location and an instance name"), true
	}
	if cleanName(name) != name || filepath.Base(name) != name {
		return a.L(`名称不能包含 \ / : * ? " < > |`, `The name cannot contain \ / : * ? " < > |`), true
	}
	if st, e := os.Stat(parent); e != nil || !st.IsDir() {
		return a.L("安装位置不存在，请重新选择", "This location does not exist. Choose another"), true
	}
	if !dirWritable(parent) {
		return a.L("没有权限写入这个位置，请换一个文件夹或以管理员身份运行", "No permission to write here. Choose another folder or run as administrator"), true
	}
	full := filepath.Join(parent, name)
	if _, e := os.Stat(full); e == nil {
		return a.L("已有同名文件夹，请换一个名称", "A folder with this name already exists. Use another name"), true
	}
	return a.L("将创建 ", "Creates ") + full, false
}

func dirWritable(dir string) bool {
	if v, ok := writableCache[dir]; ok {
		return v
	}
	f, e := os.CreateTemp(dir, ".mcl-w-*")
	ok := e == nil
	if ok {
		n := f.Name()
		f.Close()
		os.Remove(n)
	}
	writableCache[dir] = ok
	return ok
}

// packDestFields: location and name on aligned rows, with one fixed-height
// line below that either previews the folder or says what to fix.
func (a *App) packDestFields(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	labelW := 72 * s
	ui.Column(c.Key("pack-dest")).FillWidth().Gap(10 * s).Children(func() {
		ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
			ui.Text(c, t.ImportParentLabel).FontSize(typeFootnote * s).FontWeight(600).TextColor(inkSecondary).Width(labelW).Shrink(0)
			textField(c.Key("pack-parent"), &a.form.Parent).Label(t.ImportParentLabel).Placeholder(t.DialogChooseFolder).Grow(1).MinWidth(0)
			if button(c, "folder-pack-parent", t.DialogBrowseBtn, "folder", false, false) {
				a.chooseFolder(&a.form.Parent)
			}
		})
		msg, bad := a.packDestCheck()
		ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
			ui.Text(c, t.ImportNameLabel).FontSize(typeFootnote * s).FontWeight(600).TextColor(inkSecondary).Width(labelW).Shrink(0)
			fld := textField(c.Key("pack-name"), &a.form.Name).Label(t.ImportNameLabel).Grow(1).MinWidth(0)
			if bad && strings.TrimSpace(a.form.Name) != "" {
				fld.Border(1, danger.Alpha(.6))
			}
		})
		ui.Row(c).FillWidth().Height(18*s).Gap(6*s).Padding(0, 0, 0, labelW+12*s).AlignItems(ui.Center).Children(func() {
			col, icon := muted, "folder"
			if bad {
				col, icon = danger, "info"
			}
			ui.Icon(c, icons[icon]).Size(11*s, 11*s).TextColor(col).Shrink(0)
			ui.Text(c, msg).FontSize(typeCaption * s).TextColor(col).SingleLine().Shrink(1).MinWidth(0).Tooltip(msg)
		})
	})
}

// packSummary is "共 74 个模组  约 180 MB" for the selected pack version,
// read from its index in the background; "" until known.
func (a *App) packSummary() string {
	p := a.project
	if p == nil || p.Type != "modpack" || a.form.VersionID == "" {
		return ""
	}
	if a.packRemote == nil {
		a.packRemote = map[string]*launcher.PackPreview{}
	}
	id := a.form.VersionID
	pv, asked := a.packRemote[id]
	if !asked {
		a.packRemote[id] = nil
		// silent: a large pack just shows the version size; no error banner
		payload, _ := json.Marshal(map[string]string{"source": p.Source, "projectId": p.ID, "versionId": id})
		go func() {
			raw, e := a.service.Dispatch(a.ctx, "previewRemotePack", string(payload))
			a.post(func() {
				v := launcher.PackPreview{}
				if e == nil {
					v = decode[launcher.PackPreview](json.RawMessage(raw))
				}
				a.packRemote[id] = &v
			})
		}()
		return ""
	}
	if pv == nil {
		return ""
	}
	out := ""
	if pv.ModCount > 0 {
		out = fmt.Sprintf(a.L("共 %d 个模组", "%d mods"), pv.ModCount)
	}
	if pv.DownloadMB > 0 {
		out += fmt.Sprintf(a.L("  约 %d MB", "  about %d MB"), pv.DownloadMB)
	}
	return strings.TrimSpace(out)
}
