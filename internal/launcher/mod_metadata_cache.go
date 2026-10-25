package launcher

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
)

const (
	sharedModCacheFile    = "cache/mod-metadata.bin"
	sharedModCacheMagic   = "YMCmods1"
	sharedModCacheBytes   = 8 << 20
	sharedModCacheEntries = 4096
	sharedModRecordBytes  = 64 << 10
	sharedModHeaderBytes  = 40
)

type modMetadataLocation struct {
	offset   int64
	size     int64
	length   uint32
	checksum uint32
}

// modMetadataCache stores only metadata, never jar bytes or decoded artwork.
// The offset directory and file handle exist only while a scan/install uses
// them. The gate also orders ClearModCache against publication of local caches.
type modMetadataCache struct {
	gate        chan struct{}
	path        string
	generation  uint64
	users       int
	file        *os.File
	entries     map[[20]byte]modMetadataLocation
	end         int64
	unavailable bool
}

type modMetadataSession struct {
	cache      *modMetadataCache
	generation uint64
}

func (s *Service) modMetadataCache() *modMetadataCache {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.modMetadata == nil {
		s.modMetadata = &modMetadataCache{gate: make(chan struct{}, 1), path: filepath.Join(s.dir, filepath.FromSlash(sharedModCacheFile))}
	}
	return s.modMetadata
}

func (c *modMetadataCache) lock(ctx context.Context) bool {
	if ctx.Err() != nil {
		return false
	}
	select {
	case c.gate <- struct{}{}:
		if ctx.Err() == nil {
			return true
		}
		c.unlock()
	case <-ctx.Done():
	}
	return false
}

func (c *modMetadataCache) unlock() { <-c.gate }

func (c *modMetadataCache) begin(ctx context.Context) *modMetadataSession {
	if !c.lock(ctx) {
		return nil
	}
	defer c.unlock()
	if c.users == 0 {
		c.unavailable = false
	}
	c.users++
	return &modMetadataSession{cache: c, generation: c.generation}
}

func (s *modMetadataSession) close() {
	if s == nil {
		return
	}
	c := s.cache
	c.lock(context.Background())
	defer c.unlock()
	c.users--
	if c.users == 0 {
		c.release()
	}
}

func (c *modMetadataCache) release() {
	if c.file != nil {
		_ = c.file.Close()
	}
	c.file, c.entries, c.end = nil, nil, 0
}

// open rebuilds a small offset directory without decoding the metadata. A
// cancelled open leaves disk untouched; incomplete trailing records are dropped.
func (c *modMetadataCache) open(ctx context.Context) bool {
	if c.file != nil {
		return true
	}
	if c.unavailable {
		return false
	}
	c.unavailable = true
	base := filepath.Dir(filepath.Dir(c.path))
	path, err := safeJoin(base, sharedModCacheFile)
	if err != nil || os.MkdirAll(filepath.Dir(path), 0700) != nil {
		return false
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return false
	}
	ok := false
	defer func() {
		if !ok {
			_ = f.Close()
		}
	}()
	st, err := f.Stat()
	if err != nil {
		return false
	}
	r := bufio.NewReaderSize(f, modReadChunk)
	var magic [len(sharedModCacheMagic)]byte
	_, err = io.ReadFull(r, magic[:])
	entries := make(map[[20]byte]modMetadataLocation)
	end := int64(len(magic))
	valid := err == nil && string(magic[:]) == sharedModCacheMagic && st.Size() <= sharedModCacheBytes
	if valid {
		var header [sharedModHeaderBytes]byte
		for end < st.Size() {
			if ctx.Err() != nil {
				return false
			}
			if _, err = io.ReadFull(r, header[:]); err != nil {
				break
			}
			n := binary.LittleEndian.Uint32(header[20:24])
			if n == 0 || n > sharedModRecordBytes || crc32.ChecksumIEEE(header[:36]) != binary.LittleEndian.Uint32(header[36:]) {
				break
			}
			if _, err = r.Discard(int(n)); err != nil {
				break
			}
			var hash [20]byte
			copy(hash[:], header[:20])
			if _, exists := entries[hash]; !exists && len(entries) == sharedModCacheEntries {
				break
			}
			entries[hash] = modMetadataLocation{offset: end, size: int64(binary.LittleEndian.Uint64(header[24:32])), length: n, checksum: binary.LittleEndian.Uint32(header[32:36])}
			end += sharedModHeaderBytes + int64(n)
		}
	}
	if ctx.Err() != nil {
		return false
	}
	if !valid {
		if err = f.Truncate(0); err != nil {
			return false
		}
		if _, err = f.WriteAt([]byte(sharedModCacheMagic), 0); err != nil {
			return false
		}
	} else if end != st.Size() {
		if err = f.Truncate(end); err != nil {
			return false
		}
	}
	c.file, c.entries, c.end, c.unavailable = f, entries, end, false
	ok = true
	return true
}

func modMetadataKey(sha string) (key [20]byte, ok bool) {
	if len(sha) != hex.EncodedLen(len(key)) {
		return key, false
	}
	n, err := hex.Decode(key[:], []byte(sha))
	return key, err == nil && n == len(key)
}

func (s *modMetadataSession) lookup(ctx context.Context, sha string, size int64) (modCacheEntry, bool) {
	var out modCacheEntry
	if s == nil {
		return out, false
	}
	key, ok := modMetadataKey(sha)
	c := s.cache
	if !ok || !c.lock(ctx) {
		return out, false
	}
	defer c.unlock()
	if s.generation != c.generation || !c.open(ctx) {
		return out, false
	}
	loc, ok := c.entries[key]
	if !ok || loc.size != size {
		return out, false
	}
	b := make([]byte, loc.length)
	_, err := c.file.ReadAt(b, loc.offset+sharedModHeaderBytes)
	if err != nil || crc32.ChecksumIEEE(b) != loc.checksum || json.Unmarshal(b, &out) != nil {
		delete(c.entries, key)
		return modCacheEntry{}, false
	}
	out.SHA1, out.Size = sha, size
	return out, ctx.Err() == nil
}

func (s *modMetadataSession) put(ctx context.Context, entry modCacheEntry) {
	if s == nil || ctx.Err() != nil {
		return
	}
	key, ok := modMetadataKey(entry.SHA1)
	c := s.cache
	if !ok || !c.lock(ctx) {
		return
	}
	defer c.unlock()
	if s.generation != c.generation || !c.open(ctx) {
		return
	}
	if old, exists := c.entries[key]; exists && old.size == entry.Size {
		return
	}
	size := entry.Size
	entry.Size, entry.MTime, entry.SHA1 = 0, 0, ""
	entry.Meta = metaOnly(entry.Meta)
	b, err := json.Marshal(entry)
	if err != nil || len(b) > sharedModRecordBytes {
		return
	}
	length := int64(sharedModHeaderBytes + len(b))
	if c.end+length > sharedModCacheBytes || len(c.entries) >= sharedModCacheEntries {
		if !c.compact(ctx) {
			return
		}
	}
	if ctx.Err() != nil {
		return
	}
	var h [sharedModHeaderBytes]byte
	copy(h[:20], key[:])
	binary.LittleEndian.PutUint32(h[20:24], uint32(len(b)))
	binary.LittleEndian.PutUint64(h[24:32], uint64(size))
	checksum := crc32.ChecksumIEEE(b)
	binary.LittleEndian.PutUint32(h[32:36], checksum)
	binary.LittleEndian.PutUint32(h[36:], crc32.ChecksumIEEE(h[:36]))
	if _, err = c.file.WriteAt(h[:], c.end); err != nil {
		return
	}
	if _, err = c.file.WriteAt(b, c.end+sharedModHeaderBytes); err != nil {
		return
	}
	c.entries[key] = modMetadataLocation{offset: c.end, size: size, length: uint32(len(b)), checksum: checksum}
	c.end += length
}

// compact evicts the oldest inserted records in batches. It copies at most
// one 64 KiB record at a time, leaving headroom to amortize disk maintenance.
func (c *modMetadataCache) compact(ctx context.Context) bool {
	keys := make([][20]byte, 0, len(c.entries))
	for key := range c.entries {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return c.entries[keys[i]].offset > c.entries[keys[j]].offset })
	keep, size := 0, int64(len(sharedModCacheMagic))
	for _, key := range keys {
		n := int64(sharedModHeaderBytes) + int64(c.entries[key].length)
		if keep >= sharedModCacheEntries*3/4 || size+n > sharedModCacheBytes*3/4 {
			break
		}
		keep++
		size += n
	}
	// A fixed scratch name bounds crash leftovers to one compaction file.
	temp, err := safeJoin(filepath.Dir(filepath.Dir(c.path)), sharedModCacheFile+".tmp")
	if err != nil {
		return false
	}
	f, err := os.OpenFile(temp, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close(); _ = os.Remove(f.Name()) }()
	if _, err = f.WriteString(sharedModCacheMagic); err != nil {
		return false
	}
	entries := make(map[[20]byte]modMetadataLocation, keep)
	buf := make([]byte, modReadChunk)
	end := int64(len(sharedModCacheMagic))
	for i := keep - 1; i >= 0; i-- {
		if ctx.Err() != nil {
			return false
		}
		key := keys[i]
		loc := c.entries[key]
		n := int64(sharedModHeaderBytes) + int64(loc.length)
		// Hide os.File.ReadFrom so CopyBuffer actually reuses buf instead of
		// allocating another transfer buffer for every retained record.
		if _, err = io.CopyBuffer(struct{ io.Writer }{f}, io.NewSectionReader(c.file, loc.offset, n), buf); err != nil {
			return false
		}
		loc.offset = end
		entries[key] = loc
		end += n
	}
	if err = f.Close(); err != nil || ctx.Err() != nil {
		return false
	}
	_ = c.file.Close() // Windows requires closing the destination before replacement.
	c.file = nil
	if err = os.Rename(f.Name(), c.path); err == nil {
		c.entries, c.end = entries, end
	}
	c.file, _ = os.OpenFile(c.path, os.O_RDWR, 0600)
	if c.file == nil {
		c.release()
		c.unavailable = true
	}
	return err == nil && c.file != nil
}

func (s *modMetadataSession) saveLocal(ctx context.Context, root string, files map[string]modCacheEntry) {
	if s == nil {
		return
	}
	c := s.cache
	if !c.lock(ctx) {
		return
	}
	defer c.unlock()
	if s.generation == c.generation {
		saveModCacheContext(ctx, root, files, true)
	}
}

func (s *modMetadataSession) checkpointLocal(ctx context.Context, root string, files map[string]modCacheEntry) {
	if s == nil {
		return
	}
	c := s.cache
	if !c.lock(ctx) {
		return
	}
	defer c.unlock()
	if s.generation == c.generation {
		checkpointModCache(ctx, root, files, true)
	}
}
