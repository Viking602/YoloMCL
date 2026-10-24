package nativeui

import (
	"path/filepath"
	"strings"

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

func label(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).TextColor(muted).FontSize(typeCallout * uiScale(c))
}

func pathLabel(c *ui.Context, path string, size float32) ui.Element {
	s := uiScale(c)
	dir, base := filepath.Split(path)
	if base == "" {
		dir, base = filepath.Split(strings.TrimRight(path, `/\`))
		base += string(filepath.Separator)
	}
	return ui.Row(c).Label("路径 " + path).Grow(1).Basis(0).MinWidth(0).Gap(0).AlignItems(ui.Center).Tooltip(path).Children(func() {
		if dir != "" {
			ui.Text(c, dir).FontSize(size).TextColor(muted).SingleLine().Shrink(1).MinWidth(0)
		}
		ui.Text(c, base).FontSize(size).FontWeight(600).TextColor(inkSecondary).SingleLine().Shrink(0).MaxWidthPercent(70)
		_ = s
	})
}

// midEllipsis shortens text to fit maxW by cutting its middle, keeping the
// start and the end ("一个非常…目录名称", "C:\Users\…\.minecraft").
func midEllipsis(c *ui.Context, text string, maxW, size float32, weight int) string {
	measure := func(t string) float32 {
		w, _ := c.MeasureText(0, ui.Span{Text: t, Font: theme.Font, Size: size, Weight: weight})
		return w
	}
	if measure(text) <= maxW {
		return text
	}
	r := []rune(text)
	lo, hi := 0, len(r)
	for lo < hi { // largest keep count that fits
		mid := (lo + hi + 1) / 2
		head := (mid + 1) / 2
		cand := string(r[:head]) + "…" + string(r[len(r)-(mid-head):])
		if measure(cand) <= maxW {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	head := (lo + 1) / 2
	return string(r[:head]) + "…" + string(r[len(r)-(lo-head):])
}

// uiMonoFamily is the bundled JetBrains Mono, the primary face for Latin
// text, digits, versions and paths.
const uiMonoFamily = "MCL JetBrains Mono"

// uiFontFamily is the UI family list: the bundled JetBrains Mono first
// (it has no CJK glyphs), then the OS's own CJK face, then the system UI.
func uiFontFamily(goos string) string {
	cjk := "Noto Sans CJK SC, Noto Sans SC, WenQuanYi Micro Hei"
	switch goos {
	case "darwin":
		cjk = "PingFang SC, Hiragino Sans GB"
	case "windows":
		cjk = "Microsoft YaHei UI, Microsoft YaHei"
	}
	return uiMonoFamily + ", " + cjk + ", system-ui"
}
