package nativeui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// uiSansFamily is the platform's proportional UI face, for dialogs and
// numbers where the bundled monospace reads as code.
func uiSansFamily() string {
	switch titlebarOS() {
	case "windows":
		return "Segoe UI Variable Text, Segoe UI, Microsoft YaHei UI, Microsoft YaHei, system-ui"
	case "darwin":
		return "SF Pro Text, PingFang SC, Hiragino Sans GB, system-ui"
	}
	return "Noto Sans, Noto Sans CJK SC, system-ui"
}

// numText is a number or date in the UI face with tabular figures.
func numText(c *ui.Context, text string, size float32, color ui.Color) ui.Element {
	return ui.Text(c, text).Font(uiSansFamily()).FontFeatures("tnum").FontSize(size).TextColor(color).NoWrap()
}

// updatedAgo is the one relative format for "updated" dates.
func updatedAgo(en bool, t, now time.Time) string {
	d := now.Sub(t)
	L := func(zh, e string, n int) string {
		if en {
			if n == 1 {
				return strings.Replace(e, "s ago", " ago", 1)
			}
			return fmt.Sprintf(e, n)
		}
		return fmt.Sprintf(zh, n)
	}
	days := int(d.Hours() / 24)
	switch {
	case d < 24*time.Hour:
		if en {
			return "today"
		}
		return "今天"
	case days < 2:
		if en {
			return "yesterday"
		}
		return "昨天"
	case days < 7:
		return L("%d 天前", "%d days ago", days)
	case days < 30:
		return L("%d 周前", "%d weeks ago", days/7)
	case days < 365:
		return L("%d 个月前", "%d months ago", days/30)
	}
	return L("%d 年前", "%d years ago", days/365)
}

func loaderLabel(l string) string {
	switch strings.ToLower(l) {
	case "forge":
		return "Forge"
	case "neoforge":
		return "NeoForge"
	case "fabric":
		return "Fabric"
	case "quilt":
		return "Quilt"
	case "iris":
		return "Iris"
	case "optifine":
		return "OptiFine"
	case "minecraft", "vanilla":
		return ""
	}
	if l == "" {
		return ""
	}
	return strings.ToUpper(l[:1]) + l[1:]
}

func (a *App) releaseLabel(t string) (string, ui.Color) {
	switch strings.ToLower(t) {
	case "beta":
		return "Beta", warningInk
	case "alpha":
		return "Alpha", danger
	}
	return a.L("正式", "Release"), successInk
}
