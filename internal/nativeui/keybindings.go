package nativeui

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type keyBindingsState struct {
	rootID, version, query, scope, tab, mod, capture, manual string
	view                                                     *launcher.KeyBindingsView
	bindings                                                 map[string]string
	options, inputs                                          map[string]string
	invalid                                                  map[string]bool
	listFilter                                               string
	labelsCancel                                             context.CancelFunc
	labelsGen                                                uint64
	labelsLoading, saved                                     bool
	pendingLabels                                            *launcher.KeyBindingsView
	backups                                                  []launcher.GameSettingsBackup
	backupsLoaded                                            bool
	backupConfirm, backupMessage                             string
}

func (a *App) globalKeyBindingsSection(c *ui.Context) {
	settingsGroup(c, "global-keybindings", a.L("keybindings.global_title"), "", func() {
		settingRow(c, nil, a.L("keybindings.title"), a.L("keybindings.global_desc"), func() {
			if button(c, "global-keybindings-open", a.L("keybindings.manage"), "settings", false, false) {
				a.openKeyBindings("", "")
			}
		}, nil)
	})
}

func (a *App) packKeyBindingsSection(c *ui.Context, root launcher.Instance) {
	version := a.form.ConfigVersion
	if version == "" {
		version = root.SelectedVersion
	}
	setCard(c, "pack-keybindings", "settings", a.L("keybindings.pack_title"), a.L("keybindings.pack_desc"), 0, func() {
		if button(c, "pack-keybindings-open", a.L("keybindings.manage"), "settings", false, version == "") {
			a.openKeyBindings(root.ID, version)
		}
	})
}

func (a *App) openKeyBindings(rootID, version string) {
	form := a.form
	a.openDialog("keyBindings")
	a.form = form
	a.keyBindings = &keyBindingsState{rootID: rootID, version: version, scope: "all", tab: "keys", mod: "all"}
	a.loadKeyBindings()
}

func (a *App) loadKeyBindings() {
	g := a.keyBindings
	if g == nil {
		return
	}
	a.cancelKeyBindingLabels()
	g.saved = false
	g.backupsLoaded, g.backupConfirm, g.backupMessage = false, "", ""
	a.execute("keyBindings", map[string]string{"rootId": g.rootID, "version": g.version}, func(b json.RawMessage) {
		if a.keyBindings == g {
			g.setView(decode[launcher.KeyBindingsView](b))
			a.notice("")
			a.loadKeyBindingLabels(g)
			if g.tab == "backups" {
				a.loadGameSettingsBackups(g)
			}
		}
	})
}

func (g *keyBindingsState) setView(view launcher.KeyBindingsView) {
	if g.view != nil {
		mergeBindingLabels(&view, *g.view)
	}
	g.view, g.bindings = &view, map[string]string{}
	g.options, g.inputs, g.invalid = map[string]string{}, map[string]string{}, map[string]bool{}
	g.capture, g.manual = "", ""
	for _, row := range view.Options {
		if row.Configured {
			g.options[row.ID] = row.Override
		}
	}
	for _, row := range view.Rows {
		if row.Configured {
			g.bindings[row.ID] = row.Override
		}
	}
}

func (a *App) keyBindingName(row launcher.KeyBindingRow) string {
	if row.Label != "" {
		return a.L(row.Label)
	}
	locales := [3]string{"zh_cn", "zh_tw", "en_us"}
	if a.isEN() {
		locales = [3]string{"en_us", "zh_cn", "zh_tw"}
	}
	for _, locale := range locales {
		if name := row.Names[locale]; name != "" {
			return name
		}
	}
	return strings.TrimPrefix(row.ID, "key_")
}

func hasChineseBindingName(row launcher.KeyBindingRow) bool {
	return row.Vanilla || row.Names["zh_cn"] != "" || row.Names["zh_tw"] != ""
}

func (a *App) keyBindingMatches(row launcher.KeyBindingRow, query, scope string) bool {
	if scope == "vanilla" && !row.Vanilla || scope == "mods" && row.Vanilla || scope == "untranslated" && hasChineseBindingName(row) {
		return false
	}
	text := strings.Join([]string{a.keyBindingName(row), row.ID, row.Namespace, row.ModName, row.Names["zh_cn"], row.Names["zh_tw"], row.Names["en_us"]}, " ")
	return strings.Contains(strings.ToLower(text), strings.ToLower(strings.TrimSpace(query)))
}

func (a *App) keyBindingValue(value string) string {
	if value == "" {
		return a.L("keybindings.unset")
	}
	base, modifier, modified := strings.Cut(value, ":")
	name := base
	switch {
	case strings.HasPrefix(base, "key.mouse."):
		n := strings.TrimPrefix(base, "key.mouse.")
		if n == "left" || n == "right" || n == "middle" {
			name = a.L("keybindings.key.mouse." + n)
		} else {
			name = fmt.Sprintf(a.L("keybindings.key.mouse.number"), n)
		}
	case strings.HasPrefix(base, "scancode."):
		name = fmt.Sprintf(a.L("keybindings.key.scancode"), strings.TrimPrefix(base, "scancode."))
	case strings.HasPrefix(base, "key.keyboard."):
		n := strings.TrimPrefix(base, "key.keyboard.")
		switch {
		case len(n) == 1, strings.HasPrefix(n, "f") && len(n) > 1 && len(n) <= 3 && n[1] >= '0' && n[1] <= '9':
			name = strings.ToUpper(n)
		case strings.HasPrefix(n, "keypad."):
			name = fmt.Sprintf(a.L("keybindings.key.numpad"), strings.TrimPrefix(n, "keypad."))
		default:
			name = a.L("keybindings.key." + n)
			if name == "keybindings.key."+n {
				name = strings.ReplaceAll(n, ".", " ")
			}
		}
	}
	if modified && modifier != "NONE" {
		name = a.L("keybindings.modifier."+modifier) + " + " + name
	}
	return name
}

func (a *App) keyBindingsFooter(c *ui.Context) {
	g := a.keyBindings
	if g == nil {
		return
	}
	s := uiScale(c)
	running := false
	if r := a.rootByID(g.rootID); r != nil {
		running = r.Running
	}
	if running {
		label(c, a.L("keybindings.running")).FillWidth().MinWidth(0)
	}
	ui.Box(c).FillWidth().Height(1).Background(line)
	ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
		if button(c, "keybindings-reload", a.L("keybindings.reload"), "refresh", false, a.dialogPending()) {
			if !a.guardDiscard(g.dirty(), a.loadKeyBindings) {
				a.loadKeyBindings()
			}
		}
		if g.view == nil {
			return
		}
		status := ""
		switch {
		case a.pendingOp("saveKeyBindings"):
			status = "game_options.saving"
		case a.pendingOp("restoreGameSettings"):
			status = "game_options.backup.restoring"
		case a.pendingOp("keyBindings"):
			status = "keybindings.loading"
		case len(g.invalid) > 0:
			status = "game_options.invalid_draft"
		case g.dirty():
			status = "game_options.unsaved"
		case g.saved && g.tab != "backups" && !a.errorMessage:
			status = "keybindings.saved"
			if g.rootID != "" {
				status = "keybindings.applied"
			}
		}
		label(c, a.L(status)).FontSize(typeCaption * s).TextColor(accent).Grow(1).Basis(0).MinWidth(0)
		if g.tab == "backups" {
			return
		}
		blocked := a.dialogPending() || running || len(g.invalid) > 0 || g.capture != "" || g.rootID != "" && !g.view.HasOptions
		text := a.L("keybindings.save")
		if g.rootID != "" {
			text = a.L("game_options.save_apply")
		}
		if button(c, "keybindings-save", text, "check", true, blocked) {
			a.saveKeyBindings()
		}
	})
}

func (g *keyBindingsState) dirty() bool {
	if len(g.invalid) > 0 {
		return true
	}
	if g.view == nil {
		return false
	}
	saved, options := map[string]string{}, map[string]string{}
	for _, row := range g.view.Rows {
		if row.Configured {
			saved[row.ID] = row.Override
		}
	}
	for _, row := range g.view.Options {
		if row.Configured {
			options[row.ID] = row.Override
		}
	}
	return !maps.Equal(saved, g.bindings) || !maps.Equal(options, g.options)
}

func (a *App) saveKeyBindings() {
	g := a.keyBindings
	if g == nil || g.view == nil {
		return
	}
	a.execute("saveKeyBindings", map[string]any{"rootId": g.rootID, "version": g.version, "revision": g.view.Revision, "bindings": maps.Clone(g.bindings), "options": maps.Clone(g.options), "apply": g.rootID != ""}, func(b json.RawMessage) {
		if a.keyBindings != g {
			return
		}
		g.setView(decode[launcher.KeyBindingsView](b))
		g.saved = true
		g.backupsLoaded, g.backupMessage = false, ""
		if g.tab == "backups" {
			a.loadGameSettingsBackups(g)
		}
		key := "keybindings.saved"
		if g.rootID != "" {
			key = "keybindings.applied"
		}
		a.notice(a.L(key))
	})
}
