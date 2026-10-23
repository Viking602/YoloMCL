package nativeui

import (
	"encoding/json"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func openPath(path string) error { return mygo.Shell.OpenPath(path) }

func field(c *ui.Context, name string, value *string, placeholder string) {
	s := uiScale(c)
	ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
		label(c, name).FontWeight(600)
		textField(c.Key(name), value).Label(name + "输入").Placeholder(placeholder).FillWidth()
	})
}

func (a *App) folderField(c *ui.Context, name string, value *string) {
	s := uiScale(c)
	t := a.tr()
	ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
		label(c, name).FontWeight(600)
		ui.Row(c).FillWidth().Gap(10 * s).Children(func() {
			textField(c.Key(name), value).Label(name + "输入").Placeholder(t.DialogChooseFolder).Grow(1).MinWidth(0)
			if button(c, "folder-"+name, t.DialogBrowseBtn, "folder", false, a.dialogPending()) {
				a.chooseFolder(value)
			}
		})
	})
}

func (a *App) finished(s string) func(json.RawMessage) {
	return func(json.RawMessage) { a.closeDialog(); a.notice(s) }
}

func (a *App) taskAdded(json.RawMessage) {
	a.closeDialogNow()
	a.goPage("tasks")
	a.notice(a.tr().InstallTaskCreated)
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// mixColor blends from a toward b by t in [0, 1].
func mixColor(a, b ui.Color, t float32) ui.Color {
	t = max(0, min(1, t))
	return ui.RGBA(uint8(float32(a.R)+(float32(b.R)-float32(a.R))*t), uint8(float32(a.G)+(float32(b.G)-float32(a.G))*t), uint8(float32(a.B)+(float32(b.B)-float32(a.B))*t), (float32(a.A)+(float32(b.A)-float32(a.A))*t)/255)
}
