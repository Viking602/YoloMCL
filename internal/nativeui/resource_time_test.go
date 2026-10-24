package nativeui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestResourceRelativeTime(t *testing.T) {
	now := time.Date(2026, 10, 23, 0, 30, 0, 0, time.UTC)
	for _, tc := range []struct {
		age    time.Duration
		zh, en string
	}{
		{-time.Minute, "刚刚", "just now"},
		{0, "刚刚", "just now"},
		{59 * time.Second, "刚刚", "just now"},
		{time.Minute, "1 分钟前", "1 minute ago"},
		{59*time.Minute + 59*time.Second, "59 分钟前", "59 minutes ago"},
		{time.Hour, "1 小时前", "1 hour ago"},
		{3*time.Hour + 45*time.Minute, "3 小时前", "3 hours ago"},
		{24*time.Hour - time.Second, "23 小时前", "23 hours ago"},
		{24 * time.Hour, "昨天", "yesterday"},
		{48 * time.Hour, "2 天前", "2 days ago"},
		{7 * 24 * time.Hour, "1 周前", "1 week ago"},
		{14 * 24 * time.Hour, "2 周前", "2 weeks ago"},
		{30 * 24 * time.Hour, "1 个月前", "1 month ago"},
		{365 * 24 * time.Hour, "1 年前", "1 year ago"},
		{730 * 24 * time.Hour, "2 年前", "2 years ago"},
	} {
		// The API's offset and crossing midnight must not change elapsed time.
		at := now.Add(-tc.age).In(time.FixedZone("API", -7*60*60))
		if zh, en := updatedAgo(false, at, now), updatedAgo(true, at, now); zh != tc.zh || en != tc.en {
			t.Errorf("age %v: got %q / %q, want %q / %q", tc.age, zh, en, tc.zh, tc.en)
		}
	}
	if got := updatedAgo(false, time.Time{}, now); got != "—" {
		t.Fatalf("zero timestamp became a real date: %q", got)
	}
}

func TestResourceTimeRefreshesWhileIdle(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		age           time.Duration
		before, after string
	}{
		{59 * time.Second, "just now", "1 minute ago"},
		{59*time.Minute + 59*time.Second, "59 minutes ago", "1 hour ago"},
		{24*time.Hour - time.Second, "23 hours ago", "yesterday"},
	} {
		t.Run(tc.before, func(t *testing.T) {
			a := &App{}
			a.OverrideLanguage(LangEN)
			var raw, relative, exact string
			frames := 0
			tt := ui.NewTester(func(c *ui.Context) {
				frames++
				relative, exact = a.resourceTime(c, raw)
				ui.Text(c, relative)
			}, 240, 60)
			at := tt.Now().Add(-tc.age).In(time.FixedZone("API", -7*60*60))
			raw = at.Format(time.RFC3339Nano)
			tt.Frame()
			if relative != tc.before || exact != at.Local().Format("2006-01-02 15:04:05") {
				t.Fatalf("time or local hover details incorrect: %q / %q", relative, exact)
			}
			tt.Advance(2 * time.Second)
			if !tt.HasText(tc.after) {
				t.Fatalf("idle timestamp did not roll over: %v", tt.Texts())
			}
			settled := frames
			tt.Advance(10 * time.Second)
			if frames != settled {
				t.Fatalf("unchanged time caused unnecessary frames: %d", frames-settled)
			}
			for _, invalid := range []string{"", "not-a-date", "0001-01-01T00:00:00Z"} {
				raw = invalid
				tt.Frame()
				if relative != "" || exact != "" {
					t.Fatalf("invalid timestamp %q was displayed: %q / %q", invalid, relative, exact)
				}
			}
		})
	}
}

func TestDiscoverResourceTimeAcrossTypes(t *testing.T) {
	for _, source := range []string{"modrinth", "curseforge"} {
		for _, kind := range []string{"modpack", "mod", "resourcepack", "shader"} {
			for _, lang := range []string{LangZH, LangEN} {
				t.Run(source+"/"+kind+"/"+lang, func(t *testing.T) {
					a, tt := discoverMotionTester(t)
					a.OverrideLanguage(lang)
					ps := discoverMotionProjects(0, 3)
					ages := []time.Duration{23*time.Hour + 10*time.Minute, 59*time.Minute + 10*time.Second, 20 * time.Second}
					for i := range ps {
						ps[i].Source, ps[i].Type = source, kind
						ps[i].Updated = tt.Now().Add(-ages[i]).Format(time.RFC3339)
					}
					a.provider, a.resourceType = source, kind
					showDiscoverResults(a, tt, launcher.SearchQuery{Source: source, Type: kind}, ps)
					tt.Advance(700 * time.Millisecond)
					for _, width := range []int{1536, 900} {
						tt.SetSize(width, 850)
						tt.Frame()
						for i, p := range ps {
							at, _ := time.Parse(time.RFC3339, p.Updated)
							text := updatedAgo(a.isEN(), at, tt.Now())
							date, ok := tt.Find(text)
							card, cardOK := tt.Find("搜索结果 " + p.Title)
							button, buttonOK := tt.Find("按钮文字 view-" + p.ID)
							if !ok || !cardOK || !buttonOK || date.X < card.X || date.X+date.W > button.X || date.Y+date.H > card.Y+card.H {
								t.Fatalf("card %d timestamp missing or overlaps action: date=%+v card=%+v button=%+v", i, date, card, button)
							}
						}
						snapshotUI(t, tt, fmt.Sprintf("resource-time-cards-%s-%s-%s-%d", source, kind, lang, width))
					}
					at, _ := time.Parse(time.RFC3339, ps[0].Updated)
					date, _ := tt.Find(updatedAgo(a.isEN(), at, tt.Now()))
					tt.Move(date.X+date.W/2, date.Y+date.H/2)
					tt.Advance(time.Second)
					hint := fmt.Sprintf(a.L("更新时间：%s", "Updated: %s"), at.Local().Format("2006-01-02 15:04:05"))
					if !strings.Contains(strings.Join(tt.Texts(), "\n"), hint) {
						t.Fatalf("card hover does not show its full local update time: %v", tt.Texts())
					}
				})
			}
		}
	}
}
