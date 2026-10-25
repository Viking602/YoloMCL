package launcher

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Chinese project names come from the MC百科 (mcmod.cn) mapping that HMCL
// publishes in its repository (assets/addon/mod.csv and modpack.csv):
// mcmod id, CurseForge slug, Modrinth slug, Chinese name, English name,
// abbreviation, mod ids. Nothing is scraped from mcmod.cn; the two files are
// fetched at most once a week, cached on disk and used offline.
var zhNameSources = map[string][]string{
	"mod.csv": {
		"https://raw.githubusercontent.com/HMCL-dev/HMCL/main/HMCL/src/main/resources/assets/addon/mod.csv",
		"https://cdn.jsdelivr.net/gh/HMCL-dev/HMCL@main/HMCL/src/main/resources/assets/addon/mod.csv",
	},
	"modpack.csv": {
		"https://raw.githubusercontent.com/HMCL-dev/HMCL/main/HMCL/src/main/resources/assets/addon/modpack.csv",
		"https://cdn.jsdelivr.net/gh/HMCL-dev/HMCL@main/HMCL/src/main/resources/assets/addon/modpack.csv",
	},
}

// ZhNames looks up Chinese names by platform slug or mod id.
type ZhNames struct {
	cf, mr, packCF, packMR map[string]zhProjectRef
	modID                  map[string]zhModRef
	rows                   []zhNameRow
	ids                    []string
	size                   int
}

// All lookups refer to the same immutable CSV rows. Separate references keep
// the existing precedence: the last translated project name, all project IDs,
// and the first translated name / platform slugs for each mod ID. The indexes
// store two integers instead of duplicating strings and slices in eight maps.
// Row zero means absent; duplicate slugs get their own merged ID-list row.
type zhProjectRef struct{ name, ids uint32 }
type zhModRef struct{ name, slugs uint32 }
type zhNameRow struct {
	// One allocation holds the name and both slugs. The map keys and returned
	// strings are slices of it; offsets avoid three string/slice headers per row.
	text             string
	nameEnd, cfEnd   uint32
	idsStart, idsEnd uint32
}

func (r zhNameRow) slugs() (string, string) {
	return r.text[r.nameEnd:r.cfEnd], r.text[r.cfEnd:]
}

func (z *ZhNames) rowIDs(r zhNameRow) []string {
	// No spare capacity escapes: callers may append their own candidates.
	return z.ids[r.idsStart:r.idsEnd:r.idsEnd]
}

var (
	zhMu     sync.Mutex
	zhLoaded *ZhNames
	zhAt     time.Time
)

func (s *Service) zhDir() string { return filepath.Join(s.dir, "cache", "mcmod-names") }

// ZhNames returns the mapping, refreshing the cached files when they are
// older than a week; with no network it uses whatever is cached.
func (s *Service) ZhNames(ctx context.Context) (*ZhNames, error) {
	zhMu.Lock()
	defer zhMu.Unlock()
	if zhLoaded != nil && time.Since(zhAt) < time.Hour {
		return zhLoaded, nil
	}
	dir := s.zhDir()
	var lastErr error
	for name, urls := range zhNameSources {
		p := filepath.Join(dir, name)
		if st, e := os.Stat(p); e == nil && time.Since(st.ModTime()) < 7*24*time.Hour {
			continue
		}
		if b, e := s.fetchFirst(ctx, urls); e == nil {
			_ = os.MkdirAll(dir, 0755)
			tmp := p + ".tmp"
			if e = os.WriteFile(tmp, b, 0644); e == nil {
				e = os.Rename(tmp, p)
			}
			lastErr = e
		} else {
			lastErr = e
		}
	}
	z := newZhNames()
	ok := false
	if b, e := os.ReadFile(filepath.Join(dir, "mod.csv")); e == nil {
		z.parse(b, z.cf, z.mr, z.modID)
		ok = true
	}
	if b, e := os.ReadFile(filepath.Join(dir, "modpack.csv")); e == nil {
		z.parse(b, z.packCF, z.packMR, nil)
		ok = true
	}
	if !ok {
		if lastErr == nil {
			lastErr = errors.New("no name data")
		}
		return nil, lastErr
	}
	zhLoaded, zhAt = z, time.Now()
	return z, nil
}

func newZhNames() *ZhNames {
	return &ZhNames{
		cf: map[string]zhProjectRef{}, mr: map[string]zhProjectRef{},
		packCF: map[string]zhProjectRef{}, packMR: map[string]zhProjectRef{},
		modID: map[string]zhModRef{}, rows: []zhNameRow{{}},
	}
}

func (s *Service) fetchFirst(ctx context.Context, urls []string) ([]byte, error) {
	var last error
	for _, u := range urls {
		c, cancel := context.WithTimeout(ctx, 20*time.Second)
		req, _ := http.NewRequestWithContext(c, "GET", u, nil)
		r, e := s.http.Do(req)
		if e == nil {
			var b []byte
			if r.StatusCode == 200 {
				b, e = io.ReadAll(io.LimitReader(r.Body, 32<<20))
			} else {
				e = fmt.Errorf("HTTP %d", r.StatusCode)
			}
			r.Body.Close()
			if e == nil && len(b) > 1024 {
				cancel()
				return b, nil
			}
		}
		cancel()
		last = e
	}
	return nil, last
}

func hasHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func (z *ZhNames) parse(b []byte, cf, mr map[string]zhProjectRef, modID map[string]zhModRef) {
	// One useful row per line in the usual corpus. Reserving its contiguous
	// storage avoids repeatedly copying the live index while loading names.
	lines := bytes.Count(b, []byte{'\n'}) + 1
	z.rows = slices.Grow(z.rows, lines)
	if modID != nil {
		z.ids = slices.Grow(z.ids, lines+bytes.Count(b, []byte{';'}))
	}
	r := csv.NewReader(bytes.NewReader(b))
	r.Comment = '#'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.ReuseRecord = true
	// IDs repeat across projects. Intern only those; each row packs its name
	// and slugs directly, without retaining the CSV's unused English columns.
	stringsByValue := make(map[string]string)
	keep := func(v string) string {
		if s, ok := stringsByValue[v]; ok {
			return s
		}
		s := strings.Clone(v)
		stringsByValue[s] = s
		return s
	}
	for {
		rec, e := r.Read()
		if e == io.EOF {
			return
		}
		if e != nil || len(rec) < 4 {
			continue
		}
		cfS := strings.ToLower(strings.TrimSpace(rec[1]))
		mrS := strings.ToLower(strings.TrimSpace(rec[2]))
		zh := strings.TrimSpace(rec[3])
		if !hasHan(zh) {
			zh = ""
		}
		idsStart := uint32(len(z.ids))
		if modID != nil && len(rec) > 6 { // ids are useful even without a Chinese name
			for id := range strings.SplitSeq(rec[6], ";") {
				if id = strings.ToLower(strings.TrimSpace(id)); id == "" {
					continue
				}
				id = keep(id)
				z.ids = append(z.ids, id)
			}
		}
		if zh == "" && int(idsStart) == len(z.ids) {
			continue
		}
		row := uint32(len(z.rows))
		data := zhNameRow{
			text:    strings.Join([]string{zh, cfS, mrS}, ""),
			nameEnd: uint32(len(zh)), cfEnd: uint32(len(zh) + len(cfS)),
			idsStart: idsStart, idsEnd: uint32(len(z.ids)),
		}
		z.rows = append(z.rows, data)
		cfS, mrS = data.slugs()
		named := z.indexProject(cf, cfS, row) + z.indexProject(mr, mrS, row)
		if modID != nil {
			z.size += named
		}
		for _, id := range z.rowIDs(data) {
			ref := modID[id]
			if zh != "" && ref.name == 0 {
				ref.name = row
			}
			if ref.slugs == 0 && (cfS != "" || mrS != "") {
				ref.slugs = row
			}
			if ref != (zhModRef{}) {
				modID[id] = ref
			}
		}
	}
}

// Return whether this slug acquired its first translated name. Merging IDs
// allocates a new immutable row: the other provider may still use either of
// the source lists, and dependency lookups may be reading them concurrently.
func (z *ZhNames) indexProject(index map[string]zhProjectRef, slug string, row uint32) int {
	if slug == "" {
		return 0
	}
	ref, data := index[slug], z.rows[row]
	named := 0
	if data.nameEnd != 0 {
		if ref.name == 0 {
			named = 1
		}
		ref.name = row
	}
	if data.idsEnd != data.idsStart {
		if ref.ids == 0 {
			ref.ids = row
		} else {
			before, next := z.rowIDs(z.rows[ref.ids]), z.rowIDs(data)
			start := uint32(len(z.ids))
			z.ids = append(z.ids, before...)
			z.ids = append(z.ids, next...)
			ref.ids = uint32(len(z.rows))
			z.rows = append(z.rows, zhNameRow{idsStart: start, idsEnd: uint32(len(z.ids))})
		}
	}
	if ref != (zhProjectRef{}) {
		index[slug] = ref
	}
	return named
}

func (z *ZhNames) name(row uint32) string {
	if row == 0 {
		return ""
	}
	r := z.rows[row]
	return r.text[:r.nameEnd]
}

// Mod returns the Chinese name for a mod, or "".
func (z *ZhNames) Mod(cfSlug, mrSlug, modID string) string {
	if z == nil {
		return ""
	}
	if v := z.mr[strings.ToLower(mrSlug)]; mrSlug != "" && v.name != 0 {
		return z.name(v.name)
	}
	if v := z.cf[strings.ToLower(cfSlug)]; cfSlug != "" && v.name != 0 {
		return z.name(v.name)
	}
	if modID != "" {
		return z.name(z.modID[strings.ToLower(modID)].name)
	}
	return ""
}

// Project returns the Chinese name for a Discover result, or "".
func (z *ZhNames) Project(p Project) string {
	if z == nil {
		return ""
	}
	slug := p.URL
	if i := strings.LastIndex(strings.TrimRight(slug, "/"), "/"); i >= 0 {
		slug = strings.TrimRight(slug, "/")[i+1:]
	}
	slug = strings.ToLower(slug)
	if slug == "" {
		return ""
	}
	cf, mr := z.cf, z.mr
	if p.Type == "modpack" {
		cf, mr = z.packCF, z.packMR
	} else if p.Type != "mod" {
		return ""
	}
	if p.Source == "curseforge" {
		return z.name(cf[slug].name)
	}
	return z.name(mr[slug].name)
}

// Size reports how many names are known (for diagnostics).
func (z *ZhNames) Size() int {
	if z == nil {
		return 0
	}
	return z.size
}

// ModIDs lists the jar mod ids MC百科 records for a platform slug. Treat the
// existing entries as read-only; appending candidates always allocates a copy.
func (z *ZhNames) ModIDs(source, slug string) []string {
	if z == nil || slug == "" {
		return nil
	}
	ref := z.mr[strings.ToLower(slug)]
	if source == "curseforge" {
		ref = z.cf[strings.ToLower(slug)]
	}
	if ref.ids == 0 {
		return nil
	}
	return z.rowIDs(z.rows[ref.ids])
}

// SlugsFor returns the CurseForge and Modrinth slugs for a jar mod id.
func (z *ZhNames) SlugsFor(modID string) (cf, mr string) {
	if z == nil {
		return "", ""
	}
	ref := z.modID[strings.ToLower(modID)]
	if ref.slugs == 0 {
		return "", ""
	}
	return z.rows[ref.slugs].slugs()
}
