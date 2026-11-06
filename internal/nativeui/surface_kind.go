package nativeui

import "github.com/egoist/mygo/ui"

type surfaceKind uint8

const (
	surfaceDialog surfaceKind = iota
	surfaceDrawer
	surfacePage
)

func (a *App) surfaceKind(name string) surfaceKind {
	switch name {
	case "install", "logs", "java":
		return surfacePage
	case "launcherUpdate":
		if a.launcherUpdateRequired() {
			return surfacePage
		}
	case "project":
		if a.project != nil && a.project.Type == "modpack" {
			return surfacePage
		}
		return surfaceDrawer
	case "account", "export", "keyBindings", "packReport":
		return surfaceDrawer
	case "packUpdate":
		if a.packPlanReport {
			return surfaceDrawer
		}
	}
	return surfaceDialog
}

// Main tasks occupy the existing page slot. Navigation and the original close
// action return to the page behind them, with the same draft guard as a dialog.
func (a *App) taskPage(c *ui.Context) bool {
	if !a.dialogOpen || a.surfaceKind(a.dialog) != surfacePage {
		return false
	}
	s := uiScale(c)
	a.dialogShown = a.dialog
	_, h := c.Size()
	contentPanel(ui.Column(c.Key("task-page-"+a.dialog)).Lang(a.lang()).Fill().Margin(0, 10*s, 10*s, 10*s), s).Children(func() {
		ui.Column(c).Fill().MaxWidth(1160 * s).AlignSelf(ui.Center).Padding(24 * s).Children(func() {
			box := ui.Column(c).Fill().MinHeight(0)
			box.Children(func() { a.surfacePanel(c, ui.Element{}, box, 1, s, h-24*s, surfacePage) })
		})
	})
	return true
}
