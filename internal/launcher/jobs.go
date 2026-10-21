package launcher

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

type progress func(message string, done, total int)

func (s *Service) startJob(title, rootID string, fn func(context.Context, progress) error) (string, error) {
	s.mu.Lock()
	if rootID != "" && (s.rootLocks[rootID] || s.running[rootID] != nil) {
		s.mu.Unlock()
		return "", errors.New("此目录正在运行游戏或执行任务，请完成后再操作")
	}
	if rootID != "" {
		s.rootLocks[rootID] = true
	}
	ctx, cancel := context.WithCancel(context.Background())
	id := newID()
	j := &jobState{Job: Job{ID: id, Title: title, State: "running", Message: "正在准备", Started: time.Now()}, cancel: cancel}
	ctx = context.WithValue(ctx, jobCtxKey{}, j)
	s.jobs[id] = j
	s.mu.Unlock()
	go func() {
		defer cancel()
		p := func(msg string, done, total int) {
			s.mu.Lock()
			j.Message = msg
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
		s.mu.Lock()
		defer s.mu.Unlock()
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
	for _, j := range s.jobs {
		cp := j.Job
		if len(j.Items) > 0 {
			cp.Items = append([]string(nil), j.Items...)
		}
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Started.After(out[j].Started) })
	return out
}

type jobCtxKey struct{}

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
