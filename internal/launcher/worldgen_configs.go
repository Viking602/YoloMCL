package launcher

import (
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// worldgenConfigDecl is one mod adapter's declaration of the config/ files
// that change what it generates. Every adapter reads them through the one
// shared reader (readWorldgenModConfigs) into WorldgenSnapshot.ModConfigs,
// so the snapshot hash changes when any of them does. A file name may hold
// filepath.Match wildcards, for configs whose names carry the loader and
// Minecraft version (YUNG's "betterdungeons-neoforge-1_21.toml").
type worldgenConfigDecl struct {
	ModID string
	Files []string
}

var worldgenConfigDecls = []worldgenConfigDecl{
	{"terrablender", []string{"terrablender.toml"}},
	{"blueprint", []string{"blueprint-common.toml"}},
	{"biomesoplenty", []string{"biomesoplenty/generation.toml", "biomesoplenty/biome_toggles.json"}},
	{"lithostitched", []string{"lithostitched.json"}},
	{"tectonic", []string{"tectonic.json"}},
	{"moonlight", []string{"moonlight-common.toml"}},
	{"waystones", []string{"waystones-common.toml"}},
	yungConfigDecl(),
	aquamiraeConfigDecl(),
}

// yungConfigDecl covers the YUNG structure mods: one TOML per mod with
// spawn rates and toggles, plus JSON pools for some (ores, rare blocks).
func yungConfigDecl() worldgenConfigDecl {
	d := worldgenConfigDecl{ModID: "yungsapi"}
	for _, m := range []string{
		"betterdungeons", "bettermineshafts", "betterstrongholds", "betterdeserttemples",
		"betterjungletemples", "betteroceanmonuments", "betterwitchhuts", "betterfortresses",
		"betterendisland", "bettercaves",
	} {
		d.Files = append(d.Files, m+"-*.toml", m+"/*/*.json")
	}
	return d
}

// worldgenConfigLimit leaves room under the snapshot's 128-entry cap for
// configurations adapters read lazily.
const worldgenConfigLimit = 96

// worldgenModConfigFiles expands every declaration against root/config,
// sorted and deduplicated, slash-separated relative to config/.
func worldgenModConfigFiles(root string) []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range worldgenConfigDecls {
		for _, f := range d.Files {
			if !strings.ContainsAny(f, "*?[") {
				if !seen[f] {
					seen[f] = true
					out = append(out, f)
				}
				continue
			}
			matches, _ := filepath.Glob(filepath.Join(root, "config", filepath.FromSlash(f)))
			for _, m := range matches {
				rel, err := filepath.Rel(filepath.Join(root, "config"), m)
				if err != nil {
					continue
				}
				rel = path.Clean(filepath.ToSlash(rel))
				if !seen[rel] && !strings.HasSuffix(strings.ToLower(rel), "readme.txt") {
					seen[rel] = true
					out = append(out, rel)
				}
			}
		}
	}
	sort.Strings(out)
	if len(out) > worldgenConfigLimit {
		out = out[:worldgenConfigLimit]
	}
	return out
}

// WorldgenConfigKeys reports which declared files belong to a mod id, for
// adapters and diagnostics.
func WorldgenConfigKeys(modID string) []string {
	for _, d := range worldgenConfigDecls {
		if d.ModID == modID {
			return append([]string(nil), d.Files...)
		}
	}
	return nil
}
