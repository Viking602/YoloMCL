package nativeui

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// worldView is the 存档 tab: world cards with 打开 / 备份 / 还原.
type worldView struct {
	key            string // root|version the list belongs to
	list           []launcher.World
	loading        bool
	open           string // world whose backups are expanded
	backups        []launcher.WorldBackup
	backupsLoading bool
	confirm        string // "restore:<file>" or "delete:<file>"
	seen           map[string]string
}

func (a *App) loadWorlds() {
	r := a.root()
	if r == nil || a.service == nil {
		return
	}
	id, ver, key := r.ID, r.SelectedVersion, r.ID+"|"+r.SelectedVersion
	if a.wv.key != key {
		a.wv = worldView{key: key, seen: a.wv.seen}
	}
	a.wv.loading = true
	a.worldListGen++
	gen := a.worldListGen
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		ws, e := a.service.ListWorlds(id, ver)
		a.post(func() {
			if a.wv.key != key || gen != a.worldListGen {
				return
			}
			a.wv.loading = false
			if e == nil {
				a.wv.list = ws
			}
			if a.wv.open != "" {
				a.loadWorldBackups(a.wv.open)
			}
		})
	}()
}

func (a *App) loadWorldBackups(world string) {
	r := a.root()
	if r == nil || a.service == nil {
		return
	}
	id, ver, key := r.ID, r.SelectedVersion, r.ID+"|"+r.SelectedVersion
	a.worldBackupsGen++
	gen := a.worldBackupsGen
	a.wv.backupsLoading = true
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		bs, _ := a.service.WorldBackups(id, ver, world)
		a.post(func() {
			if gen == a.worldBackupsGen && a.wv.key == key && a.wv.open == world {
				a.wv.backups = bs
				a.wv.backupsLoading = false
			}
		})
	}()
}

func (a *App) currentWorldJob(j *launcher.Job) bool {
	r := a.root()
	return r != nil && strings.HasPrefix(j.TitleKey, "world.") && j.TargetRootID == r.ID && j.TargetVersion == r.SelectedVersion
}

// worldJob includes an all-worlds backup; an empty world matches any job that
// would conflict with the "back up all" action, only in the selected instance.
func (a *App) worldJob(world string) *launcher.Job {
	for i := range a.snapshot.Jobs {
		j := &a.snapshot.Jobs[i]
		if j.State != "running" || !a.currentWorldJob(j) {
			continue
		}
		if world == "" || j.TitleKey == "world.backupAll" || j.TitleArg == world {
			return j
		}
	}
	return nil
}

// Covers the gap between clicking and the job appearing in the snapshot.
func (a *App) worldBusy(world string) bool {
	if a.worldJob(world) != nil {
		return true
	}
	r := a.root()
	if r == nil {
		return false
	}
	for _, req := range a.pendingRequests {
		if req.method != "backupWorld" && req.method != "restoreWorld" && req.method != "deleteWorldBackup" {
			continue
		}
		if req.fields["rootId"] == r.ID && req.fields["version"] == r.SelectedVersion && (world == "" || req.fields["name"] == "" || req.fields["name"] == world) {
			return true
		}
	}
	return false
}

// watchWorldJobs reloads the list when a backup or restore finishes.
func (a *App) watchWorldJobs() {
	if a.wv.seen == nil {
		a.wv.seen = map[string]string{}
	}
	changed := false
	for _, j := range a.snapshot.Jobs {
		if !a.currentWorldJob(&j) {
			continue
		}
		if old := a.wv.seen[j.ID]; old != j.State {
			if j.State != "running" { // fast jobs may finish before the first poll
				changed = true
			}
			a.wv.seen[j.ID] = j.State
		}
	}
	if changed {
		a.loadWorlds()
	}
}

var worldModes = map[string][2]string{
	"survival":  {"生存", "Survival"},
	"creative":  {"创造", "Creative"},
	"adventure": {"冒险", "Adventure"},
	"spectator": {"旁观", "Spectator"},
	"hardcore":  {"极限", "Hardcore"},
}

func (a *App) worldsView(c *ui.Context, r launcher.Instance) {
	s := uiScale(c)
	if a.wv.key != r.ID+"|"+r.SelectedVersion {
		a.loadWorlds()
	}
	a.watchWorldJobs()
	running := verRunning(&r)
	var total int64
	for _, w := range a.wv.list {
		total += w.Size
	}
	// header: count + total size, then 全部备份 / 打开存档文件夹
	ui.Row(c.Key("worlds-head")).FillWidth().Height(36 * s).Gap(10 * s).AlignItems(ui.Center).Children(func() {
		ui.Text(c, fmt.Sprintf(a.L("%d 个世界", "%d worlds"), len(a.wv.list))).FontSize(typeHeadline * s).FontWeight(700).TextColor(ink).NoWrap()
		if total > 0 {
			numText(c, a.L("共 ", "")+fmtSize(total), typeFootnote*s, muted)
		}
		ui.Spacer(c).Grow(1)
		allJob := a.worldJob("")
		if allJob != nil {
			numText(c, fmt.Sprintf("%d / %d", allJob.Done, allJob.Total), typeFootnote*s, accent)
		}
		if fixedButton(c, "worlds-backup-all", a.L("全部备份", "Back up all"), "download", 0, btnSm*s, false, len(a.wv.list) == 0 || a.worldBusy("")) {
			a.execute("backupWorld", map[string]string{"rootId": r.ID, "version": r.SelectedVersion, "name": ""}, nil)
		}
		if fixedButton(c, "worlds-open", a.L("打开存档文件夹", "Open saves folder"), "folder", 0, btnSm*s, false, false) {
			a.openSub("saves")
		}
	})
	if running {
		ui.Row(c.Key("worlds-running")).FillWidth().Height(36 * s).PaddingX(12 * s).Gap(8 * s).Radius(10 * s).Background(warning.Alpha(.10)).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons["info"]).Size(14*s, 14*s).TextColor(warning).Shrink(0)
			label(c, a.L("游戏正在运行：备份可能不是最新状态，还原需要先退出游戏", "Game running: backups may lag, restore needs the game closed")).FontSize(typeFootnote * s).SingleLine().Grow(1).Basis(0).MinWidth(0)
		})
	}
	if len(a.wv.list) == 0 {
		if a.wv.loading {
			label(c, a.L("正在读取存档", "Reading worlds"))
			return
		}
		emptyState(c, "worlds-empty", "world", a.L("还没有存档", "No worlds yet"), a.L("进入游戏创建世界后会显示在这里", "Worlds you create in game show up here"), nil)
		return
	}
	ui.Column(c.Key("worlds-list")).FillWidth().Gap(10 * s).Children(func() {
		for _, w := range a.wv.list {
			a.worldCard(c, r, w, running)
		}
	})
}

func (a *App) worldCard(c *ui.Context, r launcher.Instance, w launcher.World, running bool) {
	s := uiScale(c)
	open := a.wv.open == w.Dir
	job := a.worldJob(w.Dir)
	busy := a.worldBusy(w.Dir)
	ui.Column(c.Key("world-"+w.Dir)).Label("存档 "+w.Name).FillWidth().Radius(radiusCard*s).Background(ui.RGBA(255, 255, 255, .76)).Border(1, glassEdge).Children(func() {
		ui.Row(c).FillWidth().Height(76 * s).PaddingX(16 * s).Gap(14 * s).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(48*s, 48*s).Radius(8 * s).Background(fieldColor).Center().Clip().Shrink(0).Children(func() {
				var img ui.ImageSource
				if w.HasIcon {
					id, ver, dir := r.ID, r.SelectedVersion, w.Dir
					img = a.localIcon("world:"+id+":"+ver+":"+dir, func() ([]byte, error) { return a.service.WorldIcon(id, ver, dir) })
				}
				if img != nil {
					ui.Image(c, img).Size(48*s, 48*s).Fit(ui.Cover)
				} else {
					ui.Icon(c, icons["cube"]).Size(22*s, 22*s).TextColor(muted)
				}
			})
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4 * s).Children(func() {
				ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
					tooltip(c, ui.Text(c, w.Name).FontSize(typeSubhead*s).Bold().TextColor(ink).SingleLine().Shrink(1).MinWidth(0), w.Name)
					m := worldModes[w.Mode]
					tone := muted
					if w.Mode == "hardcore" {
						tone = danger
					}
					quietTag(c, a.L(m[0], m[1]), tone)
				})
				ui.Row(c).FillWidth().Gap(16 * s).AlignItems(ui.Center).Children(func() {
					cell := func(text string, wd float32) {
						ui.Box(c).Width(wd * s).Shrink(0).Children(func() { numText(c, text, typeFootnote*s, muted) })
					}
					cell(a.L("上次游玩 ", "Played ")+agoText(a, w.LastPlayed), 150)
					cell(fmtSize(w.Size), 72)
					if w.Backups > 0 {
						cell(fmt.Sprintf(a.L("%d 个备份", "%d backups"), w.Backups), 80)
					} else {
						cell(a.L("未备份", "No backup"), 80)
					}
				})
			})
			if job != nil {
				msg := job.Message
				if job.BytesTotal > 0 {
					msg = fmt.Sprintf("%d%%", job.BytesDone*100/max(job.BytesTotal, 1))
				}
				ui.Box(c).Width(90 * s).Shrink(0).AlignItems(ui.End).Children(func() { numText(c, msg, typeFootnote*s, accent) })
			}
			if iconOnlyButton(c, "world-open-"+w.Dir, "folder", a.L("打开存档文件夹", "Open folder"), btnSm*s, false) {
				a.openWorldDir(w.Dir)
			}
			if fixedButton(c, "world-backup-"+w.Dir, a.L("备份", "Back up"), "download", 0, btnSm*s, false, busy) {
				a.execute("backupWorld", map[string]string{"rootId": r.ID, "version": r.SelectedVersion, "name": w.Dir}, nil)
			}
			rl := a.L("还原", "Restore")
			if fixedButton(c, "world-restore-"+w.Dir, rl, "refresh", 0, btnSm*s, open, busy) {
				a.wv.confirm = ""
				if open {
					a.wv.open, a.wv.backups = "", nil
				} else {
					a.wv.open, a.wv.backups = w.Dir, nil
					a.loadWorldBackups(w.Dir)
				}
			}
		})
		if job != nil && job.BytesTotal > 0 {
			ui.Box(c).FillWidth().PaddingX(16*s).Padding(0, 16*s, 10*s, 16*s).Children(func() {
				ui.Progress(c, float64(job.BytesDone)/float64(job.BytesTotal))
			})
		}
		if open {
			a.worldBackupList(c, r, w, running)
		}
	})
}

func (a *App) openWorldDir(dir string) {
	r := a.root()
	if r == nil {
		return
	}
	p, e := a.service.InstanceFolder(r.ID, r.SelectedVersion, "saves")
	if e == nil {
		e = openPath(filepath.Join(p, dir))
	}
	if e != nil {
		a.message, a.errorMessage = a.friendlyErr(e), true
	}
}

func (a *App) worldBackupList(c *ui.Context, r launcher.Instance, w launcher.World, running bool) {
	s := uiScale(c)
	busy := a.worldBusy(w.Dir)
	ui.Column(c.Key("world-backups-"+w.Dir)).FillWidth().Padding(0, 16*s, 14*s, 16*s).Gap(6 * s).Children(func() {
		ui.Box(c).FillWidth().Height(1).Background(line).Margin(0, 0, 4*s, 0)
		if running {
			label(c, a.L("游戏正在运行，退出游戏后才能还原", "Close the game to restore")).FontSize(typeFootnote * s).TextColor(danger)
		}
		if len(a.wv.backups) == 0 {
			if a.wv.backupsLoading {
				ui.Row(c).FillWidth().Height(36 * s).AlignItems(ui.Center).Children(func() {
					label(c, a.L("正在读取备份", "Reading backups")).FontSize(typeFootnote * s)
				})
			} else {
				emptyNote(c, "world-backups-empty", "folder", a.L("还没有备份", "No backups yet"), a.L("点上方的「备份」，保存当前世界的副本", "Choose Back up above to save a copy of this world"))
			}
			return
		}
		for _, b := range a.wv.backups {
			b := b
			ui.Row(c.Key("wb-" + b.File)).FillWidth().Height(40 * s).PaddingX(10 * s).Gap(12 * s).Radius(8 * s).Background(raised).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons["doc"]).Size(13*s, 13*s).TextColor(muted).Shrink(0)
				ui.Box(c).Width(150 * s).Shrink(0).Children(func() {
					numText(c, b.Time.Local().Format("2006-01-02 15:04:05"), typeFootnote*s, ink)
				})
				ui.Box(c).Width(70 * s).Shrink(0).Children(func() { numText(c, fmtSize(b.Size), typeFootnote*s, muted) })
				if b.Auto {
					quietTag(c, a.L("还原前自动备份", "Auto before restore"), muted)
				}
				ui.Spacer(c).Grow(1)
				switch a.wv.confirm {
				case "restore:" + b.File:
					label(c, a.L("会先自动备份当前存档再替换", "Current world is backed up first")).FontSize(typeFootnote * s).SingleLine().Shrink(1).MinWidth(0)
					if fixedButton(c, "wb-cancel-"+b.File, a.L("取消", "Cancel"), "", 0, btnSm*s, false, false) {
						a.wv.confirm = ""
					}
					if fixedButton(c, "wb-do-restore-"+b.File, a.L("确认还原", "Restore"), "refresh", 0, btnSm*s, true, running || busy) {
						a.wv.confirm = ""
						a.execute("restoreWorld", map[string]string{"rootId": r.ID, "version": r.SelectedVersion, "name": w.Dir, "path": b.File}, nil)
					}
				case "delete:" + b.File:
					label(c, a.L("删除后无法恢复", "This can't be undone")).FontSize(typeFootnote * s).SingleLine().Shrink(1).MinWidth(0)
					if fixedButton(c, "wb-cancel-"+b.File, a.L("取消", "Cancel"), "", 0, btnSm*s, false, false) {
						a.wv.confirm = ""
					}
					if fixedButton(c, "wb-do-delete-"+b.File, a.L("确认删除", "Delete"), "trash", 0, btnSm*s, false, busy) {
						a.wv.confirm = ""
						world := w.Dir
						a.execute("deleteWorldBackup", map[string]string{"rootId": r.ID, "version": r.SelectedVersion, "name": w.Dir, "path": b.File}, func(json.RawMessage) {
							a.loadWorldBackups(world)
							a.loadWorlds()
						})
					}
				default:
					if fixedButton(c, "wb-restore-"+b.File, a.L("还原到此", "Restore"), "refresh", 0, btnSm*s, false, running || busy) {
						a.wv.confirm = "restore:" + b.File
					}
					if iconOnlyButton(c, "wb-delete-"+b.File, "trash", a.L("删除备份", "Delete backup"), btnSm*s, busy) {
						a.wv.confirm = "delete:" + b.File
					}
				}
			})
		}
	})
}

var _ = time.Now
