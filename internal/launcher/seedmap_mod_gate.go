package launcher

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// This gate reads archive indexes and loader/data metadata only. It never
// loads mod classes, runs registration code or starts the seed-map sampler.
// Unknown data-driven terrain mods are detected independently of their names.
func seedMapTerrainMod(root *os.Root, name string) bool {
	return seedMapTerrainModContext(context.Background(), root, name)
}

func seedMapTerrainModContext(ctx context.Context, root *os.Root, name string) bool {
	f, err := root.Open(name)
	if err != nil {
		return true
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() > 2<<30 {
		return true
	}
	z, err := zip.NewReader(f, info.Size())
	if err != nil {
		return true // unreadable enabled mods cannot be verified as vanilla terrain
	}
	budget := 48 << 20
	blocked, err := seedMapTerrainArchive(ctx, z, 0, &budget)
	return blocked || err != nil
}

func seedMapTerrainArchive(ctx context.Context, z *zip.Reader, depth int, budget *int) (bool, error) {
	if depth > 4 || len(z.File) > 200000 {
		return false, errors.New("terrain metadata limit exceeded")
	}
	nested := map[string]bool{}
	for _, f := range z.File {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if seedMapTerrainData(f.Name) {
			return true, nil
		}
		switch f.Name {
		case "fabric.mod.json", "quilt.mod.json":
			var meta struct {
				ID, Environment string
				Depends         map[string]json.RawMessage
				Entrypoints     map[string]json.RawMessage
				Jars            []json.RawMessage
				QuiltLoader     struct {
					ID          string
					Depends     []json.RawMessage
					Entrypoints map[string]json.RawMessage
					Jars        []json.RawMessage
				} `json:"quilt_loader"`
			}
			if err := seedMapModJSON(ctx, f, &meta); err != nil {
				return false, err
			}
			if meta.Environment != "client" {
				if seedMapTerrainID(meta.ID) || seedMapTerrainID(meta.QuiltLoader.ID) || meta.Entrypoints["terrablender"] != nil || meta.QuiltLoader.Entrypoints["terrablender"] != nil {
					return true, nil
				}
				for id := range meta.Depends {
					if seedMapTerrainAPI(id) {
						return true, nil
					}
				}
				for _, id := range quiltIDs(meta.QuiltLoader.Depends, true) {
					if seedMapTerrainAPI(id) {
						return true, nil
					}
				}
			}
			for _, raw := range append(meta.Jars, meta.QuiltLoader.Jars...) {
				var path string
				if json.Unmarshal(raw, &path) != nil {
					var entry struct{ File string }
					if json.Unmarshal(raw, &entry) == nil {
						path = entry.File
					}
				}
				nested[path] = true
			}
		case "META-INF/mods.toml", "META-INF/neoforge.mods.toml":
			data, err := readZipBytesContext(ctx, f, 2<<20)
			if err != nil {
				return false, err
			}
			var meta struct {
				Mods []struct {
					ID string `toml:"modId"`
				} `toml:"mods"`
				Dependencies any `toml:"dependencies"`
			}
			if err := safeTOML(data, &meta); err != nil {
				// Multi-loader jars can carry templates for an inactive loader.
				continue
			}
			for _, mod := range meta.Mods {
				if seedMapTerrainID(mod.ID) {
					return true, nil
				}
				tables, _ := meta.Dependencies.(map[string]any)
				raw, _ := json.Marshal(tables[mod.ID])
				var deps []struct {
					ID        string `json:"modId"`
					Mandatory *bool
					Type      string
				}
				_ = json.Unmarshal(raw, &deps)
				for _, dep := range deps {
					if seedMapTerrainAPI(dep.ID) && (dep.Mandatory != nil && *dep.Mandatory || dep.Type == "required") {
						return true, nil
					}
				}
			}
		case "mcmod.info":
			var mods []struct{ ModID string }
			if seedMapModJSON(ctx, f, &mods) != nil {
				var meta struct{ ModList []struct{ ModID string } }
				if seedMapModJSON(ctx, f, &meta) == nil {
					mods = meta.ModList
				}
			}
			for _, mod := range mods {
				if seedMapTerrainID(mod.ModID) {
					return true, nil
				}
			}
		}
	}
	for _, f := range z.File {
		if !strings.HasSuffix(f.Name, ".jar") || !(nested[f.Name] || strings.HasPrefix(f.Name, "META-INF/jars/") || strings.HasPrefix(f.Name, "META-INF/jarjar/")) {
			continue
		}
		if f.UncompressedSize64 > uint64(*budget) {
			return false, errors.New("nested terrain metadata limit exceeded")
		}
		data, err := readZipBytesContext(ctx, f, uint64(*budget))
		if err != nil {
			return false, err
		}
		*budget -= len(data)
		child, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return false, err
		}
		if blocked, err := seedMapTerrainArchive(ctx, child, depth+1, budget); blocked || err != nil {
			return blocked, err
		}
	}
	return false, nil
}

func seedMapModJSON(ctx context.Context, f *zip.File, target any) error {
	raw, err := readZipBytesContext(ctx, f, 2<<20)
	if err != nil {
		return err
	}
	// Loader descriptors in shipped mods can contain comments and literal
	// newlines in descriptions. Match the existing loader-compatible reader.
	raw, ok := worldgenJSONComments(raw)
	if !ok {
		return errors.New("invalid mod JSON comments")
	}
	return json.Unmarshal(worldgenJSONControls(raw), target)
}

// CheckSeedMapMods classifies only the selected instance, on the UI's background
// command worker. Cold startup and Snapshot read stamps/counts without opening
// every installed mod jar. Results reuse the normal inventory invalidation.
func (s *Service) CheckSeedMapMods(ctx context.Context, rootID, versionID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r, err := s.rootAt(rootID, versionID)
	if err != nil {
		return err
	}
	data := s.rootInventory(r)
	dir := data.Directories[r.SelectedVersion]
	if dir == "" {
		dir = "."
	}
	count := data.Mods[dir]
	if count.SeedMapChecked {
		return nil
	}
	root, err := openScanRoot(r.Path)
	if err != nil {
		return err
	}
	defer root.Close()
	mods := maps.Clone(data.TerrainMods)
	var files []string
	for file := range data.Watched {
		if filepath.Dir(file) == filepath.Join(dir, "mods") && strings.HasSuffix(strings.ToLower(file), ".jar") {
			files = append(files, file)
		}
	}
	slices.Sort(files)
	blocked := false
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		value, known := mods[file]
		if !known {
			value = seedMapTerrainModContext(ctx, root, file)
			mods[file] = value
		}
		if value {
			blocked = true
			break
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !inventoryUnchanged(r.Path, data) {
		s.invalidateInventory(rootID)
		return nil
	}
	next := *data
	next.Mods, next.TerrainMods = maps.Clone(data.Mods), mods
	count.SeedMapChecked, count.SeedMapDisabled = true, blocked
	next.Mods[dir] = count
	entry := s.inventoryEntry(r.Path)
	entry.mu.Lock()
	if entry.data != data || entry.dirty {
		entry.mu.Unlock()
		return nil
	}
	entry.data = &next
	entry.mu.Unlock()
	s.persistInventory(entry, &next)
	return nil
}

func seedMapTerrainData(name string) bool {
	if !strings.HasSuffix(name, ".json") || strings.Contains(name, "..") || strings.ContainsAny(name, "\\:") || strings.HasPrefix(name, "/") {
		return false
	}
	// Bundled optional packs are included conservatively: the installed mod
	// can enable them at runtime without altering its jar or loader descriptor.
	if i := strings.Index(name, "/data/"); i >= 0 {
		name = name[i+1:]
	}
	parts := strings.Split(name, "/")
	if len(parts) < 4 || parts[0] != "data" || parts[1] == "" {
		return false
	}
	switch parts[2] {
	case "dimension", "dimension_type", "biome", "lithostitched":
		return true
	case "worldgen":
		switch parts[3] {
		case "biome", "noise_settings", "noise", "density_function", "multi_noise_biome_source_parameter_list":
			return true
		}
	}
	return false
}

func seedMapTerrainAPI(id string) bool {
	switch strings.ToLower(id) {
	case "terrablender", "lithostitched", "biolith", "bclib":
		return true
	}
	return false
}

func seedMapTerrainID(id string) bool {
	if seedMapTerrainAPI(id) {
		return true
	}
	switch strings.ToLower(id) {
	case "terralith", "tectonic", "biomesoplenty", "byg", "biomeswevegone", "regions_unexplored", "regionsunexplored",
		"traverse", "terrestria", "cinderscapes", "biomemakeover", "terraforged", "terra", "tfc", "terrafirmacraft",
		"otg", "openterraingenerator", "rtg", "bigglobe", "modern_beta", "modernbeta", "continents", "lithosphere",
		"betterend", "betternether", "aether", "deep_aether", "twilightforest":
		return true
	}
	return false
}
