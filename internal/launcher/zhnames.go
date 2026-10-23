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
	cf, mr, modID  map[string]string
	packCF, packMR map[string]string
	// mod ids per platform slug, and slugs per mod id (from the same rows)
	idsCF, idsMR map[string][]string
	slugByID     map[string][2]string // mod id -> {cf slug, mr slug}
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
	return &ZhNames{cf: map[string]string{}, mr: map[string]string{}, modID: map[string]string{}, packCF: map[string]string{}, packMR: map[string]string{}, idsCF: map[string][]string{}, idsMR: map[string][]string{}, slugByID: map[string][2]string{}}
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

func (z *ZhNames) parse(b []byte, cf, mr, modID map[string]string) {
	r := csv.NewReader(bytes.NewReader(b))
	r.Comment = '#'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	for {
		rec, e := r.Read()
		if e == io.EOF {
			return
		}
		if e != nil || len(rec) < 4 {
			continue
		}
		if modID != nil && len(rec) > 6 { // ids are useful even without a Chinese name
			cfS, mrS := strings.ToLower(strings.TrimSpace(rec[1])), strings.ToLower(strings.TrimSpace(rec[2]))
			for _, id := range strings.Split(rec[6], ";") {
				if id = strings.ToLower(strings.TrimSpace(id)); id == "" {
					continue
				}
				if cfS != "" {
					z.idsCF[cfS] = append(z.idsCF[cfS], id)
				}
				if mrS != "" {
					z.idsMR[mrS] = append(z.idsMR[mrS], id)
				}
				if _, dup := z.slugByID[id]; !dup && (cfS != "" || mrS != "") {
					z.slugByID[id] = [2]string{cfS, mrS}
				}
			}
		}
		zh := strings.TrimSpace(rec[3])
		if !hasHan(zh) {
			continue
		}
		if v := strings.TrimSpace(rec[1]); v != "" {
			cf[strings.ToLower(v)] = zh
		}
		if v := strings.TrimSpace(rec[2]); v != "" {
			mr[strings.ToLower(v)] = zh
		}
		if modID != nil && len(rec) > 6 {
			for _, id := range strings.Split(rec[6], ";") {
				if id = strings.ToLower(strings.TrimSpace(id)); id != "" {
					if _, dup := modID[id]; !dup {
						modID[id] = zh
					}
				}
			}
		}
	}
}

// Mod returns the Chinese name for a mod, or "".
func (z *ZhNames) Mod(cfSlug, mrSlug, modID string) string {
	if z == nil {
		return ""
	}
	if v := z.mr[strings.ToLower(mrSlug)]; mrSlug != "" && v != "" {
		return v
	}
	if v := z.cf[strings.ToLower(cfSlug)]; cfSlug != "" && v != "" {
		return v
	}
	if modID != "" {
		return z.modID[strings.ToLower(modID)]
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
		return cf[slug]
	}
	return mr[slug]
}

// Size reports how many names are known (for diagnostics).
func (z *ZhNames) Size() int {
	if z == nil {
		return 0
	}
	return len(z.cf) + len(z.mr)
}

// ModIDs lists the jar mod ids MC百科 records for a platform slug.
func (z *ZhNames) ModIDs(source, slug string) []string {
	if z == nil || slug == "" {
		return nil
	}
	if source == "curseforge" {
		return z.idsCF[strings.ToLower(slug)]
	}
	return z.idsMR[strings.ToLower(slug)]
}

// SlugsFor returns the CurseForge and Modrinth slugs for a jar mod id.
func (z *ZhNames) SlugsFor(modID string) (cf, mr string) {
	if z == nil {
		return "", ""
	}
	v := z.slugByID[strings.ToLower(modID)]
	return v[0], v[1]
}
