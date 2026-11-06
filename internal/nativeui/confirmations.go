package nativeui

import "github.com/egoist/mygo/ui"

type confirmationState struct {
	key, title, detail, action string
	confirm                    func()
	extra                      func(*ui.Context)
}

// confirmAction uses the existing modal surface for actions affecting a whole
// running game, several files, or an unsaved draft. Single-item confirmations
// use popconfirm instead, attached to their original button.
func (a *App) confirmAction(key, title, detail, action string, confirm func()) {
	if a.confirmation == nil {
		a.popconfirmKey = ""
		a.confirmation = &confirmationState{key: key, title: title, detail: detail, action: action, confirm: confirm}
		if a.win != nil {
			a.win.Update(func() {})
		}
	}
}

func (a *App) confirmationView(c *ui.Context) {
	req := a.confirmation
	if req == nil {
		return
	}
	s, open := uiScale(c), true
	_, height := c.Size()
	ui.DialogBase(c.Key("confirmation-"+req.key), &open, func(backdrop, box ui.Element) {
		backdrop.Background(ui.RGBA(20, 24, 48, .30))
		dialogSurface(box.Lang(a.lang()).Label(req.title).Width(520*s).MaxWidthPercent(94).MaxHeight(height-42-24*s).Padding(24*s).Focusable().AutoFocus().FocusRing(false), s)
		ui.Column(c).FillWidth().Gap(16 * s).Children(func() {
			ui.Text(c, req.title).FontSize(typeTitle2 * s).Bold().TextColor(ink).FillWidth()
			scrollArea(c, "confirmation-detail").FillWidth().MaxHeight(max(100*s, height-220*s)).Children(func() { label(c, req.detail).FillWidth().MinWidth(0) })
			if req.extra != nil {
				req.extra(c)
			}
			ui.Row(c).FillWidth().Gap(12 * s).Justify(ui.End).Children(func() {
				if button(c, "confirmation-cancel", a.L("install_dialog.cancel"), "", false, false) {
					a.confirmation = nil
				}
				if button(c, "confirmation-accept", req.action, "", true, false) {
					a.confirmation = nil
					req.confirm()
				}
			})
		})
	})
	if !open {
		a.confirmation = nil
	}
}

// Popconfirm is deliberately built from PopoverBase: outside presses and Escape
// dismiss only this confirmation, and the toolkit restores its opener's focus.
func (a *App) popconfirm(c *ui.Context, anchor ui.Element, key string, clicked, disabled bool, title, detail, action string, confirm func()) {
	if clicked && !disabled {
		if a.popconfirmKey == key {
			a.popconfirmKey = ""
		} else {
			a.popconfirmKey = key
		}
	}
	if a.popconfirmKey != key {
		return
	}
	if disabled || a.confirmation != nil {
		a.popconfirmKey = ""
		return
	}
	s, open := uiScale(c), true
	close := func() { a.popconfirmKey = ""; anchor.Focus() }
	ui.PopoverBase(c.Key("popconfirm-"+key), anchor, &open, func(p ui.Element) {
		popover(p.Lang(a.lang()).Label(title).Width(336*s).MaxWidthPercent(94).Margin(8*s, 0, 0, 0).Padding(16*s).Focusable().AutoFocus().FocusRing(false), radiusPanel*s, s)
		if !c.Preferences().ReduceMotion {
			origin := float32(0)
			position, target := p.LayoutBounds(), anchor.LayoutBounds()
			originX := float32(0)
			if position.W > 0 {
				originX = max(0, min(1, (target.X+target.W/2-position.X)/position.W))
			}
			if position.Y < target.Y {
				origin = 1
			}
			p.TransformOrigin(originX, origin).Transition(ui.ElementTransition{Animation: &motionDialogEnter, Colors: true,
				Enter: &ui.Motion{Opacity: 0, Scale: .98}, Exit: &ui.Motion{Opacity: 0, Scale: .985}})
		}
		ui.Column(c).FillWidth().Gap(12 * s).Children(func() {
			ui.Text(c, title).FontSize(typeSubhead * s).Bold().TextColor(ink).FillWidth()
			label(c, detail).FontSize(typeFootnote * s).FillWidth()
			ui.Row(c).FillWidth().Gap(8 * s).Justify(ui.End).Children(func() {
				if button(c, "popconfirm-cancel-"+key, a.L("install_dialog.cancel"), "", false, false) {
					close()
				}
				if button(c, "popconfirm-accept-"+key, action, "", true, false) {
					close()
					confirm()
				}
			})
		})
	})
	if !open {
		close()
	}
}

func (a *App) confirmIcon(c *ui.Context, key, icon, action, title, detail string, height float32, disabled bool, confirm func()) {
	b := iconButtonElement(c, key, icon, action, height, disabled)
	a.popconfirm(c, b, key, b.Clicked(), disabled, title, detail, action, confirm)
}

func (a *App) confirmFixed(c *ui.Context, key, action, icon, title, detail string, width, height float32, disabled bool, confirm func(), confirmLabel ...string) {
	b, clicked := fixedButtonElement(c, key, action, icon, width, height, buttonSecondary, disabled)
	if len(confirmLabel) > 0 {
		action = confirmLabel[0]
	}
	a.popconfirm(c, b, key, clicked, disabled, title, detail, action, confirm)
}

func (a *App) requestStop(rootID, sessionID, name string) {
	if a.confirmation != nil {
		return
	}
	ids, labels := []string{}, map[string]string{}
	selected := sessionID
	if sessionID == "" && a.service != nil {
		sessions := a.service.Sessions("")
		for _, session := range sessions {
			if session.RootID != rootID || session.State != "running" {
				continue
			}
			ids = append(ids, session.ID)
			labels[session.ID] = a.sessionLabel(session, sessions)
			if selected == "" || a.root() != nil && session.Version == a.root().SelectedVersion {
				selected = session.ID
			}
		}
		if len(ids) == 1 {
			name += "\n" + labels[ids[0]]
		}
	}
	a.confirmAction("stop-game", a.tr().HomeStopBtn, name+"\n"+a.L("floating.stop_hint"), a.L("floating.stop_action"), func() {
		a.execute("stopSession", map[string]string{"id": selected}, nil)
	})
	if len(ids) > 1 {
		a.confirmation.extra = func(c *ui.Context) {
			selectBoxFull(c, "stop-session-choice", a.L("floating.stop_target"), &selected, ids, nil, func(id string) string { return labels[id] }).FillWidth()
		}
	}
}

func (a *App) requestPackRollback(rootID, backupID, name string) {
	a.confirmAction("rollback-pack", a.tr().PackRollbackPrefix+name, name+"\n"+a.L("floating.rollback_hint"), a.L("floating.rollback_action"), func() {
		a.execute("rollbackPack", map[string]string{"rootId": rootID, "id": backupID}, a.taskAdded)
	})
}
