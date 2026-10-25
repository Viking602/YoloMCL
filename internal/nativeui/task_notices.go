package nativeui

import (
	"fmt"
	"strings"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func jobSignature(jobs []launcher.Job) string {
	var b strings.Builder
	for _, j := range jobs {
		b.WriteString(fmt.Sprintf("%s:%s:%d:%s|", j.ID, j.State, j.Done, j.Message))
	}
	return b.String()
}

// applyJobNotice replaces the "任务已启动" notice once tasks finish: 导入完成
// for a successful import, the failure reason otherwise.
func (a *App) applyJobNotice(old, cur []launcher.Job) {
	t := a.tr()
	was := map[string]string{}
	for _, j := range old {
		was[j.ID] = j.State
	}
	for _, j := range cur {
		if was[j.ID] != "running" || j.State == "running" {
			continue
		}
		if a.javaJobEnded(j) {
			continue
		}
		switch {
		case j.State == "completed" && j.TitleKey == "pack.export":
			a.message, a.errorMessage = a.L("整合包已导出，可在任务中查看文件。", "Modpack exported. Show the file from Tasks."), false
		case j.State == "completed" && j.TitleKey == "pack.update":
			a.message, a.errorMessage = t.PackUpdateDone, false
		case j.State == "completed" && j.TitleKey == "pack.rollback":
			a.message, a.errorMessage = t.PackRollbackDone, false
		case j.State == "failed" && (j.TitleKey == "pack.update" || j.TitleKey == "pack.rollback"):
			a.message, a.errorMessage = classifyError(a.isEN(), j.Message, nil).Text(), true // no URLs in the toast
		case j.State == "completed" && j.TitleKey == "pack.download":
			a.message, a.errorMessage = t.ImportDoneNotice, false
		case j.State == "failed" && a.message == t.InstallTaskCreated:
			a.message, a.errorMessage = j.Message, true
		case a.message == t.InstallTaskCreated:
			a.message = ""
		}
	}
}
