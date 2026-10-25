package launcher

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sort"
	"time"
)

type progress func(message string, done, total int)

func (s *Service) startJob(title, rootID string, fn func(context.Context, progress) error) (string, error) {
	return s.startJobFor(title, rootID, "", fn)
}

// startJobFor with a launchVersion only refuses when that same version is
// already running, so other versions in the folder can run side by side.
func (s *Service) startJobFor(title, rootID, launchVersion string, fn func(context.Context, progress) error) (string, error) {
	return s.startJobForMode(title, rootID, launchVersion, false, fn)
}

// startJobForMode with again only waits for other jobs in the folder, not
// for running games: it starts another copy alongside them.
func (s *Service) startJobForMode(title, rootID, launchVersion string, again bool, fn func(context.Context, progress) error) (string, error) {
	s.mu.Lock()
	busy := false
	if rootID != "" {
		busy = s.rootLocks[rootID]
		switch {
		case again: // another copy alongside the running game
		case launchVersion != "":
			busy = busy || s.running[runKey(rootID, launchVersion)] != nil
		default:
			busy = busy || s.rootRunningLocked(rootID)
		}
	}
	if busy {
		s.mu.Unlock()
		return "", errors.New("此目录正在运行游戏或执行任务，请完成后再操作")
	}
	if rootID != "" {
		s.rootLocks[rootID] = true
	}
	id := newID()
	j := &jobState{Job: Job{ID: id, Title: title, State: "running", Message: "正在准备", Started: time.Now()}, fn: fn, rootID: rootID}
	j.TitleKey, j.TitleArg, _ = JobTitleKey(title)
	s.jobs[id] = j
	s.runJobLocked(j)
	s.mu.Unlock()
	return id, nil
}
func (s *Service) cancelJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j := s.jobs[id]; j != nil && j.State == "running" {
		j.cancel()
		return nil
	}
	return errors.New("任务已经结束")
}
func (s *Service) jobListLocked() []Job {
	out := []Job{}
	now := time.Now()
	for _, j := range s.jobs {
		if j.State == "running" {
			sampleSpeedLocked(j, now)
		}
		cp := j.Job
		cp.Steps = maps.Clone(j.Steps)
		if j.State == "running" {
			cp.Speed, cp.BytesDone, cp.BytesTotal = int64(j.speed), j.bytes.Load(), j.bytesTotal.Load()
		}
		if len(j.Items) > 0 {
			cp.Items = append([]string(nil), j.Items...)
		}
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Started.After(out[j].Started) })
	return out
}

type jobCtxKey struct{}

func (s *Service) setJobStep(ctx context.Context, key, state string) {
	if j, ok := ctx.Value(jobCtxKey{}).(*jobState); ok {
		s.mu.Lock()
		if j.Steps == nil {
			j.Steps = map[string]string{}
		}
		j.Steps[key] = state
		s.mu.Unlock()
	}
}

// setJobResultRoot records the directory a job produced (for 「启动」 /
// 「查看目录」 on the finished task card).
func (s *Service) setJobResultRef(ctx context.Context, ref string) {
	if j, ok := ctx.Value(jobCtxKey{}).(*jobState); ok {
		s.mu.Lock()
		j.ResultRef = ref
		s.mu.Unlock()
	}
}
func (s *Service) setJobResultRoot(ctx context.Context, rootID string) {
	if j, ok := ctx.Value(jobCtxKey{}).(*jobState); ok {
		s.mu.Lock()
		j.ResultRoot = rootID
		s.mu.Unlock()
	}
}

// runJobLocked starts (or restarts) a task's work; s.mu held.
func (s *Service) runJobLocked(j *jobState) {
	ctx, cancel := context.WithCancel(context.Background())
	j.cancel = cancel
	ctx = context.WithValue(ctx, jobCtxKey{}, j)
	id, fn, rootID := j.ID, j.fn, j.rootID
	go func() {
		defer cancel()
		p := func(msg string, done, total int) {
			s.mu.Lock()
			j.Message = msg
			// After an in-place retry the bar holds its place while the
			// finished pieces are re-checked, then moves on from there.
			if j.holdDone > 0 {
				switch {
				case total == 0 || (total == j.Total && done < j.holdDone):
					done, total = j.Done, j.Total
				case total == j.Total:
					j.holdDone = 0
				}
			}
			j.Done = done
			j.Total = total
			if msg != "" && (len(j.Items) == 0 || j.Items[len(j.Items)-1] != msg) {
				j.Items = append(j.Items, msg)
				if len(j.Items) > 50 {
					j.Items = append([]string(nil), j.Items[len(j.Items)-50:]...)
				}
			}
			s.mu.Unlock()
		}
		err := func() (err error) {
			defer func() {
				if v := recover(); v != nil {
					err = fmt.Errorf("任务失败: %v", v)
				}
			}()
			return fn(ctx, p)
		}()
		s.invalidateInventory(rootID)
		s.mu.Lock()
		defer s.mu.Unlock()
		// Cleanup temporarily releases s.mu for disk IO. Keep the task
		// running and its root reserved until that work really finishes:
		// observers must not report idle, retry, or start another install
		// in a directory whose cancelled task is still removing files.
		if errors.Is(err, context.Canceled) || j.dropOnEnd && err != nil {
			s.cleanupJobLocked(j)
		}
		delete(s.rootLocks, rootID)
		now := time.Now()
		j.Finished = &now
		if err == nil {
			j.State = "completed"
			j.Message = "已完成"
			if j.Total > 0 {
				j.Done = j.Total
			}
		} else if errors.Is(err, context.Canceled) {
			j.State = "cancelled"
			j.Message = "已取消"
		} else {
			j.State = "failed"
			j.Message = err.Error()
		}
		if err == nil {
			j.created, j.undo = nil, nil // the result belongs to the user now
		}
		if j.dropOnEnd {
			s.cleanupJobLocked(j)
			delete(s.jobs, j.ID)
			return
		}
		// Retain a bounded task history, including failed jobs for diagnosis.
		if len(s.jobs) > 80 {
			var oldest *jobState
			for _, other := range s.jobs {
				if other.State != "running" && other.ID != id && (oldest == nil || other.Started.Before(oldest.Started)) {
					oldest = other
				}
			}
			if oldest != nil {
				delete(s.jobs, oldest.ID)
			}
		}
	}()
}

// RetryJob runs a failed task again in place: same entry, same ID, the
// created-files ledger kept, finished pieces reused (downloads skip files
// that already verify and resume partial ones).
func (s *Service) RetryJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.jobs[id]
	if j == nil || j.State != "failed" || j.fn == nil {
		return errors.New("这个任务不能重试")
	}
	if j.rootID != "" {
		if s.rootLocks[j.rootID] || s.rootRunningLocked(j.rootID) {
			return errors.New("此目录正在运行游戏或执行任务，请完成后再操作")
		}
		s.rootLocks[j.rootID] = true
	}
	j.Attempt++
	j.holdDone = max(j.Done, 1)
	j.State, j.Message, j.Finished = "running", "正在重试", nil
	j.speed, j.lastAt = 0, time.Time{}
	j.cleaned = false
	s.runJobLocked(j)
	return nil
}
