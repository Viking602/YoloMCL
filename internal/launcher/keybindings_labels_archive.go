package launcher

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxBindingLanguageBytes = 4 << 20

type bindingLabelScan struct {
	ctx             context.Context
	wanted          map[string]bool
	labels          map[string]bindingLabel
	owners          map[string]string
	bytesLeft       int64
	nestedBytesLeft int64
	nestedLeft      int
	incomplete      bool
}

func (s *bindingLabelScan) readSource(source bindingLabelSource) {
	f, err := os.Open(source.path)
	if err != nil {
		s.incomplete = true
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() != source.size || st.ModTime().UnixNano() != source.mtime {
		s.incomplete = true
		return
	}
	if source.locale != "" {
		if source.size > maxBindingLanguageBytes || !s.consume(uint64(source.size)) {
			s.incomplete = true
			return
		}
		data, err := io.ReadAll(io.LimitReader(modReader{ctx: s.ctx, r: f}, maxBindingLanguageBytes+1))
		if err != nil || len(data) > maxBindingLanguageBytes {
			s.incomplete = true
			return
		}
		s.language(data, source.path, source.namespace, source.locale, true)
	} else {
		s.archive(f, st.Size(), 0, source.resource)
	}
	if st, err := f.Stat(); err != nil || st.Size() != source.size || st.ModTime().UnixNano() != source.mtime {
		s.incomplete = true
	}
}

func (s *bindingLabelScan) consume(size uint64) bool {
	if s.ctx.Err() != nil || size > uint64(max(0, s.bytesLeft)) {
		s.incomplete = true
		return false
	}
	s.bytesLeft -= int64(size)
	return true
}

func (s *bindingLabelScan) archive(reader io.ReaderAt, size int64, depth int, resource bool) {
	z, err := zip.NewReader(modReaderAt{ctx: s.ctx, r: reader}, size)
	if err != nil || len(z.File) > 200000 {
		s.incomplete = true
		return
	}
	files := slices.Clone(z.File)
	slices.SortStableFunc(files, func(a, b *zip.File) int { return strings.Compare(a.Name, b.Name) })
	if !resource {
		// Read only the small identity manifests. Do not hash the mod or run
		// its code, and do not retain descriptions or dependency metadata.
		nested := s.modNames(files)
		for _, f := range files {
			if s.ctx.Err() != nil {
				return
			}
			if !strings.HasSuffix(f.Name, ".jar") || !(nested[f.Name] || strings.HasPrefix(f.Name, "META-INF/jars/") || strings.HasPrefix(f.Name, "META-INF/jarjar/")) {
				continue
			}
			if depth >= 4 || s.nestedLeft <= 0 || f.UncompressedSize64 > maxNestedJar || f.UncompressedSize64 > uint64(s.nestedBytesLeft) {
				s.incomplete = true
				continue
			}
			s.nestedLeft--
			s.nestedBytesLeft -= int64(f.UncompressedSize64)
			if f.Method == zip.Store {
				if off, err := f.DataOffset(); err == nil {
					section := io.NewSectionReader(reader, off, int64(f.UncompressedSize64))
					s.archive(section, section.Size(), depth+1, false)
					continue
				}
			}
			b, err := readZipBytesContext(s.ctx, f, maxNestedJar)
			if err != nil {
				s.incomplete = true
				continue
			}
			s.archive(bytes.NewReader(b), int64(len(b)), depth+1, false)
		}
	}
	// JSON has priority when a mod or resource pack also ships legacy .lang.
	for _, extension := range []string{".lang", ".json"} {
		for _, f := range files {
			if s.ctx.Err() != nil {
				return
			}
			namespace, locale := bindingLanguagePath(f.Name)
			if locale == "" || !strings.EqualFold(filepath.Ext(f.Name), extension) {
				continue
			}
			if f.UncompressedSize64 > maxBindingLanguageBytes || !s.consume(f.UncompressedSize64) {
				s.incomplete = true
				continue
			}
			b, err := readZipBytesContext(s.ctx, f, maxBindingLanguageBytes)
			if err != nil {
				s.incomplete = true
				continue
			}
			s.language(b, f.Name, namespace, locale, resource)
		}
	}
}

func (s *bindingLabelScan) language(data []byte, name, namespace, locale string, resource bool) {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	values := map[string]string{}
	if strings.EqualFold(filepath.Ext(name), ".json") {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			s.incomplete = true
			return
		}
		for key, value := range raw {
			if s.wanted[key] {
				var text string
				if json.Unmarshal(value, &text) == nil {
					values[key] = text
				}
			}
		}
	} else if legacyBindingEscapes(data) {
		values = parseProps(data)
	} else {
		// Minecraft's legacy .lang format is UTF-8 key=value, not Java
		// properties unless Forge's explicit escape marker is present.
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "#") {
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if ok && s.wanted[strings.TrimSpace(key)] {
				values[strings.TrimSpace(key)] = value
			}
		}
	}
	for key, value := range values {
		if !s.wanted[key] {
			continue
		}
		value = cleanBindingLabel(value)
		if value == "" || value == key {
			continue
		}
		label := s.labels[key]
		if label.Names == nil {
			label.Names = map[string]string{}
		}
		label.Names[locale] = value
		// A resource pack may translate another mod's keys under its own
		// namespace. Keep the mod ownership learned from its original JAR.
		if !resource || label.Namespace == "" {
			score := 1
			for _, part := range strings.FieldsFunc(key, func(r rune) bool { return r == '.' || r == ':' || r == '/' || r == '_' }) {
				if strings.EqualFold(part, namespace) {
					score = 2
					break
				}
			}
			if label.Namespace == "" || score > label.ownerScore {
				label.Namespace, label.ownerScore = namespace, score
			}
		}
		s.labels[key] = label
	}
}

func legacyBindingEscapes(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "#PARSE_ESCAPES" {
			return true
		}
	}
	return false
}

func cleanBindingLabel(value string) string {
	if !utf8.ValidString(value) {
		return ""
	}
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, stripFormatting(value))
	value = strings.Join(strings.Fields(value), " ")
	if utf8.RuneCountInString(value) > 240 {
		value = string([]rune(value)[:240]) + "…"
	}
	return value
}

func (s *bindingLabelScan) modNames(files []*zip.File) map[string]bool {
	nested := map[string]bool{}
	add := func(id, name string) {
		if id != "" && name != "" {
			s.owners[id] = cleanBindingLabel(name)
		}
	}
	for _, f := range files {
		if s.ctx.Err() != nil {
			return nested
		}
		switch f.Name {
		case "fabric.mod.json", "quilt.mod.json", "META-INF/mods.toml", "META-INF/neoforge.mods.toml", "mcmod.info", "META-INF/jarjar/metadata.json":
		default:
			continue
		}
		if f.UncompressedSize64 > 2<<20 || !s.consume(f.UncompressedSize64) {
			s.incomplete = true
			continue
		}
		b, err := readZipBytesContext(s.ctx, f, 2<<20)
		if err != nil {
			s.incomplete = true
			continue
		}
		switch f.Name {
		case "fabric.mod.json":
			var mod struct {
				ID, Name string
				Jars     []struct{ File string }
			}
			if json.Unmarshal(b, &mod) == nil {
				add(mod.ID, mod.Name)
				for _, jar := range mod.Jars {
					nested[jar.File] = true
				}
			}
		case "quilt.mod.json":
			var mod struct {
				Loader struct {
					ID       string
					Metadata struct{ Name string }
					Jars     []string
				} `json:"quilt_loader"`
			}
			if json.Unmarshal(b, &mod) == nil {
				add(mod.Loader.ID, mod.Loader.Metadata.Name)
				for _, jar := range mod.Loader.Jars {
					nested[jar] = true
				}
			}
		case "META-INF/mods.toml", "META-INF/neoforge.mods.toml":
			var mod struct {
				Mods []struct {
					ID   string `toml:"modId"`
					Name string `toml:"displayName"`
				} `toml:"mods"`
			}
			if safeTOML(b, &mod) == nil {
				for _, m := range mod.Mods {
					add(m.ID, m.Name)
				}
			}
		case "mcmod.info":
			type legacyMod struct {
				ID   string `json:"modid"`
				Name string
			}
			var list []legacyMod
			if json.Unmarshal(b, &list) != nil {
				var wrapper struct{ ModList []legacyMod }
				if json.Unmarshal(b, &wrapper) == nil {
					list = wrapper.ModList
				}
			}
			for _, mod := range list {
				add(mod.ID, mod.Name)
			}
		case "META-INF/jarjar/metadata.json":
			var data struct{ Jars []struct{ Path string } }
			if json.Unmarshal(b, &data) == nil {
				for _, jar := range data.Jars {
					nested[jar.Path] = true
				}
			}
		}
	}
	return nested
}
