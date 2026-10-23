package nativeui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

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
			if strings.Contains(strings.ToLower(l.raw), q) {
				h := i
				if l.cont && l.parent >= 0 {
					h = l.parent
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
		if l.cont && l.parent >= 0 {
			head = l.parent
		}
		h := lv.lines[head]
		if !l.cont {
			lv.counts[levelKey(l.level)]++
		}
		if !all && !lv.levels[levelKey(h.level)] {
			continue
		}
		if l.cont && h.stack && !lv.open[h.raw] && !groupHit[head] {
			continue
		}
		if q != "" && strings.Contains(strings.ToLower(l.raw), q) {
			lv.hits = append(lv.hits, len(lv.vis))
		}
		lv.vis = append(lv.vis, i)
	}
	if lv.match >= len(lv.hits) {
		lv.match = max(0, len(lv.hits)-1)
	}
}

// loadLogs reloads the newest page in the background; only the log view
// changes (no busy state).
func (a *App) loadLogs() {
	r := a.root()
	if r == nil || a.service == nil {
		return
	}
	lv := &a.logView
	id, ver := r.ID, r.SelectedVersion
	scope := a.logScope()
	sid, key, file := "", "", lv.file
	if g := a.selectedSession(scope, a.scopedSessions(scope)); g != nil && file == "" {
		sid, key = g.ID, "sess:"+g.ID
	} else if file != "" {
		key = "file:" + file
	} else {
		lv.key, lv.lines, lv.loading = "", nil, false
		return // the empty state lists recent files instead
	}
	if lv.key != key {
		// Each session keeps its own chips and search.
		if a.logFilters == nil {
			a.logFilters = map[string]logFilter{}
		}
		if lv.key != "" {
			a.logFilters[lv.key] = logFilter{lv.levels, lv.query}
		}
		f := a.logFilters[key]
		report := false
		for _, item := range lv.recent {
			if item.Path == file && item.Kind == "crash" {
				report = true
			}
		}
		*lv = logViewState{key: key, file: file, follow: !report, levels: f.levels, query: f.query, recent: lv.recent, recentKey: lv.recentKey, recentAt: lv.recentAt}
		if report {
			if tr := scrollTracks[a.logScrollKey()]; tr != nil {
				tr.st.Y = 0
			}
		}
	}
	lv.gen++
	gen := lv.gen
	lv.loading = true
	lv.lastPoll = time.Now()
	go func() {
		var chunk launcher.LogChunk
		var e error
		if sid != "" {
			chunk, e = a.service.SessionTail(sid, logPageLines)
			chunk.Start = 0 // the ring is all there is; no older pages
		} else {
			chunk, e = a.service.FileTail(id, ver, file, logPageLines, -1)
		}
		lines := parseLogLines(chunk.Lines, "")
		a.post(func() {
			if gen != lv.gen || lv.key != key {
				return
			}
			lv.loading = false
			if e != nil {
				lv.err = a.friendlyErr(e)
				return
			}
			lv.err = ""
			lv.lines = lines
			lv.ver++
			lv.start, lv.path, lv.live, lv.dropped = chunk.Start, chunk.Path, chunk.Live, chunk.Dropped
			a.logText = ""
		})
	}()
}

func (a *App) loadOlderLogs() {
	lv := &a.logView
	r := a.root()
	if r == nil || lv.file == "" || lv.loadingOlder || lv.start <= 0 || len(lv.lines) >= logMaxLoaded {
		return
	}
	lv.loadingOlder = true
	id, ver, key, before, file := r.ID, r.SelectedVersion, lv.key, lv.start, lv.file
	go func() {
		chunk, e := a.service.FileTail(id, ver, file, 2000, before)
		older := parseLogLines(chunk.Lines, "")
		a.post(func() {
			lv.loadingOlder = false
			if e != nil || lv.key != key || lv.start != before {
				return
			}
			oldLine := 0
			if len(lv.vis) > 0 {
				oldLine = lv.vis[0]
			}
			oldY := lv.lineOffset(oldLine)
			for i := range lv.lines {
				if lv.lines[i].parent >= 0 {
					lv.lines[i].parent += len(older)
				}
			}
			lv.lines = append(older, lv.lines...)
			lv.start = chunk.Start
			lv.ver++
			// Rebase the current viewport on the same old line after the new
			// variable-height prefix is measured on the next frame.
			if tr := scrollTracks[a.logScrollKey()]; tr != nil {
				lv.prependLine, lv.prependOffset = oldLine+len(older), tr.st.Y-oldY
				lv.prepended = true
			}
		})
	}()
}

var logRowHeight float32 = 24

func logRowH(a *App) float32 { return logRowHeight * a.lastScale }

func levelTone(l string) (ui.Color, ui.Color) {
	switch l {
	case "WARN":
		return ui.Hex("#b25e00"), ui.Hex("#ff9f0a").Alpha(.14)
	case "ERROR", "FATAL":
		return ui.Hex("#d70015"), ui.Hex("#ff3b30").Alpha(.12)
	}
	return muted, ui.RGBA(120, 130, 160, .10)
}

func (a *App) logsDialog(c *ui.Context) {
	sk := a.logScrollKey() // fixed for this build even if a click opens the window
	s := uiScale(c)
	a.lastScale = s
	lv := &a.logView
	r := a.root()
	scope := a.logScope()
	sess := a.scopedSessions(scope)
	cur := a.selectedSession(scope, sess)
	want := ""
	if lv.file != "" {
		want, cur = "file:"+lv.file, nil
	} else if cur != nil {
		want = "sess:" + cur.ID
	}
	if r != nil && lv.key != want && !lv.loading {
		a.loadLogs()
	}
	live := cur != nil && (cur.State == "running" || lv.live) // one last read after exit
	if r != nil && live && time.Since(lv.lastPoll) > time.Second && !lv.loading {
		a.loadLogs() // live tail of the selected session only
	}
	if live {
		c.After(time.Second) // the run time ticks
	}
	// compact: the pane is too narrow for every label; the toolbar goes
	// to icons and the header drops its least useful column.
	ww, _ := c.Size()
	paneW := float32(ww) - 300*s
	if a.dialog == "logs" {
		paneW = min(1100*s, float32(ww)*.94) - 48*s
	}
	lv.compact = paneW < 900*s
	if len(sess) > 1 && lv.file == "" {
		a.sessionSwitcher(c, scope, sess, cur)
	}
	if r != nil && cur == nil && lv.file == "" {
		a.logEmpty(c, r, scope)
		return
	}
	if lv.file != "" {
		a.fileHeader(c)
	} else if cur != nil {
		a.sessionHeader(c, cur, scope, sess)
		if cur.State == "crashed" && cur.Insight != nil {
			a.crashCard(c, cur)
		}
	}
	lv.derive()
	a.logToolbar(c, cur, sk)
	if lv.start > 0 && lv.file != "" {
		ui.Row(c.Key("log-truncated")).FillWidth().Height(20 * s).Shrink(0).AlignItems(ui.Center).Children(func() {
			label(c, fmt.Sprintf(a.L("已载入最近 %d 行，向上滚动可加载更早内容", "Latest %d lines loaded; scroll up for more"), len(lv.lines))).FontSize(typeFootnote * s).SingleLine().Shrink(1)
		})
	}
	a.logFrame(c, sk)
}

// logFrame is the virtualized line list, with follow-latest.
func (a *App) logFrame(c *ui.Context, sk string) {
	s := uiScale(c)
	lv := &a.logView
	rowH := logRowHeight * s
	n := len(lv.vis)
	frame := ui.Box(c.Key("log-frame")).Label("日志内容").FillWidth().Grow(1).Basis(0).MinHeight(0).Radius(radiusCard*s).Clip().Background(ui.Hex("#fbfcfe")).Border(1, glassEdge)
	viewH, viewW := frame.LayoutBounds().H, frame.LayoutBounds().W
	if viewH <= 0 {
		// Build enough rows on the first frame; LayoutBounds requests another
		// frame once the flex layout knows the actual viewport height.
		_, viewH = c.Size()
	}
	if viewW <= 0 {
		viewW, _ = c.Size()
		viewW -= 400 * s
	}
	a.layoutLogRows(c, max(100*s, viewW-10*s))
	frame.Children(func() {
		if n == 0 {
			msg := a.L("还没有日志", "No log lines yet")
			switch {
			case lv.loading && len(lv.lines) == 0:
				msg = a.L("正在读取日志", "Reading log")
			case lv.err != "":
				msg = lv.err
			case lv.clearAt > 0:
				msg = a.L("已清空，新的日志会继续显示在这里", "Cleared; new lines appear here")
			case len(lv.lines) > 0:
				msg = a.L("没有符合筛选的行", "No lines match the filter")
			}
			ui.Column(c.Key("log-frame-empty")).Fill().Gap(10 * s).Center().Children(func() {
				ui.Icon(c, icons["terminal"]).Size(22*s, 22*s).TextColor(muted)
				label(c, msg).FontSize(typeCallout * s)
				if lv.clearAt > 0 && fixedButton(c, "log-unclear", a.L("显示全部", "Show all"), "", 0, btnSm*s, false, false) {
					lv.clearAt = 0
				}
			})
			return
		}
		body := ui.Column(c.Key("log-body-" + lv.key)).Fill().MinHeight(0)
		if !c.Preferences().ReduceMotion {
			body.Transition(ui.ElementTransition{Animation: &motionFast, Colors: true, Enter: &ui.Motion{Opacity: .82, Y: 4 * s}})
		}
		body.Children(func() {
			area := scrollArea(c, sk).Fill().Padding(8*s, 0)
			tr := scrollTracks[sk]
			if tr != nil {
				// Follow stays on unless the user scrolls up: a jump caused by
				// the content shrinking (a reload, a filter) re-pins instead.
				shrunk := tr.st.MaxY < lv.maxY-1
				if lv.follow {
					if tr.st.Y < lv.setY-1 && !shrunk {
						lv.follow = false
					} else {
						tr.st.Y = tr.st.MaxY
						lv.setY = tr.st.MaxY
					}
				} else if tr.st.Y >= tr.st.MaxY-2 && tr.st.MaxY > 0 {
					lv.follow = true
					lv.setY = tr.st.Y
				}
				if tr.st.Y > tr.st.MaxY {
					tr.st.Y = tr.st.MaxY // never scrolled past the newest line
				}
				lv.maxY = tr.st.MaxY
				if lv.prepended {
					tr.st.Y = lv.lineOffset(lv.prependLine) + lv.prependOffset
					lv.setY, lv.prepended = tr.st.Y, false
				}
				if lv.anchor != nil {
					tr.st.Y = lv.lineOffset(*lv.anchor) + 8*s
					lv.setY, lv.follow, lv.anchor = tr.st.Y, false, nil
				}
				if tr.st.Y < rowH*40 {
					a.loadOlderLogs()
				}
			}
			y := float32(0)
			if tr != nil {
				y = tr.st.Y
			}
			first := max(0, lv.rowAt(y)-10)
			last := min(n, lv.rowAt(y+viewH)+20)
			cur := -1
			if len(lv.hits) > 0 {
				cur = lv.hits[lv.match]
			}
			hit := map[int]bool{}
			for _, h := range lv.hits {
				if h >= first && h < last {
					hit[h] = true
				}
			}
			area.Children(func() {
				// Few lines sit at the bottom, by the newest, not under a gap.
				pad := float32(0)
				if lv.live {
					pad = max(0, viewH-16*s-lv.offsets[n])
				}
				ui.Column(c).FillWidth().Gap(0).Selectable().Children(func() {
					ui.Box(c).FillWidth().Height(pad + lv.offsets[first])
					for k := first; k < last; k++ {
						mark := 0
						if hit[k] {
							mark = 1
							if k == cur {
								mark = 2
							}
						}
						a.logRow(c, lv.vis[k], lv.offsets[k+1]-lv.offsets[k], mark)
					}
					ui.Box(c).FillWidth().Height(lv.offsets[n] - lv.offsets[last])
				})
			})
		})
		if !lv.follow && lv.maxY > 0 {
			ui.Box(c).Absolute().Right(14 * s).Bottom(12 * s).Children(func() {
				if fixedButton(c, "log-jump-latest", a.L("跳到最新", "Jump to latest"), "chevron-down", 0, btnSm*s, true, false) {
					a.logJumpLatest()
				}
			})
		}
	})
}

func (a *App) logJumpLatest() {
	lv := &a.logView
	lv.follow = true
	if tr := scrollTracks[a.logScrollKey()]; tr != nil {
		tr.st.Y = tr.st.MaxY
		lv.setY = tr.st.Y
	}
}

// logGoHit scrolls the current search hit into the middle of the view.
func (a *App) logGoHit(viewRows int) {
	lv := &a.logView
	if len(lv.hits) == 0 {
		return
	}
	lv.follow = false
	if tr := scrollTracks[a.logScrollKey()]; tr != nil {
		rowH := logRowH(a)
		y := float32(lv.hits[lv.match]) * rowH
		if len(lv.offsets) > lv.hits[lv.match] {
			y = lv.offsets[lv.hits[lv.match]]
		}
		tr.st.Y = max(0, min(tr.st.MaxY, y-float32(viewRows/2)*rowH))
		lv.setY = tr.st.Y
	}
}

// highlightSpans splits msg around case-insensitive hits of q.
func highlightSpans(msg, q string, fg ui.Color, size float32, strong bool) []ui.Span {
	if q == "" {
		return []ui.Span{{Text: msg, Color: fg, Size: size}}
	}
	lo, lq := strings.ToLower(msg), strings.ToLower(q)
	bg := ui.Hex("#ffd60a").Alpha(.45)
	if strong {
		bg = ui.Hex("#ff9f0a").Alpha(.55)
	}
	var out []ui.Span
	for {
		i := strings.Index(lo, lq)
		if i < 0 || len(lo) != len(msg) {
			out = append(out, ui.Span{Text: msg, Color: fg, Size: size})
			return out
		}
		if i > 0 {
			out = append(out, ui.Span{Text: msg[:i], Color: fg, Size: size})
		}
		out = append(out, ui.Span{Text: msg[i : i+len(q)], Color: ink, Size: size, Weight: 700, Background: bg})
		msg, lo = msg[i+len(q):], lo[i+len(q):]
		if msg == "" {
			return out
		}
	}
}

func (a *App) logRow(c *ui.Context, i int, rowH float32, mark int) {
	s := uiScale(c)
	lv := &a.logView
	l := lv.lines[i]
	head := l
	if l.cont && l.parent >= 0 {
		head = lv.lines[l.parent]
	}
	fg, chip := levelTone(head.level)
	q := strings.TrimSpace(lv.query)
	row := ui.Row(c.Key(fmt.Sprint("log-", i))).Label("日志行 "+fmt.Sprint(i)).FillWidth().Height(rowH).Padding(4*s, 14*s).Gap(10 * s).AlignItems(ui.Center)
	switch {
	case mark == 2:
		row.Background(ui.Hex("#ff9f0a").Alpha(.16))
	case mark == 1:
		row.Background(ui.Hex("#ffd60a").Alpha(.10))
	case levelRank(head.level) == 2:
		row.Background(ui.Hex("#ff3b30").Alpha(.04))
	}
	msgFg := ink
	if levelRank(head.level) > 0 {
		msgFg = fg
	}
	row.Children(func() {
		if l.cont {
			ui.Box(c).Width(2 * s).Height(rowH - 8*s).Background(chip).Shrink(0)
			if levelRank(head.level) == 0 {
				msgFg = inkSecondary
			}
			ui.RichText(c, highlightSpans(logMessage(l), q, msgFg, typeCaption*s, mark == 2)...).Label("日志正文 " + fmt.Sprint(i)).Grow(1).MinWidth(0).Tooltip(l.raw)
			return
		}
		if l.structured() {
			full := l.full
			if full == "" {
				full = l.time
			}
			ui.Text(c, l.time).Width(84 * s).Shrink(0).FontSize(typeCaption * s).FontFeatures("tnum").TextColor(muted).SingleLine().Tooltip(full)
			if l.level != "" {
				ui.Row(c).Width(52 * s).Height(16 * s).Shrink(0).Radius(4 * s).Background(chip).Center().Unselectable().Children(func() {
					ui.Text(c, l.level).FontSize(typeCaption2 * s).FontWeight(700).TextColor(fg).NoWrap()
				})
			} else {
				ui.Box(c).Width(52 * s).Shrink(0)
			}
			who := l.logger
			if who == "" {
				who = l.thread
			}
			ui.Text(c, who).Width(140 * s).Shrink(0).FontSize(typeCaption * s).TextColor(muted).SingleLine().Tooltip(strings.TrimSpace(l.thread + "  " + l.logger))
		}
		text := ui.RichText(c, highlightSpans(logMessage(l), q, msgFg, typeCaption*s, mark == 2)...).Label("日志正文 " + fmt.Sprint(i)).Grow(1).MinWidth(0).Tooltip(l.raw)
		if l.structured() {
			text.SingleLine()
		}
		if l.repeat > 1 {
			ui.Row(c).Height(18 * s).PaddingX(7 * s).Radius(999).Background(ui.RGBA(120, 130, 160, .12)).AlignItems(ui.Center).Shrink(0).Unselectable().Children(func() {
				ui.Text(c, fmt.Sprintf("×%d", l.repeat)).FontSize(typeCaption2 * s).FontWeight(700).FontFeatures("tnum").TextColor(inkSecondary).NoWrap()
			})
		}
		if l.stack && l.kids > 0 {
			open := lv.open[l.raw]
			text := a.logStackLabel(l)
			b := ui.ButtonBase(c.Key(fmt.Sprint("log-stack-", i))).Label(text).Height(18 * s).PaddingX(8 * s).Gap(4 * s).Radius(999).AlignItems(ui.Center).Shrink(0).Background(chip).Unselectable()
			if b.Children(func() {
				ic := "chevron"
				if open {
					ic = "chevron-down"
				}
				ui.Icon(c, icons[ic]).Size(9*s, 9*s).TextColor(fg)
				ui.Text(c, text).FontSize(typeCaption2 * s).FontWeight(600).TextColor(fg).NoWrap()
			}).Clicked() {
				if lv.open == nil {
					lv.open = map[string]bool{}
				}
				if open {
					delete(lv.open, l.raw)
				} else {
					lv.open[l.raw] = true
					line := i
					lv.anchor, lv.follow = &line, false
				}
			}
		}
	})
}

// logScrollKey keeps the all-games window and the instance tab on separate
// scroll handles, since both can build in the frame the window opens.
func (a *App) logScrollKey() string {
	if a.dialog == "logs" {
		return "log-scroll-all"
	}
	return "log-scroll"
}

// logFilter is one session's chips and search, kept across switches.
type logFilter struct {
	levels map[string]bool
	query  string
}
