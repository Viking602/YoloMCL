package nativeui

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type projectVersionListState struct {
	track  scrollTrack
	filter verFilter
	query  string
	count  int
	all    bool
	serial uint64
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
	a.initProjectFilters()
	f := &a.verFilter
	vis := a.visibleVersions()
	state := &a.versionList
	ui.Column(c.Key("ver-picker")).FillWidth().Gap(10 * s).Children(func() {
		ui.Row(c).FillWidth().Height(30 * s).Gap(10 * s).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.L("javaget_ui.version")).FontSize(typeCallout * s).FontWeight(650).TextColor(ink).Shrink(0)
			if n := len(a.projectVersions); n > 0 {
				cnt := fmt.Sprintf(a.L("project_version_picker.versions"), n)
				if len(vis) != n {
					cnt = fmt.Sprintf(a.L("project_version_picker.of_versions"), len(vis), n)
				}
				numText(c, cnt, typeFootnote*s, muted)
			}
			ui.Spacer(c).Grow(1)
			textField(c.Key("project-ver-filter"), &a.projectVersionFilter).Label(a.tr().ProjectFilterAria).Placeholder(a.L("project_version_picker.search_versions")).Width(200 * s).Shrink(1)
		})
		a.projectFilterControls(c)
		// Controls commit before selecting a file or resolving its dependencies.
		vis = a.visibleVersions()
		if state.filter != *f || state.query != a.projectVersionFilter || state.count != len(vis) || state.all != a.projShowAll {
			*state = projectVersionListState{filter: *f, query: a.projectVersionFilter, count: len(vis), all: a.projShowAll, serial: state.serial + 1}
		}
		if a.projLoading || a.projectFetch.err != "" || len(vis) == 0 {
			a.form.VersionID = ""
		} else if !slices.ContainsFunc(vis, func(v launcher.ProjectVersion) bool { return v.ID == a.form.VersionID }) {
			a.form.VersionID = vis[0].ID
			if rec := a.recommendedVersion(); slices.ContainsFunc(vis, func(v launcher.ProjectVersion) bool { return v.ID == rec }) {
				a.form.VersionID = rec
			}
		}
		// fixed-height list so filtering never moves the dialog
		listH := float32(verRowH*verRows+4*(verRows-1)) * s
		ui.Box(c.Key("ver-list")).FillWidth().Height(listH).Radius(12*s).Border(1, line).Background(ui.RGBA(255, 255, 255, .55)).Clip().Children(func() {
			switch {
			case a.projectFetch.err != "":
				if a.loadFailed(c, "project-versions-failed", failPanel, false) {
					a.reloadProjectVersions()
				}
			case a.projLoading:
				ui.Row(c).FillWidth().FillHeight().Justify(ui.Center).AlignItems(ui.Center).Gap(10 * s).Children(func() {
					statusSpinner(c, 16*s)
					label(c, a.L("project_version_picker.loading_versions"))
				})
			case len(vis) == 0:
				ui.Column(c).FillWidth().FillHeight().Justify(ui.Center).AlignItems(ui.Center).Gap(10 * s).Children(func() {
					msg := a.L("project_version_picker.no_version_matches")
					if len(a.projectVersions) == 0 {
						msg = a.L("project_version_picker.no_version_fits_this_instance")
					}
					ui.Text(c, msg).FontSize(typeCallout * s).FontWeight(600).TextColor(inkSecondary)
					if fixedButton(c, "ver-reset", a.L("project_version_picker.show_all_versions"), "", 0, btnSm*s, false, false) {
						f.mc, f.loader, f.allTypes, a.projectVersionFilter = "", "", true, ""
						a.projShowAll = true
					}
				})
			default:
				rec := a.recommendedVersion()
				stride := (verRowH + 4) * s
				// Bound the cold frame too. One spare row above and two below
				// cover fractional scrolling without building hundreds of rows.
				first := min(max(0, int(state.track.st.Y/stride)-1), max(0, len(vis)-verRows))
				last := min(len(vis), first+verRows+3)
				scroll := ui.Scroll(c.Key("project-versions-scroll")).Label("整合包版本列表").TrackScroll(&state.track.st)
				overlayScrollbar(scroll, &state.track).FillWidth().FillHeight().Children(func() {
					ui.Column(c.Key(fmt.Sprintf("version-results-%d", state.serial))).FillWidth().Gap(4*s).Padding(4*s, scrollGutter*s+4*s, 4*s, 4*s).Transition(heroSwap(c)).Children(func() {
						if first > 0 {
							ui.Box(c.Key("versions-before")).FillWidth().Height(float32(first)*stride - 4*s).Shrink(0)
						}
						for _, v := range vis[first:last] {
							a.versionRow(c, v, v.ID == rec)
						}
						if last < len(vis) {
							ui.Box(c.Key("versions-after")).FillWidth().Height(float32(len(vis)-last)*stride - 4*s).Shrink(0)
						}
					})
				})
			}
		})
	})
}

// Keep the version name above its badges so long pack titles leave their
// distinguishing version number visible. Downloads, date and size align right.
func (a *App) versionRow(c *ui.Context, v launcher.ProjectVersion, recommended bool) {
	s := uiScale(c)
	sel := v.ID == a.form.VersionID
	b := ui.ButtonBase(c.Key("proj-ver-" + v.ID)).Label(v.Number).FillWidth().Height(verRowH * s).Shrink(0).PaddingX(12 * s).Radius(9 * s).Gap(10 * s).AlignItems(ui.Center)
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
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4 * s).Children(func() {
			tooltip(c, ui.Text(c, num).FontFeatures("tnum").FontSize(typeCallout*s).FontWeight(650).TextColor(fg).SingleLine().FillWidth(), num+"\n"+v.Name)
			ui.Row(c).FillWidth().MinWidth(0).Gap(6 * s).AlignItems(ui.Center).Clip().Children(func() {
				a.versionGameBadge(c, v)
				for _, l := range v.Loaders {
					if lbl := loaderLabel(l); lbl != "" {
						quietTag(c, lbl, inkSecondary)
					}
				}
				rl, tone := a.releaseLabel(v.Type)
				quietTag(c, rl, tone)
				if recommended {
					quietTag(c, a.L("overview_runtime.recommended"), successInk)
				}
			})
		})
		a.versionMetrics(c, v, sel)
	}).Clicked() {
		a.form.VersionID = v.ID
	}
}
