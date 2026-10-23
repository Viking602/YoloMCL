package nativeui

import (
	"bytes"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func logReadingTester(t *testing.T, raw []string) (*App, *ui.Tester) {
	t.Helper()
	a, tt := homeTester(t, LangZH)
	t.Cleanup(func() { a.cancel(); a.bgWG.Wait() })
	a.bgWG.Wait()
	tt.Frame()
	a.message, a.errorMessage = "", false
	a.instTab = "logs"
	a.logSess = logSessions{polled: time.Now().Add(time.Hour)}
	a.logView = logViewState{key: "file:report.txt", file: "report.txt", lines: parseLogLines(raw, "")}
	delete(scrollTracks, "log-scroll")
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	return a, tt
}

func TestCrashReportUsesFullWidthAndShowsCause(t *testing.T) {
	a, tt := logReadingTester(t, []string{
		"---- Minecraft Crash Report ----", "", "Description: Ticking screen", "",
		"java.lang.NullPointerException: " + strings.Repeat("entity.tick().", 45) + " ROOT_CAUSE_END",
		"\tat net.minecraft.client.Screen.tick(Screen.java:123)",
		"\tat net.minecraft.client.Main.main(Main.java:456)",
		"", "-- System Details --",
	})
	frame, _ := tt.Find("日志内容")
	for _, index := range []int{0, 2, 4} {
		body, ok := tt.Find(fmt.Sprint("日志正文 ", index))
		if !ok || body.X-frame.X > 25 || body.Y < frame.Y || body.X+body.W > frame.X+frame.W {
			t.Fatalf("report retained metadata gutter: %+v / %+v", body, frame)
		}
	}
	if a.logView.lines[4].cont || a.logView.lines[4].kids != 2 {
		t.Fatal("root cause hidden under a blank line")
	}
	row, _ := tt.Find("日志行 4")
	text, _ := tt.Find("日志正文 4")
	if row.H <= logRowHeight*a.lastScale || text.Y+text.H > row.Y+row.H {
		t.Fatalf("long report line clipped: %+v / %+v", row, text)
	}
	snapshotUI(t, tt, "crash-report-full-width")
}

func TestExpandedStackWrapsAndKeepsItsStartVisible(t *testing.T) {
	var raw []string
	for i := 0; i < 40; i++ {
		raw = append(raw, fmt.Sprintf("[19:56:%02d] [main/INFO]: Earlier line %d", i, i))
	}
	raw = append(raw, "[19:56:41] [Render thread/ERROR] [Apotheosis]: Failed to execute ShieldBreakerTest", "java.lang.NullPointerException: "+strings.Repeat("net.minecraft.world.entity.LivingEntity.tick(); ", 10)+"CAUSE_END")
	for i := 0; i < 72; i++ {
		raw = append(raw, fmt.Sprintf("\tat package.long.classname.method%d(ShieldBreakerTest.java:48) ~[mod.jar] %s STACK_END_%d", i, strings.Repeat("stack detail ", 20), i))
	}
	a, tt := logReadingTester(t, raw)
	a.logJumpLatest()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	clickUI(t, tt, "堆栈 73 行")
	tt.Frame()
	tt.Advance(250 * time.Millisecond)
	frame, _ := tt.Find("日志内容")
	head, _ := tt.Find("日志行 40")
	text, ok := tt.Find("日志正文 41")
	row, _ := tt.Find("日志行 41")
	snapshotUI(t, tt, "stack-expanded-debug")
	if !ok || text.X-frame.X > 35 || row.H <= logRowHeight*a.lastScale || text.Y+text.H > row.Y+row.H+1 {
		t.Fatalf("stack is indented or clipped: %+v %+v frame=%+v; follow=%v scroll=%+v open=%v vis=%d", row, text, frame, a.logView.follow, scrollTracks[a.logScrollKey()].st, a.logView.open, len(a.logView.vis))
	}
	if head.Y < frame.Y-1 || head.Y > frame.Y+30 || a.logView.follow {
		t.Fatalf("expansion lost its beginning: %+v, frame=%+v follow=%v", head, frame, a.logView.follow)
	}
	snapshotUI(t, tt, "stack-expanded-full-width")
	a.logJumpLatest()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	last, ok := tt.Find(fmt.Sprint("日志正文 ", len(a.logView.lines)-1))
	if !ok || last.Y+last.H > frame.Y+frame.H+1 {
		t.Fatalf("last wrapped stack line is unreachable: %+v", last)
	}
	// Search uses the measured offsets, including wrapped stack children.
	a.logView.query = "STACK_END_35"
	tt.Frame()
	a.logGoHit(6)
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	hit, ok := tt.Find("日志正文 77")
	if !ok || hit.Y < frame.Y || hit.Y > frame.Y+frame.H {
		t.Fatalf("search jumped to an old fixed-line offset: %+v", hit)
	}
	snapshotUI(t, tt, "stack-search-wrapped")
	a.logView.query = ""
	tt.SetSize(900, 540)
	tt.Advance(300 * time.Millisecond)
	a.logJumpLatest()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	frame, _ = tt.Find("日志内容")
	last, ok = tt.Find(fmt.Sprint("日志正文 ", len(a.logView.lines)-1))
	if !ok || last.Y+last.H > frame.Y+frame.H+1 || last.X+last.W > frame.X+frame.W {
		t.Fatalf("resize clipped final wrapped line: %+v / %+v", last, frame)
	}
	snapshotUI(t, tt, "stack-small-window-bottom")
}

func TestLogFolderActionsKeepTheFileListOpen(t *testing.T) {
	a, tt := homeTester(t, LangZH)
	defer func() { a.cancel(); a.bgWG.Wait() }()
	a.instTab = "logs"
	a.logSess = logSessions{polled: time.Now().Add(time.Hour)}
	root := a.root()
	base := filepath.Join(t.TempDir(), "My Pack #1 中文")
	files := []launcher.LogFile{
		{Name: "latest.log", Path: filepath.Join(base, "logs", "latest.log"), Kind: "log"},
		{Name: "crash-test.txt", Path: filepath.Join(base, "crash-reports", "crash-test.txt"), Kind: "crash"},
	}
	a.logView = logViewState{recent: files, recentKey: root.ID + "|" + root.SelectedVersion, recentAt: time.Now()}
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	for i, f := range files {
		clickUI(t, tt, "打开所在目录："+f.Name)
		opened := tt.OpenedURLs()
		if len(opened) != i+1 {
			t.Fatal("folder action did not open a URL")
		}
		u, err := url.Parse(opened[i])
		if err != nil || strings.TrimPrefix(u.Path, "/") != filepath.ToSlash(filepath.Dir(f.Path)) || a.logView.file != "" {
			t.Fatalf("wrong folder or row also opened the log: %v, %s", opened, a.logView.file)
		}
	}
	a.logView = logViewState{key: "file:" + files[1].Path, file: files[1].Path, path: files[1].Path, lines: parseLogLines([]string{"Crash report"}, "")}
	tt.Frame()
	clickUI(t, tt, "打开所在目录")
	if len(tt.OpenedURLs()) != 3 {
		t.Fatal("viewer missing folder action")
	}
	snapshotUI(t, tt, "log-folder-action")
}

func TestOpeningLogAnimatesOnlyTheBody(t *testing.T) {
	for _, reduce := range []bool{false, true} {
		a, tt := logReadingTester(t, []string{"First file"})
		tt.SetPreferences(ui.Preferences{ReduceMotion: reduce})
		a.logView.key, a.logView.file = "file:second.txt", "second.txt"
		a.logView.lines = parseLogLines([]string{"Second file content"}, "")
		a.logView.ver++
		tt.Frame()
		frame, _ := tt.Find("日志内容")
		first := append([]byte(nil), tt.Image().Pix...)
		firstBody, _ := tt.Find("日志正文 0")
		tt.Advance(200 * time.Millisecond)
		if now, _ := tt.Find("日志内容"); now != frame {
			t.Fatal("opening a file moved the frame")
		}
		same := bytes.Equal(first, tt.Image().Pix)
		if reduce && !same {
			t.Fatal("reduced motion still animates")
		}
		if !reduce && same {
			t.Fatalf("opening another file has no animation: %+v", firstBody)
		}
	}
}
