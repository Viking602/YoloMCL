package nativeui

import (
	"fmt"
	"hash/maphash"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// logMeasureKey identifies a wrapped height by the content hashes of the
// message and the search text, so the cache keeps no message strings and
// stays valid when lines are replaced or prepended (indices shift, text
// does not).
type logMeasureKey struct {
	text, query  uint64
	width, scale float32
}

var logHashSeed = maphash.MakeSeed()

func logMessage(l logLine) string {
	if !l.Cont && !l.Structured() {
		return strings.ReplaceAll(l.Raw, "\t", "    ")
	}
	return l.Msg
}

func (a *App) logStackLabel(l logLine) string {
	return fmt.Sprintf(a.L("堆栈 %d 行", "Stack %d lines"), l.Kids)
}

// Heights and prefix offsets replace fixed-line arithmetic for reports and
// expanded stacks. Measurements are cached; only viewport rows create widgets.
func (a *App) layoutLogRows(c *ui.Context, width float32) {
	lv, s := &a.logView, uiScale(c)
	if lv.layoutSig == lv.sig && lv.layoutWidth == width && lv.layoutScale == s && lv.layoutLang == a.settings.Language {
		return
	}
	lv.layoutSig, lv.layoutWidth, lv.layoutScale, lv.layoutLang = lv.sig, width, s, a.settings.Language
	// Only long lines are cached; a bounded multiple of the rows in play
	// keeps scrolled-away measurements from accumulating.
	if lv.measured == nil || len(lv.measured) > 64+4*len(lv.vis) {
		lv.measured = make(map[logMeasureKey]float32)
	}
	lv.offsets = make([]float32, len(lv.vis)+1)
	q := strings.TrimSpace(lv.query)
	qh := maphash.String(logHashSeed, q)
	for pos, index := range lv.vis {
		l := lv.lines[index]
		height := logRowHeight * s
		if l.Cont || !l.Structured() {
			textWidth := width - 28*s // row padding; scroll gutter is excluded by the caller
			if l.Cont {
				textWidth -= 12 * s
			}
			if l.Stack && l.Kids > 0 && !l.Cont {
				w, _ := c.MeasureText(0, ui.Span{Text: a.logStackLabel(l), Size: typeCaption2 * s, Weight: 600})
				textWidth -= w + 39*s
			}
			if l.Repeat > 1 {
				w, _ := c.MeasureText(0, ui.Span{Text: fmt.Sprintf("×%d", l.Repeat), Size: typeCaption2 * s, Weight: 700})
				textWidth -= w + 24*s
			}
			textWidth = max(32*s, textWidth)
			msg := logMessage(l)
			// A short line fits even with wide glyphs. Long lines use the same
			// shaping engine and spans as the actual text, including search hits.
			if float32(utf8.RuneCountInString(msg))*typeCaption*s*1.2 > textWidth || strings.ContainsAny(msg, "\r\n") {
				key := logMeasureKey{maphash.String(logHashSeed, msg), qh, textWidth, s}
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
