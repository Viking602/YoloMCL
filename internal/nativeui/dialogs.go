package nativeui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

var lastDialogPrimary string

// Keep the panel at its native size: scaling the whole tree makes the software
// renderer resample every glyph, rounded clip and large shadow.
// Layout margins provide a small slide without resampling its contents.
func (a *App) dialogView(c *ui.Context, host ui.Element) {
	name := a.dialog
	if name == "" {
		name = a.dialogShown
	}
	if a.surfaceKind(name) == surfacePage {
		if !a.dialogOpen {
			a.dialogShown = ""
		}
		return
	}
	target, anim := float32(0), motionDialogExit
	if a.dialogOpen {
		target, anim = 1, motionDialogEnter
		a.dialogShown = a.dialog
	}
	presence := host.AnimateTo("dialog-presence", target, anim)
	if c.Preferences().ReduceMotion {
		presence = target
	}
	if !a.dialogOpen {
		if presence <= 0.01 || a.dialogShown == "" {
			if a.dialogShown == "export" {
				a.packExport = nil
			}
			a.dialogShown = ""
			return
		}
		// draw the dialog that is leaving, without reopening it
		a.dialog = a.dialogShown
		defer func() { a.dialog = "" }()
	}
	if a.device != nil && a.dialog == "accountAdd" {
		a.pollDevice()
		if a.device != nil && !a.polling && !a.pendingOp("startMicrosoft") {
			c.After(max(0, min(time.Until(a.nextDevicePoll), time.Until(a.deviceUntil))))
		}
	}
	a.normalizeStateValues()
	s := uiScale(c)
	_, height := c.Size()
	open := a.dialogOpen
	closing := !open
	capturing := a.dialog == "keyBindings" && a.keyBindings != nil && a.keyBindings.capture != ""
	if closing {
		open = true
	}
	ui.DialogBase(c, &open, func(backdrop, box ui.Element) {
		kind := a.surfaceKind(a.dialog)
		if kind == surfaceDrawer {
			backdrop.AlignItems(ui.End).Justify(ui.Start).Padding(42+12*s, 12*s, 12*s, 12*s)
		}
		a.surfacePanel(c, backdrop, box, presence, s, height, kind)
	})
	if !closing && !open {
		if capturing && a.keyBindings != nil {
			a.keyBindings.capture = ""
			return
		}
		a.requestCloseDialog() // Cancel, backdrop and Escape share draft protection
	}
}

// surfacePanel keeps the original content, controls and styling shared by the
// modal, edge drawer and main-task page. Only the outer container changes.
func (a *App) surfacePanel(c *ui.Context, backdrop, box ui.Element, presence, s, height float32, kind surfaceKind) {
	t := a.tr()
	capturing := a.dialog == "keyBindings" && a.keyBindings != nil && a.keyBindings.capture != ""
	inDialog, dialogPrimaryKey = true, ""
	defer func() { inDialog, lastDialogPrimary = false, dialogPrimaryKey }()
	// Enter activates the dialog's primary action (the first enabled
	// primary button), unless a control inside takes Enter itself.
	// The panel takes the focus as it opens (no ring: not from the
	// keyboard), so Enter and Escape reach it before any field is used.
	box.Lang(a.lang()).Description(a.dialogTitle(a.dialog)).Focusable().AutoFocus().FocusRing(false).IgnoreInput(!a.dialogOpen)
	if !capturing && box.Focused() && box.Shortcut(0, ui.KeyEnter) && lastDialogPrimary != "" {
		enterFired = lastDialogPrimary
	}
	if kind != surfacePage {
		backdrop.Background(ui.RGBA(20, 24, 48, .30*presence))
	}
	y := (1 - presence) * 12 * s
	box.Opacity(presence).Margin(y, 0, -y, 0)
	if kind == surfaceDrawer {
		box.Margin(0, -y, 0, y)
	}
	width := float32(680)
	switch a.dialog {
	case "keyBindings":
		width = 940
	case "disableMod":
		width = 520
	case "logs":
		width = 1100
	case "packReport", "java", "root", "install", "project", "packUpdate":
		width = 780
	}
	if a.dialog == "project" {
		if kind != surfacePage {
			backdrop.Background(ui.RGBA(16, 20, 40, .28*presence))
		}
		dialogSurface(box.Label("活动弹窗").Width(max(560, 820*s)).MaxWidthPercent(92).MaxHeight(height-42-24*s).Padding(0), s).Clip()
		if kind == surfaceDrawer {
			box.Height(height - 42 - 24*s)
		}
		if kind == surfacePage {
			box.FillWidth().MaxWidthPercent(100).FillHeight().Padding(0)
		}
		a.projectInstallDialog(c, height-42-24*s)
		return
	}
	dialogSurface(box.Label("活动弹窗").Width(max(480, width*s)).MaxWidthPercent(94).MaxHeight(height-42-24*s).Padding(24*s), s)
	if kind == surfaceDrawer {
		box.Height(height - 42 - 24*s)
	}
	if kind == surfacePage {
		box.FillWidth().MaxWidthPercent(100).FillHeight().Padding(0).Background(ui.Transparent).Border(0, ui.Transparent).Shadow(0, 0, 0, 0, ui.Transparent)
	}
	body := ui.Column(c).FillWidth().Gap(16 * s)
	if a.dialog == "logs" && kind != surfacePage {
		box.Height(min(820*s, height-42-24*s))
		body.FillHeight().MinHeight(0)
	}
	if a.dialog == "keyBindings" {
		// Keep the controls reachable as the searchable list grows or
		// shrinks, including in the smallest supported window.
		box.Height(min(780*s, height-96))
		body.FillHeight().MinHeight(0)
	}
	if kind != surfaceDialog {
		body.FillHeight().MinHeight(0)
	}
	body.Children(func() {
		ui.Row(c).FillWidth().Shrink(0).Gap(12 * s).AlignItems(ui.Center).Children(func() {
			if a.dialog == "root" || a.dialog == "version" {
				tone := navTones["settings"]
				appleSoftIconBox(c, 30*s, "settings", tone[0], tone[1], panel)
			}
			title := a.dialogTitle(a.dialog)
			titleLabel := ui.Text(c, title).Label("弹窗标题 " + title).FontSize(typeTitle2 * s).Bold().TextColor(ink).Grow(1).MinWidth(0).Shrink(1).SingleLine()
			textTooltip(c, titleLabel, title, "", ui.Font{Family: c.Theme().Font, Size: typeTitle2 * s, Weight: 700}, 1)
			if !(a.dialog == "launcherUpdate" && a.launcherUpdateRequired()) && button(c, "close-dialog", t.DialogCloseBtn, "cross", false, false) {
				a.requestCloseDialog()
			}
		})
		ui.Box(c).FillWidth().Height(1).Background(line)
		a.dialogFeedback(c)
		if a.dialog == "logs" {
			ui.Column(c.Key("dialog-log-pane")).FillWidth().Grow(1).Basis(0).MinHeight(0).Gap(16 * s).Clip().Children(func() { a.logsDialog(c) })
			return
		}
		if a.dialog == "keyBindings" {
			a.keyBindingsDialog(c)
			ui.Column(c).FillWidth().Shrink(0).Gap(12 * s).Children(func() { a.keyBindingsFooter(c) })
			return
		}
		content := scrollArea(c, "dialog-content-"+a.dialog).Label("弹窗内容").FillWidth().Margin(0, -scrollGutter*s, 0, 0).Padding(0, scrollGutter*s, 0, 0).MinHeight(0).MaxHeight(max(140, min(580*s, height-42-175*s))).Gap(16 * s)
		if kind != surfaceDialog {
			content.Grow(1).MinHeight(0).MaxHeight(height)
		}
		content.Children(func() {
			switch a.dialog {
			case "launcherUpdate":
				a.launcherUpdateDialog(c)
			case "java":
				a.javaDialog(c)
			case "add":
				a.addDialog(c)
			case "root":
				a.rootDialog(c)
			case "version":
				a.versionDialog(c)
			case "removeRoot":
				a.removeRootDialog(c)
			case "removeMod":
				a.removeModDialog(c)
			case "disableMod":
				a.disableModDialog(c)
			case "account":
				a.accountDialog(c)
			case "accountAdd":
				a.accountAddDialog(c)
			case "import":
				a.importDialog(c)
			case "export":
				a.packExportDialog(c)
			case "install":
				a.installDialog(c)
			case "packReport":
				a.packDialog(c)
			case "packUpdate":
				a.packUpdateDialog(c)
			case "packDrop":
				a.packDropDialog(c)
			case "javaGet":
				a.javaGetDialog(c)
			case "launchAgain":
				a.launchAgainDialog(c)
			}
		})
		if a.dialog == "packUpdate" {
			a.packUpdateFooter(c)
		}
		if a.dialog == "export" {
			a.packExportFooter(c)
		}
		if a.dialog == "version" && a.root() != nil {
			r := a.root()
			ui.Box(c).FillWidth().Height(1).Background(line)
			ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
				if button(c, "save-version-settings", t.RootCfgSaveBtn, "check", true, a.dialogPending()) {
					a.saveVersionConfig(r.ID)
				}
			})
		}
		if a.dialog == "root" && a.root() != nil {
			r := a.root()
			ui.Box(c).FillWidth().Height(1).Background(line)
			ui.Row(c).FillWidth().Wrap().Gap(12 * s).AlignItems(ui.Center).Children(func() {
				if button(c, "save-root-settings", t.RootCfgSaveBtn, "check", true, a.dialogPending()) {
					a.saveRootConfig(r.ID)
				}
				ui.Spacer(c).Grow(1)
				if r.Running && button(c, "stop-game", t.HomeStopBtn, "cross", false, a.dialogPending()) {
					a.requestStop(r.ID, "", r.Name)
				}
				if button(c, "unregister-root", t.RootCfgUnregisterBtn, "cross", false, a.dialogPending()) {
					id := r.ID
					if !a.guardDiscard(a.dialogDirty(), func() { a.closeDialog(); a.removeRootUndo(id) }) {
						a.closeDialog()
						a.removeRootUndo(id)
					}
				}
			})
		}
		if a.dialogPending() && !(a.dialog == "accountAdd" && normalizeAccountTab(a.accountTab) == "third" && a.pendingOp("thirdLogin")) {
			label(c, t.DialogBusyHint)
		}
	})
}

func dialogSurface(e ui.Element, s float32) ui.Element {
	return e.Radius(radiusDialog*s).Background(panel).Border(1, glassEdge).
		Shadow(0, 12*s, 28*s, -8*s, ui.RGBA(24, 32, 64, .18))
}

func (a *App) addDialog(c *ui.Context) {
	t := a.tr()
	label(c, t.AddRootDesc)
	a.folderField(c, t.AddRootFolderLabel, &a.form.Path)
	field(c, t.AddRootNameLabel, &a.form.Name, t.AddRootNamePlaceholder)
	if buttonWhy(c, "save-root", t.AddRootConfirmBtn, "plus", true, whyOf(a.dialogPending(), t.WhyBusy, strings.TrimSpace(a.form.Path) == "", t.WhyNoFolder), true) {
		a.execute("addRoot", map[string]string{"path": a.form.Path, "name": a.form.Name}, a.finished(t.AddRootSuccess))
	}
}

func (a *App) removeRootDialog(c *ui.Context) {
	t := a.tr()
	r := a.root()
	if r == nil {
		return
	}
	ui.Text(c, fmt.Sprintf(t.RemRootConfirmTitle, r.Name)).FontWeight(600).TextColor(ink)
	label(c, t.RemRootSafeHint)
	if button(c, "confirm-remove-root", t.RemRootConfirmBtn, "cross", true, a.dialogPending()) {
		a.execute("removeRoot", map[string]string{"rootId": r.ID}, a.finished(t.RemRootSuccess))
	}
}

func (a *App) removeModDialog(c *ui.Context) {
	t := a.tr()
	ui.Text(c, fmt.Sprintf(t.RemModConfirmTitle, a.form.Name)).FontWeight(600).TextColor(ink)
	label(c, t.RemModBackupHint)
	if button(c, "confirm-remove-mod", t.RemModConfirmBtn, "cross", true, a.dialogPending() || a.root() == nil) {
		a.execute("removeMod", map[string]string{"rootId": a.root().ID, "name": a.form.Name}, func(json.RawMessage) { a.closeDialog(); a.loadMods() })
	}
}
