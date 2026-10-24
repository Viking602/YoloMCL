package nativeui

import (
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

func (a *App) addonEntries() []addonEntry {
	var out []addonEntry
	if a.addonTab == "mods" {
		inst := ""
		if v := a.gameVersion(); v != nil {
			inst = strings.ToLower(v.Loader)
		}
		have := map[string]bool{}
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
			it := a.modItem(*m)
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
	q := strings.ToLower(strings.TrimSpace(a.modFilter))
	var out []addonEntry
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
	return out
}

func (a *App) addonKindWord() string {
	switch a.addonTab {
	case "resourcepack":
		return a.L("资源包", "resource packs")
	case "shader":
		return a.L("光影", "shaders")
	}
	return a.L("模组", "mods")
}
