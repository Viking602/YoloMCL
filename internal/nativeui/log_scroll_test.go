package nativeui

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestOnlyLogLinesScroll(t *testing.T) {
	for _, size := range [][2]int{{900, 540}, {1536, 1024}} {
		for _, dialog := range []bool{false, true} {
			for _, kind := range []string{"file", "crash"} {
				t.Run(fmt.Sprintf("%dx%d/dialog=%v/%s", size[0], size[1], dialog, kind), func(t *testing.T) {
					a, tt := homeTester(t, "zh-CN")
					a.instTab = "logs"
					if dialog {
						a.openDialog("logs")
					}
					r := a.root()
					a.logSess = logSessions{polled: time.Now().Add(time.Hour)}
					lines := make([]string, 1000)
					for i := range lines {
						lines[i] = fmt.Sprintf("[17:32:45] [Render thread/INFO]: Log line %04d", i)
					}
					a.logView = logViewState{gen: 1000, key: "file:debug.log", file: "debug.log", lines: parseLogLines(lines, ""), start: 1, loadingOlder: true}
					if kind == "crash" {
						a.logSess.list = []launcher.GameSession{
							{ID: "first", RootID: r.ID, Version: r.SelectedVersion, Name: "Fixture", Seq: 1, State: "exited"},
							{ID: "second", RootID: r.ID, Version: r.SelectedVersion, Name: "Fixture", Seq: 2, State: "crashed", Insight: &launcher.CrashInsight{Cause: "测试崩溃信息", Advice: "检查日志"}},
						}
						a.logView.key, a.logView.file, a.logView.start = "sess:second", "", 0
					}
					delete(scrollTracks, "page-scroll-home-logs")
					delete(scrollTracks, "dialog-content-logs")
					delete(scrollTracks, a.logScrollKey())
					tt.SetSize(size[0], size[1])
					tt.Frame()
					tt.Advance(400 * time.Millisecond)
					fixed := map[string]ui.Rect{}
					for _, label := range []string{"实例标签", "日志摘要", "日志工具栏"} {
						var ok bool
						fixed[label], ok = tt.Find(label)
						if !ok {
							t.Fatalf("missing %s", label)
						}
					}
					frame, ok := tt.Find("日志内容")
					if !ok || frame.H < 60 || frame.Y+frame.H > float32(size[1]) {
						t.Fatalf("log viewport escaped the window: %+v", frame)
					}
					track := scrollTracks[a.logScrollKey()]
					if track == nil || track.st.MaxY <= 0 {
						t.Fatal("log lines must be scrollable")
					}
					before := track.st.Y
					tt.Scroll(frame.X+frame.W/2, frame.Y+frame.H/2, 0, 180)
					tt.Advance(300 * time.Millisecond)
					if track.st.Y <= before {
						t.Fatal("scrolling did not move log lines")
					}
					// Scroll beyond both boundaries, then over the fixed header.
					for _, dy := range []float32{100000, 100000, -100000, -100000} {
						tt.Scroll(frame.X+frame.W/2, frame.Y+frame.H/2, 0, dy)
						tt.Advance(200 * time.Millisecond)
					}
					header := fixed["日志摘要"]
					tt.Scroll(header.X+header.W/2, header.Y+header.H/2, 0, 500)
					tt.Advance(300 * time.Millisecond)
					for label, want := range fixed {
						if got, _ := tt.Find(label); got != want {
							t.Fatalf("scrolling moved %s: %+v -> %+v", label, want, got)
						}
					}
					if scrollTracks["page-scroll-home-logs"] != nil || scrollTracks["dialog-content-logs"] != nil {
						t.Fatal("logs still have an outer scroll container")
					}
					snapshotUI(t, tt, fmt.Sprintf("fixed-log-%dx%d-dialog-%v-%s", size[0], size[1], dialog, kind))
				})
			}
		}
	}
}
