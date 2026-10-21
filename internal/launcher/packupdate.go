package launcher

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Modpack updates (Phase 1, Modrinth .mrpack only).
//
// Policy: files the pack owns (listed in pack-lock.json) are always replaced
// with the new pack's copy, even when the player edited them; files the pack
// no longer ships are deleted. Files the player added are never touched unless
// the new pack ships a file at the same path. Everything that is overwritten or
// deleted is copied into .mcl/backups/<id> first, so a failed apply restores
// automatically and a finished update can be rolled back by hand.

const packBackupKeep = 3

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

type stagedFile struct {
	PackFile
	staged  string // override already extracted into staging
	write   bool
	disable bool // write as .disabled (player had disabled the old copy)
}

type planState struct {
	plan    PackUpdatePlan
	root    string
	stage   string
	newLock PackLock
	files   []stagedFile
	removed []string
}

var packPlans = struct {
	sync.Mutex
	m map[string]*planState
}{m: map[string]*planState{}}

// applyFaultHook lets tests inject a failure after n files were applied.
var applyFaultHook func(n int) error

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
	b, e := os.ReadFile(path)
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
	if e := s.cfJSON(c, "POST", "/fingerprints", map[string]any{"fingerprints": []uint32{curseFingerprint(b)}}, &r); e != nil {
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
	b, e := os.ReadFile(filepath.Join(root, ".mcl", "pack-update.json"))
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

func gameLabel(l *PackLock) string {
	if l.Loader == "" || l.Loader == "vanilla" {
		return "Minecraft " + l.Minecraft
	}
	return "Minecraft " + l.Minecraft + " · " + l.Loader + " " + l.LoaderVersion
}

func normPath(p string) string { return strings.ToLower(strings.ReplaceAll(p, "\\", "/")) }

// diskState reports where a pack path currently lives (plain or .disabled).
func diskState(root, rel string) (path string, disabled bool, ok bool) {
	p, e := safeJoin(root, rel)
	if e != nil {
		return "", false, false
	}
	if st, e := os.Stat(p); e == nil && !st.IsDir() {
		return p, false, true
	}
	if st, e := os.Stat(p + ".disabled"); e == nil && !st.IsDir() {
		return p + ".disabled", true, true
	}
	return p, false, false
}

// PlanPackUpdate stages a newer pack (Modrinth version or local .mrpack) and
// computes what applying it would change. Nothing in the game dir is touched.
func (s *Service) PlanPackUpdate(ctx context.Context, rootID, path, versionID string) (PackUpdatePlan, error) {
	r, e := s.getRoot(rootID)
	if e != nil {
		return PackUpdatePlan{}, e
	}
	old := readPack(r.Path)
	if old == nil {
		return PackUpdatePlan{}, errors.New("此目录没有整合包导入记录")
	}
	if !fileUpdatable(old.Source) || (path == "" && !packSourceSupported(old.Source)) {
		return PackUpdatePlan{}, errors.New("此整合包来源暂不支持在线更新，请使用「从文件更新」")
	}
	if why := s.packUnavailable(*old); why != "" {
		return PackUpdatePlan{}, errors.New(why)
	}
	id := newID()
	stageRoot := filepath.Join(r.Path, ".mcl", "update-staging")
	_ = os.RemoveAll(stageRoot)
	stage := filepath.Join(stageRoot, id)
	if e = os.MkdirAll(filepath.Join(stage, "files"), 0755); e != nil {
		return PackUpdatePlan{}, e
	}
	fail := func(e error) (PackUpdatePlan, error) {
		_ = os.RemoveAll(stage)
		return PackUpdatePlan{}, e
	}
	var remote *ProjectVersion
	local := path != ""
	if !local {
		if old.ProjectID == "" {
			return fail(errors.New("未找到此整合包的在线来源，请使用「从文件更新」"))
		}
		v, e := s.projectVersion(ctx, old.Source, old.ProjectID, versionID)
		if e != nil {
			return fail(e)
		}
		if v.ProjectID != "" && v.ProjectID != old.ProjectID {
			return fail(errors.New("整合包项目与版本不匹配"))
		}
		f, e := primaryFile(v)
		if e != nil {
			return fail(e)
		}
		if f.URL == "" {
			return fail(errors.New("作者未开放第三方下载，请从 CurseForge 网站下载新版本后使用「从文件更新」"))
		}
		path = filepath.Join(stage, "pack.zip")
		if e = s.download(ctx, f, path); e != nil {
			return fail(e)
		}
		remote = &v
	}
	z, e := zip.OpenReader(path)
	if e != nil {
		return fail(fmt.Errorf("无法读取整合包压缩文件: %w", e))
	}
	defer z.Close()
	if len(z.File) > 40000 {
		return fail(errors.New("整合包文件数量超过限制"))
	}
	var total uint64
	for _, f := range z.File {
		total += f.UncompressedSize64
		if total > 12<<30 {
			return fail(errors.New("整合包解压大小超过 12 GiB"))
		}
	}
	lock, overrides, e := parsePack(z)
	if e != nil {
		return fail(e)
	}
	if lock.Source != old.Source {
		return fail(errors.New("整合包格式与已安装的不同，请使用「导入为新目录」"))
	}
	lock.ImportedAt = old.ImportedAt
	lock.History = append([]PackHistory(nil), old.History...)
	if remote != nil {
		lock.ProjectID, lock.VersionID = old.ProjectID, remote.ID
		s.stampLock(ctx, &lock, path, false)
	} else {
		s.stampLock(ctx, &lock, path, true)
		if lock.ProjectID != "" && old.ProjectID != "" && lock.ProjectID != old.ProjectID {
			return fail(errors.New("这不是同一个整合包，请使用「导入为新目录」"))
		}
		if lock.ProjectID == "" && !strings.EqualFold(strings.TrimSpace(lock.Name), strings.TrimSpace(old.Name)) {
			return fail(errors.New("整合包名称不一致，请使用「导入为新目录」"))
		}
		if lock.ProjectID == "" {
			lock.ProjectID = old.ProjectID
		}
		if lock.PackFile != nil && old.PackFile != nil && lock.PackFile.SHA1 == old.PackFile.SHA1 {
			return fail(errors.New("这就是当前已安装的整合包版本"))
		}
	}
	plan := PackUpdatePlan{ID: id, RootID: rootID, Name: lock.Name, OldVersion: old.Version, NewVersion: lock.Version, OldGame: gameLabel(old), NewGame: gameLabel(&lock)}
	plan.rootPath = r.Path
	plan.GameChanged = old.Minecraft != lock.Minecraft || old.Loader != lock.Loader || old.LoaderVersion != lock.LoaderVersion
	if remote != nil && remote.Changelog != "" {
		plan.Changelog = remote.Changelog
	} else if lock.VersionID != "" && lock.ProjectID != "" {
		plan.Changelog = s.packChangelog(ctx, lock.Source, lock.ProjectID, lock.VersionID)
	}
	if lock.Source == "curseforge" {
		if e := s.resolveCurseFiles(ctx, &lock, old, &plan); e != nil {
			return fail(e)
		}
	}
	if len(plan.Changelog) > 20000 {
		plan.Changelog = plan.Changelog[:20000]
	}
	// Build the new file set: remote files, then overrides, then client-overrides.
	files := []stagedFile{}
	index := map[string]int{}
	for _, f := range lock.Files {
		k := normPath(f.Path)
		if strings.HasPrefix(k, ".mcl/") {
			return fail(errors.New("整合包不能写入启动器元数据目录"))
		}
		if _, e := safeJoin(r.Path, f.Path); e != nil {
			return fail(e)
		}
		if _, dup := index[k]; dup {
			return fail(errors.New("整合包包含重复文件路径"))
		}
		index[k] = len(files)
		files = append(files, stagedFile{PackFile: f})
	}
	prefixes := []string{strings.Trim(overrides, "/")}
	if lock.Source == "modrinth" {
		prefixes = append(prefixes, "client-overrides")
	}
	for _, prefix := range prefixes {
		if prefix == "" {
			continue
		}
		for _, f := range z.File {
			if !strings.HasPrefix(f.Name, prefix+"/") || f.FileInfo().IsDir() {
				continue
			}
			name := strings.TrimPrefix(f.Name, prefix+"/")
			if name == "" {
				continue
			}
			if strings.HasPrefix(normPath(name), ".mcl/") {
				return fail(errors.New("整合包覆盖文件包含保留目录"))
			}
			if _, e := safeJoin(r.Path, name); e != nil {
				return fail(e)
			}
			dst, e := safeJoin(filepath.Join(stage, "files"), name)
			if e != nil {
				return fail(e)
			}
			_ = os.Remove(dst)
			if e = extractZipEntry(f, dst); e != nil {
				return fail(e)
			}
			sha, sha512, e := fileHashes(dst)
			if e != nil {
				return fail(e)
			}
			pf := PackFile{Path: name, SHA1: sha, SHA512: sha512, Source: "override", Kind: overrideKind(prefix), Size: int64(f.UncompressedSize64)}
			sf := stagedFile{PackFile: pf, staged: dst}
			if i, ok := index[normPath(name)]; ok {
				files[i] = sf
			} else {
				index[normPath(name)] = len(files)
				files = append(files, sf)
			}
		}
	}
	oldIndex := map[string]PackFile{}
	oldByProject := map[string]PackFile{}
	for _, f := range old.Files {
		oldIndex[normPath(f.Path)] = f
		if !strings.HasPrefix(normPath(f.Path), "mods/") {
			continue
		}
		k := fileProject(f)
		if k == "" {
			if cur, _, ok := diskState(r.Path, f.Path); ok {
				k = jarModKey(cur)
			}
		}
		if k != "" {
			oldByProject[k] = f
		}
	}
	newLockFiles := []PackFile{}
	renamedFrom := map[string]bool{}
	for i := range files {
		f := &files[i]
		if playerData(normPath(f.Path)) {
			continue // never write worlds/screenshots/logs from an archive
		}
		newLockFiles = append(newLockFiles, f.PackFile)
		cur, disabled, exists := diskState(r.Path, f.Path)
		ch := PackChange{Path: f.Path, Disabled: disabled, Size: f.Size}
		prev, owned := oldIndex[normPath(f.Path)]
		onDiskNew := exists && checkHashes(cur, f.SHA1, f.SHA512)
		// Same mod under a new file name: treat as an update of the old file
		// and keep its disabled state.
		if !owned && !exists {
			k := fileProject(f.PackFile)
			if k == "" && f.staged != "" && strings.HasPrefix(normPath(f.Path), "mods/") {
				k = jarModKey(f.staged)
			}
			if k != "" {
				if o, ok := oldByProject[k]; ok && !strings.EqualFold(o.Path, f.Path) {
					if _, keepPath := index[normPath(o.Path)]; !keepPath {
						if _, oDisabled, oExists := diskState(r.Path, o.Path); oExists {
							ch.Disabled, ch.From, ch.Reason = oDisabled, o.Path, "renamed"
							f.disable = oDisabled
							renamedFrom[normPath(o.Path)] = true
							plan.Updated = append(plan.Updated, ch)
							f.write = true
							if f.staged == "" {
								plan.DownloadBytes += f.Size
							}
							continue
						}
					}
				}
			}
		}
		switch {
		case onDiskNew:
			plan.Unchanged++
			continue
		case !owned && exists:
			ch.Reason = "user"
			plan.Overwritten = append(plan.Overwritten, ch)
		case !owned:
			plan.Added = append(plan.Added, ch)
		case !exists:
			ch.Reason = "missing"
			plan.Added = append(plan.Added, ch)
		case !checkHashes(cur, prev.SHA1, prev.SHA512):
			ch.Reason = "modified"
			plan.Overwritten = append(plan.Overwritten, ch)
		default:
			plan.Updated = append(plan.Updated, ch)
		}
		f.write = true
		f.disable = disabled
		if f.staged == "" {
			plan.DownloadBytes += f.Size
		}
	}
	removed := []string{}
	for _, f := range old.Files {
		if _, keep := index[normPath(f.Path)]; keep || playerData(normPath(f.Path)) {
			continue
		}
		if _, disabled, exists := diskState(r.Path, f.Path); exists {
			if !renamedFrom[normPath(f.Path)] {
				plan.Removed = append(plan.Removed, PackChange{Path: f.Path, Disabled: disabled})
			}
			removed = append(removed, f.Path)
		}
	}
	for _, l := range []*[]PackChange{&plan.Added, &plan.Removed, &plan.Updated, &plan.Overwritten} {
		sort.Slice(*l, func(i, j int) bool { return (*l)[i].Path < (*l)[j].Path })
	}
	lock.Files = newLockFiles
	packPlans.Lock()
	packPlans.m[id] = &planState{plan: plan, root: r.Path, stage: stage, newLock: lock, files: files, removed: removed}
	packPlans.Unlock()
	return plan, nil
}

// DiscardPackUpdate drops a plan that the user did not apply.
func (s *Service) DiscardPackUpdate(id string) {
	packPlans.Lock()
	st := packPlans.m[id]
	delete(packPlans.m, id)
	packPlans.Unlock()
	if st != nil {
		_ = os.RemoveAll(st.stage)
	}
}

type backupManifest struct {
	ID              string         `json:"id"`
	At              string         `json:"at"`
	FromVersion     string         `json:"fromVersion"`
	ToVersion       string         `json:"toVersion"`
	SelectedVersion string         `json:"selectedVersion"`
	Paths           []string       `json:"paths"` // every base path the update touches
	Saved           []string       `json:"saved"` // files (incl. .disabled variants) copied into files/
	HadModsJSON     bool           `json:"hadModsJson"`
	Done            bool           `json:"done"`
	Plan            PackUpdatePlan `json:"plan"`
}

func backupsDir(root string) string { return filepath.Join(root, ".mcl", "backups") }

func listPackBackups(root string) []PackBackup {
	entries, e := os.ReadDir(backupsDir(root))
	if e != nil {
		return nil
	}
	out := []PackBackup{}
	for _, en := range entries {
		var m backupManifest
		b, e := os.ReadFile(filepath.Join(backupsDir(root), en.Name(), "backup.json"))
		if e != nil || json.Unmarshal(b, &m) != nil || !m.Done {
			continue
		}
		out = append(out, PackBackup{ID: m.ID, At: m.At, FromVersion: m.FromVersion, ToVersion: m.ToVersion, Done: m.Done})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func readBackup(root, id string) (backupManifest, string, error) {
	var m backupManifest
	if id == "" || filepath.Base(id) != id {
		return m, "", errors.New("备份不存在")
	}
	dir := filepath.Join(backupsDir(root), id)
	b, e := os.ReadFile(filepath.Join(dir, "backup.json"))
	if e != nil || json.Unmarshal(b, &m) != nil {
		return m, "", errors.New("备份不存在或已损坏")
	}
	return m, dir, nil
}

// PackUpdateReport returns the change summary stored with a backup.
func (s *Service) PackUpdateReport(rootID, backupID string) (PackUpdatePlan, error) {
	r, e := s.getRoot(rootID)
	if e != nil {
		return PackUpdatePlan{}, e
	}
	m, _, e := readBackup(r.Path, backupID)
	return m.Plan, e
}

func replaceFile(src, dst string) error {
	tmp := dst + ".mcl-tmp"
	_ = os.Remove(tmp)
	if e := copyFile(src, tmp); e != nil {
		return e
	}
	return os.Rename(tmp, dst)
}

// restoreBackup puts every touched path back to its pre-update state.
func restoreBackup(root, dir string, m backupManifest) error {
	var first error
	keep := func(e error) {
		if e != nil && first == nil {
			first = e
		}
	}
	for _, rel := range m.Paths {
		p, e := safeJoin(root, rel)
		if e != nil {
			keep(e)
			continue
		}
		for _, v := range []string{p, p + ".disabled"} {
			if e := os.Remove(v); e != nil && !os.IsNotExist(e) {
				keep(e)
			}
		}
	}
	for _, rel := range m.Saved {
		p, e := safeJoin(root, rel)
		if e != nil {
			keep(e)
			continue
		}
		src, _ := safeJoin(filepath.Join(dir, "files"), rel)
		if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			keep(e)
			continue
		}
		keep(replaceFile(src, p))
	}
	lockPath := filepath.Join(root, ".mcl", "pack-lock.json")
	keep(replaceFile(filepath.Join(dir, "pack-lock.json"), lockPath))
	modsPath := filepath.Join(root, ".mcl", "mods.json")
	if m.HadModsJSON {
		keep(replaceFile(filepath.Join(dir, "mods.json"), modsPath))
	} else {
		_ = os.Remove(modsPath)
	}
	return first
}

func prunePackBackups(root string) {
	entries, e := os.ReadDir(backupsDir(root))
	if e != nil {
		return
	}
	names := []string{}
	for _, en := range entries {
		if en.IsDir() {
			names = append(names, en.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for i, n := range names {
		if i >= packBackupKeep {
			_ = os.RemoveAll(filepath.Join(backupsDir(root), n))
		}
	}
}

// ApplyPackUpdate runs a staged plan as a background job.
func (s *Service) ApplyPackUpdate(planID string) (string, error) {
	packPlans.Lock()
	st := packPlans.m[planID]
	packPlans.Unlock()
	if st == nil {
		return "", errors.New("更新计划已失效，请重新检查更新")
	}
	r, e := s.getRoot(st.plan.RootID)
	if e != nil {
		return "", e
	}
	jobID, e := s.startJob("更新整合包 · "+st.plan.Name, r.ID, func(ctx context.Context, p progress) error {
		defer func() {
			packPlans.Lock()
			delete(packPlans.m, planID)
			packPlans.Unlock()
			_ = os.RemoveAll(filepath.Join(r.Path, ".mcl", "update-staging"))
		}()
		old := readPack(r.Path)
		if old == nil || old.Version != st.plan.OldVersion {
			return errors.New("整合包已发生变化，请重新检查更新")
		}
		// 1. Download and verify everything into staging first.
		writes := []stagedFile{}
		for _, f := range st.files {
			if f.write {
				writes = append(writes, f)
			}
		}
		for i := range writes {
			f := &writes[i]
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if f.staged == "" {
				dst, e := safeJoin(filepath.Join(st.stage, "files"), f.Path)
				if e != nil {
					return e
				}
				p(fmt.Sprintf("下载新版本文件 (%d/%d): %s", i+1, len(writes), filepath.Base(f.Path)), i+1, len(writes))
				var last error = errors.New("整合包文件缺少下载地址")
				for _, raw := range f.URLs {
					if last = s.download(ctx, DownloadFile{Name: f.Path, URL: raw, SHA1: f.SHA1, SHA512: f.SHA512}, dst); last == nil {
						break
					}
				}
				if last != nil {
					return last
				}
				f.staged = dst
			}
			if !checkHashes(f.staged, f.SHA1, f.SHA512) {
				return fmt.Errorf("文件校验失败: %s", f.Path)
			}
		}
		// 2. Game / loader change: install side by side, old version stays.
		newGame := old.GameVersion
		if newGame == "" {
			newGame = r.SelectedVersion
		}
		if st.plan.GameChanged || old.GameVersion == "" {
			if _, e := s.installVanilla(ctx, r.Path, st.newLock.Minecraft, p); e != nil {
				return e
			}
			id, e := s.installLoader(ctx, r.Path, st.newLock.Minecraft, st.newLock.Loader, st.newLock.LoaderVersion, p)
			if e != nil {
				return e
			}
			gv, e := s.resolveVersion(ctx, r.Path, id, p)
			if e != nil {
				return e
			}
			if _, _, e = s.prepareVersion(ctx, r.Path, gv, p); e != nil {
				return e
			}
			newGame = id
		}
		// 3. Backup.
		p("备份将被修改的文件", 0, 0)
		bid := time.Now().Format("20060102-150405") + "-" + planID[:6]
		bdir := filepath.Join(backupsDir(r.Path), bid)
		m := backupManifest{ID: bid, At: nowStamp(), FromVersion: old.Version, ToVersion: st.newLock.Version, SelectedVersion: r.SelectedVersion, Plan: st.plan}
		bases := append([]string{}, st.removed...)
		for _, f := range writes {
			bases = append(bases, f.Path)
		}
		m.Paths = bases
		for _, rel := range bases {
			src, _ := safeJoin(r.Path, rel)
			for _, v := range []string{rel, rel + ".disabled"} {
				full := src + strings.TrimPrefix(v, rel)
				if fi, e := os.Stat(full); e == nil && !fi.IsDir() {
					dst, _ := safeJoin(filepath.Join(bdir, "files"), v)
					if e := copyFile(full, dst); e != nil {
						_ = os.RemoveAll(bdir)
						return fmt.Errorf("备份失败: %w", e)
					}
					m.Saved = append(m.Saved, v)
				}
			}
		}
		if e := copyFile(filepath.Join(r.Path, ".mcl", "pack-lock.json"), filepath.Join(bdir, "pack-lock.json")); e != nil {
			_ = os.RemoveAll(bdir)
			return fmt.Errorf("备份失败: %w", e)
		}
		if _, e := os.Stat(filepath.Join(r.Path, ".mcl", "mods.json")); e == nil {
			m.HadModsJSON = true
			if e := copyFile(filepath.Join(r.Path, ".mcl", "mods.json"), filepath.Join(bdir, "mods.json")); e != nil {
				_ = os.RemoveAll(bdir)
				return fmt.Errorf("备份失败: %w", e)
			}
		}
		if e := atomicJSON(filepath.Join(bdir, "backup.json"), m); e != nil {
			_ = os.RemoveAll(bdir)
			return e
		}
		// 4. Apply; restore automatically on any failure.
		apply := func() error {
			for _, rel := range st.removed {
				full, _ := safeJoin(r.Path, rel)
				for _, v := range []string{full, full + ".disabled"} {
					if e := os.Remove(v); e != nil && !os.IsNotExist(e) {
						return e
					}
				}
			}
			for i, f := range writes {
				if applyFaultHook != nil {
					if e := applyFaultHook(i); e != nil {
						return e
					}
				}
				p(fmt.Sprintf("应用更新 (%d/%d): %s", i+1, len(writes), f.Path), i+1, len(writes))
				full, _ := safeJoin(r.Path, f.Path)
				target := full
				if f.disable {
					target = full + ".disabled"
				}
				_ = os.Remove(full)
				_ = os.Remove(full + ".disabled")
				if e := os.MkdirAll(filepath.Dir(target), 0755); e != nil {
					return e
				}
				if e := replaceFile(f.staged, target); e != nil {
					return e
				}
			}
			nl := st.newLock
			nl.GameVersion = newGame
			nl.UpdatedAt = nowStamp()
			nl.History = append(nl.History, PackHistory{Version: nl.Version, At: nl.UpdatedAt, Action: "update"})
			if len(nl.History) > 30 {
				nl.History = nl.History[len(nl.History)-30:]
			}
			if nl.Source == "curseforge" {
				records := map[string]modRecord{}
				b, _ := os.ReadFile(filepath.Join(r.Path, ".mcl", "mods.json"))
				_ = json.Unmarshal(b, &records)
				for _, f := range old.Files {
					delete(records, strings.TrimPrefix(f.Path, "mods/"))
				}
				for _, f := range nl.Files {
					if f.Source == "curseforge" && strings.HasPrefix(f.Path, "mods/") {
						records[strings.TrimPrefix(f.Path, "mods/")] = modRecord{SHA1: f.SHA1, Source: f.Source, ProjectID: f.ProjectID, VersionID: f.VersionID}
					}
				}
				if e := atomicJSON(filepath.Join(r.Path, ".mcl", "mods.json"), records); e != nil {
					return e
				}
			}
			return atomicJSON(filepath.Join(r.Path, ".mcl", "pack-lock.json"), nl)
		}
		if e := apply(); e != nil {
			p("更新失败，正在恢复原状", 0, 0)
			if re := restoreBackup(r.Path, bdir, m); re != nil {
				return fmt.Errorf("更新失败（%v），且恢复时出错：%v。备份保留在 %s", e, re, bdir)
			}
			_ = os.RemoveAll(bdir)
			return fmt.Errorf("更新失败，已恢复原状：%w", e)
		}
		m.Done = true
		_ = atomicJSON(filepath.Join(bdir, "backup.json"), m)
		if newGame != "" && newGame != r.SelectedVersion {
			s.mu.Lock()
			for i := range s.cfg.Roots {
				if s.cfg.Roots[i].ID == r.ID {
					s.cfg.Roots[i].SelectedVersion = newGame
				}
			}
			_ = s.saveLocked()
			s.mu.Unlock()
		}
		_ = os.Remove(filepath.Join(r.Path, ".mcl", "pack-update.json"))
		prunePackBackups(r.Path)
		s.setJobResultRoot(ctx, r.ID)
		s.setJobResultRef(ctx, bid)
		p(fmt.Sprintf("更新完成：%s → %s", old.Version, st.newLock.Version), 1, 1)
		return nil
	})
	return jobID, e
}

// RollbackPack restores the state saved before an update.
func (s *Service) RollbackPack(rootID, backupID string) (string, error) {
	r, e := s.getRoot(rootID)
	if e != nil {
		return "", e
	}
	if backupID == "" {
		if b := listPackBackups(r.Path); len(b) > 0 {
			backupID = b[0].ID
		}
	}
	m, dir, e := readBackup(r.Path, backupID)
	if e != nil {
		return "", e
	}
	if !m.Done {
		return "", errors.New("该备份不完整，无法回滚")
	}
	return s.startJob("回滚整合包 · "+m.Plan.Name, r.ID, func(ctx context.Context, p progress) error {
		p(fmt.Sprintf("正在回滚 %s → %s", m.ToVersion, m.FromVersion), 0, 1)
		if e := restoreBackup(r.Path, dir, m); e != nil {
			return fmt.Errorf("回滚未完全成功：%w", e)
		}
		if lock := readPack(r.Path); lock != nil {
			lock.History = append(lock.History, PackHistory{Version: lock.Version, At: nowStamp(), Action: "rollback"})
			_ = atomicJSON(filepath.Join(r.Path, ".mcl", "pack-lock.json"), lock)
		}
		if m.SelectedVersion != "" {
			s.mu.Lock()
			for i := range s.cfg.Roots {
				if s.cfg.Roots[i].ID == r.ID {
					s.cfg.Roots[i].SelectedVersion = m.SelectedVersion
				}
			}
			_ = s.saveLocked()
			s.mu.Unlock()
		}
		_ = os.RemoveAll(dir)
		_ = os.Remove(filepath.Join(r.Path, ".mcl", "pack-update.json"))
		s.setJobResultRoot(ctx, r.ID)
		p(fmt.Sprintf("已回滚到 %s", m.FromVersion), 1, 1)
		return nil
	})
}

// resolveCurseFiles turns manifest project/file IDs into paths, hashes and
// download URLs with one batch request. Files whose author disabled
// third-party downloads are kept if the same file is already installed,
// otherwise they are listed as manual downloads.
func old0Root(p *PackUpdatePlan) string { return p.rootPath }

func (s *Service) resolveCurseFiles(ctx context.Context, lock *PackLock, old *PackLock, plan *PackUpdatePlan) error {
	ids := []int{}
	for _, f := range lock.Files {
		if f.Source == "curseforge" {
			n, e := strconv.Atoi(f.VersionID)
			if e != nil {
				return errors.New("整合包文件 ID 无效")
			}
			ids = append(ids, n)
		}
	}
	byID := map[string]cfFile{}
	for start := 0; start < len(ids); start += 500 {
		end := min(start+500, len(ids))
		var r struct {
			Data []cfFile `json:"data"`
		}
		if e := s.cfJSON(ctx, "POST", "/mods/files", map[string]any{"fileIds": ids[start:end]}, &r); e != nil {
			return e
		}
		for _, f := range r.Data {
			byID[strconv.Itoa(f.ID)] = f
		}
	}
	oldBy := map[string]PackFile{}
	for _, f := range old.Files {
		if f.Source == "curseforge" {
			oldBy[f.ProjectID+"/"+f.VersionID] = f
		}
	}
	kept := lock.Files[:0]
	lock.Manual = []ManualDownload{}
	for _, f := range lock.Files {
		if f.Source != "curseforge" {
			kept = append(kept, f)
			continue
		}
		cf, ok := byID[f.VersionID]
		if !ok {
			return fmt.Errorf("CurseForge 找不到整合包文件 #%s", f.VersionID)
		}
		v := cf.version()
		df := v.Files[0]
		f.Path, f.SHA1, f.Size, f.Kind = "mods/"+df.Name, df.SHA1, df.Size, "remote"
		f.URLs = nil
		if df.URL != "" {
			f.URLs = []string{df.URL}
			kept = append(kept, f)
			continue
		}
		if prev, ok := oldBy[f.ProjectID+"/"+f.VersionID]; ok {
			if p, _, exists := diskState(old0Root(plan), prev.Path); exists && checkHashes(p, prev.SHA1, prev.SHA512) {
				f.Path = prev.Path
				kept = append(kept, f)
				continue
			}
		}
		md := ManualDownload{ProjectID: f.ProjectID, FileID: f.VersionID, Name: df.Name, SHA1: df.SHA1, URL: "https://www.curseforge.com/projects/" + f.ProjectID + "/files/" + f.VersionID}
		lock.Manual = append(lock.Manual, md)
		ch := PackChange{Path: f.Path, Size: f.Size}
		// Keep the installed older version until the player provides the new
		// file, so the pack keeps working.
		for _, prev := range old.Files {
			if prev.Source == "curseforge" && prev.ProjectID == f.ProjectID {
				if _, _, exists := diskState(old0Root(plan), prev.Path); exists {
					kept = append(kept, prev)
					ch.Reason, ch.From = "kept-old", prev.Path
					break
				}
			}
		}
		plan.Manual = append(plan.Manual, ch)
	}
	lock.Files = kept
	return nil
}

// jarModKey identifies a mod jar by its loader mod ID, for packs (MultiMC)
// whose files carry no project ID.
func jarModKey(path string) string {
	if !strings.HasSuffix(strings.ToLower(strings.TrimSuffix(path, ".disabled")), ".jar") {
		return ""
	}
	var m Mod
	readModMetadata(path, &m)
	if m.ModID == "" {
		return ""
	}
	return "modid:" + strings.ToLower(m.ModID)
}
