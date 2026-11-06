package nativeui

import (
	"math"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/i18n"
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
	return bodyText(c, s).TextColor(muted).FontSize(typeCallout * uiScale(c))
}

// bodyText is shared by descriptions and explanatory prose. Formatting is
// presentation-only: translation strings, paths, code and input values are not
// rewritten. MyGo keeps its Unicode line breaking and emergency wrapping.
func bodyText(c *ui.Context, text string) ui.Element {
	return ui.Text(c, i18n.BindTextPairs(text)).Label(text).MinWidth(0)
}

type textBalanceMeasure struct {
	text            string
	font            ui.Font
	maxWidth, width float32
}

// balancedText narrows short messages to even out their existing lines.
// MyGo still chooses every break, including Unicode punctuation rules; no
// breaks are inserted into the copy. Cache shaping until text or width changes.
func balancedText(c *ui.Context, text string, font ui.Font, maxWidth float32) ui.Element {
	bound := i18n.BindTextPairs(text)
	e := ui.Text(c, bound).Label(text).Font(font.Family).FontSize(font.Size).FontWeight(font.Weight).MinWidth(0).Shrink(0)
	state := ui.Local(e, "text-balance", func() textBalanceMeasure { return textBalanceMeasure{} })
	maxWidth = max(1, maxWidth)
	if state.text != bound || state.font != font || state.maxWidth != maxWidth {
		state.text, state.font, state.maxWidth, state.width = bound, font, maxWidth, maxWidth
		lines := len(ui.ShapeText(bound, font, maxWidth).Lines())
		if lines >= 2 && lines <= 6 {
			lo, hi := float32(1), maxWidth
			for i := 0; i < 12 && hi-lo > .25; i++ {
				mid := (lo + hi) / 2
				if len(ui.ShapeText(bound, font, mid).Lines()) > lines {
					lo = mid
				} else {
					hi = mid
				}
			}
			state.width = min(maxWidth, float32(math.Ceil(float64(hi))))
		}
	}
	return e.FillWidth().MaxWidth(state.width)
}

// textTooltip is the shared overflow policy. Callers pass the actual font and
// line limit used by the anchor; element bounds are the allocated text box,
// without padding. Measurements are cached until the text or its box changes.
// Registration precedes hover so MyGo can start its normal tooltip delay.
// Independent
// contextual hints (e.g. an original version ID) remain available even if the
// displayed label fits.
func textTooltip(c *ui.Context, anchor ui.Element, text, hint string, font ui.Font, lines int) ui.Element {
	if hint == text {
		hint = ""
	}
	measured := text
	if lines != 1 {
		measured = i18n.BindTextPairs(text)
	}
	if cachedTextClipped(c, anchor, measured, font, lines) {
		if hint != "" {
			hint = text + "\n" + hint
		} else {
			hint = text
		}
	}
	return tooltip(c, anchor, hint)
}

type textOverflowMeasure struct {
	text           string
	font           ui.Font
	w, h           float32
	lines          int
	ready, clipped bool
}

func cachedTextClipped(c *ui.Context, anchor ui.Element, text string, font ui.Font, lines int) bool {
	state := ui.Local(anchor, "text-overflow", func() textOverflowMeasure { return textOverflowMeasure{} })
	r := anchor.LayoutBounds()
	if !state.ready {
		// First layout has no bounds yet. One settled frame registers a tip
		// before the user hovers; zero-width/hidden nodes never spin a timer.
		state.ready = true
		if r.W <= 0 {
			c.AnimationFrame()
		}
	}
	if r.W <= 0 {
		return false
	}
	if state.text != text || state.font != font || state.w != r.W || state.h != r.H || state.lines != lines {
		state.text, state.font, state.w, state.h, state.lines = text, font, r.W, r.H, lines
		state.clipped = textClipped(text, font, r, lines)
	}
	return state.clipped
}

func textClipped(text string, font ui.Font, box ui.Rect, lines int) bool {
	width := box.W
	if lines == 1 {
		width = 0
	}
	layout := ui.ShapeText(text, font, width)
	w, h := layout.Size()
	return w > box.W+.25 || h > box.H+.25 || (lines > 0 && len(layout.Lines()) > lines)
}

// tooltip sizes the text before adding padding. Rounding up leaves room for
// fractional font metrics, which otherwise push the last glyph onto a new line.
// Only the visible tooltip is measured; long paths and descriptions still wrap.
func tooltip(c *ui.Context, anchor ui.Element, text string) ui.Element {
	if text == "" {
		return anchor
	}
	anchor.Description(text)
	ui.TooltipBase(c, anchor, func(tip ui.Element) {
		tooltipContent(c, anchor, tip, text)
	})
	return anchor
}

func tooltipContent(c *ui.Context, anchor, tip ui.Element, text string) {
	tip.Label("文本提示")
	t := c.Theme()
	windowW, _ := c.Size()
	s := uiScale(c)
	padX, padY := space3*s, space2*s
	size := t.FontSize - 1
	textW, _ := c.MeasureText(0, ui.Span{Text: text, Font: t.Font, Size: size, Weight: 400})
	contentW := min(float32(math.Ceil(float64(textW)))+1, max(1, min(1280.0/3*s, windowW-8)-2*padX))
	fill, foreground := t.Inverse, t.InverseText
	if fill == ui.Transparent {
		fill = t.Text
	}
	if foreground == ui.Transparent {
		foreground = t.Background
	}
	tip.Width(contentW+2*padX).Padding(padY, padX).Radius(radiusChip*s).
		Font(t.Font).FontSize(size).FontWeight(400).Background(fill).TextColor(foreground).
		Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, .2)).
		AttachTo(anchor, ui.AnchorTop, ui.AnchorBottom).Margin(0, 0, space2*s, 0)
	tip.Children(func() {
		label := ui.Text(c, text).FillWidth().MinWidth(0)
		if textW <= contentW {
			label.NoWrap()
		}
	})
}

func pathLabel(c *ui.Context, path string, size float32) ui.Element {
	dir, base := filepath.Split(path)
	if base == "" {
		dir, base = filepath.Split(strings.TrimRight(path, `/\`))
		base += string(filepath.Separator)
	}
	row := ui.Row(c).Label("路径 " + path).Grow(1).Basis(0).MinWidth(0).Gap(0).AlignItems(ui.Center)
	clipped := false
	row.Children(func() {
		if dir != "" {
			e := ui.Text(c, dir).FontSize(size).TextColor(muted).SingleLine().Shrink(1).MinWidth(0)
			clipped = cachedTextClipped(c, e, dir, ui.Font{Family: c.Theme().Font, Size: size}, 1)
		}
		e := ui.Text(c, base).FontSize(size).FontWeight(600).TextColor(inkSecondary).SingleLine().Shrink(0).MinWidth(0).MaxWidthPercent(70)
		baseClipped := cachedTextClipped(c, e, base, ui.Font{Family: c.Theme().Font, Size: size, Weight: 600}, 1)
		clipped = clipped || baseClipped
	})
	if clipped {
		tooltip(c, row, path)
	}
	return row
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
