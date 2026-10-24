package nativeui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

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
				ui.Text(c, a.L("Java 运行环境", "Java runtimes")).FontSize(typeHeadline * s).FontWeight(700).TextColor(ink)
				ui.Row(c).Height(18 * s).Gap(10 * s).AlignItems(ui.Center).Children(func() {
					if len(g.list) == 0 && (busy || !a.javaScannedOnce) {
						ui.Text(c, a.L("正在查找可用环境", "Finding available runtimes")).FontSize(typeFootnote * s).TextColor(muted)
						return
					}
					a.rollNumber(c, "java-installed-count", fmt.Sprintf(a.L("%d 个环境", "%d installed"), len(g.list)), typeFootnote*s, 500, muted)
					var total int64
					for _, j := range g.list {
						total += max(0, j.Size)
					}
					if total > 0 {
						ui.Box(c).Width(1).Height(10 * s).Background(line)
						a.rollNumber(c, "java-installed-size", fmtSize(total), typeFootnote*s, 500, muted).Tooltip(a.L("已检测到的安装目录占用空间", "Disk space of the detected installations"))
					}
					if busy {
						statusSpinner(c, 12*s)
					}
				})
			})
			ui.Row(c).Shrink(0).Gap(8 * s).AlignItems(ui.Center).Children(func() {
				if fixedButton(c, "java-rescan", a.L("重新扫描", "Rescan"), "refresh", 0, btnSm*s, false, busy) {
					a.refreshJavaList(true)
				}
				if fixedButton(c, "java-get-open", a.L("下载 Java", "Download Java"), "download", 0, btnSm*s, true, false) {
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
				loadingState(c, "java-none", a.L("正在检测已安装的 Java", "Looking for installed Java"))
			} else {
				emptyState(c, "java-none", "terminal", a.L("还没有 Java 运行环境", "No Java runtimes yet"), a.L("下载所需版本后，就可以启动对应的游戏。", "Download a compatible Java version to launch your game."), nil)
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
							ui.Text(c, javaSuits(a, major)).FontSize(typeFootnote * s).TextColor(muted).SingleLine().Tooltip(javaSuits(a, major))
						})
						ui.Row(c).Shrink(0).Padding(4*s, 9*s).Radius(999).Background(panel).Border(1, line.Alpha(.65)).Children(func() {
							a.rollNumber(c, fmt.Sprintf("java-group-count-%d", major), fmt.Sprintf(a.L("%d 个环境", "%d installed"), len(by[major])), typeCaption*s, 500, inkSecondary)
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
	ui.Row(c.Key("java-card-"+j.Path)).Label("Java 安装项 "+j.Path).FillWidth().MinHeight(76*s).Padding(14*s, 16*s).Gap(16 * s).AlignItems(ui.Center).Children(func() {
		info := ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(7 * s)
		info.Children(func() {
			ui.Row(c).FillWidth().Wrap().Gap(7 * s).AlignItems(ui.Center).Children(func() {
				vendor := javaVendor(a, j)
				ui.Text(c, vendor).Label("Java 发行方 " + j.Path).MaxWidth(190 * s).MinWidth(0).Shrink(1).FontSize(typeCallout * s).FontWeight(650).TextColor(ink).SingleLine().Tooltip(vendor)
				numText(c, j.Version, typeFootnote*s, inkSecondary).MaxWidth(160 * s).MinWidth(0).Shrink(1).SingleLine().Tooltip(j.Version)
				if isAuto {
					ui.Row(c).Shrink(0).Padding(3*s, 7*s).Gap(4 * s).Radius(999).Background(successInk.Alpha(.08)).AlignItems(ui.Center).Tooltip(a.L("自动选择此 Java 主版本时，优先使用这个环境", "Preferred when automatically choosing this Java major version")).Children(func() {
						ui.Icon(c, icons["check"]).Size(10*s, 10*s).TextColor(successInk)
						ui.Text(c, a.L("自动选用", "Auto pick")).FontSize(typeCaption * s).FontWeight(550).TextColor(successInk).NoWrap()
					})
				}
				if j.Managed {
					quietTag(c, a.L("YoloMCL 安装", "By YoloMCL"), accent)
				}
			})
			// Measure the actual text column; system installations no longer
			// reserve an empty Uninstall slot at the expense of their paths.
			pathW := info.LayoutBounds().W
			if pathW <= 0 {
				w, _ := c.Size()
				pathW = min(settingsColumnWidth*s, w-2*pageGutter*s) - (32+32+100+66)*s
				if j.Managed {
					pathW -= 82 * s
				}
				c.After(0)
			}
			path := midEllipsis(c, j.Home, pathW, typeCaption*s, 400)
			ui.Text(c, path).Label("Java 安装路径 " + j.Path).Font(uiFontFamily(titlebarOS())).FontSize(typeCaption * s).TextColor(muted).FillWidth().MinWidth(0).SingleLine().Tooltip(j.Home)
		})
		ui.Column(c).Label("Java 使用情况 " + j.Path).Width(100 * s).Shrink(0).Gap(6 * s).AlignItems(ui.End).Children(func() {
			if used > 0 {
				text := fmt.Sprintf(a.L("%d 个实例在用", "%d instances"), used)
				if a.isEN() && used == 1 {
					text = "1 instance"
				}
				a.rollNumber(c, "java-use-"+j.Path, text, typeFootnote*s, 550, accent, true)
			} else {
				ui.Text(c, a.L("暂未使用", "Not in use")).FontSize(typeFootnote * s).TextColor(muted).NoWrap()
			}
			if j.Size > 0 {
				a.rollNumber(c, "java-size-"+j.Path, fmtSize(j.Size), typeCaption*s, 400, muted, true)
			}
		})
		ui.Row(c).Label("Java 操作 " + j.Path).Shrink(0).Gap(6 * s).AlignItems(ui.Center).Children(func() { a.javaListActions(c, j) })
	})
}

func (a *App) javaListActions(c *ui.Context, j launcher.InstalledJava) {
	s := uiScale(c)
	g := &a.javaGet
	if iconOnlyButton(c, "java-copy-"+j.Path, "doc", a.L("复制路径", "Copy path"), btnSm*s, false) {
		c.WriteClipboard(j.Path)
		a.notice(a.L("已复制路径", "Path copied"))
	}
	if iconOnlyButton(c, "java-open-"+j.Path, "folder", a.L("打开文件夹", "Open folder"), btnSm*s, false) {
		if e := openPath(j.Home); e != nil {
			a.message, a.errorMessage = a.friendlyErr(e), true
		}
	}
	if !j.Managed {
		return
	}
	if g.confirm != j.Home {
		if fixedButton(c, "java-uninstall-"+j.Path, a.L("卸载", "Uninstall"), "trash", 76*s, btnSm*s, false, false) {
			g.confirm = j.Home
		}
	} else if dangerButton(c, "java-uninstall-ok-"+j.Path, a.L("确认卸载", "Confirm"), btnSm*s, false) {
		g.confirm = ""
		if e := a.service.UninstallJava(j.Home); e != nil {
			a.message, a.errorMessage = a.friendlyErr(e), true
		} else {
			a.notice(a.L("已卸载 ", "Uninstalled ") + "Java " + j.Version)
		}
		a.refreshJavaList(true)
	}
}
