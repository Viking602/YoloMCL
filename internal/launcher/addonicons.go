package launcher

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// AddonIcon returns the embedded icon of an installed mod, resource pack or
// shader pack: fabric/quilt "icon", (neo)forge mods.toml logoFile, or
// pack.png. Folders are read the same way. kind is "mod", "resourcepack"
// or "shader".
func (s *Service) AddonIcon(rootID, versionID, kind, file string) ([]byte, error) {
	r, e := s.rootAt(rootID, versionID)
	if e != nil {
		return nil, e
	}
	sub := "mods"
	if kind != "mod" {
		if sub, e = packFolder(kind); e != nil {
			return nil, e
		}
	}
	p, e := safeJoin(r.VersionDir(), sub+"/"+file)
	if e != nil {
		return nil, e
	}
	if st, e := os.Stat(p); e == nil && st.IsDir() {
		for _, n := range []string{"pack.png", "icon.png", "logo.png"} {
			if b, e := readCapped(filepath.Join(p, n)); e == nil {
				return b, nil
			}
		}
		return nil, errors.New("no icon")
	}
	z, e := zip.OpenReader(p)
	if e != nil {
		return nil, e
	}
	defer z.Close()
	files := map[string]*zip.File{}
	for _, f := range z.File {
		files[strings.TrimPrefix(f.Name, "/")] = f
	}
	read := func(name string) ([]byte, bool) {
		f := files[strings.TrimPrefix(path.Clean("/"+name), "/")]
		if f == nil || f.UncompressedSize64 > 4<<20 {
			return nil, false
		}
		rc, e := f.Open()
		if e != nil {
			return nil, false
		}
		defer rc.Close()
		b, e := io.ReadAll(io.LimitReader(rc, 4<<20))
		return b, e == nil && len(b) > 0
	}
	var candidates []string
	if b, ok := read("fabric.mod.json"); ok {
		var m struct{ Icon json.RawMessage }
		if json.Unmarshal(b, &m) == nil {
			candidates = append(candidates, iconField(m.Icon))
		}
	}
	if b, ok := read("quilt.mod.json"); ok {
		var m struct {
			QuiltLoader struct {
				Metadata struct{ Icon json.RawMessage } `json:"metadata"`
			} `json:"quilt_loader"`
		}
		if json.Unmarshal(b, &m) == nil {
			candidates = append(candidates, iconField(m.QuiltLoader.Metadata.Icon))
		}
	}
	for _, toml := range []string{"META-INF/neoforge.mods.toml", "META-INF/mods.toml"} {
		if b, ok := read(toml); ok {
			if m := logoFileRE.FindSubmatch(b); m != nil {
				candidates = append(candidates, string(m[1]))
			}
		}
	}
	candidates = append(candidates, "pack.png", "icon.png", "logo.png")
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if b, ok := read(c); ok {
			return b, nil
		}
	}
	return nil, errors.New("no icon")
}

var logoFileRE = regexp.MustCompile(`(?m)^\s*logoFile\s*=\s*["']([^"']+)["']`)

// iconField reads fabric's icon: a path, or {"16": path, "128": path}
// (largest wins).
func iconField(raw json.RawMessage) string {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return one
	}
	var many map[string]string
	if json.Unmarshal(raw, &many) != nil || len(many) == 0 {
		return ""
	}
	best, bestN := "", -1
	for k, v := range many {
		if n, _ := strconv.Atoi(k); n > bestN {
			best, bestN = v, n
		}
	}
	return best
}

func readCapped(p string) ([]byte, error) {
	st, e := os.Stat(p)
	if e != nil {
		return nil, e
	}
	if st.Size() > 4<<20 {
		return nil, errors.New("icon too large")
	}
	return os.ReadFile(p)
}

// InstanceStatus is what the overview shows about the selected version's
// game folder; every field comes from disk.
type InstanceStatus struct {
	Mods          int                 `json:"mods"`
	EnabledMods   int                 `json:"enabledMods"`
	Dir           string              `json:"dir"`
	ResourcePacks int                 `json:"resourcePacks"`
	Shaders       int                 `json:"shaders"`
	Saves         int                 `json:"saves"`
	Screenshots   int                 `json:"screenshots"`
	LastLog       time.Time           `json:"lastLog,omitempty"`
	LastCrash     time.Time           `json:"lastCrash,omitempty"`
	CrashFile     string              `json:"crashFile,omitempty"`
	RecentWorld   *World              `json:"recentWorld,omitempty"`
	RecentServer  *InstanceServer     `json:"recentServer,omitempty"`
	LatestShot    *InstanceScreenshot `json:"latestShot,omitempty"`
	// Sizes are bytes per sub folder (mods, resourcepacks, shaderpacks,
	// saves, screenshots); Total covers Minecraft data and launcher metadata,
	// never unrelated files sharing the instance directory.
	Sizes          map[string]int64 `json:"sizes,omitempty"`
	Total          int64            `json:"total"`
	Play           PlayStats        `json:"play"`
	StoragePending bool             `json:"storagePending,omitempty"`
}

func (s *Service) InstanceStatus(rootID, versionID string) (InstanceStatus, error) {
	return s.InstanceStatusContext(context.Background(), rootID, versionID)
}

// InstanceStatusContext returns the complete summary for callers that need
// exact sizes. The UI reads InstanceOverviewContext first so a large directory
// can never hold its basic information behind the storage walk.
func (s *Service) InstanceStatusContext(ctx context.Context, rootID, versionID string) (InstanceStatus, error) {
	st, err := s.InstanceOverviewContext(ctx, rootID, versionID)
	if err != nil {
		return st, err
	}
	return s.CompleteInstanceStorage(ctx, st)
}

func (s *Service) InstanceOverviewContext(ctx context.Context, rootID, versionID string) (InstanceStatus, error) {
	if e := ctx.Err(); e != nil {
		return InstanceStatus{}, e
	}
	r, e := s.rootAt(rootID, versionID)
	if e != nil {
		return InstanceStatus{}, e
	}
	dir := r.VersionDir()
	if _, e := os.Stat(dir); e != nil {
		return InstanceStatus{}, e
	}
	st := InstanceStatus{Dir: dir, StoragePending: true}
	count := func(sub string, ok func(os.DirEntry) bool) int {
		path, err := safeJoin(dir, sub)
		if err != nil {
			return 0
		}
		entries, _ := os.ReadDir(path)
		n := 0
		for _, en := range entries {
			if en.Type()&os.ModeSymlink == 0 && !strings.HasPrefix(en.Name(), ".") && ok(en) {
				n++
			}
		}
		return n
	}
	pack := func(en os.DirEntry) bool {
		n := strings.ToLower(en.Name())
		return en.IsDir() || strings.HasSuffix(n, ".zip") || strings.HasSuffix(n, ".zip.disabled")
	}
	st.Mods = count("mods", func(en os.DirEntry) bool {
		name := strings.ToLower(en.Name())
		if en.IsDir() {
			return false
		}
		if strings.HasSuffix(name, ".jar") {
			st.EnabledMods++
			return true
		}
		return strings.HasSuffix(name, ".jar.disabled")
	})
	st.ResourcePacks = count("resourcepacks", pack)
	st.Shaders = count("shaderpacks", pack)
	st.Saves = count("saves", func(en os.DirEntry) bool {
		if !en.IsDir() || ctx.Err() != nil {
			return false
		}
		w := readWorldSummary(filepath.Join(dir, "saves"), en)
		if st.RecentWorld == nil || w.LastPlayed.After(st.RecentWorld.LastPlayed) {
			st.RecentWorld = &w
		}
		return true
	})
	st.Screenshots = count("screenshots", func(en os.DirEntry) bool {
		if en.IsDir() || !strings.HasSuffix(strings.ToLower(en.Name()), ".png") || ctx.Err() != nil {
			return false
		}
		if fi, e := en.Info(); e == nil && (st.LatestShot == nil || fi.ModTime().After(st.LatestShot.Modified)) {
			st.LatestShot = &InstanceScreenshot{File: en.Name(), Modified: fi.ModTime(), Size: fi.Size()}
		}
		return true
	})
	if e := ctx.Err(); e != nil {
		return InstanceStatus{}, e
	}
	st.Play = readPlayStats(dir)
	if st.RecentServer, e = readRecentServer(ctx, dir); e != nil {
		return InstanceStatus{}, e
	}
	if st.RecentWorld != nil {
		st.RecentWorld.Backups = len(listWorldBackups(filepath.Join(dir, "backups", st.RecentWorld.Dir)))
	}
	if fi, e := os.Stat(filepath.Join(dir, "logs", "latest.log")); e == nil {
		st.LastLog = fi.ModTime()
	}
	if entries, e := os.ReadDir(filepath.Join(dir, "crash-reports")); e == nil {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() > entries[j].Name() })
		for _, en := range entries {
			if fi, e := en.Info(); e == nil && !en.IsDir() && fi.ModTime().After(st.LastCrash) {
				st.LastCrash, st.CrashFile = fi.ModTime(), en.Name()
			}
		}
	}
	return st, nil
}

// CompleteInstanceStorage scans in bounded directory batches, without sorting
// whole directories or computing a relative path for every file. It returns a
// new summary: the already published world metadata must not be mutated.
func (s *Service) CompleteInstanceStorage(ctx context.Context, st InstanceStatus) (InstanceStatus, error) {
	if st.Dir == "" {
		return InstanceStatus{}, errors.New("missing instance directory")
	}
	st.Total = 0
	st.Sizes = map[string]int64{}
	for _, sub := range []string{"mods", "resourcepacks", "shaderpacks", "saves", "screenshots"} {
		st.Sizes[sub] = 0
	}
	if st.RecentWorld != nil {
		world := *st.RecentWorld
		world.Size = 0
		st.RecentWorld = &world
	}
	var walk func(string, string, string) error
	walk = func(dir, category, world string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		f, err := os.Open(dir)
		if err != nil {
			return nil
		}
		defer f.Close()
		for {
			entries, err := f.ReadDir(256)
			for _, entry := range entries {
				if err := ctx.Err(); err != nil {
					return err
				}
				sub, save := category, world
				if category == "saves" && world == "" {
					save = entry.Name()
				}
				if entry.Type()&fs.ModeSymlink != 0 {
					continue
				}
				if entry.IsDir() {
					if category == "mods" || instanceToolDirectory(entry.Name()) {
						continue
					}
					if err := walk(filepath.Join(dir, entry.Name()), sub, save); err != nil {
						return err
					}
				} else if info, err := entry.Info(); err == nil && info.Mode().IsRegular() {
					if category == "mods" && !strings.HasSuffix(strings.ToLower(entry.Name()), ".jar") && !strings.HasSuffix(strings.ToLower(entry.Name()), ".jar.disabled") {
						continue
					}
					st.Total += info.Size()
					if _, ok := st.Sizes[sub]; ok {
						st.Sizes[sub] += info.Size()
					}
					if sub == "saves" && st.RecentWorld != nil && save == st.RecentWorld.Dir {
						st.RecentWorld.Size += info.Size()
					}
				}
			}
			if err != nil {
				return nil
			}
		}
	}
	// Open only these known data directories. In particular, never enumerate
	// the instance root recursively: packs may share it with huge dev projects.
	for _, sub := range instanceStorageDirectories {
		if err := ctx.Err(); err != nil {
			return InstanceStatus{}, err
		}
		dir := filepath.Join(st.Dir, filepath.FromSlash(sub))
		if info, err := os.Lstat(dir); err != nil || !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
			continue
		}
		if err := walk(dir, sub, ""); err != nil {
			return InstanceStatus{}, err
		}
	}
	files := append([]string{filepath.Base(st.Dir) + ".json", filepath.Base(st.Dir) + ".jar"}, instanceStorageFiles...)
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return InstanceStatus{}, err
		}
		if info, err := os.Lstat(filepath.Join(st.Dir, filepath.FromSlash(file))); err == nil && info.Mode().IsRegular() {
			st.Total += info.Size()
		}
	}
	st.StoragePending = false
	return st, nil
}

var instanceStorageDirectories = []string{
	"mods", "resourcepacks", "shaderpacks", "saves", "screenshots",
	"config", "defaultconfigs", "kubejs", "scripts", "logs", "crash-reports",
	"backups", ".mcl/backups",
}

var instanceStorageFiles = []string{
	"options.txt", "optionsof.txt", "optionsshaders.txt", "servers.dat", "servers.dat_old",
	"usercache.json", "usernamecache.json", "icon.png", "pack.mcmeta", "manifest.json", "modrinth.index.json",
	".mcl/mods.json", ".mcl/mod-checks.json", ".mcl/" + modCacheFile,
	".mcl/playstats.json", ".mcl/pack-lock.json", ".mcl/pack-update.json", ".mcl/icon.png",
	".mcl/last-log.json", ".mcl/loader-install-log.json",
}

func instanceToolDirectory(name string) bool {
	switch strings.ToLower(name) {
	case ".git", ".svn", ".hg", ".gradle", ".idea", ".vscode", ".cache", "node_modules", "__pycache__":
		return true
	}
	return false
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

// VersionIcon returns an instance's own icon when one is on disk: the
// version folder's icon.png (HMCL / imports), PCL's Logo.png, or the
// pack icon MCL saved at import (.mcl/icon.png).
func (s *Service) VersionIcon(rootID, versionID string) ([]byte, error) {
	r, e := s.getRoot(rootID)
	if e != nil {
		return nil, e
	}
	vDir, e := safeJoin(r.Path, "versions/"+versionID)
	if e != nil {
		return nil, e
	}
	for _, p := range []string{
		filepath.Join(vDir, "icon.png"),
		filepath.Join(vDir, "PCL", "Logo.png"),
		filepath.Join(vDir, ".mcl", "icon.png"),
		filepath.Join(r.Path, ".mcl", "icon.png"),
	} {
		if b, e := readCapped(p); e == nil && len(b) > 0 {
			return b, nil
		}
	}
	return nil, errors.New("no icon")
}
