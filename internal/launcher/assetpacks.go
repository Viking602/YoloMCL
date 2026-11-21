package launcher

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func isPackArchive(kind, low string) bool {
	if strings.HasSuffix(low, ".zip") || strings.HasSuffix(low, ".zip.disabled") {
		return true
	}
	return kind == "shader" && (strings.HasSuffix(low, ".hshader") || strings.HasSuffix(low, ".hshader.disabled"))
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
	st, err := os.Stat(path)
	if err != nil {
		return ""
	}
	var meta struct {
		Pack struct {
			Description any `json:"description"`
		} `json:"pack"`
	}
	if st.IsDir() {
		b, err := os.ReadFile(filepath.Join(path, "pack.mcmeta"))
		if err != nil || len(b) == 0 || len(b) > 1<<20 {
			return ""
		}
		if json.Unmarshal(b, &meta) != nil {
			return ""
		}
		return packDescriptionText(meta.Pack.Description)
	}
	r, e := zip.OpenReader(path)
	if e != nil {
		return ""
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == "pack.mcmeta" && f.UncompressedSize64 <= 1<<20 {
			if readZipJSON(f, &meta) == nil {
				return packDescriptionText(meta.Pack.Description)
			}
		}
	}
	return ""
}

// packDescriptionText pulls plain text from pack.mcmeta description: a string,
// a text component {"text":"..."}, or an array of those (vanilla / many packs).
// translate-only components are skipped (no locale table here).
func packDescriptionText(v any) string {
	return strings.TrimSpace(packDescriptionTextRaw(v))
}

func packDescriptionTextRaw(v any) string {
	switch t := v.(type) {
	case string:
		return stripFormattingKeepSpace(t)
	case map[string]any:
		if s, ok := t["text"].(string); ok {
			return stripFormattingKeepSpace(s)
		}
		if extra, ok := t["extra"].([]any); ok {
			return packDescriptionTextRaw(extra)
		}
	case []any:
		var b strings.Builder
		for _, e := range t {
			b.WriteString(packDescriptionTextRaw(e))
		}
		return b.String()
	}
	return ""
}

// stripFormattingKeepSpace drops § codes but keeps leading/trailing spaces so
// adjacent text-component pieces still join correctly.
func stripFormattingKeepSpace(s string) string {
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
	return out.String()
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
		full, e := safeJoin(vDir, folder+"/"+name)
		if e != nil {
			continue
		}
		st, e := os.Stat(full)
		if e != nil {
			continue
		}
		if entry.IsDir() {
			// Folder resource packs (pack.mcmeta at root) or Horizon folder shaders (horizon-shaders.toml).
			var desc string
			switch kind {
			case "resourcepack":
				if _, err := os.Stat(filepath.Join(full, "pack.mcmeta")); err != nil {
					continue
				}
				desc = readPackMcmeta(full)
			case "shader":
				if _, err := os.Stat(filepath.Join(full, "horizon-shaders.toml")); err != nil {
					continue
				}
				desc = readHorizonShaderDescription(full)
			default:
				continue
			}
			out = append(out, AssetPack{
				File:        name,
				Name:        name,
				Description: desc,
				Kind:        kind,
				Enabled:     true, // enablement comes from options.txt at preview time
				Size:        st.Size(),
			})
			continue
		}
		low := strings.ToLower(name)
		if !isPackArchive(kind, low) {
			continue
		}
		base := strings.TrimSuffix(name, ".disabled")
		clean := strings.TrimSuffix(base, filepath.Ext(base))
		desc := ""
		if kind == "resourcepack" {
			desc = readPackMcmeta(full)
		} else if kind == "shader" && strings.HasSuffix(strings.ToLower(base), ".hshader") {
			desc = readHorizonShaderDescription(full)
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
		if filepath.Base(name) != name || !isPackArchive(kind, low) {
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
		if filepath.Base(name) != name || !isPackArchive(kind, low) {
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
			ext := strings.ToLower(filepath.Ext(src))
			if ext != ".zip" && !(kind == "shader" && ext == ".hshader") {
				if kind == "shader" {
					return errors.New("只能导入 .zip 或 .hshader 格式的光影包")
				}
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
		if source == "horizon" {
			_, proj, ver, err := s.horizonLookup(ctx, project, id)
			if err != nil {
				return err
			}
			if horizonDiscoverType(proj.Kind) != kind {
				return horizonErr("所选 Horizon 资源类型与目标分类不匹配", "Selected Horizon asset kind does not match target category")
			}
			exts := []string{".zip"}
			if kind == "shader" {
				exts = []string{".hshader", ".zip"}
			}
			f, err := s.verifyHorizonRegistryDownload(proj, ver, exts...)
			if err != nil {
				return err
			}
			if strings.HasSuffix(strings.ToLower(f.Name), ".hshader") && !isHorizonRootVersion(r) {
				return horizonErr("Horizon 光影（.hshader）仅适用于 Horizon 实例", "Horizon shaders (.hshader) can only be installed into a Horizon instance")
			}
			vDir := r.VersionDir()
			dest, err := safeJoin(vDir, folder+"/"+f.Name)
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
			staged, err := safeJoin(stage, f.Name)
			if err != nil {
				return err
			}
			p("正在下载 "+f.Name, 0, 1)
			if err := s.download(ctx, f, staged); err != nil {
				return err
			}
			if err := s.verifyDownloadedMinisign(proj.Key, ver.File.Signature, staged); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			_ = os.Remove(dest)
			if err := os.Rename(staged, dest); err != nil {
				return err
			}
			s.invalidateStatus(vDir)
			p("已安装 "+f.Name, 1, 1)
			return nil
		}
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
