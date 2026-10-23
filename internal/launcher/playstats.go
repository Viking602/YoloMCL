package launcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// PlayStats is persisted per game folder so the overview can show total
// playtime and launches across restarts of the launcher.
type PlayStats struct {
	Launches    int       `json:"launches"`
	TotalSec    int64     `json:"totalSec"`
	LastStart   time.Time `json:"lastStart,omitempty"`
	LastSec     int64     `json:"lastSec"`
	LastExit    string    `json:"lastExit,omitempty"` // exited | crashed | stopped
	LastCode    int       `json:"lastCode"`
	LastSummary string    `json:"lastSummary,omitempty"`
}

func playStatsPath(gameDir string) string { return filepath.Join(gameDir, ".mcl", "playstats.json") }

func readPlayStats(gameDir string) PlayStats {
	var p PlayStats
	if b, e := os.ReadFile(playStatsPath(gameDir)); e == nil {
		_ = json.Unmarshal(b, &p)
	}
	return p
}

// recordPlay adds one finished session to the folder's stats.
func recordPlay(gs GameSession, stopped bool) {
	if gs.GameDir == "" || gs.Started.IsZero() {
		return
	}
	_ = os.MkdirAll(filepath.Dir(playStatsPath(gs.GameDir)), 0o755)
	p := readPlayStats(gs.GameDir)
	d := int64(gs.Ended.Sub(gs.Started).Seconds())
	if d < 0 {
		d = 0
	}
	p.Launches++
	p.TotalSec += d
	p.LastStart, p.LastSec, p.LastCode = gs.Started, d, gs.ExitCode
	p.LastExit = gs.State
	if stopped {
		p.LastExit = "stopped"
	}
	p.LastSummary = ""
	if gs.Insight != nil {
		p.LastSummary = gs.Insight.Cause
	}
	_ = atomicJSON(playStatsPath(gs.GameDir), p)
}
