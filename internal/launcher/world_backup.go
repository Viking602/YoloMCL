package launcher

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func zipWorld(ctx context.Context, src, dest string, onBytes func(int64)) error {
	if e := os.MkdirAll(filepath.Dir(dest), 0o755); e != nil {
		return e
	}
	tmp := dest + ".mcl-part"
	f, e := os.Create(tmp)
	if e != nil {
		return e
	}
	zw := zip.NewWriter(f)
	base := filepath.Base(src)
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() || d.Name() == "session.lock" {
			return nil
		}
		rel, _ := filepath.Rel(src, p)
		in, e := os.Open(p)
		if e != nil {
			return e
		}
		defer in.Close()
		fi, _ := d.Info()
		h, _ := zip.FileInfoHeader(fi)
		h.Name = filepath.ToSlash(filepath.Join(base, rel))
		h.Method = zip.Deflate
		w, e := zw.CreateHeader(h)
		if e != nil {
			return e
		}
		n, e := io.Copy(w, in)
		if onBytes != nil {
			onBytes(n)
		}
		return e
	})
	if e := zw.Close(); err == nil {
		err = e
	}
	if e := f.Close(); err == nil {
		err = e
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}

// unzipWorld extracts a backup made by zipWorld into dir (its top folder
// is stripped), refusing paths that escape.
func unzipWorld(ctx context.Context, archive, dir string) error {
	zr, e := zip.OpenReader(archive)
	if e != nil {
		return e
	}
	defer zr.Close()
	for _, f := range zr.File {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		name := filepath.ToSlash(f.Name)
		if i := strings.IndexByte(name, '/'); i >= 0 {
			name = name[i+1:]
		}
		if name == "" || strings.HasSuffix(name, "/") {
			continue
		}
		out := filepath.Join(dir, filepath.FromSlash(name))
		if rel, e := filepath.Rel(dir, out); e != nil || strings.HasPrefix(rel, "..") {
			return errors.New("备份中有不安全的路径")
		}
		if e := os.MkdirAll(filepath.Dir(out), 0o755); e != nil {
			return e
		}
		rc, e := f.Open()
		if e != nil {
			return e
		}
		w, e := os.Create(out)
		if e == nil {
			_, e = io.Copy(w, rc)
			if e2 := w.Close(); e == nil {
				e = e2
			}
		}
		rc.Close()
		if e != nil {
			return e
		}
	}
	return nil
}

func backupName(world string, t time.Time, auto bool) string {
	n := world + "-" + t.Format("20060102-150405")
	if auto {
		n += "-auto"
	}
	return n + ".zip"
}

// BackupWorld zips one world (or every world when world is "") into
// backups/<world>/. A world open in a running game is still backed up
// (session.lock skipped), and the job message says so.
func (s *Service) BackupWorld(rootID, versionID, world string) (string, error) {
	saves, backups, e := s.worldPaths(rootID, versionID)
	if e != nil {
		return "", e
	}
	var worlds []string
	if world == "" {
		ws, _ := s.ListWorlds(rootID, versionID)
		for _, w := range ws {
			worlds = append(worlds, w.Dir)
		}
		if len(worlds) == 0 {
			return "", errors.New("没有可备份的存档")
		}
	} else {
		if e := validWorldDir(world); e != nil {
			return "", e
		}
		worlds = []string{world}
	}
	running := s.instanceRunning(rootID, versionID)
	title := "备份存档 " + world
	if world == "" {
		title = "备份全部存档"
	}
	return s.startWorldJob(rootID, versionID, title, func(ctx context.Context, p progress) error {
		var total int64
		for _, w := range worlds {
			total += treeBytes(filepath.Join(saves, w))
		}
		setJobBytesTotal(ctx, total)
		now := time.Now()
		for i, w := range worlds {
			msg := fmt.Sprintf("正在备份 %s", w)
			if running {
				msg += "（游戏运行中，备份可能不是最新状态）"
			}
			p(msg, i, len(worlds))
			dest := filepath.Join(backups, w, backupName(w, now, false))
			s.trackCreated(ctx, dest)
			s.trackCreated(ctx, dest+".mcl-part")
			if e := zipWorld(ctx, filepath.Join(saves, w), dest, func(n int64) { addJobBytes(ctx, n) }); e != nil {
				return e
			}
		}
		p("备份完成", len(worlds), len(worlds))
		return nil
	})
}

// RestoreWorld replaces a world with a backup: blocked while the instance
// runs, auto-backs up the current world first, extracts to a temp folder
// and swaps it in so a failure never leaves a half world.
func (s *Service) RestoreWorld(rootID, versionID, world, file string) (string, error) {
	if e := validWorldDir(world); e != nil {
		return "", e
	}
	if file != filepath.Base(file) || !strings.HasSuffix(file, ".zip") {
		return "", errors.New("无效的备份")
	}
	saves, backups, e := s.worldPaths(rootID, versionID)
	if e != nil {
		return "", e
	}
	if s.instanceRunning(rootID, versionID) {
		return "", errors.New("游戏正在运行，请先退出游戏再还原存档")
	}
	archive := filepath.Join(backups, world, file)
	if _, e := os.Stat(archive); e != nil {
		return "", errors.New("备份文件不存在")
	}
	return s.startWorldJob(rootID, versionID, "还原存档 "+world, func(ctx context.Context, p progress) error {
		return s.restoreWorld(ctx, p, saves, backups, world, archive)
	})
}

func (s *Service) startWorldJob(rootID, versionID, title string, fn func(context.Context, progress) error) (string, error) {
	id, e := s.startJobForMode(title, "", "", true, fn)
	if e == nil {
		s.mu.Lock()
		s.jobs[id].TargetRootID, s.jobs[id].TargetVersion = rootID, versionID
		s.mu.Unlock()
	}
	return id, e
}

func (s *Service) restoreWorld(ctx context.Context, p progress, saves, backups, world, archive string) error {
	cur := filepath.Join(saves, world)
	if _, e := os.Stat(cur); e == nil {
		p("还原前自动备份", 0, 3)
		if e := zipWorld(ctx, cur, filepath.Join(backups, world, backupName(world, time.Now(), true)), nil); e != nil {
			return fmt.Errorf("还原前自动备份失败：%w", e)
		}
	}
	p("正在解压备份", 1, 3)
	tag := fmt.Sprint(time.Now().UnixNano())
	tmp := filepath.Join(saves, ".mcl-restore-"+tag)
	s.trackCreated(ctx, tmp)
	if e := unzipWorld(ctx, archive, tmp); e != nil {
		_ = os.RemoveAll(tmp)
		return e
	}
	if _, e := os.Stat(filepath.Join(tmp, "level.dat")); e != nil {
		_ = os.RemoveAll(tmp)
		return errors.New("备份里没有 level.dat，不是有效的存档")
	}
	p("正在替换存档", 2, 3)
	old := filepath.Join(saves, ".mcl-old-"+tag)
	hadOld := false
	if _, e := os.Stat(cur); e == nil {
		if e := os.Rename(cur, old); e != nil {
			_ = os.RemoveAll(tmp)
			return fmt.Errorf("存档被占用，无法替换：%w", e)
		}
		hadOld = true
	}
	if e := os.Rename(tmp, cur); e != nil {
		if hadOld {
			if rollbackErr := os.Rename(old, cur); rollbackErr != nil {
				return fmt.Errorf("替换存档失败（%w），恢复原存档失败（%w）；原存档保留在 %s", e, rollbackErr, old)
			}
		}
		_ = os.RemoveAll(tmp)
		return e
	}
	if hadOld {
		_ = os.RemoveAll(old)
	}
	p("还原完成", 3, 3)
	return nil
}

// DeleteWorldBackup removes one backup zip.
func (s *Service) DeleteWorldBackup(rootID, versionID, world, file string) error {
	if e := validWorldDir(world); e != nil {
		return e
	}
	if file != filepath.Base(file) || !strings.HasSuffix(file, ".zip") {
		return errors.New("无效的备份")
	}
	_, backups, e := s.worldPaths(rootID, versionID)
	if e != nil {
		return e
	}
	return os.Remove(filepath.Join(backups, world, file))
}
