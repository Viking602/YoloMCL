package launcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

func (s *Service) Snapshot() Snapshot {
	// The platform memory query can be a syscall or a subprocess; never make
	// configuration, job or launch operations wait for it.
	sysMem, _ := SystemMemory()
	s.mu.Lock()
	cfg := s.cfg.clone()
	authServers := s.authServersLocked()
	jobs := s.jobListLocked()
	running := map[string]bool{}
	runVers := map[string][]string{}
	for k := range s.running {
		id, ver, _ := strings.Cut(k, "|")
		ver, _, _ = strings.Cut(ver, "#") // a second copy of the same version
		running[id] = true
		if !slices.Contains(runVers[id], ver) {
			runVers[id] = append(runVers[id], ver)
		}
	}
	s.mu.Unlock()
	out := Snapshot{Settings: cfg.Settings, Instances: []Instance{}, Accounts: cfg.Accounts, ActiveRoot: cfg.ActiveRoot, ActiveAccount: cfg.ActiveAccount, Jobs: jobs, AuthServers: authServers, DataDir: s.dir}
	out.Services = ServiceAvailability{Microsoft: s.publisher.HasMicrosoft(), CurseForge: s.publisher.HasCurseForge()}
	for _, r := range cfg.Roots {
		index := s.rootInventory(r)
		versions := index.versions()
		for i := range versions {
			if vc, ok := r.VersionConfigs[versions[i].ID]; ok {
				versions[i].JavaPath = vc.JavaPath
				versions[i].MemoryMB = vc.MemoryMB
				versions[i].MemoryAutoSet = vc.MemoryAuto
				versions[i].JVMArgs = vc.JVMArgs
				versions[i].DisplayName = vc.DisplayName
			}
			versions[i].MemoryAuto, _ = r.MemoryPlan(versions[i].ID, cfg.Settings)
			// AvailableMB is left out on purpose: free memory changes every
			// poll and would make the snapshot digest (and the UI) churn.
			versions[i].AutoMemoryMB = AutoMemoryMB(AutoMemoryInput{TotalMB: sysMem.TotalMB,
				Minecraft: versions[i].Minecraft, Loader: versions[i].Loader, Mods: index.modCount(versions[i].ID).Enabled})
		}
		out.Instances = append(out.Instances, Instance{Root: r, Available: index.Available, Versions: versions, ModCount: index.modCount(r.SelectedVersion).Total, Pack: index.Pack, Running: running[r.ID], RunningVersions: runVers[r.ID], PackUpdate: index.PackUpdate, PackBackups: slices.Clone(index.PackBackups)})
	}
	return out
}

// Snapshots of jobs/selection are frequent. Filesystem inventories are not:
// reuse them, validate known Minecraft metadata after filesystem changes, and
// invalidate immediately after launcher operations. Never hold Service.mu
// during disk IO, nor hold an entry's publication lock while rebuilding it.
const inventoryInterval = 5 * time.Second

type inventoryStamp struct {
	Size, Modified int64
	Mode           fs.FileMode
}

type inventoryMods struct{ Total, Enabled int }

type inventoryData struct {
	Schema      int
	Path        string
	Available   bool
	Versions    []VersionSummary
	Directories map[string]string
	Mods        map[string]inventoryMods
	Pack        *PackLock
	PackUpdate  *PackUpdateInfo
	PackBackups []PackBackup
	Watched     map[string]inventoryStamp
}

type inventoryEntry struct {
	mu       sync.Mutex
	scanMu   sync.Mutex
	writeMu  sync.Mutex
	data     *inventoryData // immutable after publication
	checked  time.Time
	revision uint64
	dirty    bool
}

func (s *Service) inventoryPath(path string) string {
	sum := sha256.Sum256([]byte(javaPathKey(filepath.Clean(path))))
	return filepath.Join(s.dir, "cache", "instances", hex.EncodeToString(sum[:16])+".json")
}

func (s *Service) inventoryEntry(path string) *inventoryEntry {
	s.inventoryMu.Lock()
	defer s.inventoryMu.Unlock()
	if s.inventories == nil {
		s.inventories = map[string]*inventoryEntry{}
	}
	if entry := s.inventories[path]; entry != nil {
		return entry
	}
	entry := &inventoryEntry{}
	var data inventoryData
	// This cache lives in the launcher's independent data directory. Paths in
	// Watched are relative and are checked through os.Root, even for stale or
	// edited cache files; links cannot turn validation into an outside scan.
	if b, err := os.ReadFile(s.inventoryPath(path)); err == nil && len(b) <= 16<<20 && json.Unmarshal(b, &data) == nil && data.Schema == 1 && data.Path == path && data.Watched != nil {
		entry.data = &data
	}
	s.inventories[path] = entry
	return entry
}

func (s *Service) rootInventory(r Root) *inventoryData {
	entry := s.inventoryEntry(r.Path)
	entry.mu.Lock()
	data, dirty := entry.data, entry.dirty
	entry.mu.Unlock()
	if data == nil || dirty {
		return s.refreshInventory(r, entry)
	}
	return data
}

// RefreshInventories checks metadata when the scan interval permits. Desktop
// maintenance uses InventoryMonitor to skip unchanged directories. Snapshot
// never launches workers or reads files just because progress is being polled.
func (s *Service) RefreshInventories(ctx context.Context) {
	s.mu.Lock()
	roots := append([]Root(nil), s.cfg.Roots...)
	s.mu.Unlock()
	for _, r := range roots {
		if ctx.Err() != nil {
			return
		}
		s.refreshInventory(r, s.inventoryEntry(r.Path))
	}
}

func (s *Service) invalidateInventory(rootID string) {
	s.mu.Lock()
	var paths []string
	for _, r := range s.cfg.Roots {
		if rootID == "" || r.ID == rootID {
			paths = append(paths, r.Path)
		}
	}
	s.mu.Unlock()
	s.inventoryMu.Lock()
	for _, path := range paths {
		if entry := s.inventories[path]; entry != nil {
			entry.mu.Lock()
			entry.dirty = true
			entry.revision++
			entry.mu.Unlock()
		}
	}
	s.inventoryMu.Unlock()
	for _, path := range paths {
		s.invalidateStatus(path)
	}
}

// cacheGameDir fills r.gameDir from the published inventory, without scanning
// or touching the disk. It stays empty (VersionDir then probes) unless the
// entry was validated in this process, is not invalidated, and knows the
// selected version. The result is keyed to r.SelectedVersion.
func (s *Service) cacheGameDir(r *Root) {
	r.gameDir, r.gameDirFor = "", ""
	if r.SelectedVersion == "" {
		return
	}
	s.inventoryMu.Lock()
	entry := s.inventories[r.Path]
	s.inventoryMu.Unlock()
	if entry == nil {
		return
	}
	entry.mu.Lock()
	data, dirty, checked := entry.data, entry.dirty, entry.checked
	entry.mu.Unlock()
	if data == nil || dirty || checked.IsZero() {
		return
	}
	// The persisted cache is user-writable: accept only the two shapes the
	// scan produces.
	switch dir, ok := data.Directories[r.SelectedVersion]; {
	case !ok:
		return
	case dir == ".":
		r.gameDir = r.Path
	case dir == filepath.Join("versions", r.SelectedVersion):
		abs, err := filepath.Abs(r.Path)
		if err != nil {
			return
		}
		r.gameDir = filepath.Join(abs, dir)
	default:
		return
	}
	r.gameDirFor = r.SelectedVersion
}

// Configured roots are canonicalized when added. If a root is later replaced
// by a junction/symlink, do not silently adopt its outside target for scans.
func openScanRoot(path string) (*os.Root, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("scan root is not a directory")
	}
	return os.OpenRoot(path)
}

func inventoryFileStamp(root *os.Root, name string) inventoryStamp {
	info, err := root.Stat(name)
	if err != nil {
		return inventoryStamp{}
	}
	return inventoryStamp{Size: info.Size(), Modified: info.ModTime().UnixNano(), Mode: info.Mode()}
}

func inventoryUnchanged(path string, data *inventoryData) bool {
	root, err := openScanRoot(path)
	if err != nil {
		return !data.Available
	}
	defer root.Close()
	if !data.Available {
		return false
	}
	for name, stamp := range data.Watched {
		if inventoryFileStamp(root, name) != stamp {
			return false
		}
	}
	return true
}

func (s *Service) refreshInventory(r Root, entry *inventoryEntry) *inventoryData {
	data, _ := s.checkInventory(r, entry)
	return data
}

// The boolean reports validation performed by this call, not by a concurrent
// snapshot that happened to publish while this caller waited for scanMu.
func (s *Service) checkInventory(r Root, entry *inventoryEntry) (*inventoryData, bool) {
	entry.scanMu.Lock()
	defer entry.scanMu.Unlock()
	entry.mu.Lock()
	old, dirty, revision := entry.data, entry.dirty, entry.revision
	// Coalesce concurrent initial snapshots and invalidations.
	if old != nil && !dirty && time.Since(entry.checked) < inventoryInterval {
		entry.mu.Unlock()
		return old, false
	}
	entry.mu.Unlock()
	data := old
	if old == nil || dirty || !inventoryUnchanged(r.Path, old) {
		data = scanInventory(r)
	}
	entry.mu.Lock()
	entry.data, entry.checked = data, time.Now()
	entry.dirty = entry.revision != revision
	entry.mu.Unlock()
	if data != old {
		s.persistInventory(entry, data)
	}
	return data, true
}

func (s *Service) persistInventory(entry *inventoryEntry, data *inventoryData) {
	entry.writeMu.Lock()
	defer entry.writeMu.Unlock()
	entry.mu.Lock()
	current := entry.data == data
	entry.mu.Unlock()
	if current {
		_ = atomicJSON(s.inventoryPath(data.Path), data)
	}
}

func scanInventory(r Root) *inventoryData {
	d := &inventoryData{Schema: 1, Path: r.Path, Directories: map[string]string{}, Mods: map[string]inventoryMods{}, Watched: map[string]inventoryStamp{}}
	root, err := openScanRoot(r.Path)
	if err != nil {
		return d
	}
	defer root.Close()
	d.Available = true
	watch := func(name string) { d.Watched[name] = inventoryFileStamp(root, name) }
	watch(".")
	watch("versions")
	var metadataFiles []string
	d.Versions, metadataFiles = scanVersionsWithFiles(r.Path)
	for _, name := range metadataFiles {
		watch(name)
	}
	sharedModsPresent := false
	count := func(dir string) {
		if _, exists := d.Mods[dir]; exists {
			return
		}
		name := filepath.Join(dir, "mods")
		watch(name)
		var count inventoryMods
		if entries, err := fs.ReadDir(root.FS(), filepath.ToSlash(name)); err == nil {
			if dir == "." {
				sharedModsPresent = len(entries) > 0
			}
			for _, en := range entries {
				if en.Type().IsRegular() {
					name := strings.ToLower(en.Name())
					if strings.HasSuffix(name, ".jar") {
						count.Enabled++
						count.Total++
					} else if strings.HasSuffix(name, ".jar.disabled") {
						count.Total++
					}
				}
			}
		}
		d.Mods[dir] = count
	}
	count(".")
	for _, v := range d.Versions {
		base := filepath.Join("versions", v.ID)
		dir := base
		if sharedModsPresent {
			isolated := false
			for _, sub := range []string{"mods", "resourcepacks", "shaderpacks", "saves", "config", "options.txt"} {
				if _, err := root.Lstat(filepath.Join(base, sub)); err == nil {
					isolated = true
					break
				}
			}
			if !isolated {
				dir = "."
			}
		}
		d.Directories[v.ID] = dir
		count(dir)
	}
	for _, name := range []string{".mcl/pack-lock.json", ".mcl/pack-update.json", ".mcl/backups"} {
		watch(filepath.FromSlash(name))
	}
	d.Pack, d.PackUpdate, d.PackBackups = readPack(r.Path), readPackUpdate(r.Path), listPackBackups(r.Path)
	for _, b := range d.PackBackups {
		watch(filepath.Join(".mcl", "backups", b.ID, "backup.json"))
	}
	return d
}

func (d *inventoryData) modCount(version string) inventoryMods {
	dir, ok := d.Directories[version]
	if !ok {
		dir = "."
	}
	return d.Mods[dir]
}

func (d *inventoryData) versions() []VersionSummary { return slices.Clone(d.Versions) }
