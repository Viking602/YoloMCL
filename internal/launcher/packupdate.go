package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type PackUpdateInfo struct {
	CheckedAt string          `json:"checkedAt"`
	Current   string          `json:"current"`
	Latest    *ProjectVersion `json:"latest,omitempty"`
	HasUpdate bool            `json:"hasUpdate"`
}

type PackBackup struct {
	ID          string `json:"id"`
	At          string `json:"at"`
	FromVersion string `json:"fromVersion"`
	ToVersion   string `json:"toVersion"`
	Done        bool   `json:"done"`
}

type PackChange struct {
	Path     string `json:"path"`
	Reason   string `json:"reason,omitempty"` // modified | user | missing
	Disabled bool   `json:"disabled,omitempty"`
	Size     int64  `json:"size,omitempty"`
	From     string `json:"from,omitempty"` // previous path when a mod was renamed
}

type PackUpdatePlan struct {
	ID            string       `json:"id"`
	RootID        string       `json:"rootId"`
	Name          string       `json:"name"`
	OldVersion    string       `json:"oldVersion"`
	NewVersion    string       `json:"newVersion"`
	OldGame       string       `json:"oldGame"`
	NewGame       string       `json:"newGame"`
	Changelog     string       `json:"changelog,omitempty"`
	Added         []PackChange `json:"added"`
	Removed       []PackChange `json:"removed"`
	Updated       []PackChange `json:"updated"`
	Overwritten   []PackChange `json:"overwritten"`
	Unchanged     int          `json:"unchanged"`
	Manual        []PackChange `json:"manual,omitempty"` // CurseForge files that must be downloaded by hand
	DownloadBytes int64        `json:"downloadBytes"`
	GameChanged   bool         `json:"gameChanged"`
	rootPath      string
}

func overrideKind(prefix string) string {
	if prefix == "client-overrides" {
		return "client-override"
	}
	return "override"
}

func upgradeLock(p *PackLock) {
	for i := range p.Files {
		if p.Files[i].Kind == "" {
			if p.Files[i].Source == "override" {
				p.Files[i].Kind = "override"
			} else {
				p.Files[i].Kind = "remote"
			}
		}
	}
	if p.Origin == "" {
		if p.ProjectID != "" {
			p.Origin = p.Source
		} else {
			p.Origin = "file"
		}
	}
}

func nowStamp() string { return time.Now().UTC().Format(time.RFC3339) }

// stampLock fills schema 2 metadata at import/update time and resolves the
// origin of a local .mrpack through Modrinth's version_file hash lookup.
func (s *Service) stampLock(ctx context.Context, lock *PackLock, path string, local bool) {
	lock.Schema = 2
	if lock.ImportedAt == "" {
		lock.ImportedAt = nowStamp()
	}
	if sha, sha512, e := fileHashes(path); e == nil {
		st, _ := os.Stat(path)
		lock.PackFile = &PackArchive{Name: filepath.Base(path), SHA1: sha, SHA512: sha512}
		if st != nil {
			lock.PackFile.Size = st.Size()
		}
	}
	lock.Origin = lock.Source
	if local {
		lock.Origin = "file"
	}
	if local && lock.Source == "modrinth" && lock.ProjectID == "" && lock.PackFile != nil {
		if v, e := s.lookupMrpack(ctx, lock.PackFile.SHA1); e == nil && v.ProjectID != "" {
			lock.ProjectID, lock.VersionID = v.ProjectID, v.ID
		}
	}
	if local && lock.Source == "curseforge" && lock.ProjectID == "" && s.CurseForgeAvailable() {
		if v, e := s.lookupCursePack(ctx, path); e == nil && v.ProjectID != "" {
			lock.ProjectID, lock.VersionID = v.ProjectID, v.ID
		}
	}
}

func (s *Service) lookupCursePack(ctx context.Context, path string) (ProjectVersion, error) {
	fp, e := fingerprintFile(ctx, path)
	if e != nil {
		return ProjectVersion{}, e
	}
	c, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var r struct {
		Data struct {
			ExactMatches []struct {
				File cfFile `json:"file"`
			} `json:"exactMatches"`
		} `json:"data"`
	}
	if e := s.cfJSON(c, "POST", "/fingerprints", map[string]any{"fingerprints": []uint32{fp}}, &r); e != nil {
		return ProjectVersion{}, e
	}
	if len(r.Data.ExactMatches) == 0 {
		return ProjectVersion{}, errors.New("not found")
	}
	return r.Data.ExactMatches[0].File.version(), nil
}

func (s *Service) packChangelog(ctx context.Context, source, project, id string) string {
	if source == "curseforge" {
		var r struct {
			Data string `json:"data"`
		}
		if s.cfJSON(ctx, "GET", "/mods/"+url.PathEscape(project)+"/files/"+url.PathEscape(id)+"/changelog", nil, &r) == nil {
			return r.Data
		}
		return ""
	}
	if v, e := s.projectVersion(ctx, source, project, id); e == nil {
		return v.Changelog
	}
	return ""
}

func packSourceSupported(src string) bool { return src == "modrinth" || src == "curseforge" }

// fileUpdatable: sources that can be updated from a file (MultiMC/Prism has
// no online origin).
func fileUpdatable(src string) bool { return packSourceSupported(src) || src == "multimc" }

// playerData lists paths an update never writes or deletes, even if a pack
// archive (typically a MultiMC export) contains them.
func playerData(p string) bool {
	for _, d := range []string{"saves/", "screenshots/", "logs/", "crash-reports/", "backups/"} {
		if strings.HasPrefix(p, d) {
			return true
		}
	}
	return false
}

// modrinthProject extracts the project ID from a Modrinth CDN URL
// (cdn.modrinth.com/data/<project>/versions/...).
func modrinthProject(urls []string) string {
	for _, u := range urls {
		if i := strings.Index(u, "cdn.modrinth.com/data/"); i >= 0 {
			rest := u[i+len("cdn.modrinth.com/data/"):]
			if j := strings.Index(rest, "/"); j > 0 {
				return rest[:j]
			}
		}
	}
	return ""
}

func fileProject(f PackFile) string {
	if f.ProjectID != "" {
		return f.Source + ":" + f.ProjectID
	}
	if p := modrinthProject(f.URLs); p != "" {
		return "modrinth:" + p
	}
	return ""
}

func (s *Service) lookupMrpack(ctx context.Context, sha1 string) (ProjectVersion, error) {
	c, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	var r mrVersion
	if e := s.getJSON(c, modrinthAPI+"/version_file/"+url.PathEscape(sha1)+"?algorithm=sha1", &r); e != nil {
		return ProjectVersion{}, e
	}
	return r.version(), nil
}

func readPackUpdate(root string) *PackUpdateInfo {
	path, e := safeJoin(root, ".mcl/pack-update.json")
	if e != nil {
		return nil
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return nil
	}
	var v PackUpdateInfo
	if json.Unmarshal(b, &v) != nil {
		return nil
	}
	if p := readPack(root); p == nil || p.Version != v.Current {
		v.HasUpdate = false
	}
	return &v
}

// latestPackVersion returns the newest release built for the pack's own
// Minecraft version and loader.
func (s *Service) latestPackVersion(ctx context.Context, lock *PackLock, includePre bool) (*ProjectVersion, error) {
	loader := lock.Loader
	if loader == "vanilla" {
		loader = ""
	}
	if lock.Source == "curseforge" {
		loader = "" // modpack files are rarely tagged with a loader
	}
	versions, e := s.Versions(ctx, lock.Source, lock.ProjectID, lock.Minecraft, loader)
	if e != nil {
		return nil, e
	}
	if lock.Source == "curseforge" {
		sort.SliceStable(versions, func(i, j int) bool { return versions[i].Published > versions[j].Published })
		cur := ""
		for _, v := range versions {
			if v.ID == lock.VersionID {
				cur = v.Published
			}
		}
		if cur != "" {
			kept := versions[:0]
			for _, v := range versions {
				if v.Published >= cur {
					kept = append(kept, v)
				}
			}
			versions = kept
		}
	}
	for i := range versions {
		v := versions[i]
		if !includePre && v.Type != "" && v.Type != "release" {
			continue
		}
		if len(v.GameVersions) > 0 && !contains(v.GameVersions, lock.Minecraft) {
			continue
		}
		return &v, nil
	}
	return nil, nil
}

func (s *Service) CheckPackUpdate(ctx context.Context, rootID string) (PackUpdateInfo, error) {
	r, e := s.getRoot(rootID)
	if e != nil {
		return PackUpdateInfo{}, e
	}
	lock := readPack(r.Path)
	if lock == nil {
		return PackUpdateInfo{}, errors.New("此目录没有整合包导入记录")
	}
	if lock.ProjectID == "" {
		return PackUpdateInfo{}, errors.New("未找到此整合包的在线来源，请使用「从文件更新」")
	}
	if !packSourceSupported(lock.Source) {
		return PackUpdateInfo{}, errors.New("此整合包来源暂不支持在线更新")
	}
	if lock.Source == "curseforge" && !s.CurseForgeAvailable() {
		return PackUpdateInfo{}, errors.New("CurseForge 服务不可用，无法检查更新")
	}
	s.mu.Lock()
	beta := s.cfg.Settings.PackIncludeBeta
	s.mu.Unlock()
	latest, e := s.latestPackVersion(ctx, lock, beta)
	if e != nil {
		return PackUpdateInfo{}, e
	}
	info := PackUpdateInfo{CheckedAt: nowStamp(), Current: lock.Version, Latest: latest}
	if latest != nil && latest.ID != lock.VersionID && (lock.Source == "curseforge" || latest.Number != lock.Version) {
		info.HasUpdate = true
		if latest.Changelog == "" {
			latest.Changelog = s.packChangelog(ctx, lock.Source, lock.ProjectID, latest.ID)
		}
	}
	if info.Latest != nil && len(info.Latest.Changelog) > 20000 {
		info.Latest.Changelog = info.Latest.Changelog[:20000]
	}
	_ = atomicJSON(filepath.Join(r.Path, ".mcl", "pack-update.json"), info)
	s.invalidateInventory(rootID)
	return info, nil
}

// AutoCheckPackUpdates checks Modrinth packs whose last check is older than
// 24 hours. Network failures are ignored silently.
func (s *Service) AutoCheckPackUpdates(ctx context.Context) {
	s.mu.Lock()
	roots := append([]Root(nil), s.cfg.Roots...)
	off := s.cfg.Settings.PackAutoCheckOff
	s.mu.Unlock()
	if off {
		return
	}
	for _, r := range roots {
		lock := readPack(r.Path)
		if lock == nil || lock.ProjectID == "" || !packSourceSupported(lock.Source) || (lock.Source == "curseforge" && !s.CurseForgeAvailable()) {
			continue
		}
		if info := readPackUpdate(r.Path); info != nil && info.Current == lock.Version {
			if t, e := time.Parse(time.RFC3339, info.CheckedAt); e == nil && time.Since(t) < 24*time.Hour {
				continue
			}
		}
		c, cancel := context.WithTimeout(ctx, 15*time.Second)
		_, _ = s.CheckPackUpdate(c, r.ID)
		cancel()
	}
}
