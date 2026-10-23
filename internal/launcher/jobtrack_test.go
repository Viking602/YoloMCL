package launcher

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Cancel deletes only what the task created; files that were there before stay.
func TestCancelDeletesOnlyCreated(t *testing.T) {
	s := testService(t)
	dir := t.TempDir()
	pre := filepath.Join(dir, "user.jar")
	writeTestFile(t, pre, []byte("mine"))
	made := filepath.Join(dir, "new", "mod.jar")
	started := make(chan struct{})
	id, _ := s.startJob("下载测试", "", func(ctx context.Context, p progress) error {
		s.trackCreated(ctx, pre) // existed: must be ignored
		s.trackCreated(ctx, filepath.Dir(made))
		_ = os.MkdirAll(filepath.Dir(made), 0o755)
		writeTestFile(t, made, []byte("x"))
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	<-started
	if e := s.cancelJob(id); e != nil {
		t.Fatal(e)
	}
	if j := waitJob(t, s, id); j.State != "cancelled" {
		t.Fatalf("state %s", j.State)
	}
	if _, e := os.Stat(pre); e != nil {
		t.Fatal("pre-existing file was deleted")
	}
	if _, e := os.Stat(filepath.Dir(made)); !os.IsNotExist(e) {
		t.Fatal("created folder was left behind")
	}
}

// Cancelling during extraction stops at the next entry and leaves the
// task's stage for cleanup, so nothing half-extracted survives.
func TestCancelDuringExtract(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "a.zip")
	f, _ := os.Create(archive)
	zw := zip.NewWriter(f)
	for i := 0; i < 50; i++ {
		w, _ := zw.Create("jdk/bin/f" + string(rune('a'+i%26)) + string(rune('a'+i/26)))
		_, _ = w.Write([]byte(strings.Repeat("x", 1000)))
	}
	zw.Close()
	f.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stage := filepath.Join(dir, "stage")
	if e := unzipInto(ctx, archive, stage); !errors.Is(e, context.Canceled) {
		t.Fatalf("want cancel, got %v", e)
	}
	if ents, _ := os.ReadDir(filepath.Join(stage, "jdk", "bin")); len(ents) > 0 {
		t.Fatalf("extracted %d files after cancel", len(ents))
	}
}

// Retry runs in place: same entry, progress held, ledger kept, and 移除
// afterwards cleans files from every attempt.
func TestRetryInPlace(t *testing.T) {
	s := testService(t)
	dir := t.TempDir()
	var attempt atomic.Int32
	id, _ := s.startJob("下载测试", "", func(ctx context.Context, p progress) error {
		n := attempt.Add(1)
		f := filepath.Join(dir, "part"+string(rune('0'+n)))
		s.trackCreated(ctx, f)
		writeTestFile(t, f, []byte("x"))
		if n == 1 {
			p("下载 (3/10)", 3, 10)
			return errors.New("dial tcp: connection refused")
		}
		p("正在检查", 0, 0)
		s.mu.Lock()
		j := s.jobs[jobOf(ctx).ID]
		held := j.Done
		s.mu.Unlock()
		if held != 3 {
			return errors.New("progress was reset on retry")
		}
		p("下载 (4/10)", 4, 10)
		return errors.New("still offline")
	})
	if j := waitJob(t, s, id); j.State != "failed" {
		t.Fatal(j.State)
	}
	if e := s.RetryJob(id); e != nil {
		t.Fatal(e)
	}
	time.Sleep(20 * time.Millisecond)
	j := waitJob(t, s, id)
	if j.State != "failed" || j.Attempt != 1 || j.Done != 4 || !strings.Contains(j.Message, "still offline") {
		t.Fatalf("retry not in place: %+v", j)
	}
	if n := len(s.Snapshot().Jobs); n != 1 {
		t.Fatalf("%d task entries, want 1", n)
	}
	if e := s.RemoveJob(id); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"part1", "part2"} {
		if _, e := os.Stat(filepath.Join(dir, p)); !os.IsNotExist(e) {
			t.Fatalf("%s survived 移除", p)
		}
	}
}
