package launcher

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// LogRingLines bounds the live game output kept in memory, and the default
// page of a log file the UI loads at once.
const LogRingLines = 5000

// lineRing is a fixed-capacity ring of lines: memory stays bounded however
// long the game runs.
type lineRing struct {
	buf   []string
	head  int // index of the oldest line
	count int
	total int // lines ever added
}

func newLineRing(n int) *lineRing { return &lineRing{buf: make([]string, n)} }

func (r *lineRing) Add(s string) {
	r.total++
	if r.count < len(r.buf) {
		r.buf[(r.head+r.count)%len(r.buf)] = s
		r.count++
		return
	}
	r.buf[r.head] = s
	r.head = (r.head + 1) % len(r.buf)
}

// Lines returns the kept lines, oldest first.
func (r *lineRing) Lines() []string {
	out := make([]string, r.count)
	for i := range out {
		out[i] = r.buf[(r.head+i)%len(r.buf)]
	}
	return out
}

// Dropped is how many lines fell out of the ring.
func (r *lineRing) Dropped() int { return r.total - r.count }

// TailLines reads up to n lines that end before byte offset before (-1 =
// end of file) by seeking backwards in 64 KiB chunks, so a huge log is
// never loaded whole. It returns the lines (oldest first, CR stripped,
// invalid UTF-8 replaced) and the offset of the first returned line; 0
// means the start of the file was reached.
func TailLines(path string, n int, before int64) ([]string, int64, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, 0, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return nil, 0, e
	}
	end := st.Size()
	if before >= 0 && before < end {
		end = before
	}
	const chunk = 64 << 10
	pos := end
	var tail []byte // bytes from pos to end
	for pos > 0 && bytes.Count(tail, []byte{'\n'}) <= n {
		step := int64(chunk)
		if pos < step {
			step = pos
		}
		pos -= step
		buf := make([]byte, step)
		if _, e := f.ReadAt(buf, pos); e != nil && !errors.Is(e, io.EOF) {
			return nil, 0, e
		}
		tail = append(buf, tail...)
		if len(tail) > 64<<20 { // pathological single line: stop growing
			break
		}
	}
	text := tail
	start := pos
	// Drop a trailing newline so the last line is not an empty entry.
	text = bytes.TrimSuffix(text, []byte{'\n'})
	parts := bytes.Split(text, []byte{'\n'})
	if start > 0 && len(parts) > 0 { // first part is a partial line
		start += int64(len(parts[0]) + 1)
		parts = parts[1:]
	}
	if len(parts) > n {
		for _, p := range parts[:len(parts)-n] {
			start += int64(len(p) + 1)
		}
		parts = parts[len(parts)-n:]
	}
	lines := make([]string, len(parts))
	for i, p := range parts {
		lines[i] = DecodeLine(bytes.TrimRight(p, "\r"))
	}
	return lines, start, nil
}

// LogChunk is one page of an instance's log.
type LogChunk struct {
	Lines   []string `json:"lines"`
	Start   int64    `json:"start"`   // offset of Lines[0]; >0: older lines exist
	Path    string   `json:"path"`    // file the lines came from ("" = live)
	Live    bool     `json:"live"`    // from the running game
	Dropped int      `json:"dropped"` // live lines that left the ring
}

// LogTail returns the latest n lines: the running game's ring, else the
// tail of logs/latest.log, else MCL's saved last-log.json. before > 0
// pages further back in the same file.
func (s *Service) LogTail(rootID, versionID string, n int, before int64) (LogChunk, error) {
	if n <= 0 {
		n = LogRingLines
	}
	s.mu.Lock()
	g := s.running[runKey(rootID, versionID)]
	s.mu.Unlock()
	if g != nil && before < 0 {
		lines, dropped := g.log.snapshot()
		if len(lines) > n {
			dropped += len(lines) - n
			lines = lines[len(lines)-n:]
		}
		return LogChunk{Lines: lines, Live: true, Dropped: dropped}, nil
	}
	r, e := s.rootAt(rootID, versionID)
	if e != nil {
		return LogChunk{}, e
	}
	p := filepath.Join(r.VersionDir(), "logs", "latest.log")
	if _, e := os.Stat(p); e == nil {
		lines, start, e := TailLines(p, n, before)
		return LogChunk{Lines: lines, Start: start, Path: p}, e
	}
	if before >= 0 {
		return LogChunk{}, nil
	}
	b, e := os.ReadFile(filepath.Join(r.Path, ".mcl", "last-log.json"))
	if os.IsNotExist(e) {
		return LogChunk{Lines: []string{}}, nil
	}
	if e != nil {
		return LogChunk{}, e
	}
	var lines []string
	e = json.Unmarshal(b, &lines)
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return LogChunk{Lines: lines}, e
}
