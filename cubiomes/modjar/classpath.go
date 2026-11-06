package modjar

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"sync/atomic"
)

// Archive is an already selected loader input. Embedded JARs are followed only
// when their active loader's metadata declares them.
type Archive struct {
	Name   string
	Reader *zip.Reader
}
type classEntry struct {
	source string
	entry  *zip.File
}

// Classpath resolves exact class names, including declared nested libraries.
// It never extracts or executes files. Root readers must remain open while the
// classpath is in use. Conflicting definitions fail instead of choosing a
// different dependency version according to archive enumeration order.
type Classpath struct {
	loader          string
	ctx             context.Context
	archives        []Archive
	entries         map[string][]classEntry
	bytes           int64
	retained        int64
	indexed         int
	streams         []*nestedReaderAt
	closed          atomic.Bool
	contained       []containedJar
	selectionErrors map[string]string
}

// Resources preserves the loader's archive order, including declared nested
// dependencies. It does not search other installations or extract archive data.
func (c *Classpath) Resources(name string) ([][]byte, error) {
	if c.closed.Load() {
		return nil, fmt.Errorf("closed classpath")
	}
	if name == "" || path.Clean(name) != name || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") || strings.HasPrefix(name, "../") {
		return nil, fmt.Errorf("invalid resource name")
	}
	var out [][]byte
	total := 0
	for _, a := range c.archives {
		if err := c.ctx.Err(); err != nil {
			return nil, err
		}
		var found []byte
		hasResource := false
		for _, f := range a.Reader.File {
			if f.Name != name {
				continue
			}
			if issue := c.selectionErrors[a.Name]; issue != "" {
				return nil, fmt.Errorf("%s", issue)
			}
			if f.UncompressedSize64 > 64<<10 {
				return nil, fmt.Errorf("resource size budget: %s", name)
			}
			r, err := f.Open()
			if err != nil {
				return nil, err
			}
			b, err := io.ReadAll(io.LimitReader(r, (64<<10)+1))
			r.Close()
			if err != nil {
				return nil, err
			}
			total += len(b)
			if len(b) > 64<<10 || total > 1<<20 || len(out) >= 256 {
				return nil, fmt.Errorf("resource budget: %s", name)
			}
			if hasResource && !bytes.Equal(found, b) {
				return nil, fmt.Errorf("ambiguous resource %s!/%s", a.Name, name)
			}
			found = b
			hasResource = true
		}
		if hasResource {
			out = append(out, found)
		}
	}
	return out, nil
}

func NewClasspath(ctx context.Context, roots []Archive, loader string) (*Classpath, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch loader {
	case "forge", "neoforge", "fabric", "quilt":
	default:
		return nil, fmt.Errorf("unbound archive loader %q", loader)
	}
	c := &Classpath{ctx: ctx, entries: map[string][]classEntry{}, loader: loader}
	for _, root := range roots {
		if err := c.add(root, loader, 0); err != nil {
			c.Close()
			return nil, fmt.Errorf("classpath %s: %w", root.Name, err)
		}
	}
	if loader == "forge" || loader == "neoforge" {
		c.selectContained()
	}
	if loader == "fabric" || loader == "quilt" {
		c.selectFabricMods()
	}
	return c, nil
}

func (c *Classpath) Archives() []Archive { return append([]Archive(nil), c.archives...) }

// ReflectionOpen recognizes input jars in the unnamed/automatic-module class
// space. Explicit module descriptors require module-specific opens resolution;
// they are never assumed to grant private access. No host classes are exposed.
func (c *Classpath) ReflectionOpen(name string) (bool, error) {
	if err := c.ctx.Err(); err != nil {
		return false, err
	}
	entries := c.entries[name]
	if len(entries) == 0 {
		return false, nil
	}
	if c.loader == "fabric" || c.loader == "quilt" {
		return true, nil
	}
	for _, entry := range entries {
		for _, archive := range c.archives {
			if archive.Name == entry.source {
				for _, file := range archive.Reader.File {
					if file.Name == "module-info.class" || strings.HasPrefix(file.Name, "META-INF/versions/") && strings.HasSuffix(file.Name, "/module-info.class") {
						return false, nil
					}
				}
			}
		}
	}
	return true, nil
}

// Close releases retained decompression streams. It does not close root
// archives, which remain owned by the caller. Further class reads fail.
func (c *Classpath) Close() error {
	if c.closed.Swap(true) {
		return nil
	}
	var first error
	for i := len(c.streams) - 1; i >= 0; i-- {
		if err := c.streams[i].Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func archiveBytes(ctx context.Context, entry *zip.File, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if entry.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("archive entry exceeds limit: %s", entry.Name)
	}
	f, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("archive entry exceeds limit: %s", entry.Name)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return b, nil
}

func (c *Classpath) add(a Archive, loader string, depth int) error {
	if a.Reader == nil || a.Name == "" {
		return fmt.Errorf("invalid classpath archive")
	}
	if depth > 8 || len(c.archives) >= 4096 {
		return fmt.Errorf("nested archive count/depth limit")
	}
	if err := c.ctx.Err(); err != nil {
		return err
	}
	c.archives = append(c.archives, a)
	files := map[string]*zip.File{}
	duplicates := map[string]bool{}
	for _, entry := range a.Reader.File {
		if _, exists := files[entry.Name]; exists {
			duplicates[entry.Name] = true
		}
		files[entry.Name] = entry
		if strings.HasSuffix(entry.Name, ".class") && !strings.HasPrefix(entry.Name, "META-INF/") {
			name := strings.TrimSuffix(entry.Name, ".class")
			if c.indexed >= 1_000_000 {
				return fmt.Errorf("classpath index limit")
			}
			c.indexed++
			c.entries[name] = append(c.entries[name], classEntry{a.Name, entry})
		}
	}
	var nested []string
	metadataName := "META-INF/jarjar/metadata.json"
	if loader == "fabric" || loader == "quilt" {
		metadataName = "fabric.mod.json"
		if loader == "quilt" && files["quilt.mod.json"] != nil {
			metadataName = "quilt.mod.json"
		}
	}
	if duplicates[metadataName] {
		return fmt.Errorf("ambiguous metadata %s!/%s", a.Name, metadataName)
	}
	switch loader {
	case "forge", "neoforge":
		if entry := files["META-INF/jarjar/metadata.json"]; entry != nil {
			b, err := archiveBytes(c.ctx, entry, 2<<20)
			if err != nil {
				return err
			}
			var meta struct {
				Jars []containedJar `json:"jars"`
			}
			if err := json.Unmarshal(loaderJSON(b), &meta); err != nil {
				return fmt.Errorf("nested library metadata: %w", err)
			}
			for _, jar := range meta.Jars {
				nested = append(nested, jar.Path)
				jar.source = a.Name + "!/" + jar.Path
				c.contained = append(c.contained, jar)
			}
		}
	case "fabric", "quilt":
		if entry := files["fabric.mod.json"]; entry != nil && (loader != "quilt" || files["quilt.mod.json"] == nil) {
			b, err := archiveBytes(c.ctx, entry, 2<<20)
			if err != nil {
				return err
			}
			var meta struct {
				Jars []struct {
					File string `json:"file"`
				} `json:"jars"`
			}
			if err := json.Unmarshal(loaderJSON(b), &meta); err != nil {
				return err
			}
			for _, jar := range meta.Jars {
				nested = append(nested, jar.File)
			}
		}
		if loader == "quilt" {
			if entry := files["quilt.mod.json"]; entry != nil {
				b, err := archiveBytes(c.ctx, entry, 2<<20)
				if err != nil {
					return err
				}
				var meta struct {
					Loader struct {
						Jars []string `json:"jars"`
					} `json:"quilt_loader"`
				}
				if err := json.Unmarshal(loaderJSON(b), &meta); err != nil {
					return err
				}
				nested = append(nested, meta.Loader.Jars...)
			}
		}
	default:
		return fmt.Errorf("unbound archive loader %q", loader)
	}
	seen := map[string]bool{}
	for _, name := range nested {
		if duplicates[name] {
			return fmt.Errorf("ambiguous nested library %s!/%s", a.Name, name)
		}
		if name == "" || path.Clean(name) != name || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") || strings.ContainsAny(name, "\\:") {
			return fmt.Errorf("invalid nested archive path %q", name)
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		entry := files[name]
		if entry == nil {
			return fmt.Errorf("declared library missing: %s!/%s", a.Name, name)
		}
		if entry.UncompressedSize64 > 512<<20 {
			return fmt.Errorf("nested archive size limit")
		}
		size := int64(entry.UncompressedSize64)
		c.bytes += size
		if c.bytes > 2<<30 {
			return fmt.Errorf("nested library inspection limit")
		}
		var reader io.ReaderAt
		if size <= 8<<20 && c.retained+size <= 16<<20 {
			b, err := archiveBytes(c.ctx, entry, 8<<20)
			if err != nil {
				return err
			}
			reader = bytes.NewReader(b)
			c.retained += int64(len(b))
		} else {
			// Large embedded libraries often contain native binaries unrelated to
			// generation. Index their directory through a bounded read window;
			// they must not inflate the retained class or archive memory budgets.
			stream := &nestedReaderAt{ctx: c.ctx, entry: entry}
			reader = stream
			c.streams = append(c.streams, stream)
		}
		z, err := zip.NewReader(reader, size)
		if err != nil {
			return err
		}
		if err := c.add(Archive{a.Name + "!/" + name, z}, loader, depth+1); err != nil {
			return err
		}
	}
	return nil
}

type nestedReaderAt struct {
	mu       sync.Mutex
	ctx      context.Context
	entry    *zip.File
	offset   int64
	window   []byte
	stream   io.ReadCloser
	position int64
	closed   bool
}

func (r *nestedReaderAt) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	r.window = nil
	if r.stream != nil {
		err := r.stream.Close()
		r.stream = nil
		return err
	}
	return nil
}

func (r *nestedReaderAt) ReadAt(dst []byte, offset int64) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return 0, fmt.Errorf("closed nested archive")
	}
	if offset < 0 {
		return 0, fmt.Errorf("negative nested archive offset")
	}
	n := 0
	for len(dst) > 0 {
		if err := r.ctx.Err(); err != nil {
			return n, err
		}
		if offset >= int64(r.entry.UncompressedSize64) {
			return n, io.EOF
		}
		if offset < r.offset || offset >= r.offset+int64(len(r.window)) {
			var err error
			// ZIP directory reads advance through adjacent windows. Keep their
			// inflater cursor so a large library is not decompressed from zero
			// for every 64 KiB of its central directory.
			if r.stream == nil || offset < r.position {
				if r.stream != nil {
					r.stream.Close()
				}
				r.stream, err = r.entry.Open()
				r.position = 0
				if err != nil {
					return n, err
				}
			}
			remaining := offset - r.position
			var scratch [32 << 10]byte
			for remaining > 0 && err == nil {
				if err = r.ctx.Err(); err != nil {
					break
				}
				var read int
				read, err = io.ReadFull(r.stream, scratch[:min(int64(len(scratch)), remaining)])
				remaining -= int64(read)
				r.position += int64(read)
			}
			if err == nil {
				size := int(min(int64(64<<10), int64(r.entry.UncompressedSize64)-offset))
				if cap(r.window) < size {
					r.window = make([]byte, size)
				} else {
					r.window = r.window[:size]
				}
				r.offset = offset
				var read int
				read, err = io.ReadFull(r.stream, r.window)
				r.position += int64(read)
			}
			if err != nil {
				r.window = nil
				r.stream.Close()
				r.stream = nil
				return n, err
			}
		}
		copied := copy(dst, r.window[offset-r.offset:])
		n += copied
		offset += int64(copied)
		dst = dst[copied:]
	}
	return n, nil
}

func (c *Classpath) Class(name string) ([]byte, error) {
	if c.closed.Load() {
		return nil, fmt.Errorf("closed classpath")
	}
	if err := c.ctx.Err(); err != nil {
		return nil, err
	}
	var found []byte
	var origin string
	for _, candidate := range c.entries[name] {
		if issue := c.selectionErrors[candidate.source]; issue != "" {
			return nil, fmt.Errorf("%s", issue)
		}
		b, err := archiveBytes(c.ctx, candidate.entry, 2<<20)
		if err != nil {
			return nil, err
		}
		if found != nil && !bytes.Equal(found, b) {
			return nil, fmt.Errorf("conflicting class %s in %s and %s", name, origin, candidate.source)
		}
		found, origin = b, candidate.source
	}
	return found, nil
}
