package launcher

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func readModMetadata(path string, m *Mod) {
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
	zr, e := zip.NewReader(f, st.Size())
	if e != nil {
		m.Status = "invalid"
		return
	}
	readModMetadataFrom(zr, f, m)
}

// readModMetadataFrom reads mod info from an opened jar; ra is the jar's
// bytes so stored (uncompressed) nested jars are read in place.
func readModMetadataFrom(r *zip.Reader, ra io.ReaderAt, m *Mod) {
scan:
	for _, f := range r.File {
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
			if readZipJSON(f, &v) == nil {
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
			if readZipJSON(f, &v) == nil {
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
			rd, e := f.Open()
			if e != nil {
				continue
			}
			b, e := io.ReadAll(io.LimitReader(rd, 2<<20))
			rd.Close()
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
			if f.Name == "mcmod.info" && f.UncompressedSize64 < 1<<20 {
				var list []struct {
					Desc string `json:"description"`
				}
				if readZipJSON(f, &list) == nil && len(list) > 0 {
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
		if !strings.HasSuffix(f.Name, ".jar") || f.UncompressedSize64 > 16<<20 || !(strings.HasPrefix(f.Name, "META-INF/jars/") || strings.HasPrefix(f.Name, "META-INF/jarjar/")) {
			continue
		}
		if f.Method == zip.Store { // usual for jar-in-jar: open in place, no copy
			if off, e := f.DataOffset(); e == nil {
				if z, e := zip.NewReader(io.NewSectionReader(ra, off, int64(f.UncompressedSize64)), int64(f.UncompressedSize64)); e == nil {
					m.Provides = append(m.Provides, nestedZipIDs(z)...)
					continue
				}
			}
		}
		rd, e := f.Open()
		if e != nil {
			continue
		}
		b, e := io.ReadAll(io.LimitReader(rd, 16<<20))
		rd.Close()
		if e != nil {
			continue
		}
		m.Provides = append(m.Provides, nestedModIDs(b)...)
	}
	if strings.Contains(m.Version, "${") {
		m.Version = ""
		for _, f := range r.File {
			if f.Name == "META-INF/MANIFEST.MF" {
				rd, e := f.Open()
				if e != nil {
					break
				}
				b, _ := io.ReadAll(io.LimitReader(rd, 2<<20))
				rd.Close()
				for _, line := range strings.Split(string(b), "\n") {
					if strings.HasPrefix(line, "Implementation-Version:") {
						m.Version = strings.TrimSpace(strings.TrimPrefix(line, "Implementation-Version:"))
					}
				}
			}
		}
	}
}

// CurseForge's fingerprint is MurmurHash2(seed=1), with ASCII whitespace removed.
func curseFingerprint(data []byte) uint32 {
	ws := func(v byte) bool { return v == 9 || v == 10 || v == 13 || v == 32 }
	n := 0
	for _, v := range data {
		if !ws(v) {
			n++
		}
	}
	const mul uint32 = 0x5bd1e995
	h := uint32(1) ^ uint32(n)
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

// nestedModIDs reads the mod ids of a jar held inside another jar.
func nestedModIDs(b []byte) []string {
	z, e := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if e != nil {
		return nil
	}
	return nestedZipIDs(z)
}

// nestedZipIDs lists the mod ids a nested jar declares.
func nestedZipIDs(z *zip.Reader) []string {
	var out []string
	for _, f := range z.File {
		if f.UncompressedSize64 > 1<<20 {
			continue
		}
		switch f.Name {
		case "fabric.mod.json":
			var v struct {
				ID       string   `json:"id"`
				Provides []string `json:"provides"`
			}
			if readZipJSON(f, &v) == nil {
				out = append(append(out, v.ID), v.Provides...)
			}
		case "quilt.mod.json":
			var v struct {
				Loader struct {
					ID string `json:"id"`
				} `json:"quilt_loader"`
			}
			if readZipJSON(f, &v) == nil {
				out = append(out, v.Loader.ID)
			}
		case "META-INF/mods.toml", "META-INF/neoforge.mods.toml":
			rd, e := f.Open()
			if e != nil {
				continue
			}
			tb, _ := io.ReadAll(io.LimitReader(rd, 1<<20))
			rd.Close()
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
