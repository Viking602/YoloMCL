package nativeui

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type overviewPresentation struct {
	key    string
	root   launcher.Instance
	status launcher.InstanceStatus
}

// The overview only consumes cached data. Folder scans and thumbnail decoding
// use the existing background workers, including when switching cold packs.
func (a *App) instanceOverview(c *ui.Context, r launcher.Instance) {
	s := uiScale(c)
	if a.instStatusKey != r.ID+"|"+r.SelectedVersion {
		a.loadInstanceStatus()
	}
	st := a.instStatus
	a.autoCheckPack(r)
	requested := a.instStatusKey
	display := &a.overviewDisplay
	holding := st.Dir == "" && a.instStatusErr == "" && display.key != ""
	if holding {
		r, st = display.root, display.status
	} else if st.Dir != "" {
		display.key, display.root, display.status = requested, r, st
	}
	a.overviewHolding = holding
	defer func() { a.overviewHolding = false }()
	// The fixed overview canvas fits the viewport, so keep the dashboard grid
	// instead of turning small windows into a vertically scrolling page.
	narrow := false
	ui.Box(c.Key("overview-stage")).Label("概览过渡区域").FillWidth().Children(func() {
		ui.Column(c.Key("overview")).Label("整合包概览").FillWidth().Gap(16 * s).Children(func() {
			if a.instStatusErr != "" && st.Dir == "" {
				emptyState(c, "overview-error", "info", a.L("概览读取失败", "Couldn't read this instance"), a.instStatusErr, func() {
					if button(c, "overview-retry", a.L("重新读取概览", "Reload overview"), "refresh", true, false) {
						a.loadInstanceStatus()
					}
				})
				return
			}
			a.ovStats(c, r, st, narrow)
			moments := []func(){func() { a.ovWorld(c, r, st) }, func() { a.ovServer(c, st) }, func() { a.ovScreenshot(c, r, st) }}
			if narrow {
				for i, panel := range moments {
					ui.Row(c.Key(fmt.Sprint("ov-moment-", i))).FillWidth().Height(260 * s).Shrink(0).Children(panel)
				}
			} else {
				ui.Row(c.Key("ov-moments")).FillWidth().Height(260 * s).Shrink(0).Gap(16 * s).AlignItems(ui.Stretch).Children(func() {
					for _, panel := range moments {
						panel()
					}
				})
			}
			// Reserve the full row height for every pack, including cold reads.
			// Storage categories and pack actions must not move the panel footer.
			ovPair(c, "ov-details", narrow, 340, func() {
				a.ovPanel(c, "ov-run", a.L("运行环境", "Runtime"), st, func() {
					if iconOnlyButton(c, "ov-config", "settings", a.L("版本设置", "Version settings"), 28*s, false) && a.overviewActionReady("ov-run") {
						a.setInstTab("settings")
					}
				}, func() {
					var version *launcher.VersionSummary
					for i := range r.Versions {
						if r.Versions[i].ID == r.SelectedVersion {
							version = &r.Versions[i]
							break
						}
					}
					a.ovRuntime(c, r, version, st)
					if r.Pack != nil {
						a.ovPackActions(c, r)
					}
					ui.Spacer(c).Grow(1)
					a.ovLastSession(c, r, st)
				})
			}, func() {
				a.ovPanel(c, "ov-dir", a.L("存储空间", "Storage"), st, func() {
					text := "—"
					if st.StoragePending {
						text = a.L("统计中…", "Calculating…")
					} else if st.Dir != "" {
						text = fmtSize(st.Total)
					}
					a.flipNumber(c, "storage-total", text, typeHeadline*s, 700, ink, true).Width(104 * s)
					if iconOnlyButton(c, "ov-refresh", "refresh", a.L("刷新概览", "Refresh overview"), 26*s, false) && a.overviewActionReady("ov-dir") {
						a.loadInstanceStatus()
					}
				}, func() { a.ovDir(c, st, r, false) })
			})
		})
	})
}

func ovPair(c *ui.Context, key string, narrow bool, height float32, left, right func()) {
	s := uiScale(c)
	if narrow {
		ui.Column(c.Key(key)).FillWidth().Gap(16 * s).Children(func() {
			ui.Row(c.Key(key + "-first")).FillWidth().Height(height * s).Shrink(0).Children(left)
			ui.Row(c.Key(key + "-second")).FillWidth().Height(height * s).Shrink(0).Children(right)
		})
		return
	}
	ui.Row(c.Key(key)).FillWidth().Height(height * s).Shrink(0).Gap(16 * s).AlignItems(ui.Stretch).Children(func() {
		left()
		right()
	})
}

func (a *App) ovStats(c *ui.Context, r launcher.Instance, st launcher.InstanceStatus, narrow bool) {
	s := uiScale(c)
	ready := st.Dir != ""
	count := func(n int) string {
		if !ready {
			return "—"
		}
		return strconv.Itoa(n)
	}
	modHint := a.L("管理模组与更新", "Manage installed mods")
	if ready {
		modHint = fmt.Sprintf(a.L("%d 个已启用", "%d enabled"), st.EnabledMods)
	}
	play, playHint := "—", a.L("从这里启动后记录时长", "Tracked when launched here")
	if st.Play.Launches > 0 {
		play = durText(a, st.Play.TotalSec)
		playHint = fmt.Sprintf(a.L("累计 %d 次启动", "%d recorded launches"), st.Play.Launches)
	}
	visualHint := a.L("资源包与光影", "Resource packs and shaders")
	if ready {
		visualHint = fmt.Sprintf(a.L("%d 资源包，%d 光影", "%d packs, %d shaders"), st.ResourcePacks, st.Shaders)
	}
	items := []struct {
		key, title, value, hint, icon, tab string
		tone                               ui.Color
	}{
		{"mods", a.L("已安装模组", "Installed mods"), count(st.Mods), modHint, "mods", "mods", accent},
		{"worlds", a.L("本地世界", "Local worlds"), count(st.Saves), a.L("管理存档与备份", "Manage saves and backups"), "globe", "worlds", successInk},
		{"visuals", a.L("画面扩展", "Visual extras"), count(st.ResourcePacks + st.Shaders), visualHint, "layers", "resourcepack", accentViolet},
		{"play", a.L("累计游玩", "Play time"), play, playHint, "gamepad", "logs", accentIndigo},
	}
	cols := 4
	if narrow {
		cols = 2
	}
	ui.Column(c.Key("ov-stats")).FillWidth().Gap(12 * s).Children(func() {
		for start := 0; start < len(items); start += cols {
			ui.Row(c).FillWidth().Height(108 * s).Shrink(0).Gap(12 * s).Children(func() {
				for _, it := range items[start:min(start+cols, len(items))] {
					b := ui.ButtonBase(c.Key("ov-stat-"+it.key)).Label(it.title).Grow(1).Basis(0).MinWidth(0).FillHeight().Clip().Padding(14*s, 16*s).Radius(radiusCard*s).Border(1, glassEdge)
					bg := ui.RGBA(255, 255, 255, .76)
					if b.Hovered() {
						bg = it.tone.Alpha(.08)
					}
					b.Background(b.AnimateColor("bg", bg, fastAnim))
					if b.Children(func() {
						ui.Column(c).Fill().Gap(6 * s).Children(func() {
							ui.Row(c).FillWidth().Gap(7 * s).AlignItems(ui.Center).Children(func() {
								ui.Icon(c, icons[it.icon]).Size(13*s, 13*s).TextColor(it.tone).Shrink(0)
								ui.Text(c, it.title).FontSize(typeFootnote * s).TextColor(muted).SingleLine().Grow(1).MinWidth(0)
							})
							key := "stat-value-" + it.key
							a.flipNumber(c, key, it.value, typeDisplay*s, 750, ink).FillWidth()
							hintKey := "stat-hint-" + it.key
							a.flipNumber(c, hintKey, it.hint, typeCaption*s, 400, muted).FillWidth().Tooltip(it.hint)
						})
					}).Clicked() && a.overviewActionReady("stat-value-"+it.key) {
						a.setInstTab(it.tab)
					}
				}
			})
		}
	})
}

func (a *App) ovWorld(c *ui.Context, r launcher.Instance, st launcher.InstanceStatus) {
	s := uiScale(c)
	iconScope := ""
	var worldSignature any
	if st.RecentWorld != nil && st.RecentWorld.HasIcon {
		iconScope = r.ID + "|" + r.SelectedVersion
	}
	if w := st.RecentWorld; w != nil {
		worldSignature = []any{w.Dir, w.Name, w.Mode, w.LastPlayed, w.HasIcon}
	}
	r, st = a.overviewPanelData(c, "ov-world", []any{st.Dir != "", worldSignature, iconScope}, r, st)
	a.resetContentChildren("ov-world", "world-size", "world-backups")
	a.ovPanel(c, "ov-world", a.L("最近世界", "Recent world"), st, func() {
		if fixedButton(c, "ov-worlds", a.L("全部存档", "All worlds"), "chevron", 0, 26*s, false, false) && a.overviewActionReady("ov-world") {
			a.setInstTab("worlds")
		}
	}, func() {
		if st.Dir == "" {
			ovLoading(c)
			return
		}
		w := st.RecentWorld
		if w == nil {
			title, hint := a.L("下一个世界，等你创造", "Your next world starts here"), a.L("进入游戏创建世界后，会在这里显示最近的存档", "Create a world in game to see it here")
			ui.Column(c).FillWidth().Grow(1).Gap(12 * s).Center().Children(func() {
				ui.Box(c).Size(56*s, 56*s).Radius(16 * s).Background(successSoft).Center().Children(func() {
					ui.Icon(c, icons["globe"]).Size(26*s, 26*s).TextColor(successInk)
				})
				ui.Text(c, title).FontSize(typeTitle * s).FontWeight(650).TextColor(ink).SingleLine()
				ui.Text(c, hint).FontSize(typeFootnote * s).TextColor(muted).SingleLine().FillWidth().TextAlign(ui.Center)
			})
			return
		}
		ui.Row(c).FillWidth().PaddingY(12 * s).Gap(16 * s).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(72*s, 72*s).Shrink(0).Radius(14 * s).Clip().Background(successSoft).Center().Children(func() {
				var img ui.ImageSource
				if w.HasIcon {
					id, ver, dir := r.ID, r.SelectedVersion, w.Dir
					img = a.localIcon("world:"+id+":"+ver+":"+dir, func() ([]byte, error) { return a.service.WorldIcon(id, ver, dir) })
				}
				if img != nil {
					image := ui.Image(c.Key("world-preview-ready"), img).Size(72*s, 72*s).Fit(ui.Cover)
					if !c.Preferences().ReduceMotion {
						image.Transition(ui.ElementTransition{Animation: &motionEnter, Enter: &ui.Motion{Opacity: 0}})
					}
				} else {
					ui.Icon(c, icons["cube"]).Size(32*s, 32*s).TextColor(successInk)
				}
			})
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(9 * s).Children(func() {
				ui.Text(c, w.Name).FontSize(typeTitle1 * s).FontWeight(700).TextColor(ink).SingleLine().FillWidth().Tooltip(w.Name)
				ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
					if mode, ok := worldModes[w.Mode]; ok {
						quietTag(c, a.L(mode[0], mode[1]), successInk)
					}
					ui.Text(c, a.L("上次游玩 ", "Played ")+agoText(a, w.LastPlayed)).FontSize(typeFootnote * s).TextColor(muted).SingleLine().Grow(1).MinWidth(0)
				})
			})
		})
		ui.Row(c).FillWidth().Height(64*s).Shrink(0).Padding(10*s, 14*s).Gap(20 * s).Radius(10 * s).Background(raised).Children(func() {
			size := fmtSize(w.Size)
			if st.StoragePending {
				size = a.L("统计中…", "Calculating…")
			}
			for _, it := range [][3]string{
				{a.L("世界大小", "World size"), size, "world-size"},
				{a.L("本地备份", "Local backups"), fmt.Sprintf(a.L("%d 份", "%d saved"), w.Backups), "world-backups"},
			} {
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(5 * s).Children(func() {
					ui.Text(c, it[0]).FontSize(typeCaption * s).TextColor(muted).SingleLine()
					a.flipNumber(c, it[2], it[1], typeCallout*s, 650, ink).FillWidth()
				})
			}
		})
		ui.Spacer(c).Grow(1)
		ui.Row(c).FillWidth().Height(30 * s).Shrink(0).AlignItems(ui.Center).Children(func() {
			ui.Text(c, w.Dir).FontSize(typeCaption * s).TextColor(muted).SingleLine().Grow(1).Basis(0).MinWidth(0).Tooltip(w.Dir)
			if fixedButton(c, "ov-world-backups", a.L("管理备份", "Manage backups"), "folder", 0, btnSm*s, false, false) && a.overviewActionReady("ov-world") {
				a.setInstTab("worlds")
				a.wv.open = w.Dir
			}
		})
	})
}

func (a *App) ovScreenshot(c *ui.Context, r launcher.Instance, st launcher.InstanceStatus) {
	s := uiScale(c)
	imageKey := ""
	if st.LatestShot != nil {
		imageKey = overviewShotKey(r, *st.LatestShot)
	}
	r, st = a.overviewPanelData(c, "ov-shot", []any{st.Dir != "", st.Screenshots, imageKey}, r, st)
	a.ovPanel(c, "ov-shot", a.L("游戏截图", "Screenshots"), st, func() {
		if st.Screenshots > 0 {
			quietTag(c, fmt.Sprintf(a.L("%d 张", "%d shots"), st.Screenshots), muted)
		}
		if iconOnlyButton(c, "ov-shots-open", "folder", a.L("打开截图文件夹", "Open screenshots folder"), 28*s, false) && a.overviewActionReady("ov-shot") {
			a.openSub("screenshots")
		}
	}, func() {
		if st.Dir == "" {
			ovLoading(c)
			return
		}
		var img ui.ImageSource
		if shot := st.LatestShot; shot != nil {
			id, ver, file := r.ID, r.SelectedVersion, shot.File
			key := overviewShotKey(r, *shot)
			img = a.localIcon(key, func() ([]byte, error) { return a.service.ScreenshotPreview(id, ver, file) })
		}
		ui.Box(c.Key("ov-shot-image")).FillWidth().Grow(1).Basis(0).MinHeight(0).Radius(12 * s).Clip().Background(accentSoft).Children(func() {
			if img != nil {
				image := ui.Image(c.Key("screenshot-preview-ready"), img).Absolute().Left(0).Top(0).Right(0).Bottom(0).Fit(ui.Cover)
				if !c.Preferences().ReduceMotion {
					image.Transition(ui.ElementTransition{Animation: &motionEnter, Enter: &ui.Motion{Opacity: 0}})
				}
				return
			}
			ui.Image(c, landscape).Absolute().Left(0).Top(0).Right(0).Bottom(0).Fit(ui.Cover).Opacity(.16)
			ui.Column(c).Fill().Center().PaddingX(16 * s).Gap(9 * s).Children(func() {
				title, hint := a.L("留住游戏里的风景", "Keep a little of your world"), a.L("游戏中按 F2，截图会出现在这里", "Press F2 in game to save a screenshot")
				if st.LatestShot != nil {
					title, hint = a.L("最新截图", "Latest screenshot"), a.L("可打开截图文件夹查看原图", "Open the folder to view the original")
				}
				ui.Text(c, title).FontSize(typeTitle * s).FontWeight(650).TextColor(ink).SingleLine()
				ui.Text(c, hint).FontSize(typeFootnote * s).TextColor(muted).SingleLine().FillWidth().TextAlign(ui.Center)
			})
		})
		ui.Row(c).FillWidth().Height(26 * s).Shrink(0).Gap(12 * s).AlignItems(ui.Center).Children(func() {
			text, date := a.L("记录你的下一次冒险", "A place for your next adventure"), "F2"
			if shot := st.LatestShot; shot != nil {
				text, date = shot.File, agoText(a, shot.Modified)
			}
			ui.Text(c, text).FontSize(typeCaption * s).TextColor(muted).SingleLine().Grow(1).Basis(0).MinWidth(0).Tooltip(text)
			ui.Text(c, date).FontSize(typeCaption * s).TextColor(muted).NoWrap().Shrink(0)
		})
	})
}

func overviewShotKey(r launcher.Instance, shot launcher.InstanceScreenshot) string {
	return fmt.Sprintf("screenshot:%s:%s:%s:%d:%d", r.ID, r.SelectedVersion, shot.File, shot.Modified.UnixNano(), shot.Size)
}

// Card surfaces and titles stay mounted. Only a changed content slot transitions.
func (a *App) ovPanel(c *ui.Context, key, title string, st launcher.InstanceStatus, actions, content func()) {
	ovRegion(c, key, title, func() {
		if actions != nil {
			ui.Row(c.Key(key + "-actions")).Gap(6 * uiScale(c)).AlignItems(ui.Center).Children(actions)
		}
	}, func() {
		a.contentBody(c, key).Grow(1).MinHeight(0).Gap(4 * uiScale(c)).Children(content)
	})
}

func ovLoading(c *ui.Context) {
	s := uiScale(c)
	ui.Column(c).Label("概览加载占位").FillWidth().Grow(1).MinHeight(0).PaddingY(12 * s).Gap(12 * s).Children(func() {
		ui.Box(c).Size(44*s, 44*s).Radius(12 * s).Background(raised)
		for _, width := range []float32{.72, .48, .60} {
			ui.Row(c).FillWidth().Height(10 * s).Children(func() {
				ui.Box(c).Grow(width).Basis(0).FillHeight().Radius(5 * s).Background(raised)
				ui.Spacer(c).Grow(1 - width).Basis(0)
			})
		}
	})
}

// Runs with the status scan's cancellation token, off the UI thread.
func (a *App) prepareOverviewImages(ctx context.Context, id, ver string, st launcher.InstanceStatus) {
	prepare := func(key string, read func() ([]byte, error)) {
		a.imageMu.Lock()
		known := a.images[key] != nil || !a.imageRetry[key].IsZero()
		a.imageMu.Unlock()
		if known || ctx.Err() != nil {
			return
		}
		select {
		case a.overviewImageSlots <- struct{}{}:
			defer func() { <-a.overviewImageSlots }()
		case <-ctx.Done():
			return
		}
		var source ui.ImageSource
		if b, err := read(); err == nil && ctx.Err() == nil {
			source, _ = decodeResourceImage(b)
		}
		if ctx.Err() != nil {
			return
		}
		a.imageMu.Lock()
		if source != nil {
			a.images[key] = source
		} else {
			a.imageRetry[key] = time.Now().Add(24 * time.Hour)
		}
		a.imageMu.Unlock()
	}
	if world := st.RecentWorld; world != nil && world.HasIcon {
		prepare("world:"+id+":"+ver+":"+world.Dir, func() ([]byte, error) { return a.service.WorldIcon(id, ver, world.Dir) })
	}
	if shot := st.LatestShot; shot != nil {
		r := launcher.Instance{Root: launcher.Root{ID: id, SelectedVersion: ver}}
		prepare(overviewShotKey(r, *shot), func() ([]byte, error) { return a.service.ScreenshotPreview(id, ver, shot.File) })
	}
}

func (a *App) ovLastSession(c *ui.Context, r launcher.Instance, st launcher.InstanceStatus) {
	s := uiScale(c)
	title, detail, tone := a.L("尚无游玩记录", "No recorded sessions"), a.L("启动后可在日志中查看运行记录", "Launch to start recording sessions"), muted
	switch {
	case verRunning(&r):
		title, detail, tone = a.L("游戏正在运行", "Game is running"), a.L("打开日志查看实时输出", "View live output in logs"), successInk
	case st.Play.Launches > 0:
		title = a.L("上次游玩 ", "Last played ") + durText(a, st.Play.LastSec)
		detail = agoText(a, st.Play.LastStart)
		if st.Play.LastExit == "crashed" {
			title, tone = a.L("上次运行异常退出", "Last session crashed"), danger
		} else if st.Play.LastExit == "exited" || st.Play.LastExit == "stopped" {
			detail += a.L("，已结束", ", finished")
		}
	case !st.LastLog.IsZero():
		title, detail = a.L("最近日志", "Recent log"), agoText(a, st.LastLog)
	}
	b := ui.ButtonBase(c.Key("ov-last-session")).Label(a.L("查看运行日志", "View session logs")).FillWidth().Height(58*s).Shrink(0).Padding(10*s, 12*s).Gap(10 * s).Radius(10 * s).Background(raised).AlignItems(ui.Center)
	if b.Children(func() {
		ui.Icon(c, icons["terminal"]).Size(18*s, 18*s).TextColor(tone).Shrink(0)
		text := swappedContent(a, c, "last-session-text", [2]string{title, detail}, [2]string{title, detail})
		a.contentBody(c, "last-session-text").Grow(1).Basis(0).MinWidth(0).Gap(4 * s).Children(func() {
			ui.Text(c, text[0]).FontSize(typeFootnote * s).FontWeight(600).TextColor(tone).SingleLine().FillWidth()
			ui.Text(c, text[1]).FontSize(typeCaption * s).TextColor(muted).SingleLine().FillWidth()
		})
		ui.Icon(c, icons["chevron"]).Size(10*s, 10*s).TextColor(muted).Shrink(0)
	}).Clicked() && a.overviewActionReady("last-session-text") {
		a.setInstTab("logs")
	}
}
