package nativeui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/i18n"
	"github.com/Viking602/YoloMCL/internal/launcher"
)

func (a *App) javaSection(c *ui.Context) {
	s := uiScale(c)
	g := &a.javaGet
	a.refreshJavaList(false)
	busy := g.listBusy || a.javaScanning
	ui.Column(c.Key("java-section")).Label("Java 运行环境面板").Font(uiSansFamily()).FillWidth().Gap(14 * s).Children(func() {
		ui.Row(c).FillWidth().Wrap().Gap(12 * s).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(180 * s).Gap(5 * s).Children(func() {
				ui.Text(c, a.L("java_installed.java_runtimes")).FontSize(typeHeadline * s).FontWeight(700).TextColor(ink)
				ui.Row(c).Height(18 * s).Gap(10 * s).AlignItems(ui.Center).Children(func() {
					if len(g.list) == 0 && (busy || !a.javaScannedOnce) {
						ui.Text(c, a.L("java_installed.finding_available_runtimes")).FontSize(typeFootnote * s).TextColor(muted)
						return
					}
					a.rollNumber(c, "java-installed-count", fmt.Sprintf(a.L("java_installed.installed"), len(g.list)), typeFootnote*s, 500, muted)
					var total int64
					for _, j := range g.list {
						total += max(0, j.Size)
					}
					if total > 0 {
						ui.Box(c).Width(1).Height(10 * s).Background(line)
						tooltip(c, a.rollNumber(c, "java-installed-size", fmtSize(total), typeFootnote*s, 500, muted), a.L("java_installed.disk_space_of_the_detected_installations"))
					}
					if busy {
						statusSpinner(c, 12*s)
					}
				})
			})
			ui.Row(c).Shrink(0).Gap(8 * s).AlignItems(ui.Center).Children(func() {
				if fixedButton(c, "java-rescan", a.L("java_installed.rescan"), "refresh", 0, btnSm*s, false, busy) {
					a.refreshJavaList(true)
				}
				if fixedButton(c, "java-get-open", a.L("i18n.download_java"), "download", 0, btnSm*s, true, false) {
					need := 0
					if r := a.root(); r != nil {
						if v := a.versionInfo(r.ID, r.SelectedVersion); v != nil {
							need = javaFor(*r, *v)
						}
					}
					a.openJavaGet(max(need, 0), "", false)
				}
			})
		})
		if len(g.list) == 0 {
			if busy || !a.javaScannedOnce {
				loadingState(c, "java-none", a.L("java_installed.looking_for_installed_java"))
			} else {
				emptyState(c, "java-none", "terminal", a.L("java_installed.no_java_runtimes_yet"), a.L("java_installed.download_a_compatible_java_version_to_launch_your"), nil)
			}
			return
		}
		a.javaGroups(c)
	})
}

// Each major gets a distinct card; the list keeps its own bounded scroll area
// so machines with many JDKs do not push the rest of Settings out of reach.
func (a *App) javaGroups(c *ui.Context) {
	s := uiScale(c)
	use, auto := a.javaUse()
	var majors []int
	by := map[int][]launcher.InstalledJava{}
	for _, j := range a.javaGet.list {
		if by[j.Major] == nil {
			majors = append(majors, j.Major)
		}
		by[j.Major] = append(by[j.Major], j)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(majors)))
	scrollArea(c, "java-list").Label("Java 环境列表").FillWidth().MaxHeight(600 * s).Padding(1).Children(func() {
		ui.Column(c).FillWidth().Gap(12 * s).Children(func() {
			for _, major := range majors {
				ui.Column(c.Key(fmt.Sprintf("java-group-%d", major))).Label("Java 环境组 "+fmt.Sprint(major)).FillWidth().Radius(16*s).Background(panel).Border(1, line.Alpha(.65)).Clip().Children(func() {
					ui.Row(c).FillWidth().Padding(13*s, 16*s).Gap(12 * s).AlignItems(ui.Center).Background(raised.Alpha(.65)).Children(func() {
						ui.Box(c).Size(40*s, 40*s).Shrink(0).Radius(12 * s).Background(accent.Alpha(.08)).Center().Children(func() {
							numText(c, fmt.Sprint(major), 21*s, accent).FontWeight(700)
						})
						ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(3 * s).Children(func() {
							ui.Text(c, fmt.Sprintf("Java %d", major)).FontSize(typeHeadline * s).FontWeight(650).TextColor(ink).SingleLine()
							tooltip(c, ui.Text(c, javaSuits(a, major)).FontSize(typeFootnote*s).TextColor(muted).SingleLine(), javaSuits(a, major))
						})
						ui.Row(c).Shrink(0).Padding(4*s, 9*s).Radius(999).Background(panel).Border(1, line.Alpha(.65)).Children(func() {
							a.rollNumber(c, fmt.Sprintf("java-group-count-%d", major), fmt.Sprintf(a.L("java_installed.installed"), len(by[major])), typeCaption*s, 500, inkSecondary)
						})
					})
					for i, j := range by[major] {
						if i > 0 {
							ui.Box(c).FillWidth().Height(1).Margin(0, 16*s).Background(line.Alpha(.6))
						}
						a.javaListRow(c, j, use[strings.ToLower(j.Path)], auto[major] == strings.ToLower(j.Path))
					}
				})
			}
		})
	})
}

func (a *App) javaListRow(c *ui.Context, j launcher.InstalledJava, used int, isAuto bool) {
	s := uiScale(c)
	vendor := javaVendor(a, j)
	ui.Row(c.Key("java-card-"+j.Path)).Label("Java 安装项 "+j.Path).FillWidth().MinHeight(76*s).Padding(14*s, 16*s).Gap(16 * s).AlignItems(ui.Center).Children(func() {
		javaBrandIcon(c, vendor, 36*s)
		info := ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(7 * s)
		info.Children(func() {
			ui.Row(c).FillWidth().Wrap().Gap(7 * s).AlignItems(ui.Center).Children(func() {
				tooltip(c, ui.Text(c, vendor).Label("Java 发行方 "+j.Path).MaxWidth(190*s).MinWidth(0).Shrink(1).FontSize(typeCallout*s).FontWeight(650).TextColor(ink).SingleLine(), vendor)
				tooltip(c, numText(c, j.Version, typeFootnote*s, inkSecondary).MaxWidth(160*s).MinWidth(0).Shrink(1).SingleLine(), j.Version)
				if isAuto {
					tooltip(c, ui.Row(c).Shrink(0).Padding(3*s, 7*s).Gap(4*s).Radius(999).Background(successInk.Alpha(.08)).AlignItems(ui.Center), a.L("java_installed.preferred_when_automatically_choosing_this_java_major_version")).Children(func() {
						ui.Icon(c, icons["check"]).Size(10*s, 10*s).TextColor(successInk)
						ui.Text(c, a.L("java_installed.auto_pick")).FontSize(typeCaption * s).FontWeight(550).TextColor(successInk).NoWrap()
					})
				}
				if j.Managed {
					quietTag(c, a.L("java_installed.by_yolomcl"), accent)
				}
			})
			// Measure the actual text column; system installations no longer
			// reserve an empty Uninstall slot at the expense of their paths.
			pathW := info.LayoutBounds().W
			if pathW <= 0 {
				w, _ := c.Size()
				pathW = min(settingsColumnWidth*s, w-2*pageGutter*s) - (32+48+36+100+66)*s
				if j.Managed {
					pathW -= 82 * s
				}
				c.After(0)
			}
			path := midEllipsis(c, j.Home, pathW, typeCaption*s, 400)
			tooltip(c, ui.Text(c, path).Label("Java 安装路径 "+j.Path).Font(uiFontFamily(titlebarOS())).FontSize(typeCaption*s).TextColor(muted).FillWidth().MinWidth(0).SingleLine(), j.Home)
		})
		ui.Column(c).Label("Java 使用情况 " + j.Path).Width(100 * s).Shrink(0).Gap(6 * s).AlignItems(ui.End).Children(func() {
			if used > 0 {
				text := fmt.Sprintf(i18n.Plural(a.isEN(), "java.used", used), used)
				a.rollNumber(c, "java-use-"+j.Path, text, typeFootnote*s, 550, accent, true)
			} else {
				ui.Text(c, a.L("java_installed.not_in_use")).FontSize(typeFootnote * s).TextColor(muted).NoWrap()
			}
			if j.Size > 0 {
				a.rollNumber(c, "java-size-"+j.Path, fmtSize(j.Size), typeCaption*s, 400, muted, true)
			}
		})
		ui.Row(c).Label("Java 操作 " + j.Path).Shrink(0).Gap(6 * s).AlignItems(ui.Center).Children(func() { a.javaListActions(c, j) })
	})
}

func javaBrandIcon(c *ui.Context, vendor string, size float32) {
	background := panel
	if vendor == "GraalVM" {
		background = ui.Hex("#163047")
	}
	ui.Box(c).Label("Java 品牌图标 "+vendor).Size(size, size).Shrink(0).Radius(size*.27).
		Background(background).Border(1, line.Alpha(.65)).Center().Children(func() {
		iconSize := size * .72
		switch vendor {
		case "Microsoft":
			ui.Image(c, microsoftLogo).Size(iconSize, iconSize).Fit(ui.Contain)
		case "Mojang":
			ui.Image(c, loaderLogos["vanilla"]).Size(iconSize, iconSize).Fit(ui.Contain)
		default:
			if logo := javaLogos[strings.ToLower(vendor)]; logo != nil {
				ui.Image(c, logo).Size(iconSize, iconSize).Fit(ui.Contain)
			} else {
				ui.Text(c, "JDK").Font(uiSansFamily()).FontSize(size * .30).FontWeight(650).TextColor(muted).NoWrap()
			}
		}
	})
}

func (a *App) javaListActions(c *ui.Context, j launcher.InstalledJava) {
	s := uiScale(c)
	g := &a.javaGet
	if iconOnlyButton(c, "java-copy-"+j.Path, "doc", a.L("java_installed.copy_path"), btnSm*s, false) {
		c.WriteClipboard(j.Path)
		a.notice(a.L("java_installed.path_copied"))
	}
	if iconOnlyButton(c, "java-open-"+j.Path, "folder", a.L("addons_tab.open_folder"), btnSm*s, false) {
		if e := openPath(j.Home); e != nil {
			a.message, a.errorMessage = a.friendlyErr(e), true
		}
	}
	if !j.Managed {
		return
	}
	if g.confirm != j.Home {
		if fixedButton(c, "java-uninstall-"+j.Path, a.L("java_installed.uninstall"), "trash", 76*s, btnSm*s, false, false) {
			g.confirm = j.Home
		}
	} else if dangerButton(c, "java-uninstall-ok-"+j.Path, a.L("java_installed.confirm"), btnSm*s, false) {
		g.confirm = ""
		if e := a.service.UninstallJava(j.Home); e != nil {
			a.message, a.errorMessage = a.friendlyErr(e), true
		} else {
			a.notice(a.L("java_installed.uninstalled") + "Java " + j.Version)
		}
		a.refreshJavaList(true)
	}
}
