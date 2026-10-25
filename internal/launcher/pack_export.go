package launcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	maxPackExportFiles = 39998                   // leave room for the manifest and overrides directory
	maxPackExportBytes = (12 << 30) - (16 << 20) // reserve the bounded manifest
)

// ExportPackRequest captures the version and content selection when the user
// confirms the export. A nil Entries uses the preview defaults; an empty slice
// deliberately exports only the game/loader manifest.
type ExportPackRequest struct {
	RootID      string   `json:"rootId"`
	Version     string   `json:"version"`
	Format      string   `json:"format"` // modrinth | curseforge
	Name        string   `json:"name"`
	PackVersion string   `json:"packVersion"`
	Author      string   `json:"author"`
	Path        string   `json:"path"`
	Entries     []string `json:"entries"`
	Overwrite   bool     `json:"overwrite"`
}

type PackExportEntry struct {
	Name     string `json:"name"`
	Folder   bool   `json:"folder"`
	Selected bool   `json:"selected"`
}

type PackExportPreview struct {
	Name          string            `json:"name"`
	PackVersion   string            `json:"packVersion"`
	Minecraft     string            `json:"minecraft"`
	Loader        string            `json:"loader"`
	LoaderVersion string            `json:"loaderVersion"`
	Entries       []PackExportEntry `json:"entries"`
}

// PreviewPackExport only enumerates top-level choices. It does not parse jars,
// hash files, or load the instance's mod inventory into memory.
func (s *Service) PreviewPackExport(ctx context.Context, rootID, version string) (PackExportPreview, error) {
	r, err := s.rootAt(rootID, version)
	if err != nil {
		return PackExportPreview{}, err
	}
	preview, err := packExportVersion(r)
	if err != nil {
		return preview, err
	}
	dir, err := os.OpenRoot(r.VersionDir())
	if err != nil {
		return preview, err
	}
	defer dir.Close()
	preview.Entries, err = packExportEntries(ctx, dir, r.SelectedVersion)
	return preview, err
}

func packExportVersion(r Root) (PackExportPreview, error) {
	for _, v := range scanVersions(r.Path) {
		if v.ID != r.SelectedVersion {
			continue
		}
		switch v.Loader {
		case "vanilla", "fabric", "quilt", "forge", "neoforge":
		default:
			return PackExportPreview{}, fmt.Errorf("不支持导出此加载器：%s", v.Loader)
		}
		if v.Minecraft == "" || v.Loader != "vanilla" && v.LoaderVersion == "" {
			return PackExportPreview{}, errors.New("无法识别游戏或加载器版本，请先修复此实例")
		}
		out := PackExportPreview{Name: r.Name, PackVersion: "1.0.0", Minecraft: v.Minecraft, Loader: v.Loader, LoaderVersion: v.LoaderVersion}
		if c := r.VersionConfigs[v.ID]; c.DisplayName != "" {
			out.Name = c.DisplayName
		}
		if lock := readPack(r.Path); lock != nil && (lock.GameVersion == "" || lock.GameVersion == v.ID) {
			if lock.Name != "" {
				out.Name = lock.Name
			}
			if lock.Version != "" {
				out.PackVersion = lock.Version
			}
		}
		return out, nil
	}
	return PackExportPreview{}, errors.New("请先选择一个已安装的游戏版本")
}

// Launcher state and generated files are never exportable. Personal game data
// remains available as an explicit choice, separate from the default pack.
func packExportExcluded(name, version string) bool {
	n := strings.ToLower(name)
	if strings.HasPrefix(n, ".") || strings.EqualFold(name, version+".json") || strings.EqualFold(name, version+".jar") {
		return true
	}
	switch n {
	case "assets", "libraries", "versions", "natives", "runtime", "java", "cache", "caches", "downloads", "webcache",
		"logs", "crash-reports", "backups", "launcher_profiles.json", "launcher_accounts.json", "launcher_settings.json",
		"accounts.json", "credentials.json", "launcher_accounts_microsoft_store.json", "launcher_msa_credentials.bin", "hmcl.json", "pcl", "pcl2",
		"usercache.json", "usernamecache.json", "session.lock", "manifest.json", "modrinth.index.json", "instance.cfg", "mmc-pack.json":
		return true
	}
	switch strings.ToLower(filepath.Ext(n)) {
	case ".exe", ".dll", ".so", ".dylib", ".log", ".tmp", ".part", ".mrpack", ".bat", ".cmd", ".ps1", ".sh":
		return true
	}
	return false
}

func packExportDefault(name string) bool {
	n := strings.ToLower(name)
	switch n {
	case "saves", "screenshots", "recordings", "replay_recordings", "replay_videos", "schematics", "litematica", "xaerowaypoints", "xaeroworldmap", "journeymap",
		"servers.dat", "servers.dat_old", "hotbar.nbt", "realms_persistence.json":
		return false
	}
	return !strings.HasPrefix(n, "options")
}

func packExportEntries(ctx context.Context, dir *os.Root, version string) ([]PackExportEntry, error) {
	f, err := dir.Open(".")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(maxPackExportFiles + 1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(entries) > maxPackExportFiles {
		return nil, errors.New("整合包文件数量超过导出限制")
	}
	choices := make([]PackExportEntry, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if packExportExcluded(entry.Name(), version) || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		if !entry.IsDir() && !entry.Type().IsRegular() {
			continue
		}
		choices = append(choices, PackExportEntry{Name: entry.Name(), Folder: entry.IsDir(), Selected: packExportDefault(entry.Name())})
	}
	slices.SortFunc(choices, func(a, b PackExportEntry) int {
		if a.Folder != b.Folder {
			if a.Folder {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	return choices, nil
}

func (s *Service) ExportPack(req ExportPackRequest) (string, error) {
	r, err := s.rootAt(req.RootID, req.Version)
	if err != nil {
		return "", err
	}
	if req.Format != "modrinth" && req.Format != "curseforge" {
		return "", errors.New("请选择 Modrinth 或 CurseForge 导出格式")
	}
	req.Name, req.PackVersion, req.Author = strings.TrimSpace(req.Name), strings.TrimSpace(req.PackVersion), strings.TrimSpace(req.Author)
	if req.Name == "" || req.PackVersion == "" || len(req.Name) > 256 || len(req.PackVersion) > 128 || len(req.Author) > 256 {
		return "", errors.New("请填写整合包名称和版本号（名称最多 256 字节，版本最多 128 字节）")
	}
	if strings.TrimSpace(req.Path) == "" {
		return "", errors.New("请选择导出文件的位置")
	}
	req.Path, err = filepath.Abs(req.Path)
	if err != nil {
		return "", err
	}
	ext := ".mrpack"
	if req.Format == "curseforge" {
		ext = ".zip"
	}
	if !strings.EqualFold(filepath.Ext(req.Path), ext) {
		return "", fmt.Errorf("导出文件的扩展名应为 %s", ext)
	}
	// Freeze the request: callers may change their checkboxes after submission.
	req.Entries = slices.Clone(req.Entries)
	req.Version = r.SelectedVersion
	return s.startJob("导出整合包："+req.Name, r.ID, func(ctx context.Context, p progress) error {
		s.mu.Lock()
		if j := jobOf(ctx); j != nil {
			j.TargetRootID, j.TargetVersion = r.ID, r.SelectedVersion
		}
		s.mu.Unlock()
		return s.exportPack(ctx, r, req, p)
	})
}

type packExportFile struct {
	path         string
	size         int64
	mode         fs.FileMode
	mtime        time.Time
	sha1, sha512 string
	mr           *mrPackFile
	cf           *cfPackFile
}

func collectPackExportFiles(ctx context.Context, dir *os.Root, version string, names []string) ([]packExportFile, error) {
	choices, err := packExportEntries(ctx, dir, version)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]bool, len(choices))
	useDefaults := names == nil
	for _, choice := range choices {
		allowed[choice.Name] = true
		if useDefaults && choice.Selected {
			names = append(names, choice.Name)
		}
	}
	names = slices.Clone(names)
	slices.Sort(names)
	names = slices.Compact(names)
	var files []packExportFile
	var total int64
	seen := map[string]bool{}
	for _, name := range names {
		if !allowed[name] {
			return nil, fmt.Errorf("不能导出此文件或目录：%s", name)
		}
		err := fs.WalkDir(dir.FS(), name, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("导出内容包含符号链接，请取消选择或移除链接：%s", path)
			}
			// Top-level runtime/cache exclusions must not remove pack content
			// such as kubejs/assets, scripts/libraries or a resource pack's assets.
			if d.Name() == ".mcl" || d.Name() == ".git" || d.Name() == "session.lock" {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if _, err := safeJoin(dir.Name(), path); err != nil {
				return err
			}
			st, err := d.Info()
			if err != nil {
				return err
			}
			if !st.Mode().IsRegular() {
				return fmt.Errorf("不能导出特殊文件：%s", path)
			}
			total += st.Size()
			if len(files) >= maxPackExportFiles || total > maxPackExportBytes {
				return errors.New("整合包超过导出限制（40,000 个文件 / 12 GiB）")
			}
			key := strings.ToLower(path)
			if seen[key] {
				return fmt.Errorf("文件名大小写冲突：%s", path)
			}
			seen[key] = true
			files = append(files, packExportFile{path: path, size: st.Size(), mode: st.Mode(), mtime: st.ModTime()})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}
