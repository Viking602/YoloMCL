package launcher

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// modWorkers hashes and parses new or changed jars in parallel.
const modWorkers = 6

// Keep concurrent cold scans from allocating hundreds of MB per worker.
// Larger jars use a bounded buffer; fingerprints retain the existing 256 MB
// limit (MurmurHash2 needs the filtered length before hashing).
const maxInMemoryJar = 256 << 10
const maxFingerprintJar = 256 << 20

// readJarOnce uses one in-memory read for small jars and bounded streaming
// passes for large ones, returning the same hashes and metadata for both.
func readJarOnce(path string, size int64, m *Mod) (sha string, fp uint32, hasFP bool, err error) {
	return readJarContext(context.Background(), path, size, m)
}

func readJarContext(ctx context.Context, path string, size int64, m *Mod) (sha string, fp uint32, hasFP bool, err error) {
	w := beginModScan()
	defer endModScan()
	return w.readJar(ctx, path, size, m)
}

func (w *modScanWorkspace) readJar(ctx context.Context, path string, size int64, m *Mod) (sha string, fp uint32, hasFP bool, err error) {
	return w.readJarCached(ctx, path, size, m, nil, nil, nil)
}

// identity comes only from a verified download into our private staging area.
// Arbitrary files always compute their SHA-1 before consulting the shared index.
func (w *modScanWorkspace) readJarCached(ctx context.Context, path string, size int64, m *Mod, cache *modMetadataSession, identity *downloadIdentity, stamp os.FileInfo) (sha string, fp uint32, hasFP bool, err error) {
	hit := false
	defer func() {
		if err == nil && ctx.Err() == nil {
			cache.put(ctx, modCacheEntry{Size: size, SHA1: sha, FP: fp, HasFP: hasFP, Meta: metaOnly(*m)})
		}
	}()
	if identity != nil {
		sha = identity.SHA1
		if hit, ok := cache.lookup(ctx, sha, size); ok {
			applyMeta(m, hit.Meta)
			return sha, hit.FP, hit.HasFP, nil
		}
	}
	scratch, err := w.acquire(ctx)
	if err != nil {
		return "", 0, false, err
	}
	defer func() { w.workers <- scratch }()
	if err := ctx.Err(); err != nil {
		return "", 0, false, err
	}
	f, e := os.Open(path)
	if e != nil {
		return "", 0, false, e
	}
	defer f.Close()
	defer func() {
		// Reuse the directory scan's stamp. Only newly parsed metadata needs
		// a final check against concurrent edits before it enters the index.
		if err != nil || stamp == nil || hit {
			return
		}
		after, statErr := f.Stat()
		if statErr != nil {
			err = statErr
			return
		}
		if after.Size() != stamp.Size() || !after.ModTime().Equal(stamp.ModTime()) {
			err = fmt.Errorf("模组文件在读取过程中发生变化: %s", path)
		}
	}()
	if size > maxInMemoryJar {
		h := sha1.New()
		scratch.jar = resizeModBuffer(scratch.jar, modReadChunk)
		buf := scratch.jar
		filtered := uint32(0)
		hasFP = size <= maxFingerprintJar
		if identity != nil {
			filtered = identity.Filtered
		}
		for identity == nil {
			if e := ctx.Err(); e != nil {
				return "", 0, false, e
			}
			n, e := f.Read(buf)
			_, _ = h.Write(buf[:n])
			if hasFP {
				filtered += uint32(fingerprintLength(buf[:n]))
			}
			if e == io.EOF {
				break
			}
			if e != nil {
				return "", 0, false, e
			}
		}
		if identity == nil {
			sha = hex.EncodeToString(h.Sum(nil))
			if cached, ok := cache.lookup(ctx, sha, size); ok {
				hit = true
				applyMeta(m, cached.Meta)
				return sha, cached.FP, cached.HasFP, ctx.Err()
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
		w.readMetadata(ctx, f, size, scratch, m)
		return sha, fp, hasFP, ctx.Err()
	}
	if size < 0 {
		return "", 0, false, zip.ErrFormat
	}
	scratch.jar = resizeModBuffer(scratch.jar, int(size))
	b := scratch.jar
	if e := readModBytes(ctx, f, b); e != nil {
		return "", 0, false, e
	}
	if identity == nil {
		sum := sha1.Sum(b)
		sha = hex.EncodeToString(sum[:])
		if cached, ok := cache.lookup(ctx, sha, size); ok {
			hit = true
			applyMeta(m, cached.Meta)
			return sha, cached.FP, cached.HasFP, ctx.Err()
		}
	}
	w.readMetadata(ctx, bytes.NewReader(b), int64(len(b)), scratch, m)
	if err := ctx.Err(); err != nil {
		return "", 0, false, err
	}
	return sha, curseFingerprint(b), true, ctx.Err()
}

// fingerprintFile bounds temporary memory even for large imported packs.
// Count using byte searches so the extra streaming pass does not add a
// per-byte branch loop; both passes reuse the same small buffer.
func fingerprintFile(ctx context.Context, path string) (uint32, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	buf := make([]byte, 64<<10)
	var filtered uint32
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		n, err := f.Read(buf)
		filtered += uint32(fingerprintLength(buf[:n]))
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	return streamFingerprint(ctx, f, filtered, buf)
}

// Read-only lookup: compressed jar contents almost always contain runs without
// whitespace. Combining four lookups avoids four byte-assembly iterations.
var fingerprintWhitespace = [256]uint8{9: 1, 10: 1, 13: 1, 32: 1}

func mixFingerprintWord(h, word uint32) uint32 {
	const mul uint32 = 0x5bd1e995
	word *= mul
	word ^= word >> 24
	word *= mul
	return h*mul ^ word
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
		for i := 0; i < n; {
			if got == 0 && i+4 <= n && fingerprintWhitespace[buf[i]]|fingerprintWhitespace[buf[i+1]]|fingerprintWhitespace[buf[i+2]]|fingerprintWhitespace[buf[i+3]] == 0 {
				h = mixFingerprintWord(h, binary.LittleEndian.Uint32(buf[i:i+4]))
				i += 4
				continue
			}
			v := buf[i]
			i++
			if v == 9 || v == 10 || v == 13 || v == 32 {
				continue
			}
			k |= uint32(v) << (8 * got)
			got++
			if got == 4 {
				h = mixFingerprintWord(h, k)
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
	return scanModsCached(ctx, root, emit, nil)
}

func (s *Service) scanModDirectory(ctx context.Context, root string, emit func([]Mod)) ([]Mod, error) {
	cache := s.modMetadataCache().begin(ctx)
	defer cache.close()
	return scanModsCached(ctx, root, emit, cache)
}

func scanModsCached(ctx context.Context, root string, emit func([]Mod), shared *modMetadataSession) ([]Mod, error) {
	return scanModsIdentified(ctx, root, emit, shared, nil)
}

func scanModsIdentified(ctx context.Context, root string, emit func([]Mod), shared *modMetadataSession, verified map[string]verifiedModFile) ([]Mod, error) {
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
	stored := loadModCacheData(root)
	cache := stored.Files
	type job struct {
		i    int
		path string
		st   os.FileInfo
	}
	out := make([]Mod, 0, len(entries))
	var jobs []job
	fresh := make(map[string]modCacheEntry, len(entries))
	changed := false
	completed := 0
	defer func() {
		if ctx.Err() == nil || completed == 0 {
			return
		}
		// Navigation cancels content IO immediately. Preserve only completed,
		// stamp-checked entries so returning does not rehash those JARs. This
		// bounded metadata flush owns no scan buffers and respects cache clears.
		flush, cancel := context.WithTimeout(context.WithoutCancel(ctx), 250*time.Millisecond)
		defer cancel()
		if shared != nil {
			shared.checkpointLocal(flush, root, fresh)
		} else {
			checkpointModCache(flush, root, fresh, false)
		}
	}()
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
		st, e := entry.Info() // free on Windows; regular files need no link follow
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
	names := make([]string, len(out))
	for i := range order {
		order[i] = i
		names[i] = strings.ToLower(out[i].Name)
	}
	sort.SliceStable(order, func(a, b int) bool {
		return names[order[a]] < names[order[b]]
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
		workspace := beginModScan()
		defer endModScan()
		ch := make(chan job)
		var wg sync.WaitGroup
		var firstErr error
		last := time.Now()
		for w := 0; w < modWorkers && w < len(jobs); w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range ch {
					if ctx.Err() != nil {
						return
					}
					var meta Mod
					var identity *downloadIdentity
					if known, ok := verified[out[j.i].File]; ok && known.Size == j.st.Size() && known.mtime == j.st.ModTime().UnixNano() {
						identity = &known.downloadIdentity
					}
					sha, fp, hasFP, e := workspace.readJarCached(ctx, j.path, j.st.Size(), &meta, shared, identity, j.st)
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
					completed++
					var snap []Mod
					if emit != nil && (completed == 1 || time.Since(last) > 150*time.Millisecond) {
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
	if shared != nil && !stored.Shared {
		// Migrate valid existing entries once, without reparsing their jars.
		// Publish ready rows before this optional background cache maintenance.
		if emit != nil {
			emit(snapshot())
		}
		for _, entry := range fresh {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			shared.put(ctx, entry)
		}
		changed = true
	}
	if changed {
		if shared != nil {
			shared.saveLocal(ctx, root, fresh)
		} else {
			saveModCache(root, fresh)
		}
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
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
