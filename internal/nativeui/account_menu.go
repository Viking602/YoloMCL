package nativeui

import (
	"encoding/json"

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
	b := ui.ButtonBase(c.Key("account-menu")).Label(t.SideAccountManage).Tooltip(tip).Height(btnSm*s).PaddingX(8*s).Gap(6*s).AlignItems(ui.Center).Radius(999).Border(1, glassEdge).Shrink(0)
	bg := ui.RGBA(255, 255, 255, .6)
	if a.accountMenuOpen || a.page == "settings" {
		bg = accentSoft
	} else if b.Hovered() {
		bg = panel
	}
	b.Background(b.AnimateColor("bg", bg, fastAnim))
	if b.Children(func() {
		a.headAvatar(c, a.account(), 22*s, func() {
			ui.Box(c).Size(22*s, 22*s).Radius(11 * s).LinearGradient(tintGradient(ui.Hex("#64d2ff"), accentIndigo)).Center().Children(func() {
				ui.Icon(c, icons["account"]).Size(12*s, 12*s).TextColor(panel)
			})
		})
		ui.Text(c, midEllipsis(c, name, 130*s, typeCallout*s, 600)).FontSize(typeCallout * s).FontWeight(600).TextColor(ink).NoWrap()
		if ac := a.account(); ac != nil {
			kind := ac.Kind
			if kind != "microsoft" && kind != "yggdrasil" {
				kind = "offline"
			}
			col := accountTypeColor(kind)
			ui.Box(c).Size(18*s, 18*s).Radius(999).Background(col.Alpha(.12)).Center().Shrink(0).Children(func() {
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
		ui.Icon(c, icons["chevron-down"]).Label("账号菜单箭头").Size(9.5*s, 9.5*s).TextColor(muted).Rotate(angle)
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
	ui.PopoverBase(c.Key("account-menu-pop"), anchor, &a.accountMenuOpen, func(pop ui.Element) {
		popover(pop.Label("账号快速切换").AttachTo(anchor, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(6*s, 0, 0, 0).Width(264*s).MaxWidthPercent(40).Padding(6*s), 14*s, s)
		if !c.Preferences().ReduceMotion {
			// A small expansion starts at the account chip's corner; the
			// toolkit keeps interrupted entrances and inert exits continuous.
			pop.TransformOrigin(1, 0).Transition(ui.ElementTransition{Animation: &motionDialogEnter, Colors: true,
				Enter: &ui.Motion{Opacity: 0, Scale: .98}, Exit: &ui.Motion{Opacity: 0, Scale: .985}})
		}
		ui.Column(c).FillWidth().Gap(2 * s).Children(func() {
			cur := a.account()
			scrollArea(c, "account-menu-scroll").FillWidth().MaxHeight(320 * s).Gap(2 * s).Children(func() {
				for _, acc := range a.snapshot.Accounts {
					ac := acc
					active := cur != nil && cur.ID == ac.ID
					row := ui.ButtonBase(c.Key("qs-" + ac.ID)).Label("切换到 " + ac.Name).FillWidth().Height(44 * s).PaddingX(8 * s).Gap(10 * s).Radius(9 * s).AlignItems(ui.Center)
					bg := ui.Transparent
					if active {
						bg = accentSoft
					} else if row.Hovered() {
						bg = ui.RGBA(120, 130, 160, .10)
					}
					row.Background(bg)
					if row.Children(func() {
						a.headAvatar(c, &ac, 26*s, func() {
							ui.Box(c).Size(26*s, 26*s).Radius(6.5*s).Background(panel).Border(1, line).Center().Shrink(0).Children(func() {
								ui.Icon(c, icons["account"]).Size(13*s, 13*s).TextColor(accent)
							})
						})
						fg := ink
						if active {
							fg = accent
						}
						ui.Text(c, ac.Name).FontSize(typeCallout * s).FontWeight(600).TextColor(fg).SingleLine().Grow(1).MinWidth(0).Tooltip(ac.Name)
						a.accountTypeTag(c, ac)
						if active {
							ui.Icon(c, icons["check"]).Size(13*s, 13*s).TextColor(accent).Shrink(0)
						} else {
							ui.Box(c).Size(13*s, 13*s).Shrink(0)
						}
					}).Clicked() && !a.pendingOp("select") {
						a.accountMenuOpen = false
						if !active {
							a.execute("select", map[string]string{"accountId": ac.ID}, func(json.RawMessage) {
								if a.page == "wardrobe" {
									a.loadAppearance()
								}
							})
						}
					}
				}
			})
			ui.Box(c).FillWidth().Height(1).Margin(4*s, 0).Background(line)
			// Footer rows share the account rows' geometry: same height, hover,
			// a 26-DIP icon column, so the text lines up with the names.
			item := func(key, text, icon string) bool {
				b := ui.ButtonBase(c.Key(key)).Label(text).FillWidth().Height(44 * s).PaddingX(8 * s).Radius(9 * s)
				if b.Hovered() {
					b.Background(ui.RGBA(120, 130, 160, .10))
				}
				return b.Children(func() {
					ui.Row(c).FillWidth().FillHeight().Gap(10 * s).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(26*s, 26*s).Center().Shrink(0).Children(func() {
							ui.Icon(c, icons[icon]).Size(14*s, 14*s).TextColor(inkSecondary)
						})
						ui.Text(c, text).FontSize(typeCallout * s).TextColor(ink).SingleLine().Grow(1).MinWidth(0)
					})
				}).Clicked()
			}
			if item("qs-manage", a.L("账号管理", "Manage accounts"), "account") {
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
