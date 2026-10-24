package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// applyFaultHook lets tests inject a failure after n files were applied.
var applyFaultHook func(n int) error

// ApplyPackUpdate runs a staged plan as a background job.
func (s *Service) ApplyPackUpdate(planID string) (string, error) {
	packPlans.Lock()
	st := packPlans.m[planID]
	packPlans.Unlock()
	if st == nil {
		return "", errors.New("更新计划已失效，请重新检查更新")
	}
	r, e := s.getRoot(st.plan.RootID)
	if e != nil {
		return "", e
	}
	jobID, e := s.startJob("更新整合包："+st.plan.Name, r.ID, func(ctx context.Context, p progress) error {
		// On failure the staged files and the plan stay for an in-place
		// 重试; success, cancel or 移除 clears them.
		dropPlan := func() {
			packPlans.Lock()
			delete(packPlans.m, planID)
			packPlans.Unlock()
			_ = os.RemoveAll(filepath.Join(r.Path, ".mcl", "update-staging"))
		}
		s.onUndo(ctx, dropPlan)
		ok := false
		defer func() {
			if ok || ctx.Err() != nil {
				dropPlan()
			}
		}()
		old := readPack(r.Path)
		if old == nil || old.Version != st.plan.OldVersion {
			return errors.New("整合包已发生变化，请重新检查更新")
		}
		setJobBytesTotal(ctx, st.plan.DownloadBytes)
		// 1. Download and verify everything into staging first.
		writes := []stagedFile{}
		for _, f := range st.files {
			if f.write {
				writes = append(writes, f)
			}
		}
		for i := range writes {
			f := &writes[i]
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if f.staged == "" {
				dst, e := safeJoin(filepath.Join(st.stage, "files"), f.Path)
				if e != nil {
					return e
				}
				p(fmt.Sprintf("下载新版本文件 (%d/%d): %s", i+1, len(writes), filepath.Base(f.Path)), i+1, len(writes))
				last := errors.New("整合包文件缺少下载地址")
				for _, raw := range f.URLs {
					if last = s.download(ctx, DownloadFile{Name: f.Path, URL: raw, SHA1: f.SHA1, SHA512: f.SHA512}, dst); last == nil {
						break
					}
				}
				if last != nil {
					return last
				}
				f.staged = dst
			}
			if !checkHashes(f.staged, f.SHA1, f.SHA512) {
				return fmt.Errorf("文件校验失败: %s", f.Path)
			}
		}
		// 2. Game / loader change: install side by side, old version stays.
		newGame := old.GameVersion
		if newGame == "" {
			newGame = r.SelectedVersion
		}
		if st.plan.GameChanged || old.GameVersion == "" {
			if _, e := s.installVanilla(ctx, r.Path, st.newLock.Minecraft, p); e != nil {
				return e
			}
			id, e := s.installLoader(ctx, r.Path, st.newLock.Minecraft, st.newLock.Loader, st.newLock.LoaderVersion, p)
			if e != nil {
				return e
			}
			gv, e := s.resolveVersion(ctx, r.Path, id, p)
			if e != nil {
				return e
			}
			if _, _, e = s.prepareVersion(ctx, r.Path, gv, p); e != nil {
				return e
			}
			newGame = id
		}
		// 3. Backup.
		p("备份将被修改的文件", 0, 0)
		bid := time.Now().Format("20060102-150405") + "-" + planID[:6]
		bdir := filepath.Join(backupsDir(r.Path), bid)
		m := backupManifest{ID: bid, At: nowStamp(), FromVersion: old.Version, ToVersion: st.newLock.Version, SelectedVersion: r.SelectedVersion, Plan: st.plan}
		bases := append([]string{}, st.removed...)
		for _, f := range writes {
			bases = append(bases, f.Path)
		}
		m.Paths = bases
		for _, rel := range bases {
			src, _ := safeJoin(r.Path, rel)
			for _, v := range []string{rel, rel + ".disabled"} {
				full := src + strings.TrimPrefix(v, rel)
				if fi, e := os.Stat(full); e == nil && !fi.IsDir() {
					dst, _ := safeJoin(filepath.Join(bdir, "files"), v)
					if e := copyFile(full, dst); e != nil {
						_ = os.RemoveAll(bdir)
						return fmt.Errorf("备份失败: %w", e)
					}
					m.Saved = append(m.Saved, v)
				}
			}
		}
		if e := copyFile(filepath.Join(r.Path, ".mcl", "pack-lock.json"), filepath.Join(bdir, "pack-lock.json")); e != nil {
			_ = os.RemoveAll(bdir)
			return fmt.Errorf("备份失败: %w", e)
		}
		if _, e := os.Stat(filepath.Join(r.Path, ".mcl", "mods.json")); e == nil {
			m.HadModsJSON = true
			if e := copyFile(filepath.Join(r.Path, ".mcl", "mods.json"), filepath.Join(bdir, "mods.json")); e != nil {
				_ = os.RemoveAll(bdir)
				return fmt.Errorf("备份失败: %w", e)
			}
		}
		if e := atomicJSON(filepath.Join(bdir, "backup.json"), m); e != nil {
			_ = os.RemoveAll(bdir)
			return e
		}
		// 4. Apply; restore automatically on any failure.
		apply := func() error {
			for _, rel := range st.removed {
				full, _ := safeJoin(r.Path, rel)
				for _, v := range []string{full, full + ".disabled"} {
					if e := os.Remove(v); e != nil && !os.IsNotExist(e) {
						return e
					}
				}
			}
			for i, f := range writes {
				if ctx.Err() != nil {
					return ctx.Err() // cancelled while applying: restored below
				}
				if applyFaultHook != nil {
					if e := applyFaultHook(i); e != nil {
						return e
					}
				}
				p(fmt.Sprintf("应用更新 (%d/%d): %s", i+1, len(writes), f.Path), i+1, len(writes))
				full, _ := safeJoin(r.Path, f.Path)
				target := full
				if f.disable {
					target = full + ".disabled"
				}
				_ = os.Remove(full)
				_ = os.Remove(full + ".disabled")
				if e := os.MkdirAll(filepath.Dir(target), 0755); e != nil {
					return e
				}
				if e := replaceFile(f.staged, target); e != nil {
					return e
				}
			}
			nl := st.newLock
			nl.GameVersion = newGame
			nl.UpdatedAt = nowStamp()
			nl.History = append(nl.History, PackHistory{Version: nl.Version, At: nl.UpdatedAt, Action: "update"})
			if len(nl.History) > 30 {
				nl.History = nl.History[len(nl.History)-30:]
			}
			if nl.Source == "curseforge" {
				records := map[string]modRecord{}
				b, _ := os.ReadFile(filepath.Join(r.Path, ".mcl", "mods.json"))
				_ = json.Unmarshal(b, &records)
				for _, f := range old.Files {
					delete(records, strings.TrimPrefix(f.Path, "mods/"))
				}
				for _, f := range nl.Files {
					if f.Source == "curseforge" && strings.HasPrefix(f.Path, "mods/") {
						records[strings.TrimPrefix(f.Path, "mods/")] = modRecord{SHA1: f.SHA1, Source: f.Source, ProjectID: f.ProjectID, VersionID: f.VersionID}
					}
				}
				if e := atomicJSON(filepath.Join(r.Path, ".mcl", "mods.json"), records); e != nil {
					return e
				}
			}
			if e := atomicJSON(filepath.Join(r.Path, ".mcl", "pack-lock.json"), nl); e != nil {
				return e
			}
			m.Done = true
			if e := atomicJSON(filepath.Join(bdir, "backup.json"), m); e != nil {
				return e
			}
			if newGame != "" && newGame != r.SelectedVersion {
				return s.setRootVersion(r.ID, newGame)
			}
			return nil
		}
		if e := apply(); e != nil {
			p("更新失败，正在恢复原状", 0, 0)
			if re := restoreBackup(r.Path, bdir, m); re != nil {
				return fmt.Errorf("更新失败（%w），且恢复时出错：%w。备份保留在 %s", e, re, bdir)
			}
			_ = os.RemoveAll(bdir)
			return fmt.Errorf("更新失败，已恢复原状：%w", e)
		}
		_ = os.Remove(filepath.Join(r.Path, ".mcl", "pack-update.json"))
		prunePackBackups(r.Path)
		s.setJobResultRoot(ctx, r.ID)
		s.setJobResultRef(ctx, bid)
		p(fmt.Sprintf("更新完成：%s → %s", old.Version, st.newLock.Version), 1, 1)
		ok = true
		return nil
	})
	return jobID, e
}
