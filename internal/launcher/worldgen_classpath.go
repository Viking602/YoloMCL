package launcher

import (
	"archive/zip"
	"context"
	"encoding/json"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Viking602/YoloMCL/cubiomes/modjar"
)

var (
	forgeDepHeader = regexp.MustCompile(`(?m)^\s*\[\[dependencies\.[^\]]+\]\][ \t]*(?:#.*)?\r?$`) // TOML allows a trailing comment
	forgeDepModID  = regexp.MustCompile(`(?m)^\s*modId\s*=\s*"([^"]+)"`)
	forgeDepType   = regexp.MustCompile(`(?m)^\s*type\s*=\s*"([^"]+)"`)
	forgeDepMand   = regexp.MustCompile(`(?m)^\s*mandatory\s*=\s*(true|false)`)
)

// worldgenRegistrationSources selects mod archives that belong on the
// registration classpath and mixin weave: worldgen contributors, the
// TerraBlender API jar, and the dependency closure those mods declare.
// Crash reporters and other non-worldgen helpers stay off the weave so a
// missing nested mixin config cannot fail unrelated region registration.
func worldgenRegistrationSources(ctx context.Context, paths map[string]string, mods []WorldgenMod, hasRegions []bool) map[string]bool {
	byID := map[string][]string{}
	deps := map[string][]string{}
	keep := map[string]bool{}
	// Per-archive facts are pure functions of (path, loader); compute them in
	// parallel, then combine in mod order exactly as before.
	type jarInfo struct {
		ok, relevant bool
		deps         []string
	}
	infos := make([]jarInfo, len(mods))
	var next atomic.Int64
	var wg sync.WaitGroup
	for range min(max(4, runtime.NumCPU()), len(mods)) {
		wg.Go(func() {
			for {
				i := int(next.Add(1) - 1)
				if i >= len(mods) || ctx.Err() != nil {
					return
				}
				mod := mods[i]
				path := paths[mod.Source]
				if path == "" {
					continue
				}
				regions := i < len(hasRegions) && hasRegions[i]
				key := jarFactKey(path)
				if f, ok := jarFactGet(key); ok && f.Relevant != nil && f.Deps != nil {
					if d, ok := f.Deps[mod.Loader]; ok {
						infos[i] = jarInfo{true, regions || *f.Relevant, d}
						continue
					}
				}
				z, err := zip.OpenReader(path)
				if err != nil {
					continue
				}
				// hasRegions[i] is this archive's own DiscoverTerraBlender result, so the
				// static checks plus it equal WorldgenRelevant without a second class scan.
				jarRelevant := (modjar.WorldgenRelevantStatic(&z.Reader) || regions) && !modjar.ClientOnlyMod(&z.Reader)
				d := modDeclaredDependencies(&z.Reader, mod.Loader)
				z.Close()
				infos[i] = jarInfo{true, regions || jarRelevant, d}
				jarFactUpdate(key, func(e *jarFact) {
					e.Relevant = &jarRelevant
					if e.Deps == nil {
						e.Deps = map[string][]string{}
					}
					e.Deps[mod.Loader] = append([]string{}, d...)
				})
			}
		})
	}
	wg.Wait()
	if ctx.Err() != nil {
		return keep
	}
	for i, mod := range mods {
		byID[mod.ID] = append(byID[mod.ID], mod.Source)
		if !infos[i].ok {
			continue
		}
		if infos[i].relevant {
			keep[mod.Source] = true
		}
		deps[mod.Source] = infos[i].deps
	}
	// Fixed-point dependency closure so TerraBlender/BOP prerequisites such as
	// GlitchCore stay on the classpath without an ID allow-list.
	for changed := true; changed; {
		changed = false
		for source := range keep {
			for _, id := range deps[source] {
				for _, depSource := range byID[id] {
					if !keep[depSource] {
						keep[depSource] = true
						changed = true
					}
				}
			}
		}
	}
	return keep
}

func modDeclaredDependencies(z *zip.Reader, loader string) []string {
	var ids []string
	seen := map[string]bool{}
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || id == "minecraft" || id == "java" || id == "fabricloader" || id == "quilt_loader" || id == "forge" || id == "neoforge" || seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}
	switch loader {
	case "fabric", "quilt":
		for _, name := range []string{"fabric.mod.json", "quilt.mod.json"} {
			raw := zipNamedBytes(z, name)
			if raw == nil {
				continue
			}
			var meta map[string]json.RawMessage
			if json.Unmarshal(raw, &meta) != nil {
				continue
			}
			if name == "quilt.mod.json" {
				if quilt, ok := meta["quilt_loader"]; ok {
					var q struct {
						Depends map[string]json.RawMessage `json:"depends"`
					}
					if json.Unmarshal(quilt, &q) == nil {
						for id := range q.Depends {
							add(id)
						}
					}
				}
				continue
			}
			for _, key := range []string{"depends", "requires"} {
				rawDeps := meta[key]
				if len(rawDeps) == 0 {
					continue
				}
				var obj map[string]json.RawMessage
				if json.Unmarshal(rawDeps, &obj) == nil {
					for id := range obj {
						add(id)
					}
				}
			}
		}
	default:
		for _, name := range []string{"META-INF/neoforge.mods.toml", "META-INF/mods.toml"} {
			raw := zipNamedBytes(z, name)
			if raw == nil {
				continue
			}
			text := string(raw)
			idxs := forgeDepHeader.FindAllStringIndex(text, 256)
			for i, loc := range idxs {
				start := loc[1]
				end := len(text)
				if i+1 < len(idxs) {
					end = idxs[i+1][0]
				}
				body := text[start:end]
				if cut := strings.Index(body, "\n["); cut >= 0 {
					body = body[:cut]
				}
				m := forgeDepModID.FindStringSubmatch(body)
				if m == nil {
					continue
				}
				if t := forgeDepType.FindStringSubmatch(body); t != nil {
					switch t[1] {
					case "optional", "incompatible", "discouraged":
						continue
					}
				}
				if man := forgeDepMand.FindStringSubmatch(body); man != nil && man[1] == "false" {
					continue
				}
				add(m[1])
			}
		}
	}
	return ids
}

func zipNamedBytes(z *zip.Reader, name string) []byte {
	var selected *zip.File
	for _, f := range z.File {
		if f.Name == name {
			if selected != nil {
				return nil
			}
			selected = f
		}
	}
	if selected == nil {
		return nil
	}
	raw, err := readZipBytes(selected, 2<<20)
	if err != nil {
		return nil
	}
	return raw
}
