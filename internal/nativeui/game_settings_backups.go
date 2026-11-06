package nativeui

import (
	"encoding/json"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func (a *App) loadGameSettingsBackups(g *keyBindingsState) {
	a.execute("gameSettingsBackups", map[string]string{"rootId": g.rootID, "version": g.version}, func(b json.RawMessage) {
		if a.keyBindings == g {
			g.backups = decode[[]launcher.GameSettingsBackup](b)
			g.backupsLoaded = true
		}
	})
}

func (a *App) createGameSettingsBackup(g *keyBindingsState) {
	g.backupMessage = ""
	a.execute("backupGameSettings", launcher.GameSettingsBackupRequest{RootID: g.rootID, Version: g.version, Revision: g.view.Revision}, func(b json.RawMessage) {
		if a.keyBindings == g {
			g.backups = decode[[]launcher.GameSettingsBackup](b)
			g.backupsLoaded = true
			g.backupMessage = "game_options.backup.created"
			a.notice(a.L(g.backupMessage))
		}
	})
}

func (a *App) restoreGameSettingsBackup(g *keyBindingsState, id string) {
	g.backupMessage = ""
	a.execute("restoreGameSettings", launcher.GameSettingsBackupRequest{RootID: g.rootID, Version: g.version, Revision: g.view.Revision, BackupID: id}, func(b json.RawMessage) {
		if a.keyBindings != g {
			return
		}
		a.cancelKeyBindingLabels()
		g.setView(decode[launcher.KeyBindingsView](b))
		g.mod, g.scope = "all", "all"
		g.backupConfirm, g.saved = "", false
		g.backupMessage = "game_options.backup.restored"
		a.notice(a.L(g.backupMessage))
		a.loadGameSettingsBackups(g)
		a.loadKeyBindingLabels(g)
	})
}

func (a *App) gameSettingsBackupsView(c *ui.Context, g *keyBindingsState) {
	s := uiScale(c)
	blocked := a.dialogPending()
	if r := a.rootByID(g.rootID); r != nil {
		blocked = blocked || r.Running
	}
	label(c, a.L("game_options.backup.description")).FontSize(typeFootnote * s).FillWidth().MinWidth(0).Shrink(0)
	ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Shrink(0).Children(func() {
		if button(c, "settings-backup-create", a.L("game_options.backup.create"), "download", false, blocked || g.rootID != "" && !g.view.HasOptions) {
			a.createGameSettingsBackup(g)
		}
		if g.backupMessage != "" && !a.errorMessage {
			label(c, a.L(g.backupMessage)).FontSize(typeCaption * s).TextColor(accent).Grow(1).MinWidth(0)
		}
	})
	scrollArea(c, "settings-backup-list").Grow(1).MinHeight(0).FillWidth().Padding(4*s, 8*s, 8*s, 4*s).Gap(12 * s).Children(func() {
		if !g.backupsLoaded {
			if a.dialogPending() {
				ui.Column(c).Label("备份加载占位").FillWidth().Gap(12 * s).Children(func() {
					settingsRowsSkeleton(c, "backup-loading", 3)
					label(c, a.L("keybindings.loading")).FontSize(typeCaption * s)
				})
			} else if button(c, "settings-backup-retry", a.L("keybindings.reload"), "refresh", false, false) {
				a.loadGameSettingsBackups(g)
			}
			return
		}
		if len(g.backups) == 0 {
			label(c, a.L("game_options.backup.empty"))
		}
		for _, backup := range g.backups {
			a.preferenceCard(c, "backup-"+backup.ID, "", func() {
				ui.Column(c).FillWidth().Padding(12*s, 16*s).Gap(8 * s).Children(func() {
					ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
						ui.Column(c).Grow(1).MinWidth(0).Gap(4 * s).Children(func() {
							ui.Text(c, backup.CreatedAt.Local().Format("2006-01-02 15:04:05")).FontSize(typeBody * s).TextColor(ink)
							ui.Row(c).Gap(12 * s).Children(func() {
								label(c, a.L("game_options.backup.reason."+backup.Reason)).FontSize(typeCaption * s)
								label(c, fmtSize(backup.Size)).FontSize(typeCaption * s)
							})
						})
						if button(c, "settings-backup-restore-"+backup.ID, a.L("game_options.backup.restore"), "refresh", false, blocked) {
							if !a.guardDiscard(g.dirty(), func() { a.restoreGameSettingsBackup(g, backup.ID) }) {
								a.restoreGameSettingsBackup(g, backup.ID)
							}
						}
					})
				})
			})
		}
	})
}
