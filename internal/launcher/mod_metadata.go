package launcher

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func readModMetadata(path string, m *Mod) {
	ctx := context.Background()
	w := beginModScan()
	defer endModScan()
	scratch, err := w.acquire(ctx)
	if err != nil {
		return
	}
	defer func() { w.workers <- scratch }()
	f, e := os.Open(path)
	if e != nil {
		m.Status = "invalid"
		return
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		m.Status = "invalid"
		return
	}
	w.readMetadata(ctx, f, st.Size(), scratch, m)
}

// readModMetadataFrom reads mod info from an opened jar; ra is the jar's
// bytes so stored (uncompressed) nested jars are read in place.
func (w *modScanWorkspace) readModMetadataFrom(ctx context.Context, r *zip.Reader, ra io.ReaderAt, scratch *modScanBuffer, m *Mod) {
scan:
	for _, f := range r.File {
		if ctx.Err() != nil {
			return
		}
		if f.UncompressedSize64 > 2<<20 {
			continue
		}
		switch f.Name {
		case "fabric.mod.json":
			var v struct {
				ID       string         `json:"id"`
				Name     string         `json:"name"`
				Version  string         `json:"version"`
				Desc     string         `json:"description"`
				Provides []string       `json:"provides"`
				Depends  map[string]any `json:"depends"`
			}
			if readModJSON(ctx, f, &v) == nil {
				m.Description = v.Desc
				m.Provides = v.Provides
				for id := range v.Depends {
					m.Requires = append(m.Requires, id)
				}
				m.ModID = v.ID
				m.Name = v.Name
				m.Version = v.Version
				m.Loader = "fabric"
				break scan
			}
		case "quilt.mod.json":
			var v struct {
				Loader struct {
					ID       string `json:"id"`
					Version  string `json:"version"`
					Metadata struct {
						Name string `json:"name"`
						Desc string `json:"description"`
					} `json:"metadata"`
					Provides []json.RawMessage `json:"provides"`
					Depends  []json.RawMessage `json:"depends"`
				} `json:"quilt_loader"`
			}
			if readModJSON(ctx, f, &v) == nil {
				m.ModID = v.Loader.ID
				m.Version = v.Loader.Version
				m.Name = v.Loader.Metadata.Name
				m.Description = v.Loader.Metadata.Desc
				m.Provides = quiltIDs(v.Loader.Provides, false)
				m.Requires = quiltIDs(v.Loader.Depends, true)
				m.Loader = "quilt"
				break scan
			}
		case "META-INF/mods.toml", "META-INF/neoforge.mods.toml":
			b, e := readZipBytesContext(ctx, f, 2<<20)
			if e != nil {
				continue
			}
			var v struct {
				Mods []struct {
					ID      string `toml:"modId"`
					Name    string `toml:"displayName"`
					Version string `toml:"version"`
					Desc    string `toml:"description"`
				} `toml:"mods"`
				Deps map[string][]struct {
					ModID     string `toml:"modId"`
					Mandatory *bool  `toml:"mandatory"`
					Type      string `toml:"type"`
				} `toml:"dependencies"`
			}
			if safeTOML(b, &v) == nil && len(v.Mods) > 0 {
				m.ModID = v.Mods[0].ID
				m.Name = v.Mods[0].Name
				m.Version = v.Mods[0].Version
				m.Description = v.Mods[0].Desc
				for owner, deps := range v.Deps {
					if !strings.EqualFold(owner, m.ModID) {
						continue
					}
					for _, dep := range deps {
						if dep.ModID != "" && (dep.Mandatory != nil && *dep.Mandatory || strings.EqualFold(dep.Type, "required")) {
							m.Requires = append(m.Requires, dep.ModID)
						}
					}
				}
				for _, extra := range v.Mods[1:] { // a jar may hold several mods
					if extra.ID != "" {
						m.Provides = append(m.Provides, extra.ID)
					}
				}
				m.Loader = "forge"
				if strings.Contains(f.Name, "neoforge") {
					m.Loader = "neoforge"
				}
			}
		}
	}
	if m.Description == "" { // pre-1.13 Forge: mcmod.info
		for _, f := range r.File {
			if ctx.Err() != nil {
				return
			}
			if f.Name == "mcmod.info" && f.UncompressedSize64 < 1<<20 {
				var list []struct {
					Desc string `json:"description"`
				}
				if readModJSON(ctx, f, &list) == nil && len(list) > 0 {
					m.Description = list[0].Desc
				}
			}
		}
	}
	m.Description = strings.Join(strings.Fields(stripFormatting(m.Description)), " ")
	m.Requires = cleanRequires(m.Requires)
	// Jar-in-jar libraries (Fabric META-INF/jars, Forge jarjar) count as
	// provided: fabric-api ships its modules this way.
	for _, f := range r.File {
		if ctx.Err() != nil {
			return
		}
		if !strings.HasSuffix(f.Name, ".jar") || f.UncompressedSize64 > maxNestedJar || !(strings.HasPrefix(f.Name, "META-INF/jars/") || strings.HasPrefix(f.Name, "META-INF/jarjar/")) {
			continue
		}
		m.Provides = append(m.Provides, w.readNestedModIDs(ctx, f, ra, scratch)...)
	}
	if strings.Contains(m.Version, "${") {
		m.Version = ""
		for _, f := range r.File {
			if ctx.Err() != nil {
				return
			}
			if f.Name == "META-INF/MANIFEST.MF" {
				b, e := readZipBytesContext(ctx, f, 2<<20)
				if e != nil {
					break
				}
				for _, line := range strings.Split(string(b), "\n") {
					if strings.HasPrefix(line, "Implementation-Version:") {
						m.Version = strings.TrimSpace(strings.TrimPrefix(line, "Implementation-Version:"))
					}
				}
			}
		}
	}
}

func fingerprintLength(data []byte) int {
	return len(data) - bytes.Count(data, []byte{9}) - bytes.Count(data, []byte{10}) - bytes.Count(data, []byte{13}) - bytes.Count(data, []byte{32})
}

// CurseForge's fingerprint is MurmurHash2(seed=1), with ASCII whitespace removed.
func curseFingerprint(data []byte) uint32 {
	ws := func(v byte) bool { return v == 9 || v == 10 || v == 13 || v == 32 }
	const mul uint32 = 0x5bd1e995
	h := uint32(1) ^ uint32(fingerprintLength(data))
	var k uint32
	got := 0
	for _, v := range data { // no copy: hash the filtered stream in place
		if ws(v) {
			continue
		}
		k |= uint32(v) << (8 * got)
		got++
		if got == 4 {
			k *= mul
			k ^= k >> 24
			k *= mul
			h *= mul
			h ^= k
			k, got = 0, 0
		}
	}
	if got > 0 {
		h ^= k
		h *= mul
	}
	h ^= h >> 13
	h *= mul
	h ^= h >> 15
	return h
}

// quiltIDs reads quilt.mod.json provides / depends entries, which are
// either plain ids or objects with "id" (and "optional" for depends).
func quiltIDs(raw []json.RawMessage, requiredOnly bool) []string {
	var out []string
	for _, r := range raw {
		var id string
		if json.Unmarshal(r, &id) == nil {
			out = append(out, id)
			continue
		}
		var o struct {
			ID       string `json:"id"`
			Optional bool   `json:"optional"`
		}
		if json.Unmarshal(r, &o) == nil && o.ID != "" && !(requiredOnly && o.Optional) {
			out = append(out, o.ID)
		}
	}
	return out
}

// cleanRequires drops the platform itself (game, loaders, Java) from a
// jar's required ids: those are the instance, not mods to install.
func cleanRequires(ids []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.ToLower(strings.TrimSpace(id))
		switch id {
		case "", "minecraft", "java", "forge", "neoforge", "fabricloader", "fabric-loader", "quilt_loader", "quilt-loader", "fml", "javafml", "lowcodefml", "mcp":
			continue
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// Medium compressed nested jars share one scratch buffer across all scans;
// larger ones use temporary files. Small jars stay parallel and stored jars
// are read in place. Decoded metadata must not reference scratch bytes.
func (w *modScanWorkspace) readNestedModIDs(ctx context.Context, f *zip.File, ra io.ReaderAt, scratch *modScanBuffer) []string {
	if ctx.Err() != nil || f.UncompressedSize64 > maxNestedJar {
		return nil
	}
	if f.Method == zip.Store {
		if off, err := f.DataOffset(); err == nil {
			section := io.NewSectionReader(ra, off, int64(f.UncompressedSize64))
			if z, err := zip.NewReader(modReaderAt{ctx: ctx, r: section}, section.Size()); err == nil {
				return nestedZipIDs(ctx, z)
			}
		}
	}
	size := int(f.UncompressedSize64)
	if size > keepNestedJar {
		if ids, complete := readNestedModFile(ctx, f, scratch); complete {
			return ids
		}
		// Read-only/unavailable temp storage must not lose dependency IDs.
		// The fallback remains serialized and is not retained by the workspace.
	}
	var b []byte
	if size <= smallNestedJar {
		scratch.nested = resizeModBuffer(scratch.nested, size)
		b = scratch.nested
	} else {
		select {
		case b = <-w.nested:
		case <-ctx.Done():
			return nil
		}
		// A single oversized dependency must not keep a 16 MiB buffer live
		// during every subsequent small jar and inflate the GC heap target.
		retained := b
		defer func() {
			if cap(b) <= keepNestedJar {
				retained = b
			}
			w.nested <- retained
		}()
		if ctx.Err() != nil {
			return nil
		}
		if size > keepNestedJar {
			b = make([]byte, size)
		} else {
			b = resizeModBuffer(b, size)
		}
	}
	if readZipBuffer(ctx, f, b) != nil {
		return nil
	}
	z, e := zip.NewReader(modReaderAt{ctx: ctx, r: bytes.NewReader(b)}, int64(len(b)))
	if e != nil {
		return nil
	}
	return nestedZipIDs(ctx, z)
}

// readNestedModFile reports complete=false only when temporary storage fails,
// so callers can fall back to bounded memory without retrying corrupt input.
func readNestedModFile(ctx context.Context, f *zip.File, scratch *modScanBuffer) (ids []string, complete bool) {
	temp, err := os.CreateTemp("", "yolomcl-nested-*.jar")
	if err != nil {
		return nil, false
	}
	defer func() {
		// The file is disposable scratch, with no durable data to flush.
		// Always try removal, including when Close reports an error.
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()
	r, err := f.Open()
	if err != nil {
		return nil, true
	}
	defer r.Close()
	scratch.nested = resizeModBuffer(scratch.nested, modReadChunk)
	reader := modReader{ctx: ctx, r: io.LimitReader(r, int64(f.UncompressedSize64)+1)}
	// Hide File.ReadFrom so CopyBuffer uses this scan's scratch. File-backed
	// random access keeps the inflated archive out of the Go heap.
	n, err := io.CopyBuffer(struct{ io.Writer }{temp}, reader, scratch.nested)
	if ctx.Err() != nil {
		return nil, true
	}
	if err != nil {
		var fileErr *os.PathError
		// A full/unwritable temp volume must not silently discard dependency
		// IDs. ZIP checksum, format and decompression errors are final.
		return nil, !errors.As(err, &fileErr)
	}
	if n != int64(f.UncompressedSize64) {
		return nil, true
	}
	z, err := zip.NewReader(modReaderAt{ctx: ctx, r: temp}, int64(n))
	if err != nil {
		var fileErr *os.PathError
		return nil, !errors.As(err, &fileErr)
	}
	return nestedZipIDs(ctx, z), true
}

// nestedZipIDs lists the mod ids a nested jar declares.
func nestedZipIDs(ctx context.Context, z *zip.Reader) []string {
	var out []string
	for _, f := range z.File {
		if ctx.Err() != nil {
			return nil
		}
		if f.UncompressedSize64 > 1<<20 {
			continue
		}
		switch f.Name {
		case "fabric.mod.json":
			var v struct {
				ID       string   `json:"id"`
				Provides []string `json:"provides"`
			}
			if readModJSON(ctx, f, &v) == nil {
				out = append(append(out, v.ID), v.Provides...)
			}
		case "quilt.mod.json":
			var v struct {
				Loader struct {
					ID string `json:"id"`
				} `json:"quilt_loader"`
			}
			if readModJSON(ctx, f, &v) == nil {
				out = append(out, v.Loader.ID)
			}
		case "META-INF/mods.toml", "META-INF/neoforge.mods.toml":
			tb, e := readZipBytesContext(ctx, f, 1<<20)
			if e != nil {
				continue
			}
			var v struct {
				Mods []struct {
					ID string `toml:"modId"`
				} `toml:"mods"`
			}
			if safeTOML(tb, &v) == nil {
				for _, m := range v.Mods {
					out = append(out, m.ID)
				}
			}
		}
	}
	return out
}

func readModJSON(ctx context.Context, f *zip.File, v any) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	return decodeJSON(io.LimitReader(modReader{ctx: ctx, r: r}, 2<<20), v)
}

// safeTOML decodes a jar's TOML; go-toml can panic on some odd mods.toml
// files (array tables under a map), and one bad jar must not take the
// launcher down.
func safeTOML(b []byte, v any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("toml: %v", r)
		}
	}()
	return toml.Unmarshal(b, v)
}
