package launcher

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
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
	V     int                      `json:"v"`
	Files map[string]modCacheEntry `json:"files"`
}

var modCacheMu sync.Mutex // serialises cache writes per process

// loadModCache reads the cache; a missing, corrupt or old-format file is an
// empty cache (it is rebuilt, never trusted partially).
func loadModCache(root string) map[string]modCacheEntry {
	var c modCache
	path, e := safeJoin(root, ".mcl/"+modCacheFile)
	if e != nil {
		return map[string]modCacheEntry{}
	}
	b, e := os.ReadFile(path)
	if e != nil || json.Unmarshal(b, &c) != nil || c.V != modCacheVersion || c.Files == nil {
		return map[string]modCacheEntry{}
	}
	return c.Files
}

func saveModCache(root string, files map[string]modCacheEntry) {
	modCacheMu.Lock()
	defer modCacheMu.Unlock()
	if path, err := safeJoin(root, ".mcl/"+modCacheFile); err == nil {
		_ = atomicJSON(path, modCache{V: modCacheVersion, Files: files})
	}
}

// modWorkers hashes and parses new or changed jars in parallel.
const modWorkers = 6

// Keep concurrent cold scans from allocating hundreds of MB per worker.
// Larger jars use a bounded buffer; fingerprints retain the existing 256 MB
// limit (MurmurHash2 needs the filtered length before hashing).
const maxInMemoryJar = 1 << 20
const maxFingerprintJar = 256 << 20

var modScanSlots = make(chan struct{}, modWorkers)

// readJarOnce uses one in-memory read for small jars and bounded streaming
// passes for large ones, returning the same hashes and metadata for both.
func readJarOnce(path string, size int64, m *Mod) (sha string, fp uint32, hasFP bool, err error) {
	return readJarContext(context.Background(), path, size, m)
}

func readJarContext(ctx context.Context, path string, size int64, m *Mod) (sha string, fp uint32, hasFP bool, err error) {
	if err := ctx.Err(); err != nil {
		return "", 0, false, err
	}
	if size > maxInMemoryJar {
		f, e := os.Open(path)
		if e != nil {
			return "", 0, false, e
		}
		defer f.Close()
		h := sha1.New()
		buf := make([]byte, 64<<10)
		filtered := uint32(0)
		hasFP = size <= maxFingerprintJar
		for {
			if e := ctx.Err(); e != nil {
				return "", 0, false, e
			}
			n, e := f.Read(buf)
			_, _ = h.Write(buf[:n])
			if hasFP {
				for _, v := range buf[:n] {
					if v != 9 && v != 10 && v != 13 && v != 32 {
						filtered++
					}
				}
			}
			if e == io.EOF {
				break
			}
			if e != nil {
				return "", 0, false, e
			}
		}
		if hasFP {
			if _, e := f.Seek(0, io.SeekStart); e != nil {
				return "", 0, false, e
			}
			if fp, e = streamFingerprint(ctx, f, filtered, buf); e != nil {
				return "", 0, false, e
			}
		}
		if e := ctx.Err(); e != nil {
			return "", 0, false, e
		}
		if zr, e := zip.NewReader(f, size); e == nil {
			readModMetadataFrom(zr, f, m)
		} else {
			m.Status = "invalid"
		}
		return hex.EncodeToString(h.Sum(nil)), fp, hasFP, ctx.Err()
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return "", 0, false, e
	}
	sum := sha1.Sum(b)
	if zr, e := zip.NewReader(bytes.NewReader(b), int64(len(b))); e == nil {
		readModMetadataFrom(zr, bytes.NewReader(b), m)
	} else {
		m.Status = "invalid"
	}
	return hex.EncodeToString(sum[:]), curseFingerprint(b), true, nil
}

// streamFingerprint hashes the filtered stream without holding the jar in
// memory. Word assembly continues across read boundaries, including whitespace.
func streamFingerprint(ctx context.Context, r io.Reader, filtered uint32, buf []byte) (uint32, error) {
	const mul uint32 = 0x5bd1e995
	h, k, got := uint32(1)^filtered, uint32(0), uint(0)
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		n, err := r.Read(buf)
		for _, v := range buf[:n] {
			if v == 9 || v == 10 || v == 13 || v == 32 {
				continue
			}
			k |= uint32(v) << (8 * got)
			got++
			if got == 4 {
				k *= mul
				k ^= k >> 24
				k *= mul
				h = h*mul ^ k
				k, got = 0, 0
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
	}
	if got > 0 {
		h = (h ^ k) * mul
	}
	h ^= h >> 13
	h *= mul
	h ^= h >> 15
	return h, nil
}

func metaOnly(m Mod) Mod {
	return Mod{ModID: m.ModID, Name: m.Name, Version: m.Version, Loader: m.Loader, Status: m.Status, Description: m.Description, Provides: m.Provides, Requires: m.Requires}
}

// scanModsStream lists root/mods. Cached jars are ready at once; new or
// changed ones are read by a worker pool. emit (optional) gets the whole
// list, in its final order, whenever more rows are ready: unread rows are
// placeholders with Pending set. Row order is fixed up front (cached name,
// else file name) so streaming never reshuffles rows.
func scanModsStream(root string, emit func([]Mod)) ([]Mod, error) {
	return scanModsStreamContext(context.Background(), root, emit)
}

func scanModsStreamContext(ctx context.Context, root string, emit func([]Mod)) ([]Mod, error) {
	modsDir, e := safeJoin(root, "mods")
	if e != nil {
		return nil, e
	}
	entries, e := readDirRetryContext(ctx, modsDir)
	if os.IsNotExist(e) {
		return []Mod{}, nil
	}
	if e != nil {
		return nil, e
	}
	records, checks := readRecords(root), readChecks(root)
	cache := loadModCache(root)
	type job struct {
		i    int
		path string
		st   os.FileInfo
	}
	var out []Mod
	var jobs []job
	fresh := map[string]modCacheEntry{}
	changed := false
	for _, entry := range entries {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		name := entry.Name()
		if !entry.Type().IsRegular() || (!strings.HasSuffix(name, ".jar") && !strings.HasSuffix(name, ".jar.disabled")) {
			continue
		}
		path, e := safeJoin(root, "mods/"+name)
		if e != nil {
			return nil, e
		}
		st, e := os.Stat(path)
		if e != nil {
			return nil, e
		}
		m := Mod{File: name, Name: strings.TrimSuffix(strings.TrimSuffix(name, ".disabled"), ".jar"), Enabled: !strings.HasSuffix(name, ".disabled"), Size: st.Size(), Status: "unchecked"}
		if c, ok := cache[name]; ok && c.Size == st.Size() && c.MTime == st.ModTime().UnixNano() && c.SHA1 != "" {
			applyMeta(&m, c.Meta)
			m.SHA1, m.Fingerprint = c.SHA1, c.FP
			fresh[name] = c
		} else {
			m.Pending = true
			jobs = append(jobs, job{i: len(out), path: path, st: st})
			changed = true
		}
		out = append(out, m)
	}
	if len(fresh) != len(cache) {
		changed = true // files removed
	}
	order := make([]int, len(out))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return strings.ToLower(out[order[a]].Name) < strings.ToLower(out[order[b]].Name)
	})
	finish := func(m Mod) Mod {
		rec := records[strings.TrimSuffix(m.File, ".disabled")]
		if rec.SHA1 != "" && rec.SHA1 == m.SHA1 {
			m.Source, m.ProjectID, m.VersionID = rec.Source, rec.ProjectID, rec.VersionID
		}
		if c, ok := checks[m.SHA1]; ok && m.SHA1 != "" {
			m.Source, m.ProjectID, m.VersionID, m.Latest, m.Status = c.Source, c.ProjectID, c.VersionID, c.Latest, c.Status
		}
		return m
	}
	var mu sync.Mutex
	snapshot := func() []Mod {
		list := make([]Mod, 0, len(out))
		for _, i := range order {
			m := out[i]
			if !m.Pending {
				m = finish(m)
			}
			list = append(list, m)
		}
		return list
	}
	if emit != nil && len(jobs) > 0 {
		emit(snapshot())
	}
	if len(jobs) > 0 {
		ch := make(chan job)
		var wg sync.WaitGroup
		var firstErr error
		last := time.Now()
		for w := 0; w < modWorkers && w < len(jobs); w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range ch {
					select {
					case modScanSlots <- struct{}{}:
					case <-ctx.Done():
						return
					}
					var meta Mod
					sha, fp, hasFP, e := readJarContext(ctx, j.path, j.st.Size(), &meta)
					<-modScanSlots
					mu.Lock()
					if e != nil {
						if firstErr == nil {
							firstErr = e
						}
						mu.Unlock()
						continue
					}
					m := &out[j.i]
					applyMeta(m, meta)
					m.SHA1, m.Fingerprint, m.Pending = sha, fp, false
					fresh[m.File] = modCacheEntry{Size: j.st.Size(), MTime: j.st.ModTime().UnixNano(), SHA1: sha, FP: fp, HasFP: hasFP, Meta: metaOnly(meta)}
					var snap []Mod
					if emit != nil && time.Since(last) > 150*time.Millisecond {
						last = time.Now()
						snap = snapshot()
					}
					if snap != nil {
						emit(snap)
					}
					mu.Unlock()
				}
			}()
		}
	feed:
		for _, j := range jobs {
			select {
			case ch <- j:
			case <-ctx.Done():
				break feed
			}
		}
		close(ch)
		wg.Wait()
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if firstErr != nil {
			return nil, firstErr
		}
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if changed {
		saveModCache(root, fresh)
	}
	return snapshot(), nil
}

func applyMeta(m *Mod, c Mod) {
	m.ModID, m.Version, m.Loader, m.Description = c.ModID, c.Version, c.Loader, c.Description
	if c.Name != "" {
		m.Name = c.Name
	}
	m.Provides, m.Requires = c.Provides, c.Requires
	if c.Status == "invalid" {
		m.Status = c.Status
	}
}
