package launcher

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type runningGame struct {
	cmd     *exec.Cmd
	tree    *procTree
	log     *gameLog
	started time.Time
}

// killTree stops the game and every process it spawned.
func (g *runningGame) killTree() error {
	if g.tree != nil {
		return g.tree.Kill()
	}
	return g.cmd.Process.Kill()
}

// waitTreeGone blocks until no process of the game's tree is left, so a
// session only ends once child JVMs (e.g. Crash Assistant) are gone too.
func (g *runningGame) waitTreeGone() {
	if g.tree == nil {
		return
	}
	for g.tree.Alive() {
		time.Sleep(200 * time.Millisecond)
	}
	g.tree.Close()
}

type gameLog struct {
	mu      sync.Mutex
	lines   []string // unused when ring is set (kept for installer logs)
	ring    *lineRing
	secret  string
	pending []byte
	file    *os.File // per-session copy on disk; nil for installer logs
	out     []byte   // decoded lines of the current write, flushed once
}

func (l *gameLog) Write(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	// Decode each whole line as it completes: Java writes stdout in the
	// system code page (GBK on Chinese Windows), not UTF-8. Only a partial
	// last line is carried over to the next write.
	rest := b
	if len(l.pending) > 0 {
		i := bytes.IndexByte(b, '\n')
		if i < 0 {
			l.pending = append(l.pending, b...)
			l.trimPending()
			return len(b), nil
		}
		l.pending = append(l.pending, b[:i]...)
		l.addLine(l.pending)
		l.pending = l.pending[:0]
		rest = b[i+1:]
	}
	for {
		i := bytes.IndexByte(rest, '\n')
		if i < 0 {
			break
		}
		l.addLine(rest[:i])
		rest = rest[i+1:]
	}
	l.pending = append(l.pending, rest...)
	l.trimPending()
	if len(l.lines) > 1200 {
		l.lines = l.lines[len(l.lines)-1200:]
	}
	// One file write per pipe read instead of one syscall per log line.
	if len(l.out) > 0 {
		if l.file != nil {
			_, _ = l.file.Write(l.out)
		}
		if cap(l.out) > 64<<10 {
			l.out = nil
		} else {
			l.out = l.out[:0]
		}
	}
	return len(b), nil
}

func (l *gameLog) trimPending() {
	if len(l.pending) > 16384 {
		l.pending = append(l.pending[:0], l.pending[len(l.pending)-16384:]...)
	}
}

func (l *gameLog) addLine(raw []byte) {
	v := DecodeLine(bytes.TrimRight(raw, "\r"))
	if l.secret != "" && strings.Contains(v, l.secret) {
		v = strings.ReplaceAll(v, l.secret, "[REDACTED]")
	}
	if l.file != nil {
		l.out = append(append(l.out, v...), '\n')
	}
	if l.ring != nil {
		l.ring.Add(v)
		return
	}
	l.lines = append(l.lines, v)
}

func (l *gameLog) read() []string {
	lines, _ := l.snapshot()
	return lines
}

// snapshot copies the kept lines and how many were dropped.
func (l *gameLog) snapshot() ([]string, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.ring != nil {
		return l.ring.Lines(), l.ring.Dropped()
	}
	return append([]string{}, l.lines...), 0
}
