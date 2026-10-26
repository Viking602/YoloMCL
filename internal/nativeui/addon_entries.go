package nativeui

import (
	"slices"
	"sort"
	"strings"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type addonEntry struct {
	it     addonItem
	mod    *launcher.Mod
	pack   *launcher.AssetPack
	name   string
	size   int64
	update bool
}

// Only the current mod list is prepared. The comparable key covers display
// fields; slice contents are copied once so in-place dependency/category
// changes are detected too. No extra archive or image data is retained.
type addonModKey struct {
	file, name, modID, version, loader, status, source, sha1 string
	description, latest, icon, summary, cfSlug, mrSlug       string
	size                                                     int64
	enabled, update, pending                                 bool
}

type addonModInput struct {
	key                      addonModKey
	requires, provides, cats []string
}

type addonListData struct {
	loader, language, query, filter, sort string
	zhNames                               *launcher.ZhNames
	inputs                                []addonModInput
	entries, shown                        []addonEntry
	filtered                              bool
}

func (a *App) addonModInput(m *launcher.Mod) addonModInput {
	rem := a.addonRemote[m.SHA1]
	key := addonModKey{file: m.File, name: m.Name, modID: m.ModID, version: m.Version, loader: m.Loader, sha1: m.SHA1,
		status: m.Status, source: m.Source, description: m.Description, size: m.Size, enabled: m.Enabled, pending: m.Pending,
		icon: a.modIconURLs[m.SHA1], summary: rem.Summary, cfSlug: rem.CFSlug, mrSlug: rem.MRSlug}
	if m.Latest != nil {
		key.latest, key.update = m.Latest.Number, true
	}
	return addonModInput{key: key, requires: m.Requires, provides: m.Provides, cats: rem.Cats}
}

func (a *App) addonEntries() []addonEntry {
	if a.addonTab != "mods" {
		a.addonListData = addonListData{}
		return a.buildAddonEntries()
	}
	loader := ""
	if v := a.gameVersion(); v != nil {
		loader = v.Loader
	}
	prev := &a.addonListData
	valid := len(prev.inputs) == len(a.mods) && len(prev.entries) == len(a.mods) && prev.loader == loader && prev.language == a.lang() && prev.zhNames == a.zhNames
	if valid {
		for i := range a.mods {
			input, old := a.addonModInput(&a.mods[i]), prev.inputs[i]
			if prev.entries[i].mod != &a.mods[i] || input.key != old.key || !slices.Equal(input.requires, old.requires) || !slices.Equal(input.provides, old.provides) || !slices.Equal(input.cats, old.cats) {
				valid = false
				break
			}
		}
	}
	if valid {
		return prev.entries
	}
	next := addonListData{loader: loader, language: a.lang(), zhNames: a.zhNames, inputs: make([]addonModInput, len(a.mods)), entries: a.buildAddonEntries()}
	for i := range a.mods {
		input := a.addonModInput(&a.mods[i])
		input.requires, input.provides, input.cats = slices.Clone(input.requires), slices.Clone(input.provides), slices.Clone(input.cats)
		next.inputs[i] = input
	}
	a.addonListData = next
	return next.entries
}

func (a *App) buildAddonEntries() []addonEntry {
	if a.addonTab == "mods" {
		out := make([]addonEntry, 0, len(a.mods))
		inst := ""
		if v := a.gameVersion(); v != nil {
			inst = strings.ToLower(v.Loader)
		}
		have := make(map[string]bool, len(a.mods))
		for _, m := range a.mods {
			if m.Enabled {
				have[strings.ToLower(m.ModID)] = true
				for _, p := range m.Provides {
					have[strings.ToLower(p)] = true
				}
			}
		}
		for i := range a.mods {
			m := &a.mods[i]
			it := addonItem{key: "mod-" + stableFile(m.File), kind: "mod", file: m.File, name: m.Name, enabled: m.Enabled, status: m.Status, sha1: m.SHA1, pending: m.Pending}
			if m.Latest != nil {
				it.latest = m.Latest.Number
			}
			if m.Enabled {
				for _, r := range m.Requires {
					if !have[strings.ToLower(r)] {
						it.missing = append(it.missing, r)
					}
				}
			}
			loader := a.loaderDisplay(m.Loader)
			if strings.EqualFold(m.Loader, inst) || m.Loader == "" {
				loader = "" // same as the instance: no need to repeat it on every row
			}
			it.meta = joinMeta(m.Version, loader)
			it.version, it.loader = m.Version, loader
			it.size, it.source = sizeText(m.Size), m.Source
			it.iconURL = a.modIconURLs[m.SHA1]
			p, sec := addonNames(m.Name, m.File)
			rem := a.addonRemote[m.SHA1]
			it.summary, it.cats = rem.Summary, rem.Cats
			if it.summary == "" {
				it.summary = m.Description
			}
			if !a.isEN() {
				if zh := a.zhNames.Mod(rem.CFSlug, rem.MRSlug, m.ModID); zh != "" && !hasCJK(p) {
					it.zhName = zh
				}
			}
			out = append(out, addonEntry{it: it, mod: m, name: strings.ToLower(p + " " + sec + " " + m.File + " " + m.ModID), size: m.Size, update: m.Latest != nil})
		}
		return out
	}
	out := make([]addonEntry, 0, len(a.assetPacks))
	for i := range a.assetPacks {
		p := &a.assetPacks[i]
		it := a.packItem(*p)
		it.meta = strings.Join(strings.Fields(p.Description), " ")
		it.size = sizeText(p.Size)
		rem := a.addonRemote[p.Kind+"|"+stableFile(p.File)]
		it.iconURL, it.cats = rem.Icon, rem.Cats
		it.summary = strings.Join(strings.Fields(p.Description), " ")
		if rem.Summary != "" && (it.summary == "" || len([]rune(it.summary)) < 12) {
			it.summary = rem.Summary
		}
		out = append(out, addonEntry{it: it, pack: p, name: strings.ToLower(p.Name + " " + p.File + " " + p.Description), size: p.Size})
	}
	return out
}

func (a *App) addonFilterCounts(all []addonEntry) (on, off, upd int) {
	for _, e := range all {
		if e.it.enabled {
			on++
		} else {
			off++
		}
		if e.update {
			upd++
		}
	}
	return
}

func (a *App) filteredAddons(all []addonEntry) []addonEntry {
	data := &a.addonListData
	// Mod results are read-only. Pack results remain fresh because the
	// renderer annotates their active shader badge and resource-pack rank.
	cached := a.addonTab == "mods" && len(all) > 0 && len(all) == len(data.entries) && &all[0] == &data.entries[0]
	if cached && data.filtered && data.query == a.modFilter && data.filter == a.addonFilterKind && data.sort == a.addonSortKey {
		return data.shown
	}
	q := strings.ToLower(strings.TrimSpace(a.modFilter))
	out := make([]addonEntry, 0, len(all))
	for _, e := range all {
		if q != "" && !strings.Contains(e.name, q) {
			continue
		}
		switch a.addonFilterKind {
		case "on":
			if !e.it.enabled {
				continue
			}
		case "off":
			if e.it.enabled {
				continue
			}
		case "update":
			if !e.update {
				continue
			}
		}
		out = append(out, e)
	}
	if a.addonSortKey == "size" {
		sort.SliceStable(out, func(i, j int) bool { return out[i].size > out[j].size })
	} else {
		sort.SliceStable(out, func(i, j int) bool { return out[i].name < out[j].name })
	}
	if cached {
		data.shown, data.filtered = out, true
		data.query, data.filter, data.sort = a.modFilter, a.addonFilterKind, a.addonSortKey
	}
	return out
}

func (a *App) addonKindWord() string {
	switch a.addonTab {
	case "resourcepack":
		return a.L("addon_entries.resource_packs")
	case "shader":
		return a.L("addon_entries.shaders")
	}
	return a.L("addon_entries.mods")
}
