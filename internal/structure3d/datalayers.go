package structure3d

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Data layering follows the game's pack order: the vanilla jar first, then
// mod jars (each mod's built-in data pack), then data packs in the world's
// enabled order. A later layer replaces a resource at the same path; tags
// are the one kind of file that merges (ReadTagValues). Nothing here knows
// any mod by name: every mod and pack is just more data/ files.

// OpenModsDir layers every enabled .jar in dir, in file-name order, the
// order mod loaders list them when no dependency ordering applies.
func OpenModsDir(s *Store, dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".jar") {
			names = append(names, e.Name())
		}
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	n := 0
	for _, name := range names {
		if s.OpenJar("mod:"+name, filepath.Join(dir, name)) == nil {
			n++
		}
	}
	return n
}

// OpenPack layers one data pack: a zip/jar, or a folder holding data/.
func OpenPack(s *Store, name, p string) bool {
	st, err := os.Stat(p)
	if err != nil {
		return false
	}
	if st.IsDir() {
		if _, err := os.Stat(filepath.Join(p, "data")); err != nil {
			return false
		}
		return s.OpenDir(name, p) == nil
	}
	low := strings.ToLower(p)
	if strings.HasSuffix(low, ".zip") || strings.HasSuffix(low, ".jar") {
		return s.OpenJar(name, p) == nil
	}
	return false
}

// WorldDatapackOrder is level.dat's Data.DataPacks.Enabled list, lowest
// priority first ("vanilla", "file/<name>", mod pack ids, ...); nil when
// the file has none.
func WorldDatapackOrder(worldDir string) []string {
	data, err := os.ReadFile(filepath.Join(worldDir, "level.dat"))
	if err != nil {
		return nil
	}
	root, err := DecodeNBT(data)
	if err != nil {
		return nil
	}
	d, _ := root["Data"].(map[string]any)
	dp, _ := d["DataPacks"].(map[string]any)
	list, _ := dp["Enabled"].([]any)
	var out []string
	for _, v := range list {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// OpenWorldDatapacks layers the world's datapacks/ folder. Packs named in
// level.dat ("file/<name>") go in that order; packs on disk the list does
// not name follow in file-name order, as a new world would enable them.
// It returns the layer names it opened.
func OpenWorldDatapacks(s *Store, label, worldDir string) []string {
	dir := filepath.Join(worldDir, "datapacks")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	onDisk := map[string]bool{}
	for _, e := range entries {
		onDisk[e.Name()] = true
	}
	var order []string
	seen := map[string]bool{}
	for _, id := range WorldDatapackOrder(worldDir) {
		if name, ok := strings.CutPrefix(id, "file/"); ok && onDisk[name] && !seen[name] {
			order, seen[name] = append(order, name), true
		}
	}
	var rest []string
	for name := range onDisk {
		if !seen[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	order = append(order, rest...)
	var opened []string
	for _, name := range order {
		layer := "datapack:" + label + ":" + name
		if OpenPack(s, layer, filepath.Join(dir, name)) {
			opened = append(opened, layer)
		}
	}
	return opened
}

// ReadTagValues merges a tag file across every layer, lowest first: each
// layer adds its values, and "replace": true drops what came before.
func (s *Store) ReadTagValues(p string) []string {
	var out []string
	for _, b := range s.ReadAll(p) {
		var t struct {
			Replace bool              `json:"replace"`
			Values  []json.RawMessage `json:"values"`
		}
		if json.Unmarshal(b, &t) != nil {
			continue
		}
		if t.Replace {
			out = out[:0]
		}
		for _, raw := range t.Values {
			var id string
			if json.Unmarshal(raw, &id) != nil {
				var obj struct {
					ID string `json:"id"`
				}
				if json.Unmarshal(raw, &obj) != nil {
					continue
				}
				id = obj.ID
			}
			if id != "" {
				out = append(out, id)
			}
		}
	}
	return out
}

// ReadAll returns every layer's bytes for p, lowest priority first.
func (s *Store) ReadAll(p string) [][]byte {
	p = filepath.ToSlash(filepath.Clean("/" + strings.ReplaceAll(p, "\\", "/")))[1:]
	s.mu.Lock()
	layers := append([]layer(nil), s.layers...)
	s.mu.Unlock()
	var out [][]byte
	for _, l := range layers {
		if b, err := l.read(p); err == nil {
			out = append(out, b)
		}
	}
	return out
}
