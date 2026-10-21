package nativeui

import (
	"encoding/json"
	"fmt"
	neturl "net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"mcl/internal/launcher"
)

const g1gcPreset = "-XX:+UseG1GC -XX:+ParallelRefProcEnabled -XX:MaxGCPauseMillis=50 -XX:+UnlockExperimentalVMOptions -XX:+DisableExplicitGC -XX:G1NewSizePercent=20 -XX:G1ReservePercent=20 -XX:G1HeapRegionSize=32M"

func openPath(path string) error { return mygo.Shell.OpenPath(path) }

func field(c *ui.Context, name string, value *string, placeholder string) {
	s := uiScale(c)
	ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
		label(c, name).FontWeight(600)
		ui.TextInput(c.Key(name), value).Label(name + "输入").Placeholder(placeholder).FillWidth()
	})
}

func (a *App) folderField(c *ui.Context, name string, value *string) {
	s := uiScale(c)
	t := a.tr()
	ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
		label(c, name).FontWeight(600)
		ui.Row(c).FillWidth().Gap(10 * s).Children(func() {
			ui.TextInput(c.Key(name), value).Label(name + "输入").Placeholder(t.DialogChooseFolder).Grow(1).MinWidth(0)
			if button(c, "folder-"+name, t.DialogBrowseBtn, "folder", false, a.busy) {
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

// dialogView shows the active dialog. Opening eases it in (fade, slight
// rise and scale-up); closing eases it out faster while the last dialog is
// still drawn, so neither direction pops. host keeps the animated value.
var lastDialogPrimary string

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
		case "logs", "packReport", "java", "root", "install", "project", "packUpdate":
			width = 780
		}
		popover(box.Label("活动弹窗").Width(max(480, width*s)).MaxWidthPercent(94).MaxHeight(height-42-24*s).Padding(24*s), 26*s, s)
		ui.Column(c).FillWidth().Gap(16 * s).Children(func() {
			ui.Row(c).FillWidth().Gap(14 * s).AlignItems(ui.Center).Children(func() {
				if a.dialog == "root" || a.dialog == "version" {
					tone := navTones["settings"]
					appleSoftIconBox(c, 32*s, "settings", tone[0], tone[1], panel)
				}
				ui.Text(c, a.dialogTitle(a.dialog)).FontSize(22 * s).Bold().TextColor(ink).Grow(1).MinWidth(0).Shrink(1).SingleLine()
				if button(c, "close-dialog", t.DialogCloseBtn, "cross", false, false) {
					a.closeDialog()
				}
			})
			ui.Box(c).FillWidth().Height(1).Background(line)
			if a.message != "" && a.errorMessage {
				a.inlineNotice(c, "error", a.message, "")
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
				case "project":
					a.projectDialog(c)
				case "packReport":
					a.packDialog(c)
				case "packUpdate":
					a.packUpdateDialog(c)
				case "packDrop":
					a.packDropDialog(c)
				case "logs":
					a.logsDialog(c)
				}
			})
			if a.dialog == "packUpdate" {
				a.packUpdateFooter(c)
			}
			if a.dialog == "version" && a.root() != nil {
				r := a.root()
				ui.Box(c).FillWidth().Height(1).Background(line)
				ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
					if button(c, "save-version-settings", t.RootCfgSaveBtn, "check", true, a.busy) {
						a.saveVersionConfig(r.ID)
					}
				})
			}
			if a.dialog == "root" && a.root() != nil {
				r := a.root()
				ui.Box(c).FillWidth().Height(1).Background(line)
				ui.Row(c).FillWidth().Wrap().Gap(10 * s).AlignItems(ui.Center).Children(func() {
					if button(c, "save-root-settings", t.RootCfgSaveBtn, "check", true, a.busy) {
						a.saveRootConfig(r.ID)
					}
					ui.Spacer(c).Grow(1)
					if r.Running && button(c, "stop-game", t.HomeStopBtn, "cross", false, a.busy) {
						a.execute("stop", map[string]string{"rootId": r.ID}, a.finished(t.HomeStopBtn))
					}
					if button(c, "unregister-root", t.RootCfgUnregisterBtn, "cross", false, a.busy) {
						a.dialog = "removeRoot"
					}
				})
			}
			if a.busy {
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
	if buttonWhy(c, "save-root", t.AddRootConfirmBtn, "plus", true, whyOf(a.busy, t.WhyBusy, strings.TrimSpace(a.form.Path) == "", t.WhyNoFolder), true) {
		a.execute("addRoot", map[string]string{"path": a.form.Path, "name": a.form.Name}, a.finished(t.AddRootSuccess))
	}
}

func (a *App) versionDialog(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	r := a.root()
	if r == nil {
		return
	}

	// Section 1: Per-Version Dedicated Java, Memory & JVM Arguments
	if len(r.Versions) > 0 {
		ui.Column(c).FillWidth().Gap(10 * s).Children(func() {
			if a.form.ConfigVersion == "" {
				a.loadVersionConfigForm(r.SelectedVersion)
			}
			targetVer := a.form.ConfigVersion
			for _, ver := range r.Versions {
				if ver.ID != targetVer {
					continue
				}
				vv := ver
				ui.Row(c).Label("版本设置标题").FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
					if !loaderLogo(c, vv.Loader, 36*s) {
						appleSoftIconBox(c, 36*s, "gamepad", ui.Hex("#eaf3ff"), ui.Hex("#dbeafe"), accent)
					}
					ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(2 * s).Children(func() {
						title, sub := a.cardText(*r, vv)
						titleWithTags(c, title, vv.ID, 17*s, 700, a.versionTags(*r, vv, false), 3)
						label(c, joinDistinct(sub, r.Name)).FillWidth().MinWidth(0).FontSize(12.5 * s).SingleLine()
					})
				})
				ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
					ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
						label(c, t.VersionNameLabel).FontWeight(600).NoWrap().Shrink(0)
						label(c, t.VersionNameHint).FontSize(12 * s).SingleLine().MinWidth(0)
					})
					def := vv
					def.DisplayName = ""
					ui.TextInput(c.Key("ver-display-name"), &a.form.VerName).Label(t.VersionNameLabel).Placeholder(a.nameOf(*r, def).Title).FillWidth().Height(40 * s)
				})
			}

			// Per-Version Java Runtime Selector (with detected Java runtimes + Browse)
			ui.Row(c).Gap(8 * s).AlignItems(ui.Center).Children(func() {
				label(c, t.RootCfgVerJavaLabel).FontWeight(600)
				if r := a.root(); r != nil {
					for _, v := range r.Versions {
						if v.ID == targetVer {
							javaTag(c, javaFor(*r, v))
						}
					}
				}
			})
			ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
				javaNames := []string{t.RootCfgVerJavaFollow}
				javaPaths := map[string]string{t.RootCfgVerJavaFollow: ""}
				selectedJava := t.RootCfgVerJavaFollow
				for _, j := range a.java {
					name := fmt.Sprintf("Java %s · %s (%s)", j.Version, javaDisplay(j.Vendor), javaDisplay(filepath.Base(filepath.Dir(filepath.Dir(j.Path)))))
					if _, exists := javaPaths[name]; exists {
						name += " · " + j.Path
					}
					javaNames = append(javaNames, name)
					javaPaths[name] = j.Path
					if j.Path == a.form.VerJava {
						selectedJava = name
					}
				}
				if a.form.VerJava != "" && selectedJava == t.RootCfgVerJavaFollow {
					selectedJava = a.form.VerJava
					javaNames = append(javaNames, selectedJava)
					javaPaths[selectedJava] = a.form.VerJava
				}
				if selectBox(c, "ver-java-select", t.RootCfgVerJavaLabel, &selectedJava, javaNames).Grow(1).MinWidth(0).Changed() {
					a.form.VerJava = javaPaths[selectedJava]
				}
				if button(c, "ver-java-browse", t.DialogBrowseBtn, "folder", false, a.busy) {
					a.chooseFiles("java", func(paths []string) { a.form.VerJava = paths[0] })
				}
			})
			if a.form.VerJava != "" {
				label(c, a.form.VerJava).FontSize(12.5 * s).Ellipsis("…")
			}

			// Per-Version Max Memory: 自动 / 手动
			a.memoryModeRow(c, "ver-mem-mode", t.RootCfgVerMemLabel, &a.form.VerMemAuto, a.autoMemoryFor(a.form.ConfigVersion))
			if !a.form.VerMemAuto {
				ui.Row(c).FillWidth().Wrap().Gap(8 * s).AlignItems(ui.Center).Children(func() {
					for _, preset := range []struct {
						label string
						mb    string
					}{
						{"2 GB", "2048"},
						{"4 GB", "4096"},
						{"6 GB", "6144"},
						{"8 GB", "8192"},
						{"12 GB", "12288"},
						{"16 GB", "16384"},
					} {
						p := preset
						active := strings.TrimSpace(a.form.VerMemory) == p.mb
						chip := ui.ButtonBase(c.Key("mem-preset-"+p.mb)).Label(p.label).Height(28*s).PaddingX(10*s).Radius(6*s).Border(1, line).AlignItems(ui.Center)
						if active {
							chip.Background(chip.AnimateColor("bg", accent, motionFast)).TextColor(panel)
						} else {
							chip.Background(chip.AnimateColor("bg", fieldColor, motionFast)).TextColor(inkSecondary)
						}
						if chip.Children(func() {
							ui.Text(c, p.label).FontSize(12.5 * s).NoWrap()
						}).Clicked() {
							a.form.VerMemory = p.mb
						}
					}
				})
				ui.TextInput(c.Key("ver-memory-input"), &a.form.VerMemory).Label(t.RootCfgVerMemLabel + "输入").Placeholder("4096").FillWidth()
			}

			// Per-Version Custom JVM Arguments & Recommended Presets
			label(c, t.RootCfgVerJVMLabel).FontWeight(600)
			ui.TextInput(c.Key("ver-jvm-input"), &a.form.VerJVM).Label(t.RootCfgVerJVMLabel + "输入").Placeholder(t.RootCfgVerJVMHint).FillWidth()
			renderJVMPresetBar(c, "ver-jvm-preset", &a.form.VerJVM, t, nil)
		})
	}
	a.packSection(c, *r)
}

func (a *App) rootDialog(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	r := a.root()
	if r == nil {
		return
	}
	// Section 2: Instance Directory & Default Settings
	ui.Column(c).FillWidth().Gap(10*s).Padding(16*s).Radius(12*s).Background(raised).Border(1, line).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10 * s).Children(func() {
			ui.Icon(c, icons["folder"]).Size(18*s, 18*s).TextColor(accent).Shrink(0)
			pathLabel(c, r.Path, 13*s)
			if button(c, "root-folder", t.HomeOpenFolderBtn, "folder", false, false) {
				a.openFolder(r.ID)
			}
			if button(c, "root-logs", t.HomeLogsBtn, "terminal", false, false) {
				a.openDialog("logs")
			}
		})
		field(c, t.AddRootNameLabel, &a.form.Name, "")
		field(c, t.RootCfgJavaLabel, &a.form.Java, t.RootCfgJavaHint)
		a.memoryModeRow(c, "root-mem-mode", t.RootCfgMemLabel, &a.form.MemAuto, a.autoMemoryFor(r.SelectedVersion))
		if !a.form.MemAuto {
			ui.TextInput(c.Key("root-memory-input"), &a.form.Memory).Label(t.RootCfgMemLabel + "输入").Placeholder("4096").FillWidth()
		}
	})
}

// saveRootConfig saves only the directory's name and default Java/memory.
func (a *App) saveRootConfig(rootID string) {
	t := a.tr()
	memStr := strings.TrimSpace(a.form.Memory)
	if memStr == "" {
		memStr = "0"
	}
	n, e := strconv.Atoi(memStr)
	if a.form.MemAuto {
		n, e = -1, nil // explicit directory auto
	}
	if e != nil || (n > 0 && (n < 512 || n > 65536)) || n == 0 {
		a.message = t.RootCfgMemErr
		a.errorMessage = true
		return
	}
	a.execute("updateRoot", map[string]any{"rootId": rootID, "name": a.form.Name, "javaPath": a.form.Java, "memoryMB": n, "jvmArgs": a.form.JVM}, a.finished(t.RootCfgSaved))
}

// saveVersionConfig saves only the per-version Java/memory/JVM overrides.
func (a *App) saveVersionConfig(rootID string) {
	t := a.tr()
	verMemStr := strings.TrimSpace(a.form.VerMemory)
	if verMemStr == "" {
		verMemStr = "0"
	}
	vn, ve := strconv.Atoi(verMemStr)
	if a.form.VerMemAuto {
		vn, ve = 0, nil
		if r := a.root(); r != nil {
			probe := *r
			probe.VersionConfigs = nil
			if inherited, _ := probe.MemoryPlan(a.form.ConfigVersion, a.settings); !inherited {
				vn = -1 // directory/global is manual: pin auto on this version
			}
		}
	}
	if ve != nil || (vn > 0 && (vn < 512 || vn > 65536)) || (!a.form.VerMemAuto && vn == 0) {
		a.message = t.RootCfgMemErr
		a.errorMessage = true
		return
	}
	ver, name := strings.TrimSpace(a.form.ConfigVersion), strings.TrimSpace(a.form.VerName)
	a.execute("updateVersion", map[string]any{"rootId": rootID, "version": ver, "javaPath": strings.TrimSpace(a.form.VerJava), "memoryMB": vn, "jvmArgs": strings.TrimSpace(a.form.VerJVM)}, func(json.RawMessage) {
		a.execute("setVersionName", map[string]any{"rootId": rootID, "version": ver, "name": name}, a.finished(t.RootCfgSaved))
	})
}

func (a *App) removeRootDialog(c *ui.Context) {
	t := a.tr()
	r := a.root()
	if r == nil {
		return
	}
	ui.Text(c, fmt.Sprintf(t.RemRootConfirmTitle, r.Name)).FontWeight(600).TextColor(ink)
	label(c, t.RemRootSafeHint)
	if button(c, "confirm-remove-root", t.RemRootConfirmBtn, "cross", true, a.busy) {
		a.execute("removeRoot", map[string]string{"rootId": r.ID}, a.finished(t.RemRootSuccess))
	}
}

func (a *App) removeModDialog(c *ui.Context) {
	t := a.tr()
	ui.Text(c, fmt.Sprintf(t.RemModConfirmTitle, a.form.Name)).FontWeight(600).TextColor(ink)
	label(c, t.RemModBackupHint)
	if button(c, "confirm-remove-mod", t.RemModConfirmBtn, "cross", true, a.busy || a.root() == nil) {
		a.execute("removeMod", map[string]string{"rootId": a.root().ID, "name": a.form.Name}, func(json.RawMessage) { a.closeDialog(); a.loadMods() })
	}
}

func (a *App) accountDialog(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	// With saved accounts the list is the focus; adding one is a separate
	// step behind "+ 添加账号". With none, go straight to the add form.
	if len(a.snapshot.Accounts) > 0 && !a.accountAdding {
		a.accountList(c)
		ui.Row(c).FillWidth().Gap(space2 * s).AlignItems(ui.Center).Children(func() {
			if button(c, "account-add-open", t.AccAddAccountBtn, "plus", true, a.busy) {
				a.accountAdding = true
			}
		})
		return
	}
	if len(a.snapshot.Accounts) > 0 {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(space2 * s).Children(func() {
			if button(c, "account-add-back", t.AccBackBtn, "", false, a.busy) {
				a.accountAdding = false
				a.serverEdit = nil
			}
			label(c, t.AccAddSection).FontSize(13.5 * s).FontWeight(700)
		})
	} else {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			label(c, t.AccAddSection).FontSize(13.5 * s).FontWeight(700).Grow(1)
		})
	}
	a.accountAddForm(c)
}

// accountServerName returns the saved display name of a Yggdrasil server.
func (a *App) accountServerName(url string) string {
	if b, ok := launcher.BuiltinAuthServer(url); ok {
		return b.Name
	}
	for _, v := range a.snapshot.AuthServers {
		if launcher.SameAuthServer(v.URL, url) {
			return v.Name
		}
	}
	if u, e := neturl.Parse(url); e == nil && u.Host != "" {
		return u.Host
	}
	return url
}

// accountKindTag is the short type label: 正版 / 离线 / 第三方·<server>.
func (a *App) accountKindTag(ac launcher.Account) string {
	t := a.tr()
	switch ac.Kind {
	case "microsoft":
		return t.AccTagMicrosoft
	case "yggdrasil":
		return t.AccTagThird + " · " + a.accountServerName(ac.Server)
	}
	return t.AccTagOffline
}

// Account type colours: 正版 green, 离线 slate, 第三方 indigo.
var (
	accountTagMS      = ui.Hex("#1f9d55")
	accountTagOffline = ui.Hex("#5b6474")
	accountTagThird   = ui.Hex("#5b4fd6")
)

func accountTypeColor(kind string) ui.Color {
	switch kind {
	case "microsoft":
		return accountTagMS
	case "yggdrasil":
		return accountTagThird
	}
	return accountTagOffline
}

// accountTypeIcon draws the leading icon for an account type: Microsoft
// logo, person (offline) or globe (Yggdrasil server).
func accountTypeIcon(c *ui.Context, kind string, size float32, tint ui.Color) {
	accountTypeIconL(c, nil, "类型图标 ", kind, "", size, tint)
}

// accountTypeIconL draws the type icon; for Yggdrasil accounts with a known
// server it shows the site's favicon when available (globe otherwise).
func accountTypeIconL(c *ui.Context, a *App, prefix, kind, server string, size float32, tint ui.Color) {
	if kind == "yggdrasil" && server != "" && a != nil {
		if bm := a.authIcon(server); bm != nil {
			ui.Image(c, bm).Label(prefix+"favicon").Size(size, size).Radius(size * .2).Fit(ui.Contain).Shrink(0)
			return
		}
	}
	switch kind {
	case "microsoft":
		ui.Image(c, microsoftLogo).Label(prefix+"microsoft").Size(size, size).Fit(ui.Contain).Shrink(0)
	case "yggdrasil":
		ui.Icon(c, icons["globe"]).Label(prefix+"yggdrasil").Size(size, size).TextColor(tint).Shrink(0)
	default:
		ui.Icon(c, icons["account"]).Label(prefix+"offline").Size(size, size).TextColor(tint).Shrink(0)
	}
}

// accountTypeTag is the coloured pill (icon + 正版/离线/第三方 · server).
func (a *App) accountTypeTag(c *ui.Context, ac launcher.Account) {
	s := uiScale(c)
	col := accountTypeColor(ac.Kind)
	kind := ac.Kind
	if kind != "microsoft" && kind != "yggdrasil" {
		kind = "offline"
	}
	ui.Row(c).Label("类型标签 "+kind).Shrink(1).MinWidth(0).Padding(2*s, 8*s, 2*s, 6*s).Gap(4*s).Radius(999).Background(col.Alpha(.12)).Border(1, col.Alpha(.22)).AlignItems(ui.Center).Children(func() {
		accountTypeIconL(c, a, "类型图标 ", kind, ac.Server, 12*s, col)
		ui.Text(c, a.accountKindTag(ac)).FontSize(11.5 * s).FontWeight(600).TextColor(col).Shrink(1).MinWidth(0).SingleLine().Tooltip(ac.Server)
	})
}

// accountList renders saved accounts as compact rows in a scroll area.
func (a *App) accountList(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
		label(c, t.AccSavedSection).FontSize(13.5 * s).FontWeight(700).Grow(1)
		label(c, fmt.Sprint(len(a.snapshot.Accounts))).FontSize(13 * s)
	})
	scrollArea(c, "account-list-scroll").Label("账号列表").FillWidth().Margin(0, -scrollGutter*s, 0, 0).Padding(0, scrollGutter*s, 0, 0).MaxHeight(360 * s).Children(func() {
		ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
			for _, account := range a.snapshot.Accounts {
				ac := account
				active := ac.ID == a.snapshot.ActiveAccount
				bg, edge := ui.RGBA(255, 255, 255, .7), line
				if active {
					bg, edge = accentSoft, accent.Alpha(.30)
				}
				ui.Row(c.Key("ac-"+ac.ID)).Label("账号行 "+ac.ID).FillWidth().Height(60*s).Padding(0, space3*s).Radius(radiusRow*s+2*s).Background(bg).Border(1, edge).Gap(space3 * s).AlignItems(ui.Center).Children(func() {
					a.headAvatar(c, &ac, 34*s, func() {
						ui.Box(c).Size(34*s, 34*s).Radius(8*s).Background(panel).Border(1, line).Center().Shrink(0).Children(func() {
							ui.Icon(c, icons["account"]).Size(17*s, 17*s).TextColor(accent)
						})
					})
					ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4 * s).Children(func() {
						fg := ink
						if active {
							fg = accent
						}
						ui.Text(c, ac.Name).FillWidth().MinWidth(0).FontSize(15 * s).FontWeight(700).TextColor(fg).SingleLine().Tooltip(ac.Name)
						ui.Row(c).FillWidth().MinWidth(0).Gap(6 * s).AlignItems(ui.Center).Children(func() {
							a.accountTypeTag(c, ac)
							if active {
								ui.Text(c, t.HomeBadgeActive).FontSize(11.5 * s).FontWeight(700).TextColor(accent).NoWrap().Shrink(0)
							}
						})
					})
					if !active && fixedButton(c, "use-account-"+ac.ID, t.AccSetCurrentBtn, "", 0, 32*s, false, a.busy) {
						a.execute("select", map[string]string{"accountId": ac.ID}, func(json.RawMessage) {
							if a.page == "wardrobe" {
								a.loadAppearance()
							}
						})
					}
					ui.Box(c).Width(space2 * s).Shrink(0)
					if a.confirmRemove != ac.ID {
						if iconOnlyButton(c, "delete-account-"+ac.ID, "cross", t.AccRemoveBtn, 32*s, a.busy) {
							a.confirmRemove = ac.ID // second click removes; anything else cancels
						}
					} else if dangerButton(c, "confirm-delete-account-"+ac.ID, t.AccRemoveConfirm, 32*s, a.busy) {
						a.confirmRemove = ""
						a.execute("removeAccount", map[string]string{"accountId": ac.ID}, func(json.RawMessage) {
							a.invalidateHead(ac.ID)
							if a.page == "wardrobe" {
								a.loadAppearance()
							}
						})
					}
				})
			}
		})
	})
}

func (a *App) accountAddForm(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	// Segmented mode switch with a sliding white thumb: the thumb position
	// eases toward the active index over ~220 ms instead of jumping.
	tabs := []struct{ label, icon string }{
		{t.AccTabMicrosoft, "microsoft"},
		{t.AccTabOffline, "offline"},
		{t.AccTabThirdParty, "yggdrasil"},
	}
	activeIndex := 0
	for i, tab := range tabs {
		if a.accountTab == tab.label {
			activeIndex = i
		}
	}
	const segPad, segGap = 4, 4
	segRow := ui.Row(c.Key("auth-segment")).FillWidth().Padding(segPad * s).Gap(segGap * s).Radius(13 * s).Background(ui.Hex("#eaeff7"))
	thumb := segRow.AnimateTo("thumb", float32(activeIndex), accountTabAnim)
	segRow.Draw(func(p *ui.Painter, r ui.Rect) {
		n := float32(len(tabs))
		w := (r.W - 2*segPad*s - (n-1)*segGap*s) / n
		box := ui.Rect{X: r.X + segPad*s + thumb*(w+segGap*s), Y: r.Y + segPad*s, W: w, H: r.H - 2*segPad*s}
		p.Shadow(box, 10*s, 0, 2*s, 6*s, 0, ui.RGBA(15, 23, 42, .10))
		p.Fill(box, panel, 10*s)
	}).Children(func() {
		for i, item := range tabs {
			tab := item
			active := i == activeIndex
			// Label color crossfades with the thumb's distance from this tab.
			near := 1 - min(1, abs32(thumb-float32(i)))
			fg := mixColor(inkSecondary, accent, near)
			b := ui.ButtonBase(c.Key("auth-" + tab.label)).Label(tab.label).Grow(1).Basis(0).MinWidth(0).Height(38 * s).Radius(10 * s).AlignItems(ui.Center).Justify(ui.Center).Gap(8 * s).Disabled(a.busy)
			hover := ui.Transparent
			if !active && b.Hovered() {
				hover = ui.RGBA(255, 255, 255, .45)
			}
			b.Background(b.AnimateColor("bg", hover, fastAnim))
			if b.Children(func() {
				accountTypeIcon(c, tab.icon, 15*s, mixColor(fg, accountTypeColor(tab.icon), near))
				ui.Text(c, tab.label).FontSize(15 * s).FontWeight(600).TextColor(fg).NoWrap()
			}).Clicked() && !active {
				if a.device != nil {
					a.service.CancelMicrosoft(a.device.ID)
					a.device = nil
				}
				a.accountTabDir = 1
				if i < activeIndex {
					a.accountTabDir = -1
				}
				a.accountTab = tab.label
				a.profiles = nil
				a.form.Password = ""
			}
		}
	})

	// Framed Form Card for the selected authentication method
	mode := normalizeAccountTab(a.accountTab)
	ui.Column(c.Key("auth-form-"+mode)).FillWidth().Gap(14*s).Padding(18*s).Radius(12*s).Background(raised).Border(1, line).Transition(slideTransition(s, max(a.accountTabDir, -1), false)).Children(func() {
		switch mode {
		case "offline":
			label(c, t.AccOfflineDesc)
			field(c, t.AccOfflineNameLabel, &a.form.Username, t.AccOfflineNameHint)
			ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
				if buttonWhy(c, "offline-login", t.AccOfflineAddBtn, "account", true, whyOf(a.busy, t.WhyBusy, strings.TrimSpace(a.form.Username) == "", t.WhyNeedName), true) {
					a.execute("offlineLogin", map[string]string{"name": a.form.Username}, func(json.RawMessage) {
						a.closeDialog()
						a.notice(a.tr().AccOfflineAdded)
						if a.page == "wardrobe" {
							a.loadAppearance()
						}
					})
				}
			})
		case "microsoft":
			ui.Row(c).Gap(12 * s).AlignItems(ui.Center).Children(func() {
				ui.Image(c, microsoftLogo).Size(24*s, 24*s).Shrink(0).Fit(ui.Contain)
				label(c, t.AccMSDesc).Grow(1)
			})
			if !a.snapshot.Services.Microsoft {
				ui.Column(c).FillWidth().Gap(4*s).Padding(12*s).Radius(8*s).Background(panel).Border(1, line).Children(func() {
					ui.Text(c, t.AccMSDisabled1).FontSize(14.5 * s).FontWeight(600).TextColor(inkSecondary)
					label(c, t.AccMSDisabled2).FontSize(13.5 * s)
				})
			} else if a.device == nil {
				ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
					if button(c, "start-microsoft", t.AccMSStartBtn, "account", true, a.busy) {
						a.execute("startMicrosoft", nil, func(b json.RawMessage) {
							v := decode[launcher.DeviceInfo](b)
							a.device = &v
							a.deviceUntil = time.Now().Add(time.Duration(v.ExpiresIn) * time.Second)
							a.nextDevicePoll = time.Now().Add(time.Duration(max(5, v.Interval)) * time.Second)
						})
					}
				})
			} else {
				label(c, t.AccMSCodePrompt)
				ui.Row(c).FillWidth().Padding(14*s).Radius(10*s).Background(panel).Border(1, blue).Justify(ui.Center).Children(func() {
					ui.Text(c, a.device.UserCode).FontSize(30 * s).Bold().TextColor(accent).NoWrap()
				})
				ui.Row(c).Gap(12 * s).Children(func() {
					if button(c, "copy-device", t.AccMSCopyCodeBtn, "", false, false) {
						c.WriteClipboard(a.device.UserCode)
					}
					if button(c, "open-device", t.AccMSOpenBrowserBtn, "arrow", true, false) {
						c.OpenURL(a.device.VerificationURI)
					}
				})
				statusLine(c, t.AccMSWaiting, muted, true)
			}
		case "third":
			label(c, t.AccThirdDesc)
			a.authServerPicker(c)
			field(c, t.AccThirdUserLabel, &a.form.Username, t.AccThirdUserHint)
			ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
				label(c, t.AccThirdPassLabel).FontWeight(600)
				ui.TextInput(c, &a.form.Password).Label(t.AccThirdPassLabel).Placeholder(t.AccThirdPassHint).Password().FillWidth()
			})
			if len(a.profiles) > 0 {
				label(c, t.AccThirdProfileLabel).FontWeight(600)
				names := []string{}
				for _, p := range a.profiles {
					names = append(names, p.Name)
				}
				selectBox(c, "third-profile-select", t.AccThirdProfileLabel, &a.form.Profile, names).FillWidth()
			}
			ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
				if buttonWhy(c, "third-login", t.AccThirdLoginBtn, "account", true, whyOf(a.busy, t.WhyBusy, a.form.Server == "" || a.form.Username == "" || a.form.Password == "", t.WhyNeedLogin), true) {
					id := ""
					for _, p := range a.profiles {
						if p.Name == a.form.Profile {
							id = p.ID
						}
					}
					a.execute("thirdLogin", map[string]string{"server": a.form.Server, "username": a.form.Username, "password": a.form.Password, "profileId": id}, func(b json.RawMessage) {
						r := decode[launcher.ThirdLoginResult](b)
						if r.Account != nil {
							a.closeDialog()
							a.notice(a.tr().AccThirdAdded)
							if a.page == "wardrobe" {
								a.loadAppearance()
							}
						} else {
							a.profiles = r.Profiles
							if len(r.Profiles) > 0 {
								a.form.Profile = r.Profiles[0].Name
							}
						}
					})
				}
			})
		}
	})
}

func (a *App) pollDevice() {
	if a.device == nil || a.polling || a.busy || time.Now().Before(a.nextDevicePoll) {
		return
	}
	if time.Now().After(a.deviceUntil) {
		a.device = nil
		a.message = a.tr().AccMSExpired
		a.errorMessage = true
		return
	}
	id := a.device.ID
	interval := max(5, a.device.Interval)
	a.nextDevicePoll = time.Now().Add(time.Duration(interval) * time.Second)
	a.polling = true
	go func() {
		r, e := a.service.PollMicrosoft(a.ctx, id)
		a.post(func() {
			a.polling = false
			if a.device == nil || a.device.ID != id {
				return
			}
			if e != nil {
				a.message = e.Error()
				a.errorMessage = true
				a.device = nil
				return
			}
			if r.Account != nil {
				a.snapshot = a.service.Snapshot()
				a.closeDialog()
				a.notice(a.tr().AccMSAdded)
				if a.page == "wardrobe" {
					a.loadAppearance()
				}
			}
		})
	}()
}

func (a *App) importDialog(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	label(c, t.ImportDesc)
	ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
		label(c, t.ImportFileLabel).FontWeight(600)
		ui.Row(c).FillWidth().Gap(10 * s).Children(func() {
			input := ui.TextInput(c, &a.form.Path).Label(t.ImportFileLabel).Placeholder(t.ImportFilePlaceholder).Tooltip(t.ImportFileHint).Grow(1).MinWidth(0)
			if input.Changed() && strings.TrimSpace(a.form.Path) != "" {
				a.inspectPackFile(a.form.Path)
			}
			if button(c, "pick-pack", t.DialogBrowseBtn, "folder", false, a.busy) {
				a.chooseFiles("pack", func(paths []string) {
					a.inspectPackFile(paths[0])
				})
			}
		})
		label(c, t.ImportFileHint).FontSize(12 * s).FillWidth().MinWidth(0)
	})
	if p := a.packPreview; p != nil {
		ui.Row(c).FillWidth().Padding(14*s, 16*s).Radius(12*s).Background(accentSoft).Border(1, blue).Gap(14 * s).AlignItems(ui.Center).Children(func() {
			logo := modrinthLogo
			if p.Source == "curseforge" {
				logo = curseforgeLogo
			}
			ui.Box(c).Size(40*s, 40*s).Radius(10*s).Background(panel).Border(1, line).Center().Shrink(0).Children(func() {
				if p.Source == "multimc" {
					if !loaderLogo(c, p.Loader, 26*s) {
						ui.Icon(c, icons["layers"]).Size(20*s, 20*s).TextColor(accent)
					}
					return
				}
				ui.Image(c, logo).Size(24*s, 24*s).Fit(ui.Contain)
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(4 * s).Children(func() {
				ui.Row(c).Gap(8 * s).AlignItems(ui.Center).Children(func() {
					ui.Text(c, p.Name).FontSize(16.5 * s).Bold().TextColor(ink).SingleLine()
					if p.Version != "" {
						badge(c, p.Version, accent)
					}
				})
				meta := fmt.Sprintf("Minecraft %s  ·  %s", p.Minecraft, a.loaderDisplay(p.Loader))
				if p.LoaderVersion != "" {
					meta += " " + p.LoaderVersion
				}
				if p.Source == "multimc" {
					meta += "  ·  " + t.ImportSourceMultiMC
				} else {
					meta += fmt.Sprintf("  ·  %d files", p.FileCount)
				}
				label(c, meta).FontSize(13 * s).SingleLine()
			})
		})
		if p.Unavailable != "" {
			ui.Row(c).Label("整合包不可用").FillWidth().Padding(10*s, 14*s).Radius(10 * s).Background(dangerSoft).Gap(8 * s).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons["cross"]).Size(14*s, 14*s).TextColor(danger).Shrink(0)
				ui.Text(c, p.Unavailable).FontSize(13 * s).TextColor(danger).Grow(1).MinWidth(0)
			})
		}
	} else if a.packErr != "" && strings.TrimSpace(a.form.Path) != "" {
		ui.Row(c).Label("整合包错误").FillWidth().Padding(10*s, 14*s).Radius(10 * s).Background(dangerSoft).Children(func() {
			ui.Text(c, a.packErr).FontSize(13 * s).TextColor(danger).Grow(1).MinWidth(0)
		})
	}
	a.folderField(c, t.ImportParentLabel, &a.form.Parent)
	field(c, t.ImportNameLabel, &a.form.Name, t.ImportNamePlaceholder)
	label(c, t.ImportNote).FontSize(13.5 * s)
	if p := a.packPreview; p != nil && p.DownloadMB > 0 && p.Unavailable == "" {
		size := fmt.Sprintf("%d MB", p.DownloadMB)
		if p.DownloadMB >= 1024 {
			size = fmt.Sprintf("%.1f GB", float64(p.DownloadMB)/1024)
		}
		label(c, fmt.Sprintf(t.ImportSizeNote, size)).Label("导入下载大小").FontSize(13.5 * s).TextColor(accent)
	}
	blocked := a.packPreview == nil || a.packPreview.Unavailable != ""
	if buttonWhy(c, "import-pack-now", t.ImportConfirmBtn, "tasks", true, whyOf(a.busy, t.WhyBusy, a.packPreview != nil && a.packPreview.Unavailable != "", a.packPreviewUnavailable(), blocked || a.form.Path == "" || a.form.Parent == "" || a.form.Name == "", t.WhyImportMissing), true) {
		a.execute("importPack", launcher.ImportRequest{Path: a.form.Path, Parent: a.form.Parent, Name: a.form.Name}, a.taskAdded)
	}
}

func (a *App) installDialog(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	label(c, t.InstallDesc)

	// Step 1: Target Location
	ui.Column(c).FillWidth().Gap(10*s).Padding(16*s).Radius(12*s).Background(raised).Border(1, line).Children(func() {
		ui.Text(c, t.InstallStep1Title).FontSize(16 * s).Bold().TextColor(ink)
		if a.root() != nil {
			if selectBox(c, "install-target", t.InstallTargetLabel, &a.form.Target, []string{t.InstallTargetCurrent, t.InstallTargetNew}).FillWidth().Changed() {
				a.form.Name = ""
			}
		}
		if normalizeInstallTarget(a.form.Target) == "current" && a.root() != nil {
			pathLabel(c, a.root().Path, 13*s)
		} else {
			a.folderField(c, t.InstallParentLabel, &a.form.Parent)
			field(c, t.InstallNameLabel, &a.form.Name, t.InstallNamePlaceholder)
		}
	})

	// Step 2: Official Minecraft Version
	ui.Column(c).FillWidth().Gap(10*s).Padding(16*s).Radius(12*s).Background(raised).Border(1, line).Children(func() {
		ui.Text(c, t.InstallStep2Title).FontSize(16 * s).Bold().TextColor(ink)
		a.minecraftVersionPicker(c)
	})

	// Step 3: Mod Loader & Version with Interactive Icon Cards + Scrollable Version Selector
	ui.Column(c).FillWidth().Gap(10*s).Padding(16*s).Radius(12*s).Background(raised).Border(1, line).Children(func() {
		ui.Text(c, t.InstallStep3Title).FontSize(16 * s).Bold().TextColor(ink)
		ui.Row(c).FillWidth().Gap(8 * s).Children(func() {
			for _, item := range []struct {
				name, sub string
			}{
				{t.LoaderVanilla, t.InstallLoaderVanillaSub},
				{t.LoaderFabric, t.InstallLoaderFabricSub},
				{t.LoaderForge, t.InstallLoaderForgeSub},
				{t.LoaderNeoForge, t.InstallLoaderNeoSub},
				{t.LoaderQuilt, t.InstallLoaderQuiltSub},
			} {
				ld := item
				selected := loaderKey(a.form.Loader) == loaderKey(ld.name)
				cardBg := panel
				borderCol := line
				if selected {
					cardBg = accentSoft
					borderCol = accent
				}
				b := ui.ButtonBase(c.Key("loader-card-"+loaderKey(ld.name))).Label(ld.name).Grow(1).MinWidth(0).Padding(10*s, 8*s).Radius(10*s).Border(1, borderCol).AlignItems(ui.Center).Justify(ui.Center)
				if b.Hovered() && !selected {
					cardBg = fieldColor
				}
				b.Background(b.AnimateColor("bg", cardBg, fastAnim))
				if b.Children(func() {
					ui.Column(c).AlignItems(ui.Center).Gap(5 * s).Children(func() {
						if !loaderLogo(c, ld.name, 34*s) {
							ui.Icon(c, icons[loaderIconName(ld.name)]).Size(22*s, 22*s).TextColor(loaderTone(ld.name))
						}
						ui.Text(c, ld.name).FontSize(14.5 * s).Bold().TextColor(ink).NoWrap()
						label(c, ld.sub).FontSize(11 * s).SingleLine()
					})
				}).Clicked() && !selected {
					a.form.Loader = ld.name
					a.loadLoaderVersions()
				}
			}
		})
		if loaderSelectBox(c, "install-loader-select", t.InstallLoaderAria, &a.form.Loader, []string{t.LoaderVanilla, t.LoaderFabric, t.LoaderForge, t.LoaderNeoForge, t.LoaderQuilt}).FillWidth().Changed() {
			a.loadLoaderVersions()
		}
		isVanilla := loaderKey(a.form.Loader) == "vanilla"
		if !isVanilla {
			label(c, t.InstallLoaderVerLabel).FontWeight(600)
			if a.loaderLoading {
				statusLine(c, t.InstallFetchingLoader, muted, true)
			} else if a.loaderError != "" {
				statusLine(c, a.loaderError, danger, false)
			} else if len(a.loaders) == 0 {
				statusLine(c, t.InstallNoLoaderVer, muted, false)
			} else {
				selectBox(c, "install-loader-ver", t.InstallLoaderVerLabel, &a.form.LoaderVersion, a.loaders).FillWidth()
			}
		}
	})

	isCurrent := normalizeInstallTarget(a.form.Target) == "current" && a.root() != nil
	isVanilla := loaderKey(a.form.Loader) == "vanilla"
	invalid := a.form.Minecraft == "" || (!isVanilla && a.form.LoaderVersion == "") || (!isCurrent && (a.form.Parent == "" || a.form.Name == ""))
	if buttonWhy(c, "install-game-now", t.InstallConfirmBtn, "tasks", true, whyOf(a.busy, t.WhyBusy, invalid, t.WhyInstallInvalid), true) {
		root := ""
		if isCurrent {
			root = a.root().ID
		}
		a.execute("installGame", map[string]string{"rootId": root, "parent": a.form.Parent, "name": a.form.Name, "minecraft": a.form.Minecraft, "loader": loaderKey(a.form.Loader), "loaderVersion": a.form.LoaderVersion}, a.taskAdded)
	}
}

func (a *App) projectDialog(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	p := a.project
	if p == nil {
		return
	}
	ui.Row(c).Gap(16 * s).AlignItems(ui.Center).Children(func() {
		if img := a.remoteImage(p.Icon); img != nil {
			ui.Image(c, img).Size(48*s, 48*s).Fit(ui.Contain)
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(6 * s).Children(func() {
			ui.Text(c, p.Title).FontSize(18 * s).Bold().TextColor(ink).SingleLine()
			label(c, p.Description)
		})
		if button(c, "project-site", t.ProjectWebsiteBtn, "arrow", false, false) && p.URL != "" {
			c.OpenURL(p.URL)
		}
	})
	if len(a.projectVersions) == 0 {
		label(c, t.ProjectNoVersions)
		return
	}
	label(c, t.ProjectVersionLabel).FontWeight(600)
	ui.TextInput(c.Key("project-ver-filter"), &a.projectVersionFilter).Label(t.ProjectFilterAria).Placeholder(t.ProjectFilterHint).FillWidth()

	filtered := []launcher.ProjectVersion{}
	q := strings.ToLower(strings.TrimSpace(a.projectVersionFilter))
	for _, v := range a.projectVersions {
		meta := strings.ToLower(v.Name + " " + v.Number + " " + strings.Join(v.GameVersions, " ") + " " + strings.Join(v.Loaders, " "))
		if q != "" && !strings.Contains(meta, q) {
			continue
		}
		filtered = append(filtered, v)
	}

	// Scrollable Version List with full details so any version can be scrolled and chosen
	scrollArea(c, "project-versions-scroll").MaxHeight(230*s).FillWidth().Padding(4*s, scrollGutter*s, 4*s, 4*s).Radius(11*s).Background(raised).Border(1, line).Children(func() {
		ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
			for _, ver := range filtered {
				v := ver
				selected := v.ID == a.form.VersionID
				rowBg := panel
				borderCol := line
				if selected {
					rowBg = accentSoft
					borderCol = accent
				}
				b := ui.ButtonBase(c.Key("proj-ver-"+v.ID)).Label(v.Name+" "+v.Number).FillWidth().Padding(10*s, 14*s).Radius(9*s).Border(1, borderCol).AlignItems(ui.Center).Justify(ui.Start).Gap(10 * s)
				if b.Hovered() && !selected {
					rowBg = fieldColor
				}
				b.Background(b.AnimateColor("bg", rowBg, fastAnim))
				if b.Children(func() {
					ui.Column(c).Grow(1).MinWidth(0).Gap(3 * s).Children(func() {
						ui.Row(c).Gap(8 * s).AlignItems(ui.Center).Children(func() {
							ui.Text(c, v.Name).FontSize(15 * s).Bold().TextColor(ink).SingleLine()
							if v.Number != "" && v.Number != v.Name {
								badge(c, v.Number, blue)
							}
						})
						meta := strings.Join(v.GameVersions, ", ")
						if len(v.Loaders) > 0 {
							meta += "  ·  " + strings.Join(v.Loaders, ", ")
						}
						label(c, meta).FontSize(12.5 * s).SingleLine()
					})
					if selected {
						ui.Icon(c, icons["check"]).Size(16*s, 16*s).TextColor(accent).Shrink(0)
					}
				}).Clicked() {
					a.form.VersionID = v.ID
				}
			}
		})
	})

	switch p.Type {
	case "mod":
		if a.root() == nil {
			label(c, t.ProjectNeedRootHint)
			return
		}
		targetText := a.root().Name
		if a.root().SelectedVersion != "" {
			targetText += " / " + a.root().SelectedVersion
		}
		label(c, t.ProjectTargetPrefix+targetText)
		if buttonWhy(c, "download-mod-now", t.ProjectInstallModBtn, "tasks", true, whyOf(a.busy, t.WhyBusy, a.form.VersionID == "", t.WhyPickVersion), true) {
			a.execute("installMod", map[string]string{"rootId": a.root().ID, "source": p.Source, "projectId": p.ID, "versionId": a.form.VersionID}, a.taskAdded)
		}
	case "resourcepack", "shader":
		if a.root() == nil {
			label(c, t.ProjectNeedRootHint)
			return
		}
		targetText := a.root().Name
		if a.root().SelectedVersion != "" {
			targetText += " / " + a.root().SelectedVersion
		}
		label(c, t.ProjectTargetPrefix+targetText)
		btnLabel := t.ProjectInstallRPBtn
		if p.Type == "shader" {
			btnLabel = t.ProjectInstallShaderBtn
		}
		if buttonWhy(c, "download-assetpack-now", btnLabel, "tasks", true, whyOf(a.busy, t.WhyBusy, a.form.VersionID == "", t.WhyPickVersion), true) {
			a.execute("installAssetPack", map[string]string{"rootId": a.root().ID, "kind": p.Type, "source": p.Source, "projectId": p.ID, "versionId": a.form.VersionID}, a.taskAdded)
		}
	default:
		a.folderField(c, t.ImportParentLabel, &a.form.Parent)
		field(c, t.ImportNameLabel, &a.form.Name, "")
		if buttonWhy(c, "download-pack-now", t.ProjectInstallPackBtn, "tasks", true, whyOf(a.busy, t.WhyBusy, a.form.VersionID == "", t.WhyPickVersion, a.form.Parent == "" || a.form.Name == "", t.WhyImportMissing), true) {
			a.execute("importPack", launcher.ImportRequest{Parent: a.form.Parent, Name: a.form.Name, Source: p.Source, ProjectID: p.ID, VersionID: a.form.VersionID}, a.taskAdded)
		}
	}
}

func (a *App) packDialog(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	r := a.packReport
	if r == nil || r.Pack == nil {
		return
	}
	ui.Text(c, r.Pack.Name+" / "+r.Pack.Version).FontSize(18 * s).Bold().TextColor(ink).SingleLine()
	if r.Latest != nil {
		badge(c, t.PackUpdateFoundPrefix+r.Latest.Number, accent)
		if button(c, "pack-update-install", t.PackUpdateInstallBtn, "tasks", true, a.busy) {
			p := launcher.Project{ID: r.Pack.ProjectID, Source: r.Pack.Source, Title: r.Pack.Name, Type: "modpack"}
			v := *r.Latest
			a.project = &p
			a.projectVersions = []launcher.ProjectVersion{v}
			a.form = formState{Name: cleanName(p.Title + "-" + v.Number), VersionID: v.ID}
			a.dialog = "project"
		}
	}
	for _, f := range r.Files {
		ui.Row(c).Gap(14 * s).AlignItems(ui.Center).Children(func() {
			pathLabel(c, f.Path, 13*s)
			status := a.packFileStatusText(f.Status)
			color := muted
			if f.Status == "missing" || f.Status == "modified" {
				color = accent
			}
			badge(c, status, color)
		})
	}
	for _, manual := range r.Pack.Manual {
		m := manual
		label(c, t.PackManualPrefix+m.Name)
		if button(c, "manual-"+m.FileID, t.PackManualOpenBtn, "arrow", false, false) {
			c.OpenURL(m.URL)
		}
	}
}

func (a *App) loadLogs() {
	r := a.root()
	if r == nil {
		return
	}
	a.execute("logs", map[string]string{"rootId": r.ID}, func(b json.RawMessage) { a.logText = strings.Join(decode[[]string](b), "\n") })
}

func (a *App) logsDialog(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	ui.Row(c).Gap(12 * s).Children(func() {
		if button(c, "refresh-logs", t.LogsRefreshBtn, "refresh", false, a.busy) {
			a.loadLogs()
		}
		if button(c, "copy-logs", t.LogsCopyBtn, "", false, false) {
			c.WriteClipboard(a.logText)
		}
	})
	if a.logText == "" {
		label(c, t.LogsEmpty)
	} else {
		ui.TextArea(c, &a.logText).Label(t.LogsAreaAria).Height(380 * s).FillWidth()
	}
}

// accountTabAnim is the ease-out curve shared by the login-mode thumb and
// the form crossfade.
var accountTabAnim = motionMove

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

// authServerEditor holds the inline add/edit form for a saved server.
type authServerEditor struct {
	prev, name, url, err string
}

// authServerPicker lets the user choose a saved Yggdrasil server (which
// pre-fills the address) or type a custom one, and manage saved servers.
func (a *App) authServerPicker(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	if a.serverEdit != nil {
		a.authServerEditorView(c)
		return
	}
	servers := a.snapshot.AuthServers
	names := []string{}
	byName := map[string]launcher.AuthServer{}
	selected := t.AccServerCustom
	for _, v := range servers {
		n := v.Name
		if _, dup := byName[n]; dup {
			n += " (" + v.URL + ")"
		}
		names = append(names, n)
		byName[n] = v
		if a.form.Server != "" && launcher.SameAuthServer(a.form.Server, v.URL) {
			selected = n
			a.form.Server = v.URL // e.g. a typed site root → resolved built-in API
		}
	}
	if a.form.Server == "" && len(servers) > 0 && !a.serverCustom {
		selected = names[0]
		a.form.Server = byName[selected].URL
	}
	names = append(names, t.AccServerCustom)
	ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
		label(c, t.AccThirdServerLabel).FontWeight(600)
		ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
			drawIcon := func(opt string) {
				if v, ok := byName[opt]; ok {
					accountTypeIconL(c, a, "服务器图标 ", "yggdrasil", v.URL, 16*s, accountTagThird)
				} else {
					ui.Icon(c, icons["globe"]).Size(16*s, 16*s).TextColor(muted).Shrink(0)
				}
			}
			if selectBoxImpl(c, "auth-server-select", t.AccThirdServerLabel, &selected, names, nil, nil, drawIcon, func(opt string, hl bool) {
				if v, ok := byName[opt]; ok && v.Builtin {
					col := accent
					if hl {
						col = panel
					}
					ui.Text(c, t.AccBuiltinServer).Label("内置标记 "+opt).FontSize(11*s).FontWeight(600).TextColor(col).Padding(1*s, 6*s).Radius(999).Background(col.Alpha(.12)).SingleLine().Shrink(0)
				}
			}).Grow(1).Basis(0).MinWidth(0).Changed() {
				if v, ok := byName[selected]; ok {
					a.form.Server, a.serverCustom = v.URL, false
				} else {
					a.form.Server, a.serverCustom = "", true
				}
			}
			cur, saved := byName[selected]
			saved = saved && !cur.Builtin // built-ins are fixed
			if saved && iconOnlyButton(c, "auth-server-edit", "settings", t.AccEditServerBtn, 40*s, a.busy) {
				a.serverEdit = &authServerEditor{prev: cur.URL, name: cur.Name, url: cur.URL}
			}
			if saved && iconOnlyButton(c, "auth-server-delete", "cross", t.AccDeleteServerBtn, 40*s, a.busy) {
				a.execute("deleteAuthServer", map[string]string{"server": cur.URL}, func(json.RawMessage) { a.form.Server = "" })
			}
			if button(c, "auth-server-add", t.AccAddServerBtn, "plus", false, a.busy) {
				a.serverEdit = &authServerEditor{}
			}
		})
		if _, saved := byName[selected]; saved {
			label(c, a.form.Server).FillWidth().MinWidth(0).FontSize(12.5 * s).SingleLine().Tooltip(a.form.Server)
		} else {
			ui.TextInput(c, &a.form.Server).Label(t.AccThirdServerLabel + "输入").Placeholder("https://example.com/api/yggdrasil").FillWidth()
		}
	})
}

func (a *App) authServerEditorView(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	ed := a.serverEdit
	ui.Column(c.Key("auth-server-editor")).Label("服务器编辑").FillWidth().Gap(10*s).Padding(14*s).Radius(12*s).Background(panel).Border(1, accent.Alpha(.25)).Children(func() {
		title := t.AccAddServerBtn
		if ed.prev != "" {
			title = t.AccEditServerBtn
		}
		ui.Text(c, title).FontSize(15 * s).Bold().TextColor(ink)
		ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
			label(c, t.AccServerURLLabel).FontWeight(600)
			ui.Row(c).FillWidth().Gap(8 * s).Children(func() {
				ui.TextInput(c, &ed.url).Label(t.AccServerURLLabel).Placeholder("https://littleskin.cn/api/yggdrasil").Grow(1).Basis(0).MinWidth(0)
				if button(c, "auth-server-fetch", t.AccFetchNameBtn, "refresh", false, a.busy || strings.TrimSpace(ed.url) == "") {
					if _, e := launcher.NormalizeAuthServerURL(ed.url); e != nil {
						ed.err = t.AccServerURLInvalid
					} else {
						ed.err = ""
						a.execute("fetchAuthServerName", map[string]string{"server": ed.url}, func(b json.RawMessage) {
							if n := decode[string](b); n != "" {
								ed.name = n
							}
						})
					}
				}
			})
		})
		field(c, t.AccServerNameLabel, &ed.name, t.AccServerNameHint)
		if ed.err != "" {
			ui.Text(c, ed.err).FontSize(13 * s).TextColor(danger)
		}
		ui.Row(c).FillWidth().Gap(8 * s).Justify(ui.End).Children(func() {
			if button(c, "auth-server-cancel", t.AccServerCancelBtn, "", false, false) {
				a.serverEdit = nil
			}
			if button(c, "auth-server-save", t.AccServerSaveBtn, "check", true, a.busy || strings.TrimSpace(ed.url) == "") {
				u, e := launcher.NormalizeAuthServerURL(ed.url)
				if e != nil {
					ed.err = t.AccServerURLInvalid
					return
				}
				a.execute("saveAuthServer", map[string]string{"id": ed.prev, "name": ed.name, "server": u}, func(json.RawMessage) {
					a.form.Server, a.serverCustom = u, false
					a.serverEdit = nil
				})
			}
		})
	})
}

// autoMemoryFor is the computed automatic heap (MiB) of a version of the
// current directory, 0 when unknown.
func (a *App) autoMemoryFor(versionID string) int {
	if r := a.root(); r != nil {
		for _, v := range r.Versions {
			if v.ID == versionID {
				if v.AutoMemoryMB > 0 {
					return v.AutoMemoryMB
				}
				return launcher.AutoMemoryMB(launcher.AutoMemoryInput{TotalMB: a.memoryInfo.TotalMB, AvailableMB: a.memoryInfo.AvailableMB, Minecraft: v.Minecraft, Loader: v.Loader, Mods: r.ModCount})
			}
		}
	}
	return 0
}

// memoryModeRow is the 自动 / 手动 switch with the computed value and a
// one-line explanation when automatic.
func (a *App) memoryModeRow(c *ui.Context, key, title string, auto *bool, autoMB int) {
	s := uiScale(c)
	t := a.tr()
	ui.Column(c.Key(key)).FillWidth().Gap(6 * s).Children(func() {
		ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
			label(c, title).FontWeight(600).Grow(1).MinWidth(0).SingleLine()
			cur := t.MemModeManual
			if *auto {
				cur = t.MemModeAuto
			}
			if picked, ok := segmented(c, key+"-seg", []string{t.MemModeAuto, t.MemModeManual}, cur, 28*s); ok {
				*auto = picked == t.MemModeAuto
			}
		})
		if *auto {
			val := t.MemModeAuto
			if autoMB > 0 {
				val = fmt.Sprintf(t.MemAutoValue, float64(autoMB)/1024)
			}
			ui.Text(c, val).Label("自动内存值").FontSize(14 * s).FontWeight(700).TextColor(accent).SingleLine()
			label(c, t.MemAutoExplain).FontSize(12 * s).FillWidth().MinWidth(0)
		}
	})
}
