package nativeui

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

type logMeasureKey struct {
	text, query  string
	width, scale float32
}

func logMessage(l logLine) string {
	if !l.cont && !l.structured() {
		return strings.ReplaceAll(l.raw, "\t", "    ")
	}
	return l.msg
}

func (a *App) logStackLabel(l logLine) string {
	return fmt.Sprintf(a.L("堆栈 %d 行", "Stack %d lines"), l.kids)
}

// Heights and prefix offsets replace fixed-line arithmetic for reports and
// expanded stacks. Measurements are cached; only viewport rows create widgets.
func (a *App) layoutLogRows(c *ui.Context, width float32) {
	lv, s := &a.logView, uiScale(c)
	if lv.layoutSig == lv.sig && lv.layoutWidth == width && lv.layoutScale == s && lv.layoutLang == a.settings.Language {
		return
	}
	lv.layoutSig, lv.layoutWidth, lv.layoutScale, lv.layoutLang = lv.sig, width, s, a.settings.Language
	if lv.measured == nil || len(lv.measured) > 2*logMaxLoaded {
		lv.measured = make(map[logMeasureKey]float32)
	}
	lv.offsets = make([]float32, len(lv.vis)+1)
	q := strings.TrimSpace(lv.query)
	for pos, index := range lv.vis {
		l := lv.lines[index]
		height := logRowHeight * s
		if l.cont || !l.structured() {
			textWidth := width - 28*s // row padding; scroll gutter is excluded by the caller
			if l.cont {
				textWidth -= 12 * s
			}
			if l.stack && l.kids > 0 && !l.cont {
				w, _ := c.MeasureText(0, ui.Span{Text: a.logStackLabel(l), Size: typeCaption2 * s, Weight: 600})
				textWidth -= w + 39*s
			}
			if l.repeat > 1 {
				w, _ := c.MeasureText(0, ui.Span{Text: fmt.Sprintf("×%d", l.repeat), Size: typeCaption2 * s, Weight: 700})
				textWidth -= w + 24*s
			}
			textWidth = max(32*s, textWidth)
			msg := logMessage(l)
			// A short line fits even with wide glyphs. Long lines use the same
			// shaping engine and spans as the actual text, including search hits.
			if float32(utf8.RuneCountInString(msg))*typeCaption*s*1.2 > textWidth || strings.ContainsAny(msg, "\r\n") {
				key := logMeasureKey{msg, q, textWidth, s}
				if h, ok := lv.measured[key]; ok {
					height = h
				} else {
					_, h := c.MeasureText(textWidth, highlightSpans(msg, q, ink, typeCaption*s, false)...)
					height = max(height, h+8*s)
					lv.measured[key] = height
				}
			}
		}
		lv.offsets[pos+1] = lv.offsets[pos] + height
	}
}

func (lv *logViewState) rowAt(y float32) int {
	return min(len(lv.vis), sort.Search(len(lv.vis), func(i int) bool { return lv.offsets[i+1] > y }))
}

func (lv *logViewState) lineOffset(index int) float32 {
	pos := sort.SearchInts(lv.vis, index)
	if pos < len(lv.offsets) {
		return lv.offsets[pos]
	}
	return 0
}
