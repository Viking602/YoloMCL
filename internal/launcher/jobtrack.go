package launcher

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func jobOf(ctx context.Context) *jobState {
	j, _ := ctx.Value(jobCtxKey{}).(*jobState)
	return j
}

// addJobBytes feeds the task's live download speed.
func addJobBytes(ctx context.Context, n int64) {
	if j := jobOf(ctx); j != nil {
		j.bytes.Add(n)
	}
}

// setJobBytesTotal fixes the task's whole download size when known.
func setJobBytesTotal(ctx context.Context, n int64) {
	if j := jobOf(ctx); j != nil && n > 0 {
		j.bytesTotal.Store(j.bytes.Load() + n)
		j.totalFixed.Store(true)
	}
}

// addJobFileTotal grows the total by one file, unless it was fixed.
func addJobFileTotal(ctx context.Context, n int64) {
	if j := jobOf(ctx); j != nil && n > 0 && !j.totalFixed.Load() {
		j.bytesTotal.Add(n)
	}
}

// trackCreated records path as made by this task, only when it does not
// exist yet, so cleanup never touches what was there before. Call it just
// before creating the path.
func (s *Service) trackCreated(ctx context.Context, path string) {
	j := jobOf(ctx)
	if j == nil || path == "" {
		return
	}
	if _, e := os.Lstat(path); e == nil {
		return
	}
	s.mu.Lock()
	j.created = append(j.created, filepath.Clean(path))
	s.mu.Unlock()
}

// onUndo registers a step that restores state if the task is cancelled
// or removed after failing.
func (s *Service) onUndo(ctx context.Context, fn func()) {
	if j := jobOf(ctx); j != nil {
		s.mu.Lock()
		j.undo = append(j.undo, fn)
		s.mu.Unlock()
	}
}

// cleanupJobLocked deletes what the task created, newest first; s.mu held,
// released around the disk work.
func (s *Service) cleanupJobLocked(j *jobState) {
	if j.cleaned {
		return
	}
	j.cleaned = true
	created, undo := j.created, j.undo
	j.created, j.undo = nil, nil
	s.mu.Unlock()
	defer s.mu.Lock()
	for i := len(undo) - 1; i >= 0; i-- {
		undo[i]()
	}
	for i := len(created) - 1; i >= 0; i-- {
		removeWithRetry(created[i])
	}
}

// removeWithRetry tolerates Windows briefly holding a just-closed file.
func removeWithRetry(p string) {
	for i := 0; i < 5; i++ {
		if e := os.RemoveAll(p); e == nil {
			return
		}
		time.Sleep(time.Duration(i+1) * 120 * time.Millisecond)
	}
}

// sampleSpeedLocked smooths bytes/s over about 2.5 s; s.mu held.
func sampleSpeedLocked(j *jobState, now time.Time) {
	b := j.bytes.Load()
	if j.lastAt.IsZero() {
		j.lastAt, j.lastBytes = now, b
		return
	}
	dt := now.Sub(j.lastAt).Seconds()
	if dt < .25 {
		return
	}
	inst := float64(b-j.lastBytes) / dt
	j.speed += (inst - j.speed) * (1 - math.Exp(-dt/2.5))
	if j.speed < 1 {
		j.speed = 0
	}
	j.lastAt, j.lastBytes = now, b
}

// RemoveJob drops a task from the list. A running task is cancelled and
// cleaned when it stops; a failed or cancelled one is cleaned now; a
// finished one is only removed from the list, its content stays.
func (s *Service) RemoveJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.jobs[id]
	if j == nil {
		return errors.New("任务已经不存在")
	}
	switch j.State {
	case "running":
		j.dropOnEnd = true
		j.cancel()
		return nil
	case "completed":
	default:
		s.cleanupJobLocked(j)
	}
	delete(s.jobs, id)
	return nil
}

// sweepTemp removes download leftovers a crash may have left: partial
// downloads, Java staging folders, pack previews and update staging.
func (s *Service) sweepTemp() {
	s.mu.Lock()
	roots := append([]Root(nil), s.cfg.Roots...)
	busy := len(s.rootLocks) > 0
	for _, job := range s.jobs {
		busy = busy || job.State == "running"
	}
	s.mu.Unlock()
	if busy {
		return
	}
	// A user can begin downloading immediately after the first frame. Only
	// old leftovers qualify; never remove a fresh preview/staging directory.
	cutoff := time.Now().Add(-24 * time.Hour)
	removeOld := func(root *os.Root, path string) {
		if info, err := root.Lstat(path); err == nil && info.ModTime().Before(cutoff) {
			_ = root.RemoveAll(path)
		}
	}
	sweep := func(root *os.Root, dir string, match func(string) bool) {
		f, err := root.Open(dir)
		if err != nil {
			return
		}
		entries, _ := f.ReadDir(-1)
		f.Close()
		for _, en := range entries {
			if match(en.Name()) {
				removeOld(root, filepath.Join(dir, en.Name()))
			}
		}
	}
	isTemp := func(n string) bool {
		return strings.HasPrefix(n, ".mcl-download-") || strings.HasPrefix(n, ".mcl-stage-") || strings.HasSuffix(n, ".mcl-part")
	}
	if root, err := os.OpenRoot(s.dir); err == nil {
		sweep(root, "cache", func(n string) bool { return isTemp(n) || strings.HasSuffix(n, ".pack") })
		if rel, err := filepath.Rel(s.dir, s.runtimeDir()); err == nil {
			sweep(root, rel, func(n string) bool { return isTemp(n) || strings.Contains(n, ".old-") || strings.HasPrefix(n, ".tmp-") })
		}
		root.Close()
	}
	for _, r := range roots {
		root, err := openScanRoot(r.Path)
		if err != nil {
			continue
		}
		removeOld(root, filepath.Join(".mcl", "update-staging"))
		bases := []string{"."}
		if f, err := root.Open("versions"); err == nil {
			entries, _ := f.ReadDir(-1)
			f.Close()
			for _, en := range entries {
				if en.IsDir() && en.Type()&os.ModeSymlink == 0 {
					bases = append(bases, filepath.Join("versions", en.Name()))
				}
			}
		}
		for _, base := range bases {
			for _, sub := range []string{"mods", "resourcepacks", "shaderpacks", ".mcl"} {
				sweep(root, filepath.Join(base, sub), isTemp)
			}
		}
		root.Close()
	}
}

// Maintain performs bounded housekeeping once the window is usable. A cancelled
// startup or an immediately closed window does no maintenance work.
func (s *Service) Maintain(ctx context.Context) {
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		s.sweepTemp()
	}
}

// jobKey names a task's work folders; stable across in-place retries.
func jobKey(ctx context.Context) string {
	if j := jobOf(ctx); j != nil {
		return j.ID
	}
	return newID()
}
