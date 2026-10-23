package nativeui

import (
	"encoding/json"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

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
					ui.Text(c, t.AccBuiltinServer).Label("内置标记 "+opt).FontSize(typeCaption2*s).FontWeight(600).TextColor(col).Padding(1*s, 6*s).Radius(999).Background(col.Alpha(.12)).SingleLine().Shrink(0)
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
			if saved && iconOnlyButton(c, "auth-server-edit", "settings", t.AccEditServerBtn, btnMd*s, a.dialogPending()) {
				a.serverEdit = &authServerEditor{prev: cur.URL, name: cur.Name, url: cur.URL}
			}
			if saved && iconOnlyButton(c, "auth-server-delete", "cross", t.AccDeleteServerBtn, btnMd*s, a.dialogPending()) {
				a.execute("deleteAuthServer", map[string]string{"server": cur.URL}, func(json.RawMessage) { a.form.Server = "" })
			}
			if button(c, "auth-server-add", t.AccAddServerBtn, "plus", false, a.dialogPending()) {
				a.serverEdit = &authServerEditor{}
			}
		})
		if _, saved := byName[selected]; saved {
			label(c, a.form.Server).FillWidth().MinWidth(0).FontSize(typeFootnote * s).SingleLine().Tooltip(a.form.Server)
		} else {
			textField(c, &a.form.Server).Label(t.AccThirdServerLabel + "输入").Placeholder("https://example.com/api/yggdrasil").FillWidth()
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
		ui.Text(c, title).FontSize(typeBody * s).Bold().TextColor(ink)
		ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
			label(c, t.AccServerURLLabel).FontWeight(600)
			ui.Row(c).FillWidth().Gap(8 * s).Children(func() {
				textField(c, &ed.url).Label(t.AccServerURLLabel).Placeholder("https://littleskin.cn/api/yggdrasil").Grow(1).Basis(0).MinWidth(0)
				if button(c, "auth-server-fetch", t.AccFetchNameBtn, "refresh", false, a.dialogPending() || strings.TrimSpace(ed.url) == "") {
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
			ui.Text(c, ed.err).FontSize(typeCallout * s).TextColor(danger)
		}
		ui.Row(c).FillWidth().Gap(8 * s).Justify(ui.End).Children(func() {
			if button(c, "auth-server-cancel", t.AccServerCancelBtn, "", false, false) {
				a.serverEdit = nil
			}
			if button(c, "auth-server-save", t.AccServerSaveBtn, "check", true, a.dialogPending() || strings.TrimSpace(ed.url) == "") {
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
