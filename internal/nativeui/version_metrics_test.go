package nativeui

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDownloadCountFormatting(t *testing.T) {
	for _, tc := range []struct {
		count         int64
		zh, en, exact string
	}{
		{0, "0", "0", "0"},
		{999, "999", "999", "999"},
		{1000, "1,000", "1K", "1,000"},
		{9999, "9,999", "9.9K", "9,999"},
		{10000, "1万", "10K", "10,000"},
		{12345, "1.2万", "12.3K", "12,345"},
		{999999, "99.9万", "999.9K", "999,999"},
		{1000000, "100万", "1M", "1,000,000"},
		{100000000, "1亿", "100M", "100,000,000"},
		{4294967297, "42.9亿", "4.2B", "4,294,967,297"},
	} {
		if zh, en, exact := downloadCountText(tc.count, false), downloadCountText(tc.count, true), exactDownloadCount(tc.count); zh != tc.zh || en != tc.en || exact != tc.exact {
			t.Errorf("%d: got %q / %q / %q", tc.count, zh, en, exact)
		}
	}
}

func TestVersionDownloadsAcrossResourceTypes(t *testing.T) {
	for _, source := range []string{"modrinth", "curseforge"} {
		for _, kind := range []string{"modpack", "mod", "resourcepack", "shader"} {
			for _, lang := range []string{LangZH, LangEN} {
				t.Run(source+"/"+kind+"/"+lang, func(t *testing.T) {
					a, tt, _ := projectDialogFixture(t, 5)
					a.OverrideLanguage(lang)
					a.project.Source, a.project.Type = source, kind
					a.project.Title = map[string]string{"modpack": "All the Mods 11 - ATM11", "mod": "Sodium", "resourcepack": "Faithful 32x", "shader": "Complementary Reimagined"}[kind]
					ages := []time.Duration{3*time.Hour + 15*time.Minute, 59*time.Minute + 10*time.Second, time.Hour + 10*time.Second, 20 * time.Second, 23*time.Hour + 10*time.Minute}
					for i, count := range []int64{124803, 0, -1, 9876, 1287654321} {
						a.projectVersions[i].Number = fmt.Sprintf("%s 1.21.1 - release %d", a.project.Title, 5-i)
						a.projectVersions[i].Published = tt.Now().Add(-ages[i]).In(time.FixedZone("API", -7*60*60)).Format(time.RFC3339)
						a.projectVersions[i].Downloads = &count
						if count < 0 {
							a.projectVersions[i].Downloads = nil
						}
					}
					a.dialog, a.dialogOpen = "project", true
					tt.Frame()
					tt.Advance(300 * time.Millisecond)
					countText := downloadCountText(124803, a.isEN())
					if !tt.HasText(countText) || !tt.HasText("0") || !tt.HasText("—") {
						t.Fatalf("per-version count/zero/unknown missing: %v", tt.Texts())
					}
					for _, width := range []int{1536, 900} {
						tt.SetSize(width, map[int]int{1536: 1024, 900: 540}[width])
						tt.Frame()
						row, _ := tt.Find(a.projectVersions[0].Number)
						metric, ok := tt.Find(countText)
						if !ok || metric.X < row.X+row.W/2 || metric.X+metric.W > row.X+row.W+.5 || metric.Y < row.Y || metric.Y+metric.H > row.Y+row.H {
							t.Fatalf("count is clipped or overlaps the title: row=%+v metric=%+v", row, metric)
						}
						for _, v := range a.projectVersions {
							published, _ := time.Parse(time.RFC3339, v.Published)
							text := updatedAgo(a.isEN(), published, tt.Now()) + "  116 MB"
							date, ok := tt.Find(text)
							versionRow, _ := tt.Find(v.Number)
							if !ok || date.X < versionRow.X+versionRow.W/2 || date.X+date.W > versionRow.X+versionRow.W+.5 || date.Y+date.H > versionRow.Y+versionRow.H {
								t.Fatalf("version date/size missing or clipped: text=%q row=%+v date=%+v", text, versionRow, date)
							}
						}
						snapshotUI(t, tt, fmt.Sprintf("version-downloads-%s-%s-%s-%d", source, kind, lang, width))
					}
					metric, _ := tt.Find(countText)
					tt.Move(metric.X+metric.W/2, metric.Y+metric.H/2)
					tt.Advance(time.Second)
					if !strings.Contains(strings.Join(tt.Texts(), " "), "124,803") {
						t.Fatal("hover does not reveal the precise version download count")
					}
					published, _ := time.Parse(time.RFC3339, a.projectVersions[0].Published)
					date, _ := tt.Find(updatedAgo(a.isEN(), published, tt.Now()) + "  116 MB")
					tt.Move(date.X+date.W/2, date.Y+date.H/2)
					tt.Advance(time.Second)
					hint := fmt.Sprintf(a.L("version_metrics.published"), published.Local().Format("2006-01-02 15:04:05"))
					if !strings.Contains(strings.Join(tt.Texts(), "\n"), hint) {
						t.Fatalf("version hover does not show its full local release time: %v", tt.Texts())
					}
				})
			}
		}
	}
}
