package nativeui

import (
	"github.com/egoist/mygo/ui"
)

func (a *App) accountEntry(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	name := t.SideAccountAdd
	if ac := a.account(); ac != nil {
		name = ac.Name
	}
	tip := t.SideAccountManage
	if ac := a.account(); ac != nil {
		tip = ac.Name + "   " + a.accountKindTag(*ac)
	}
	b := tooltip(c, ui.ButtonBase(c.Key("account-menu")).Label(t.SideAccountManage), tip).Height(btnSm*s).PaddingX(8*s).Gap(7*s).AlignItems(ui.Center).Radius(radiusControl*s).Border(1, lineStrong).Shrink(0)
	bg := panel
	if a.accountMenuOpen {
		bg = raised
	} else if b.Hovered() {
		bg = raised
	}
	b.Background(b.AnimateColor("bg", bg, fastAnim)).Shadow(0, s, 2*s, 0, ui.RGBA(0, 0, 0, .03))
	if b.Children(func() {
		a.headAvatar(c, a.account(), 20*s, func() {
			ui.Box(c).Size(20*s, 20*s).Radius(radiusMin * s).Background(lobePrimary).Center().Children(func() {
				ui.Icon(c, icons["account"]).Size(12*s, 12*s).TextColor(lobeLight.PrimaryForeground)
			})
		})
		ui.Text(c, midEllipsis(c, name, 130*s, typeFootnote*s, 600)).FontSize(typeFootnote * s).FontWeight(600).TextColor(ink).NoWrap()
		if ac := a.account(); ac != nil {
			kind := ac.Kind
			if kind != "microsoft" && kind != "yggdrasil" {
				kind = "offline"
			}
			col := accountTypeColor(kind)
			ui.Box(c).Size(18*s, 18*s).Radius(radiusMin * s).Background(col.Alpha(.10)).Center().Shrink(0).Children(func() {
				accountTypeIconL(c, a, "当前账号类型 ", kind, ac.Server, 11*s, col)
			})
		}
		angle := float32(0)
		if a.accountMenuOpen {
			angle = 180
		}
		if c.Preferences().ReduceMotion {
			angle = b.Jump("menu-arrow", angle)
		} else {
			angle = b.AnimateTo("menu-arrow", angle, motionFast)
		}
		ui.Icon(c, icons["chevron-down"]).Label("账号菜单箭头").Size(12*s, 12*s).TextColor(muted).Rotate(angle)
	}).Clicked() {
		if len(a.snapshot.Accounts) == 0 {
			a.openDialog("accountAdd")
		} else {
			a.accountMenuOpen = !a.accountMenuOpen
		}
	}
	a.accountMenu(c, b)
}

// accountMenu is the quick switcher under the top-bar account chip: every
// account, one click to switch, then the manager and add entries. A click
// outside or Escape closes it.
func (a *App) accountMenu(c *ui.Context, anchor ui.Element) {
	s := uiScale(c)
	t := a.tr()
	// Keep the overlay parent alive while closed, so the first opening can
	// enter and the toolkit can finish an inert exit without retaining focus.
	ui.Overlay(c, func() {})
	// A closed PopoverBase builds nothing and leaves Context.Key pending.
	// Only assign the key when the menu is built, or the next page/control
	// inherits it and is recreated each time the menu opens or closes.
	if !a.accountMenuOpen {
		return
	}
	ui.PopoverBase(c.Key("account-menu-pop"), anchor, &a.accountMenuOpen, func(pop ui.Element) {
		popover(pop.Label("账号快速切换").AttachTo(anchor, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(6*s, 0, 0, 0).Width(264*s).MaxWidthPercent(40).Padding(8*s), radiusPanel*s, s)
		if !c.Preferences().ReduceMotion {
			// A small expansion starts at the account chip's corner; the
			// toolkit keeps interrupted entrances and inert exits continuous.
			pop.TransformOrigin(1, 0).Transition(ui.ElementTransition{Animation: &motionDialogEnter, Colors: true,
				Enter: &ui.Motion{Opacity: 0, Scale: .98}, Exit: &ui.Motion{Opacity: 0, Scale: .985}})
		}
		ui.Column(c).FillWidth().Gap(4 * s).Children(func() {
			cur := a.account()
			scrollArea(c, "account-menu-scroll").FillWidth().MaxHeight(320 * s).Gap(4 * s).Children(func() {
				for _, acc := range a.snapshot.Accounts {
					ac := acc
					active := cur != nil && cur.ID == ac.ID
					row := ui.ButtonBase(c.Key("qs-" + ac.ID)).Label("切换到 " + ac.Name).FillWidth().Height(44 * s).PaddingX(8 * s).Gap(12 * s).Radius(radiusControl * s).AlignItems(ui.Center)
					bg := ui.Transparent
					if active {
						bg = lobeLight.FillSecondary
					} else if row.Hovered() {
						bg = lobeLight.FillTertiary
					}
					row.Background(bg)
					if row.Children(func() {
						a.headAvatar(c, &ac, 26*s, func() {
							ui.Box(c).Size(26*s, 26*s).Radius(radiusChip*s).Background(raised).Border(1, line).Center().Shrink(0).Children(func() {
								ui.Icon(c, icons["account"]).Size(13*s, 13*s).TextColor(inkSecondary)
							})
						})
						fg := ink
						tooltip(c, ui.Text(c, ac.Name).FontSize(typeCallout*s).FontWeight(600).TextColor(fg).SingleLine().Grow(1).MinWidth(0), ac.Name)
						a.accountTypeTag(c, ac)
						if active {
							ui.Icon(c, icons["check"]).Size(14*s, 14*s).TextColor(lobePrimary).Shrink(0)
						} else {
							ui.Box(c).Size(14*s, 14*s).Shrink(0)
						}
					}).Clicked() && !a.pendingOp("select") {
						a.accountMenuOpen = false
						if !active {
							a.selectAccount(ac.ID)
						}
					}
				}
			})
			ui.Box(c).FillWidth().Height(1).Margin(4*s, 0).Background(line)
			// Footer rows share the account rows' geometry: same height, hover,
			// a 26-DIP icon column, so the text lines up with the names.
			item := func(key, text, icon string) bool {
				b := ui.ButtonBase(c.Key(key)).Label(text).FillWidth().Height(44 * s).PaddingX(8 * s).Radius(radiusControl * s)
				if b.Hovered() {
					b.Background(lobeLight.FillTertiary)
				}
				return b.Children(func() {
					ui.Row(c).FillWidth().FillHeight().Gap(12 * s).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(26*s, 26*s).Center().Shrink(0).Children(func() {
							ui.Icon(c, icons[icon]).Size(14*s, 14*s).TextColor(inkSecondary)
						})
						ui.Text(c, text).FontSize(typeCallout * s).TextColor(ink).SingleLine().Grow(1).MinWidth(0)
					})
				}).Clicked()
			}
			if item("qs-manage", a.L("account_menu.manage_accounts"), "account") {
				a.accountMenuOpen = false
				a.openDialog("account")
			}
			if item("qs-add", t.AccAddAccountBtn, "plus") {
				a.accountMenuOpen = false
				a.openDialog("accountAdd")
			}
		})
	})
}
