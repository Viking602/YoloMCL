package launcher

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// StructureAssetStore builds a layered asset store for structure preview:
// game client jar (lowest) → enabled mods → enabled resource packs (highest).
func (s *Service) StructureAssetStore(rootID, versionID string) (*structure3d.Store, error) {
	return s.StructureAssetStoreWorld(rootID, versionID, "")
}

// StructureAssetStoreWorld is StructureAssetStore plus the world's datapacks
// (saves/<world>/datapacks), which override mods like the game does.
// Mods and packs come from the instance game folder (VersionDir), not the
// launcher root: isolated instances keep them under versions/<id>/.
func (s *Service) StructureAssetStoreWorld(rootID, versionID, world string) (*structure3d.Store, error) {
	return s.StructureAssetStoreEra(rootID, versionID, world, nil)
}

// StructureAssetStoreEra is StructureAssetStoreWorld for a generation era
// (GenEra.Without): mod jars whose name contains one of without are left out,
// so a pre-Lithostitched era uses vanilla jigsaw and vanilla pool files.
func (s *Service) StructureAssetStoreEra(rootID, versionID, world string, without []string) (*structure3d.Store, error) {
	r, err := s.rootAt(rootID, versionID)
	if err != nil {
		return nil, err
	}
	store := &structure3d.Store{}
	gameDir := r.VersionDir()
	// Client jar
	jarName := r.SelectedVersion
	if versionID != "" {
		jarName = versionID
	}
	client := filepath.Join(r.Path, "versions", jarName, jarName+".jar")
	if st, e := os.Stat(client); e == nil && !st.IsDir() {
		_ = store.OpenJar("client:"+jarName, client)
	} else {
		// inherited vanilla jar: try versions/<inherits>/<inherits>.jar via version json later; best-effort scan
		_ = openInheritedClientJar(store, r.Path, jarName)
	}
	mods, _ := s.ListModsAt(rootID, versionID)
	mods = sortModsForAssetLayers(mods)
	for _, m := range mods {
		if !m.Enabled || !strings.HasSuffix(strings.ToLower(m.File), ".jar") || eraExcluded(m.File, without) {
			continue
		}
		path := filepath.Join(gameDir, "mods", m.File)
		_ = store.OpenJar("mod:"+m.File, path)
	}
	structure3d.DetectLithostitched(store, gameDir)
	structure3d.ApplyKnownModPoolEdits(store)
	for _, m := range mods {
		if m.Enabled && strings.Contains(strings.ToLower(m.File), "tectonic") && !eraExcluded(m.File, without) {
			if cfg := readTectonicConfig(gameDir); !tectonicPackDisabled && cfg.bool("general.mod_enabled", true) {
				store.SetMonumentYOffset(int(cfg.num("oceans.monument_offset", 0)))
			}
			break
		}
	}
	// KubeJS loads kubejs/data as a datapack and kubejs/assets as a
	// resource pack, both above mods and below user packs.
	if kjs := filepath.Join(gameDir, "kubejs"); dirHasAny(kjs, "data", "assets") {
		_ = store.OpenDir("kubejs", kjs)
	}
	openStructureResourcePacks(store, gameDir, mods, func() []AssetPack {
		packs, _ := s.ListAssetPacksAt(rootID, versionID, "resourcepack")
		return packs
	})
	openDatapackDir(store, "global", filepath.Join(gameDir, "datapacks"))
	if world != "" {
		if wdir, e := safeJoin(gameDir, "saves/"+world); e == nil {
			structure3d.OpenWorldDatapacks(store, world, wdir) // level.dat enabled order
		}
	}
	return store, nil
}

// openDatapackDir layers every zip or folder datapack in dir, in name order.
func openDatapackDir(store *structure3d.Store, label, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		low := strings.ToLower(e.Name())
		switch {
		case e.IsDir():
			if _, err := os.Stat(filepath.Join(p, "data")); err == nil {
				_ = store.OpenDir("datapack:"+label+":"+e.Name(), p)
			}
		case strings.HasSuffix(low, ".zip") || strings.HasSuffix(low, ".jar"):
			_ = store.OpenJar("datapack:"+label+":"+e.Name(), p)
		}
	}
}

func openInheritedClientJar(store *structure3d.Store, root, id string) error {
	// Walk versions for a jar that exists matching id or parent folders.
	dir := filepath.Join(root, "versions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		jar := filepath.Join(dir, e.Name(), e.Name()+".jar")
		if st, err := os.Stat(jar); err == nil && !st.IsDir() {
			// Prefer exact id match; otherwise open first as fallback only when id jar missing.
			if e.Name() == id {
				return store.OpenJar("client:"+e.Name(), jar)
			}
		}
	}
	return nil
}

// PreviewStructure renders a structure at a seedmap hit using layered assets.
func PreviewStructure(store *structure3d.Store, structureOrTemplate string, seed int64, x, z int, surface structure3d.HeightFunc, w, h int, cam structure3d.Camera) (*structure3d.PreviewResult, error) {
	return PreviewStructureIn(store, structureOrTemplate, "", seed, x, z, surface, w, h, cam)
}

// PreviewStructureIn is PreviewStructure in a seedmap dimension, which sets
// the build height jigsaw structures are fitted into.
func PreviewStructureIn(store *structure3d.Store, structureOrTemplate, dimension string, seed int64, x, z int, surface structure3d.HeightFunc, w, h int, cam structure3d.Camera) (*structure3d.PreviewResult, error) {
	return PreviewStructureGround(store, structureOrTemplate, dimension, seed, x, z, surface, nil, w, h, cam)
}

// PreviewStructureGround is PreviewStructureIn with the pre-structure
// terrain the code-built pieces carve into and rest on (nil: estimated from
// the surface height).
func PreviewStructureGround(store *structure3d.Store, structureOrTemplate, dimension string, seed int64, x, z int, surface structure3d.HeightFunc, ground structure3d.Ground, w, h int, cam structure3d.Camera) (*structure3d.PreviewResult, error) {
	return PreviewStructureSea(store, structureOrTemplate, dimension, seed, x, z, surface, ground, 0, w, h, cam)
}

// PreviewStructureSea is PreviewStructureGround for a surface that is the
// solid noise terrain of a dimension whose sea surface is at seaLevel:
// jigsaw starts then use the sea-capped WORLD_SURFACE_WG height, or the sea
// bed for OCEAN_FLOOR starts (0: surface is used as is).
func PreviewStructureSea(store *structure3d.Store, structureOrTemplate, dimension string, seed int64, x, z int, surface structure3d.HeightFunc, ground structure3d.Ground, seaLevel, w, h int, cam structure3d.Camera) (*structure3d.PreviewResult, error) {
	req := structure3d.PreviewRequest{
		Seed: seed, X: x, Z: z, Width: w, Height: h, Camera: cam, Surface: surface, Dimension: dimension, Ground: ground, SeaLevel: seaLevel,
	}
	// Prefer structure registry id; fall back to template path when structure JSON is absent.
	if _, err := store.Read(structure3d.DataPath(nsOf(structureOrTemplate), "worldgen/structure", pathOf(structureOrTemplate), ".json")); err == nil {
		req.StructureID = structureOrTemplate
	} else if store.HasTemplate(structureOrTemplate) {
		req.TemplateID = structureOrTemplate
	} else {
		return nil, &structure3d.PreviewError{Kind: structure3d.KindMissingStructure, ID: structureOrTemplate, Err: err}
	}
	return structure3d.Preview(store, req)
}

func nsOf(id string) string   { ns, _ := structure3d.Namespaced(id); return ns }
func pathOf(id string) string { _, p := structure3d.Namespaced(id); return p }

// ResolvePreviewTarget maps a seedmap feature id (structure or structure_set) to a
// structure registry id or template id suitable for PreviewStructure.
func ResolvePreviewTarget(store *structure3d.Store, featureID, biome string) (string, error) {
	ns, name := structure3d.Namespaced(featureID)
	if _, err := store.Read(structure3d.DataPath(ns, "worldgen/structure", name, ".json")); err == nil {
		return featureID, nil
	}
	if data, err := store.Read(structure3d.DataPath(ns, "worldgen/structure_set", name, ".json")); err == nil {
		var set struct {
			Structures []struct {
				Structure string `json:"structure"`
				Weight    int    `json:"weight"`
			} `json:"structures"`
		}
		if json.Unmarshal(data, &set) == nil && len(set.Structures) > 0 {
			if strings.Contains(biome, "badlands") {
				for _, s := range set.Structures {
					if strings.HasSuffix(s.Structure, "mineshaft_mesa") {
						return s.Structure, nil
					}
				}
			}
			if biome != "" {
				for _, s := range set.Structures {
					if strings.Contains(s.Structure, biomeVillageSuffix(biome)) {
						return s.Structure, nil
					}
				}
			}
			return set.Structures[0].Structure, nil
		}
	}
	if _, err := store.Read(structure3d.DataPath(ns, "structure", name, ".nbt")); err == nil {
		return featureID, nil
	}
	return featureID, nil
}

func biomeVillageSuffix(biome string) string {
	switch {
	case strings.Contains(biome, "desert"):
		return "desert"
	case strings.Contains(biome, "savanna"):
		return "savanna"
	case strings.Contains(biome, "snow"), strings.Contains(biome, "ice"):
		return "snowy"
	case strings.Contains(biome, "taiga"):
		return "taiga"
	default:
		return "plains"
	}
}

func dirHasAny(dir string, subs ...string) bool {
	for _, sub := range subs {
		if st, err := os.Stat(filepath.Join(dir, sub)); err == nil && st.IsDir() {
			return true
		}
	}
	return false
}

// openStructureResourcePacks layers resource packs for 3D preview.
// When options.txt lists resourcePacks, file/ entries and modid:pack embedded
// packs are opened lowest-priority first so later Open* overlays match
// GameOptions.ResourcePackOrder (highest-first). Without options.txt, every
// listed pack and every folder pack with pack.mcmeta is opened (alpha).
func openStructureResourcePacks(store *structure3d.Store, gameDir string, mods []Mod, listFallback func() []AssetPack) {
	openOne := func(name string) {
		path := filepath.Join(gameDir, "resourcepacks", name)
		st, err := os.Stat(path)
		if err != nil {
			return
		}
		if st.IsDir() {
			_ = store.OpenDir("pack:"+name, path)
			return
		}
		low := strings.ToLower(name)
		if strings.HasSuffix(low, ".zip") || strings.HasSuffix(low, ".jar") {
			_ = store.OpenJar("pack:"+name, path)
		}
	}
	openEntry := func(id string) {
		if strings.Contains(id, ":") {
			openEmbeddedResourcePack(store, mods, gameDir, id)
			return
		}
		openOne(id)
	}
	opts, err := readGameOptions(context.Background(), gameDir)
	if err == nil && opts.HasOptions {
		// ResourcePackOrder is highest-first; reverse so later overlays win.
		for i := len(opts.ResourcePackOrder) - 1; i >= 0; i-- {
			openEntry(opts.ResourcePackOrder[i])
		}
		return
	}
	seen := map[string]bool{}
	for _, p := range listFallback() {
		if !p.Enabled {
			continue
		}
		seen[strings.ToLower(p.File)] = true
		openOne(p.File)
	}
	// Folder packs are skipped by zip-only listing history; include alphabetically.
	entries, _ := os.ReadDir(filepath.Join(gameDir, "resourcepacks"))
	var folders []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if seen[strings.ToLower(name)] {
			continue
		}
		if _, err := os.Stat(filepath.Join(gameDir, "resourcepacks", name, "pack.mcmeta")); err != nil {
			continue
		}
		folders = append(folders, name)
	}
	sort.Strings(folders)
	for _, name := range folders {
		openOne(name)
	}
}

// openEmbeddedResourcePack layers resourcepacks/<pack>/ from the enabled mod
// jar that owns modid (ModID or Provides). Paths remapped via OpenJarSubroot.
func openEmbeddedResourcePack(store *structure3d.Store, mods []Mod, gameDir, id string) {
	modID, pack, ok := strings.Cut(id, ":")
	if !ok || modID == "" || pack == "" {
		return
	}
	want := strings.ToLower(modID)
	root := "resourcepacks/" + pack + "/"
	for _, m := range mods {
		if !m.Enabled || !strings.HasSuffix(strings.ToLower(m.File), ".jar") {
			continue
		}
		ids := append([]string{m.ModID}, m.Provides...)
		match := false
		for _, x := range ids {
			if strings.ToLower(strings.TrimSpace(x)) == want {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		path := filepath.Join(gameDir, "mods", m.File)
		_ = store.OpenJarSubroot("pack:"+id, path, root)
		return
	}
}

// sortModsForAssetLayers orders jars so required mods open before dependents
// (Forge/Fabric classpath overlay). Alphabetical File name breaks ties; cycles
// keep remaining jars in that alpha order. Matches Requires/Provides metadata.
func sortModsForAssetLayers(mods []Mod) []Mod {
	n := len(mods)
	if n < 2 {
		return mods
	}
	provide := map[string][]int{}
	for i, m := range mods {
		for _, id := range append([]string{m.ModID}, m.Provides...) {
			id = strings.ToLower(strings.TrimSpace(id))
			if id == "" {
				continue
			}
			provide[id] = append(provide[id], i)
		}
	}
	indeg := make([]int, n)
	adj := make([][]int, n)
	for i, m := range mods {
		seen := map[int]bool{}
		for _, req := range m.Requires {
			req = strings.ToLower(strings.TrimSpace(req))
			for _, j := range provide[req] {
				if j == i || seen[j] {
					continue
				}
				seen[j] = true
				adj[j] = append(adj[j], i)
				indeg[i]++
			}
		}
	}
	ready := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if indeg[i] == 0 {
			ready = append(ready, i)
		}
	}
	lessFile := func(a, b int) bool {
		fa, fb := strings.ToLower(mods[a].File), strings.ToLower(mods[b].File)
		if fa != fb {
			return fa < fb
		}
		return a < b
	}
	sort.Slice(ready, func(a, b int) bool { return lessFile(ready[a], ready[b]) })
	out := make([]Mod, 0, n)
	placed := make([]bool, n)
	for len(ready) > 0 {
		i := ready[0]
		ready = ready[1:]
		out = append(out, mods[i])
		placed[i] = true
		var added []int
		for _, k := range adj[i] {
			indeg[k]--
			if indeg[k] == 0 {
				added = append(added, k)
			}
		}
		if len(added) > 0 {
			ready = append(ready, added...)
			sort.Slice(ready, func(a, b int) bool { return lessFile(ready[a], ready[b]) })
		}
	}
	if len(out) < n {
		var rest []int
		for i := 0; i < n; i++ {
			if !placed[i] {
				rest = append(rest, i)
			}
		}
		sort.Slice(rest, func(a, b int) bool { return lessFile(rest[a], rest[b]) })
		for _, i := range rest {
			out = append(out, mods[i])
		}
	}
	return out
}
