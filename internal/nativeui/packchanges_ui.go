package nativeui

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

var (
	amberSoft = ui.Hex("#fff4e0")
	amberLine = ui.Hex("#f5c46b")
	slate     = ui.Hex("#5b6b80")
	slateSoft = ui.Hex("#eef2f6")
)

const (
	changeRowH   = 28 // DIPs per file row
	changeRowCap = 8  // rows shown before the group scrolls
)

type changeGroup struct {
	key, title string
	items      []launcher.PackChange
	color      ui.Color
}

// packChangeGroups is the compact grouped change list used by the update
// dialog and 查看变更: slim clickable headers, rows that ease open/closed.
func (a *App) packChangeGroups(c *ui.Context, p *launcher.PackUpdatePlan) {
	s := uiScale(c)
	t := a.tr()
	groups := []changeGroup{
		{"overwritten", t.PackChOverwritten, p.Overwritten, warning},
		{"updated", t.PackChUpdated, p.Updated, accent},
		{"added", t.PackChAdded, p.Added, success},
		{"removed", t.PackChRemoved, p.Removed, danger},
		{"manual", t.PackChManual, p.Manual, warning},
	}
	shown := 0
	for _, g := range groups {
		if len(g.items) > 0 {
			shown++
		}
	}
	if shown == 0 {
		return
	}
	ui.Column(c.Key("pack-changes")).Label("变更列表").FillWidth().Radius(12*s).Background(panel).Border(1, line).Clip().Children(func() {
		first := true
		for _, g := range groups {
			if len(g.items) == 0 {
				continue
			}
			if !first {
				ui.Box(c).FillWidth().Height(1).Background(line)
			}
			first = false
			a.changeGroupView(c, s, g)
		}
	})
}

func (a *App) changeGroupView(c *ui.Context, s float32, g changeGroup) {
	// mods first, then config and other files, each alphabetical
	items := slices.Clone(g.items)
	isMod := func(p string) bool {
		return strings.HasPrefix(strings.ToLower(strings.ReplaceAll(p, "\\", "/")), "mods/")
	}
	mods := 0
	for _, it := range items {
		if isMod(it.Path) {
			mods++
		}
	}
	slices.SortStableFunc(items, func(x, y launcher.PackChange) int {
		if mx, my := isMod(x.Path), isMod(y.Path); mx != my {
			if mx {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(x.Path), strings.ToLower(y.Path))
	})
	g.items = items
	open := a.packExpand[g.key]
	header := ui.ButtonBase(c.Key("pack-grp-" + g.key)).Label("变更分组 " + g.key).FillWidth().Height(38 * s).PaddingX(14 * s).Gap(10 * s).AlignItems(ui.Center)
	bg := ui.RGBA(255, 255, 255, 0)
	if header.Hovered() {
		bg = fieldColor
	}
	header.Background(header.AnimateColor("bg", bg, motionFast))
	target, anim := float32(0), motionExit
	if open {
		target, anim = 1, motionEnter
	}
	progress := header.AnimateTo("open", target, anim)
	if header.Children(func() {
		ui.Box(c).Size(8*s, 8*s).Radius(999).Background(g.color).Shrink(0)
		ui.Text(c, g.title).FontSize(typeCallout * s).FontWeight(650).TextColor(ink).SingleLine().Shrink(1).MinWidth(0)
		ui.Row(c).Shrink(0).Height(18 * s).PaddingX(7 * s).Radius(999).Background(fieldColor).Center().Children(func() {
			ui.Text(c, fmt.Sprint(len(g.items))).FontSize(typeCaption2 * s).FontWeight(650).TextColor(inkSecondary).NoWrap()
		})
		if mods > 0 && mods < len(g.items) {
			ui.Text(c, fmt.Sprintf(a.L("模组 %d  其他文件 %d", "%d mods  %d other files"), mods, len(g.items)-mods)).FontSize(typeCaption * s).FontFeatures("tnum").TextColor(muted).NoWrap()
		}
		ui.Spacer(c).Grow(1)
		ic := "chevron"
		if open {
			ic = "chevron-down"
		}
		ui.Icon(c, icons[ic]).Size(11*s, 11*s).TextColor(muted).Shrink(0)
	}).Clicked() {
		if a.packExpand == nil {
			a.packExpand = map[string]bool{}
		}
		a.packExpand[g.key] = !open
	}
	if progress <= .01 {
		return
	}
	rows := min(len(g.items), changeRowCap)
	full := float32(rows)*changeRowH*s + 8*s
	body := scrollArea(c, "pack-grp-body-"+g.key).Label("变更文件 "+g.key).FillWidth().Height(full*progress).Clip().Opacity(progress).Padding(0, 0, 8*s, 0)
	body.Children(func() {
		for _, it := range g.items {
			a.changeRow(c, s, g, it)
		}
	})
}

// changeRow: muted folder, emphasised file name (the folder shrinks first,
// so long paths are shortened in the middle), aligned reason tags.
func (a *App) changeRow(c *ui.Context, s float32, g changeGroup, it launcher.PackChange) {
	t := a.tr()
	dir, name := path.Split(strings.ReplaceAll(it.Path, "\\", "/"))
	ui.Row(c).Label("变更文件行").FillWidth().Height(changeRowH*s).Padding(0, 14*s, 0, 32*s).Gap(8 * s).AlignItems(ui.Center).Tooltip(it.Path).Children(func() {
		ui.Row(c).Grow(1).Basis(0).MinWidth(0).AlignItems(ui.Center).Children(func() {
			if dir != "" {
				ui.Text(c, dir).FontSize(typeCaption * s).TextColor(muted).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
			}
			ui.Text(c, name).FontSize(typeFootnote * s).FontWeight(600).TextColor(ink).SingleLine().Ellipsis("…").Shrink(0).MaxWidthPercent(85)
		})
		switch it.Reason {
		case "modified":
			reasonTag(c, s, t.PackReasonModified, warningInk, amberSoft, amberSoft)
		case "user":
			reasonTag(c, s, t.PackReasonUser, warningInk, panel, amberLine)
		case "missing":
			reasonTag(c, s, t.PackReasonMissing, successInk, ui.Hex("#e8faed"), ui.Hex("#e8faed"))
		case "renamed":
			reasonTag(c, s, t.PackReasonRenamed, accent, ui.Hex("#eaf3ff"), ui.Hex("#eaf3ff"))
		case "kept-old":
			reasonTag(c, s, t.PackReasonKeptOld, warningInk, amberSoft, amberSoft)
		}
		if it.Disabled {
			reasonTag(c, s, t.PackKeepDisabled, slate, slateSoft, slateSoft)
		}
	})
}

func reasonTag(c *ui.Context, s float32, text string, fg, bg, border ui.Color) {
	ui.Row(c).Shrink(0).Height(20*s).PaddingX(8*s).Radius(999).Background(bg).Border(1, border).Center().Children(func() {
		ui.Text(c, text).FontSize(typeCaption2 * s).FontWeight(600).TextColor(fg).NoWrap()
	})
}
