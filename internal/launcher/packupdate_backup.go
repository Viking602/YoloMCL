package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const packBackupKeep = 3

type backupManifest struct {
	ID              string         `json:"id"`
	At              string         `json:"at"`
	FromVersion     string         `json:"fromVersion"`
	ToVersion       string         `json:"toVersion"`
	SelectedVersion string         `json:"selectedVersion"`
	Paths           []string       `json:"paths"` // every base path the update touches
	Saved           []string       `json:"saved"` // files (incl. .disabled variants) copied into files/
	HadModsJSON     bool           `json:"hadModsJson"`
	Done            bool           `json:"done"`
	Plan            PackUpdatePlan `json:"plan"`
}

func backupsDir(root string) string { return filepath.Join(root, ".mcl", "backups") }

func listPackBackups(root string) []PackBackup {
	dir, e := safeJoin(root, ".mcl/backups")
	if e != nil {
		return nil
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return nil
	}
	out := []PackBackup{}
	for _, en := range entries {
		var m backupManifest
		if !en.IsDir() || en.Type()&os.ModeSymlink != 0 {
			continue
		}
		path, e := safeJoin(root, ".mcl/backups/"+en.Name()+"/backup.json")
		if e != nil {
			continue
		}
		b, e := os.ReadFile(path)
		if e != nil || json.Unmarshal(b, &m) != nil || !m.Done {
			continue
		}
		out = append(out, PackBackup{ID: m.ID, At: m.At, FromVersion: m.FromVersion, ToVersion: m.ToVersion, Done: m.Done})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func readBackup(root, id string) (backupManifest, string, error) {
	var m backupManifest
	if id == "" || filepath.Base(id) != id {
		return m, "", errors.New("备份不存在")
	}
	dir := filepath.Join(backupsDir(root), id)
	b, e := os.ReadFile(filepath.Join(dir, "backup.json"))
	if e != nil || json.Unmarshal(b, &m) != nil {
		return m, "", errors.New("备份不存在或已损坏")
	}
	return m, dir, nil
}

// PackUpdateReport returns the change summary stored with a backup.
func (s *Service) PackUpdateReport(rootID, backupID string) (PackUpdatePlan, error) {
	r, e := s.getRoot(rootID)
	if e != nil {
		return PackUpdatePlan{}, e
	}
	m, _, e := readBackup(r.Path, backupID)
	return m.Plan, e
}

func replaceFile(src, dst string) error {
	tmp := dst + ".mcl-tmp"
	_ = os.Remove(tmp)
	if e := copyFile(src, tmp); e != nil {
		return e
	}
	return os.Rename(tmp, dst)
}

// restoreBackup puts every touched path back to its pre-update state.
func restoreBackup(root, dir string, m backupManifest) error {
	var first error
	keep := func(e error) {
		if e != nil && first == nil {
			first = e
		}
	}
	for _, rel := range m.Paths {
		p, e := safeJoin(root, rel)
		if e != nil {
			keep(e)
			continue
		}
		for _, v := range []string{p, p + ".disabled"} {
			if e := os.Remove(v); e != nil && !os.IsNotExist(e) {
				keep(e)
			}
		}
	}
	for _, rel := range m.Saved {
		p, e := safeJoin(root, rel)
		if e != nil {
			keep(e)
			continue
		}
		src, _ := safeJoin(filepath.Join(dir, "files"), rel)
		if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			keep(e)
			continue
		}
		keep(replaceFile(src, p))
	}
	lockPath := filepath.Join(root, ".mcl", "pack-lock.json")
	keep(replaceFile(filepath.Join(dir, "pack-lock.json"), lockPath))
	modsPath := filepath.Join(root, ".mcl", "mods.json")
	if m.HadModsJSON {
		keep(replaceFile(filepath.Join(dir, "mods.json"), modsPath))
	} else {
		_ = os.Remove(modsPath)
	}
	return first
}

func prunePackBackups(root string) {
	entries, e := os.ReadDir(backupsDir(root))
	if e != nil {
		return
	}
	names := []string{}
	for _, en := range entries {
		if en.IsDir() {
			names = append(names, en.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for i, n := range names {
		if i >= packBackupKeep {
			_ = os.RemoveAll(filepath.Join(backupsDir(root), n))
		}
	}
}

// RollbackPack restores the state saved before an update.
func (s *Service) RollbackPack(rootID, backupID string) (string, error) {
	r, e := s.getRoot(rootID)
	if e != nil {
		return "", e
	}
	if backupID == "" {
		if b := listPackBackups(r.Path); len(b) > 0 {
			backupID = b[0].ID
		}
	}
	m, dir, e := readBackup(r.Path, backupID)
	if e != nil {
		return "", e
	}
	if !m.Done {
		return "", errors.New("该备份不完整，无法回滚")
	}
	return s.startJob("回滚整合包："+m.Plan.Name, r.ID, func(ctx context.Context, p progress) error {
		p(fmt.Sprintf("正在回滚 %s → %s", m.ToVersion, m.FromVersion), 0, 1)
		if e := restoreBackup(r.Path, dir, m); e != nil {
			return fmt.Errorf("回滚未完全成功：%w", e)
		}
		if lock := readPack(r.Path); lock != nil {
			lock.History = append(lock.History, PackHistory{Version: lock.Version, At: nowStamp(), Action: "rollback"})
			if e := atomicJSON(filepath.Join(r.Path, ".mcl", "pack-lock.json"), lock); e != nil {
				return fmt.Errorf("回滚记录保存失败，备份已保留：%w", e)
			}
		}
		if m.SelectedVersion != "" {
			if e := s.setRootVersion(r.ID, m.SelectedVersion); e != nil {
				return fmt.Errorf("回滚版本选择保存失败，备份已保留：%w", e)
			}
		}
		_ = os.RemoveAll(dir)
		_ = os.Remove(filepath.Join(r.Path, ".mcl", "pack-update.json"))
		s.setJobResultRoot(ctx, r.ID)
		p(fmt.Sprintf("已回滚到 %s", m.FromVersion), 1, 1)
		return nil
	})
}
