package nativeui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// jobTitles renders a task title from its key at display time, so it
// follows the UI language and re-renders on a live switch.
var jobTitles = map[string][2]string{
	"pack.download":      {"下载 %s", "Download %s"},
	"pack.update":        {"更新 %s", "Update %s"},
	"pack.rollback":      {"回滚 %s", "Roll back %s"},
	"game.install":       {"安装 Minecraft %s", "Install Minecraft %s"},
	"java.download":      {"下载 Java %s", "Download Java %s"},
	"game.launch":        {"启动 %s", "Launch %s"},
	"world.restore":      {"还原存档 %s", "Restore world %s"},
	"world.backup":       {"备份存档 %s", "Back up world %s"},
	"world.backupAll":    {"备份全部存档%s", "Back up all worlds%s"},
	"mods.installDeps":   {"安装模组及前置%s", "Install mods and dependencies%s"},
	"mods.detect":        {"检测模组版本%s", "Check mod versions%s"},
	"addon.resourcepack": {"安装材质包%s", "Install resource packs%s"},
	"addon.shader":       {"安装光影包%s", "Install shaders%s"},
}

// jobTitle is a task's display name in the current language.
func (a *App) jobTitle(j launcher.Job) string {
	key, arg := j.TitleKey, j.TitleArg
	if key == "" {
		key, arg, _ = launcher.JobTitleKey(j.Title)
	}
	if f, ok := jobTitles[key]; ok {
		if a.isEN() {
			return fmt.Sprintf(f[1], arg)
		}
		return fmt.Sprintf(f[0], arg)
	}
	return j.Title
}

// jobPill is the fixed-width title-bar progress pill: name, thin bar,
// smoothed speed (or the step name) and percent; several tasks aggregate.
func (a *App) jobPill(c *ui.Context) {
	s := uiScale(c)
	var run []launcher.Job
	for _, j := range a.snapshot.Jobs {
		if j.State == "running" || j.State == "queued" {
			run = append(run, j)
		}
	}
	if len(run) == 0 {
		return
	}
	var done, total, speed, bDone, bTotal int64
	for _, j := range run {
		speed += j.Speed
		bDone += j.BytesDone
		bTotal += j.BytesTotal
		done += int64(j.Done)
		total += int64(j.Total)
	}
	frac := 0.0
	switch {
	case bTotal > 0:
		frac = float64(bDone) / float64(bTotal)
	case total > 0:
		frac = float64(done) / float64(total)
	}
	frac = min(1, max(0, frac))
	name := a.jobTitle(run[0])
	tip := name
	if run[0].Total > 0 {
		tip += fmt.Sprintf("  (%d/%d)", run[0].Done, run[0].Total)
	}
	if len(run) > 1 {
		name = fmt.Sprintf(a.L("%d 个任务", "%d tasks"), len(run))
		tip = ""
		for _, j := range run {
			tip += a.jobTitle(j) + "\n"
		}
	}
	right := fmt.Sprintf("%d%%", int(frac*100))
	mid := ""
	if speed >= 1024 {
		mid = fmtSize(speed) + "/s"
	} else if len(run) == 1 {
		mid = jobStep(a.jobMsg(run[0].Message))
	}
	b := ui.ButtonBase(c.Key("title-job")).Label(a.L("查看下载", "View downloads")).Tooltip(strings.TrimSpace(tip)).Width(300 * s).Height(btnSm * s).Shrink(0).Radius(999).Clip().Background(accentSoft)
	if b.Hovered() {
		b.Background(accent.Alpha(.16))
	}
	if b.Children(func() {
		ui.Column(c).Fill().Gap(0).Children(func() {
			ui.Row(c).FillWidth().Grow(1).PaddingX(12 * s).Gap(8 * s).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons["download"]).Size(12*s, 12*s).TextColor(accent).Shrink(0)
				ui.Text(c, name).FontSize(typeFootnote * s).FontWeight(600).TextColor(accent).SingleLine().Grow(1).Basis(0).MinWidth(0)
				ui.Box(c).Width(70 * s).Shrink(0).AlignItems(ui.End).Children(func() {
					numText(c, mid, typeCaption*s, inkSecondary).SingleLine()
				})
				ui.Box(c).Width(32 * s).Shrink(0).AlignItems(ui.End).Children(func() {
					numText(c, right, typeCaption*s, accent).FontWeight(650)
				})
			})
			ui.Row(c).FillWidth().Height(3 * s).Shrink(0).Background(accent.Alpha(.12)).Children(func() {
				ui.Box(c).FillHeight().Width(0).Grow(float32(frac)).Background(accent)
				ui.Box(c).FillHeight().Width(0).Grow(float32(1 - frac))
			})
		})
	}).Clicked() {
		a.goPage("tasks")
	}
}

// jobStep is the step part of a progress message ("正在校验" from
// "正在校验 (3/10): x.jar"), shown instead of a speed.
func jobStep(msg string) string {
	if i := strings.IndexAny(msg, "(（:："); i > 0 {
		msg = msg[:i]
	}
	return strings.TrimSpace(msg)
}
