package nativeui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"
)

// noticeTone is a notice type's Lobe Alert colours, Lucide icon and spoken name.
func (a *App) noticeTone(kind string) (fg, fill ui.Color, icon *ui.SVG, name string) {
	t := a.tr()
	switch kind {
	case "error":
		return danger, lobetheme.Mix(panel, danger, .06), lucide.CircleX(), t.NoticeError
	case "warning":
		return warningInk, lobetheme.Mix(panel, warning, .07), lucide.TriangleAlert(), t.NoticeWarning
	}
	return accent, lobetheme.Mix(panel, accent, .05), lucide.Info(), t.NoticeInfo
}

// noticeBody is the shared Lobe Alert-styled notice: Lucide tone icon, message, actions on the right.
func (a *App) noticeBody(c *ui.Context, kind, msg, detail string, actions func()) ui.Element {
	s := uiScale(c)
	fg, fill, icon, name := a.noticeTone(kind)
	row := ui.Row(c).Label(name+"通知").FillWidth().Padding(space3*s, space4*s).Radius(radiusRow*s).Background(fill).Border(1, fg.Alpha(.20)).Gap(space3 * s).AlignItems(ui.Center)
	return row.Children(func() {
		ui.Icon(c, icon).Size(18*s, 18*s).TextColor(fg).Shrink(0)
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(3 * s).Children(func() {
			ui.Text(c, msg).FontSize(typeCallout * s).FontWeight(550).TextColor(ink).Wrap()
			if detail != "" && detail != msg {
				ui.Text(c, detail).FontSize(typeFootnote * s).TextColor(inkSecondary).Wrap()
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

// lobeAlert renders a scaled Lobe UI soft Alert callout matching feedback.Alert's
// visual tokens (tinted container mix + 1px accent ring + Lucide status icon)
// while scaling cleanly with uiScale(c) down to 900×540 windows.
func lobeAlert(c *ui.Context, kind, text string) {
	s := uiScale(c)
	fg, icon := accent, lucide.Info()
	switch kind {
	case "success":
		fg, icon = successInk, lucide.CircleCheck()
	case "warning":
		fg, icon = warningInk, lucide.TriangleAlert()
	case "error":
		fg, icon = danger, lucide.CircleX()
	}
	fill := lobetheme.Mix(panel, fg, .05)
	ui.Row(c).FillWidth().Padding(10*s, 12*s).Gap(10*s).Radius(radiusControl*s).
		Background(fill).InsetShadow(0, 0, 0, 1, fg.Alpha(.18)).AlignItems(ui.Start).Children(func() {
		ui.Box(c).Height(18 * s).AlignItems(ui.Center).Justify(ui.Center).Shrink(0).Children(func() {
			ui.Icon(c, icon).Size(15*s, 15*s).TextColor(fg)
		})
		bodyText(c, text).FontSize(typeFootnote * s).TextColor(inkSecondary).Grow(1).MinWidth(0)
	})
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
	undo := a.noticeUndo
	hasUndo := undo != nil && undo.message == a.message
	if a.message != "" && (a.dialog == "" || a.surfaceKind(a.dialog) == surfacePage) && a.confirmation == nil {
		target, anim = 1, motionEnter
	}
	p := host.AnimateTo("toast-presence", target, anim)
	if p <= .01 || a.noticeShown == "" {
		if a.message == "" {
			a.noticeShown, a.noticeCopied = "", false
			a.toastUntil = time.Time{}
		}
		host.Height(0)
		return
	}
	msg, kind := a.noticeShown, a.noticeShownKind
	// Ordinary info notices leave after 4 s; Undo stays until used or
	// dismissed. Hover and keyboard focus pause an ordinary notice's clock.
	if kind == "info" && a.message != "" && !hasUndo {
		if host.Hovered() || host.FocusWithin() {
			a.toastHeld = true
		} else if a.toastHeld {
			a.toastHeld = false
			a.toastGen++
		}
		key := fmt.Sprintf("toast-life-%d-%s", a.toastGen, msg)
		if a.toastHeld {
			a.toastUntil = time.Time{}
		} else {
			if a.toastLifeKey != key || a.toastUntil.IsZero() {
				a.toastLifeKey, a.toastUntil = key, c.Now().Add(toastLife)
			}
			// Only the visible entrance/exit animates. The four-second hold
			// needs one wakeup, not a full view rebuild on every display frame.
			remaining := a.toastUntil.Sub(c.Now())
			if remaining <= 0 {
				a.message, a.noticeKind = "", ""
			}
			c.After(max(0, remaining))
		}
	}
	host.Opacity(p).Translate(0, (1-p)*10*s).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(860 * s).AlignSelf(ui.Center).Children(func() {
			elevate2(a.noticeBody(c, kind, msg, "", func() {
				if hasUndo && button(c, "notice-undo", a.L("floating.undo"), "refresh", false, undo.pending) {
					a.undoRemoval()
				}
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
