package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// PlanItem is one row of the install preview: the mod itself or one of its
// dependencies, with whether the instance already has it.
type PlanItem struct {
	Key        string `json:"key"` // source:projectId
	Source     string `json:"source"`
	ProjectID  string `json:"projectId"`
	VersionID  string `json:"versionId,omitempty"`
	Title      string `json:"title"`
	Slug       string `json:"slug,omitempty"`
	Icon       string `json:"icon,omitempty"`
	Version    string `json:"version,omitempty"`
	Role       string `json:"role"`             // target | required | optional | incompatible
	Status     string `json:"status"`           // resolving | install | installed | disabled | none | conflict | absent
	File       string `json:"file,omitempty"`   // the present file, when installed or disabled
	Parent     string `json:"parent,omitempty"` // key of the mod that needs it
	Depth      int    `json:"depth"`
	Note       string `json:"note,omitempty"`
	Unverified bool   `json:"unverified,omitempty"` // presence could not be checked
}

type projectMeta struct{ Title, Slug, Icon string }

// installedIndex finds a mod in the instance by platform id or jar mod id.
type installedIndex struct {
	byKey    map[string]Mod
	byModID  map[string]Mod
	degraded bool // the mods folder could not be read; presence unknown
}

func (s *Service) installedIndexFor(ctx context.Context, r Root) (installedIndex, error) {
	idx := installedIndex{byKey: map[string]Mod{}, byModID: map[string]Mod{}}
	_ = ctx
	mods, e := scanMods(r.VersionDir())
	if e != nil {
		// An unreadable folder must not sink the whole plan: resolve as if
		// nothing were installed and let each install report for itself.
		idx.degraded = true
		mods = nil
	}
	refs := make([]AddonRef, 0, len(mods))
	for _, m := range mods {
		refs = append(refs, AddonRef{SHA1: m.SHA1, File: m.File})
	}
	info, _ := s.AddonRemoteInfo(ctx, r.ID, r.SelectedVersion, "mod", refs)
	put := func(k string, m Mod) {
		if old, ok := idx.byKey[k]; !ok || (!old.Enabled && m.Enabled) {
			idx.byKey[k] = m
		}
	}
	for _, m := range mods {
		if m.Source != "" && m.ProjectID != "" {
			put(m.Source+":"+m.ProjectID, m)
		}
		if ri := info[m.SHA1]; ri.MRID != "" || ri.CFID != "" {
			if ri.MRID != "" {
				put("modrinth:"+ri.MRID, m)
			}
			if ri.CFID != "" {
				put("curseforge:"+ri.CFID, m)
			}
		}
		for _, id := range append([]string{m.ModID}, m.Provides...) {
			if id = strings.ToLower(strings.TrimSpace(id)); id != "" {
				if old, ok := idx.byModID[id]; !ok || (!old.Enabled && m.Enabled) {
					idx.byModID[id] = m
				}
			}
		}
	}
	return idx, nil
}

// find looks a project up by platform id, then by the jar mod ids its slug
// maps to (MC百科 data) or the slug itself (fabric-api -> fabric_api, fabric).
func (idx installedIndex) find(z *ZhNames, source, project, slug string) (Mod, bool) {
	if m, ok := idx.byKey[source+":"+project]; ok {
		return m, true
	}
	cands := z.ModIDs(source, slug)
	if slug != "" {
		sl := strings.ToLower(slug)
		cands = append(cands, sl, strings.ReplaceAll(sl, "-", "_"), strings.ReplaceAll(sl, "-", ""))
		if sl == "fabric-api" {
			cands = append(cands, "fabric")
		}
	}
	for _, id := range cands {
		if m, ok := idx.byModID[strings.ToLower(id)]; ok {
			return m, true
		}
	}
	return Mod{}, false
}

// projectMetas fetches titles, slugs and icons for a batch of projects.
func (s *Service) projectMetas(ctx context.Context, source string, ids []string) map[string]projectMeta {
	out := map[string]projectMeta{}
	if len(ids) == 0 {
		return out
	}
	if source == "curseforge" {
		nums := []int{}
		for _, id := range ids {
			if n, e := strconv.Atoi(id); e == nil {
				nums = append(nums, n)
			}
		}
		var r struct {
			Data []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
				Slug string `json:"slug"`
				Logo struct {
					Thumb string `json:"thumbnailUrl"`
				} `json:"logo"`
			} `json:"data"`
		}
		if s.cfJSON(ctx, "POST", "/mods", map[string]any{"modIds": nums}, &r) == nil {
			for _, m := range r.Data {
				out[strconv.Itoa(m.ID)] = projectMeta{Title: m.Name, Slug: m.Slug, Icon: m.Logo.Thumb}
			}
		}
		return out
	}
	q, _ := json.Marshal(ids)
	var ps []struct {
		ID    string `json:"id"`
		Slug  string `json:"slug"`
		Title string `json:"title"`
		Icon  string `json:"icon_url"`
	}
	if s.jsonRequest(ctx, "GET", modrinthAPI+"/projects?ids="+url.QueryEscape(string(q)), nil, nil, &ps) == nil {
		for _, p := range ps {
			out[p.ID] = projectMeta{Title: p.Title, Slug: p.Slug, Icon: p.Icon}
		}
	}
	return out
}

// ResolveInstall builds the install preview for one mod version: the mod,
// its required dependencies (transitively, deduplicated, cycle-safe), its
// optional ones (one level) and known incompatibilities, each checked
// against the instance. emit receives the growing list after each level so
// the UI can show rows while the rest resolves.
func (s *Service) ResolveInstall(ctx context.Context, rootID, source, project, versionID string, emit func([]PlanItem)) ([]PlanItem, error) {
	r, e := s.getRoot(rootID)
	if e != nil {
		return nil, e
	}
	mc, loader, e := instanceCompatibility(r)
	if e != nil {
		return nil, e
	}
	z, _ := s.ZhNames(ctx)
	idx, e := s.installedIndexFor(ctx, r)
	if e != nil {
		return nil, e
	}
	target, e := s.projectVersion(ctx, source, project, versionID)
	if e != nil {
		return nil, e
	}
	meta := s.projectMetas(ctx, source, []string{project})
	items := []PlanItem{}
	pos := map[string]int{}
	add := func(it PlanItem) {
		pos[it.Key] = len(items)
		items = append(items, it)
	}
	tm := meta[project]
	t := PlanItem{Key: source + ":" + project, Source: source, ProjectID: project, VersionID: target.ID, Title: tm.Title, Slug: tm.Slug, Icon: tm.Icon, Version: target.Number, Role: "target", Status: "install"}
	if m, ok := idx.find(z, source, project, tm.Slug); ok {
		t.File = m.File
		t.Note = m.Version // the present version; installing replaces it
	}
	if !compatible(target, mc, loader) {
		t.Status, t.Note = "none", fmt.Sprintf("%s %s", mc, loader)
	}
	add(t)
	emit(markUnverified(append([]PlanItem(nil), items...), idx.degraded))

	type pending struct {
		dep    Dependency
		parent string
		depth  int
	}
	level := []pending{}
	for _, d := range target.Dependencies {
		level = append(level, pending{d, t.Key, 1})
	}
	for len(level) > 0 && len(items) < 128 {
		ids := []string{}
		for _, pd := range level {
			k := source + ":" + pd.dep.ProjectID
			if _, seen := pos[k]; !seen && pd.dep.ProjectID != "" {
				ids = append(ids, pd.dep.ProjectID)
			}
		}
		metas := s.projectMetas(ctx, source, ids)
		var todo []int
		for _, pd := range level {
			k := source + ":" + pd.dep.ProjectID
			if _, seen := pos[k]; seen || pd.dep.ProjectID == "" {
				if i, ok := pos[k]; ok && items[i].Role == "optional" && pd.dep.Kind == "required" {
					items[i].Role = "required" // required somewhere wins
				}
				continue
			}
			if pd.dep.Kind == "optional" && pd.depth > 1 {
				continue // only the chosen mod's own optional extras are offered
			}
			md := metas[pd.dep.ProjectID]
			it := PlanItem{Key: k, Source: source, ProjectID: pd.dep.ProjectID, VersionID: pd.dep.VersionID, Title: md.Title, Slug: md.Slug, Icon: md.Icon, Role: pd.dep.Kind, Parent: pd.parent, Depth: pd.depth, Status: "resolving"}
			if it.Title == "" {
				it.Title = pd.dep.ProjectID
			}
			if m, ok := idx.find(z, source, pd.dep.ProjectID, md.Slug); ok {
				it.File, it.Version = m.File, m.Version
				it.Status = "installed"
				if !m.Enabled {
					it.Status = "disabled"
				}
				if it.Role == "incompatible" {
					it.Status = "conflict"
				}
			} else if it.Role == "incompatible" {
				continue // not present: nothing to warn about
			}
			add(it)
			if it.Status == "resolving" {
				todo = append(todo, pos[k])
			}
		}
		emit(markUnverified(append([]PlanItem(nil), items...), idx.degraded))
		// Pick a compatible file for each missing one, in parallel.
		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, 6)
		found := map[int]ProjectVersion{}
		for _, i := range todo {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				it := items[i]
				var v ProjectVersion
				var e error
				if it.VersionID != "" {
					v, e = s.projectVersion(ctx, source, it.ProjectID, it.VersionID)
				} else {
					var vs []ProjectVersion
					vs, e = s.Versions(ctx, source, it.ProjectID, mc, loader)
					e = firstCompatible(vs, mc, loader, &v, e)
				}
				mu.Lock()
				if e == nil {
					found[i] = v
				}
				mu.Unlock()
			}(i)
		}
		wg.Wait()
		next := []pending{}
		for _, i := range todo {
			v, ok := found[i]
			if !ok {
				items[i].Status = "none"
				continue
			}
			items[i].Status, items[i].VersionID, items[i].Version = "install", v.ID, v.Number
			for _, d := range v.Dependencies {
				if d.Kind == "required" {
					next = append(next, pending{d, items[i].Key, items[i].Depth + 1})
				}
			}
		}
		emit(markUnverified(append([]PlanItem(nil), items...), idx.degraded))
		level = next
		if ctx.Err() != nil {
			return items, ctx.Err()
		}
	}
	return markUnverified(items, idx.degraded), nil
}

func firstCompatible(vs []ProjectVersion, mc, loader string, out *ProjectVersion, err error) error {
	if err != nil {
		return err
	}
	for _, v := range vs {
		if compatible(v, mc, loader) {
			*out = v
			return nil
		}
	}
	return errors.New("没有兼容版本")
}

// PlanInstall is one checked row to install.
type PlanInstall struct {
	Key       string `json:"key"`
	Source    string `json:"source"`
	ProjectID string `json:"projectId"`
	VersionID string `json:"versionId"`
	Title     string `json:"title"`
	Replace   string `json:"replace,omitempty"`
	Depth     int    `json:"depth"`
}

// InstallPlanned enables the chosen disabled files, then installs the
// checked rows deepest-first (dependencies before what needs them). Each
// row reports running / done / failed in the job's Steps; a failure does
// not stop the others.
func (s *Service) InstallPlanned(rootID string, items []PlanInstall, enable []string, skip []string) (string, error) {
	r, e := s.getRoot(rootID)
	if e != nil {
		return "", e
	}
	if len(items) == 0 && len(enable) == 0 {
		return "", errors.New("没有要安装的项目")
	}
	return s.startJob("安装模组及前置", rootID, func(ctx context.Context, p progress) error {
		total := len(items) + len(enable)
		done := 0
		var failed []string
		for _, f := range enable {
			key := "enable:" + f
			s.setJobStep(ctx, key, "running")
			p("启用 "+strings.TrimSuffix(f, ".disabled"), done, total)
			if e := s.toggleModIn(r, f); e != nil {
				s.setJobStep(ctx, key, "failed: "+e.Error())
				failed = append(failed, f)
			} else {
				s.setJobStep(ctx, key, "done")
			}
			done++
		}
		skipSet := map[string]bool{}
		for _, k := range skip {
			skipSet[k] = true
		}
		order := append([]PlanInstall(nil), items...)
		for i := 1; i < len(order); i++ { // stable: deepest first
			for j := i; j > 0 && order[j].Depth > order[j-1].Depth; j-- {
				order[j], order[j-1] = order[j-1], order[j]
			}
		}
		for _, it := range order {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.setJobStep(ctx, it.Key, "running")
			sub := func(msg string, _, _ int) { p(msg, done, total) }
			p("安装 "+it.Title, done, total)
			if e := s.installModIn(ctx, sub, r, it.Source, it.ProjectID, it.VersionID, it.Replace, skipSet); e != nil {
				s.setJobStep(ctx, it.Key, "failed: "+e.Error())
				failed = append(failed, it.Title)
			} else {
				s.setJobStep(ctx, it.Key, "done")
				skipSet[it.Key] = true
			}
			done++
		}
		if len(failed) > 0 {
			return fmt.Errorf("%d 项未能安装：%s", len(failed), strings.Join(failed, "、"))
		}
		p("模组及前置已安装", total, total)
		return nil
	})
}

// markUnverified flags missing-looking dependencies when the installed set
// could not be read, so the UI says "unconfirmed" instead of "will install".
func markUnverified(items []PlanItem, degraded bool) []PlanItem {
	if !degraded {
		return items
	}
	for i := range items {
		if items[i].Role != "target" && items[i].Status == "install" {
			items[i].Unverified = true
		}
	}
	return items
}
