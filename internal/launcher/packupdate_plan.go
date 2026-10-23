package launcher

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

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

func gameLabel(l *PackLock) string {
	if l.Loader == "" || l.Loader == "vanilla" {
		return "Minecraft " + l.Minecraft
	}
	return l.Minecraft + "   " + l.Loader + " " + l.LoaderVersion
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
