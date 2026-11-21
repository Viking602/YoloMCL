package launcher

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	horizonModsTxtLimit    = 512 << 10
	horizonModTOMLLimit    = 64 << 10
	horizonDatapacksLimit  = 512 << 10
	horizonModFileSuffix   = ".hmod"
	horizonModManifestName = "horizon-mod.toml"
)

type horizonDatapacksConfig struct {
	Enabled  []string `json:"enabled"`
	Disabled []string `json:"disabled"`
}

// readHorizonEnabledMods parses Horizon's <gameDir>/horizon-mods.txt ("mod.<id>:true").
// Per Horizon Content API, mods without an explicit ":true" entry stay disabled.
func readHorizonEnabledMods(ctx context.Context, gameDir string) (map[string]bool, error) {
	p, err := safeJoin(gameDir, "horizon-mods.txt")
	if err != nil {
		return nil, err
	}
	raw, err := readWorldgenFile(ctx, p, horizonModsTxtLimit)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	enabled := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if !strings.HasPrefix(k, "mod.") {
			continue
		}
		id := strings.TrimSpace(strings.TrimPrefix(k, "mod."))
		if id == "" {
			continue
		}
		enabled[id] = strings.EqualFold(strings.TrimSpace(v), "true")
	}
	return enabled, nil
}

func parseHorizonModIDFromTOML(raw []byte) string {
	var meta struct {
		ID  string `toml:"id"`
		Mod struct {
			ID string `toml:"id"`
		} `toml:"mod"`
	}
	if err := safeTOML(raw, &meta); err != nil {
		return ""
	}
	if id := strings.TrimSpace(meta.ID); id != "" {
		return id
	}
	return strings.TrimSpace(meta.Mod.ID)
}

func horizonModEntryID(ctx context.Context, path, name string, isDir bool) (id string, ok bool) {
	if isDir {
		manifest, err := safeJoin(path, horizonModManifestName)
		if err != nil {
			return "", false
		}
		raw, err := readWorldgenFile(ctx, manifest, horizonModTOMLLimit)
		if err != nil {
			return "", false
		}
		if parsed := parseHorizonModIDFromTOML(raw); parsed != "" {
			return parsed, true
		}
		return name, true
	}
	if !strings.HasSuffix(strings.ToLower(name), horizonModFileSuffix) {
		return "", false
	}
	fallback := name[:len(name)-len(horizonModFileSuffix)]
	f, err := os.Open(path)
	if err != nil {
		return fallback, true
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return fallback, true
	}
	z, err := zip.NewReader(f, info.Size())
	if err != nil {
		return fallback, true
	}
	for _, entry := range z.File {
		if entry.Name == horizonModManifestName {
			if raw, rerr := readZipBytesContext(ctx, entry, horizonModTOMLLimit); rerr == nil {
				if parsed := parseHorizonModIDFromTOML(raw); parsed != "" {
					return parsed, true
				}
			}
			break
		}
	}
	return fallback, true
}

// seedMapHorizonMods discovers enabled Horizon content mods under <gameDir>/horizon-mods/.
func seedMapHorizonMods(ctx context.Context, gameDir string, issues *[]WorldgenIssue) []WorldgenSource {
	enabled, err := readHorizonEnabledMods(ctx, gameDir)
	if err != nil {
		if issues != nil {
			*issues = append(*issues, WorldgenIssue{"source_unreadable", "horizon-mods.txt"})
		}
		return nil
	}
	if len(enabled) == 0 {
		return nil
	}
	entries, err := seedMapPackEntries(ctx, gameDir, "horizon-mods")
	if err != nil {
		if issues != nil {
			*issues = append(*issues, WorldgenIssue{"source_unreadable", "horizon-mods"})
		}
		return nil
	}
	var sources []WorldgenSource
	for _, en := range entries {
		if err := ctx.Err(); err != nil {
			return sources
		}
		if en.Type()&os.ModeSymlink != 0 {
			continue
		}
		p, err := safeJoin(gameDir, "horizon-mods/"+en.Name())
		if err != nil {
			if issues != nil {
				*issues = append(*issues, WorldgenIssue{"source_unreadable", "horizon-mods/" + en.Name()})
			}
			continue
		}
		id, ok := horizonModEntryID(ctx, p, en.Name(), en.IsDir())
		if !ok || !enabled[id] {
			continue
		}
		sources = append(sources, WorldgenSource{Path: p, Name: "horizon-mod:" + en.Name()})
	}
	return sources
}

// readHorizonDatapacksJSON reads Horizon's saves/<world>/datapacks.json when present.
func readHorizonDatapacksJSON(ctx context.Context, gameDir, world string) (horizonDatapacksConfig, bool, error) {
	if world == "" {
		return horizonDatapacksConfig{}, false, nil
	}
	p, err := safeJoin(gameDir, "saves/"+world+"/datapacks.json")
	if err != nil {
		return horizonDatapacksConfig{}, false, err
	}
	raw, err := readWorldgenFile(ctx, p, horizonDatapacksLimit)
	if os.IsNotExist(err) {
		return horizonDatapacksConfig{}, false, nil
	}
	if err != nil {
		return horizonDatapacksConfig{}, false, err
	}
	var cfg horizonDatapacksConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return horizonDatapacksConfig{}, false, err
	}
	return cfg, true, nil
}

// resolveHorizonWorldDatapacks applies Horizon's datapacks.json order:
// explicit enabled packs first, disabled packs excluded, and remaining
// packs in neither list enabled by default in alphabetical order.
func resolveHorizonWorldDatapacks(available map[string]os.DirEntry, cfg horizonDatapacksConfig) (ordered, missing []string) {
	disabled := map[string]bool{}
	for _, id := range cfg.Disabled {
		if name, ok := strings.CutPrefix(id, "file/"); ok {
			disabled[name] = true
		}
	}
	seen := map[string]bool{}
	for _, id := range cfg.Enabled {
		name, ok := strings.CutPrefix(id, "file/")
		if !ok || disabled[name] || seen[name] {
			continue
		}
		if _, exists := available[name]; exists {
			ordered = append(ordered, name)
			seen[name] = true
		} else {
			missing = append(missing, id)
		}
	}
	var rest []string
	for name := range available {
		if !seen[name] && !disabled[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	ordered = append(ordered, rest...)
	return ordered, missing
}

// horizonClientJarCandidates returns ordered version IDs whose client JAR may
// carry vanilla worldgen/structure/texture assets for an installed version.
func horizonClientJarCandidates(v gameVersion) []string {
	var out []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		for _, existing := range out {
			if existing == id {
				return
			}
		}
		out = append(out, id)
	}
	add(v.Jar)
	add(minecraftVersionOf(v))
	if v.Horizon != nil {
		add(v.Horizon.Minecraft)
	}
	add(v.InheritsFrom)
	return out
}

func horizonEnabledModPaths(ctx context.Context, gameDir string) []WorldgenSource {
	return seedMapHorizonMods(ctx, gameDir, nil)
}

func openHorizonDatapacksForStore(ctx context.Context, gameDir, world string) ([]string, bool) {
	cfg, ok, err := readHorizonDatapacksJSON(ctx, gameDir, world)
	if err != nil || !ok {
		return nil, false
	}
	dir, err := safeJoin(gameDir, "saves/"+world+"/datapacks")
	if err != nil {
		return nil, true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, true
	}
	available := map[string]os.DirEntry{}
	for _, en := range entries {
		if en.IsDir() || strings.HasSuffix(strings.ToLower(en.Name()), ".zip") {
			available[en.Name()] = en
		}
	}
	ordered, _ := resolveHorizonWorldDatapacks(available, cfg)
	paths := make([]string, 0, len(ordered))
	for _, name := range ordered {
		paths = append(paths, filepath.Join(dir, name))
	}
	return paths, true
}
