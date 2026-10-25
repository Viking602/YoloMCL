package launcher

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// World is one save folder in the instance's saves directory.
type World struct {
	Dir        string    `json:"dir"`
	Name       string    `json:"name"`
	Mode       string    `json:"mode"` // survival, creative, adventure, spectator, hardcore
	LastPlayed time.Time `json:"lastPlayed,omitempty"`
	Size       int64     `json:"size"`
	Backups    int       `json:"backups"`
	HasIcon    bool      `json:"hasIcon"`
}

// WorldBackup is one zip in backups/<world>/.
type WorldBackup struct {
	File string    `json:"file"`
	Time time.Time `json:"time"`
	Size int64     `json:"size"`
	Auto bool      `json:"auto"` // 还原前自动备份
}

func (s *Service) worldPaths(rootID, versionID string) (saves, backups string, e error) {
	r, e := s.rootAt(rootID, versionID)
	if e != nil {
		return "", "", e
	}
	d := r.VersionDir()
	saves, e = safeJoin(d, "saves")
	if e != nil {
		return "", "", e
	}
	backups, e = safeJoin(d, "backups")
	return saves, backups, e
}

func validWorldDir(n string) error {
	if n == "" || n != filepath.Base(n) || n == "." || n == ".." || strings.HasPrefix(n, ".") {
		return errors.New("无效的存档名")
	}
	return nil
}

func treeBytes(p string) int64 {
	var n int64
	_ = filepath.WalkDir(p, func(_ string, d fs.DirEntry, e error) error {
		if e == nil && !d.IsDir() {
			if fi, e := d.Info(); e == nil {
				n += fi.Size()
			}
		}
		return nil
	})
	return n
}

// ListWorlds reads every world folder (newest played first).
func (s *Service) ListWorlds(rootID, versionID string) ([]World, error) {
	saves, backups, e := s.worldPaths(rootID, versionID)
	if e != nil {
		return nil, e
	}
	entries, _ := os.ReadDir(saves)
	out := []World{}
	for _, en := range entries {
		if !en.IsDir() || strings.HasPrefix(en.Name(), ".") {
			continue
		}
		w := readWorldSummary(saves, en)
		dir := filepath.Join(saves, en.Name())
		if size, ok := s.cachedWorldSize(dir, s.instanceRunning(rootID, versionID)); ok {
			w.Size = size
		} else {
			w.Size = treeBytes(dir)
			if !s.instanceRunning(rootID, versionID) {
				s.storeWorldSize(dir, w.Size)
			}
		}
		w.Backups = len(listWorldBackups(filepath.Join(backups, en.Name())))
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastPlayed.After(out[j].LastPlayed) })
	return out, nil
}

// readWorldSummary reads only the save's metadata, without walking its regions.
// Both the world list and the overview use the same name and play date.
func readWorldSummary(saves string, en os.DirEntry) World {
	p := filepath.Join(saves, en.Name())
	w := World{Dir: en.Name(), Name: en.Name(), Mode: "survival"}
	level, levelErr := safeJoin(p, "level.dat")
	if levelErr == nil {
		if f, e := os.Open(level); e == nil {
			if li, e := ParseLevelDat(f); e == nil {
				if li.Name != "" {
					w.Name = li.Name
				}
				w.Mode = [...]string{"survival", "creative", "adventure", "spectator"}[min(max(li.GameType, 0), 3)]
				if li.Hardcore {
					w.Mode = "hardcore"
				}
				if li.LastPlayed > 0 {
					w.LastPlayed = time.UnixMilli(li.LastPlayed)
				}
			}
			f.Close()
		}
	}
	if w.LastPlayed.IsZero() {
		if fi, e := en.Info(); e == nil {
			w.LastPlayed = fi.ModTime()
		}
	}
	_, e := os.Stat(filepath.Join(p, "icon.png"))
	w.HasIcon = e == nil
	return w
}

func listWorldBackups(dir string) []WorldBackup {
	entries, _ := os.ReadDir(dir)
	out := []WorldBackup{}
	for _, en := range entries {
		n := en.Name()
		if en.IsDir() || !strings.HasSuffix(n, ".zip") {
			continue
		}
		fi, e := en.Info()
		if e != nil {
			continue
		}
		b := WorldBackup{File: n, Time: fi.ModTime(), Size: fi.Size(), Auto: strings.HasSuffix(n, "-auto.zip")}
		stem := strings.TrimSuffix(strings.TrimSuffix(n, ".zip"), "-auto")
		if i := len(stem) - 15; i > 0 {
			if t, e := time.ParseInLocation("20060102-150405", stem[i:], time.Local); e == nil {
				b.Time = t
			}
		}
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out
}

// WorldBackups lists a world's backups, newest first.
func (s *Service) WorldBackups(rootID, versionID, world string) ([]WorldBackup, error) {
	if e := validWorldDir(world); e != nil {
		return nil, e
	}
	_, backups, e := s.worldPaths(rootID, versionID)
	if e != nil {
		return nil, e
	}
	return listWorldBackups(filepath.Join(backups, world)), nil
}

// WorldIcon returns a world's icon.png.
func (s *Service) WorldIcon(rootID, versionID, world string) ([]byte, error) {
	if e := validWorldDir(world); e != nil {
		return nil, e
	}
	saves, _, e := s.worldPaths(rootID, versionID)
	if e != nil {
		return nil, e
	}
	p := filepath.Join(saves, world, "icon.png")
	if fi, e := os.Stat(p); e != nil || fi.Size() > 1<<20 {
		return nil, errors.New("no icon")
	}
	return os.ReadFile(p)
}

func (s *Service) instanceRunning(rootID, versionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := runKey(rootID, versionID)
	for key := range s.running {
		if key == k || strings.HasPrefix(key, k+"#") {
			return true
		}
	}
	return false
}

// zipWorld writes src into dest atomically (dest.tmp then rename),
// skipping session.lock which the game keeps locked.
