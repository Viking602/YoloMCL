package nativeui

import (
	"strings"
	"time"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// logViewState holds at most logMaxLoaded parsed lines; the file is read
// from its end in pages (launcher.TailLines) and only rows in view are laid
// out.
type logViewState struct {
	compact      bool   // narrow pane: icon toolbar, fewer header columns
	key          string // sess:<id> | file:<path>
	lines        []logLine
	start        int64 // file offset of lines[0]; >0 means older lines exist
	path         string
	file         string // a log file picked from the recent list
	live         bool
	dropped      int
	ringTotal    int // session lines ever received when lines were parsed
	loading      bool
	loadingOlder bool
	follow       bool
	setY, maxY   float32
	levels       map[string]bool // chips picked; none = all
	query        string
	queryIn      string // query that queryLow was computed from
	queryLow     string // lowercased, trimmed query
	match        int    // current search hit, an index into hits
	open         map[string]bool
	clearAt      int // session lines before this global index are hidden
	lastPoll     time.Time
	gen, ver     int
	err          string
	// derived from lines, rebuilt when sig changes
	sig                      logSig
	vis                      []int
	hits                     []int // positions in vis
	counts                   map[string]int
	offsets                  []float32
	measured                 map[logMeasureKey]float32
	layoutSig                logSig
	layoutLang               string
	layoutWidth, layoutScale float32
	anchor                   *int // clicked stack heading to keep visible after expansion
	prepended                bool
	prependLine              int
	prependOffset            float32
	// recent files for the empty state
	recent    []launcher.LogFile
	recentKey string
	recentAt  time.Time
}

const (
	logPageLines = launcher.LogRingLines
	logMaxLoaded = 4 * launcher.LogRingLines
)

// clearLines drops every reference to the old text, including keys used for
// wrapping and expanded stacks. Keep the filters and recent-file inventory;
// the caller owns navigation and invalidates any outstanding read generation.
func (lv *logViewState) clearLines() {
	lv.lines, lv.vis, lv.hits, lv.offsets = nil, nil, nil, nil
	lv.measured, lv.open, lv.counts = nil, nil, nil
	lv.sig, lv.layoutSig = logSig{}, logSig{}
	lv.anchor, lv.prepended = nil, false
}

// logSig is everything derive's output depends on. The zero value never
// matches a computed signature.
type logSig struct {
	valid                                         bool
	ver, nlines, dropped, clearAt, nopen, nlevels int
	levels                                        uint8 // bit per levelKey result
	query                                         string
}

var logLevelKeys = [...]string{"info", "warn", "error", "debug"}

func (lv *logViewState) lowerQuery() string {
	if lv.query != lv.queryIn {
		lv.queryIn, lv.queryLow = lv.query, strings.ToLower(strings.TrimSpace(lv.query))
	}
	return lv.queryLow
}

func (lv *logViewState) signature(q string) logSig {
	sig := logSig{true, lv.ver, len(lv.lines), lv.dropped, lv.clearAt, len(lv.open), len(lv.levels), 0, q}
	for i, k := range logLevelKeys {
		if lv.levels[k] {
			sig.levels |= 1 << i
		}
	}
	return sig
}

// derive rebuilds visible rows, search hits and level counts when needed.
func (lv *logViewState) derive() {
	q := lv.lowerQuery()
	sig := lv.signature(q)
	if sig == lv.sig {
		return
	}
	lv.sig = sig
	all := len(lv.levels) == 0
	// match and groupHit are only built while searching: one lowercase of
	// each line per derive.
	var match, groupHit []bool
	if q != "" {
		match, groupHit = make([]bool, len(lv.lines)), make([]bool, len(lv.lines))
		for i, l := range lv.lines {
			if strings.Contains(strings.ToLower(l.Raw), q) {
				match[i] = true
				h := i
				if l.Cont && l.Parent >= 0 {
					h = l.Parent
				}
				groupHit[h] = true
			}
		}
	}
	lv.counts = map[string]int{}
	lv.vis, lv.hits = lv.vis[:0], lv.hits[:0]
	for i, l := range lv.lines {
		if i+lv.dropped < lv.clearAt {
			continue
		}
		head := i
		if l.Cont && l.Parent >= 0 {
			head = l.Parent
		}
		h := lv.lines[head]
		if !l.Cont {
			lv.counts[levelKey(l.Level)]++
		}
		if !all && !lv.levels[levelKey(h.Level)] {
			continue
		}
		if l.Cont && h.Stack && !lv.open[h.Raw] && (groupHit == nil || !groupHit[head]) {
			continue
		}
		if match != nil && match[i] {
			lv.hits = append(lv.hits, len(lv.vis))
		}
		lv.vis = append(lv.vis, i)
	}
	if lv.match >= len(lv.hits) {
		lv.match = max(0, len(lv.hits)-1)
	}
}
