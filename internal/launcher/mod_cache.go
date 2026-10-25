package launcher

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// modCacheFile persists per-jar hashes and metadata across launcher runs so
// reopening a pack never rehashes unchanged jars.
const modCacheFile = "modcache.json"

const modCacheVersion = 1

type modCacheEntry struct {
	Size  int64  `json:"size"`
	MTime int64  `json:"mtime"`
	SHA1  string `json:"sha1"`
	FP    uint32 `json:"fp,omitempty"`
	HasFP bool   `json:"hasFp,omitempty"`
	Meta  Mod    `json:"meta"`
}

type modCache struct {
	V      int                      `json:"v"`
	Shared bool                     `json:"shared,omitempty"`
	Files  map[string]modCacheEntry `json:"files"`
}

var modCacheMu sync.Mutex // serialises cache writes per process

// loadModCache reads the cache; a missing, corrupt or old-format file is an
// empty cache (it is rebuilt, never trusted partially).
func loadModCache(root string) map[string]modCacheEntry {
	return loadModCacheData(root).Files
}

const maxLocalModCacheBytes = 4 << 20

func loadModCacheData(root string) modCache {
	var c modCache
	path, e := safeJoin(root, ".mcl/"+modCacheFile)
	if e != nil {
		return modCache{Files: map[string]modCacheEntry{}}
	}
	f, e := os.Open(path)
	if e == nil {
		defer f.Close()
		st, err := f.Stat()
		if err == nil && st.Size() <= maxLocalModCacheBytes {
			// Decode holds the complete JSON value anyway. An exact-sized
			// bounded read avoids the decoder's geometric buffer growth.
			b := make([]byte, int(st.Size()))
			if _, e = io.ReadFull(f, b); e == nil {
				e = json.Unmarshal(b, &c)
			}
		}
	}
	if e != nil || c.V != modCacheVersion || c.Files == nil {
		return modCache{Files: map[string]modCacheEntry{}}
	}
	return c
}

func saveModCache(root string, files map[string]modCacheEntry) {
	saveModCacheContext(context.Background(), root, files, false)
}

func saveModCacheContext(ctx context.Context, root string, files map[string]modCacheEntry, shared bool) {
	modCacheMu.Lock()
	defer modCacheMu.Unlock()
	writeModCacheContext(ctx, root, files, shared)
}

// A cancelled older scan may finish its metadata flush after a new scan has
// published. Keep those newer entries; only fill identities it has not saved.
func checkpointModCache(ctx context.Context, root string, files map[string]modCacheEntry, shared bool) {
	modCacheMu.Lock()
	defer modCacheMu.Unlock()
	if ctx.Err() != nil {
		return
	}
	current := loadModCacheData(root)
	for name, entry := range files {
		if ctx.Err() != nil {
			return
		}
		if previous, exists := current.Files[name]; exists {
			if previous.Size == entry.Size && previous.MTime == entry.MTime {
				continue
			}
			// Replace an old stamp only if our completed result still describes
			// the file; another scan may have observed a newer replacement.
			path, err := safeJoin(root, "mods/"+name)
			if err != nil {
				continue
			}
			st, err := os.Stat(path)
			if err != nil || st.Size() != entry.Size || st.ModTime().UnixNano() != entry.MTime {
				continue
			}
		}
		current.Files[name] = entry
	}
	writeModCacheContext(ctx, root, current.Files, shared && current.Shared)
}

func writeModCacheContext(ctx context.Context, root string, files map[string]modCacheEntry, shared bool) {
	path, err := safeJoin(root, ".mcl/"+modCacheFile)
	if err != nil || ctx.Err() != nil || os.MkdirAll(filepath.Dir(path), 0755) != nil {
		return
	}
	temp, err := safeJoin(root, ".mcl/"+modCacheFile+".tmp")
	if err != nil {
		return
	}
	f, err := os.OpenFile(temp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer func() { _ = f.Close(); _ = os.Remove(f.Name()) }()
	w := bufio.NewWriterSize(f, modReadChunk)
	header := fmt.Sprintf(`{"v":%d,"shared":%t,"files":{`, modCacheVersion, shared)
	if _, err = w.WriteString(header); err != nil {
		return
	}
	size, count := len(header)+2, 0
	// Serialize one small entry at a time instead of building another complete
	// copy of the pack's metadata. Entries exceeding the budget remain misses.
	for name, entry := range files {
		if ctx.Err() != nil {
			return
		}
		b, err := json.Marshal(entry)
		if err != nil || len(b) > sharedModRecordBytes {
			continue
		}
		key, err := json.Marshal(name)
		if err != nil {
			continue
		}
		n := len(key) + len(b) + 2
		if size+n > maxLocalModCacheBytes {
			continue
		}
		if count > 0 {
			if err = w.WriteByte(','); err != nil {
				return
			}
		}
		if _, err = w.Write(key); err != nil {
			return
		}
		if err = w.WriteByte(':'); err != nil {
			return
		}
		if _, err = w.Write(b); err != nil {
			return
		}
		size += n
		count++
	}
	if _, err = w.WriteString("}}"); err != nil {
		return
	}
	if err = w.Flush(); err != nil {
		return
	}
	if err = f.Close(); err != nil || ctx.Err() != nil {
		return
	}
	_ = os.Rename(f.Name(), path) // Best-effort, disposable cache; never fail a scan.
}

// ModCacheInfo reports rebuildable metadata only, excluding mods, installation
// records, update history, backups and artwork.
type ModCacheInfo struct {
	Bytes       int64
	Files       int
	SharedLimit int64
}

// ModCacheUsage inspects cache file sizes without loading their contents.
func (s *Service) ModCacheUsage(ctx context.Context) (ModCacheInfo, error) {
	out := ModCacheInfo{SharedLimit: sharedModCacheBytes}
	err := s.visitModCaches(ctx, func(root *os.Root, path string) error {
		st, err := root.Stat(path)
		if err == nil && st.Mode().IsRegular() {
			out.Bytes += st.Size()
			out.Files++
		}
		return err
	})
	return out, err
}

// ClearModCache removes only rebuildable metadata from registered instances.
// Old scans/installations may finish, but cannot republish pre-clear caches.
func (s *Service) ClearModCache(ctx context.Context) error {
	c := s.modMetadataCache()
	if !c.lock(ctx) {
		return ctx.Err()
	}
	defer c.unlock()
	c.generation++
	c.release()
	c.unavailable = false
	modCacheMu.Lock()
	defer modCacheMu.Unlock()
	return s.visitModCaches(ctx, func(root *os.Root, path string) error { return root.Remove(path) })
}

func (s *Service) visitModCaches(ctx context.Context, visit func(*os.Root, string) error) error {
	s.mu.Lock()
	roots := append([]Root(nil), s.cfg.Roots...)
	s.mu.Unlock()
	var errs []error
	check := func(err error) {
		if err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		}
	}
	if root, err := os.OpenRoot(s.dir); err == nil {
		check(visit(root, sharedModCacheFile))
		check(visit(root, sharedModCacheFile+".tmp"))
		_ = root.Close()
	} else {
		check(err)
	}
	for _, r := range roots {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		root, err := openScanRoot(r.Path)
		if err != nil {
			check(err)
			continue
		}
		check(visit(root, filepath.Join(".mcl", modCacheFile)))
		check(visit(root, filepath.Join(".mcl", modCacheFile+".tmp")))
		if versions, err := root.Open("versions"); err == nil {
			// Read directory entries in small batches; never descend into mods.
			for {
				entries, err := versions.ReadDir(64)
				for _, entry := range entries {
					if ctx.Err() != nil {
						_ = versions.Close()
						_ = root.Close()
						return ctx.Err()
					}
					if entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
						check(visit(root, filepath.Join("versions", entry.Name(), ".mcl", modCacheFile)))
						check(visit(root, filepath.Join("versions", entry.Name(), ".mcl", modCacheFile+".tmp")))
					}
				}
				if err != nil {
					if err != io.EOF {
						check(err)
					}
					break
				}
			}
			_ = versions.Close()
		} else {
			check(err)
		}
		_ = root.Close()
	}
	return errors.Join(errs...)
}
