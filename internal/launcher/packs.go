package launcher

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ImportRequest struct {
	Path      string `json:"path"`
	Parent    string `json:"parent"`
	Name      string `json:"name"`
	Source    string `json:"source"`
	ProjectID string `json:"projectId"`
	VersionID string `json:"versionId"`
}
type mrPack struct {
	FormatVersion int               `json:"formatVersion"`
	Game          string            `json:"game"`
	VersionID     string            `json:"versionId"`
	Name          string            `json:"name"`
	Dependencies  map[string]string `json:"dependencies"`
	Files         []struct {
		Path   string `json:"path"`
		Hashes struct {
			SHA1   string `json:"sha1"`
			SHA512 string `json:"sha512"`
		} `json:"hashes"`
		Env struct {
			Client string `json:"client"`
		} `json:"env"`
		Downloads []string `json:"downloads"`
		Size      int64    `json:"fileSize"`
	} `json:"files"`
}
type cfPack struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	ManifestType    string `json:"manifestType"`
	ManifestVersion int    `json:"manifestVersion"`
	Overrides       string `json:"overrides"`
	Minecraft       struct {
		Version string `json:"version"`
		Loaders []struct {
			ID      string `json:"id"`
			Primary bool   `json:"primary"`
		} `json:"modLoaders"`
	} `json:"minecraft"`
	Files []struct {
		ProjectID int  `json:"projectID"`
		FileID    int  `json:"fileID"`
		Required  bool `json:"required"`
	} `json:"files"`
}

func parsePack(z *zip.ReadCloser) (PackLock, string, error) {
	for _, f := range z.File {
		if f.Name == "modrinth.index.json" {
			var index mrPack
			if e := readZipJSON(f, &index); e != nil {
				return PackLock{}, "", e
			}
			if index.FormatVersion != 1 || index.Game != "minecraft" {
				return PackLock{}, "", errors.New("不支持此 Modrinth 整合包格式")
			}
			lock := PackLock{Name: index.Name, Version: index.VersionID, Source: "modrinth", Minecraft: index.Dependencies["minecraft"], Loader: "vanilla", Files: []PackFile{}, Manual: []ManualDownload{}}
			for key, loader := range map[string]string{"fabric-loader": "fabric", "quilt-loader": "quilt", "forge": "forge", "neoforge": "neoforge"} {
				if v := index.Dependencies[key]; v != "" {
					if lock.Loader != "vanilla" {
						return lock, "", errors.New("整合包声明了多个加载器")
					}
					lock.Loader = loader
					lock.LoaderVersion = v
				}
			}
			for _, file := range index.Files {
				if file.Env.Client == "unsupported" {
					continue
				}
				if len(file.Downloads) == 0 || len(file.Hashes.SHA1) != 40 || len(file.Hashes.SHA512) != 128 {
					return lock, "", errors.New("整合包缺少下载地址或文件校验值")
				}
				lock.Files = append(lock.Files, PackFile{Path: file.Path, SHA1: file.Hashes.SHA1, SHA512: file.Hashes.SHA512, Source: "modrinth", Kind: "remote", Size: file.Size, URLs: file.Downloads})
			}
			if lock.Minecraft == "" {
				return lock, "", errors.New("整合包未指定 Minecraft 版本")
			}
			return lock, "overrides", nil
		}
	}
	for _, f := range z.File {
		if f.Name == "manifest.json" {
			var index cfPack
			if e := readZipJSON(f, &index); e != nil {
				return PackLock{}, "", e
			}
			if index.ManifestType != "minecraftModpack" || index.ManifestVersion != 1 {
				return PackLock{}, "", errors.New("不是受支持的 CurseForge 整合包")
			}
			lock := PackLock{Name: index.Name, Version: index.Version, Source: "curseforge", Minecraft: index.Minecraft.Version, Loader: "vanilla", Files: []PackFile{}, Manual: []ManualDownload{}}
			for i, loader := range index.Minecraft.Loaders {
				if !loader.Primary && i > 0 {
					continue
				}
				for _, kind := range []string{"neoforge", "forge", "fabric", "quilt"} {
					if strings.HasPrefix(loader.ID, kind+"-") {
						lock.Loader = kind
						lock.LoaderVersion = strings.TrimPrefix(loader.ID, kind+"-")
						break
					}
				}
				if loader.Primary {
					break
				}
			}
			for _, f := range index.Files {
				lock.Files = append(lock.Files, PackFile{Source: "curseforge", ProjectID: fmt.Sprint(f.ProjectID), VersionID: fmt.Sprint(f.FileID)})
			}
			if lock.Minecraft == "" {
				return lock, "", errors.New("整合包未指定 Minecraft 版本")
			}
			return lock, index.Overrides, nil
		}
	}
	if lock, overrides, ok, e := parseMultiMC(z); ok || e != nil {
		return lock, overrides, e
	}
	return PackLock{}, "", errors.New("压缩包根目录中找不到 modrinth.index.json、CurseForge manifest.json 或 MultiMC instance.cfg")
}
func (s *Service) InspectPack(path string) (PackPreview, error) {
	p, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return PackPreview{}, err
	}
	z, err := zip.OpenReader(p)
	if err != nil {
		return PackPreview{}, fmt.Errorf("无法读取整合包压缩文件: %w", err)
	}
	defer z.Close()
	lock, _, err := parsePack(z)
	if err != nil {
		return PackPreview{}, err
	}
	name := strings.TrimSpace(lock.Name)
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	}
	return PackPreview{
		Path:          p,
		Name:          name,
		Version:       lock.Version,
		Source:        lock.Source,
		Minecraft:     lock.Minecraft,
		Loader:        lock.Loader,
		LoaderVersion: lock.LoaderVersion,
		FileCount:     len(lock.Files),
		ModCount:      countMods(lock),
		DownloadMB:    estimateDownloadMB(lock, z),
		Unavailable:   s.packUnavailable(lock),
	}, nil
}
func (s *Service) ImportPack(req ImportRequest) (string, error) {
	if req.Name == "" || filepath.Base(req.Name) != req.Name {
		return "", errors.New("请输入新整合包文件夹名称")
	}
	return s.startJob("下载整合包："+req.Name, "", func(ctx context.Context, p progress) error {
		path := req.Path
		if path == "" {
			v, e := s.projectVersion(ctx, req.Source, req.ProjectID, req.VersionID)
			if e != nil {
				return e
			}
			if v.ProjectID != req.ProjectID {
				return errors.New("整合包项目与版本不匹配")
			}
			f, e := primaryFile(v)
			if e != nil {
				return e
			}
			if f.URL == "" {
				return errors.New("作者未开放第三方下载，请从 CurseForge 网站下载整合包后导入")
			}
			path = filepath.Join(s.dir, "cache", jobKey(ctx)+".pack")
			s.trackCreated(ctx, path)
			p("下载整合包", 0, 0)
			if e = s.download(ctx, f, path); e != nil {
				return e
			}
		}
		parent, e := filepath.Abs(req.Parent)
		if e != nil {
			return e
		}
		parent, e = filepath.EvalSymlinks(parent)
		if e != nil {
			return errors.New("请选择已存在的存放文件夹")
		}
		dest, e := safeJoin(parent, req.Name)
		if e != nil {
			return e
		}
		if _, e = os.Stat(dest); !os.IsNotExist(e) {
			return errors.New("目标文件夹已存在，请使用新名称")
		}
		// The stage is named after the task so an in-place retry reuses
		// what was already downloaded; cancel or 移除 deletes it.
		stage, e := safeJoin(parent, ".mcl-import-"+jobKey(ctx))
		if e != nil {
			return e
		}
		s.trackCreated(ctx, stage)
		if e = os.MkdirAll(stage, 0755); e != nil {
			return e
		}
		root := filepath.Join(stage, ".minecraft")
		if e = os.MkdirAll(root, 0755); e != nil {
			return e
		}
		z, e := zip.OpenReader(path)
		if e != nil {
			return fmt.Errorf("无法读取整合包压缩文件: %w", e)
		}
		defer z.Close()
		if len(z.File) > 40000 {
			return errors.New("整合包文件数量超过限制")
		}
		var total uint64
		for _, f := range z.File {
			total += f.UncompressedSize64
			if total > 12<<30 {
				return errors.New("整合包解压大小超过 12 GiB")
			}
		}
		lock, overrides, e := parsePack(z)
		if e != nil {
			return e
		}
		var want int64
		for _, f := range lock.Files {
			want += f.Size
		}
		setJobBytesTotal(ctx, want)
		if why := s.packUnavailable(lock); why != "" {
			return errors.New(why)
		}
		lock.ProjectID = req.ProjectID
		lock.VersionID = req.VersionID
		s.stampLock(ctx, &lock, path, req.Path != "")
		lock.History = []PackHistory{{Version: lock.Version, At: lock.ImportedAt, Action: "import"}}
		seen := map[string]bool{}
		for i := range lock.Files {
			f := &lock.Files[i]
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if f.Source == "curseforge" {
				p(fmt.Sprintf("解析整合包模组 (%d/%d): 项目 #%s", i+1, len(lock.Files), f.ProjectID), i, len(lock.Files))
				v, e := s.projectVersion(ctx, "curseforge", f.ProjectID, f.VersionID)
				if e != nil {
					return e
				}
				df, e := primaryFile(v)
				if e != nil {
					return e
				}
				f.Path = "mods/" + df.Name
				f.SHA1 = df.SHA1
				if df.URL == "" {
					var project struct {
						Data struct {
							Name  string `json:"name"`
							Links struct {
								WebsiteURL string `json:"websiteUrl"`
							} `json:"links"`
						} `json:"data"`
					}
					if e = s.cfJSON(ctx, "GET", "/mods/"+f.ProjectID, nil, &project); e != nil {
						return e
					}
					lock.Manual = append(lock.Manual, ManualDownload{ProjectID: f.ProjectID, FileID: f.VersionID, Name: df.Name, URL: project.Data.Links.WebsiteURL + "/files/" + f.VersionID})
					continue
				}
				f.URLs = []string{df.URL}
			}
			if strings.HasPrefix(strings.ToLower(strings.ReplaceAll(f.Path, "\\", "/")), ".mcl/") {
				return errors.New("整合包不能写入启动器元数据目录")
			}
			target, e := safeJoin(root, f.Path)
			if e != nil {
				return e
			}
			key := strings.ToLower(target)
			if seen[key] {
				return errors.New("整合包包含重复文件路径")
			}
			seen[key] = true
			p(fmt.Sprintf("下载整合包文件 (%d/%d): %s", i+1, len(lock.Files), filepath.Base(f.Path)), i+1, len(lock.Files))
			var last error
			for _, raw := range f.URLs {
				last = s.download(ctx, DownloadFile{Name: f.Path, URL: raw, SHA1: f.SHA1, SHA512: f.SHA512}, target)
				if last == nil {
					break
				}
			}
			if last != nil {
				return last
			}
			if len(f.URLs) == 0 {
				return errors.New("整合包文件缺少下载地址")
			}
		}
		// Client overrides are deliberately applied after common overrides, as specified by mrpack.
		prefixes := []string{strings.Trim(overrides, "/")}
		if lock.Source == "modrinth" {
			prefixes = append(prefixes, "client-overrides")
		}
		for _, prefix := range prefixes {
			if prefix == "" {
				continue
			}
			for _, f := range z.File {
				if !strings.HasPrefix(f.Name, prefix+"/") {
					continue
				}
				name := strings.TrimPrefix(f.Name, prefix+"/")
				if name == "" {
					continue
				}
				if strings.HasPrefix(strings.ToLower(name), ".mcl/") {
					return errors.New("整合包覆盖文件包含保留目录")
				}
				target, e := safeJoin(root, name)
				if e != nil {
					return e
				}
				if f.FileInfo().IsDir() {
					if e = os.MkdirAll(target, 0755); e != nil {
						return e
					}
					continue
				}
				if _, e = os.Stat(target); e == nil {
					if e = os.Remove(target); e != nil {
						return e
					}
				}
				if e = extractZipEntry(f, target); e != nil {
					return e
				}
				sha, sha512, e := fileHashes(target)
				if e != nil {
					return e
				}
				found := false
				for i, existing := range lock.Files {
					if strings.EqualFold(existing.Path, name) {
						lock.Files[i].SHA1 = sha
						lock.Files[i].SHA512 = sha512
						lock.Files[i].Kind = overrideKind(prefix)
						found = true
						break
					}
				}
				if !found {
					lock.Files = append(lock.Files, PackFile{Path: name, SHA1: sha, SHA512: sha512, Source: "override", Kind: overrideKind(prefix)})
				}
			}
		}
		if _, e = s.installVanilla(ctx, root, lock.Minecraft, p); e != nil {
			return e
		}
		id, e := s.installLoader(ctx, root, lock.Minecraft, lock.Loader, lock.LoaderVersion, p)
		if e != nil {
			return e
		}
		// Download the whole game now (client, libraries, assets) so the
		// first launch starts immediately.
		gv, e := s.resolveVersion(ctx, root, id, p)
		if e != nil {
			return e
		}
		if _, _, e = s.prepareVersion(ctx, root, gv, p); e != nil {
			return e
		}
		_ = os.RemoveAll(filepath.Join(root, ".mcl", "natives"))
		lock.GameVersion = id
		if e = atomicJSON(filepath.Join(root, ".mcl", "pack-lock.json"), lock); e != nil {
			return e
		}
		s.savePackIcon(ctx, lock.Source, lock.ProjectID, root)
		records := map[string]modRecord{}
		for _, f := range lock.Files {
			if f.Source == "curseforge" && strings.HasPrefix(f.Path, "mods/") {
				records[strings.TrimPrefix(f.Path, "mods/")] = modRecord{SHA1: f.SHA1, Source: f.Source, ProjectID: f.ProjectID, VersionID: f.VersionID}
			}
		}
		if e = atomicJSON(filepath.Join(root, ".mcl", "mods.json"), records); e != nil {
			return e
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s.trackCreated(ctx, dest)
		if e = os.Rename(stage, dest); e != nil {
			return e
		}
		r, e := s.AddRoot(dest, req.Name)
		if e != nil {
			return e
		}
		if id := r.ID; id != "" {
			s.onUndo(ctx, func() { _ = s.RemoveRoot(id) })
		}
		s.setJobResultRoot(ctx, r.ID)
		if req.Path == "" {
			_ = os.Remove(path) // the downloaded pack file was only a carrier
		}
		p("导入完成", 0, 0)
		return s.Select(r.ID, "", id)
	})
}

type PackFileStatus struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}
type PackReport struct {
	Pack   *PackLock        `json:"pack"`
	Files  []PackFileStatus `json:"files"`
	Latest *ProjectVersion  `json:"latest,omitempty"`
}

func (s *Service) CheckPack(ctx context.Context, rootID string) (PackReport, error) {
	r, e := s.getRoot(rootID)
	if e != nil {
		return PackReport{}, e
	}
	lock := readPack(r.Path)
	if lock == nil {
		return PackReport{}, errors.New("此目录没有整合包导入记录")
	}
	out := PackReport{Pack: lock, Files: []PackFileStatus{}}
	remaining := []ManualDownload{}
	changed := false
	for _, f := range lock.Files {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		path, e := safeJoin(r.Path, f.Path)
		if e != nil {
			return out, e
		}
		status := "original"
		if _, e = os.Stat(path); os.IsNotExist(e) {
			if _, e = os.Stat(path + ".disabled"); e == nil {
				path += ".disabled"
				status = "disabled"
			} else {
				status = "missing"
			}
		}
		if status != "missing" && !checkHashes(path, f.SHA1, f.SHA512) {
			status = "modified"
		}
		out.Files = append(out.Files, PackFileStatus{Path: f.Path, Status: status})
	}
	for _, manual := range lock.Manual {
		path, e := safeJoin(r.Path, "mods/"+manual.Name)
		if e != nil {
			return out, e
		}
		found := false
		for _, f := range lock.Files {
			if f.ProjectID == manual.ProjectID && f.VersionID == manual.FileID && checkHashes(path, f.SHA1, f.SHA512) {
				found = true
				break
			}
		}
		if !found && manual.SHA1 != "" && checkHashes(path, manual.SHA1, "") {
			// The player supplied a manual file that replaces an older copy
			// kept by an update: retire the old one now.
			found = true
			replaced := false
			for i, f := range lock.Files {
				if f.Source == "curseforge" && f.ProjectID == manual.ProjectID {
					if !strings.EqualFold(f.Path, "mods/"+manual.Name) {
						if oldPath, e := safeJoin(r.Path, f.Path); e == nil {
							_ = os.Remove(oldPath)
							_ = os.Remove(oldPath + ".disabled")
						}
					}
					lock.Files[i] = PackFile{Kind: "remote", Path: "mods/" + manual.Name, SHA1: manual.SHA1, Source: "curseforge", ProjectID: manual.ProjectID, VersionID: manual.FileID}
					replaced = true
				}
			}
			if !replaced {
				lock.Files = append(lock.Files, PackFile{Kind: "remote", Path: "mods/" + manual.Name, SHA1: manual.SHA1, Source: "curseforge", ProjectID: manual.ProjectID, VersionID: manual.FileID})
			}
		}
		if found {
			changed = true
		} else {
			remaining = append(remaining, manual)
		}
	}
	if changed {
		lock.Manual = remaining
		if e = s.mutateRoot(rootID, func(r Root) error { return atomicJSON(filepath.Join(r.Path, ".mcl", "pack-lock.json"), lock) }); e != nil {
			return out, e
		}
	}
	if lock.ProjectID != "" {
		info, e := s.CheckPackUpdate(ctx, rootID)
		if e != nil {
			return out, e
		}
		if info.HasUpdate {
			out.Latest = info.Latest
		}
	}
	return out, nil
}

// PreviewRemotePack reads a catalog modpack version's index so the install
// dialog can show its mod count and download size before anything is
// installed. The pack file is cached by version and only fetched when
// small (indexes are kilobytes; packs with bundled files are skipped).
func (s *Service) PreviewRemotePack(ctx context.Context, source, projectID, versionID string) (PackPreview, error) {
	v, e := s.projectVersion(ctx, source, projectID, versionID)
	if e != nil {
		return PackPreview{}, e
	}
	f, e := primaryFile(v)
	if e != nil {
		return PackPreview{}, e
	}
	if f.URL == "" || f.Size > 32<<20 {
		return PackPreview{}, errors.New("整合包较大，下载后才能读取内容")
	}
	dir := filepath.Join(s.dir, "cache", "pack-preview")
	if e := os.MkdirAll(dir, 0o755); e != nil {
		return PackPreview{}, e
	}
	path, e := safeJoin(dir, previewName(source+"-"+versionID)+".pack")
	if e != nil {
		return PackPreview{}, e
	}
	if st, e := os.Stat(path); e != nil || (f.Size > 0 && st.Size() != f.Size) {
		if e := s.download(ctx, f, path); e != nil {
			return PackPreview{}, e
		}
	}
	return s.InspectPack(path)
}

func previewName(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, s)
}

func countMods(l PackLock) int {
	n := 0
	for _, f := range l.Files {
		if strings.HasPrefix(normPath(f.Path), "mods/") {
			n++
		}
	}
	return n
}
