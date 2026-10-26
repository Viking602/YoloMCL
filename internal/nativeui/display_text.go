package nativeui

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/i18n"
)

func loaderName(s string) string {
	switch s {
	case "fabric":
		return "Fabric"
	case "quilt":
		return "Quilt"
	case "forge":
		return "Forge"
	case "neoforge":
		return "NeoForge"
	}
	return "原版"
}

func loaderKey(s string) string {
	if key := i18n.Canonical("loader", s); key != "" {
		return key
	}
	v := strings.ToLower(strings.TrimSpace(s))
	switch v {
	case "", "vanilla", "原版", "all", "all loaders", "全部加载器":
		return "vanilla"
	case "fabric":
		return "fabric"
	case "quilt":
		return "quilt"
	case "forge":
		return "forge"
	case "neoforge":
		return "neoforge"
	}
	return v
}

func compact(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprint(n)
}

func sizeText(n int64) string {
	switch {
	case n <= 0:
		return ""
	case n >= 1<<30:
		return fmtSize(n)
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.0f KB", float64(n)/1024)
	case n < 100*1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
	return fmt.Sprintf("%.0f MB", float64(n)/(1024*1024))
}

func cleanName(s string) string {
	for _, r := range `<>:"/\|?*` {
		s = strings.ReplaceAll(s, string(r), "-")
	}
	return strings.TrimSpace(s)
}

var javaWord = regexp.MustCompile(`(?i)\bjava\b`)

// javaDisplay capitalises the word "java" in display-only text such as
// runtime folder names ("java-runtime-delta") or vendor strings. Never use
// it on paths or stored values.
func javaDisplay(s string) string { return javaWord.ReplaceAllString(s, "Java") }

// updatedAgo is shared by resource cards and individual releases.
func updatedAgo(en bool, t, now time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := now.Sub(t)
	format := func(unit string, n int) string { return fmt.Sprintf(i18n.Plural(en, "time.ago."+unit, n), n) }
	days := int(d.Hours() / 24)
	switch {
	case d < time.Minute:
		return i18n.Text(en, "time.ago.now")
	case d < time.Hour:
		return format("minute", int(d/time.Minute))
	case d < 24*time.Hour:
		return format("hour", int(d/time.Hour))
	case days < 2:
		return i18n.Text(en, "time.ago.yesterday")
	case days < 7:
		return format("day", days)
	case days < 30:
		return format("week", days/7)
	case days < 365:
		return format("month", days/30)
	}
	return format("year", days/365)
}

// Refresh only when the displayed time can change, including while idle.
// API timestamps keep their offset for comparison; hover details use local time.
func (a *App) resourceTime(c *ui.Context, raw string) (relative, exact string) {
	at, err := time.Parse(time.RFC3339, raw)
	if err != nil || at.IsZero() {
		return "", ""
	}
	now := c.Now()
	elapsed := max(now.Sub(at), 0)
	step := 24 * time.Hour
	if elapsed < time.Hour {
		step = time.Minute
	} else if elapsed < 24*time.Hour {
		step = time.Hour
	}
	c.After(step - elapsed%step)
	return updatedAgo(a.isEN(), at, now), at.Local().Format("2006-01-02 15:04:05")
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
