package nativeui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

var lastDialogPrimary string

// dialogView shows the active dialog. Opening eases it in (fade, slight
// rise and scale-up); closing eases it out faster while the last dialog is
// still drawn, so neither direction pops. host keeps the animated value.
func (a *App) dialogView(c *ui.Context, host ui.Element) {
	target, anim := float32(0), motionExit
	if a.dialogOpen {
		target, anim = 1, motionEnter
		a.dialogShown = a.dialog
	}
	presence := host.AnimateTo("dialog-presence", target, anim)
	if !a.dialogOpen {
		if presence <= 0.01 || a.dialogShown == "" {
			a.dialogShown = ""
			return
		}
		// draw the dialog that is leaving, without reopening it
		a.dialog = a.dialogShown
		defer func() { a.dialog = "" }()
	}
	if a.device != nil && a.dialog == "account" {
		a.pollDevice()
	}
	t := a.tr()
	a.normalizeStateValues()
	s := uiScale(c)
	_, height := c.Size()
	open := a.dialogOpen
	closing := !open
	if closing {
		open = true
	}
	inDialog, dialogPrimaryKey = true, ""
	defer func() { inDialog, lastDialogPrimary = false, dialogPrimaryKey }()
	ui.DialogBase(c, &open, func(backdrop, box ui.Element) {
		// Enter activates the dialog's primary action (the first enabled
		// primary button), unless a control inside takes Enter itself.
		// The panel takes the focus as it opens (no ring: not from the
		// keyboard), so Enter and Escape reach it before any field is used.
		box.Focusable().AutoFocus().FocusRing(false)
		if box.Shortcut(0, ui.KeyEnter) && lastDialogPrimary != "" {
			enterFired = lastDialogPrimary
		}
		backdrop.Background(ui.RGBA(20, 24, 48, .30*presence))
		box.Opacity(presence).TransformOrigin(.5, .5).Scale(.96+.04*presence, .96+.04*presence).Translate(0, (1-presence)*10*s)
		width := float32(680)
		switch a.dialog {
		case "logs":
			width = 1100
		case "packReport", "java", "root", "install", "project", "packUpdate":
			width = 780
		}
		if a.dialog == "project" {
			backdrop.Background(ui.RGBA(16, 20, 40, .42*presence))
			popover(box.Label("活动弹窗").Width(max(560, 820*s)).MaxWidthPercent(92).MaxHeight(height-42-24*s).Padding(0), 20*s, s).Background(panel).Clip()
			a.projectInstallDialog(c, height-42-24*s)
			return
		}
		popover(box.Label("活动弹窗").Width(max(480, width*s)).MaxWidthPercent(94).MaxHeight(height-42-24*s).Padding(24*s), 22*s, s)
		body := ui.Column(c).FillWidth().Gap(16 * s)
		if a.dialog == "logs" {
			box.Height(min(820*s, height-42-24*s))
			body.FillHeight().MinHeight(0)
		}
		body.Children(func() {
			ui.Row(c).FillWidth().Shrink(0).Gap(12 * s).AlignItems(ui.Center).Children(func() {
				if a.dialog == "root" || a.dialog == "version" {
					tone := navTones["settings"]
					appleSoftIconBox(c, 30*s, "settings", tone[0], tone[1], panel)
				}
				ui.Text(c, a.dialogTitle(a.dialog)).FontSize(typeTitle2 * s).Bold().TextColor(ink).Grow(1).MinWidth(0).Shrink(1).SingleLine()
				if button(c, "close-dialog", t.DialogCloseBtn, "cross", false, false) {
					a.closeDialog()
				}
			})
			ui.Box(c).FillWidth().Height(1).Background(line)
			if a.message != "" && a.errorMessage {
				a.inlineNotice(c, "error", a.message, "")
			}
			if a.dialog == "logs" {
				ui.Column(c.Key("dialog-log-pane")).FillWidth().Grow(1).Basis(0).MinHeight(0).Gap(16 * s).Clip().Children(func() { a.logsDialog(c) })
				return
			}
			scrollArea(c, "dialog-content-"+a.dialog).Label("弹窗内容").FillWidth().Margin(0, -scrollGutter*s, 0, 0).Padding(0, scrollGutter*s, 0, 0).MinHeight(0).MaxHeight(max(140, min(580*s, height-42-175*s))).Gap(16 * s).Children(func() {
				switch a.dialog {
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
				case "account":
					a.accountDialog(c)
				case "import":
					a.importDialog(c)
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
			if a.dialog == "version" && a.root() != nil {
				r := a.root()
				ui.Box(c).FillWidth().Height(1).Background(line)
				ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
					if button(c, "save-version-settings", t.RootCfgSaveBtn, "check", true, a.dialogPending()) {
						a.saveVersionConfig(r.ID)
					}
				})
			}
			if a.dialog == "root" && a.root() != nil {
				r := a.root()
				ui.Box(c).FillWidth().Height(1).Background(line)
				ui.Row(c).FillWidth().Wrap().Gap(10 * s).AlignItems(ui.Center).Children(func() {
					if button(c, "save-root-settings", t.RootCfgSaveBtn, "check", true, a.dialogPending()) {
						a.saveRootConfig(r.ID)
					}
					ui.Spacer(c).Grow(1)
					if r.Running && button(c, "stop-game", t.HomeStopBtn, "cross", false, a.dialogPending()) {
						a.execute("stop", map[string]string{"rootId": r.ID}, a.finished(t.HomeStopBtn))
					}
					if button(c, "unregister-root", t.RootCfgUnregisterBtn, "cross", false, a.dialogPending()) {
						a.dialog = "removeRoot"
					}
				})
			}
			if a.dialogPending() {
				label(c, t.DialogBusyHint)
			}
		})
	})
	if !closing && !open {
		a.dialogOpen = false // backdrop click or Escape
	}
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
