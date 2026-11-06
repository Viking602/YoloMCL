package modjar

import (
	"archive/zip"
	"context"
	"encoding/json"
	"strings"
)

// WorldgenRelevant reports whether an archive contributes terrain, biomes,
// regions, structures, dimensions, or TerraBlender registration. Crash
// reporters, UI helpers, and other non-worldgen mods return false so they stay
// off the registration classpath and mixin weave.
func WorldgenRelevant(z *zip.Reader) bool {
	if z == nil {
		return false
	}
	if hasWorldgenDatapack(z) || hasTerraBlenderAPI(z) || hasTerraBlenderEntrypoint(z) {
		return true
	}
	d, err := DiscoverTerraBlender(context.Background(), z)
	return err == nil && d.HasRegions()
}

// WorldgenRelevantStatic is WorldgenRelevant without TerraBlender class
// discovery, for callers that already ran DiscoverTerraBlender on the archive:
// WorldgenRelevant(z) == WorldgenRelevantStatic(z) || discovery.HasRegions().
func WorldgenRelevantStatic(z *zip.Reader) bool {
	return z != nil && (hasWorldgenDatapack(z) || hasTerraBlenderAPI(z) || hasTerraBlenderEntrypoint(z))
}

func hasWorldgenDatapack(z *zip.Reader) bool {
	for _, f := range z.File {
		if worldgenDataPath(f.Name) {
			return true
		}
	}
	return false
}

func worldgenDataPath(name string) bool {
	if strings.Contains(name, "..") || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") {
		return false
	}
	parts := strings.Split(name, "/")
	if len(parts) < 3 || parts[0] != "data" || parts[1] == "" {
		return false
	}
	switch parts[2] {
	case "worldgen", "dimension", "dimension_type", "biome", "lithostitched":
		return true
	case "tags":
		return len(parts) >= 4 && parts[3] == "worldgen"
	}
	return false
}

func hasTerraBlenderAPI(z *zip.Reader) bool {
	for _, f := range z.File {
		name := f.Name
		if name == "terrablender/api/Region.class" || name == "terrablender/api/TerraBlenderApi.class" ||
			strings.HasPrefix(name, "terrablender/api/") && strings.HasSuffix(name, ".class") {
			return true
		}
	}
	return false
}

func hasTerraBlenderEntrypoint(z *zip.Reader) bool {
	for _, metaName := range []string{"fabric.mod.json", "quilt.mod.json"} {
		var selected *zip.File
		for _, f := range z.File {
			if f.Name == metaName {
				if selected != nil {
					return false
				}
				selected = f
			}
		}
		if selected == nil {
			continue
		}
		raw, err := archiveBytes(context.Background(), selected, 2<<20)
		if err != nil || raw == nil {
			continue
		}
		raw = loaderJSON(raw)
		var meta map[string]json.RawMessage
		if json.Unmarshal(raw, &meta) != nil {
			continue
		}
		if metaName == "quilt.mod.json" {
			if quilt, ok := meta["quilt_loader"]; ok {
				var loader struct {
					Entrypoints map[string]json.RawMessage `json:"entrypoints"`
				}
				if json.Unmarshal(quilt, &loader) == nil {
					if _, ok := loader.Entrypoints["terrablender"]; ok {
						return true
					}
				}
			}
		}
		entries := meta["entrypoints"]
		if len(entries) == 0 {
			continue
		}
		var hooks map[string]json.RawMessage
		if json.Unmarshal(entries, &hooks) != nil {
			continue
		}
		if _, ok := hooks["terrablender"]; ok {
			return true
		}
	}
	return false
}

// ClientOnlyMod reports a loader descriptor that targets the client distribution
// exclusively. Worldgen datapack archives still count as relevant even when
// marked client-only.
func ClientOnlyMod(z *zip.Reader) bool {
	if z == nil || hasWorldgenDatapack(z) {
		return false
	}
	for _, metaName := range []string{"fabric.mod.json", "quilt.mod.json"} {
		var selected *zip.File
		for _, f := range z.File {
			if f.Name == metaName {
				selected = f
				break
			}
		}
		if selected == nil {
			continue
		}
		raw, err := archiveBytes(context.Background(), selected, 2<<20)
		if err != nil || raw == nil {
			continue
		}
		var meta struct {
			Environment string `json:"environment"`
		}
		if json.Unmarshal(loaderJSON(raw), &meta) == nil && meta.Environment == "client" {
			return true
		}
	}
	return false
}
