package nativeui

import (
	"fmt"
	"strings"
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
