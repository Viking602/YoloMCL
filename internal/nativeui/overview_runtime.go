package nativeui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func jvmPresetName(a *App, args string) string {
	switch strings.TrimSpace(args) {
	case "":
		return a.L("默认", "Default")
	case jvmPresetG1:
		return a.L("G1 优化", "G1 tuned")
	case jvmPresetZGC:
		return "ZGC"
	case jvmPresetCompact:
		return a.L("低内存", "Low memory")
	}
	return a.L("自定义", "Custom")
}

// ovJava picks the runtime this version will use: its own path, else a
// detected Java of the required major.
func (a *App) ovJava(r launcher.Instance, v launcher.VersionSummary) (launcher.JavaInfo, int) {
	need := javaFor(r, v)
	for _, j := range a.java {
		if v.JavaPath != "" && strings.EqualFold(j.Path, v.JavaPath) {
			return j, need
		}
	}
	if v.JavaPath == "" {
		for _, j := range a.java {
			if j.Major == need {
				return j, need
			}
		}
	}
	return launcher.JavaInfo{}, need
}

func (a *App) ovRuntime(c *ui.Context, r launcher.Instance, v *launcher.VersionSummary, st launcher.InstanceStatus) {
	s := uiScale(c)
	if v == nil {
		label(c, a.tr().HomeHeroNoVersion)
		return
	}
	j, need := a.ovJava(r, *v)
	kvRow(c, "Java", func() {
		text := a.L("自动匹配", "Automatic")
		if need > 0 {
			text = fmt.Sprintf("Java %d", need)
		}
		if j.Version != "" {
			text = "Java " + j.Version
			if j.Vendor != "" {
				text += "  " + j.Vendor
			}
		}
		shown := swappedContent(a, c, "runtime-java", text, text)
		a.contentBody(c, "runtime-java").Grow(1).Basis(0).MinWidth(0).Children(func() {
			tooltip(c, ui.Text(c, shown).FontSize(typeCallout*s).FontWeight(600).TextColor(ink).SingleLine().FillWidth(), j.Path)
		})
		switch {
		case j.Major > 0 && j.Major == need:
			quietTag(c, a.L("推荐", "Recommended"), success)
		case j.Major > 0 && need > 0:
			quietTag(c, fmt.Sprintf(a.L("建议 Java %d", "Java %d advised"), need), warning)
		case j.Version == "" && need > 0:
			quietTag(c, a.L("启动时下载", "Downloads on launch"), muted)
		}
	})
	auto, mb := r.MemoryPlan(v.ID, a.settings)
	if a.memoryInfo.TotalMB == 0 {
		a.refreshMemory()
	}
	if auto && mb <= 0 {
		mb = a.versionAutoMemory(r, *v, st)
	}
	mem := fmt.Sprintf("%.1f GB", float64(mb)/1024)
	if auto {
		mem = a.L("自动  约 ", "Auto  about ") + mem
	}
	kvRow(c, a.L("内存", "Memory"), func() {
		a.rollNumber(c, "runtime-memory", mem, typeCallout*s, 600, ink).Width(188 * s).Shrink(1)
		if a.memoryInfo.TotalMB > 0 {
			numText(c, fmt.Sprintf(a.L("系统 %.0f GB", "of %.0f GB"), float64(a.memoryInfo.TotalMB)/1024), typeFootnote*s, muted)
		}
	})
	kvRow(c, a.L("JVM 参数", "JVM args"), func() {
		text := jvmPresetName(a, v.JVMArgs)
		text = swappedContent(a, c, "runtime-jvm", text, text)
		a.contentBody(c, "runtime-jvm").Children(func() { valueText(c, text) })
	})
	if d := st.Dir; d != "" {
		kvRow(c, a.L("版本隔离", "Isolation"), func() {
			if !strings.EqualFold(filepath.Clean(d), filepath.Clean(r.Path)) {
				valueText(c, a.L("开启", "On"))
				numText(c, a.L("模组与存档独立存放", "own mods and worlds"), typeFootnote*s, muted)
			} else {
				valueText(c, a.L("关闭", "Off"))
				numText(c, a.L("与同目录其他版本共用", "shared with other versions"), typeFootnote*s, muted)
			}
		})
	} else {
		kv(c, a.L("版本隔离", "Isolation"), "—")
	}
}
