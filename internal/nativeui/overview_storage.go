package nativeui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

var dirParts = []struct {
	sub, tab string
	tone     ui.Color
}{
	{"mods", "mods", ui.Hex("#007aff")},
	{"resourcepacks", "resourcepack", ui.Hex("#34c759")},
	{"shaderpacks", "shader", ui.Hex("#ff9500")},
	{"saves", "worlds", ui.Hex("#af52de")},
	{"screenshots", "", ui.Hex("#5ac8fa")},
	{"other", "", ui.Hex("#8b96b0")},
}

func storageBytes(st launcher.InstanceStatus, sub string) int64 {
	if sub != "other" {
		return max(0, st.Sizes[sub])
	}
	other := max(0, st.Total)
	for _, part := range dirParts {
		if part.sub != "other" {
			other = max(0, other-max(0, st.Sizes[part.sub]))
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
	highlighted := ""
	a.storageBar(c, st, &highlighted)
	updates := 0
	for _, m := range a.addonCache[r.ID+"|"+r.SelectedVersion+"|mods"].mods {
		if m.Latest != nil {
			updates++
		}
	}
	counts := map[string]int{"mods": st.Mods, "resourcepacks": st.ResourcePacks, "shaderpacks": st.Shaders, "saves": st.Saves, "screenshots": st.Screenshots}
	for _, p := range dirParts {
		p := p
		size := storageBytes(st, p.sub)
		n := counts[p.sub]
		if p.sub != "mods" && size == 0 && n == 0 {
			continue // empty: nothing to show
		}
		b := ui.ButtonBase(c.Key("ov-dir-"+p.sub)).Label(a.L("storage.label."+p.sub)).FillWidth().Height(30*s).Shrink(0).PaddingX(8*s).Margin(0, -8*s, 0, -8*s).Radius(radiusChip * s).Gap(12 * s).AlignItems(ui.Center)
		if !c.Preferences().ReduceMotion {
			b.Transition(ui.ElementTransition{Animation: &motionFast, Enter: &ui.Motion{Opacity: 0}})
		}
		bg := ui.Transparent
		if b.Hovered() || b.FocusVisible() {
			highlighted = p.sub
		}
		if highlighted == p.sub {
			bg = accentSoft
		}
		b.Background(b.AnimateColor("bg", bg, fastAnim))
		if b.Children(func() {
			ui.Box(c).Size(8*s, 8*s).Radius(radiusMin * s).Background(p.tone).Shrink(0)
			ui.Box(c).Width(116 * s).Shrink(0).Clip().Children(func() { label(c, a.L("storage.label."+p.sub)).FontSize(typeCallout * s).NoWrap() })
			ui.Row(c).Grow(1).Basis(0).MinWidth(0).Gap(8 * s).AlignItems(ui.Center).Clip().Children(func() {
				if p.sub == "other" {
					label(c, a.L("overview_storage.config_logs_backups")).FontSize(typeCaption * s).SingleLine().MinWidth(0).Shrink(1)
				}
				if p.sub == "mods" && updates > 0 {
					quietTag(c, fmt.Sprintf(a.L("overview_storage.updates"), updates), accent)
				} else if _, ok := counts[p.sub]; ok {
					key := "storage-count-" + p.sub
					text := fmt.Sprintf(a.L("storage.count."+p.sub), n)
					a.fadeNumber(c, key, text, typeCallout*s, 600, ink).FillWidth()
				}
			})
			ui.Box(c).Width(80 * s).Shrink(0).Children(func() {
				key := "storage-size-" + p.sub
				text := sizeOrDash(size)
				a.fadeNumber(c, key, text, typeFootnote*s, 400, inkSecondary, true).FillWidth()
			})
			ui.Box(c).Width(44 * s).Shrink(0).AlignItems(ui.End).Children(func() {
				text := storagePercent(size, st.Total)
				key := "storage-percent-" + p.sub
				a.fadeNumber(c, key, text, typeFootnote*s, 400, muted, true).FillWidth()
			})
			ui.Icon(c, icons["chevron"]).Size(10*s, 10*s).TextColor(muted).Shrink(0)
		}).Clicked() {
			a.openStoragePart(p.sub)
		}
	}
	ui.Spacer(c).Grow(1)
	ui.Row(c).FillWidth().Height(32 * s).Shrink(0).Gap(8 * s).AlignItems(ui.Center).Children(func() {
		tooltip(c, ui.Box(c).Grow(1).Basis(0).MinWidth(0).Clip(), dir).Children(func() {
			ww, _ := c.Size()
			n := 44
			if ww < 1700*s {
				n = 28
			}
			text := midPath(dir, n)
			text = swappedContent(a, c, "storage-path", text, text)
			a.contentBody(c, "storage-path").Children(func() { ui.Text(c, text).FontSize(typeFootnote * s).TextColor(muted).SingleLine().FillWidth() })
		})
		if fixedButton(c, "ov-copy-path", a.L("java_installed.copy_path"), "doc", 0, btnSm*s, false, false) && a.overviewActionReady("storage-path") {
			c.WriteClipboard(dir)
			a.notice(a.L("java_installed.path_copied"))
		}
		if fixedButton(c, "ov-open-folder", a.L("addons_tab.open_folder"), "folder", 0, btnSm*s, false, false) && a.overviewActionReady("storage-path") {
			a.openSub("")
		}
	})
}

func (a *App) openStoragePart(sub string) {
	if !a.overviewActionReady("ov-dir") {
		return
	}
	for _, part := range dirParts {
		if part.sub != sub {
			continue
		}
		switch {
		case sub == "other":
			a.openSub("")
		case part.tab != "":
			a.setInstTab(part.tab)
		default:
			a.openSub(sub)
		}
		return
	}
}

func storagePercent(size, total int64) string {
	if total <= 0 || size <= 0 {
		return ""
	}
	pct := min(100, float64(size)*100/float64(total))
	if pct < 1 {
		return "<1%"
	}
	return fmt.Sprintf("%.0f%%", pct)
}

func sizeOrDash(n int64) string {
	if n <= 0 {
		return "—"
	}
	return fmtSize(n)
}
