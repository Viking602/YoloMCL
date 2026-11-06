package nativeui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Modal feedback stays inside its focus scope. A global toast above a modal
// would be visible but unavailable to keyboard and assistive-technology users.
func (a *App) dialogFeedback(c *ui.Context) {
	u := a.noticeUndo
	if u != nil && u.message == a.message && a.surfaceKind(a.dialog) != surfacePage {
		a.toastUntil = time.Time{}
		a.noticeBody(c, "info", a.message, "", func() {
			if button(c, "inline-notice-undo", a.L("floating.undo"), "refresh", false, u.pending) {
				a.undoRemoval()
			}
			if button(c, "inline-notice-dismiss", a.tr().DismissNotice, "cross", false, false) {
				a.message = ""
			}
		})
	} else if a.message != "" && a.errorMessage {
		a.noticeBody(c, "error", a.message, "", func() {
			if a.noticeFor == a.message && a.noticeRetry != nil && button(c, "inline-notice-retry", a.tr().NoticeRetry, "refresh", false, false) {
				retry := a.noticeRetry
				a.message, a.noticeRetry = "", nil
				retry()
			}
		})
	}
}
