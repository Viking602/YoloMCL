package nativeui

import (
	"time"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

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
		lv.gen++
		lv.key, lv.loading, lv.loadingOlder = "", false, false
		lv.clearLines()
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
		*lv = logViewState{gen: lv.gen, key: key, file: file, follow: !report, levels: f.levels, query: f.query, recent: lv.recent, recentKey: lv.recentKey, recentAt: lv.recentAt}
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
	seen, wasLive, loaded := lv.ringTotal, lv.live, lv.lines != nil
	go func() {
		var chunk launcher.LogChunk
		var e error
		if sid != "" {
			chunk, e = a.service.SessionTail(sid, logPageLines)
			chunk.Start = 0 // the ring is all there is; no older pages
			// The live tail polls every second; a quiet game must not
			// re-parse and re-measure the whole ring each time.
			if e == nil && loaded && chunk.Live == wasLive && chunk.Dropped+len(chunk.Lines) == seen {
				a.post(func() {
					if gen == lv.gen && lv.key == key {
						lv.loading, lv.err = false, ""
					}
				})
				return
			}
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
			lv.ringTotal = 0
			if sid != "" {
				lv.ringTotal = chunk.Dropped + len(chunk.Lines)
			}
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
	gen := lv.gen
	go func() {
		chunk, e := a.service.FileTail(id, ver, file, 2000, before)
		older := parseLogLines(chunk.Lines, "")
		a.post(func() {
			if lv.gen != gen || lv.key != key || lv.start != before {
				return
			}
			lv.loadingOlder = false
			if e != nil {
				return
			}
			oldLine := 0
			if len(lv.vis) > 0 {
				oldLine = lv.vis[0]
			}
			oldY := lv.lineOffset(oldLine)
			for i := range lv.lines {
				if lv.lines[i].Parent >= 0 {
					lv.lines[i].Parent += len(older)
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
