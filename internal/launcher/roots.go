package launcher

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func isDir(p string) bool {
	st, e := os.Stat(p)
	return e == nil && st.IsDir()
}
func (s *Service) AddRoot(path, name string) (Root, error) {
	p, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return Root{}, err
	}
	selectedVer := ""
	if strings.EqualFold(filepath.Base(filepath.Dir(p)), "versions") && isDir(p) {
		selectedVer = filepath.Base(p)
		p = filepath.Dir(filepath.Dir(p))
	} else if strings.EqualFold(filepath.Base(p), "versions") && isDir(p) {
		p = filepath.Dir(p)
	}
	if !strings.EqualFold(filepath.Base(p), ".minecraft") {
		mcDir := filepath.Join(p, ".minecraft")
		hasDirectVersions := len(scanVersions(p)) > 0 || (isDir(filepath.Join(p, "versions")) && !isDir(mcDir))
		if !hasDirectVersions {
			p = mcDir
		}
	}
	p, err = filepath.EvalSymlinks(p)
	if err != nil {
		return Root{}, errors.New("此文件夹下找不到 .minecraft 或 versions 游戏目录，请选择现有游戏目录")
	}
	st, err := os.Stat(p)
	if err != nil || !st.IsDir() {
		return Root{}, errors.New("游戏目录必须是文件夹")
	}
	key := p
	if runtime.GOOS == "windows" {
		key = strings.ToLower(p)
	}
	hash := sha256.Sum256([]byte(key))
	id := hex.EncodeToString(hash[:])[:16]
	if strings.TrimSpace(name) == "" {
		if strings.EqualFold(filepath.Base(p), ".minecraft") {
			name = filepath.Base(filepath.Dir(p))
		} else {
			name = filepath.Base(p)
		}
	}
	r := Root{ID: id, Name: name, Path: p}
	versions := scanVersions(p)
	if selectedVer != "" {
		r.SelectedVersion = selectedVer
	} else if len(versions) > 0 {
		r.SelectedVersion = versions[0].ID
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	draft := s.cfg.clone()
	for _, existing := range draft.Roots {
		if existing.ID == id {
			return existing.clone(), nil
		}
	}
	draft.Roots = append(draft.Roots, r)
	draft.ActiveRoot = id
	return r, s.saveConfigLocked(draft)
}
func (s *Service) RemoveRoot(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	draft := s.cfg.clone()
	if s.rootLocks[id] || s.rootRunningLocked(id) {
		return errors.New("此目录还有运行中的任务")
	}
	for i, r := range draft.Roots {
		if r.ID == id {
			draft.Roots = append(draft.Roots[:i], draft.Roots[i+1:]...)
			if draft.ActiveRoot == id {
				draft.ActiveRoot = ""
				if len(draft.Roots) > 0 {
					draft.ActiveRoot = draft.Roots[0].ID
				}
			}
			return s.saveConfigLocked(draft)
		}
	}
	return errors.New("目录不存在")
}
func (s *Service) Select(rootID, accountID, version string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	draft := s.cfg.clone()
	if rootID != "" {
		found := false
		for i, r := range draft.Roots {
			if r.ID == rootID {
				found = true
				draft.ActiveRoot = rootID
				if version != "" {
					draft.Roots[i].SelectedVersion = version
				}
				break
			}
		}
		if !found {
			return errors.New("目录不存在")
		}
	}
	if accountID != "" {
		found := false
		for _, a := range draft.Accounts {
			if a.ID == accountID {
				found = true
				draft.ActiveAccount = accountID
				break
			}
		}
		if !found {
			return errors.New("账号不存在")
		}
	}
	return s.saveConfigLocked(draft)
}
func (r Root) VersionDir() string {
	if r.SelectedVersion != "" {
		if vDir, err := safeJoin(r.Path, "versions/"+r.SelectedVersion); err == nil && isDir(vDir) {
			for _, sub := range []string{"mods", "resourcepacks", "shaderpacks", "saves", "config", "options.txt"} {
				if _, e := os.Stat(filepath.Join(vDir, sub)); e == nil {
					return vDir
				}
			}
			if entries, e := os.ReadDir(filepath.Join(r.Path, "mods")); e != nil || len(entries) == 0 {
				return vDir
			}
		}
	}
	return r.Path
}

// UpdateRoot saves directory defaults; memoryMB −1 selects automatic
// memory, 0 inherits the launcher setting.
func (s *Service) UpdateRoot(rootID, name, javaPath string, memoryMB int, jvmArgs ...string) error {
	auto := memoryMB < 0
	if auto {
		memoryMB = 0
	}
	if e := validateOptionalMemory(memoryMB); e != nil {
		return e
	}
	rawJVM := ""
	if len(jvmArgs) > 0 {
		rawJVM = strings.TrimSpace(jvmArgs[0])
		if _, e := ParseJVMArgs(rawJVM); e != nil {
			return e
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	draft := s.cfg.clone()
	for i, r := range draft.Roots {
		if r.ID == rootID {
			if strings.TrimSpace(name) != "" {
				draft.Roots[i].Name = strings.TrimSpace(name)
			}
			draft.Roots[i].JavaPath = strings.TrimSpace(javaPath)
			draft.Roots[i].MemoryMB = memoryMB
			draft.Roots[i].MemoryAuto = auto
			if len(jvmArgs) > 0 {
				draft.Roots[i].JVMArgs = rawJVM
			}
			return s.saveConfigLocked(draft)
		}
	}
	return errors.New("目录不存在")
}

// UpdateVersionConfig saves per-version overrides; memoryMB −1 selects
// automatic memory explicitly, 0 inherits.
func (s *Service) UpdateVersionConfig(rootID, versionID, javaPath string, memoryMB int, jvmArgs string) error {
	auto := memoryMB < 0
	if auto {
		memoryMB = 0
	}
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return errors.New("请先选择游戏版本")
	}
	if e := validateOptionalMemory(memoryMB); e != nil {
		return e
	}
	javaPath = strings.TrimSpace(javaPath)
	jvmArgs = strings.TrimSpace(jvmArgs)
	if _, e := ParseJVMArgs(jvmArgs); e != nil {
		return e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	draft := s.cfg.clone()
	for i, r := range draft.Roots {
		if r.ID == rootID {
			if draft.Roots[i].VersionConfigs == nil {
				draft.Roots[i].VersionConfigs = map[string]VersionConfig{}
			}
			name := draft.Roots[i].VersionConfigs[versionID].DisplayName
			if javaPath == "" && memoryMB == 0 && !auto && jvmArgs == "" && name == "" {
				delete(draft.Roots[i].VersionConfigs, versionID)
			} else {
				draft.Roots[i].VersionConfigs[versionID] = VersionConfig{
					JavaPath:    javaPath,
					MemoryMB:    memoryMB,
					MemoryAuto:  auto,
					JVMArgs:     jvmArgs,
					DisplayName: name,
				}
			}
			return s.saveConfigLocked(draft)
		}
	}
	return errors.New("目录不存在")
}

// InstanceFolder returns (creating it if needed) a sub folder of the
// selected version's game folder: mods, saves, screenshots, ...
func (s *Service) InstanceFolder(rootID, versionID, sub string) (string, error) {
	r, e := s.rootAt(rootID, versionID)
	if e != nil {
		return "", e
	}
	switch sub {
	case "", "mods", "saves", "screenshots", "resourcepacks", "shaderpacks", "logs", "crash-reports", "config":
	default:
		return "", errors.New("unsupported folder")
	}
	p := r.VersionDir()
	if sub != "" {
		p = filepath.Join(p, sub)
		if e := os.MkdirAll(p, 0755); e != nil {
			return "", e
		}
	}
	return p, nil
}
