package launcher

import (
	"archive/zip"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type horizonModMeta struct {
	ID          string
	Name        string
	Version     string
	Description string
}

func isHorizonRootVersion(r Root) bool {
	if r.SelectedVersion == "" {
		return false
	}
	v, err := readVersion(r.Path, r.SelectedVersion)
	if err != nil {
		return false
	}
	return installedLoader(v) == "horizon"
}

func parseHorizonModMetaTOML(raw []byte) horizonModMeta {
	var parsed struct {
		ID          string `toml:"id"`
		Name        string `toml:"name"`
		Version     string `toml:"version"`
		Description string `toml:"description"`
		Summary     string `toml:"summary"`
		Mod         struct {
			ID          string `toml:"id"`
			Name        string `toml:"name"`
			Version     string `toml:"version"`
			Description string `toml:"description"`
			Summary     string `toml:"summary"`
		} `toml:"mod"`
	}
	if err := safeTOML(raw, &parsed); err != nil {
		return horizonModMeta{}
	}
	pick := func(a, b string) string {
		if s := strings.TrimSpace(a); s != "" {
			return s
		}
		return strings.TrimSpace(b)
	}
	desc := pick(parsed.Description, parsed.Mod.Description)
	if desc == "" {
		desc = pick(parsed.Summary, parsed.Mod.Summary)
	}
	return horizonModMeta{
		ID:          pick(parsed.ID, parsed.Mod.ID),
		Name:        pick(parsed.Name, parsed.Mod.Name),
		Version:     pick(parsed.Version, parsed.Mod.Version),
		Description: desc,
	}
}

func readHorizonModMeta(ctx context.Context, path, name string, isDir bool) horizonModMeta {
	fallbackID := strings.TrimSuffix(name, filepath.Ext(name))
	if isDir {
		fallbackID = name
	}
	var raw []byte
	if isDir {
		if manifest, err := safeJoin(path, horizonModManifestName); err == nil {
			raw, _ = readWorldgenFile(ctx, manifest, horizonModTOMLLimit)
		}
	} else {
		if f, err := os.Open(path); err == nil {
			if info, serr := f.Stat(); serr == nil && info.Mode().IsRegular() {
				if z, zerr := zip.NewReader(f, info.Size()); zerr == nil {
					for _, entry := range z.File {
						if entry.Name == horizonModManifestName {
							raw, _ = readZipBytesContext(ctx, entry, horizonModTOMLLimit)
							break
						}
					}
				}
			}
			f.Close()
		}
	}
	meta := parseHorizonModMetaTOML(raw)
	if meta.ID == "" {
		meta.ID = fallbackID
	}
	if meta.Name == "" {
		meta.Name = meta.ID
	}
	return meta
}

func readHorizonShaderDescription(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return ""
	}
	var raw []byte
	if st.IsDir() {
		b, err := os.ReadFile(filepath.Join(path, "horizon-shaders.toml"))
		if err == nil && len(b) <= horizonModTOMLLimit {
			raw = b
		}
	} else if r, err := zip.OpenReader(path); err == nil {
		defer r.Close()
		for _, f := range r.File {
			if f.Name == "horizon-shaders.toml" && f.UncompressedSize64 <= horizonModTOMLLimit {
				if rc, oerr := f.Open(); oerr == nil {
					raw, _ = io.ReadAll(io.LimitReader(rc, horizonModTOMLLimit))
					rc.Close()
				}
				break
			}
		}
	}
	if len(raw) == 0 {
		return ""
	}
	var meta struct {
		Description string `toml:"description"`
		Summary     string `toml:"summary"`
		Shader      struct {
			Description string `toml:"description"`
			Summary     string `toml:"summary"`
		} `toml:"shader"`
	}
	if safeTOML(raw, &meta) != nil {
		return ""
	}
	for _, s := range []string{meta.Description, meta.Summary, meta.Shader.Description, meta.Shader.Summary} {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}

// writeHorizonModState updates or removes one "mod.<id>:<bool>" line inside
// <gameDir>/horizon-mods.txt while keeping all unrelated entries untouched.
func writeHorizonModState(gameDir, modID string, enabled, remove bool) error {
	modID = strings.TrimSpace(modID)
	if modID == "" || strings.ContainsAny(modID, ":\r\n") {
		return errors.New("无效的 Horizon 模组 ID")
	}
	p, err := safeJoin(gameDir, "horizon-mods.txt")
	if err != nil {
		return err
	}
	var existing []string
	if raw, rerr := os.ReadFile(p); rerr == nil {
		existing = strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	} else if !os.IsNotExist(rerr) {
		return rerr
	}
	targetKey := "mod." + modID
	var out []string
	replaced := false
	val := "false"
	if enabled {
		val = "true"
	}
	for _, line := range existing {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if k, _, ok := strings.Cut(trimmed, ":"); ok && strings.TrimSpace(k) == targetKey {
			if !remove && !replaced {
				out = append(out, targetKey+":"+val)
				replaced = true
			}
			continue
		}
		out = append(out, trimmed)
	}
	if !remove && !replaced {
		out = append(out, targetKey+":"+val)
	}
	content := ""
	if len(out) > 0 {
		content = strings.Join(out, "\n") + "\n"
	}
	return atomicBytes(p, []byte(content), 0o644)
}

func fileSHA1Hex(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

func scanHorizonMods(ctx context.Context, gameDir string) ([]Mod, error) {
	entries, err := readDirRetryContext(ctx, filepath.Join(gameDir, "horizon-mods"))
	if os.IsNotExist(err) {
		return []Mod{}, nil
	}
	if err != nil {
		return nil, err
	}
	enabledMap, _ := readHorizonEnabledMods(ctx, gameDir)
	records := readRecords(gameDir)
	checks := readChecks(gameDir)
	out := []Mod{}
	for _, en := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if en.Type()&os.ModeSymlink != 0 || strings.HasPrefix(en.Name(), ".") {
			continue
		}
		name := en.Name()
		isDir := en.IsDir()
		if !isDir && !strings.HasSuffix(strings.ToLower(name), horizonModFileSuffix) {
			continue
		}
		full, err := safeJoin(gameDir, "horizon-mods/"+name)
		if err != nil {
			continue
		}
		if isDir {
			if _, serr := os.Stat(filepath.Join(full, horizonModManifestName)); serr != nil {
				continue
			}
		}
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		meta := readHorizonModMeta(ctx, full, name, isDir)
		rec := records[name]
		sha1Hex := rec.SHA1
		if sha1Hex == "" && !isDir {
			if h, herr := cachedSHA1(full, info); herr == nil {
				sha1Hex = h
			}
		}
		source := rec.Source
		projectID := rec.ProjectID
		versionID := rec.VersionID
		if source == "" {
			source = "horizon"
		}
		if projectID == "" {
			projectID = meta.ID
		}
		m := Mod{
			File:        name,
			Name:        meta.Name,
			ModID:       meta.ID,
			Version:     meta.Version,
			Loader:      "horizon",
			Enabled:     enabledMap[meta.ID],
			Size:        info.Size(),
			SHA1:        sha1Hex,
			Source:      source,
			ProjectID:   projectID,
			VersionID:   versionID,
			Description: meta.Description,
		}
		if sha1Hex != "" {
			if chk, ok := checks[sha1Hex]; ok {
				if chk.Source != "" {
					m.Source = chk.Source
				}
				if chk.ProjectID != "" {
					m.ProjectID = chk.ProjectID
				}
				if chk.VersionID != "" {
					m.VersionID = chk.VersionID
				}
				m.Status = chk.Status
				m.Latest = chk.Latest
			}
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

func (s *Service) toggleHorizonModIn(r Root, name string) error {
	if filepath.Base(name) != name || name == "" || name == "." || name == ".." {
		return errors.New("无效的模组文件名")
	}
	vDir := r.VersionDir()
	full, err := safeJoin(vDir, "horizon-mods/"+name)
	if err != nil {
		return err
	}
	info, err := os.Stat(full)
	if err != nil {
		return err
	}
	meta := readHorizonModMeta(context.Background(), full, name, info.IsDir())
	enabledMap, _ := readHorizonEnabledMods(context.Background(), vDir)
	if err := writeHorizonModState(vDir, meta.ID, !enabledMap[meta.ID], false); err != nil {
		return err
	}
	s.invalidateStatus(vDir)
	return nil
}

func (s *Service) removeHorizonModIn(r Root, name string) error {
	_, err := s.removeHorizonModReceipt(r, name)
	return err
}

func (s *Service) removeHorizonModReceipt(r Root, name string) (ModRemoval, error) {
	if filepath.Base(name) != name || name == "" || name == "." || name == ".." {
		return ModRemoval{}, errors.New("无效的模组文件名")
	}
	vDir := r.VersionDir()
	src, err := safeJoin(vDir, "horizon-mods/"+name)
	if err != nil {
		return ModRemoval{}, err
	}
	info, err := os.Stat(src)
	if err != nil {
		return ModRemoval{}, err
	}
	if info.IsDir() {
		if _, serr := os.Stat(filepath.Join(src, horizonModManifestName)); serr != nil {
			return ModRemoval{}, errors.New("无效的模组文件名")
		}
	} else if !strings.HasSuffix(strings.ToLower(name), horizonModFileSuffix) {
		return ModRemoval{}, errors.New("无效的模组文件名")
	}
	meta := readHorizonModMeta(context.Background(), src, name, info.IsDir())
	enabledMap, _ := readHorizonEnabledMods(context.Background(), vDir)
	wasEnabled, hadState := enabledMap[meta.ID]
	records := readRecords(vDir)
	var savedRec *modRecord
	if rec, ok := records[name]; ok {
		cp := rec
		savedRec = &cp
	}
	dst, err := safeJoin(r.Path, ".mcl/backups/"+time.Now().Format("20060102-150405")+"-"+newID()[:6]+"/"+name)
	if err != nil {
		return ModRemoval{}, err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return ModRemoval{}, err
	}
	if err := os.Rename(src, dst); err != nil {
		return ModRemoval{}, err
	}
	_ = writeHorizonModState(vDir, meta.ID, false, true)
	if savedRec != nil {
		delete(records, name)
		_ = atomicJSON(filepath.Join(vDir, ".mcl", "mods.json"), records)
	}
	s.invalidateStatus(vDir)
	return ModRemoval{
		rootID:          r.ID,
		source:          src,
		backup:          dst,
		horizon:         true,
		modID:           meta.ID,
		hadHorizonState: hadState,
		horizonEnabled:  wasEnabled,
		recordName:      name,
		record:          savedRec,
	}, nil
}

func (s *Service) addLocalHorizonModsIn(r Root, paths []string) error {
	vDir := r.VersionDir()
	for _, src := range paths {
		if !strings.EqualFold(filepath.Ext(src), horizonModFileSuffix) {
			return horizonErr("Horizon 实例只能导入 .hmod 模组", "Horizon instances only accept .hmod mods")
		}
		base := filepath.Base(src)
		dst, err := safeJoin(vDir, "horizon-mods/"+base)
		if err != nil {
			return err
		}
		if err := copyFile(src, dst); err != nil {
			return err
		}
		meta := readHorizonModMeta(context.Background(), dst, base, false)
		if err := writeHorizonModState(vDir, meta.ID, true, false); err != nil {
			return err
		}
	}
	s.invalidateStatus(vDir)
	return nil
}

func (s *Service) verifyDownloadedMinisign(key, signature, filePath string) error {
	if strings.TrimSpace(key) == "" {
		return nil
	}
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	if err := verifyMinisign(key, signature, raw); err != nil {
		return horizonErr("Horizon 资源 minisign 签名校验失败: "+err.Error(), "Horizon asset minisign signature verification failed: "+err.Error())
	}
	return nil
}

func (s *Service) installHorizonModIn(ctx context.Context, p progress, r Root, project, versionID, replace string) error {
	_, proj, ver, err := s.horizonLookup(ctx, project, versionID)
	if err != nil {
		return err
	}
	if proj.Kind != "mod" {
		return horizonErr("该项目不是可直接安装的 Horizon 内容/WASM 模组", "This project is not a directly installable Horizon content/WASM mod")
	}
	dl, err := s.verifyHorizonRegistryDownload(proj, ver, horizonModFileSuffix)
	if err != nil {
		return err
	}
	stage, err := safeJoin(r.Path, ".mcl/staging/"+newID())
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(stage)

	staged, err := safeJoin(stage, dl.Name)
	if err != nil {
		return err
	}
	p("下载 "+proj.Name, 0, 1)
	if err := s.download(ctx, dl, staged); err != nil {
		return err
	}
	if err := s.verifyDownloadedMinisign(proj.Key, ver.File.Signature, staged); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	vDir := r.VersionDir()
	modsDir := filepath.Join(vDir, "horizon-mods")
	if err := os.MkdirAll(modsDir, 0o755); err != nil {
		return err
	}
	meta := readHorizonModMeta(ctx, staged, dl.Name, false)
	modID := proj.ID
	if meta.ID != "" {
		modID = meta.ID
	}

	installed, _ := scanHorizonMods(ctx, vDir)
	backupDir, err := safeJoin(r.Path, ".mcl/backups/"+time.Now().Format("20060102-150405")+"-"+newID()[:6])
	if err != nil {
		return err
	}
	moved := map[string]string{}
	rollback := func() {
		for orig, bak := range moved {
			_ = os.Rename(bak, orig)
		}
	}
	for _, m := range installed {
		if m.ModID == modID || (m.Source == "horizon" && m.ProjectID == proj.ID) || (replace != "" && m.File == replace) || strings.EqualFold(m.File, dl.Name) {
			orig, jerr := safeJoin(vDir, "horizon-mods/"+m.File)
			if jerr != nil {
				rollback()
				return jerr
			}
			if _, ok := moved[orig]; ok {
				continue
			}
			if err := os.MkdirAll(backupDir, 0o755); err != nil {
				rollback()
				return err
			}
			bak := filepath.Join(backupDir, m.File)
			if err := os.Rename(orig, bak); err != nil {
				rollback()
				return err
			}
			moved[orig] = bak
		}
	}

	dest, err := safeJoin(vDir, "horizon-mods/"+dl.Name)
	if err != nil {
		rollback()
		return err
	}
	if err := os.Rename(staged, dest); err != nil {
		rollback()
		return err
	}
	if err := writeHorizonModState(vDir, modID, true, false); err != nil {
		_ = os.Remove(dest)
		rollback()
		return err
	}
	records := readRecords(vDir)
	records[dl.Name] = modRecord{
		SHA1:      fileSHA1Hex(dest),
		Source:    "horizon",
		ProjectID: proj.ID,
		VersionID: ver.Version,
	}
	_ = atomicJSON(filepath.Join(vDir, ".mcl", "mods.json"), records)
	s.invalidateStatus(vDir)
	p("已安装 "+proj.Name, 1, 1)
	return nil
}

func horizonModOverviewCounts(ctx context.Context, gameDir string, entries []os.DirEntry) (total, enabled int) {
	enabledMap, _ := readHorizonEnabledMods(ctx, gameDir)
	for _, en := range entries {
		if ctx.Err() != nil {
			break
		}
		if en.Type()&os.ModeSymlink != 0 || strings.HasPrefix(en.Name(), ".") {
			continue
		}
		name := en.Name()
		isDir := en.IsDir()
		if !isDir && !strings.HasSuffix(strings.ToLower(name), horizonModFileSuffix) {
			continue
		}
		full, err := safeJoin(gameDir, "horizon-mods/"+name)
		if err != nil {
			continue
		}
		if isDir {
			if _, serr := os.Stat(filepath.Join(full, horizonModManifestName)); serr != nil {
				continue
			}
		}
		total++
		meta := readHorizonModMeta(ctx, full, name, isDir)
		if enabledMap[meta.ID] {
			enabled++
		}
	}
	return total, enabled
}

func (s *Service) checkHorizonModsIn(ctx context.Context, r Root, vDir, mc string, p progress, partial func(map[string]Mod)) error {
	mods, err := scanHorizonMods(ctx, vDir)
	if err != nil {
		return err
	}
	checks := map[string]Mod{}
	if len(mods) == 0 {
		return atomicJSON(filepath.Join(vDir, ".mcl", "mod-checks.json"), checks)
	}
	p("正在从 Horizon 目录检测模组更新", 0, len(mods))
	idx, err := s.horizonModIndex(ctx, false)
	if err != nil {
		return err
	}
	byID := make(map[string]horizonRegistryProject, len(idx.Projects))
	for _, proj := range idx.Projects {
		if proj.Kind == "mod" {
			byID[strings.ToLower(proj.ID)] = proj
		}
	}
	batch := map[string]Mod{}
	for i, m := range mods {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		m.Latest = nil
		m.Status = "unknown"
		proj, ok := byID[strings.ToLower(m.ProjectID)]
		if !ok && m.ModID != "" {
			proj, ok = byID[strings.ToLower(m.ModID)]
		}
		if ok {
			m.Source = "horizon"
			m.ProjectID = proj.ID
			m.Status = "current"
			vs := idx.activeVersions(proj)
			var best *horizonRegistryVersion
			for j := range vs {
				if mc == "" || vs[j].Game == mc {
					best = &vs[j]
					break
				}
			}
			curVer := m.VersionID
			if curVer == "" {
				curVer = m.Version
			}
			if best != nil && best.Version != "" && best.Version != curVer {
				lv := horizonToProjectVersion(proj, *best)
				m.Latest = &lv
				m.Status = "update"
			}
		}
		if m.SHA1 != "" {
			checks[m.SHA1] = m
			batch[m.SHA1] = m
		}
		p("正在识别 "+m.Name, i+1, len(mods))
	}
	if partial != nil && len(batch) > 0 {
		partial(batch)
	}
	return atomicJSON(filepath.Join(vDir, ".mcl", "mod-checks.json"), checks)
}

func (s *Service) horizonAddonRemoteInfo(ctx context.Context, r Root, kind string, items []AddonRef) map[string]RemoteInfo {
	out := map[string]RemoteInfo{}
	if len(items) == 0 {
		return out
	}
	idx, err := s.horizonModIndex(ctx, false)
	if err != nil {
		return out
	}
	vDir := r.VersionDir()
	records := readRecords(vDir)
	byID := map[string]horizonRegistryProject{}
	byFile := map[string]horizonRegistryProject{}
	for _, p := range idx.Projects {
		if horizonDiscoverType(p.Kind) != kind {
			continue
		}
		byID[strings.ToLower(p.ID)] = p
		for _, v := range p.Versions {
			if v.File.Name != "" {
				byFile[strings.ToLower(v.File.Name)] = p
			}
		}
	}
	for _, item := range items {
		if item.SHA1 == "" {
			continue
		}
		var proj horizonRegistryProject
		var ok bool
		if kind == "mod" {
			if rec, found := records[item.File]; found && rec.ProjectID != "" {
				proj, ok = byID[strings.ToLower(rec.ProjectID)]
			}
		}
		if !ok {
			clean := strings.TrimSuffix(strings.ToLower(item.File), ".disabled")
			proj, ok = byFile[clean]
			if !ok {
				stem := strings.TrimSuffix(clean, filepath.Ext(clean))
				proj, ok = byID[stem]
			}
		}
		if !ok {
			continue
		}
		out[item.SHA1] = RemoteInfo{
			Icon:    proj.Icon,
			Summary: proj.Summary,
			Cats:    append([]string(nil), proj.Categories...),
		}
	}
	return out
}
