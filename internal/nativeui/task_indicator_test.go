package nativeui

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestJobPillLongStatusDoesNotOverlap(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{LangZH, LangEN} {
		for _, size := range [][2]int{{900, 600}, {1280, 800}, {1536, 1024}} {
			t.Run(fmt.Sprintf("%s-%dx%d", lang, size[0], size[1]), func(t *testing.T) {
				a := &App{page: "home"}
				a.OverrideLanguage(lang)
				job := launcher.Job{ID: "launch", Title: "启动 Mechanomania-航空学", State: "running", Message: "校验 / 下载 Mechanomania-1.1.6.jar", Done: 99, Total: 100}
				a.snapshot.Jobs = []launcher.Job{job}
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(theme)
					ui.Column(c).Fill().Background(chromeBg).Children(func() { a.windowBar(c) })
				}, size[0], size[1])
				tt.SetTitleBar(ui.TitleBar{Right: 138})
				settle(tt)
				snapshotUI(t, tt, fmt.Sprintf("task-status-%s-%d", lang, size[0]))
				bounds := func(text string) ui.Rect {
					t.Helper()
					r, ok := tt.Find(text)
					if !ok {
						t.Fatalf("missing %q", text)
					}
					return r
				}
				name := bounds(a.jobTitle(job))
				step := bounds(jobStep(a.jobMsg(job.Message)))
				percent := bounds("99%")
				pill := bounds(a.L("task_indicator.view_downloads"))
				if name.X+name.W > step.X+.5 || step.X+step.W > percent.X+.5 {
					t.Fatalf("task text overlaps: name=%+v status=%+v percent=%+v", name, step, percent)
				}
				if name.W <= 0 || name.X < pill.X || percent.X+percent.W > pill.X+pill.W {
					t.Fatalf("task text escapes pill: name=%+v percent=%+v pill=%+v", name, percent, pill)
				}
				langButton := bounds(a.tr().LangToggleLabel)
				if pill.X+pill.W > langButton.X+.5 {
					t.Fatalf("task pill overlaps the language switch: pill=%+v language=%+v", pill, langButton)
				}
			})
		}
	}
}

func TestJobPillTooltipShowsFullStatus(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	a.OverrideLanguage(LangZH)
	job := launcher.Job{ID: "launch", Title: "启动 Mechanomania-航空学", State: "running", Message: "校验 / 下载 Mechanomania-1.1.6.jar", Done: 99, Total: 100}
	a.snapshot.Jobs = []launcher.Job{job}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Column(c).Fill().Background(chromeBg).Padding(20).Children(func() { a.jobPill(c) })
	}, 900, 600)
	r, ok := tt.Find(a.L("task_indicator.view_downloads"))
	if !ok {
		t.Fatal("task pill missing")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(time.Second)
	want := fmt.Sprintf("%s  (99/100)\n%s", a.jobTitle(job), a.jobMsg(job.Message))
	if _, ok := tt.Find(want); !ok {
		t.Fatalf("hover does not reveal the full task and status: %v", tt.Texts())
	}
	snapshotUI(t, tt, "task-status-tooltip")
}
