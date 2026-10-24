package nativeui

import (
	"fmt"
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
	loading      bool
	loadingOlder bool
	follow       bool
	setY, maxY   float32
	levels       map[string]bool // chips picked; none = all
	query        string
	match        int // current search hit, an index into hits
	open         map[string]bool
	clearAt      int // session lines before this global index are hidden
	lastPoll     time.Time
	gen, ver     int
	err          string
	// derived from lines, rebuilt when sig changes
	sig                      string
	vis                      []int
	hits                     []int // positions in vis
	counts                   map[string]int
	offsets                  []float32
	measured                 map[logMeasureKey]float32
	layoutSig, layoutLang    string
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

// derive rebuilds visible rows, search hits and level counts when needed.
func (lv *logViewState) derive() {
	q := strings.ToLower(strings.TrimSpace(lv.query))
	sig := fmt.Sprint(lv.ver, len(lv.lines), lv.dropped, lv.clearAt, q, len(lv.open), lv.levels)
	if sig == lv.sig {
		return
	}
	lv.sig = sig
	all := len(lv.levels) == 0
	var groupHit map[int]bool
	if q != "" {
		groupHit = map[int]bool{}
		for i, l := range lv.lines {
			if strings.Contains(strings.ToLower(l.Raw), q) {
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
		if l.Cont && h.Stack && !lv.open[h.Raw] && !groupHit[head] {
			continue
		}
		if q != "" && strings.Contains(strings.ToLower(l.Raw), q) {
			lv.hits = append(lv.hits, len(lv.vis))
		}
		lv.vis = append(lv.vis, i)
	}
	if lv.match >= len(lv.hits) {
		lv.match = max(0, len(lv.hits)-1)
	}
}
