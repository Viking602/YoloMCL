package nativeui

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// javaGetState backs the Java section and the 下载 Java window.
type javaGetState struct {
	list         []launcher.InstalledJava
	listRevision int
	listBusy     bool
	offers       []launcher.JavaOffer
	offersErr    error
	offersBusy   bool
	major        int
	source       string
	job          string
	jobMajor     int
	thenLaunch   bool
	needFor      string // the pack that asked for it
	lastDone     int
	lastAt       time.Time
	speed        float64 // KB/s, smoothed
	confirm      string  // home awaiting a second click to uninstall
}

var needJavaRE = regexp.MustCompile(`需要 Java (\d+)，还没有安装`)

func javaSourceName(a *App, src string) string {
	switch src {
	case "mojang":
		return a.L("Mojang 官方", "Mojang")
	case "temurin":
		return "Eclipse Temurin"
	case "zulu":
		return "Azul Zulu"
	}
	return a.L("本机", "This PC")
}

func (a *App) refreshJavaList(force bool) {
	g := &a.javaGet
	if a.service == nil {
		return
	}
	if force {
		a.startJavaScan(false)
		return
	}
	a.ensureJavaScan()
	if !a.javaScannedOnce || a.javaScanning || g.listBusy || (g.list != nil && g.listRevision == a.javaScanRevision) {
		return
	}
	// Changing instances only changes which Java is recommended. The installed
	// list and its directory sizes reuse the session's completed discovery.
	revision := a.javaScanRevision
	known := append([]launcher.JavaInfo(nil), a.java...)
	g.listBusy = true
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		l := a.service.InstalledJavaDetails(a.ctx, known)
		a.post(func() {
			g.listBusy = false
			if revision == a.javaScanRevision {
				g.list, g.listRevision = l, revision
			}
		})
	}()
}

func (a *App) loadJavaOffers() {
	g := &a.javaGet
	if a.service == nil || g.offersBusy {
		return
	}
	g.offersBusy, g.offersErr = true, nil
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		o, e := a.service.JavaOffers(ctx)
		a.post(func() {
			g.offersBusy = false
			g.offers, g.offersErr = o, e
		})
	}()
}

// openJavaGet opens the download window, preset to major (0 = 21).
func (a *App) openJavaGet(major int, needFor string, thenLaunch bool) {
	g := &a.javaGet
	if major <= 0 {
		major = 21
	}
	if g.job == "" {
		g.major, g.needFor, g.thenLaunch = major, needFor, thenLaunch
	}
	if g.source == "" {
		g.source = "mojang"
	}
	if g.offers == nil {
		a.loadJavaOffers()
	}
	a.openDialog("javaGet")
}

func (a *App) javaJob() *launcher.Job {
	for i := range a.snapshot.Jobs {
		if a.snapshot.Jobs[i].ID == a.javaGet.job {
			return &a.snapshot.Jobs[i]
		}
	}
	return nil
}

// javaJobEnded runs from applyJobNotice when a job leaves running.
func (a *App) javaJobEnded(j launcher.Job) bool {
	g := &a.javaGet
	if m := needJavaRE.FindStringSubmatch(j.Message); j.State == "failed" && m != nil && g.job != j.ID {
		n, _ := strconv.Atoi(m[1])
		name := ""
		if r := a.root(); r != nil {
			name = dirLabel(*r)
			if v := a.versionInfo(r.ID, r.SelectedVersion); v != nil {
				if t := a.nameOf(*r, *v).Title; strings.TrimSpace(t) != "" {
					name = t
				}
			}
		}
		a.message = ""
		a.openJavaGet(n, name, true)
		return true
	}
	if j.ID != g.job {
		return false
	}
	switch j.State {
	case "completed":
		a.notice(fmt.Sprintf(a.L("已安装 Java %d", "Java %d installed"), g.jobMajor))
		a.refreshJavaList(true)
		if g.thenLaunch {
			g.thenLaunch = false
			a.closeDialog()
			a.launch()
		}
		g.job, g.needFor = "", ""
	case "cancelled":
		g.job = ""
	case "failed":
		a.message, a.errorMessage = a.friendlyErr(errors.New(j.Message)), true
		g.job = ""
	}
	return true
}

func javaHint(a *App, major int) string {
	switch major {
	case 8:
		return a.L("1.16.5 及更早", "1.16.5 and older")
	case 17:
		return "1.17–1.20.4"
	case 21:
		return "1.20.5–1.21.x"
	}
	return a.L("26.1 及更新", "26.1 and newer")
}

// javaSection lists runtimes as cards in 全局设置.
func (a *App) javaSection(c *ui.Context) {
	s := uiScale(c)
	g := &a.javaGet
	a.refreshJavaList(false)
	need := 0
	if r := a.root(); r != nil {
		if v := a.versionInfo(r.ID, r.SelectedVersion); v != nil {
			need = javaFor(*r, *v)
		}
	}
	ui.Column(c.Key("java-section")).FillWidth().Gap(space2 * s).Children(func() {
		ui.Row(c).FillWidth().Height(btnMd * s).Gap(8 * s).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.L("Java 运行环境", "Java runtimes")).FontSize(typeHeadline * s).FontWeight(700).TextColor(ink)
			ui.Text(c, fmt.Sprintf(a.L("%d 个", "%d found"), len(g.list))).FontSize(typeFootnote * s).TextColor(muted).NoWrap()
			var managed int64
			for _, j := range g.list {
				if j.Managed {
					managed += j.Size
				}
			}
			if managed > 0 {
				numText(c, a.L("YoloMCL 安装共 ", "YoloMCL installs ")+fmtSize(managed), typeFootnote*s, muted)
			}
			ui.Spacer(c).Grow(1)
			if fixedButton(c, "java-rescan", a.L("重新扫描", "Rescan"), "refresh", 0, btnSm*s, false, g.listBusy || a.javaScanning) {
				a.refreshJavaList(true)
			}
			if fixedButton(c, "java-get-open", a.L("下载 Java", "Download Java"), "download", 0, btnSm*s, true, false) {
				a.openJavaGet(max(need, 0), "", false)
			}
		})
		if len(g.list) == 0 {
			ui.Row(c.Key("java-none")).FillWidth().Height(56*s).Padding(0, 16*s).Radius(radiusCard*s).Background(raised).Border(1, glassEdge).AlignItems(ui.Center).Children(func() {
				msg := a.L("还没有找到 Java，可以直接下载一个", "No Java found yet; download one")
				if g.listBusy || a.javaScanning || !a.javaScannedOnce {
					msg = a.L("正在检测已安装的 Java", "Looking for installed Java")
				}
				label(c, msg).FontSize(typeFootnote * s)
			})
			return
		}
		a.javaGroups(c)
	})
}

// javaGetDialog: pick a version and a source, then follow the download.
func (a *App) javaGetDialog(c *ui.Context) {
	s := uiScale(c)
	g := &a.javaGet
	if g.needFor != "" {
		ui.Row(c.Key("java-need")).FillWidth().Height(52*s).Padding(0, 14*s).Gap(10 * s).Radius(12 * s).Background(accentSoft).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons["info"]).Size(14*s, 14*s).TextColor(accent).Shrink(0)
			ui.Text(c, fmt.Sprintf(a.L("%s 需要 Java %d，这台电脑还没有。下载完成后会继续启动。", "%s needs Java %d, which is not installed. The launch continues after the download."), g.needFor, g.major)).FontSize(typeFootnote * s).TextColor(inkSecondary).SingleLine().Shrink(1).MinWidth(0)
		})
	}
	if j := a.javaJob(); j != nil && j.State == "running" {
		a.javaProgress(c, j)
		return
	}
	if g.offersBusy && g.offers == nil {
		ui.Row(c.Key("java-offers-wait")).FillWidth().Height(120 * s).Center().Children(func() {
			statusLine(c, a.L("正在读取可下载的版本", "Reading available versions"), muted, true)
		})
		return
	}
	if g.offersErr != nil && g.offers == nil {
		if a.loadFailed(c, "java-offers-failed", failPanel, g.offersBusy) {
			a.loadJavaOffers()
		}
		return
	}
	majors := []int{}
	seen := map[int]bool{}
	for _, o := range g.offers {
		if !seen[o.Major] {
			seen[o.Major] = true
			majors = append(majors, o.Major)
		}
	}
	installed := map[int]bool{}
	for _, j := range g.list {
		installed[j.Major] = true
	}
	label(c, a.L("版本", "Version")).FontSize(typeFootnote * s).FontWeight(600)
	// Two by two: every card the same size, nothing clipped or cut short.
	ui.Column(c.Key("java-majors")).FillWidth().Gap(10 * s).Padding(2 * s).Children(func() {
		for r := 0; r < len(majors); r += 2 {
			ui.Row(c).FillWidth().Gap(10 * s).Children(func() {
				for k := r; k < r+2; k++ {
					if k >= len(majors) {
						ui.Box(c).Grow(1).Basis(0)
						continue
					}
					m := majors[k]
					sel := g.major == m
					b := ui.ButtonBase(c.Key(fmt.Sprint("java-major-", m))).Label(fmt.Sprintf("Java %d", m)).Grow(1).Basis(0).MinWidth(0).Height(64*s).PaddingX(16*s).Gap(10*s).Radius(14*s).AlignItems(ui.Center).Border(1, line).Background(raised)
					if sel {
						b.Background(accentSoft).Border(2, accent)
					} else if b.Hovered() {
						b.Background(panel)
					}
					if b.Children(func() { a.javaMajorCard(c, m, installed[m]) }).Clicked() {
						g.major = m
					}
				}
			})
		}
	})
	label(c, a.L("来源", "Source")).FontSize(typeFootnote * s).FontWeight(600)
	var pick *launcher.JavaOffer
	ui.Column(c.Key("java-sources")).FillWidth().Gap(8 * s).Children(func() {
		for _, src := range []string{"mojang", "temurin", "zulu"} {
			src := src
			var off *launcher.JavaOffer
			for i := range g.offers {
				if g.offers[i].Major == g.major && g.offers[i].Source == src {
					off = &g.offers[i]
				}
			}
			sel := g.source == src && off != nil
			if sel {
				pick = off
			}
			b := ui.ButtonBase(c.Key("java-src-"+src)).Label(javaSourceName(a, src)).FillWidth().Height(52*s).PaddingX(14*s).Gap(12*s).Radius(12*s).AlignItems(ui.Center).Border(1, line).Background(raised)
			if sel {
				b.Background(accentSoft).Border(2, accent)
			} else if b.Hovered() && off != nil {
				b.Background(panel)
			}
			if b.Children(func() {
				ui.Box(c).Size(16*s, 16*s).Radius(999).Border(2, map[bool]ui.Color{true: accent, false: line}[sel]).Center().Shrink(0).Children(func() {
					if sel {
						ui.Box(c).Size(8*s, 8*s).Radius(999).Background(accent)
					}
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(2 * s).Children(func() {
					ui.Text(c, javaSourceName(a, src)).FontSize(typeCallout * s).FontWeight(650).TextColor(ink).NoWrap()
					desc := a.L("官方启动器同款运行环境，逐个文件校验", "What the official launcher uses, each file checked")
					switch src {
					case "temurin":
						desc = a.L("开源 JRE，连接慢时自动换用国内镜像", "Open-source JRE; switches to a China mirror when slow")
					case "zulu":
						desc = a.L("Azul 构建，支持 Java 8 到最新版", "Azul build, Java 8 to the newest")
					}
					if off == nil {
						desc = a.L("这个来源没有此版本", "Not offered for this version")
					}
					ui.Text(c, desc).FontSize(typeCaption * s).TextColor(muted).SingleLine()
				})
				if off != nil {
					v := off.Version
					if off.Size > 0 {
						v += "   " + fmtSize(off.Size)
					}
					ui.Text(c, v).FontSize(typeFootnote * s).FontFeatures("tnum").TextColor(inkSecondary).NoWrap().Shrink(0)
				}
			}).Clicked() && off != nil {
				g.source = src
			}
		}
	})
	if pick == nil {
		for i := range g.offers {
			if g.offers[i].Major == g.major {
				g.source = g.offers[i].Source
				break
			}
		}
	}
	ui.Row(c.Key("java-get-foot")).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
		label(c, a.L("安装到 YoloMCL 数据目录，装好后自动加入 Java 列表", "Installs into YoloMCL's data folder and joins the Java list")).FontSize(typeCaption * s).Grow(1).MinWidth(0).SingleLine()
		if button(c, "java-get-go", fmt.Sprintf(a.L("下载 Java %d", "Download Java %d"), g.major), "download", true, pick == nil) && pick != nil {
			id, e := a.service.InstallJava(g.major, g.source)
			if e != nil {
				a.message, a.errorMessage = a.friendlyErr(e), true
				return
			}
			g.job, g.jobMajor, g.lastDone, g.speed = id, g.major, 0, 0
		}
	})
}

// javaProgress: bar, size and speed, and 取消.
func (a *App) javaProgress(c *ui.Context, j *launcher.Job) {
	s := uiScale(c)
	g := &a.javaGet
	now := time.Now()
	if !g.lastAt.IsZero() && j.Done > g.lastDone {
		dt := now.Sub(g.lastAt).Seconds()
		if dt > .2 {
			inst := float64(j.Done-g.lastDone) / dt
			if g.speed == 0 {
				g.speed = inst
			} else {
				g.speed = g.speed*.7 + inst*.3
			}
			g.lastDone, g.lastAt = j.Done, now
		}
	} else if g.lastAt.IsZero() || j.Done < g.lastDone {
		g.lastDone, g.lastAt = j.Done, now
	}
	c.After(250 * time.Millisecond)
	frac := float32(0)
	if j.Total > 0 {
		frac = min(1, float32(j.Done)/float32(j.Total))
	}
	ui.Column(c.Key("java-progress")).FillWidth().Gap(10*s).Padding(16*s).Radius(14*s).Background(raised).Border(1, glassEdge).Children(func() {
		ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf(a.L("正在下载 Java %d", "Downloading Java %d"), g.jobMajor)).FontSize(typeCallout * s).FontWeight(700).TextColor(ink).Grow(1)
			ui.Text(c, javaSourceName(a, g.source)).FontSize(typeFootnote * s).TextColor(muted).NoWrap()
		})
		ui.Box(c).FillWidth().Height(8 * s).Radius(999).Background(ui.RGBA(120, 130, 160, .16)).Clip().Children(func() {
			ui.Box(c).WidthPercent(frac * 100).Height(8 * s).Radius(999).Background(accent)
		})
		ui.Row(c).FillWidth().Gap(16 * s).AlignItems(ui.Center).Children(func() {
			size := a.L("正在准备", "Preparing")
			if j.Total > 0 {
				size = fmt.Sprintf("%s / %s", fmtSize(int64(j.Done)<<10), fmtSize(int64(j.Total)<<10))
			} else if j.Message != "" {
				size = a.translatePhase(j.Message)
			}
			ui.Text(c, size).FontSize(typeFootnote * s).FontFeatures("tnum").TextColor(inkSecondary)
			if g.speed > 0 && j.Total > 0 && j.Done < j.Total {
				ui.Text(c, fmtSize(int64(g.speed*1024))+"/s").FontSize(typeFootnote * s).FontFeatures("tnum").TextColor(muted).NoWrap()
			}
			if j.Total > 0 && j.Done >= j.Total {
				ui.Text(c, a.L("正在校验与安装", "Verifying and installing")).FontSize(typeFootnote * s).TextColor(muted).NoWrap()
			}
			ui.Spacer(c).Grow(1)
			if fixedButton(c, "java-cancel", a.L("取消", "Cancel"), "cross", 0, btnSm*s, false, false) {
				a.execute("cancelJob", map[string]string{"id": j.ID}, nil)
				g.thenLaunch = false
			}
		})
	})
}

func (a *App) javaMajorCard(c *ui.Context, m int, installed bool) {
	s := uiScale(c)
	g := &a.javaGet
	ui.Column(c).Grow(1).MinWidth(0).Gap(4 * s).Children(func() {
		ui.Row(c).Gap(6 * s).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("Java %d", m)).FontSize(typeHeadline * s).FontWeight(700).TextColor(ink).NoWrap()
			for _, o := range g.offers {
				if o.Major == m && o.LTS {
					ui.Row(c).Padding(1*s, 6*s).Radius(999).Background(accent.Alpha(.12)).Children(func() {
						ui.Text(c, "LTS").FontSize(typeCaption2 * s).FontWeight(700).TextColor(accent).NoWrap()
					})
					break
				}
			}
		})
		ui.Text(c, javaHint(a, m)).FontSize(typeCaption * s).FontFeatures("tnum").TextColor(muted).NoWrap()
	})
	if installed {
		ui.Row(c).Shrink(0).Padding(2*s, 8*s).Radius(999).Background(successInk.Alpha(.12)).Children(func() {
			ui.Text(c, a.L("已安装", "Installed")).FontSize(typeCaption2 * s).FontWeight(700).TextColor(successInk).NoWrap()
		})
	}
}

// javaVendor normalizes vendor strings to one short name.
func javaVendor(a *App, j launcher.InstalledJava) string {
	if j.Managed {
		switch j.Source {
		case "mojang":
			return "Mojang"
		case "temurin":
			return "Temurin"
		case "zulu":
			return "Zulu"
		}
	}
	v := strings.ToLower(j.Vendor + " " + j.Home)
	for _, m := range []struct{ key, name string }{
		{"zulu", "Zulu"}, {"azul", "Zulu"}, {"adoptium", "Temurin"}, {"temurin", "Temurin"}, {"adoptopenjdk", "Temurin"},
		{"mojang", "Mojang"}, {"microsoft", "Microsoft"}, {"corretto", "Corretto"}, {"amazon", "Corretto"},
		{"liberica", "Liberica"}, {"bellsoft", "Liberica"}, {"graalvm", "GraalVM"}, {"jetbrains", "JBR"},
		{"oracle", "Oracle"}, {"jre-legacy", "Mojang"}, {"java-runtime", "Mojang"},
	} {
		if strings.Contains(v, m.key) {
			return m.name
		}
	}
	if j.Vendor != "" {
		return javaDisplay(j.Vendor)
	}
	return "OpenJDK"
}

func javaSuits(a *App, major int) string {
	switch {
	case major >= 25:
		return a.L("适合 26.1 及以上", "For 26.1 and newer")
	case major >= 21:
		return a.L("适合 1.20.5 到 1.21.x", "For 1.20.5 to 1.21.x")
	case major >= 17:
		return a.L("适合 1.17 到 1.20.4", "For 1.17 to 1.20.4")
	case major >= 8:
		return a.L("适合 1.16.5 及更早", "For 1.16.5 and older")
	}
	return ""
}

// midPath shortens a path in the middle so both the drive and the folder
// name stay readable.
func midPath(p string, n int) string {
	r := []rune(p)
	if len(r) <= n {
		return p
	}
	head := n * 2 / 5
	return string(r[:head]) + "…" + string(r[len(r)-(n-head-1):])
}

// javaUse counts instances that launch with each Java (by path): a
// version's own Java, else the first detected Java of its major, which is
// what ensureJava auto-picks.
func (a *App) javaUse() (map[string]int, map[int]string) {
	auto := map[int]string{}
	for _, j := range a.java {
		if _, ok := auto[j.Major]; !ok {
			auto[j.Major] = strings.ToLower(j.Path)
		}
	}
	use := map[string]int{}
	for _, r := range a.snapshot.Instances {
		seen := map[string]bool{}
		for _, v := range r.Versions {
			p := strings.ToLower(v.JavaPath)
			if p == "" {
				p = auto[javaFor(r, v)]
			}
			if p != "" && !seen[p] {
				seen[p] = true
				use[p]++
			}
		}
	}
	return use, auto
}

// javaGroups lists installed Javas grouped by major (newest first), each
// group headed by what it suits; rows use fixed columns.
func (a *App) javaGroups(c *ui.Context) {
	s := uiScale(c)
	g := &a.javaGet
	use, auto := a.javaUse()
	majors := []int{}
	by := map[int][]launcher.InstalledJava{}
	for _, j := range g.list {
		if by[j.Major] == nil {
			majors = append(majors, j.Major)
		}
		by[j.Major] = append(by[j.Major], j)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(majors)))
	scrollArea(c, "java-list").FillWidth().MaxHeight(460*s).Radius(radiusCard*s).Background(raised).Border(1, glassEdge).Children(func() {
		ui.Column(c).FillWidth().Padding(6*s, 0).Gap(0).Children(func() {
			for gi, m := range majors {
				m := m
				if gi > 0 {
					ui.Box(c).FillWidth().Height(1).Background(line).Margin(4*s, 0)
				}
				ui.Row(c.Key(fmt.Sprintf("java-group-%d", m))).FillWidth().Height(30 * s).PaddingX(16 * s).Gap(10 * s).AlignItems(ui.Center).Children(func() {
					ui.Text(c, fmt.Sprintf("Java %d", m)).FontSize(typeCallout * s).FontWeight(750).TextColor(ink).NoWrap()
					label(c, javaSuits(a, m)).FontSize(typeFootnote * s).NoWrap()
				})
				for _, item := range by[m] {
					a.javaListRow(c, item, use[strings.ToLower(item.Path)], auto[m] == strings.ToLower(item.Path))
				}
			}
		})
	})
}

func (a *App) javaListRow(c *ui.Context, j launcher.InstalledJava, used int, isAuto bool) {
	s := uiScale(c)
	g := &a.javaGet
	// Path budget: the card is min(window, settings column); subtract the
	// fixed right columns (usage 104 + size 76 + actions 170), padding and gaps.
	ww, _ := c.Size()
	card := ww - 2*space5*s
	if mx := settingsColumnWidth * s; card > mx {
		card = mx
	}
	pathW := card - (32+104+76+170+36)*s
	chars := int(pathW / (typeCaption * s * 0.62))
	if chars < 16 {
		chars = 16
	}
	ui.Row(c.Key("java-card-" + j.Path)).FillWidth().Height(56 * s).PaddingX(16 * s).Gap(12 * s).AlignItems(ui.Center).Children(func() {
		// left: two lines, flexes and clips
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Clip().Gap(3 * s).Children(func() {
			ui.Row(c).Gap(6 * s).AlignItems(ui.Center).Children(func() {
				ui.Text(c, javaVendor(a, j)).FontSize(typeCallout * s).FontWeight(650).TextColor(ink).NoWrap()
				numText(c, j.Version, typeFootnote*s, inkSecondary)
				if isAuto {
					quietTag(c, a.L("自动选用", "Auto pick"), successInk)
				}
				if j.Managed {
					quietTag(c, a.L("YoloMCL 安装", "By YoloMCL"), accent)
				}
			})
			ui.Box(c).FillWidth().Clip().Tooltip(j.Path).Children(func() {
				ui.Text(c, midPath(j.Home, chars)).FontSize(typeCaption * s).TextColor(muted).SingleLine()
			})
		})
		ui.Box(c).Width(104 * s).Shrink(0).Clip().AlignItems(ui.End).Children(func() {
			if used > 0 {
				label(c, fmt.Sprintf(a.L("%d 个实例在用", "used by %d"), used)).FontSize(typeFootnote * s).NoWrap()
			}
		})
		ui.Box(c).Width(76 * s).Shrink(0).Clip().AlignItems(ui.End).Children(func() {
			if j.Size > 0 {
				numText(c, fmtSize(j.Size), typeFootnote*s, muted)
			}
		})
		// actions: fixed slot
		ui.Row(c).Width(170 * s).Shrink(0).Gap(4 * s).Justify(ui.End).AlignItems(ui.Center).Children(func() {
			if iconOnlyButton(c, "java-copy-"+j.Path, "doc", a.L("复制路径", "Copy path"), 26*s, false) {
				c.WriteClipboard(j.Path)
				a.notice(a.L("已复制路径", "Path copied"))
			}
			if iconOnlyButton(c, "java-open-"+j.Path, "folder", a.L("打开文件夹", "Open folder"), 26*s, false) {
				if e := openPath(j.Home); e != nil {
					a.message, a.errorMessage = a.friendlyErr(e), true
				}
			}
			if !j.Managed {
				ui.Box(c).Width(76 * s).Shrink(0)
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
		})
	})
}
