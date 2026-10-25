package nativeui

import (
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// Session state for the log view. scope is a root ID (the instance's own
// log tab) or "" (the all-games log window).
type logSessions struct {
	list   []launcher.GameSession
	polled time.Time
	sel    map[string]string
	manual map[string]bool
}

func (a *App) logScope() string {
	if a.dialog == "logs" {
		return ""
	}
	if r := a.root(); r != nil {
		return r.ID + "|" + r.SelectedVersion
	}
	return ""
}

// scopedSessions polls the session list at most twice a second.
func (a *App) scopedSessions(scope string) []launcher.GameSession {
	ls := &a.logSess
	if a.service != nil && time.Since(ls.polled) > 500*time.Millisecond {
		ls.list, ls.polled = a.service.Sessions(""), time.Now()
	}
	out := make([]launcher.GameSession, 0, len(ls.list))
	for _, x := range ls.list {
		if scope == "" || x.RootID+"|"+x.Version == scope {
			out = append(out, x)
		}
	}
	return out
}

// selectedSession picks the user's choice, else the newest launch.
func (a *App) selectedSession(scope string, list []launcher.GameSession) *launcher.GameSession {
	ls := &a.logSess
	if ls.sel == nil {
		ls.sel, ls.manual = map[string]string{}, map[string]bool{}
	}
	if len(list) == 0 {
		return nil
	}
	if ls.manual[scope] {
		for i := range list {
			if list[i].ID == ls.sel[scope] {
				return &list[i]
			}
		}
		ls.manual[scope] = false // the picked one was closed
	}
	newest := &list[len(list)-1]
	ls.sel[scope] = newest.ID
	return newest
}

// verRunning is whether the selected version itself has a live game.
func verRunning(r *launcher.Instance) bool {
	return r != nil && slices.Contains(r.RunningVersions, r.SelectedVersion)
}

func sessionTone(state string) ui.Color {
	switch state {
	case "running":
		return successInk
	case "crashed":
		return danger
	}
	return muted
}

func (a *App) sessionStateText(state string) string {
	switch state {
	case "running":
		return a.L("运行中", "Running")
	case "crashed":
		return a.L("崩溃", "Crashed")
	}
	return a.L("已退出", "Exited")
}

func (a *App) sessionLabel(g launcher.GameSession, list []launcher.GameSession) string {
	dup := g.Seq > 1
	for _, x := range list {
		if x.ID != g.ID && x.RootID == g.RootID && x.Version == g.Version {
			dup = true
		}
	}
	if dup {
		return g.Name + " #" + strconv.Itoa(g.Seq)
	}
	return g.Name
}

func statusDot(c *ui.Context, col ui.Color, s float32) {
	ui.Box(c).Size(7*s, 7*s).Radius(999).Background(col).Shrink(0)
}

// sessionSwitcher is a compact segmented strip; only drawn for 2+ sessions.
func (a *App) sessionSwitcher(c *ui.Context, scope string, list []launcher.GameSession, cur *launcher.GameSession) {
	s := uiScale(c)
	ui.Row(c.Key("log-sessions")).FillWidth().Shrink(0).Gap(6 * s).Wrap().AlignItems(ui.Center).Children(func() {
		for _, g := range list {
			g := g
			sel := cur != nil && cur.ID == g.ID
			name := a.sessionLabel(g, list)
			if scope != "" { // the pack tab already names the pack
				name = "#" + strconv.Itoa(g.Seq)
			}
			b := tooltip(c, ui.ButtonBase(c.Key("log-sess-"+g.ID)).Label(name), name+"  "+a.sessionStateText(g.State)).Height(30*s).Padding(0, 10*s, 0, 12*s).Gap(8 * s).Radius(8 * s).AlignItems(ui.Center).MaxWidth(240 * s).Shrink(0)
			bg, fg := ui.RGBA(120, 130, 160, .10), inkSecondary
			if sel {
				bg, fg = panel, ink
				b.Border(1, line).Shadow(0, 1*s, 3*s, 0, ui.RGBA(15, 23, 42, .08))
			} else if b.Hovered() {
				bg = ui.RGBA(120, 130, 160, .16)
			}
			b.Background(b.AnimateColor("bg", bg, fastAnim))
			closeHit := false
			if b.Children(func() {
				statusDot(c, sessionTone(g.State), s)
				ui.Text(c, name).FontSize(typeFootnote * s).FontWeight(600).FontFeatures("tnum").TextColor(fg).SingleLine().Ellipsis("…").MaxWidth(180 * s).Shrink(0)
				if g.AccountName != "" && a.sessionDup(g, list) {
					a.sessionAccount(c, g, 16*s, muted)
				}
				if g.State != "running" {
					x := tooltip(c, ui.ButtonBase(c.Key("log-sess-close-"+g.ID)).Label(a.L("关闭", "Close")), a.L("关闭", "Close")).Size(18*s, 18*s).Radius(5 * s).Center()
					if x.Hovered() {
						x.Background(ui.RGBA(120, 130, 160, .18))
					}
					if x.Children(func() {
						ui.Icon(c, icons["cross"]).Size(9*s, 9*s).TextColor(muted)
					}).Clicked() {
						closeHit = true
					}
				}
			}).Clicked() && !closeHit && !sel {
				a.logSess.sel[scope], a.logSess.manual[scope] = g.ID, true
				// the key change reloads only the log pane, keeping each filter
			}
			if closeHit && a.service != nil {
				_ = a.service.CloseSession(g.ID)
				a.logSess.polled = time.Time{}
				if sel {
					a.logSess.manual[scope] = false
				}
			}
		}
	})
}

func openLogFile(c *ui.Context, p string) {
	if p != "" {
		p = filepath.ToSlash(p)
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		c.OpenURL((&url.URL{Scheme: "file", Path: p}).String())
	}
}

var revealLogPath = mygo.Shell.ShowItemInFolder

func openLogFolder(path string) {
	if path != "" {
		// Pass the file itself to the native file manager so it is selected.
		// A file:// URL to its parent only opens the directory.
		revealLogPath(filepath.Clean(path))
	}
}

// sessionDup is whether another session runs the same pack and version.
func (a *App) sessionDup(g launcher.GameSession, list []launcher.GameSession) bool {
	for _, x := range list {
		if x.ID != g.ID && x.RootID == g.RootID && x.Version == g.Version {
			return true
		}
	}
	return false
}

// sessionAccount is a small head and name: who plays this copy.
func (a *App) sessionAccount(c *ui.Context, g launcher.GameSession, size float32, col ui.Color) {
	s := uiScale(c)
	ac := launcher.Account{ID: g.AccountID, Name: g.AccountName, Kind: g.AccountKind}
	for _, x := range a.snapshot.Accounts {
		if x.ID == g.AccountID {
			ac = x
		}
	}
	ui.Row(c).Gap(5 * s).AlignItems(ui.Center).Shrink(0).Children(func() {
		a.headAvatar(c, &ac, size, func() {
			ui.Box(c).Size(size, size).Radius(size * .22).Background(ui.RGBA(120, 130, 160, .16)).Shrink(0)
		})
		ui.Text(c, g.AccountName).FontSize(typeCaption * s).TextColor(col).SingleLine().MaxWidth(110 * s).Ellipsis("…")
	})
}
