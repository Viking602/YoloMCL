package launcher

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

// RootRemoval is an opaque receipt for undoing the exact root removal performed
// by this service, without accepting arbitrary paths or configurations.
type RootRemoval struct {
	root        Root
	index       int
	active      bool
	afterActive string
}

func (s *Service) RemoveRootUndo(id string) (RootRemoval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rootLocks[id] || s.rootRunningLocked(id) {
		return RootRemoval{}, errors.New("此目录还有运行中的任务")
	}
	draft := s.cfg.clone()
	for i, r := range draft.Roots {
		if r.ID != id {
			continue
		}
		receipt := RootRemoval{root: r.clone(), index: i, active: draft.ActiveRoot == id}
		draft.Roots = append(draft.Roots[:i], draft.Roots[i+1:]...)
		if receipt.active {
			draft.ActiveRoot = ""
			if len(draft.Roots) > 0 {
				draft.ActiveRoot = draft.Roots[0].ID
			}
		}
		receipt.afterActive = draft.ActiveRoot
		if err := s.saveConfigLocked(draft); err != nil {
			return RootRemoval{}, err
		}
		return receipt, nil
	}
	return RootRemoval{}, errors.New("目录不存在")
}

func (s *Service) RestoreRoot(receipt RootRemoval) error {
	if receipt.root.ID == "" || !isDir(receipt.root.Path) {
		return errors.New("原游戏目录不存在，无法撤销")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	draft := s.cfg.clone()
	for _, r := range draft.Roots {
		if r.ID == receipt.root.ID {
			return errors.New("此目录已重新添加，现有设置未覆盖")
		}
	}
	i := min(receipt.index, len(draft.Roots))
	draft.Roots = append(draft.Roots, Root{})
	copy(draft.Roots[i+1:], draft.Roots[i:])
	draft.Roots[i] = receipt.root.clone()
	if receipt.active && draft.ActiveRoot == receipt.afterActive {
		draft.ActiveRoot = receipt.root.ID
	}
	return s.saveConfigLocked(draft)
}

type ModRemoval struct {
	rootID, source, backup string
	horizon                bool
	modID                  string
	hadHorizonState        bool
	horizonEnabled         bool
	recordName             string
	record                 *modRecord
}

func (s *Service) RemoveModUndo(rootID, version, name string) (ModRemoval, error) {
	var receipt ModRemoval
	err := s.mutateRootAllowRunning(rootID, func(r Root) error {
		if version != "" {
			if filepath.Base(version) != version || version == "." || version == ".." {
				return errors.New("无效的版本")
			}
			r.SelectedVersion = version
			s.cacheGameDir(&r)
		}
		if isHorizonRootVersion(r) {
			var err error
			receipt, err = s.removeHorizonModReceipt(r, name)
			return err
		}
		if !isJavaModFile(name) {
			return errors.New("无效的模组文件名")
		}
		var err error
		receipt, err = moveAddonToBackup(r, "mods", name)
		return err
	})
	return receipt, err
}

func (s *Service) RemoveAssetPackUndo(rootID, version, kind, name string) (ModRemoval, error) {
	folder, err := packFolder(kind)
	if err != nil {
		return ModRemoval{}, err
	}
	var receipt ModRemoval
	err = s.mutateRootAllowRunning(rootID, func(r Root) error {
		if version != "" {
			if filepath.Base(version) != version || version == "." || version == ".." {
				return errors.New("无效的版本")
			}
			r.SelectedVersion = version
			s.cacheGameDir(&r)
		}
		if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
			return errors.New("无效的资源包文件名")
		}
		src, err := safeJoin(r.VersionDir(), folder+"/"+name)
		if err != nil {
			return err
		}
		if !isValidPackRemovalTarget(kind, name, src) {
			return errors.New("无效的资源包文件名")
		}
		receipt, err = moveAddonToBackup(r, folder, name)
		return err
	})
	return receipt, err
}

func moveAddonToBackup(r Root, folder, name string) (ModRemoval, error) {
	src, err := safeJoin(r.VersionDir(), folder+"/"+name)
	if err != nil {
		return ModRemoval{}, err
	}
	dst, err := safeJoin(r.Path, ".mcl/backups/"+time.Now().Format("20060102-150405")+"-"+newID()[:6]+"/"+name)
	if err != nil {
		return ModRemoval{}, err
	}
	if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return ModRemoval{}, err
	}
	if err = os.Rename(src, dst); err != nil {
		return ModRemoval{}, err
	}
	return ModRemoval{rootID: r.ID, source: src, backup: dst}, nil
}

func (s *Service) RestoreAuthServer(server AuthServer) error {
	if server.Builtin {
		return errors.New("内置服务器不可修改")
	}
	u, err := NormalizeAuthServerURL(server.URL)
	if err != nil {
		return err
	}
	server.URL = u
	s.mu.Lock()
	defer s.mu.Unlock()
	draft := s.cfg.clone()
	for _, current := range draft.AuthServers {
		if SameAuthServer(current.URL, server.URL) {
			return errors.New("服务器已重新添加，现有配置未覆盖")
		}
	}
	draft.AuthServers = append(draft.AuthServers, server)
	return s.saveConfigLocked(draft)
}

func (s *Service) RestoreMod(receipt ModRemoval) error {
	return s.mutateRootAllowRunning(receipt.rootID, func(r Root) error {
		// Both locations came from safeJoin at removal time. Recheck against
		// the current root before writing, including the symlink boundary.
		for _, path := range []string{receipt.source, receipt.backup} {
			rel, err := filepath.Rel(r.Path, path)
			if err != nil {
				return err
			}
			if _, err = safeJoin(r.Path, rel); err != nil {
				return err
			}
		}
		if _, err := os.Stat(receipt.source); err == nil {
			return errors.New("原位置已有同名模组，现有文件未覆盖")
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(receipt.backup, receipt.source); err != nil {
			return err
		}
		if receipt.horizon {
			vDir := filepath.Dir(filepath.Dir(receipt.source))
			if receipt.hadHorizonState && receipt.modID != "" {
				_ = writeHorizonModState(vDir, receipt.modID, receipt.horizonEnabled, false)
			}
			if receipt.record != nil && receipt.recordName != "" {
				records := readRecords(vDir)
				records[receipt.recordName] = *receipt.record
				_ = atomicJSON(filepath.Join(vDir, ".mcl", "mods.json"), records)
			}
			s.invalidateStatus(vDir)
		}
		return nil
	})
}
