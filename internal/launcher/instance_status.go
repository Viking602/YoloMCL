package launcher

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
	entry := s.statusEntry(dir)
	s.statusMu.Lock()
	entry.overviewScan++
	scan := entry.overviewScan
	s.statusMu.Unlock()
	inputs, e := readOverviewInputs(ctx, dir)
	if e != nil {
		return InstanceStatus{}, e
	}
	if cached, ok := s.cachedOverview(entry, dir, inputs.stamp); ok {
		return cached, nil
	}
	st := InstanceStatus{Dir: dir, StoragePending: true}
	count := func(sub string, ok func(os.DirEntry) bool) int {
		n := 0
		for _, en := range inputs.entries[sub] {
			if ctx.Err() != nil {
				break
			}
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
	if st.RecentServer, e = readRecentServer(ctx, dir, inputs.entries["logs"]); e != nil {
		return InstanceStatus{}, e
	}
	if st.RecentWorld != nil {
		st.RecentWorld.Backups = len(listWorldBackups(filepath.Join(dir, "backups", st.RecentWorld.Dir)))
	}
	if fi, e := os.Stat(filepath.Join(dir, "logs", "latest.log")); e == nil {
		st.LastLog = fi.ModTime()
	}
	crashes := inputs.entries["crash-reports"]
	for i := len(crashes) - 1; i >= 0; i-- {
		en := crashes[i]
		if fi, e := en.Info(); e == nil && !en.IsDir() && fi.ModTime().After(st.LastCrash) {
			st.LastCrash, st.CrashFile = fi.ModTime(), en.Name()
		}
	}
	if err := ctx.Err(); err != nil {
		return InstanceStatus{}, err
	}
	s.storeOverview(entry, scan, inputs.stamp, st)
	return cloneStatus(st), nil
}

// CachedInstanceStorage reuses recent totals during navigation. Deeper changes
// may not change directory timestamps, so cache hits never extend the TTL.
// Use CompleteInstanceStorage for an explicit refresh or an exact total.
func (s *Service) CachedInstanceStorage(ctx context.Context, st InstanceStatus) (InstanceStatus, error) {
	if err := ctx.Err(); err != nil {
		return InstanceStatus{}, err
	}
	if st.Dir != "" {
		if cached, ok := s.cachedStorage(st.Dir, st); ok {
			return cached, nil
		}
	}
	return s.CompleteInstanceStorage(ctx, st)
}

// CompleteInstanceStorage always scans in bounded directory batches, without
// sorting whole directories or computing a relative path for every file. It
// returns a new summary without mutating already published world metadata.
func (s *Service) CompleteInstanceStorage(ctx context.Context, st InstanceStatus) (InstanceStatus, error) {
	if err := ctx.Err(); err != nil {
		return InstanceStatus{}, err
	}
	if st.Dir == "" {
		return InstanceStatus{}, errors.New("missing instance directory")
	}
	entry := s.statusEntry(st.Dir)
	s.statusMu.Lock()
	entry.storageScan++
	scan := entry.storageScan
	s.statusMu.Unlock()
	stamp := storageStamp(st.Dir)
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
	if err := ctx.Err(); err != nil {
		return InstanceStatus{}, err
	}
	s.storeStorage(entry, scan, stamp, st)
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
