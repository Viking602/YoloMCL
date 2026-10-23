package nativeui

import (
	"encoding/json"
	"fmt"
	neturl "net/url"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func (a *App) accountDialog(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	// With saved accounts the list is the focus; adding one is a separate
	// step behind "+ 添加账号". With none, go straight to the add form.
	if len(a.snapshot.Accounts) > 0 && !a.accountAdding {
		a.accountList(c)
		ui.Row(c).FillWidth().Gap(space2 * s).AlignItems(ui.Center).Children(func() {
			if button(c, "account-add-open", t.AccAddAccountBtn, "plus", true, a.dialogPending()) {
				a.accountAdding = true
			}
		})
		return
	}
	if len(a.snapshot.Accounts) > 0 {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(space2 * s).Children(func() {
			if button(c, "account-add-back", t.AccBackBtn, "", false, a.dialogPending()) {
				a.accountAdding = false
				a.serverEdit = nil
			}
			label(c, t.AccAddSection).FontSize(typeCallout * s).FontWeight(700)
		})
	} else {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			label(c, t.AccAddSection).FontSize(typeCallout * s).FontWeight(700).Grow(1)
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

// accountKindTag is the short type label: 正版 / 离线 / 第三方 <server>.
func (a *App) accountKindTag(ac launcher.Account) string {
	t := a.tr()
	switch ac.Kind {
	case "microsoft":
		return t.AccTagMicrosoft
	case "yggdrasil":
		return t.AccTagThird + "   " + a.accountServerName(ac.Server)
	}
	return t.AccTagOffline
}

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

// accountTypeTag is the coloured pill (icon + 正版/离线/第三方 + server).
func (a *App) accountTypeTag(c *ui.Context, ac launcher.Account) {
	s := uiScale(c)
	col := accountTypeColor(ac.Kind)
	kind := ac.Kind
	if kind != "microsoft" && kind != "yggdrasil" {
		kind = "offline"
	}
	tagText := a.accountKindTag(ac)
	ui.Row(c).Label("类型标签 "+kind).Shrink(1).MinWidth(0).Padding(2*s, 8*s, 2*s, 6*s).Gap(4*s).Radius(999).Background(col.Alpha(.12)).Border(1, col.Alpha(.22)).AlignItems(ui.Center).Children(func() {
		accountTypeIconL(c, a, "类型图标 ", kind, ac.Server, 12*s, col)
		ui.Text(c, midEllipsis(c, tagText, 240*s, typeCaption2*s, 600)).Label(tagText).FontSize(typeCaption2 * s).FontWeight(600).TextColor(col).Shrink(1).MinWidth(0).NoWrap().Tooltip(ac.Server)
	})
}

// currentTag marks the active account; same pill as the kind tag, solid accent.
func currentTag(c *ui.Context, text string) {
	s := uiScale(c)
	ui.Row(c).Label("当前账号标签").Shrink(0).Padding(2*s, 8*s).Radius(999).Background(accent).AlignItems(ui.Center).Children(func() {
		ui.Text(c, text).FontSize(typeCaption2 * s).FontWeight(700).TextColor(panel).NoWrap()
	})
}

// accountList renders saved accounts as compact rows in a scroll area.
func (a *App) accountList(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
		label(c, t.AccSavedSection).FontSize(typeCallout * s).FontWeight(700).Grow(1)
		label(c, fmt.Sprint(len(a.snapshot.Accounts))).FontSize(typeCallout * s)
	})
	scrollArea(c, "account-list-scroll").Label("账号列表").FillWidth().Margin(0, -scrollGutter*s, 0, 0).Padding(0, scrollGutter*s, 0, 0).MaxHeight(360 * s).Children(func() {
		ui.Column(c).FillWidth().Gap(9 * s).Children(func() {
			for _, account := range a.snapshot.Accounts {
				ac := account
				active := ac.ID == a.snapshot.ActiveAccount
				bg, edge := ui.RGBA(255, 255, 255, .7), line
				if active {
					bg, edge = accentSoft, accent.Alpha(.30)
				}
				ui.Row(c.Key("ac-"+ac.ID)).Label("账号行 "+ac.ID).FillWidth().Height(54*s).Padding(0, 14*s).Radius(radiusRow*s).Background(bg).Border(1, edge).Gap(12 * s).AlignItems(ui.Center).Children(func() {
					a.headAvatar(c, &ac, 30*s, func() {
						ui.Box(c).Size(30*s, 30*s).Radius(7.5*s).Background(panel).Border(1, line).Center().Shrink(0).Children(func() {
							ui.Icon(c, icons["account"]).Size(15*s, 15*s).TextColor(accent)
						})
					})
					ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(3 * s).Children(func() {
						fg := ink
						if active {
							fg = accent
						}
						ui.Text(c, ac.Name).FillWidth().MinWidth(0).FontSize(typeBody * s).FontWeight(700).TextColor(fg).SingleLine().Tooltip(ac.Name)
						ui.Row(c).FillWidth().MinWidth(0).Gap(6 * s).AlignItems(ui.Center).Children(func() {
							a.accountTypeTag(c, ac)
							if active {
								currentTag(c, t.HomeBadgeActive)
							}
						})
					})
					if !active && fixedButton(c, "use-account-"+ac.ID, t.AccSetCurrentBtn, "", 0, btnSm*s, false, a.dialogPending()) {
						a.execute("select", map[string]string{"accountId": ac.ID}, func(json.RawMessage) {
							if a.page == "wardrobe" {
								a.loadAppearance()
							}
						})
					}
					ui.Box(c).Width(space1 * s).Shrink(0)
					if a.confirmRemove != ac.ID {
						if iconOnlyButton(c, "delete-account-"+ac.ID, "cross", t.AccRemoveBtn, btnSm*s, a.dialogPending()) {
							a.confirmRemove = ac.ID // second click removes; anything else cancels
						}
					} else if dangerButton(c, "confirm-delete-account-"+ac.ID, t.AccRemoveConfirm, btnSm*s, a.dialogPending()) {
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
			b := ui.ButtonBase(c.Key("auth-" + tab.label)).Label(tab.label).Grow(1).Basis(0).MinWidth(0).Height(31 * s).Radius(8.5 * s).AlignItems(ui.Center).Justify(ui.Center).Gap(7 * s).Disabled(a.dialogPending())
			hover := ui.Transparent
			if !active && b.Hovered() {
				hover = ui.RGBA(255, 255, 255, .45)
			}
			b.Background(b.AnimateColor("bg", hover, fastAnim))
			if b.Children(func() {
				accountTypeIcon(c, tab.icon, 14*s, mixColor(fg, accountTypeColor(tab.icon), near))
				ui.Text(c, tab.label).FontSize(typeBody * s).FontWeight(600).TextColor(fg).NoWrap()
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
				if buttonWhy(c, "offline-login", t.AccOfflineAddBtn, "account", true, whyOf(a.dialogPending(), t.WhyBusy, strings.TrimSpace(a.form.Username) == "", t.WhyNeedName), true) {
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
					ui.Text(c, t.AccMSDisabled1).FontSize(typeCallout * s).FontWeight(600).TextColor(inkSecondary)
					label(c, t.AccMSDisabled2).FontSize(typeCallout * s)
				})
			} else if a.device == nil {
				ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
					if button(c, "start-microsoft", t.AccMSStartBtn, "account", true, a.dialogPending()) {
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
					ui.Text(c, a.device.UserCode).FontSize(typeHero * s).Bold().TextColor(accent).NoWrap()
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
				textField(c, &a.form.Password).Label(t.AccThirdPassLabel).Placeholder(t.AccThirdPassHint).Password().FillWidth()
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
				if buttonWhy(c, "third-login", t.AccThirdLoginBtn, "account", true, whyOf(a.dialogPending(), t.WhyBusy, a.form.Server == "" || a.form.Username == "" || a.form.Password == "", t.WhyNeedLogin), true) {
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
	if a.device == nil || a.polling || a.pendingOp("startMicrosoft") || time.Now().Before(a.nextDevicePoll) {
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
				a.message = a.friendlyErr(e)
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

// accountTabAnim is the ease-out curve shared by the login-mode thumb and
// the form crossfade.
var accountTabAnim = motionMove

// Account type colours: 正版 green, 离线 slate, 第三方 indigo.
var (
	accountTagMS      = ui.Hex("#1f9d55")
	accountTagOffline = ui.Hex("#5b6474")
	accountTagThird   = ui.Hex("#5b4fd6")
)
