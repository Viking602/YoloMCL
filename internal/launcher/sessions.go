package launcher

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// GameSession is one launch of an instance. Each keeps its own bounded log
// ring, so several games running at once never share or overwrite a log.
type GameSession struct {
	ID       string    `json:"id"`
	RootID   string    `json:"rootId"`
	Name     string    `json:"name"`
	Version  string    `json:"version"`
	Seq      int       `json:"seq"`   // 1-based per instance; #2 for a relaunch
	State    string    `json:"state"` // running | exited | crashed
	ExitCode int       `json:"exitCode"`
	Started  time.Time `json:"started"`
	Ended    time.Time `json:"ended,omitempty"`
	LogPath  string    `json:"logPath,omitempty"`
	// Java is the runtime's version string, read once after launch.
	Java        string        `json:"java,omitempty"`
	MemoryMB    int           `json:"memoryMB,omitempty"`
	GameDir     string        `json:"gameDir,omitempty"`
	CrashReport string        `json:"crashReport,omitempty"`
	Insight     *CrashInsight `json:"insight,omitempty"`
	// Account is who plays this run, so two copies of one pack differ.
	AccountID   string `json:"accountId,omitempty"`
	AccountName string `json:"accountName,omitempty"`
	AccountKind string `json:"accountKind,omitempty"`
}

type gameSession struct {
	GameSession
	log     *gameLog
	game    *runningGame
	stopped bool
}

func runKey(rootID, version string) string { return rootID + "|" + version }

// rootRunningLocked reports whether any game in the folder runs; s.mu held.
func (s *Service) rootRunningLocked(rootID string) bool {
	for k := range s.running {
		if strings.HasPrefix(k, rootID+"|") {
			return true
		}
	}
	return false
}

// MaxExitedSessions bounds how many finished sessions stay until closed.
const MaxExitedSessions = 6

// newSessionLocked registers a running session; s.mu must be held.
func (s *Service) newSessionLocked(r Root, log *gameLog, g *runningGame) *gameSession {
	seq := 0
	for _, x := range s.sessions {
		if x.RootID == r.ID && x.Version == r.SelectedVersion && x.Seq > seq {
			seq = x.Seq
		}
	}
	name := r.SelectedVersion // the instance, not the .minecraft folder
	if vc, ok := r.VersionConfigs[r.SelectedVersion]; ok && vc.DisplayName != "" {
		name = vc.DisplayName
	}
	gs := &gameSession{GameSession: GameSession{ID: newID(), RootID: r.ID, Name: name, Version: r.SelectedVersion, Seq: seq + 1, State: "running", Started: time.Now()}, log: log, game: g}
	s.sessions = append(s.sessions, gs)
	return gs
}

// endSession records how a session ended and trims old exited ones.
func (s *Service) endSession(gs *gameSession, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	gs.Ended = time.Now()
	gs.game = nil
	gs.State = "exited"
	if err != nil {
		var ee interface{ ExitCode() int }
		if errors.As(err, &ee) {
			gs.ExitCode = int(int32(uint32(ee.ExitCode()))) // Windows reports -1 as 4294967295
		} else {
			gs.ExitCode = -1
		}
		if !gs.stopped {
			gs.State = "crashed"
			diagnoseSession(gs)
		}
	}
	go recordPlay(gs.GameSession, gs.stopped)
	exited := 0
	for i := len(s.sessions) - 1; i >= 0; i-- {
		if s.sessions[i].State == "running" {
			continue
		}
		exited++
		if exited > MaxExitedSessions {
			s.sessions = append(s.sessions[:i], s.sessions[i+1:]...)
		}
	}
}

// Sessions lists sessions oldest first; rootID "" means all.
func (s *Service) Sessions(rootID string) []GameSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []GameSession{}
	for _, x := range s.sessions {
		if rootID == "" || x.RootID == rootID {
			out = append(out, x.GameSession)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Started.Before(out[j].Started) })
	return out
}

func (s *Service) session(id string) *gameSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.sessions {
		if x.ID == id {
			return x
		}
	}
	return nil
}

// SessionTail is the newest n lines of one session's own log.
func (s *Service) SessionTail(id string, n int) (LogChunk, error) {
	gs := s.session(id)
	if gs == nil {
		return LogChunk{}, errors.New("这个会话已关闭")
	}
	if n <= 0 {
		n = LogRingLines
	}
	lines, dropped := gs.log.snapshot()
	if len(lines) > n {
		dropped += len(lines) - n
		lines = lines[len(lines)-n:]
	}
	s.mu.Lock()
	live := gs.State == "running"
	s.mu.Unlock()
	return LogChunk{Lines: lines, Live: live, Dropped: dropped, Path: gs.LogPath}, nil
}

// StopSession kills only that session's game.
func (s *Service) StopSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.sessions {
		if x.ID == id && x.game != nil {
			x.stopped = true
			return x.game.killTree()
		}
	}
	return errors.New("游戏未运行")
}

// CloseSession forgets an exited session (its log file stays on disk).
func (s *Service) CloseSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, x := range s.sessions {
		if x.ID == id {
			if x.State == "running" {
				return errors.New("游戏仍在运行")
			}
			s.sessions = append(s.sessions[:i], s.sessions[i+1:]...)
			return nil
		}
	}
	return nil
}

// sessionLogFile opens a per-session log under the version's logs folder and
// prunes all but the newest 20 of them.
func sessionLogFile(versionDir string, seq int) (*os.File, string) {
	dir := filepath.Join(versionDir, "logs")
	if os.MkdirAll(dir, 0o755) != nil {
		return nil, ""
	}
	if old, _ := filepath.Glob(filepath.Join(dir, "mcl-session-*.log")); len(old) >= 20 {
		sort.Strings(old)
		for _, p := range old[:len(old)-19] {
			_ = os.Remove(p)
		}
	}
	p := filepath.Join(dir, fmt.Sprintf("mcl-session-%s-%d.log", time.Now().Format("20060102-150405"), seq))
	f, e := os.Create(p)
	if e != nil {
		return nil, ""
	}
	return f, p
}
