package launcher

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Discover only locations declared by pack loaders, not arbitrary instance
// archives. A saved world's Enabled list supplies its actual pack precedence.
// Activation is checked against installed identities during ingestion.
func seedMapGlobalPacks(ctx context.Context, root, minecraft, loader string, enabled []string, sources []WorldgenSource, issues *[]WorldgenIssue) []WorldgenSource {
	type candidate struct {
		WorldgenSource
		id       string
		required bool
	}
	var found []candidate
	add := func(loader, relative string, required bool) {
		p, err := safeJoin(root, relative)
		if err != nil {
			*issues = append(*issues, WorldgenIssue{"source_unreadable", relative})
			return
		}
		info, err := os.Stat(p)
		if os.IsNotExist(err) {
			return
		}
		if err != nil {
			*issues = append(*issues, WorldgenIssue{"source_unreadable", relative})
			return
		}
		appendPack := func(path string) {
			found = append(found, candidate{WorldgenSource{path, loader + ":" + filepath.Base(path)}, filepath.Base(path), required})
		}
		if !info.IsDir() {
			if strings.HasSuffix(info.Name(), ".zip") {
				appendPack(p)
			}
			return
		}
		if metadata, err := os.Stat(filepath.Join(p, "pack.mcmeta")); err == nil && !metadata.IsDir() {
			appendPack(p)
			return
		}
		entries, err := seedMapPackEntries(ctx, root, relative)
		if err != nil {
			*issues = append(*issues, WorldgenIssue{"source_unreadable", relative})
			return
		}
		for _, en := range entries {
			path, err := safeJoin(root, filepath.Join(relative, en.Name()))
			if err != nil {
				continue
			}
			if en.IsDir() {
				if m, e := os.Stat(filepath.Join(path, "pack.mcmeta")); e != nil || m.IsDir() {
					continue
				}
			} else if !strings.HasSuffix(en.Name(), ".zip") {
				continue
			}
			appendPack(path)
		}
	}
	configPath, err := safeJoin(root, "config/global_packs.toml")
	if err == nil {
		if raw, err := readWorldgenFile(ctx, configPath, 64<<10); err == nil {
			var config struct {
				DataPacks struct{ Required, Optional []string } `toml:"datapacks"`
			}
			if safeTOML(raw, &config) != nil {
				*issues = append(*issues, WorldgenIssue{"source_unreadable", "config/global_packs.toml"})
			} else {
				for _, path := range config.DataPacks.Required {
					add("globalpacks", path, true)
				}
				for _, path := range config.DataPacks.Optional {
					add("globalpacks", path, false)
				}
			}
		}
	}
	globalCount := len(found)
	// Paxi 5.1: unlisted packs first; explicit loadOrder entries last. Pack IDs
	// are basenames. A configured external entry is relative to the instance.
	paxiPath, err := safeJoin(root, "config/paxi")
	if err == nil {
		if info, err := os.Stat(paxiPath); err == nil && info.IsDir() {
			if seedMapPaxiBasePacks(ctx, root, minecraft, loader, issues) {
				add("paxi", "datapacks", true)
			}
			add("paxi", "config/paxi/datapacks", true)
			var order struct {
				LoadOrder []string `json:"loadOrder"`
			}
			p, err := safeJoin(root, "config/paxi/datapack_load_order.json")
			if err == nil {
				if raw, err := readWorldgenFile(ctx, p, 64<<10); err == nil {
					raw, valid := worldgenJSONComments(raw)
					if !valid || json.Unmarshal(raw, &order) != nil {
						*issues = append(*issues, WorldgenIssue{"pack_order", "paxi"})
					}
				}
			}
			for _, name := range order.LoadOrder {
				p, err := safeJoin(root, name)
				if err == nil {
					if _, err = os.Stat(p); err == nil {
						add("paxi", name, true)
						continue
					}
				}
				add("paxi", filepath.Join("config/paxi/datapacks", name), true)
			}
		}
	}
	// Global Packs' required IDs are selected in descending name order in
	// Fabric 1.21.1. Saved selection always supersedes that initial ordering.
	sort.SliceStable(found[:globalCount], func(i, j int) bool { return found[i].id > found[j].id })
	selected := make(map[string]int, len(enabled))
	for i, id := range enabled {
		selected[id] = i
	}
	// Pack repositories identify duplicates by ID, even across configured paths.
	last := make(map[string]int, len(found))
	for i, p := range found {
		last[p.id] = i
	}
	var kept []candidate
	for i, p := range found {
		if last[p.id] != i {
			continue
		}
		if enabled != nil {
			if _, ok := selected[p.id]; !ok {
				continue
			}
		} else if !p.required {
			continue
		}
		kept = append(kept, p)
	}
	if enabled != nil {
		sort.SliceStable(kept, func(i, j int) bool { return selected[kept[i].id] < selected[kept[j].id] })
	}
	for _, p := range kept {
		sources = append(sources, p.WorldgenSource)
	}
	return sources
}

func seedMapPaxiBasePacks(ctx context.Context, root, minecraft, loader string, issues *[]WorldgenIssue) bool {
	parts := strings.Split(minecraft, ".")
	if len(parts) < 2 {
		return true
	}
	version := parts[0] + "_" + parts[1]
	name := "config/paxi-" + loader + "-" + version + ".toml"
	if loader == "fabric" {
		name = "config/paxi-fabric-" + version + ".json"
	}
	path, err := safeJoin(root, name)
	if err != nil {
		return true
	}
	raw, err := readWorldgenFile(ctx, path, 64<<10)
	if os.IsNotExist(err) {
		return true
	}
	var enabled *bool
	if err == nil && loader == "fabric" {
		var config struct {
			General struct {
				Enabled *bool `json:"loadFromBaseDatapacksDirectory"`
			}
		}
		var ok bool
		raw, ok = worldgenJSONComments(raw)
		if !ok {
			*issues = append(*issues, WorldgenIssue{"source_unreadable", name})
			return true
		}
		err = json.Unmarshal(raw, &config)
		enabled = config.General.Enabled
	} else if err == nil {
		var config struct {
			General struct {
				Enabled *bool `toml:"Load from base 'datapacks' directory"`
			}
		}
		err = safeTOML(raw, &config)
		enabled = config.General.Enabled
	}
	if err != nil {
		*issues = append(*issues, WorldgenIssue{"source_unreadable", name})
	}
	return enabled == nil || *enabled
}
