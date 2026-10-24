package nativeui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

// noticeTone is a notice type's colours, icon and spoken name: the type is
// told by icon and label as well as colour.
func (a *App) noticeTone(kind string) (fg, fill ui.Color, icon, name string) {
	t := a.tr()
	switch kind {
	case "error":
		return danger, ui.Hex("#fff1f0"), "cross", t.NoticeError
	case "warning":
		return warningInk, ui.Hex("#fff7e6"), "info", t.NoticeWarning
	}
	return accent, ui.Hex("#eef5ff"), "info", t.NoticeInfo
}

// noticeBody is the shared notice: icon badge, message, actions on the right.
func (a *App) noticeBody(c *ui.Context, kind, msg, detail string, actions func()) ui.Element {
	s := uiScale(c)
	fg, fill, icon, name := a.noticeTone(kind)
	row := ui.Row(c).Label(name+"通知").FillWidth().Padding(space2*s+2*s, space3*s, space2*s+2*s, space3*s).Radius(radiusRow*s).Background(fill).Border(1, fg.Alpha(.22)).Gap(space3 * s).AlignItems(ui.Center)
	return row.Children(func() {
		ui.Box(c).Size(22*s, 22*s).Radius(11 * s).Background(fg).Center().Shrink(0).Children(func() {
			ui.Icon(c, icons[icon]).Size(12*s, 12*s).TextColor(panel)
		})
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(2 * s).Children(func() {
			ui.Text(c, msg).FontSize(typeCallout * s).FontWeight(600).TextColor(ink).Wrap()
			if detail != "" && detail != msg {
				ui.Text(c, detail).FontSize(typeFootnote * s).TextColor(muted).Wrap()
			}
		})
		if actions != nil {
			ui.Row(c).Gap(space2 * s).Shrink(0).AlignItems(ui.Center).Children(actions)
		}
	})
}

// inlineNotice shows a notice in place, where the problem is.
func (a *App) inlineNotice(c *ui.Context, kind, msg, detail string) {
	a.noticeBody(c, kind, msg, detail, nil)
}

// toast is the page notice: it floats over the bottom of the page, above the launch bar (never
// pushing the layout) and eases in and out with the motion system. Errors
// offer 重试 when the failed action can run again, 查看日志 and 复制错误.
func (a *App) toast(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	if a.message != "" {
		a.noticeShown = a.message
		a.noticeShownKind = a.noticeKind
		if a.noticeShownKind == "" {
			a.noticeShownKind = "info"
			if a.errorMessage {
				a.noticeShownKind = "error"
			}
		}
	}
	if a.noticeFor != a.message {
		a.noticeRetry, a.noticeFor = nil, ""
	}
	bottom := space3 * s
	host := ui.Box(c.Key("toast-host")).Label("页面通知").Absolute().Left(pageGutter * s).Right(pageGutter * s).Bottom(bottom)
	target, anim := float32(0), motionExit
	if a.message != "" && a.dialog == "" {
		target, anim = 1, motionEnter
	}
	p := host.AnimateTo("toast-presence", target, anim)
	if p <= .01 || a.noticeShown == "" {
		if a.message == "" {
			a.noticeShown, a.noticeCopied = "", false
		}
		host.Height(0)
		return
	}
	msg, kind := a.noticeShown, a.noticeShownKind
	// Info notices leave by themselves after about 4 s; hovering holds
	// them (the countdown restarts when the pointer leaves).
	if kind == "info" && a.message != "" {
		if host.Hovered() {
			a.toastHeld = true
		} else if a.toastHeld {
			a.toastHeld = false
			a.toastGen++
		}
		key := fmt.Sprintf("toast-life-%d-%s", a.toastGen, msg)
		if a.toastHeld {
			// paused
		} else if a.toastLifeKey != key {
			a.toastLifeKey = key
			host.AnimateTo(key, 0, ui.Animation{})
		} else if host.AnimateTo(key, 1, ui.Animation{Duration: toastLife, Ease: ui.Linear}) >= 1 {
			a.message, a.noticeKind = "", ""
		}
	}
	host.Opacity(p).Translate(0, (1-p)*10*s).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(860 * s).AlignSelf(ui.Center).Children(func() {
			elevate2(a.noticeBody(c, kind, msg, "", func() {
				if kind == "error" {
					if a.noticeRetry != nil && button(c, "notice-retry", t.NoticeRetry, "refresh", false, false) {
						retry := a.noticeRetry
						a.message, a.noticeRetry = "", nil
						retry()
					}
					if button(c, "notice-logs", t.NoticeLogs, "terminal", false, false) {
						a.message = ""
						a.openDialog("logs")
					}
					copyText := t.NoticeCopy
					if a.noticeCopied {
						copyText = t.NoticeCopied
					}
					if button(c, "notice-copy", copyText, "check", false, false) {
						c.WriteClipboard(msg)
						a.noticeCopied = true
					}
				}
				if button(c, "dismiss-msg", t.DismissNotice, "cross", false, false) {
					a.message, a.noticeKind, a.noticeCopied = "", "", false
				}
			}), s)
		})
	})
}

const toastLife = 4 * time.Second
