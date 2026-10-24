package nativeui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

var dirParts = []struct {
	sub, zh, en, tab string
	tone             ui.Color
}{
	{"mods", "模组", "Mods", "mods", ui.Hex("#007aff")},
	{"resourcepacks", "资源包", "Resource packs", "resourcepack", ui.Hex("#34c759")},
	{"shaderpacks", "光影", "Shaders", "shader", ui.Hex("#ff9500")},
	{"saves", "存档", "Worlds", "worlds", ui.Hex("#af52de")},
	{"screenshots", "截图", "Screenshots", "", ui.Hex("#5ac8fa")},
	{"other", "其他", "Other", "", ui.Hex("#8b96b0")},
}

func storageBytes(st launcher.InstanceStatus, sub string) int64 {
	if sub != "other" {
		return st.Sizes[sub]
	}
	other := st.Total
	for _, part := range dirParts {
		if part.sub != "other" {
			other -= st.Sizes[part.sub]
		}
	}
	return max(0, other)
}

func (a *App) ovDir(c *ui.Context, st launcher.InstanceStatus, r launcher.Instance, _ bool) {
	s := uiScale(c)
	if st.Dir == "" {
		ovLoading(c)
		return
	}
	dir := st.Dir
	if dir == "" {
		dir = r.Path
	}
	ui.Row(c.Key("storage-bar")).Label("容量占比").FillWidth().Height(8*s).Shrink(0).Radius(4*s).Clip().Background(fieldColor).Margin(4*s, 0, 8*s, 0).Children(func() {
		for _, p := range dirParts {
			n, share := storageBytes(st, p.sub), float32(0)
			if st.Total > 0 {
				share = float32(float64(n) / float64(st.Total))
			}
			segment := ui.Box(c.Key("storage-segment-" + p.sub)).Label("存储占比 " + p.sub).FillHeight().Basis(0).MinWidth(0).Background(p.tone).Tooltip(a.L(p.zh, p.en) + "  " + fmtSize(n))
			segment.Grow(segment.AnimateTo("share", share, motionPack))
		}
	})
	updates := 0
	for _, m := range a.addonCache[r.ID+"|"+r.SelectedVersion+"|mods"].mods {
		if m.Latest != nil {
			updates++
		}
	}
	counts := map[string]int{"resourcepacks": st.ResourcePacks, "shaderpacks": st.Shaders, "saves": st.Saves, "screenshots": st.Screenshots}
	units := map[string][2]string{"resourcepacks": {"%d 个", "%d packs"}, "shaderpacks": {"%d 个", "%d shaders"}, "saves": {"%d 个世界", "%d worlds"}, "screenshots": {"%d 张", "%d shots"}}
	for _, p := range dirParts {
		p := p
		size := storageBytes(st, p.sub)
		n := counts[p.sub]
		if p.sub != "mods" && size == 0 && n == 0 {
			continue // empty: nothing to show
		}
		b := ui.ButtonBase(c.Key("ov-dir-"+p.sub)).Label(a.L(p.zh, p.en)).FillWidth().Height(30*s).Shrink(0).PaddingX(8*s).Margin(0, -8*s, 0, -8*s).Radius(7 * s).Gap(10 * s).AlignItems(ui.Center)
		if !c.Preferences().ReduceMotion {
			b.Transition(ui.ElementTransition{Animation: &motionPack, Position: true, Enter: &ui.Motion{Opacity: 0, Y: 6 * s}})
		}
		bg := ui.Transparent
		if b.Hovered() {
			bg = accentSoft
		}
		b.Background(b.AnimateColor("bg", bg, fastAnim))
		if b.Children(func() {
			ui.Box(c).Size(8*s, 8*s).Radius(2 * s).Background(p.tone).Shrink(0)
			ui.Box(c).Width(116 * s).Shrink(0).Clip().Children(func() { label(c, a.L(p.zh, p.en)).FontSize(typeCallout * s).NoWrap() })
			ui.Row(c).Grow(1).Basis(0).MinWidth(0).Gap(8 * s).AlignItems(ui.Center).Clip().Children(func() {
				if p.sub == "other" {
					label(c, a.L("配置、日志、备份等", "Config, logs, backups…")).FontSize(typeCaption * s).SingleLine().MinWidth(0).Shrink(1)
				}
				if u, ok := units[p.sub]; ok {
					key := "storage-count-" + p.sub
					text := fmt.Sprintf(a.L(u[0], u[1]), n)
					a.rollNumber(c, key, text, typeCallout*s, 600, ink).FillWidth()
				}
				if p.sub == "mods" && updates > 0 {
					quietTag(c, fmt.Sprintf(a.L("%d 个可更新", "%d updates"), updates), accent)
				}
			})
			ui.Box(c).Width(80 * s).Shrink(0).Children(func() {
				key := "storage-size-" + p.sub
				text := sizeOrDash(size)
				a.rollNumber(c, key, text, typeFootnote*s, 400, inkSecondary, true).FillWidth()
			})
			ui.Box(c).Width(44 * s).Shrink(0).AlignItems(ui.End).Children(func() {
				text := ""
				if st.Total > 0 && size > 0 {
					pct := float64(size) * 100 / float64(st.Total)
					text = fmt.Sprintf("%.0f%%", pct)
					if pct < 1 {
						text = "<1%"
					}
				}
				key := "storage-percent-" + p.sub
				a.rollNumber(c, key, text, typeFootnote*s, 400, muted, true).FillWidth()
			})
			ui.Icon(c, icons["chevron"]).Size(10*s, 10*s).TextColor(muted).Shrink(0)
		}).Clicked() && a.overviewActionReady("ov-dir") {
			if p.sub == "other" {
				a.openSub("")
			} else if p.tab != "" {
				a.setInstTab(p.tab)
			} else {
				a.openSub(p.sub)
			}
		}
	}
	ui.Spacer(c).Grow(1)
	ui.Row(c).FillWidth().Height(32 * s).Shrink(0).Gap(8 * s).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Grow(1).Basis(0).MinWidth(0).Clip().Tooltip(dir).Children(func() {
			ww, _ := c.Size()
			n := 44
			if ww < 1700*s {
				n = 28
			}
			text := midPath(dir, n)
			text = swappedContent(a, c, "storage-path", text, text)
			a.contentBody(c, "storage-path").Children(func() { ui.Text(c, text).FontSize(typeFootnote * s).TextColor(muted).SingleLine().FillWidth() })
		})
		if fixedButton(c, "ov-copy-path", a.L("复制路径", "Copy path"), "doc", 0, btnSm*s, false, false) && a.overviewActionReady("storage-path") {
			c.WriteClipboard(dir)
			a.notice(a.L("已复制路径", "Path copied"))
		}
		if fixedButton(c, "ov-open-folder", a.L("打开文件夹", "Open folder"), "folder", 0, btnSm*s, false, false) && a.overviewActionReady("storage-path") {
			a.openSub("")
		}
	})
}

func sizeOrDash(n int64) string {
	if n <= 0 {
		return "—"
	}
	return fmtSize(n)
}
