package nativeui

import (
	"encoding/json"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// launchAgainState backs the 再开一个 dialog: which account plays the copy.
type launchAgainState struct {
	rootID, account string
}

// inUseAccounts are the accounts already playing this pack and version.
func (a *App) inUseAccounts(r launcher.Instance) map[string]bool {
	used := map[string]bool{}
	if a.service == nil {
		return used
	}
	for _, g := range a.service.Sessions("") {
		if g.State == "running" && g.RootID == r.ID && g.Version == r.SelectedVersion && g.AccountID != "" {
			used[g.AccountID] = true
		}
	}
	return used
}

// openLaunchAgain preselects an account that is not already in the game.
func (a *App) openLaunchAgain(r launcher.Instance) {
	used := a.inUseAccounts(r)
	pick := ""
	for _, ac := range a.snapshot.Accounts {
		if !used[ac.ID] {
			pick = ac.ID
			break
		}
	}
	if pick == "" {
		pick = a.snapshot.ActiveAccount
	}
	a.launchAgain = launchAgainState{rootID: r.ID, account: pick}
	a.openDialog("launchAgain")
}

func (a *App) launchAgainDialog(c *ui.Context) {
	s := uiScale(c)
	g := &a.launchAgain
	r := a.rootByID(g.rootID)
	if r == nil {
		a.closeDialog()
		return
	}
	used := a.inUseAccounts(*r)
	ui.Text(c, a.L("同一个整合包再开一个窗口，选择这次用哪个账号。", "Run one more copy of this pack. Pick the account for it.")).FontSize(typeFootnote * s).TextColor(inkSecondary)
	ui.Column(c.Key("again-accounts")).FillWidth().Gap(6 * s).Children(func() {
		for _, acc := range a.snapshot.Accounts {
			ac := acc
			sel := g.account == ac.ID
			row := ui.ButtonBase(c.Key("again-"+ac.ID)).Label(ac.Name).FillWidth().Height(48*s).PaddingX(12*s).Gap(12*s).Radius(10*s).AlignItems(ui.Center).Border(1, line)
			bg := ui.RGBA(255, 255, 255, .7)
			if sel {
				bg = accentSoft
				row.Border(1, accent.Alpha(.45))
			} else if row.Hovered() {
				bg = panel
			}
			row.Background(bg)
			if row.Children(func() {
				ui.Box(c).Size(16*s, 16*s).Radius(999).Border(1.5*s, ringColor(sel)).Center().Shrink(0).Children(func() {
					if sel {
						ui.Box(c).Size(8*s, 8*s).Radius(999).Background(accent)
					}
				})
				a.headAvatar(c, &ac, 26*s, func() {
					ui.Box(c).Size(26*s, 26*s).Radius(6*s).Background(panel).Border(1, line).Center().Shrink(0).Children(func() {
						ui.Icon(c, icons["account"]).Size(13*s, 13*s).TextColor(accent)
					})
				})
				ui.Text(c, ac.Name).FontSize(typeCallout * s).FontWeight(600).TextColor(ink).SingleLine().Grow(1).MinWidth(0).Tooltip(ac.Name)
				if used[ac.ID] {
					ui.Text(c, a.L("游戏中", "Playing")).FontSize(typeCaption * s).TextColor(successInk).NoWrap()
				}
				a.accountTypeTag(c, ac)
			}).Clicked() {
				g.account = ac.ID
			}
		}
	})
	// Same Microsoft account twice: allowed, but servers kick the older one.
	ui.Row(c.Key("again-note")).FillWidth().Height(36 * s).Gap(8 * s).AlignItems(ui.Center).Children(func() {
		if used[g.account] && a.accountKind(g.account) == "microsoft" {
			ui.Icon(c, icons["info"]).Size(13*s, 13*s).TextColor(warning).Shrink(0)
			ui.Text(c, a.L("同一个正版账号不能同时进入同一个服务器，单人世界不受影响。", "One Microsoft account can't join the same server twice; single player is fine.")).FontSize(typeFootnote * s).TextColor(inkSecondary).SingleLine().Shrink(1).MinWidth(0)
		}
	})
	ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.L("两个窗口共用存档、配置和模组。", "Both copies share saves, config and mods.")).FontSize(typeFootnote * s).TextColor(muted).SingleLine().Grow(1).Shrink(1).MinWidth(0)
		if fixedButton(c, "again-go", a.L("启动", "Launch"), "play", 0, btnMd*s, true, g.account == "" || a.pendingAt("launchAgain", "rootId", g.rootID)) {
			id, acc := g.rootID, g.account
			a.closeDialog()
			a.execute("launchAgain", map[string]string{"rootId": id, "accountId": acc}, func(json.RawMessage) { a.notice(a.tr().InstallLaunchPreparing) })
		}
	})
}

func (a *App) accountKind(id string) string {
	for _, ac := range a.snapshot.Accounts {
		if ac.ID == id {
			return ac.Kind
		}
	}
	return ""
}

func ringColor(sel bool) ui.Color {
	if sel {
		return accent
	}
	return line
}
