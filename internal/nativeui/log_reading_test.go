package nativeui

import (
	"bytes"
	"fmt"
	"hash/maphash"
	"math"
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
	if a.logView.lines[4].Cont || a.logView.lines[4].Kids != 2 {
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
	var revealed []string
	previousReveal := revealLogPath
	revealLogPath = func(path string) { revealed = append(revealed, path) }
	t.Cleanup(func() { revealLogPath = previousReveal })
	a, tt := homeTester(t, LangZH)
	defer func() { a.cancel(); a.bgWG.Wait() }()
	a.instTab = "logs"
	a.logSess = logSessions{polled: time.Now().Add(time.Hour)}
	root := a.root()
	base := filepath.Join(t.TempDir(), "My Pack #1 中文, 100% & friends")
	files := []launcher.LogFile{
		{Name: "latest.log", Path: filepath.Join(base, "logs", "latest.log"), Kind: "log"},
		{Name: "crash-test.txt", Path: filepath.Join(base, "crash-reports", "crash-test.txt"), Kind: "crash"},
	}
	a.logView = logViewState{recent: files, recentKey: root.ID + "|" + root.SelectedVersion, recentAt: time.Now()}
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	for i, f := range files {
		clickUI(t, tt, "在文件夹中显示："+f.Name)
		if len(revealed) != i+1 || revealed[i] != f.Path || len(tt.OpenedURLs()) != 0 || a.logView.file != "" {
			t.Fatalf("folder action must reveal the exact file and leave the list open: %v, %s", revealed, a.logView.file)
		}
	}
	// A selected report takes precedence over an older tail path, including
	// while its text is still loading and in the compact toolbar.
	a.logView = logViewState{key: "file:" + files[1].Path, file: files[1].Path, path: files[0].Path, lines: parseLogLines([]string{"Crash report"}, "")}
	for i, width := range []int{1536, 900} {
		tt.SetSize(width, 1024)
		clickUI(t, tt, "在文件夹中显示")
		if len(revealed) != i+3 || revealed[i+2] != files[1].Path || len(tt.OpenedURLs()) != 0 {
			t.Fatalf("viewer revealed a different file or used a URL: %v", revealed)
		}
	}
	clickUI(t, tt, "打开文件")
	opened := tt.OpenedURLs()
	if len(opened) != 1 {
		t.Fatal("opening a file must remain separate from showing it in a folder")
	}
	u, err := url.Parse(opened[0])
	wantPath := filepath.ToSlash(files[1].Path)
	if !strings.HasPrefix(wantPath, "/") {
		wantPath = "/" + wantPath
	}
	if err != nil || u.Scheme != "file" || u.Path != wantPath || len(revealed) != 4 {
		t.Fatalf("opening the report targeted a different file: %v", opened)
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

func logNavigationTester(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	a, tt := logReadingTester(t, []string{"Initial file"})
	a.bgWG.Wait()
	tt.Frame()
	a.service = nil // drive disk completion explicitly, independently of motion
	r := a.root()
	a.logView = logViewState{recentKey: r.ID + "|" + r.SelectedVersion, recentAt: time.Now(), recent: []launcher.LogFile{
		{Name: "latest.log", Path: "latest.log", Kind: "log", Size: 1024},
		{Name: "crash-report.txt", Path: "crash-report.txt", Kind: "crash", Size: 2048},
	}}
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	return a, tt
}

func TestLogNavigationAnimatesOpenAndBack(t *testing.T) {
	for _, width := range []int{900, 1536} {
		for _, reduce := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d-reduced-%v", width, reduce), func(t *testing.T) {
				a, tt := logNavigationTester(t)
				tt.SetSize(width, 1024)
				tt.SetPreferences(ui.Preferences{ReduceMotion: reduce})
				tt.Advance(400 * time.Millisecond)
				tabs, _ := tt.Find("实例标签")
				tabPixels := overviewRegionPixels(tt.Image(), tabs)
				for _, name := range []string{"latest.log", "crash-report.txt"} {
					clickUI(t, tt, name)
					a.logView.key, a.logView.loading = "file:"+name, true
					tt.Move(0, 0)
					tt.Frame()
					if a.logView.file != name || !tt.HasText("正在读取日志") {
						t.Fatal("opening a file did not enter the reader before its data arrived")
					}
					tt.Advance(70 * time.Millisecond)
					mid, _ := tt.Find("日志页面 file")
					midMarker, _ := tt.Find("按钮文字 log-file-back")
					snapshotUI(t, tt, fmt.Sprintf("log-open-%d-%v-%s", width, reduce, name))
					tt.Advance(300 * time.Millisecond)
					end, _ := tt.Find("日志页面 file")
					endMarker, _ := tt.Find("按钮文字 log-file-back")
					if mid.H != end.H || math.Abs(float64(midMarker.W-endMarker.W)) > .1 || midMarker.H != endMarker.H || (!reduce && midMarker.X <= endMarker.X) || (reduce && mid != end) {
						t.Fatalf("opening must slide in at native size, unless motion is reduced: %+v -> %+v", mid, end)
					}
					// Late disk data animates only the lines, not the reader again.
					a.logView.loading = false
					a.logView.lines = parseLogLines([]string{"Contents of " + name}, "")
					a.logView.ver++
					tt.Frame()
					if now, _ := tt.Find("日志页面 file"); now != end {
						t.Fatal("disk completion replayed navigation")
					}
					tt.Advance(200 * time.Millisecond)
					clickUI(t, tt, a.L("返回", "Back"))
					tt.Move(0, 0)
					tt.Advance(70 * time.Millisecond)
					mid, _ = tt.Find("日志页面 recent")
					midMarker, _ = tt.Find(a.L("现在没有运行中的游戏", "No game is running"))
					snapshotUI(t, tt, fmt.Sprintf("log-back-%d-%v-%s", width, reduce, name))
					tt.Advance(300 * time.Millisecond)
					end, _ = tt.Find("日志页面 recent")
					endMarker, _ = tt.Find(a.L("现在没有运行中的游戏", "No game is running"))
					if mid.H != end.H || math.Abs(float64(midMarker.W-endMarker.W)) > .1 || midMarker.H != endMarker.H || (!reduce && midMarker.X >= endMarker.X) || (reduce && mid != end) || a.logView.file != "" || !tt.HasText("日志文件列表") {
						t.Fatalf("returning must animate the list from the opposite direction: %+v -> %+v", mid, end)
					}
					if now, _ := tt.Find("实例标签"); now != tabs || !bytes.Equal(tabPixels, overviewRegionPixels(tt.Image(), tabs)) {
						t.Fatal("log navigation moved or repainted the instance tabs")
					}
				}
			})
		}
	}
}

func TestLogNavigationCanReverseWhileReading(t *testing.T) {
	a, tt := logNavigationTester(t)
	for _, name := range []string{"latest.log", "crash-report.txt", "latest.log"} {
		clickUI(t, tt, name)
		a.logView.key, a.logView.loading, a.logView.loadingOlder = "file:"+name, true, true
		generation := a.logView.gen
		tt.Frame()
		tt.Advance(35 * time.Millisecond)
		clickUI(t, tt, a.L("返回", "Back"))
		tt.Advance(35 * time.Millisecond)
		if a.logView.file != "" || a.logView.key != "" || a.logView.loading || a.logView.loadingOlder || a.logView.gen <= generation {
			t.Fatal("returning left a pending read attached to the closed file")
		}
	}
	clickUI(t, tt, "crash-report.txt")
	a.logView.key = "file:crash-report.txt"
	a.logView.lines = parseLogLines([]string{"Final report"}, "")
	a.logView.ver++
	tt.Frame()
	tt.Move(0, 0)
	tt.Advance(400 * time.Millisecond)
	reader, ok := tt.Find("日志页面 file")
	viewport, _ := tt.Find("日志切换区域")
	if !ok || reader != viewport || tt.HasText("日志文件列表") || a.logView.file != "crash-report.txt" || !tt.HasText("日志正文 0") {
		t.Fatalf("rapid navigation left a stale or displaced view: reader=%+v viewport=%+v", reader, viewport)
	}
	snapshotUI(t, tt, "log-navigation-interrupted")
}

func TestReturningFromLogReleasesTextCaches(t *testing.T) {
	a, tt := logNavigationTester(t)
	clickUI(t, tt, "latest.log")
	a.logView.key = "file:latest.log"
	a.logView.lines = parseLogLines([]string{"A report line"}, "")
	a.logView.ver++
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	// Wrapped lines and expanded stacks can keep an old multi-megabyte tail
	// alive even after the parsed lines themselves have been cleared.
	text := strings.Repeat("retained old log ", 1<<16)
	a.logView.measured = map[logMeasureKey]float32{{text: maphash.String(logHashSeed, text)}: 48}
	a.logView.open = map[string]bool{text: true}
	clickUI(t, tt, a.L("返回", "Back"))
	tt.Advance(400 * time.Millisecond)
	lv := &a.logView
	if len(lv.lines)+len(lv.measured)+len(lv.open)+len(lv.vis)+len(lv.hits)+len(lv.offsets) != 0 {
		t.Fatal("closed log still retains its text or derived layout")
	}
	if !tt.HasText("日志文件列表") || len(lv.recent) != 2 {
		t.Fatal("clearing log contents removed the recent-file list")
	}
}
