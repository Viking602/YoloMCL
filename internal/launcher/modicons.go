package launcher

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

var hashCache sync.Map

func cachedSHA1(path string, st os.FileInfo) (string, error) {
	key := path + "|" + strconv.FormatInt(st.Size(), 10) + "|" + strconv.FormatInt(st.ModTime().UnixNano(), 10)
	if v, ok := hashCache.Load(key); ok {
		return v.(string), nil
	}
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha1.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	sum := hex.EncodeToString(h.Sum(nil))
	hashCache.Store(key, sum)
	return sum, nil
}

type modIconEntry struct {
	URL     string   `json:"url"`
	MR      string   `json:"mr,omitempty"` // Modrinth slug
	CF      string   `json:"cf,omitempty"` // CurseForge slug
	Summary string   `json:"sum,omitempty"`
	MRID    string   `json:"mrid,omitempty"`
	CFID    string   `json:"cfid,omitempty"`
	Cats    []string `json:"cats,omitempty"` // platform category slugs
	Checked int64    `json:"checked"`        // unix seconds; misses are retried after a week
	V       int      `json:"v,omitempty"`    // 3: slugs, summary and categories recorded
}

// RemoteInfo is what the platforms know about one local file.
type RemoteInfo struct {
	Icon    string
	MRSlug  string
	CFSlug  string
	Summary string
	Cats    []string
	MRID    string // Modrinth project id
	CFID    string // CurseForge mod id
}

func (r RemoteInfo) known() bool { return r.Icon != "" || r.MRSlug != "" || r.CFSlug != "" }

// AddonRef names one local file by content hash.
type AddonRef struct {
	SHA1 string
	File string
}

var modIconMu sync.Mutex

func (s *Service) modIconIndexPath() string {
	return filepath.Join(s.dir, "cache", "mod-icons.json")
}

func (s *Service) loadModIconIndex() map[string]modIconEntry {
	idx := map[string]modIconEntry{}
	if b, e := os.ReadFile(s.modIconIndexPath()); e == nil {
		_ = json.Unmarshal(b, &idx)
	}
	return idx
}

// ModIconURLs maps mod SHA1s to a project icon URL.
func (s *Service) ModIconURLs(ctx context.Context, rootID, versionID string, mods []Mod) (map[string]string, error) {
	refs := make([]AddonRef, 0, len(mods))
	for _, m := range mods {
		refs = append(refs, AddonRef{SHA1: m.SHA1, File: m.File})
	}
	info, e := s.AddonRemoteInfo(ctx, rootID, versionID, "mod", refs)
	out := map[string]string{}
	for h, v := range info {
		if v.Icon != "" {
			out[h] = v.Icon
		}
	}
	return out, e
}

// PackRefs hashes resource or shader pack archives (folders are skipped:
// the platforms only know published zips).
func (s *Service) PackRefs(rootID, versionID, kind string, packs []AssetPack) []AddonRef {
	r, e := s.rootAt(rootID, versionID)
	if e != nil {
		return nil
	}
	sub, e := packFolder(kind)
	if e != nil {
		return nil
	}
	out := []AddonRef{}
	for _, p := range packs {
		path, e := safeJoin(r.VersionDir(), sub+"/"+p.File)
		if e != nil {
			continue
		}
		st, e := os.Stat(path)
		if e != nil || st.IsDir() || st.Size() > 512<<20 {
			continue
		}
		if h, e := cachedSHA1(path, st); e == nil {
			out = append(out, AddonRef{SHA1: h, File: p.File})
		}
	}
	return out
}

// AddonRemoteInfo looks files up on Modrinth (SHA-1) and then CurseForge
// (fingerprint, batch) for what Modrinth missed. Hits and misses are cached
// on disk by hash, so each file is looked up once.
func (s *Service) AddonRemoteInfo(ctx context.Context, rootID, versionID, kind string, items []AddonRef) (map[string]RemoteInfo, error) {
	modIconMu.Lock()
	defer modIconMu.Unlock()
	idx := s.loadModIconIndex()
	out := map[string]RemoteInfo{}
	now := time.Now().Unix()
	var todo []AddonRef
	for _, m := range items {
		if m.SHA1 == "" {
			continue
		}
		if e, ok := idx[m.SHA1]; ok && e.V >= 4 && (e.URL != "" || e.MR != "" || e.CF != "" || now-e.Checked < 7*86400) {
			if e.URL != "" || e.MR != "" || e.CF != "" {
				out[m.SHA1] = RemoteInfo{Icon: e.URL, MRSlug: e.MR, CFSlug: e.CF, Summary: e.Summary, Cats: e.Cats, MRID: e.MRID, CFID: e.CFID}
			}
			continue
		}
		todo = append(todo, m)
	}
	if len(todo) == 0 {
		return out, nil
	}
	found := map[string]RemoteInfo{}
	// Modrinth: sha1 -> version -> project.
	hashes := make([]string, 0, len(todo))
	for _, m := range todo {
		hashes = append(hashes, m.SHA1)
	}
	var known map[string]mrVersion
	if e := s.jsonRequest(ctx, "POST", modrinthAPI+"/version_files", map[string]any{"hashes": hashes, "algorithm": "sha1"}, nil, &known); e == nil && len(known) > 0 {
		ids, byProject := []string{}, map[string][]string{}
		for h, v := range known {
			if byProject[v.ProjectID] == nil {
				ids = append(ids, v.ProjectID)
			}
			byProject[v.ProjectID] = append(byProject[v.ProjectID], h)
		}
		for len(ids) > 0 {
			n := min(len(ids), 100)
			q, _ := json.Marshal(ids[:n])
			ids = ids[n:]
			var projects []struct {
				ID         string   `json:"id"`
				Slug       string   `json:"slug"`
				Icon       string   `json:"icon_url"`
				Desc       string   `json:"description"`
				Categories []string `json:"categories"`
			}
			if e := s.jsonRequest(ctx, "GET", modrinthAPI+"/projects?ids="+url.QueryEscape(string(q)), nil, nil, &projects); e != nil {
				break
			}
			for _, p := range projects {
				for _, h := range byProject[p.ID] {
					found[h] = RemoteInfo{Icon: p.Icon, MRSlug: p.Slug, Summary: p.Desc, Cats: p.Categories, MRID: p.ID}
				}
			}
		}
	}
	// CurseForge: fingerprint -> file -> project, for what Modrinth missed
	// or knows without an icon.
	sub := "mods"
	if kind != "mod" {
		sub, _ = packFolder(kind)
	}
	if s.publisher.HasCurseForge() && sub != "" {
		r, e := s.rootAt(rootID, versionID)
		if e == nil {
			fps, byFP := []uint32{}, map[uint32]string{}
			for _, m := range todo {
				if f := found[m.SHA1]; f.Icon != "" && f.Summary != "" {
					continue
				}
				p, e := safeJoin(r.VersionDir(), sub+"/"+m.File)
				if e != nil {
					continue
				}
				if st, e := os.Stat(p); e != nil || st.IsDir() || st.Size() > 256<<20 {
					continue
				}
				b, e := os.ReadFile(p)
				if e != nil {
					continue
				}
				fp := curseFingerprint(b)
				fps, byFP[fp] = append(fps, fp), m.SHA1
			}
			if len(fps) > 0 {
				var matches struct {
					Data struct {
						ExactMatches []struct {
							File cfFile `json:"file"`
						} `json:"exactMatches"`
					} `json:"data"`
				}
				if e := s.cfJSON(ctx, "POST", "/fingerprints", map[string]any{"fingerprints": fps}, &matches); e == nil {
					modIDs, byMod := []int{}, map[int][]string{}
					for _, m := range matches.Data.ExactMatches {
						if h, ok := byFP[m.File.Fingerprint]; ok {
							if byMod[m.File.ModID] == nil {
								modIDs = append(modIDs, m.File.ModID)
							}
							byMod[m.File.ModID] = append(byMod[m.File.ModID], h)
						}
					}
					if len(modIDs) > 0 {
						var mods struct {
							Data []struct {
								ID         int    `json:"id"`
								Slug       string `json:"slug"`
								Summary    string `json:"summary"`
								Categories []struct {
									Slug string `json:"slug"`
								} `json:"categories"`
								Logo struct {
									Thumb string `json:"thumbnailUrl"`
									URL   string `json:"url"`
								} `json:"logo"`
							} `json:"data"`
						}
						if e := s.cfJSON(ctx, "POST", "/mods", map[string]any{"modIds": modIDs}, &mods); e == nil {
							for _, m := range mods.Data {
								u := m.Logo.Thumb
								if u == "" {
									u = m.Logo.URL
								}
								for _, h := range byMod[m.ID] {
									f := found[h]
									if f.Icon == "" {
										f.Icon = u
									}
									f.CFSlug, f.CFID = m.Slug, strconv.Itoa(m.ID)
									if f.Summary == "" {
										f.Summary = m.Summary
									}
									if len(f.Cats) == 0 {
										for _, c := range m.Categories {
											f.Cats = append(f.Cats, c.Slug)
										}
									}
									found[h] = f
								}
							}
						}
					}
				}
			}
		}
	}
	for _, m := range todo {
		f := found[m.SHA1]
		idx[m.SHA1] = modIconEntry{URL: f.Icon, MR: f.MRSlug, CF: f.CFSlug, Summary: f.Summary, Cats: f.Cats, MRID: f.MRID, CFID: f.CFID, Checked: now, V: 4}
		if f.known() {
			out[m.SHA1] = f
		}
	}
	if e := atomicJSON(s.modIconIndexPath(), idx); e != nil {
		return out, fmt.Errorf("mod icon cache: %w", e)
	}
	return out, nil
}

// DiagLegacyScan times the pre-r10 list scan (SHA-1 + SHA-512 of every jar
// plus a metadata parse, no caches) for QA comparisons. Read only.
func (s *Service) DiagLegacyScan(rootID, versionID string) (time.Duration, error) {
	r, e := s.rootAt(rootID, versionID)
	if e != nil {
		return 0, e
	}
	start := time.Now()
	entries, e := os.ReadDir(filepath.Join(r.VersionDir(), "mods"))
	if e != nil {
		return 0, e
	}
	for _, en := range entries {
		p := filepath.Join(r.VersionDir(), "mods", en.Name())
		if _, _, e := fileHashes(p); e == nil {
			var m Mod
			readModMetadata(p, &m)
		}
	}
	return time.Since(start), nil
}
