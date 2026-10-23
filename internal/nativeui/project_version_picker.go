package nativeui

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// verFilter is the install dialog's version picker state, reset per project.
type verFilter struct {
	key        string // project the defaults were picked for
	mc, loader string // "" = any
	allTypes   bool   // false = releases only
}

// mcVersionsOf lists the numeric Minecraft versions in the list, newest
// first (the API lists newest versions first).
func mcVersionsOf(vs []launcher.ProjectVersion) []string {
	var out []string
	for _, v := range vs {
		for _, g := range v.GameVersions {
			if g != "" && g[0] >= '0' && g[0] <= '9' && !slices.Contains(out, g) {
				out = append(out, g)
			}
		}
	}
	slices.SortStableFunc(out, func(x, y string) int { return compareMC(y, x) })
	return out
}

// compareMC orders dotted versions numerically (1.21.10 > 1.21.9).
func compareMC(x, y string) int {
	xs, ys := strings.Split(x, "."), strings.Split(y, ".")
	for i := 0; i < max(len(xs), len(ys)); i++ {
		var a, b int
		if i < len(xs) {
			fmt.Sscan(xs[i], &a)
		}
		if i < len(ys) {
			fmt.Sscan(ys[i], &b)
		}
		if a != b {
			return a - b
		}
	}
	return 0
}

func loadersOf(vs []launcher.ProjectVersion) []string {
	var out []string
	for _, v := range vs {
		for _, l := range v.Loaders {
			if loaderLabel(l) != "" && !slices.Contains(out, l) {
				out = append(out, l)
			}
		}
	}
	return out
}

// visibleVersions applies the chips and the search.
func (a *App) visibleVersions() []launcher.ProjectVersion {
	f := a.verFilter
	q := strings.ToLower(strings.TrimSpace(a.projectVersionFilter))
	var out []launcher.ProjectVersion
	for _, v := range a.projectVersions {
		if f.mc != "" && !slices.Contains(v.GameVersions, f.mc) {
			continue
		}
		if f.loader != "" && !slices.Contains(v.Loaders, f.loader) {
			continue
		}
		if !f.allTypes && v.Type != "" && v.Type != "release" {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(v.Name+" "+v.Number), q) {
			continue
		}
		out = append(out, v)
	}
	return out
}

// pickerChip is one filter chip; selected chips are tinted, never toggles.
func (a *App) pickerChip(c *ui.Context, key, text string, sel bool) bool {
	s := uiScale(c)
	b := ui.ButtonBase(c.Key("vchip-" + key)).Label(text).Height(26 * s).PaddingX(10 * s).Radius(999).Gap(5 * s).AlignItems(ui.Center).Shrink(0)
	bg, fg, edge := ui.RGBA(255, 255, 255, .7), inkSecondary, line
	if sel {
		bg, fg, edge = accentSoft, accent, accent.Alpha(.4)
	} else if b.Hovered() {
		bg = panel
	}
	b.Background(b.AnimateColor("bg", bg, fastAnim)).Border(1, edge)
	return b.Children(func() {
		if sel {
			ui.Icon(c, icons["check"]).Size(9*s, 9*s).TextColor(fg)
		}
		ui.Text(c, text).FontSize(typeFootnote * s).FontWeight(600).FontFeatures("tnum").TextColor(fg).NoWrap()
	}).Clicked()
}

func (a *App) installVersions(c *ui.Context) {
	s := uiScale(c)
	p := a.project
	f := &a.verFilter
	if key := p.Source + p.ID; f.key != key && !a.projLoading && len(a.projectVersions) > 0 {
		*f = verFilter{key: key}
		if p.Type == "modpack" { // default: the newest Minecraft with a release
			for _, v := range a.projectVersions {
				if v.Type == "" || v.Type == "release" {
					if mcs := mcVersionsOf([]launcher.ProjectVersion{v}); len(mcs) > 0 {
						f.mc = mcs[0]
					}
					break
				}
			}
		}
		a.projectVersionFilter = ""
	}
	vis := a.visibleVersions()
	if len(vis) > 0 && !slices.ContainsFunc(vis, func(v launcher.ProjectVersion) bool { return v.ID == a.form.VersionID }) {
		a.form.VersionID = vis[0].ID // keep a visible selection
		if rec := a.recommendedVersion(); slices.ContainsFunc(vis, func(v launcher.ProjectVersion) bool { return v.ID == rec }) {
			a.form.VersionID = rec
		}
	}
	ui.Column(c.Key("ver-picker")).FillWidth().Gap(10 * s).Children(func() {
		ui.Row(c).FillWidth().Height(30 * s).Gap(10 * s).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.L("版本", "Version")).FontSize(typeCallout * s).FontWeight(650).TextColor(ink).Shrink(0)
			if n := len(a.projectVersions); n > 0 {
				cnt := fmt.Sprintf(a.L("%d 个版本", "%d versions"), n)
				if len(vis) != n {
					cnt = fmt.Sprintf(a.L("%d / %d 个版本", "%d of %d versions"), len(vis), n)
				}
				numText(c, cnt, typeFootnote*s, muted)
			}
			ui.Spacer(c).Grow(1)
			textField(c.Key("project-ver-filter"), &a.projectVersionFilter).Label(a.tr().ProjectFilterAria).Placeholder(a.L("搜索版本号", "Search versions")).Width(200 * s).Shrink(1)
		})
		// chips: Minecraft, loader, release type, compatibility
		ui.Row(c.Key("ver-chips")).FillWidth().Height(26 * s).Gap(6 * s).AlignItems(ui.Center).Clip().Children(func() {
			mcs := mcVersionsOf(a.projectVersions)
			if len(mcs) > 1 {
				if a.pickerChip(c, "mc-all", a.L("全部版本", "All versions"), f.mc == "") {
					f.mc = ""
				}
				shown := mcs[:min(4, len(mcs))]
				if f.mc != "" && !slices.Contains(shown, f.mc) {
					shown = append(shown, f.mc)
				}
				for _, m := range shown {
					if a.pickerChip(c, "mc-"+m, m, f.mc == m) {
						f.mc = m
					}
				}
			}
			if ls := loadersOf(a.projectVersions); len(ls) > 1 {
				ui.Box(c).Width(1).Height(16 * s).Background(line).Shrink(0)
				for _, l := range ls {
					if a.pickerChip(c, "ld-"+l, loaderLabel(l), f.loader == l) {
						if f.loader == l {
							f.loader = ""
						} else {
							f.loader = l
						}
					}
				}
			}
			ui.Box(c).Width(1).Height(16 * s).Background(line).Shrink(0)
			if a.pickerChip(c, "release", a.L("只看正式版", "Releases only"), !f.allTypes) {
				f.allTypes = !f.allTypes
			}
			if v := a.gameVersion(); v != nil && p.Type != "modpack" {
				if a.pickerChip(c, "compat", a.L("只看兼容", "Compatible only"), !a.projShowAll) {
					a.projShowAll = !a.projShowAll
					a.reloadProjectVersions()
				}
			}
		})
		// fixed-height list so filtering never moves the dialog
		listH := float32(verRowH*verRows+4*(verRows-1)) * s
		ui.Box(c.Key("ver-list")).FillWidth().Height(listH).Radius(12*s).Border(1, line).Background(ui.RGBA(255, 255, 255, .55)).Clip().Children(func() {
			switch {
			case a.projLoading:
				ui.Row(c).FillWidth().FillHeight().Justify(ui.Center).AlignItems(ui.Center).Gap(10 * s).Children(func() {
					statusSpinner(c, 16*s)
					label(c, a.L("正在读取版本", "Loading versions"))
				})
			case len(vis) == 0:
				ui.Column(c).FillWidth().FillHeight().Justify(ui.Center).AlignItems(ui.Center).Gap(10 * s).Children(func() {
					msg := a.L("没有符合筛选的版本", "No version matches")
					if len(a.projectVersions) == 0 {
						msg = a.L("没有适配当前实例的版本", "No version fits this instance")
					}
					ui.Text(c, msg).FontSize(typeCallout * s).FontWeight(600).TextColor(inkSecondary)
					if fixedButton(c, "ver-reset", a.L("显示全部版本", "Show all versions"), "", 0, btnSm*s, false, false) {
						f.mc, f.loader, f.allTypes, a.projectVersionFilter = "", "", true, ""
						if len(a.projectVersions) == 0 && !a.projShowAll {
							a.projShowAll = true
							a.reloadProjectVersions()
						}
					}
				})
			default:
				rec := a.recommendedVersion()
				scrollArea(c, "project-versions-scroll").FillWidth().FillHeight().Children(func() {
					ui.Column(c).FillWidth().Gap(4*s).Padding(4*s, scrollGutter*s+4*s, 4*s, 4*s).Children(func() {
						for _, v := range vis {
							a.versionRow(c, v, v.ID == rec)
						}
					})
				})
			}
		})
	})
}

func (a *App) reloadProjectVersions() {
	p := a.project
	if p == nil {
		return
	}
	mc, loader := "", ""
	if v := a.gameVersion(); v != nil && !a.projShowAll {
		if p.Type == "mod" {
			mc, loader = v.Minecraft, v.Loader
		} else if p.Type == "resourcepack" || p.Type == "shader" {
			mc = v.Minecraft
		}
	}
	a.projLoading = true
	a.form.VersionID = ""
	key := p.Source + p.ID
	a.execute("versions", map[string]string{"source": p.Source, "projectId": p.ID, "minecraft": mc, "loader": loader}, func(b json.RawMessage) {
		if a.project == nil || a.project.Source+a.project.ID != key {
			return
		}
		a.projLoading = false
		a.projectVersions = decode[[]launcher.ProjectVersion](b)
	})
}

// versionRow is one fixed-height line: number, Minecraft, loader, type,
// then date and size aligned right.
func (a *App) versionRow(c *ui.Context, v launcher.ProjectVersion, recommended bool) {
	s := uiScale(c)
	sel := v.ID == a.form.VersionID
	b := ui.ButtonBase(c.Key("proj-ver-" + v.ID)).Label(v.Number).FillWidth().Height(verRowH * s).PaddingX(12 * s).Radius(9 * s).Gap(10 * s).AlignItems(ui.Center)
	bg, edge := ui.Transparent, ui.Transparent
	if sel {
		bg, edge = accentSoft, accent.Alpha(.5)
	} else if b.Hovered() {
		bg = ui.RGBA(120, 130, 160, .07)
	}
	b.Background(b.AnimateColor("bg", bg, fastAnim)).Border(1, b.AnimateColor("edge", edge, fastAnim))
	if b.Children(func() {
		num := v.Number
		if num == "" {
			num = v.Name
		}
		fg := ink
		if sel {
			fg = accent
		}
		ui.Text(c, num).FontFeatures("tnum").FontSize(typeCallout * s).FontWeight(650).TextColor(fg).SingleLine().Width(150 * s).Shrink(1).MinWidth(60 * s).Tooltip(v.Name)
		ui.Row(c).Grow(1).MinWidth(0).Gap(6 * s).AlignItems(ui.Center).Clip().Children(func() {
			if mcs := mcVersionsOf([]launcher.ProjectVersion{v}); len(mcs) > 0 {
				txt := mcs[0]
				if len(mcs) > 1 {
					txt = fmt.Sprintf("%s +%d", mcs[0], len(mcs)-1)
				}
				quietTag(c, txt, inkSecondary)
			}
			for _, l := range v.Loaders {
				if lbl := loaderLabel(l); lbl != "" {
					quietTag(c, lbl, inkSecondary)
				}
			}
			rl, tone := a.releaseLabel(v.Type)
			quietTag(c, rl, tone)
			if recommended {
				quietTag(c, a.L("推荐", "Recommended"), successInk)
			}
		})
		date := ""
		if tm, e := time.Parse(time.RFC3339, v.Published); e == nil {
			date = updatedAgo(a.isEN(), tm, time.Now())
		}
		numText(c, date, typeCaption*s, muted).Width(64 * s).TextAlign(ui.End).Shrink(0)
		size := ""
		for _, fl := range v.Files {
			if (fl.Primary || len(v.Files) == 1) && fl.Size > 0 {
				size = sizeText(fl.Size)
				break
			}
		}
		numText(c, size, typeCaption*s, muted).Width(60 * s).TextAlign(ui.End).Shrink(0)
	}).Clicked() {
		a.form.VersionID = v.ID
	}
}
