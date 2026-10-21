package launcher

import (
	"archive/zip"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type modRecord struct {
	SHA1      string `json:"sha1"`
	Source    string `json:"source"`
	ProjectID string `json:"projectId"`
	VersionID string `json:"versionId"`
}

func readRecords(root string) map[string]modRecord {
	out := map[string]modRecord{}
	b, e := os.ReadFile(filepath.Join(root, ".mcl", "mods.json"))
	if e == nil {
		_ = json.Unmarshal(b, &out)
	}
	return out
}
func readChecks(root string) map[string]Mod {
	out := map[string]Mod{}
	b, e := os.ReadFile(filepath.Join(root, ".mcl", "mod-checks.json"))
	if e == nil {
		_ = json.Unmarshal(b, &out)
	}
	return out
}
func (s *Service) ListMods(rootID string) ([]Mod, error) {
	r, e := s.getRoot(rootID)
	if e != nil {
		return nil, e
	}
	return scanMods(r.VersionDir())
}
func scanMods(root string) ([]Mod, error) {
	entries, e := os.ReadDir(filepath.Join(root, "mods"))
	if os.IsNotExist(e) {
		return []Mod{}, nil
	}
	if e != nil {
		return nil, e
	}
	records, checks := readRecords(root), readChecks(root)
	out := []Mod{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || (!strings.HasSuffix(name, ".jar") && !strings.HasSuffix(name, ".jar.disabled")) {
			continue
		}
		path, e := safeJoin(root, "mods/"+name)
		if e != nil {
			return nil, e
		}
		st, e := os.Stat(path)
		if e != nil {
			return nil, e
		}
		m := Mod{File: name, Name: strings.TrimSuffix(strings.TrimSuffix(name, ".disabled"), ".jar"), Enabled: !strings.HasSuffix(name, ".disabled"), Size: st.Size(), Status: "unchecked"}
		m.SHA1, _, e = fileHashes(path)
		if e != nil {
			return nil, e
		}
		readModMetadata(path, &m)
		rec := records[strings.TrimSuffix(name, ".disabled")]
		if rec.SHA1 == m.SHA1 {
			m.Source = rec.Source
			m.ProjectID = rec.ProjectID
			m.VersionID = rec.VersionID
		}
		if c, ok := checks[m.SHA1]; ok {
			m.Source = c.Source
			m.ProjectID = c.ProjectID
			m.VersionID = c.VersionID
			m.Latest = c.Latest
			m.Status = c.Status
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}
func readModMetadata(path string, m *Mod) {
	r, e := zip.OpenReader(path)
	if e != nil {
		m.Status = "invalid"
		return
	}
	defer r.Close()
	for _, f := range r.File {
		if f.UncompressedSize64 > 2<<20 {
			continue
		}
		switch f.Name {
		case "fabric.mod.json":
			var v struct {
				ID      string `json:"id"`
				Name    string `json:"name"`
				Version string `json:"version"`
			}
			if readZipJSON(f, &v) == nil {
				m.ModID = v.ID
				m.Name = v.Name
				m.Version = v.Version
				m.Loader = "fabric"
				return
			}
		case "quilt.mod.json":
			var v struct {
				Loader struct {
					ID       string `json:"id"`
					Version  string `json:"version"`
					Metadata struct {
						Name string `json:"name"`
					} `json:"metadata"`
				} `json:"quilt_loader"`
			}
			if readZipJSON(f, &v) == nil {
				m.ModID = v.Loader.ID
				m.Version = v.Loader.Version
				m.Name = v.Loader.Metadata.Name
				m.Loader = "quilt"
				return
			}
		case "META-INF/mods.toml", "META-INF/neoforge.mods.toml":
			rd, e := f.Open()
			if e != nil {
				continue
			}
			b, e := io.ReadAll(io.LimitReader(rd, 2<<20))
			rd.Close()
			if e != nil {
				continue
			}
			var v struct {
				Mods []struct {
					ID      string `toml:"modId"`
					Name    string `toml:"displayName"`
					Version string `toml:"version"`
				} `toml:"mods"`
			}
			if toml.Unmarshal(b, &v) == nil && len(v.Mods) > 0 {
				m.ModID = v.Mods[0].ID
				m.Name = v.Mods[0].Name
				m.Version = v.Mods[0].Version
				m.Loader = "forge"
				if strings.Contains(f.Name, "neoforge") {
					m.Loader = "neoforge"
				}
			}
		}
	}
	if strings.Contains(m.Version, "${") {
		m.Version = ""
		for _, f := range r.File {
			if f.Name == "META-INF/MANIFEST.MF" {
				rd, e := f.Open()
				if e != nil {
					break
				}
				b, _ := io.ReadAll(io.LimitReader(rd, 2<<20))
				rd.Close()
				for _, line := range strings.Split(string(b), "\n") {
					if strings.HasPrefix(line, "Implementation-Version:") {
						m.Version = strings.TrimSpace(strings.TrimPrefix(line, "Implementation-Version:"))
					}
				}
			}
		}
	}
}

// CurseForge's fingerprint is MurmurHash2(seed=1), with ASCII whitespace removed.
func curseFingerprint(data []byte) uint32 {
	b := make([]byte, 0, len(data))
	for _, v := range data {
		if v != 9 && v != 10 && v != 13 && v != 32 {
			b = append(b, v)
		}
	}
	const mul uint32 = 0x5bd1e995
	h := uint32(1) ^ uint32(len(b))
	for len(b) >= 4 {
		k := binary.LittleEndian.Uint32(b)
		k *= mul
		k ^= k >> 24
		k *= mul
		h *= mul
		h ^= k
		b = b[4:]
	}
	switch len(b) {
	case 3:
		h ^= uint32(b[2]) << 16
		fallthrough
	case 2:
		h ^= uint32(b[1]) << 8
		fallthrough
	case 1:
		h ^= uint32(b[0])
		h *= mul
	}
	h ^= h >> 13
	h *= mul
	h ^= h >> 15
	return h
}
func (s *Service) mutateRoot(id string, fn func(Root) error) error {
	r, e := s.getRoot(id)
	if e != nil {
		return e
	}
	s.mu.Lock()
	if s.rootLocks[id] || s.running[id] != nil {
		s.mu.Unlock()
		return errors.New("请先结束此目录的任务或游戏")
	}
	s.rootLocks[id] = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.rootLocks, id); s.mu.Unlock() }()
	return fn(r)
}
func (s *Service) ToggleMod(rootID, name string) error {
	return s.mutateRoot(rootID, func(r Root) error {
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
	})
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
func instanceCompatibility(r Root) (string, string, error) {
	versions := scanVersions(r.Path)
	for _, v := range versions {
		if v.ID == r.SelectedVersion {
			return v.Minecraft, v.Loader, nil
		}
	}
	if p := readPack(r.Path); p != nil {
		return p.Minecraft, p.Loader, nil
	}
	return "", "", errors.New("请先选择一个已安装的游戏版本")
}
func (s *Service) CheckMods(rootID string) (string, error) {
	r, e := s.getRoot(rootID)
	if e != nil {
		return "", e
	}
	return s.startJob("检测模组版本", rootID, func(ctx context.Context, p progress) error {
		vDir := r.VersionDir()
		mods, e := scanMods(vDir)
		if e != nil {
			return e
		}
		mc, loader, e := instanceCompatibility(r)
		if e != nil {
			return e
		}
		hashes := []string{}
		for _, m := range mods {
			hashes = append(hashes, m.SHA1)
		}
		checks := map[string]Mod{}
		if len(hashes) == 0 {
			return atomicJSON(filepath.Join(vDir, ".mcl", "mod-checks.json"), checks)
		}
		known := map[string]mrVersion{}
		updates := map[string]mrVersion{}
		p("正在通过文件哈希识别 Modrinth 模组", 0, len(mods))
		if e = s.jsonRequest(ctx, "POST", modrinthAPI+"/version_files", map[string]any{"hashes": hashes, "algorithm": "sha1"}, nil, &known); e != nil {
			return e
		}
		if e = s.jsonRequest(ctx, "POST", modrinthAPI+"/version_files/update", map[string]any{"hashes": hashes, "algorithm": "sha1", "game_versions": []string{mc}, "loaders": []string{loader}}, nil, &updates); e != nil {
			return e
		}
		for i, m := range mods {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			m.Latest = nil
			m.Status = "unknown"
			if v, ok := known[m.SHA1]; ok {
				m.Source = "modrinth"
				m.ProjectID = v.ProjectID
				m.VersionID = v.ID
				m.Status = "current"
				if next, ok := updates[m.SHA1]; ok && next.ID != v.ID {
					nv := next.version()
					m.Latest = &nv
					m.Status = "update"
				}
			}
			checks[m.SHA1] = m
			p("正在识别 "+m.Name, i+1, len(mods))
		}
		if s.distribution.HasCurseForge() {
			fps := []uint32{}
			byFP := map[uint32]string{}
			for _, m := range mods {
				if checks[m.SHA1].Status != "unknown" && m.Source != "curseforge" {
					continue
				}
				path, e := safeJoin(r.Path, "mods/"+m.File)
				if e != nil {
					return e
				}
				st, e := os.Stat(path)
				if e != nil {
					return e
				}
				if st.Size() > 256<<20 {
					continue
				}
				b, e := os.ReadFile(path)
				if e != nil {
					return e
				}
				fp := curseFingerprint(b)
				fps = append(fps, fp)
				byFP[fp] = m.SHA1
			}
			if len(fps) > 0 {
				p("正在查询 CurseForge 文件指纹", 0, len(fps))
				var matches struct {
					Data struct {
						ExactMatches []struct {
							File cfFile `json:"file"`
						} `json:"exactMatches"`
					} `json:"data"`
				}
				if e = s.cfJSON(ctx, "POST", "/fingerprints", map[string]any{"fingerprints": fps}, &matches); e != nil {
					return e
				}
				for i, match := range matches.Data.ExactMatches {
					hash, ok := byFP[match.File.Fingerprint]
					if !ok {
						continue
					}
					m := checks[hash]
					m.Source = "curseforge"
					m.ProjectID = fmt.Sprint(match.File.ModID)
					m.VersionID = fmt.Sprint(match.File.ID)
					m.Status = "current"
					vs, e := s.Versions(ctx, "curseforge", m.ProjectID, mc, loader)
					if e != nil {
						return e
					}
					if len(vs) > 0 && vs[0].ID != m.VersionID && vs[0].Published > match.File.FileDate {
						m.Latest = &vs[0]
						m.Status = "update"
					}
					checks[hash] = m
					p("检测 CurseForge · "+m.Name, i+1, len(matches.Data.ExactMatches))
				}
			}
		}
		return atomicJSON(filepath.Join(vDir, ".mcl", "mod-checks.json"), checks)
	})
}
func (s *Service) InstallMod(rootID, source, project, id, replace string) (string, error) {
	r, e := s.getRoot(rootID)
	if e != nil {
		return "", e
	}
	return s.startJob("安装模组及依赖", rootID, func(ctx context.Context, p progress) error {
		mc, loader, e := instanceCompatibility(r)
		if e != nil {
			return e
		}
		if loader == "vanilla" {
			return errors.New("原版不加载模组，请先安装并选择 Fabric / Quilt / Forge / NeoForge")
		}
		version, e := s.projectVersion(ctx, source, project, id)
		if e != nil {
			return e
		}
		if version.ProjectID != project {
			return errors.New("模组版本与项目不匹配")
		}
		plan := []ProjectVersion{}
		seen := map[string]string{}
		vDir := r.VersionDir()
		installed, e := scanMods(vDir)
		if e != nil {
			return e
		}
		var resolve func(ProjectVersion) error
		resolve = func(v ProjectVersion) error {
			key := v.Source + ":" + v.ProjectID
			if previous, ok := seen[key]; ok {
				if previous != v.ID {
					return fmt.Errorf("%s 的依赖版本约束冲突", v.Name)
				}
				return nil
			}
			seen[key] = v.ID
			if len(seen) > 128 {
				return errors.New("依赖数量超出限制")
			}
			if !compatible(v, mc, loader) {
				return fmt.Errorf("%s 不兼容 Minecraft %s / %s", v.Name, mc, loader)
			}
			for _, d := range v.Dependencies {
				if d.Kind != "required" {
					continue
				}
				present := false
				for _, m := range installed {
					if m.Enabled && m.Source == v.Source && m.ProjectID == d.ProjectID && (d.VersionID == "" || m.VersionID == d.VersionID) {
						present = true
						break
					}
				}
				if present {
					continue
				}
				var dep ProjectVersion
				var e error
				if d.VersionID != "" {
					dep, e = s.projectVersion(ctx, v.Source, d.ProjectID, d.VersionID)
				} else {
					vs, err := s.Versions(ctx, v.Source, d.ProjectID, mc, loader)
					e = err
					if e == nil && len(vs) == 0 {
						e = fmt.Errorf("依赖 %s 没有兼容版本", d.ProjectID)
					}
					if e == nil {
						dep = vs[0]
					}
				}
				if e != nil {
					return e
				}
				if e = resolve(dep); e != nil {
					return e
				}
			}
			plan = append(plan, v)
			return nil
		}
		if e = resolve(version); e != nil {
			return e
		}
		stage, e := safeJoin(r.Path, ".mcl/staging/"+newID())
		if e != nil {
			return e
		}
		if e = os.MkdirAll(stage, 0755); e != nil {
			return e
		}
		defer os.RemoveAll(stage)
		files := []DownloadFile{}
		names := map[string]bool{}
		for i, v := range plan {
			f, e := primaryFile(v)
			if e != nil {
				return e
			}
			if f.URL == "" {
				return fmt.Errorf("%s 禁止第三方下载，请从 https://www.curseforge.com/minecraft/mc-mods/%s 下载后手动导入", v.Name, v.ProjectID)
			}
			if !strings.HasSuffix(f.Name, ".jar") || filepath.Base(f.Name) != f.Name {
				return errors.New("平台返回了无效模组文件名")
			}
			if names[strings.ToLower(f.Name)] {
				return errors.New("依赖文件名冲突")
			}
			names[strings.ToLower(f.Name)] = true
			p("下载 "+v.Name, i, len(plan))
			staged, e := safeJoin(stage, f.Name)
			if e != nil {
				return e
			}
			if e = s.download(ctx, f, staged); e != nil {
				return e
			}
			files = append(files, f)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		records := readRecords(vDir)
		backup, e := safeJoin(r.Path, ".mcl/backups/"+time.Now().Format("20060102-150405")+"-"+newID()[:6])
		if e != nil {
			return e
		}
		if e = os.MkdirAll(backup, 0755); e != nil {
			return e
		}
		moved := map[string]string{}
		added := []string{}
		rollback := func() {
			for _, path := range added {
				_ = os.Remove(path)
			}
			for orig, bak := range moved {
				_ = os.Rename(bak, orig)
			}
		}
		for i, v := range plan {
			f := files[i]
			stagedMod := Mod{}
			readModMetadata(filepath.Join(stage, f.Name), &stagedMod)
			disabled := false
			for _, m := range installed {
				if (m.Source == v.Source && m.ProjectID == v.ProjectID) || (stagedMod.ModID != "" && stagedMod.ModID == m.ModID) || m.File == replace && v.ProjectID == project || strings.TrimSuffix(m.File, ".disabled") == f.Name {
					orig, e := safeJoin(vDir, "mods/"+m.File)
					if e != nil {
						rollback()
						return e
					}
					bak := filepath.Join(backup, m.File)
					if _, ok := moved[orig]; !ok {
						if e = os.Rename(orig, bak); e != nil {
							rollback()
							return e
						}
						moved[orig] = bak
					}
					if !m.Enabled && v.ProjectID == project {
						disabled = true
					}
				}
			}
			name := f.Name
			if disabled {
				name += ".disabled"
			}
			dest, e := safeJoin(vDir, "mods/"+name)
			if e != nil {
				rollback()
				return e
			}
			if e = os.MkdirAll(filepath.Dir(dest), 0755); e != nil {
				rollback()
				return e
			}
			if _, e = os.Stat(dest); !os.IsNotExist(e) {
				rollback()
				return errors.New("目标模组文件已经存在")
			}
			if e = os.Rename(filepath.Join(stage, f.Name), dest); e != nil {
				rollback()
				return e
			}
			added = append(added, dest)
			hash, _, e := fileHashes(dest)
			if e != nil {
				rollback()
				return e
			}
			records[f.Name] = modRecord{SHA1: hash, Source: v.Source, ProjectID: v.ProjectID, VersionID: v.ID}
		}
		if e = atomicJSON(filepath.Join(vDir, ".mcl", "mods.json"), records); e != nil {
			rollback()
			return e
		}
		p("模组及依赖已安装", len(plan), len(plan))
		return nil
	})
}
func packFolder(kind string) (string, error) {
	switch kind {
	case "resourcepack":
		return "resourcepacks", nil
	case "shader":
		return "shaderpacks", nil
	}
	return "", errors.New("不支持的资源包类型")
}
func stripFormatting(s string) string {
	var out strings.Builder
	skip := false
	for _, r := range s {
		if skip {
			skip = false
			continue
		}
		if r == '§' {
			skip = true
			continue
		}
		out.WriteRune(r)
	}
	return strings.TrimSpace(out.String())
}
func readPackMcmeta(path string) string {
	r, e := zip.OpenReader(path)
	if e != nil {
		return ""
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == "pack.mcmeta" && f.UncompressedSize64 <= 1<<20 {
			var meta struct {
				Pack struct {
					Description any `json:"description"`
				} `json:"pack"`
			}
			if readZipJSON(f, &meta) == nil {
				if s, ok := meta.Pack.Description.(string); ok {
					return stripFormatting(s)
				}
			}
		}
	}
	return ""
}
func (s *Service) ListAssetPacks(rootID, kind string) ([]AssetPack, error) {
	r, e := s.getRoot(rootID)
	if e != nil {
		return nil, e
	}
	folder, e := packFolder(kind)
	if e != nil {
		return nil, e
	}
	vDir := r.VersionDir()
	entries, e := os.ReadDir(filepath.Join(vDir, folder))
	if os.IsNotExist(e) {
		return []AssetPack{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []AssetPack{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			continue
		}
		low := strings.ToLower(name)
		if !strings.HasSuffix(low, ".zip") && !strings.HasSuffix(low, ".zip.disabled") {
			continue
		}
		full, e := safeJoin(vDir, folder+"/"+name)
		if e != nil {
			continue
		}
		st, e := os.Stat(full)
		if e != nil {
			continue
		}
		clean := strings.TrimSuffix(strings.TrimSuffix(name, ".disabled"), ".zip")
		desc := ""
		if kind == "resourcepack" {
			desc = readPackMcmeta(full)
		}
		out = append(out, AssetPack{
			File:        name,
			Name:        clean,
			Description: desc,
			Kind:        kind,
			Enabled:     !strings.HasSuffix(low, ".disabled"),
			Size:        st.Size(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}
func (s *Service) ToggleAssetPack(rootID, kind, name string) error {
	folder, e := packFolder(kind)
	if e != nil {
		return e
	}
	return s.mutateRoot(rootID, func(r Root) error {
		low := strings.ToLower(name)
		if filepath.Base(name) != name || (!strings.HasSuffix(low, ".zip") && !strings.HasSuffix(low, ".zip.disabled")) {
			return errors.New("无效的资源包文件名")
		}
		target := name + ".disabled"
		if strings.HasSuffix(name, ".disabled") {
			target = strings.TrimSuffix(name, ".disabled")
		}
		vDir := r.VersionDir()
		src, e := safeJoin(vDir, folder+"/"+name)
		if e != nil {
			return e
		}
		dst, e := safeJoin(vDir, folder+"/"+target)
		if e != nil {
			return e
		}
		if _, e = os.Stat(dst); !os.IsNotExist(e) {
			return errors.New("目标文件已经存在")
		}
		return os.Rename(src, dst)
	})
}
func (s *Service) RemoveAssetPack(rootID, kind, name string) error {
	folder, e := packFolder(kind)
	if e != nil {
		return e
	}
	return s.mutateRoot(rootID, func(r Root) error {
		low := strings.ToLower(name)
		if filepath.Base(name) != name || (!strings.HasSuffix(low, ".zip") && !strings.HasSuffix(low, ".zip.disabled")) {
			return errors.New("无效的资源包文件名")
		}
		vDir := r.VersionDir()
		src, e := safeJoin(vDir, folder+"/"+name)
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
func (s *Service) AddLocalAssetPacks(rootID, kind string, paths []string) error {
	folder, e := packFolder(kind)
	if e != nil {
		return e
	}
	return s.mutateRoot(rootID, func(r Root) error {
		vDir := r.VersionDir()
		for _, src := range paths {
			if !strings.EqualFold(filepath.Ext(src), ".zip") {
				return errors.New("只能导入 .zip 格式的材质包或光影包")
			}
			dst, e := safeJoin(vDir, folder+"/"+filepath.Base(src))
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
func (s *Service) InstallAssetPack(rootID, kind, source, project, id string) (string, error) {
	folder, e := packFolder(kind)
	if e != nil {
		return "", e
	}
	r, e := s.getRoot(rootID)
	if e != nil {
		return "", e
	}
	title := "安装材质包"
	if kind == "shader" {
		title = "安装光影包"
	}
	return s.startJob(title, rootID, func(ctx context.Context, p progress) error {
		version, e := s.projectVersion(ctx, source, project, id)
		if e != nil {
			return e
		}
		f, e := primaryFile(version)
		if e != nil {
			return e
		}
		if f.URL == "" {
			return errors.New("该文件不支持直链下载，请访问项目主页手动下载")
		}
		if filepath.Base(f.Name) != f.Name || !strings.HasSuffix(strings.ToLower(f.Name), ".zip") {
			return errors.New("平台返回了无效的资源包文件名")
		}
		vDir := r.VersionDir()
		dest, e := safeJoin(vDir, folder+"/"+f.Name)
		if e != nil {
			return e
		}
		p("正在下载 "+f.Name, 0, 1)
		if e = s.download(ctx, f, dest); e != nil {
			return e
		}
		p("已安装 "+f.Name, 1, 1)
		return nil
	})
}
