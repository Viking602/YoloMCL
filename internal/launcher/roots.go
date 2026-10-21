package launcher

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
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
	for _, existing := range s.cfg.Roots {
		if existing.ID == id {
			return existing, nil
		}
	}
	s.cfg.Roots = append(s.cfg.Roots, r)
	s.cfg.ActiveRoot = id
	return r, s.saveLocked()
}
func (s *Service) RemoveRoot(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rootLocks[id] || s.running[id] != nil {
		return errors.New("此目录还有运行中的任务")
	}
	for i, r := range s.cfg.Roots {
		if r.ID == id {
			s.cfg.Roots = append(s.cfg.Roots[:i], s.cfg.Roots[i+1:]...)
			if s.cfg.ActiveRoot == id {
				s.cfg.ActiveRoot = ""
				if len(s.cfg.Roots) > 0 {
					s.cfg.ActiveRoot = s.cfg.Roots[0].ID
				}
			}
			return s.saveLocked()
		}
	}
	return errors.New("目录不存在")
}
func (s *Service) Select(rootID, accountID, version string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rootID != "" {
		found := false
		for i, r := range s.cfg.Roots {
			if r.ID == rootID {
				found = true
				s.cfg.ActiveRoot = rootID
				if version != "" {
					s.cfg.Roots[i].SelectedVersion = version
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
		for _, a := range s.cfg.Accounts {
			if a.ID == accountID {
				found = true
				s.cfg.ActiveAccount = accountID
				break
			}
		}
		if !found {
			return errors.New("账号不存在")
		}
	}
	return s.saveLocked()
}
func ParseJVMArgs(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if strings.ContainsRune(raw, 0) {
		return nil, errors.New("JVM 参数包含无效字符")
	}
	var out []string
	var cur strings.Builder
	var quote rune
	inToken := false
	for _, r := range raw {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
			inToken = true
		case r == ' ' || r == '\t' || r == '\r' || r == '\n':
			if inToken {
				out = append(out, cur.String())
				cur.Reset()
				inToken = false
			}
		default:
			cur.WriteRune(r)
			inToken = true
		}
	}
	if quote != 0 {
		return nil, errors.New("JVM 参数引号未闭合")
	}
	if inToken {
		out = append(out, cur.String())
	}
	for _, tok := range out {
		low := strings.ToLower(tok)
		if low == "-cp" || low == "-classpath" || low == "--class-path" || low == "-jar" {
			return nil, fmt.Errorf("不支持在附加 JVM 参数中覆盖类路径或主 JAR: %s", tok)
		}
	}
	return out, nil
}
func (r Root) ResolveRuntime(versionID string, settings Settings) (string, int, []string, error) {
	vc := r.VersionConfigs[versionID]
	javaPath := strings.TrimSpace(vc.JavaPath)
	if javaPath == "" && settings.UseRootSettings {
		javaPath = strings.TrimSpace(r.JavaPath)
	}
	if javaPath == "" {
		javaPath = strings.TrimSpace(settings.JavaPath)
	}
	// memoryMB 0 means automatic (see MemoryPlan / AutoMemoryFor).
	_, memoryMB := r.MemoryPlan(versionID, settings)
	rawJVM := strings.TrimSpace(vc.JVMArgs)
	if rawJVM == "" && settings.UseRootSettings {
		rawJVM = strings.TrimSpace(r.JVMArgs)
	}
	if rawJVM == "" {
		rawJVM = strings.TrimSpace(settings.JVMArgs)
	}
	jvmArgs, err := ParseJVMArgs(rawJVM)
	return javaPath, memoryMB, jvmArgs, err
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
func validateOptionalMemory(memoryMB int) error {
	if memoryMB == 0 {
		return nil
	}
	if memoryMB < 512 || memoryMB > 131072 {
		return errors.New("内存设置无效")
	}
	if info, e := SystemMemory(); e == nil && memoryMB > info.MaxMB {
		return fmt.Errorf("设置内存超过系统内存上限 %d MB", info.MaxMB)
	}
	return nil
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
	for i, r := range s.cfg.Roots {
		if r.ID == rootID {
			if strings.TrimSpace(name) != "" {
				s.cfg.Roots[i].Name = strings.TrimSpace(name)
			}
			s.cfg.Roots[i].JavaPath = strings.TrimSpace(javaPath)
			s.cfg.Roots[i].MemoryMB = memoryMB
			s.cfg.Roots[i].MemoryAuto = auto
			if len(jvmArgs) > 0 {
				s.cfg.Roots[i].JVMArgs = rawJVM
			}
			return s.saveLocked()
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
	for i, r := range s.cfg.Roots {
		if r.ID == rootID {
			if s.cfg.Roots[i].VersionConfigs == nil {
				s.cfg.Roots[i].VersionConfigs = map[string]VersionConfig{}
			}
			name := s.cfg.Roots[i].VersionConfigs[versionID].DisplayName
			if javaPath == "" && memoryMB == 0 && !auto && jvmArgs == "" && name == "" {
				delete(s.cfg.Roots[i].VersionConfigs, versionID)
			} else {
				s.cfg.Roots[i].VersionConfigs[versionID] = VersionConfig{
					JavaPath:    javaPath,
					MemoryMB:    memoryMB,
					MemoryAuto:  auto,
					JVMArgs:     jvmArgs,
					DisplayName: name,
				}
			}
			return s.saveLocked()
		}
	}
	return errors.New("目录不存在")
}
func findVersionJSON(root, dirName string) (string, error) {
	p, e := safeJoin(root, "versions/"+dirName+"/"+dirName+".json")
	if e == nil {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	vDir, e := safeJoin(root, "versions/"+dirName)
	if e != nil {
		return "", e
	}
	files, e := os.ReadDir(vDir)
	if e != nil {
		return "", e
	}
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(strings.ToLower(f.Name()), ".json") {
			continue
		}
		cand, err := safeJoin(root, "versions/"+dirName+"/"+f.Name())
		if err != nil {
			continue
		}
		b, err := os.ReadFile(cand)
		if err != nil {
			continue
		}
		var gv gameVersion
		if json.Unmarshal(b, &gv) == nil && (gv.MainClass != "" || gv.InheritsFrom != "" || len(gv.Libraries) > 0) {
			return cand, nil
		}
	}
	return "", os.ErrNotExist
}
func scanVersions(root string) []VersionSummary {
	out := []VersionSummary{}
	entries, _ := os.ReadDir(filepath.Join(root, "versions"))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		p, e := findVersionJSON(root, entry.Name())
		if e != nil {
			continue
		}
		b, e := os.ReadFile(p)
		if e != nil {
			continue
		}
		var v gameVersion
		if json.Unmarshal(b, &v) != nil {
			continue
		}
		loader := detectLoader(v)
		mc := v.ID
		if v.InheritsFrom != "" {
			mc = v.InheritsFrom
		}
		out = append(out, VersionSummary{ID: entry.Name(), Minecraft: mc, Loader: loader, LoaderVersion: loaderVersion(v, loader, mc), Snapshot: isSnapshotID(mc), JavaVersion: v.JavaVersion.MajorVersion})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}
func readPack(root string) *PackLock {
	b, e := os.ReadFile(filepath.Join(root, ".mcl", "pack-lock.json"))
	if e != nil {
		return nil
	}
	var p PackLock
	if json.Unmarshal(b, &p) != nil {
		return nil
	}
	upgradeLock(&p)
	return &p
}
func (s *Service) Snapshot() Snapshot {
	s.mu.Lock()
	cfg := s.cfg
	cfg.Roots = make([]Root, len(s.cfg.Roots))
	for i, r := range s.cfg.Roots {
		if len(r.VersionConfigs) > 0 {
			copied := make(map[string]VersionConfig, len(r.VersionConfigs))
			for k, v := range r.VersionConfigs {
				copied[k] = v
			}
			r.VersionConfigs = copied
		}
		cfg.Roots[i] = r
	}
	cfg.Accounts = append([]Account{}, s.cfg.Accounts...)
	authServers := s.authServersLocked()
	sysMem, _ := SystemMemory()
	jobs := s.jobListLocked()
	running := map[string]bool{}
	for id := range s.running {
		running[id] = true
	}
	s.mu.Unlock()
	out := Snapshot{Settings: cfg.Settings, Instances: []Instance{}, Accounts: cfg.Accounts, ActiveRoot: cfg.ActiveRoot, ActiveAccount: cfg.ActiveAccount, Jobs: jobs, AuthServers: authServers, DataDir: s.dir}
	out.Services = ServiceAvailability{Microsoft: s.distribution.HasMicrosoft(), CurseForge: s.distribution.HasCurseForge()}
	for _, r := range cfg.Roots {
		st, e := os.Stat(r.Path)
		count := 0
		mods, _ := os.ReadDir(filepath.Join(r.VersionDir(), "mods"))
		for _, m := range mods {
			if !m.IsDir() && (strings.HasSuffix(m.Name(), ".jar") || strings.HasSuffix(m.Name(), ".jar.disabled")) {
				count++
			}
		}
		versions := scanVersions(r.Path)
		for i := range versions {
			if vc, ok := r.VersionConfigs[versions[i].ID]; ok {
				versions[i].JavaPath = vc.JavaPath
				versions[i].MemoryMB = vc.MemoryMB
				versions[i].MemoryAutoSet = vc.MemoryAuto
				versions[i].JVMArgs = vc.JVMArgs
				versions[i].DisplayName = vc.DisplayName
			}
			versions[i].MemoryAuto, _ = r.MemoryPlan(versions[i].ID, cfg.Settings)
			rr := r
			rr.SelectedVersion = versions[i].ID
			versions[i].AutoMemoryMB = AutoMemoryMB(AutoMemoryInput{TotalMB: sysMem.TotalMB, AvailableMB: sysMem.AvailableMB,
				Minecraft: versions[i].Minecraft, Loader: versions[i].Loader, Mods: countJars(filepath.Join(rr.VersionDir(), "mods"))})
		}
		out.Instances = append(out.Instances, Instance{Root: r, Available: e == nil && st.IsDir(), Versions: versions, ModCount: count, Pack: readPack(r.Path), Running: running[r.ID], PackUpdate: readPackUpdate(r.Path), PackBackups: listPackBackups(r.Path)})
	}
	return out
}

// detectLoader classifies a version JSON. Libraries are checked by
// priority rather than order: NeoForge versions also ship helper libraries
// under net.minecraftforge (e.g. srgutils), so NeoForge must win over Forge.
func detectLoader(v gameVersion) string {
	found := map[string]bool{}
	for _, lib := range v.Libraries {
		switch {
		case strings.HasPrefix(lib.Name, "net.fabricmc:fabric-loader:"):
			found["fabric"] = true
		case strings.HasPrefix(lib.Name, "org.quiltmc:quilt-loader:"):
			found["quilt"] = true
		case strings.HasPrefix(lib.Name, "net.neoforged"):
			found["neoforge"] = true
		case strings.HasPrefix(lib.Name, "net.minecraftforge:"):
			found["forge"] = true
		}
	}
	for _, l := range []string{"neoforge", "quilt", "fabric", "forge"} {
		if found[l] {
			return l
		}
	}
	low := strings.ToLower(v.ID)
	for _, l := range []string{"neoforge", "forge", "fabric", "quilt"} {
		if strings.Contains(low, l) {
			return l
		}
	}
	return "vanilla"
}
