package launcher

import (
	"testing"
	"time"
)

func TestPlayStatsAccumulate(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Now().Add(-time.Hour)
	recordPlay(GameSession{GameDir: dir, Started: t0, Ended: t0.Add(30 * time.Minute), State: "exited"}, false)
	recordPlay(GameSession{GameDir: dir, Started: t0, Ended: t0.Add(10 * time.Minute), State: "crashed", ExitCode: 1, Insight: &CrashInsight{Cause: "模组冲突"}}, false)
	p := readPlayStats(dir)
	if p.Launches != 2 || p.TotalSec != 2400 || p.LastSec != 600 || p.LastExit != "crashed" || p.LastSummary != "模组冲突" {
		t.Fatalf("%+v", p)
	}
}
