package nativeui

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func (a *App) tasks(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	heading(c, t.TasksKicker, t.TasksTitle, t.TasksDesc, func() {
		if a.root() != nil && button(c, "tasks-open-logs", t.TasksLogsBtn, "terminal", false, false) {
			a.openDialog("logs")
		}
	})
	if len(a.snapshot.Jobs) == 0 {
		emptyState(c, "empty-tasks", "tasks", t.TasksEmptyTitle, t.TasksEmptyDesc, func() {
			if button(c, "tasks-install", t.HomeInstallBtn, "plus", true, false) {
				a.openDialog("install")
			}
			if button(c, "tasks-discover", t.NavDiscover, "search", false, false) {
				a.goPage("discover")
			}
		})
	}
	for _, job := range a.snapshot.Jobs {
		j := job
		card(c, "job-"+j.ID, func() {
			ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons["tasks"]).Size(20*s, 20*s).TextColor(accent)
				ui.Text(c, a.jobTitle(j)).Grow(1).Basis(0).MinWidth(0).SingleLine().FontSize(typeTitle * s).Bold().TextColor(ink).Tooltip(a.jobTitle(j))
				color := muted
				if j.State == "failed" {
					color = danger
				} else if j.State == "running" {
					color = accent
				} else if j.State == "done" || j.State == "completed" {
					color = success
				}
				badge(c, a.jobStateText(j.State), color)
				if j.Attempt > 0 {
					quietTag(c, fmt.Sprintf(a.L("重试 %d", "Retry %d"), j.Attempt), muted)
				}
				if j.State == "running" && iconOnlyButton(c, "cancel-"+j.ID, "cross", a.L("取消并清理已下载的文件", "Cancel and clean up"), btnSm*s, false) {
					a.execute("cancelJob", map[string]string{"id": j.ID}, nil)
				}
				if j.State == "failed" && iconOnlyButton(c, "job-retry-"+j.ID, "refresh", a.L("从中断处继续重试", "Retry from where it stopped"), btnSm*s, false) {
					a.execute("retryJob", map[string]string{"id": j.ID}, nil)
				}
				if j.State != "running" {
					tip := a.L("移除任务并清理未完成的文件", "Remove and clean up")
					if j.State == "completed" {
						tip = a.L("从列表移除（不删除内容）", "Remove from list (keeps content)")
					}
					if iconOnlyButton(c, "job-remove-"+j.ID, "trash", tip, btnSm*s, false) {
						a.execute("removeJob", map[string]string{"id": j.ID}, nil)
					}
				}
				if j.State == "completed" && j.ResultRef != "" && a.rootByID(j.ResultRoot) != nil {
					a.packJobButtons(c, j)
				} else if j.State == "completed" && j.ResultRoot != "" && a.rootByID(j.ResultRoot) != nil {
					if button(c, "job-open-"+j.ID, t.TaskOpenDirBtn, "folder", false, false) {
						a.openFolder(j.ResultRoot)
					}
					if button(c, "job-launch-"+j.ID, t.TaskLaunchBtn, "play", true, a.pendingAt("launch", "rootId", j.ResultRoot)) {
						a.launchRoot(j.ResultRoot)
					}
				}
			})
			ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
				msg := a.jobMsg(j.Message)
				if j.State == "failed" {
					msg = classifyError(a.isEN(), msg, nil).Text()
				}
				label(c, msg).Grow(1).Basis(0).MinWidth(0).SingleLine()
				if j.State == "running" {
					for _, m := range jobMetrics(a.isEN(), j) {
						ui.Box(c).Width(m.w * s).Shrink(0).AlignItems(ui.End).Children(func() {
							numText(c, m.text, typeFootnote*s, muted)
						})
					}
				}
				if j.Total > 0 {
					pct := int(math.Round(float64(j.Done) * 100 / float64(j.Total)))
					if j.State == "running" && j.BytesTotal > 0 { // same basis as the title-bar pill
						pct = int(j.BytesDone * 100 / j.BytesTotal)
					}
					ui.Box(c).Width(44 * s).Shrink(0).AlignItems(ui.End).Children(func() {
						numText(c, fmt.Sprintf("%d%%", pct), typeFootnote*s, accent).FontWeight(600)
					})
				}
			})
			if j.Total > 0 {
				frac := float64(j.Done) / float64(j.Total)
				if j.State == "running" && j.BytesTotal > 0 { // match the % and the pill
					frac = float64(j.BytesDone) / float64(j.BytesTotal)
				}
				ui.Progress(c, frac)
			}
			if len(j.Items) > 0 {
				ui.Column(c).FillWidth().Gap(6*s).Padding(12*s).Radius(10*s).Background(raised).Border(1, line).Children(func() {
					label(c, t.TasksItemsHeader).FontSize(typeFootnote * s).FontWeight(600)
					scrollArea(c, "job-items-"+j.ID).MaxHeight(160*s).FillWidth().Padding(0, scrollGutter*s, 0, 0).Children(func() {
						ui.Column(c).FillWidth().Gap(4 * s).Children(func() {
							for i := len(j.Items) - 1; i >= 0; i-- {
								itemText := a.jobMsg(j.Items[i])
								ui.Row(c).Gap(8 * s).AlignItems(ui.Center).Children(func() {
									iconKey := "check"
									iconCol := success
									if i == len(j.Items)-1 && j.State == "running" {
										iconKey = "tasks"
										iconCol = accent
									} else if i == len(j.Items)-1 && j.State == "failed" {
										iconKey, iconCol = "cross", danger // the step that failed
									}
									ui.Icon(c, icons[iconKey]).Size(13*s, 13*s).TextColor(iconCol).Shrink(0)
									label(c, itemText).FontSize(typeCallout * s).SingleLine()
								})
							}
						})
					})
				})
			}
			label(c, j.Started.Local().Format("2006-01-02 15:04:05")).FontSize(typeFootnote * s)
		})
	}
}

type jobMetric struct {
	text string
	w    float32
}

// jobMetrics is size done/total, smoothed speed and ETA, each in a fixed
// width so the row doesn't jitter. Without byte counts (verify/extract)
// it shows nothing and the step name in the message carries the state.
func jobMetrics(en bool, j launcher.Job) []jobMetric {
	if j.BytesDone <= 0 {
		return nil
	}
	size := fmtSize(j.BytesDone)
	if j.BytesTotal > 0 {
		size += " / " + fmtSize(j.BytesTotal)
	}
	out := []jobMetric{{size, 150}}
	if j.Speed < 1024 {
		return out
	}
	out = append(out, jobMetric{fmtSize(int64(j.Speed)) + "/s", 84})
	if j.BytesTotal > j.BytesDone {
		out = append(out, jobMetric{etaText(en, float64(j.BytesTotal-j.BytesDone)/float64(j.Speed)), 110})
	}
	return out
}

func etaText(en bool, sec float64) string {
	n := int(sec + .5)
	if en {
		if n >= 60 {
			return fmt.Sprintf("%dm %ds left", n/60, n%60)
		}
		return fmt.Sprintf("%ds left", n)
	}
	if n >= 60 {
		return fmt.Sprintf("剩余 %d 分 %d 秒", n/60, n%60)
	}
	return fmt.Sprintf("剩余 %d 秒", n)
}
