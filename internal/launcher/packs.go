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
		if e := s.downloadPackFiles(ctx, root, &lock, p); e != nil {
			return e
		}
		if e := applyPackOverrides(ctx, z, root, overrides, &lock); e != nil {
			return e
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
