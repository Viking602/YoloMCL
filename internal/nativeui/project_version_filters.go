package nativeui

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type verFilter struct {
	key        string
	mc, loader string // empty = any
	allTypes   bool
}

func mcVersionsOf(vs []launcher.ProjectVersion) []string {
	var out []string
	seen := map[string]bool{}
	for _, v := range vs {
		for _, g := range v.GameVersions {
			if g != "" && !seen[g] {
				seen[g] = true
				out = append(out, g)
			}
		}
	}
	slices.SortStableFunc(out, func(x, y string) int { return compareMC(y, x) })
	return out
}

var versionTokens = regexp.MustCompile(`[0-9]+|[^0-9]+`)

// Natural ordering includes the qualifier's number: snapshot-10 > snapshot-5,
// rc2 > rc1, and a final release follows its pre-releases.
func compareMC(x, y string) int {
	xs, ys := versionTokens.FindAllString(x, -1), versionTokens.FindAllString(y, -1)
	for i := 0; i < min(len(xs), len(ys)); i++ {
		xn, xe := strconv.Atoi(xs[i])
		yn, ye := strconv.Atoi(ys[i])
		if xe == nil && ye == nil {
			if n := cmp.Compare(xn, yn); n != 0 {
				return n
			}
		} else if n := strings.Compare(xs[i], ys[i]); n != 0 {
			phase := func(s string) int {
				switch {
				case strings.HasPrefix(s, "-rc"):
					return 3
				case strings.HasPrefix(s, "-pre"):
					return 2
				case strings.HasPrefix(s, "-snapshot"):
					return 1
				}
				return 0
			}
			if xp, yp := phase(xs[i]), phase(ys[i]); xp > 0 && yp > 0 && xp != yp {
				return cmp.Compare(xp, yp)
			}
			return n
		}
	}
	if len(xs) < len(ys) && strings.HasPrefix(ys[len(xs)], "-") {
		return 1
	}
	if len(ys) < len(xs) && strings.HasPrefix(xs[len(ys)], "-") {
		return -1
	}
	return cmp.Compare(len(xs), len(ys))
}

func loadersOf(vs []launcher.ProjectVersion) []string {
	var out []string
	for _, v := range vs {
		for _, l := range v.Loaders {
			l = strings.ToLower(l)
			if loaderLabel(l) != "" && !slices.Contains(out, l) {
				out = append(out, l)
			}
		}
	}
	slices.Sort(out)
	return out
}

func hasLoader(v launcher.ProjectVersion, loader string) bool {
	return slices.ContainsFunc(v.Loaders, func(l string) bool { return strings.EqualFold(l, loader) })
}

func (a *App) projectVersionFits(v launcher.ProjectVersion) bool {
	g := a.gameVersion()
	if g == nil || a.project == nil || a.project.Type == "modpack" {
		return true
	}
	if g.Minecraft != "" && !slices.Contains(v.GameVersions, g.Minecraft) {
		return false
	}
	if a.project.Type == "mod" {
		return g.Loader != "" && g.Loader != "vanilla" && hasLoader(v, g.Loader)
	}
	return true
}

func (a *App) selectedProjectVersionFits() bool {
	for _, v := range a.projectVersions {
		if v.ID == a.form.VersionID {
			return a.projectVersionFits(v)
		}
	}
	return false
}

func (a *App) initProjectFilters() {
	key := a.project.Source + ":" + a.project.ID
	if a.verFilter.key == key {
		return
	}
	a.verFilter = verFilter{key: key, allTypes: true}
	if a.project.Type != "modpack" && !a.projShowAll {
		a.filterForInstance()
	}
}

func (a *App) filterForInstance() {
	if v := a.gameVersion(); v != nil {
		a.verFilter.mc = v.Minecraft
		if a.project.Type == "mod" {
			a.verFilter.loader = strings.ToLower(v.Loader)
			if a.verFilter.loader == "vanilla" {
				a.verFilter.loader = ""
			}
		}
	}
}

func (a *App) visibleVersions() []launcher.ProjectVersion {
	f := a.verFilter
	q := strings.ToLower(strings.TrimSpace(a.projectVersionFilter))
	var out []launcher.ProjectVersion
	for _, v := range a.projectVersions {
		if (f.mc != "" && !slices.Contains(v.GameVersions, f.mc)) ||
			(f.loader != "" && !hasLoader(v, f.loader)) ||
			(!f.allTypes && v.Type != "" && v.Type != "release") ||
			(!a.projShowAll && !a.projectVersionFits(v)) ||
			(q != "" && !strings.Contains(strings.ToLower(v.Name+" "+v.Number), q)) {
			continue
		}
		out = append(out, v)
	}
	return out
}

func (a *App) projectFilterControls(c *ui.Context) {
	s, f := uiScale(c), &a.verFilter
	ui.Row(c.Key("project-filters")).Label("项目版本筛选").FillWidth().Gap(10 * s).Children(func() {
		ui.Box(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			if a.projectMinecraftChoice(c, mcVersionsOf(a.projectVersions)) {
				a.projShowAll = true
				// An instance's default loader must not hide every file when
				// browsing a Minecraft version that predates that loader.
				if f.loader != "" && !slices.ContainsFunc(a.projectVersions, func(v launcher.ProjectVersion) bool {
					return (f.mc == "" || slices.Contains(v.GameVersions, f.mc)) && hasLoader(v, f.loader)
				}) {
					f.loader = ""
				}
			}
		})
		if ls := loadersOf(a.projectVersions); len(ls) > 0 || a.project.Type == "mod" {
			if f.loader != "" && !slices.Contains(ls, f.loader) {
				ls = append(ls, f.loader)
			}
			if selectBoxFull(c, "project-loader", a.L("版本加载器筛选", "Version loader filter"), &f.loader, append([]string{""}, ls...), nil, func(l string) string {
				if l == "" {
					return a.L("全部加载器", "All loaders")
				}
				return loaderLabel(l)
			}).Width(190 * s).Shrink(0).Changed() {
				a.projShowAll = true
			}
		}
	})
	ui.Row(c.Key("ver-chips")).FillWidth().Height(26 * s).Gap(8 * s).AlignItems(ui.Center).Children(func() {
		if a.pickerChip(c, "release", a.L("只看正式版", "Releases only"), !f.allTypes) {
			f.allTypes = !f.allTypes
		}
		if a.gameVersion() != nil && a.project.Type != "modpack" {
			if a.pickerChip(c, "compat", a.L("只看兼容", "Compatible only"), !a.projShowAll) {
				a.projShowAll = !a.projShowAll
				if !a.projShowAll {
					a.filterForInstance()
				} else {
					f.mc, f.loader = "", ""
				}
			}
		}
		ui.Spacer(c).Grow(1)
		if a.pickerChip(c, "reset", a.L("重置筛选", "Reset filters"), false) {
			f.mc, f.loader, f.allTypes = "", "", true
			a.projShowAll, a.projectVersionFilter = true, ""
		}
	})
}

func (a *App) versionGameBadge(c *ui.Context, v launcher.ProjectVersion) {
	mcs := mcVersionsOf([]launcher.ProjectVersion{v})
	if len(mcs) == 0 {
		return
	}
	primary := a.verFilter.mc
	if primary == "" && !a.projShowAll && a.project.Type != "modpack" {
		if g := a.gameVersion(); g != nil {
			primary = g.Minecraft
		}
	}
	if !slices.Contains(mcs, primary) {
		primary = mcs[0]
	}
	text := primary
	if len(mcs) > 1 {
		text += fmt.Sprintf(" +%d", len(mcs)-1)
	}
	tooltip(c, ui.Box(c.Key("version-game-"+v.ID)).Label("版本适配 "+v.ID).Shrink(1).MinWidth(0), strings.Join(mcs, ", ")).Children(func() {
		quietTag(c, text, inkSecondary)
	})
}
