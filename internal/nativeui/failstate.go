package nativeui

import (
	"errors"
	"regexp"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Failed-state sizes. Each keeps a fixed height so retrying never shifts layout.
const (
	failInline = iota
	failCard
	failPanel
)

var hostRe = regexp.MustCompile(`(?i)\b[a-z0-9-]+(\.[a-z0-9-]+)*\.(com|net|org|io|dev|cn|gg|me|app|cc)\b`)

// looksNetwork reports whether raw error text came from a remote request.
func looksNetwork(low string) bool {
	if hostRe.MatchString(low) {
		return true
	}
	for _, k := range []string{"http", "://", "no such host", "dial tcp", "connection refused", "connection reset", "network", "tls:", "x509:", "eof", "unexpected status", "status code", "status 4", "status 5", "proxy", "dns", "i/o timeout", "timeout", "timed out", "deadline exceeded", "too many requests", "unreachable", "read tcp", "write tcp", "wsarecv", "wsasend", "forcibly closed"} {
		if strings.Contains(low, k) {
			return true
		}
	}
	return false
}

// loadFailed is the one look for anything that could not be fetched:
// "加载失败" and a 重试 that spins while busy. Returns true on retry click.
func (a *App) loadFailed(c *ui.Context, key string, size int, busy bool) bool {
	s := uiScale(c)
	label := a.L("加载失败", "Failed to load")
	retry := a.L("重试", "Retry")
	clicked := false
	btn := func() {
		if busy {
			ui.Row(c).Height(btnSm * s).Gap(6 * s).PaddingX(10 * s).AlignItems(ui.Center).Children(func() {
				statusSpinner(c, 12*s)
				ui.Text(c, retry).FontSize(typeFootnote * s).TextColor(muted).NoWrap()
			})
			return
		}
		clicked = fixedButton(c, key+"-retry", retry, "refresh", 0, btnSm*s, false, false)
	}
	switch size {
	case failPanel:
		ui.Column(c.Key(key)).FillWidth().Height(220 * s).Gap(10 * s).Center().Children(func() {
			ui.Icon(c, icons["info"]).Size(22*s, 22*s).TextColor(muted)
			ui.Text(c, label).FontSize(typeCallout * s).FontWeight(600).TextColor(inkSecondary)
			btn()
		})
	case failCard:
		ui.Row(c.Key(key)).FillWidth().Height(52*s).Padding(0, 14*s).Gap(10 * s).Radius(10 * s).Background(ui.RGBA(120, 130, 160, .08)).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons["info"]).Size(14*s, 14*s).TextColor(muted).Shrink(0)
			ui.Text(c, label).FontSize(typeFootnote * s).TextColor(inkSecondary).Grow(1).MinWidth(0)
			btn()
		})
	default:
		ui.Row(c.Key(key)).Height(btnSm * s).Gap(8 * s).AlignItems(ui.Center).Children(func() {
			ui.Text(c, label).FontSize(typeFootnote * s).TextColor(inkSecondary).NoWrap()
			btn()
		})
	}
	return clicked
}

var errStatus = errors.New("unexpected status")
