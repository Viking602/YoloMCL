package launcher

import (
	"context"
	"encoding/binary"
	"hash/maphash"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Overview inputs are checked individually. Storage directory stamps only
// detect shallow changes, so totals also expire without extending their age
// on cache hits. Explicit refreshes always scan; navigation can reuse totals
// briefly without walking every world and mod directory again.
const statusCacheLimit = 8
const storageCacheTTL = 30 * time.Second

type fileStamp struct {
	size, modified int64
	mode           os.FileMode
}

type statusCacheEntry struct {
	used            uint64
	overviewStamp   uint64
	overviewScan    uint64
	overview        InstanceStatus
	storageStamp    string
	storage         InstanceStatus
	storageComplete bool
	storageAt       time.Time
	storageScan     uint64
}

func stampFile(path string) fileStamp {
	info, err := os.Lstat(path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{size: info.Size(), modified: info.ModTime().UnixNano(), mode: info.Mode()}
}

func (s fileStamp) text() string {
	return strconv.FormatInt(s.size, 10) + ":" + strconv.FormatInt(s.modified, 10) + ":" + strconv.FormatInt(int64(s.mode), 10)
}

func (s *Service) statusEntry(dir string) *statusCacheEntry {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	if s.statuses == nil {
		s.statuses = map[string]*statusCacheEntry{}
	}
	if e := s.statuses[dir]; e != nil {
		s.statusUse++
		e.used = s.statusUse
		return e
	}
	if len(s.statuses) >= statusCacheLimit {
		var oldest string
		var used uint64
		for k, e := range s.statuses {
			if oldest == "" || e.used < used {
				oldest, used = k, e.used
			}
		}
		delete(s.statuses, oldest)
	}
	e := &statusCacheEntry{}
	s.statuses[dir] = e
	return e
}

func (s *Service) invalidateStatus(dir string) {
	if dir == "" {
		return
	}
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	for key := range s.statuses {
		if key == dir || strings.HasPrefix(key, dir+string(filepath.Separator)) {
			delete(s.statuses, key)
		}
	}
	for key := range s.worldSizes {
		if strings.HasPrefix(key, dir+string(filepath.Separator)) {
			delete(s.worldSizes, key)
		}
	}
}

func cloneStatus(st InstanceStatus) InstanceStatus {
	st.Sizes = maps.Clone(st.Sizes)
	if st.RecentWorld != nil {
		w := *st.RecentWorld
		st.RecentWorld = &w
	}
	if st.RecentServer != nil {
		sv := *st.RecentServer
		st.RecentServer = &sv
	}
	if st.LatestShot != nil {
		sh := *st.LatestShot
		st.LatestShot = &sh
	}
	return st
}

var overviewHashSeed = maphash.MakeSeed()

// Directory entries live only for this request. Validation and a cache miss
// share one enumeration, including the metadata Windows already returns with
// it, instead of reopening each file just to stat it. Only the digest survives.
type overviewInputs struct {
	stamp   uint64
	entries map[string][]os.DirEntry
}

func readOverviewInputs(ctx context.Context, dir string) (overviewInputs, error) {
	out := overviewInputs{entries: make(map[string][]os.DirEntry, 8)}
	var h maphash.Hash
	h.SetSeed(overviewHashSeed)
	var buf [24]byte
	addName := func(name string) {
		h.WriteString(name)
		h.WriteString("\x00")
	}
	addStamp := func(st fileStamp) {
		binary.LittleEndian.PutUint64(buf[:8], uint64(st.size))
		binary.LittleEndian.PutUint64(buf[8:16], uint64(st.modified))
		binary.LittleEndian.PutUint64(buf[16:], uint64(st.mode))
		h.Write(buf[:])
	}
	for _, sub := range []string{"mods", "resourcepacks", "shaderpacks", "saves", "screenshots", "logs", "crash-reports", "backups"} {
		if err := ctx.Err(); err != nil {
			return overviewInputs{}, err
		}
		path := filepath.Join(dir, sub)
		stamp := stampFile(path)
		h.WriteString("dir\x00")
		addName(sub)
		addStamp(stamp)
		if !stamp.mode.IsDir() || stamp.mode&os.ModeSymlink != 0 {
			continue
		}
		// os.ReadDir already sorts by name, so the digest has stable order.
		entries, _ := os.ReadDir(path)
		out.entries[sub] = entries
		for _, en := range entries {
			if err := ctx.Err(); err != nil {
				return overviewInputs{}, err
			}
			h.WriteString("entry\x00")
			addName(en.Name())
			// These counters depend on names and file types, not JAR/ZIP
			// contents, sizes or modification times.
			st := fileStamp{mode: en.Type()}
			if sub != "mods" && sub != "resourcepacks" && sub != "shaderpacks" {
				if info, err := en.Info(); err == nil {
					st = fileStamp{size: info.Size(), modified: info.ModTime().UnixNano(), mode: info.Mode()}
				}
			}
			addStamp(st)
			if sub == "saves" && en.IsDir() {
				addStamp(stampFile(filepath.Join(path, en.Name(), "level.dat")))
				addStamp(stampFile(filepath.Join(path, en.Name(), "icon.png")))
			}
		}
	}
	addStamp(stampFile(playStatsPath(dir)))
	if err := ctx.Err(); err != nil {
		return overviewInputs{}, err
	}
	out.stamp = h.Sum64()
	return out, nil
}

func storageStamp(dir string) string {
	var b strings.Builder
	for _, sub := range instanceStorageDirectories {
		path := filepath.Join(dir, filepath.FromSlash(sub))
		b.WriteString(sub)
		b.WriteByte('\t')
		b.WriteString(stampFile(path).text())
		b.WriteByte('\n')
	}
	files := append([]string{filepath.Base(dir) + ".json", filepath.Base(dir) + ".jar"}, instanceStorageFiles...)
	for _, file := range files {
		b.WriteString(file)
		b.WriteByte('\t')
		b.WriteString(stampFile(filepath.Join(dir, filepath.FromSlash(file))).text())
		b.WriteByte('\n')
	}
	return b.String()
}

func (s *Service) cachedOverview(e *statusCacheEntry, dir string, stamp uint64) (InstanceStatus, bool) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	if s.statuses[dir] == e && e.overview.Dir == dir && e.overviewStamp == stamp {
		return cloneStatus(e.overview), true
	}
	return InstanceStatus{}, false
}

func (s *Service) storeOverview(e *statusCacheEntry, scan, stamp uint64, st InstanceStatus) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	if s.statuses[st.Dir] != e || e.overviewScan != scan {
		return
	}
	e.overviewStamp, e.overview = stamp, cloneStatus(st)
}

func (s *Service) cachedStorage(dir string, st InstanceStatus) (InstanceStatus, bool) {
	stamp := storageStamp(dir)
	e := s.statusEntry(dir)
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	if !e.storageComplete || time.Since(e.storageAt) >= storageCacheTTL || e.storageStamp != stamp || e.storage.Dir != st.Dir {
		return InstanceStatus{}, false
	}
	out := cloneStatus(st)
	out.Sizes, out.Total, out.StoragePending = maps.Clone(e.storage.Sizes), e.storage.Total, false
	if out.RecentWorld != nil && e.storage.RecentWorld != nil && out.RecentWorld.Dir == e.storage.RecentWorld.Dir {
		w := *out.RecentWorld
		w.Size = e.storage.RecentWorld.Size
		out.RecentWorld = &w
	}
	return out, true
}

func (s *Service) storeStorage(e *statusCacheEntry, scan uint64, stamp string, st InstanceStatus) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	// An invalidation or a newer scan must not be overwritten by a late
	// result from a canceled or superseded overview worker.
	if s.statuses[st.Dir] != e || e.storageScan != scan {
		return
	}
	e.storageStamp, e.storage, e.storageComplete = stamp, cloneStatus(st), true
	e.storageAt = time.Now()
}

// worldSizeCache reuses a world's byte total. A running game rewrites region
// files without moving the directory time, so sizes are never served then.
type worldSizeEntry struct {
	stamp   string
	size    int64
	used    time.Time
	scanned time.Time
}

func worldSizeStamp(dir string) string {
	return stampFile(dir).text() + "|" + stampFile(filepath.Join(dir, "region")).text() + "|" + stampFile(filepath.Join(dir, "level.dat")).text()
}

func (s *Service) cachedWorldSize(dir string, running bool) (int64, bool) {
	if running {
		return 0, false
	}
	stamp := worldSizeStamp(dir)
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	e := s.worldSizes[dir]
	if e == nil || time.Since(e.scanned) >= storageCacheTTL || e.stamp != stamp {
		return 0, false
	}
	e.used = time.Now()
	return e.size, true
}

func (s *Service) storeWorldSize(dir string, size int64) {
	stamp := worldSizeStamp(dir)
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	if s.worldSizes == nil {
		s.worldSizes = map[string]*worldSizeEntry{}
	}
	if len(s.worldSizes) >= 64 {
		var oldest string
		var used time.Time
		for k, e := range s.worldSizes {
			if oldest == "" || e.used.Before(used) {
				oldest, used = k, e.used
			}
		}
		delete(s.worldSizes, oldest)
	}
	now := time.Now()
	s.worldSizes[dir] = &worldSizeEntry{stamp: stamp, size: size, used: now, scanned: now}
}
