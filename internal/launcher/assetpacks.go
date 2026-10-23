package launcher

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

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
	return s.ListAssetPacksAt(rootID, "", kind)
}

// ListAssetPacksAt lists versionID's resource or shader packs.
func (s *Service) ListAssetPacksAt(rootID, versionID, kind string) ([]AssetPack, error) {
	r, e := s.rootAt(rootID, versionID)
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
