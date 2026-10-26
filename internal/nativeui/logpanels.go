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
		return a.L("logpanels.under_1_min")
	case m < 60:
		return fmt.Sprintf(a.L("logpanels.min"), m)
	}
	return fmt.Sprintf(a.L("logpanels.h_min"), m/60, m%60)
}

func (a *App) fmtWhen(t time.Time) string {
	now := time.Now()
	if t.YearDay() == now.YearDay() && t.Year() == now.Year() {
		return a.L("addons_ui.today") + t.Format("15:04")
	}
	return t.Format(a.L("logpanels.jan_02_15_04"))
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
		tooltip(c, ui.Text(c, value).FontSize(typeFootnote*s).FontWeight(600).FontFeatures("tnum").TextColor(col).SingleLine(), value)
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
					tooltip(c, ui.Text(c, a.sessionLabel(*g, sess)).FontSize(typeCallout*s).FontWeight(700).TextColor(ink).SingleLine(), a.sessionLabel(*g, sess)).Shrink(1).MinWidth(0)
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
				metricTone(c, "m-state", a.L("logpanels.state"), a.sessionStateText(g.State), 64, sessionTone(g.State))
			}
			if g.AccountName != "" && !a.sessionDup(*g, sess) {
				ui.Column(c.Key("m-acc")).Gap(3 * s).Width(120 * s).Grow(1).Shrink(1).MinWidth(0).Clip().Children(func() {
					ui.Text(c, a.L("logpanels.account")).FontSize(typeCaption2 * s).TextColor(muted).SingleLine()
					a.sessionAccount(c, *g, 15*s, ink)
				})
			}
			metric(c, "m-dur", a.L("logpanels.run_time"), a.fmtDuration(end.Sub(g.Started)), 100)
			mc, ld := "—", "—"
			if v := a.versionInfo(g.RootID, g.Version); v != nil {
				if v.Minecraft != "" {
					mc = v.Minecraft
				}
				ld = a.L("logpanels.vanilla")
				if v.Loader != "" && v.Loader != "vanilla" {
					ld = strings.TrimSpace(loaderName(v.Loader) + " " + v.LoaderVersion)
				}
			}
			if !inTab {
				metric(c, "m-mc", "Minecraft", mc, 72)
				metric(c, "m-loader", a.L("logpanels.loader"), ld, 170)
			}
			java := g.Java
			if java == "" {
				java = a.L("addons_tab.reading")
			}
			metric(c, "m-java", "Java", java, 120)
			mem := a.L("logpanels.auto")
			if g.MemoryMB > 0 {
				mem = fmt.Sprintf("%d MB", g.MemoryMB)
			}
			metric(c, "m-mem", a.L("instance_settings.memory"), mem, 84)
			if !a.logView.compact {
				metric(c, "m-start", a.L("logpanels.started"), a.fmtWhen(g.Started), 110)
			}
			exit := "—"
			if g.State != "running" {
				exit = fmt.Sprint(g.ExitCode)
			}
			metric(c, "m-exit", a.L("logpanels.exit_code"), exit, 60)
		})
		if g.State == "running" && fixedButton(c, "log-stop", a.L("logpanels.stop"), "cross", 0, btnSm*s, false, false) && a.service != nil {
			if e := a.service.StopSession(g.ID); e != nil {
				a.message, a.errorMessage = a.friendlyErr(e), true
			}
			a.logSess.polled = time.Time{}
		}
		if scope != "" && len(a.logSess.list) > len(sess) && fixedButton(c, "log-all-games", a.L("logpanels.all_games"), "terminal", 0, btnSm*s, false, false) {
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
		back = fixedButton(c, "log-file-back", a.L("logpanels.back"), "chevron-left", 0, btnSm*s, false, false)
		ui.Column(c).Gap(4 * s).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, filepath.Base(lv.file)).FontSize(typeCallout * s).FontWeight(700).TextColor(ink).SingleLine()
			kind := a.L("logpanels.log_file")
			if f != nil && f.Kind == "crash" {
				kind = a.L("logpanels.crash_report")
			}
			ui.Text(c, kind).FontSize(typeCaption * s).TextColor(muted).NoWrap()
		})
		if f != nil {
			metricRule(c)
			metric(c, "m-size", a.L("logpanels.size"), fmtSize(f.Size), 80)
			metric(c, "m-mod", a.L("logpanels.modified"), a.fmtWhen(f.ModTime), 100)
		}
	})
	return back
}

// crashCard explains a crash in plain words, with the report at hand.
func (a *App) crashCard(c *ui.Context, g *launcher.GameSession) {
	s := uiScale(c)
	in := g.Insight
	cause, advice := in.Cause, in.Advice
	if a.isEN() {
		cause, advice = in.CauseEn, in.AdviceEn
	}
	ui.Row(c.Key("log-crash")).FillWidth().Height(106*s).Shrink(0).Padding(0, 16*s).Gap(14*s).Radius(radiusCard*s).Background(ui.Hex("#ff3b30").Alpha(.06)).Border(1, ui.Hex("#ff3b30").Alpha(.22)).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(34*s, 34*s).Radius(10 * s).Background(ui.Hex("#ff3b30").Alpha(.12)).Center().Shrink(0).Children(func() {
			ui.Icon(c, icons["info"]).Size(16*s, 16*s).TextColor(ui.Hex("#d70015"))
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(5 * s).Children(func() {
			ui.Text(c, a.L("logpanels.probable_cause")).FontSize(typeCaption2 * s).FontWeight(600).TextColor(ui.Hex("#d70015")).NoWrap()
			tooltip(c, ui.Text(c, cause).FontSize(typeCallout*s).FontWeight(700).TextColor(ink).SingleLine(), cause)
			ui.Row(c).Gap(8 * s).AlignItems(ui.Center).MinWidth(0).Children(func() {
				if in.Mod != "" {
					ui.Row(c).Shrink(0).Padding(2*s, 8*s).Radius(999).Background(ui.Hex("#ff3b30").Alpha(.12)).Children(func() {
						ui.Text(c, a.L("logpanels.mod")+in.Mod).FontSize(typeCaption2 * s).FontWeight(700).TextColor(ui.Hex("#d70015")).NoWrap()
					})
				}
				tooltip(c, ui.Text(c, advice).FontSize(typeFootnote*s).TextColor(inkSecondary).SingleLine().Shrink(1).MinWidth(0), advice)
			})
		})
		if g.CrashReport != "" {
			if fixedButton(c, "crash-open", a.L("logpanels.open_report"), "doc", 0, btnSm*s, false, false) {
				openLogFile(c, g.CrashReport)
			}
			if fixedButton(c, "crash-copy", a.L("logpanels.copy_report"), "", 0, btnSm*s, false, false) && a.service != nil {
				if t, e := a.service.CrashReportText(g.RootID, g.Version, g.CrashReport); e != nil {
					a.message, a.errorMessage = a.friendlyErr(e), true
				} else {
					c.WriteClipboard(t)
					a.notice(a.L("logpanels.crash_report_copied"))
				}
			}
		} else {
			ui.Text(c, a.L("logpanels.no_crash_report_written")).FontSize(typeCaption * s).TextColor(muted).NoWrap().Shrink(0)
		}
	})
}

// logToolbar: level chips with counts, search with hit navigation, actions.
func (a *App) logToolbar(c *ui.Context, cur *launcher.GameSession, sk string) {
	s := uiScale(c)
	t := a.tr()
	lv := &a.logView
	ui.Row(c.Key("log-toolbar")).Label("日志工具栏").FillWidth().Height(btnMd * s).Shrink(0).Gap(8 * s).AlignItems(ui.Center).Children(func() {
		for _, k := range []string{"info", "warn", "error", "debug"} {
			n := lv.counts[k]
			if k == "debug" && n == 0 {
				continue
			}
			sel := lv.levels[k]
			text := fmt.Sprintf("%s %d", a.L("log.level."+k), n)
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
				ui.Text(c, a.L("log.level."+k)).FontSize(typeFootnote * s).FontWeight(600).TextColor(ink).NoWrap()
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
		textField(c.Key("log-search"), &lv.query).Placeholder(a.L("logpanels.search_log")).Label(a.L("logpanels.search_log")).Grow(1).MinWidth(140 * s)
		if strings.TrimSpace(lv.query) != "" {
			pos := "0/0"
			if len(lv.hits) > 0 {
				pos = fmt.Sprintf("%d/%d", lv.match+1, len(lv.hits))
			}
			ui.Text(c, pos).FontSize(typeFootnote * s).FontFeatures("tnum").TextColor(muted).NoWrap().Shrink(0)
			rows := int(320 / logRowHeight)
			if iconOnlyButton(c, "log-hit-prev", "chevron-left", a.L("logpanels.previous"), btnSm*s, len(lv.hits) == 0) {
				lv.match = (lv.match - 1 + len(lv.hits)) % len(lv.hits)
				a.logGoHit(rows)
			}
			if iconOnlyButton(c, "log-hit-next", "chevron-right", a.L("logpanels.next"), btnSm*s, len(lv.hits) == 0) {
				lv.match = (lv.match + 1) % len(lv.hits)
				a.logGoHit(rows)
			}
		}
		compact := lv.compact
		if !compact {
			ui.Text(c, fmt.Sprintf(a.L("logpanels.lines"), len(lv.vis))).FontSize(typeFootnote * s).FontFeatures("tnum").TextColor(muted).NoWrap().Shrink(0)
		}
		if fixedButton(c, "log-latest", a.L("logpanels.latest"), "chevron-down", 0, btnSm*s, false, lv.follow) {
			a.logJumpLatest()
		}
		copyLabel := t.LogsCopyBtn
		if compact {
			copyLabel = a.L("logpanels.copy")
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
			a.notice(fmt.Sprintf(a.L("logpanels.copied_lines"), len(lv.vis)))
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
			open = p != "" && iconOnlyButton(c, "log-open-file", "doc", a.L("logpanels.open_file"), btnSm*s, false)
			clear = lv.file == "" && iconOnlyButton(c, "log-clear", "trash", a.L("logpanels.clear"), btnSm*s, len(lv.lines) == 0)
		} else {
			open = p != "" && fixedButton(c, "log-open-file", a.L("logpanels.open_file"), "doc", 0, btnSm*s, false, false)
			clear = lv.file == "" && fixedButton(c, "log-clear", a.L("logpanels.clear"), "trash", 0, btnSm*s, false, len(lv.lines) == 0)
		}
		if open {
			openLogFile(c, p)
		}
		if p != "" && iconOnlyButton(c, "log-open-folder", "folder", a.L("logpanels.show_in_folder"), btnSm*s, false) {
			openLogFolder(p)
		}
		if clear {
			lv.clearAt = lv.dropped + len(lv.lines)
		}
	})
}

// logEmpty is shown when nothing has run yet: launch, or open a recent file.
func (a *App) logEmpty(c *ui.Context, r *launcher.Instance, scope string) {
	c.After(5 * time.Second)
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
			ui.Text(c, a.L("logpanels.no_game_is_running")).FontSize(typeHeadline * s).FontWeight(600).TextColor(inkSecondary).FillWidth()
			ui.Text(c, a.L("logpanels.launch_to_follow_the_live_output_or_open")).FontSize(typeFootnote * s).TextColor(muted).FillWidth()
		})
		if scope != "" || a.dialog == "logs" {
			if buttonWhy(c, "log-launch", a.L("logpanels.launch"), "arrow", true, whyOf(a.instancePending("launch", "launchAgain"), a.tr().WhyBusy, verRunning(r), a.L("logpanels.already_running")), false) {
				a.launch()
			}
		}
	})
	ui.Row(c.Key("log-recent-head")).FillWidth().Height(24 * s).Shrink(0).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.L("logpanels.recent_logs")).FontSize(typeCallout * s).FontWeight(700).TextColor(ink).Grow(1)
		ui.Text(c, fmt.Sprintf(a.L("logpanels.files"), len(lv.recent))).FontSize(typeFootnote * s).TextColor(muted).NoWrap()
	})
	if len(lv.recent) == 0 {
		emptyNote(c, "log-recent-none", "terminal", a.L("logpanels.no_log_files_yet"), a.L("logpanels.recent_logs_will_appear_here_after_you_run"))
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
					col, kind, ic := accent, a.L("logpanels.log"), "doc"
					if f.Kind == "crash" {
						col, kind, ic = ui.Hex("#d70015"), a.L("logpanels.crash_report"), "info"
					}
					ui.Box(c).Size(26*s, 26*s).Radius(7 * s).Background(col.Alpha(.10)).Center().Shrink(0).Children(func() {
						ui.Icon(c, icons[ic]).Size(13*s, 13*s).TextColor(col)
					})
					tooltip(c, ui.Text(c, f.Name).FontSize(typeFootnote*s).FontWeight(600).TextColor(ink).SingleLine().Grow(1).MinWidth(0), f.Name)
					ui.Row(c).Shrink(0).Padding(2*s, 8*s).Radius(999).Background(col.Alpha(.10)).Children(func() {
						ui.Text(c, kind).FontSize(typeCaption2 * s).FontWeight(600).TextColor(col).NoWrap()
					})
					ui.Text(c, a.fmtWhen(f.ModTime)).Width(110 * s).FontSize(typeFootnote * s).FontFeatures("tnum").TextColor(muted).NoWrap().Shrink(0)
					ui.Text(c, fmtSize(f.Size)).Width(70 * s).FontSize(typeFootnote * s).FontFeatures("tnum").TextColor(muted).NoWrap().Shrink(0)
					folderHit = iconOnlyButton(c, "log-folder-"+f.Path, "folder", a.L("logpanels.show_in_folder_2")+f.Name, 28*s, false)
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
