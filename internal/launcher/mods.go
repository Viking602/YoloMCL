package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type modRecord struct {
	SHA1      string `json:"sha1"`
	Source    string `json:"source"`
	ProjectID string `json:"projectId"`
	VersionID string `json:"versionId"`
}

func readRecords(root string) map[string]modRecord {
	out := map[string]modRecord{}
	path, e := safeJoin(root, ".mcl/mods.json")
	if e != nil {
		return out
	}
	b, e := os.ReadFile(path)
	if e == nil {
		_ = json.Unmarshal(b, &out)
	}
	return out
}

// The on-disk check also contains jar metadata. Scanning already reads that
// from modcache.json, so decode only the fields that a check can override.
type modCheckResult struct {
	Source    string          `json:"source"`
	ProjectID string          `json:"projectId"`
	VersionID string          `json:"versionId"`
	Latest    *ProjectVersion `json:"latest,omitempty"`
	Status    string          `json:"status"`
}

func readChecks(root string) map[string]modCheckResult {
	out := map[string]modCheckResult{}
	path, e := safeJoin(root, ".mcl/mod-checks.json")
	if e != nil {
		return out
	}
	b, e := os.ReadFile(path)
	if e == nil {
		_ = json.Unmarshal(b, &out)
	}
	return out
}

func (s *Service) ListMods(rootID string) ([]Mod, error) { return s.ListModsAt(rootID, "") }

// rootAt is the root as if versionID were selected ("" = as saved), so
// per-version folders resolve right even before a selection is saved.
func (s *Service) rootAt(rootID, versionID string) (Root, error) {
	r, e := s.getRoot(rootID)
	if e == nil && versionID != "" && versionID != r.SelectedVersion {
		r.SelectedVersion = versionID
		s.cacheGameDir(&r)
	}
	return r, e
}

// ListModsAt lists the mods of versionID's game folder.
func (s *Service) ListModsAt(rootID, versionID string) ([]Mod, error) {
	r, e := s.rootAt(rootID, versionID)
	if e != nil {
		return nil, e
	}
	return s.scanModDirectory(context.Background(), r.VersionDir(), nil)
}

// ListModsStream is ListModsAt that also reports partial lists while new
// or changed jars are read; partial rows keep their final order.
func (s *Service) ListModsStream(rootID, versionID string, partial func([]Mod)) ([]Mod, error) {
	return s.ListModsStreamContext(context.Background(), rootID, versionID, partial)
}

// ListModsStreamContext stops obsolete scans when the user selects another pack.
func (s *Service) ListModsStreamContext(ctx context.Context, rootID, versionID string, partial func([]Mod)) ([]Mod, error) {
	r, e := s.rootAt(rootID, versionID)
	if e != nil {
		return nil, e
	}
	return s.scanModDirectory(ctx, r.VersionDir(), partial)
}

func readDirRetryContext(ctx context.Context, dir string) ([]os.DirEntry, error) {
	var e error
	for i := 0; i < 4; i++ {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		var out []os.DirEntry
		if out, e = os.ReadDir(dir); e == nil || os.IsNotExist(e) {
			return out, e
		}
		timer := time.NewTimer(time.Duration(100*(i+1)) * time.Millisecond)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		}
	}
	return nil, e
}

func scanMods(root string) ([]Mod, error) { return scanModsStream(root, nil) }

func (s *Service) mutateRoot(id string, fn func(Root) error) error {
	r, e := s.getRoot(id)
	if e != nil {
		return e
	}
	s.mu.Lock()
	if s.rootLocks[id] || s.rootRunningLocked(id) {
		s.mu.Unlock()
		return errors.New("请先结束此目录的任务或游戏")
	}
	s.rootLocks[id] = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.rootLocks, id); s.mu.Unlock(); s.invalidateInventory(id) }()
	return fn(r)
}

func (s *Service) ToggleMod(rootID, name string) error {
	return s.mutateRoot(rootID, func(r Root) error { return s.toggleModIn(r, name) })
}

func (s *Service) toggleModIn(r Root, name string) error {
	{
		if filepath.Base(name) != name || (!strings.HasSuffix(name, ".jar") && !strings.HasSuffix(name, ".jar.disabled")) {
			return errors.New("无效的模组文件名")
		}
		target := name + ".disabled"
		if strings.HasSuffix(name, ".disabled") {
			target = strings.TrimSuffix(name, ".disabled")
		}
		vDir := r.VersionDir()
		src, e := safeJoin(vDir, "mods/"+name)
		if e != nil {
			return e
		}
		dst, e := safeJoin(vDir, "mods/"+target)
		if e != nil {
			return e
		}
		if _, e = os.Stat(dst); !os.IsNotExist(e) {
			return errors.New("目标文件已经存在")
		}
		return os.Rename(src, dst)
	}
}

func (s *Service) RemoveMod(rootID, name string) error {
	return s.mutateRoot(rootID, func(r Root) error {
		if filepath.Base(name) != name || (!strings.HasSuffix(name, ".jar") && !strings.HasSuffix(name, ".jar.disabled")) {
			return errors.New("无效的模组文件名")
		}
		vDir := r.VersionDir()
		src, e := safeJoin(vDir, "mods/"+name)
		if e != nil {
			return e
		}
		dst, e := safeJoin(r.Path, ".mcl/backups/"+time.Now().Format("20060102-150405")+"-"+newID()[:6]+"/"+name)
		if e != nil {
			return e
		}
		if e = os.MkdirAll(filepath.Dir(dst), 0755); e != nil {
			return e
		}
		return os.Rename(src, dst)
	})
}

func (s *Service) AddLocalMods(rootID string, paths []string) error {
	return s.mutateRoot(rootID, func(r Root) error {
		for _, src := range paths {
			if !strings.EqualFold(filepath.Ext(src), ".jar") {
				return errors.New("只能导入 .jar 模组")
			}
			dst, e := safeJoin(r.VersionDir(), "mods/"+filepath.Base(src))
			if e != nil {
				return e
			}
			if e = copyFile(src, dst); e != nil {
				return e
			}
		}
		return nil
	})
}

const modCheckWorkers = 6
