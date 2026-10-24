package nativeui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func (a *App) fmtDuration(d time.Duration) string {
	m := int(d.Minutes())
	switch {
	case m < 1:
		return a.L("不到 1 分钟", "Under 1 min")
	case m < 60:
		return fmt.Sprintf(a.L("%d 分钟", "%d min"), m)
	}
	return fmt.Sprintf(a.L("%d 小时 %d 分钟", "%d h %d min"), m/60, m%60)
}

func (a *App) fmtWhen(t time.Time) string {
	now := time.Now()
	if t.YearDay() == now.YearDay() && t.Year() == now.Year() {
		return a.L("今天 ", "Today ") + t.Format("15:04")
	}
	return t.Format(a.L("01月02日 15:04", "Jan 02 15:04"))
}

func fmtSize(n int64) string {
	switch {
	case n >= 1<<40:
		return fmt.Sprintf("%.1f TB", float64(n)/(1<<40))
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%d KB", n>>10)
	}
	return fmt.Sprintf("%d B", n)
}

// versionInfo finds the version a session ran.
func (a *App) versionInfo(rootID, ver string) *launcher.VersionSummary {
	for i := range a.snapshot.Instances {
		in := &a.snapshot.Instances[i]
		if in.ID != rootID {
			continue
		}
		for j := range in.Versions {
			if in.Versions[j].ID == ver {
				return &in.Versions[j]
			}
		}
	}
	return nil
}

// metric is one label-over-value cell of the summary header.
// Each cell has a fixed basis so switching sessions never moves columns;
// narrow windows shrink cells and the value truncates (full text on hover).
func metric(c *ui.Context, key, name, value string, w float32) {
	metricTone(c, key, name, value, w, ink)
}

func metricTone(c *ui.Context, key, name, value string, w float32, col ui.Color) {
	s := uiScale(c)
	ui.Column(c.Key(key)).Gap(3 * s).Width(w * s).Grow(1).Shrink(1).MinWidth(0).Clip().Children(func() {
		ui.Text(c, name).FontSize(typeCaption2 * s).TextColor(muted).SingleLine()
		ui.Text(c, value).FontSize(typeFootnote * s).FontWeight(600).FontFeatures("tnum").TextColor(col).SingleLine().Tooltip(value)
	})
}

func metricRule(c *ui.Context) {
	s := uiScale(c)
	ui.Box(c).Width(1).Height(26 * s).Background(line).Shrink(0)
}

// sessionHeader summarizes the selected run: what, how long, on what.
func (a *App) sessionHeader(c *ui.Context, g *launcher.GameSession, scope string, sess []launcher.GameSession) {
	s := uiScale(c)
	ui.Row(c.Key("log-summary")).Label("日志摘要").FillWidth().Height(64*s).Shrink(0).Padding(0, 16*s).Gap(18*s).Radius(radiusCard*s).Background(ui.RGBA(255, 255, 255, .72)).Border(1, glassEdge).AlignItems(ui.Center).Children(func() {
		// With 2+ sessions the switcher already names the pack, so the
		// header keeps only the state; one session shows name and state.
		// In a pack's tab the hero above already shows the name, Minecraft
		// and loader, so the header keeps only what the run adds.
		inTab := scope != "" && a.dialog != "logs"
		multi := len(sess) > 1 || inTab
		if !multi {
			ui.Column(c).Gap(4 * s).Width(220 * s).Shrink(1).MinWidth(90 * s).Children(func() {
				ui.Row(c).Gap(8 * s).AlignItems(ui.Center).Children(func() {
					statusDot(c, sessionTone(g.State), s)
					ui.Text(c, a.sessionLabel(*g, sess)).FontSize(typeCallout * s).FontWeight(700).TextColor(ink).SingleLine().Tooltip(a.sessionLabel(*g, sess)).Shrink(1).MinWidth(0)
				})
				ui.Text(c, a.sessionStateText(g.State)).FontSize(typeCaption * s).TextColor(sessionTone(g.State)).NoWrap()
			})
			metricRule(c)
		}
		end := time.Now()
		if !g.Ended.IsZero() {
			end = g.Ended
		}
		ui.Row(c).Grow(1).MinWidth(0).Gap(18 * s).AlignItems(ui.Center).Clip().Children(func() {
			if multi {
				metricTone(c, "m-state", a.L("状态", "State"), a.sessionStateText(g.State), 64, sessionTone(g.State))
			}
			if g.AccountName != "" && !a.sessionDup(*g, sess) {
				ui.Column(c.Key("m-acc")).Gap(3 * s).Width(120 * s).Grow(1).Shrink(1).MinWidth(0).Clip().Children(func() {
					ui.Text(c, a.L("账号", "Account")).FontSize(typeCaption2 * s).TextColor(muted).SingleLine()
					a.sessionAccount(c, *g, 15*s, ink)
				})
			}
			metric(c, "m-dur", a.L("运行时长", "Run time"), a.fmtDuration(end.Sub(g.Started)), 100)
			mc, ld := "—", "—"
			if v := a.versionInfo(g.RootID, g.Version); v != nil {
				if v.Minecraft != "" {
					mc = v.Minecraft
				}
				ld = a.L("原版", "Vanilla")
				if v.Loader != "" && v.Loader != "vanilla" {
					ld = strings.TrimSpace(loaderName(v.Loader) + " " + v.LoaderVersion)
				}
			}
			if !inTab {
				metric(c, "m-mc", "Minecraft", mc, 72)
				metric(c, "m-loader", a.L("加载器", "Loader"), ld, 170)
			}
			java := g.Java
			if java == "" {
				java = a.L("读取中", "Reading")
			}
			metric(c, "m-java", "Java", java, 120)
			mem := a.L("自动", "Auto")
			if g.MemoryMB > 0 {
				mem = fmt.Sprintf("%d MB", g.MemoryMB)
			}
			metric(c, "m-mem", a.L("内存", "Memory"), mem, 84)
			if !a.logView.compact {
				metric(c, "m-start", a.L("启动于", "Started"), a.fmtWhen(g.Started), 110)
			}
			exit := "—"
			if g.State != "running" {
				exit = fmt.Sprint(g.ExitCode)
			}
			metric(c, "m-exit", a.L("退出码", "Exit code"), exit, 60)
		})
		if g.State == "running" && fixedButton(c, "log-stop", a.L("停止", "Stop"), "cross", 0, btnSm*s, false, false) && a.service != nil {
			if e := a.service.StopSession(g.ID); e != nil {
				a.message, a.errorMessage = a.friendlyErr(e), true
			}
			a.logSess.polled = time.Time{}
		}
		if scope != "" && len(a.logSess.list) > len(sess) && fixedButton(c, "log-all-games", a.L("全部游戏", "All games"), "terminal", 0, btnSm*s, false, false) {
			defer a.openDialog("logs") // after this pane finishes building
		}
	})
}

// fileHeader stands in for the summary while a saved log file is shown.
func (a *App) fileHeader(c *ui.Context) bool {
	s := uiScale(c)
	lv := &a.logView
	back := false
	var f *launcher.LogFile
	for i := range lv.recent {
		if lv.recent[i].Path == lv.file {
			f = &lv.recent[i]
		}
	}
	ui.Row(c.Key("log-summary")).Label("日志摘要").FillWidth().Height(64*s).Shrink(0).Padding(0, 16*s).Gap(18*s).Radius(radiusCard*s).Background(ui.RGBA(255, 255, 255, .72)).Border(1, glassEdge).AlignItems(ui.Center).Children(func() {
		back = fixedButton(c, "log-file-back", a.L("返回", "Back"), "chevron-left", 0, btnSm*s, false, false)
		ui.Column(c).Gap(4 * s).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, filepath.Base(lv.file)).FontSize(typeCallout * s).FontWeight(700).TextColor(ink).SingleLine()
			kind := a.L("日志文件", "Log file")
			if f != nil && f.Kind == "crash" {
				kind = a.L("崩溃报告", "Crash report")
			}
			ui.Text(c, kind).FontSize(typeCaption * s).TextColor(muted).NoWrap()
		})
		if f != nil {
			metricRule(c)
			metric(c, "m-size", a.L("大小", "Size"), fmtSize(f.Size), 80)
			metric(c, "m-mod", a.L("修改于", "Modified"), a.fmtWhen(f.ModTime), 100)
		}
	})
	return back
}

// crashCard explains a crash in plain words, with the report at hand.
func (a *App) crashCard(c *ui.Context, g *launcher.GameSession) {
	s := uiScale(c)
	in := g.Insight
	cause, advice := a.L(in.Cause, in.CauseEn), a.L(in.Advice, in.AdviceEn)
	ui.Row(c.Key("log-crash")).FillWidth().Height(106*s).Shrink(0).Padding(0, 16*s).Gap(14*s).Radius(radiusCard*s).Background(ui.Hex("#ff3b30").Alpha(.06)).Border(1, ui.Hex("#ff3b30").Alpha(.22)).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(34*s, 34*s).Radius(10 * s).Background(ui.Hex("#ff3b30").Alpha(.12)).Center().Shrink(0).Children(func() {
			ui.Icon(c, icons["info"]).Size(16*s, 16*s).TextColor(ui.Hex("#d70015"))
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(5 * s).Children(func() {
			ui.Text(c, a.L("可能的原因", "Probable cause")).FontSize(typeCaption2 * s).FontWeight(600).TextColor(ui.Hex("#d70015")).NoWrap()
			ui.Text(c, cause).FontSize(typeCallout * s).FontWeight(700).TextColor(ink).SingleLine().Tooltip(cause)
			ui.Row(c).Gap(8 * s).AlignItems(ui.Center).MinWidth(0).Children(func() {
				if in.Mod != "" {
					ui.Row(c).Shrink(0).Padding(2*s, 8*s).Radius(999).Background(ui.Hex("#ff3b30").Alpha(.12)).Children(func() {
						ui.Text(c, a.L("相关模组 ", "Mod ")+in.Mod).FontSize(typeCaption2 * s).FontWeight(700).TextColor(ui.Hex("#d70015")).NoWrap()
					})
				}
				ui.Text(c, advice).FontSize(typeFootnote * s).TextColor(inkSecondary).SingleLine().Shrink(1).MinWidth(0).Tooltip(advice)
			})
		})
		if g.CrashReport != "" {
			if fixedButton(c, "crash-open", a.L("打开崩溃报告", "Open report"), "doc", 0, btnSm*s, false, false) {
				openLogFile(c, g.CrashReport)
			}
			if fixedButton(c, "crash-copy", a.L("复制报告", "Copy report"), "", 0, btnSm*s, false, false) && a.service != nil {
				if t, e := a.service.CrashReportText(g.RootID, g.Version, g.CrashReport); e != nil {
					a.message, a.errorMessage = a.friendlyErr(e), true
				} else {
					c.WriteClipboard(t)
					a.notice(a.L("已复制崩溃报告", "Crash report copied"))
				}
			}
		} else {
			ui.Text(c, a.L("没有生成崩溃报告", "No crash report written")).FontSize(typeCaption * s).TextColor(muted).NoWrap().Shrink(0)
		}
	})
}

// logToolbar: level chips with counts, search with hit navigation, actions.
func (a *App) logToolbar(c *ui.Context, cur *launcher.GameSession, sk string) {
	s := uiScale(c)
	t := a.tr()
	lv := &a.logView
	ui.Row(c.Key("log-toolbar")).Label("日志工具栏").FillWidth().Height(btnMd * s).Shrink(0).Gap(8 * s).AlignItems(ui.Center).Children(func() {
		for _, it := range []struct{ key, zh, en string }{{"info", "信息", "Info"}, {"warn", "警告", "Warn"}, {"error", "错误", "Error"}, {"debug", "调试", "Debug"}} {
			k := it.key
			n := lv.counts[k]
			if k == "debug" && n == 0 {
				continue
			}
			sel := lv.levels[k]
			text := fmt.Sprintf("%s %d", a.L(it.zh, it.en), n)
			fg, _ := levelTone(map[string]string{"warn": "WARN", "error": "ERROR"}[k])
			if k == "info" || k == "debug" {
				fg = inkSecondary
			}
			b := ui.ButtonBase(c.Key("log-level-" + k)).Label(text).Height(btnSm * s).PaddingX(11 * s).Gap(6 * s).Radius(999).AlignItems(ui.Center).Shrink(0)
			bg, edge := ui.RGBA(255, 255, 255, .7), line
			if sel {
				bg, edge = accentSoft, accent.Alpha(.45)
			} else if b.Hovered() {
				bg = panel
			}
			b.Background(b.AnimateColor("bg", bg, fastAnim)).Border(1, edge)
			if b.Children(func() {
				if sel {
					ui.Icon(c, icons["check"]).Size(10*s, 10*s).TextColor(accent)
				} else {
					ui.Box(c).Size(7*s, 7*s).Radius(999).Background(fg)
				}
				ui.Text(c, a.L(it.zh, it.en)).FontSize(typeFootnote * s).FontWeight(600).TextColor(ink).NoWrap()
				ui.Text(c, fmt.Sprint(n)).FontSize(typeFootnote * s).FontFeatures("tnum").TextColor(muted).NoWrap()
			}).Clicked() {
				if lv.levels == nil {
					lv.levels = map[string]bool{}
				}
				if sel {
					delete(lv.levels, k)
				} else {
					lv.levels[k] = true
				}
			}
		}
		textField(c.Key("log-search"), &lv.query).Placeholder(a.L("搜索日志", "Search log")).Label(a.L("搜索日志", "Search log")).Grow(1).MinWidth(140 * s)
		if strings.TrimSpace(lv.query) != "" {
			pos := "0/0"
			if len(lv.hits) > 0 {
				pos = fmt.Sprintf("%d/%d", lv.match+1, len(lv.hits))
			}
			ui.Text(c, pos).FontSize(typeFootnote * s).FontFeatures("tnum").TextColor(muted).NoWrap().Shrink(0)
			rows := int(320 / logRowHeight)
			if iconOnlyButton(c, "log-hit-prev", "chevron-left", a.L("上一个", "Previous"), btnSm*s, len(lv.hits) == 0) {
				lv.match = (lv.match - 1 + len(lv.hits)) % len(lv.hits)
				a.logGoHit(rows)
			}
			if iconOnlyButton(c, "log-hit-next", "chevron-right", a.L("下一个", "Next"), btnSm*s, len(lv.hits) == 0) {
				lv.match = (lv.match + 1) % len(lv.hits)
				a.logGoHit(rows)
			}
		}
		compact := lv.compact
		if !compact {
			ui.Text(c, fmt.Sprintf(a.L("%d 行", "%d lines"), len(lv.vis))).FontSize(typeFootnote * s).FontFeatures("tnum").TextColor(muted).NoWrap().Shrink(0)
		}
		if fixedButton(c, "log-latest", a.L("最新", "Latest"), "chevron-down", 0, btnSm*s, false, lv.follow) {
			a.logJumpLatest()
		}
		copyLabel := t.LogsCopyBtn
		if compact {
			copyLabel = a.L("复制", "Copy")
		}
		if fixedButton(c, "copy-logs", copyLabel, "", 0, btnSm*s, false, len(lv.vis) == 0) {
			var b strings.Builder
			for _, i := range lv.vis {
				l := lv.lines[i]
				b.WriteString(l.Raw)
				if l.Repeat > 1 {
					fmt.Fprintf(&b, "  (×%d)", l.Repeat)
				}
				b.WriteByte('\n')
			}
			c.WriteClipboard(b.String())
			a.notice(fmt.Sprintf(a.L("已复制 %d 行", "Copied %d lines"), len(lv.vis)))
		}
		p := lv.file
		if p == "" {
			p = lv.path
			if cur != nil && cur.LogPath != "" {
				p = cur.LogPath
			}
		}
		open, clear := false, false
		if compact {
			open = p != "" && iconOnlyButton(c, "log-open-file", "doc", a.L("打开文件", "Open file"), btnSm*s, false)
			clear = lv.file == "" && iconOnlyButton(c, "log-clear", "trash", a.L("清空", "Clear"), btnSm*s, len(lv.lines) == 0)
		} else {
			open = p != "" && fixedButton(c, "log-open-file", a.L("打开文件", "Open file"), "doc", 0, btnSm*s, false, false)
			clear = lv.file == "" && fixedButton(c, "log-clear", a.L("清空", "Clear"), "trash", 0, btnSm*s, false, len(lv.lines) == 0)
		}
		if open {
			openLogFile(c, p)
		}
		if p != "" && iconOnlyButton(c, "log-open-folder", "folder", a.L("在文件夹中显示", "Show in folder"), btnSm*s, false) {
			openLogFolder(p)
		}
		if clear {
			lv.clearAt = lv.dropped + len(lv.lines)
		}
	})
}

// logEmpty is shown when nothing has run yet: launch, or open a recent file.
func (a *App) logEmpty(c *ui.Context, r *launcher.Instance, scope string) {
	s := uiScale(c)
	lv := &a.logView
	key := r.ID + "|" + r.SelectedVersion
	if a.service != nil && (lv.recentKey != key || time.Since(lv.recentAt) > 5*time.Second) {
		lv.recentKey, lv.recentAt = key, time.Now()
		id, ver := r.ID, r.SelectedVersion
		go func() {
			files, _ := a.service.RecentLogFiles(id, ver, 8)
			a.post(func() {
				if lv.recentKey == key {
					lv.recent = files
				}
			})
		}()
	}
	ui.Row(c.Key("log-empty")).Font(uiSansFamily()).FillWidth().MinHeight(108*s).Shrink(0).Padding(16*s, 20*s).Gap(16*s).Radius(radiusCard*s).Background(ui.RGBA(255, 255, 255, .76)).Border(1, glassEdge).AlignItems(ui.Center).Children(func() {
		emptyArtwork(c, "log-idle", "terminal", .8*s)
		ui.Column(c).Grow(1).MinWidth(0).Gap(5 * s).Children(func() {
			ui.Text(c, a.L("现在没有运行中的游戏", "No game is running")).FontSize(typeHeadline * s).FontWeight(600).TextColor(inkSecondary).FillWidth()
			ui.Text(c, a.L("启动游戏后查看实时输出，也可以打开下方的最近日志。", "Launch to follow the live output, or open a recent log below.")).FontSize(typeFootnote * s).TextColor(muted).FillWidth()
		})
		if scope != "" || a.dialog == "logs" {
			if buttonWhy(c, "log-launch", a.L("启动游戏", "Launch"), "arrow", true, whyOf(a.instancePending("launch", "launchAgain"), a.tr().WhyBusy, verRunning(r), a.L("这个版本正在运行", "Already running")), false) {
				a.launch()
			}
		}
	})
	ui.Row(c.Key("log-recent-head")).FillWidth().Height(24 * s).Shrink(0).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.L("最近日志", "Recent logs")).FontSize(typeCallout * s).FontWeight(700).TextColor(ink).Grow(1)
		ui.Text(c, fmt.Sprintf(a.L("%d 个文件", "%d files"), len(lv.recent))).FontSize(typeFootnote * s).TextColor(muted).NoWrap()
	})
	if len(lv.recent) == 0 {
		emptyNote(c, "log-recent-none", "terminal", a.L("还没有日志文件", "No log files yet"), a.L("运行游戏后，最近的日志会保留在这里", "Recent logs will appear here after you run the game"))
		return
	}
	scrollArea(c, a.logScrollKey()+"-recent").Label("日志文件列表").FillWidth().Grow(1).Basis(0).MinHeight(0).Children(func() {
		ui.Column(c.Key("log-recent")).FillWidth().Radius(radiusCard*s).Background(ui.RGBA(255, 255, 255, .72)).Border(1, glassEdge).Padding(6 * s).Gap(2 * s).Children(func() {
			for _, f := range lv.recent {
				f := f
				folderHit := false
				b := ui.ButtonBase(c.Key("log-recent-" + f.Name)).Label(f.Name).FillWidth().Height(44 * s).PaddingX(10 * s).Gap(12 * s).Radius(9 * s).AlignItems(ui.Center)
				if b.Hovered() {
					b.Background(ui.RGBA(120, 130, 160, .10))
				}
				if b.Children(func() {
					col, kind, ic := accent, a.L("日志", "Log"), "doc"
					if f.Kind == "crash" {
						col, kind, ic = ui.Hex("#d70015"), a.L("崩溃报告", "Crash report"), "info"
					}
					ui.Box(c).Size(26*s, 26*s).Radius(7 * s).Background(col.Alpha(.10)).Center().Shrink(0).Children(func() {
						ui.Icon(c, icons[ic]).Size(13*s, 13*s).TextColor(col)
					})
					ui.Text(c, f.Name).FontSize(typeFootnote * s).FontWeight(600).TextColor(ink).SingleLine().Grow(1).MinWidth(0).Tooltip(f.Name)
					ui.Row(c).Shrink(0).Padding(2*s, 8*s).Radius(999).Background(col.Alpha(.10)).Children(func() {
						ui.Text(c, kind).FontSize(typeCaption2 * s).FontWeight(600).TextColor(col).NoWrap()
					})
					ui.Text(c, a.fmtWhen(f.ModTime)).Width(110 * s).FontSize(typeFootnote * s).FontFeatures("tnum").TextColor(muted).NoWrap().Shrink(0)
					ui.Text(c, fmtSize(f.Size)).Width(70 * s).FontSize(typeFootnote * s).FontFeatures("tnum").TextColor(muted).NoWrap().Shrink(0)
					folderHit = iconOnlyButton(c, "log-folder-"+f.Path, "folder", a.L("在文件夹中显示：", "Show in folder: ")+f.Name, 28*s, false)
					ui.Icon(c, icons["chevron-right"]).Size(10*s, 10*s).TextColor(muted)
				}).Clicked() && !folderHit {
					lv.file, lv.key = f.Path, ""
				}
				if folderHit {
					openLogFolder(f.Path)
				}
			}
		})
	})
}
