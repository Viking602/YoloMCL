package launcher

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Viking602/YoloMCL/cubiomes"
	"github.com/Viking602/YoloMCL/cubiomes/modjar"
)

const worldgenSchema = 20

// WorldgenDocument is a winning registry entry, with its pack provenance.
type WorldgenDocument = cubiomes.Document

// WorldgenIssue is a bounded diagnostic; Code is translated by the UI.
type WorldgenIssue struct{ Code, Source string }

// WorldgenSnapshot is an immutable view of registry data, source adapter
// identities and configuration. Its hash identifies all prediction inputs.
type WorldgenSnapshot struct {
	Schema              int
	Hash, Minecraft     string
	Loader              string `json:",omitempty"`
	LoaderVersion       string `json:",omitempty"`
	JavaMajor           int    `json:",omitempty"`
	Documents           []WorldgenDocument
	Issues              []WorldgenIssue
	Generators          map[string]string
	SavedDimensions     map[string]json.RawMessage
	HasMods, SavedWorld bool
	PackFormat          *WorldgenPackFormat     `json:",omitempty"`
	WorldName           string                  `json:",omitempty"`
	EnabledPacks        []string                `json:",omitempty"`
	Mods                []WorldgenMod           `json:",omitempty"`
	ModConfigs          map[string][]byte       `json:",omitempty"`
	RegionPrograms      []WorldgenRegionProgram `json:",omitempty"`
	BiomePrograms       []WorldgenBiomeProgram  `json:",omitempty"`
}

// WorldgenRegionProgram contains the native inputs captured from a discovered
// framework consumer. It retains errors locally instead of classifying every
// installed mod by the presence of executable code.
type WorldgenRegionProgram struct {
	ModID, Source string
	Discovery     modjar.Discovery `json:",omitzero"`
	Result        *modjar.Result   `json:",omitempty"`
	Error         string           `json:",omitempty"`
}

func (s *WorldgenSnapshot) issue(code, source string) {
	for _, issue := range s.Issues {
		if issue.Code == code && issue.Source == source {
			return
		}
	}
	if len(s.Issues) < 128 {
		s.Issues = append(s.Issues, WorldgenIssue{code, source})
	}
}

// WorldgenSource is a read-only directory, JAR or ZIP, in increasing priority.
type WorldgenSource struct{ Path, Name string }

// WorldgenInput describes local sources only. CacheDir may be empty to disable
// disk caching. Callers must resolve paths within registered instance roots.
type WorldgenInput struct {
	Minecraft, CacheDir string
	JavaMajor           int
	Loader              string
	LoaderVersion       string
	Sources             []WorldgenSource
	// Libraries are the selected profile's installed classpath dependencies.
	// They supply executable classes, not additional mod or datapack entries.
	Libraries           []WorldgenSource
	Symbols             *modjar.Symbols
	classPathErr        error
	Generators          map[string]string
	SavedDimensions     map[string]json.RawMessage
	HasMods, SavedWorld bool
	Issues              []WorldgenIssue
	// PackFormat can be supplied by callers with an official version manifest.
	// Installed version.json takes precedence over the verified release fallback.
	PackFormat   *WorldgenPackFormat
	WorldName    string
	EnabledPacks []string
	ModConfigs   map[string][]byte
	// ReadModConfig resolves only filenames requested by a supported framework
	// registration. A missing file returns nil, nil and uses the mod's defaults.
	ReadModConfig func(string) ([]byte, error)
}

// SeedMapWorldgen reads save metadata and installed resources without launching
// Minecraft, resolving downloads, or opening any .mca files.
func (s *Service) SeedMapWorldgen(ctx context.Context, rootID, versionID, world string) (*WorldgenSnapshot, LevelInfo, error) {
	return s.seedMapWorldgenEra(ctx, rootID, versionID, world, nil)
}

// seedMapWorldgenEra is SeedMapWorldgen for a generation era: mod jars whose
// file name contains one of without (lower case) are left out, as if the
// chunks had been generated before they were installed (see genera.go).
func (s *Service) seedMapWorldgenEra(ctx context.Context, rootID, versionID, world string, without []string) (*WorldgenSnapshot, LevelInfo, error) {
	r, err := s.rootAt(rootID, versionID)
	if err != nil {
		return nil, LevelInfo{}, err
	}
	input := WorldgenInput{Minecraft: r.SelectedVersion, CacheDir: filepath.Join(s.dir, "cache", "seedmap")}
	input.WorldName = world
	seedMapProgress(ctx, "prepare", "")
	pre := seedMapStage("pre.version")
	v, err := seedMapInstalledVersion(ctx, r.Path, r.SelectedVersion)
	if err != nil {
		return nil, LevelInfo{}, fmt.Errorf("resolve worldgen instance profile: %w", err)
	}
	if r.SelectedVersion != "" {
		input.Minecraft = minecraftVersionOf(v)
		input.Loader = installedLoader(v)
		input.JavaMajor = v.JavaVersion.MajorVersion
		input.LoaderVersion = loaderVersion(v, input.Loader, input.Minecraft)
	}
	pre("pre.libraries")
	if libraries, e := seedMapVersionLibraries(ctx, r.Path, v); e == nil {
		input.Libraries = libraries
		// The loader itself is a mod whose data (c: and forge: tags) joins the
		// datapack list ahead of the mods folder, as in game.
		for _, lib := range libraries {
			if (strings.HasPrefix(lib.Name, "library:net.neoforged:neoforge:") || strings.Contains(lib.Name, ":forge:")) && strings.HasSuffix(lib.Name, ":universal") {
				input.Sources = append(input.Sources, WorldgenSource{lib.Path, "mod:" + filepath.Base(lib.Path)})
			}
		}
	} else if ctx.Err() != nil {
		return nil, LevelInfo{}, ctx.Err()
	} else {
		input.classPathErr = e
		input.Issues = append(input.Issues, WorldgenIssue{"source_unreadable", "classpath: " + e.Error()})
	}
	pre("pre.symbols")
	if symbols, e := s.seedMapSymbols(ctx, r.Path, v); e == nil {
		input.Symbols = symbols
	} else if ctx.Err() != nil {
		return nil, LevelInfo{}, ctx.Err()
	} else {
		input.classPathErr = e
		input.Issues = append(input.Issues, WorldgenIssue{"source_unreadable", "game mappings: " + e.Error()})
	}
	// The normal automatic launch selects the manifest's exact major. An
	// explicit runtime can differ, so read its release file without execution.
	pre("pre.runtime")
	if selected, _, _, e := r.ResolveRuntime(r.SelectedVersion, s.settings()); e == nil && selected != "" {
		input.JavaMajor = 0
		if info, e := javaReleaseInfo(selected); e == nil {
			input.JavaMajor = info.Major
		}
	}
	if p := vanillaClientJar(r.Path, input.Minecraft, r.SelectedVersion); p != "" {
		input.Sources = append(input.Sources, WorldgenSource{p, "vanilla:" + input.Minecraft})
	}
	pre("pre.level+packs")
	gameDir := r.VersionDir()
	var level LevelInfo
	if world != "" {
		if err := validWorldDir(world); err != nil {
			return nil, level, err
		}
		p, e := safeJoin(gameDir, "saves/"+world+"/level.dat")
		if e != nil {
			return nil, level, e
		}
		f, e := os.Open(p)
		if e != nil {
			return nil, level, e
		}
		level, err = parseLevelDat(ctx, f)
		f.Close()
		if err != nil {
			return nil, level, err
		}
		input.Generators, input.SavedWorld = level.Generators, true
		if level.HasDataPacks {
			input.EnabledPacks = append([]string{}, level.EnabledDataPacks...)
		}
		input.SavedDimensions = level.Dimensions
		if len(input.SavedDimensions) == 0 {
			input.SavedDimensions = seedMapLegacySavedDimensions(input.Minecraft, level)
		}
	}
	mods, err := seedMapPackEntries(ctx, gameDir, "mods")
	if err != nil {
		input.Issues = append(input.Issues, WorldgenIssue{"source_unreadable", "mods"})
	}
	for _, en := range mods {
		if !en.IsDir() && strings.HasSuffix(strings.ToLower(en.Name()), ".jar") {
			input.HasMods = true
			if p, e := safeJoin(gameDir, "mods/"+en.Name()); e == nil {
				input.Sources = append(input.Sources, WorldgenSource{p, "mod:" + en.Name()})
			} else {
				input.Issues = append(input.Issues, WorldgenIssue{"source_unreadable", en.Name()})
			}
		}
	}
	packDir := "datapacks"
	if world != "" {
		packDir = "saves/" + world + "/datapacks"
	}
	packs, err := seedMapPackEntries(ctx, gameDir, packDir)
	if err != nil {
		input.Issues = append(input.Issues, WorldgenIssue{"source_unreadable", packDir})
	}
	available := map[string]os.DirEntry{}
	var ordered []string
	for _, en := range packs {
		if en.IsDir() || strings.HasSuffix(strings.ToLower(en.Name()), ".zip") {
			available[en.Name()] = en
			ordered = append(ordered, en.Name())
		}
	}
	if level.HasDataPacks {
		ordered = nil
		for _, name := range level.EnabledDataPacks {
			if strings.HasPrefix(name, "file/") {
				n := strings.TrimPrefix(name, "file/")
				if _, ok := available[n]; ok {
					ordered = append(ordered, n)
				} else {
					input.Issues = append(input.Issues, WorldgenIssue{"source_unreadable", name})
				}
			}
		}
	} else if len(ordered) > 0 {
		input.Issues = append(input.Issues, WorldgenIssue{"pack_order", packDir})
	}
	for _, name := range ordered {
		p, e := safeJoin(gameDir, packDir+"/"+name)
		if e != nil {
			input.Issues = append(input.Issues, WorldgenIssue{"source_unreadable", name})
			continue
		}
		input.Sources = append(input.Sources, WorldgenSource{p, "datapack:" + name})
	}
	input.Sources = seedMapGlobalPacks(ctx, gameDir, input.Minecraft, input.Loader, input.EnabledPacks, input.Sources, &input.Issues)
	input.Sources = seedMapInstancePacks(ctx, gameDir, input.Sources, &input.Issues)
	appendKubeJSWorldgenIssues(gameDir, &input.Issues)
	input.Sources = withoutEraMods(input.Sources, without)
	input.Sources = seedMapTectonicPack(gameDir, input.Sources)
	pre("pre.configs")
	input.ModConfigs = readWorldgenModConfigs(ctx, gameDir, &input.Issues)
	input.ReadModConfig = func(name string) ([]byte, error) {
		return readSeedMapModConfig(ctx, gameDir, world, name)
	}
	statRoots := []string{filepath.Join(gameDir, "config"), filepath.Join(gameDir, "defaultconfigs"), filepath.Join(r.Path, "versions", r.SelectedVersion)}
	if world != "" {
		statRoots = append(statRoots, filepath.Join(gameDir, "saves", world, "serverconfig"))
	}
	pre("")
	fpStage := seedMapStage("open.fingerprint")
	key, keyErr := worldgenOpenFingerprint(ctx, input, statRoots)
	fpStage("")
	if keyErr == nil {
		if cached := lookupOpenCache(ctx, input.CacheDir, key); cached != nil {
			return cached, level, nil
		}
	} else if ctx.Err() != nil {
		return nil, level, ctx.Err()
	}
	useJarFacts(input.CacheDir)
	snap, err := WorldgenIngest(seedMapEarlyFor(ctx, level), input)
	saveJarFacts()
	if err == nil && keyErr == nil {
		storeOpenCache(input.CacheDir, key, snap)
	}
	return snap, level, err
}

// worldgenModConfigLimit bounds one mod TOML read for registration. Large
// modpack configs (Quark's quark-common.toml is ~80 KiB) must still load.
const worldgenModConfigLimit = 512 << 10

func readSeedMapModConfig(ctx context.Context, gameDir, world, name string) ([]byte, error) {
	paths := []string{"config/" + name}
	if file, server := strings.CutPrefix(name, "serverconfig/"); server {
		paths = []string{"defaultconfigs/" + file}
		if world != "" {
			if err := validWorldDir(world); err != nil {
				return nil, err
			}
			paths = append([]string{"saves/" + world + "/serverconfig/" + file}, paths...)
		}
	}
	for _, relative := range paths {
		p, err := safeJoin(gameDir, relative)
		if err != nil {
			return nil, err
		}
		if info, statErr := os.Stat(p); statErr == nil && info.IsDir() {
			// A directory is not a config file; callers probe paths freely.
			continue
		}
		data, err := readWorldgenFile(ctx, p, worldgenModConfigLimit)
		if os.IsNotExist(err) {
			continue
		}
		return data, err
	}
	return nil, nil
}

func seedMapPackEntries(ctx context.Context, root, sub string) ([]os.DirEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p, err := safeJoin(root, sub)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(513)
	if len(entries) > 512 {
		return nil, errors.New("too many worldgen packs")
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}

// WorldgenIngest reads data/*/worldgen/*.json and dimension definitions. The
// hash includes version, effective content, order/provenance and diagnostics,
// never mtimes. Invalid overrides suppress older definitions of the same ID.
func WorldgenIngest(ctx context.Context, input WorldgenInput) (*WorldgenSnapshot, error) {
	if _, err := modjar.SelectLoader(input.Loader, input.Minecraft, input.LoaderVersion); err != nil {
		return nil, err
	}
	if len(input.Sources) > 1024 || len(input.Libraries) > 1024 {
		return nil, errors.New("too many worldgen sources")
	}
	snap := &WorldgenSnapshot{Schema: worldgenSchema, Minecraft: input.Minecraft, HasMods: input.HasMods, SavedWorld: input.SavedWorld, Generators: input.Generators, SavedDimensions: input.SavedDimensions}
	snap.Loader = input.Loader
	snap.LoaderVersion = input.LoaderVersion
	snap.JavaMajor = input.JavaMajor
	snap.WorldName = input.WorldName
	if len(input.ModConfigs) != 0 {
		if len(input.ModConfigs) > 128 {
			return nil, errors.New("too many worldgen configurations")
		}
		snap.ModConfigs = make(map[string][]byte, len(input.ModConfigs))
		for name, data := range input.ModConfigs {
			if data == nil || len(data) > worldgenModConfigLimit {
				snap.ModConfigs[name] = nil
			} else {
				snap.ModConfigs[name] = append([]byte{}, data...)
			}
		}
	}
	if input.EnabledPacks != nil {
		snap.EnabledPacks = append([]string{}, input.EnabledPacks...)
	}
	for _, issue := range input.Issues {
		snap.issue(issue.Code, issue.Source)
	}
	collector := worldgenCollector{ctx: ctx, snapshot: snap, documents: map[string]WorldgenDocument{}}
	collector.readConfig = input.ReadModConfig
	collector.libraries = input.Libraries
	collector.symbols = input.Symbols
	collector.classPathErr = input.classPathErr
	collector.format = input.PackFormat
	if collector.format == nil {
		collector.format = worldgenReleasePackFormat(input.Minecraft)
	}
	snap.PackFormat = collector.format
	builtins, err := cubiomes.BuiltinDocuments(input.Minecraft)
	if err != nil {
		return nil, fmt.Errorf("load builtin climate data: %w", err)
	}
	for _, doc := range builtins {
		collector.documents[doc.Kind+"/"+doc.ID] = doc
	}
	for _, doc := range vanillaSeedMapDocuments(input.Minecraft) {
		if _, exists := collector.documents[doc.Kind+"/"+doc.ID]; !exists {
			collector.documents[doc.Kind+"/"+doc.ID] = doc
		}
	}
	if len(collector.documents) == 0 {
		snap.issue("version_unverified", input.Minecraft)
	}
	seedMapProgress(ctx, "sources", "")
	stage := seedMapStage("ingest.sources")
	prefetch := startWorldgenPrefetch(ctx, input.Sources)
	defer prefetch.finish()
	collector.prefetch = prefetch
	for i, source := range input.Sources {
		pf := prefetch.take(i)
		if err := ctx.Err(); err != nil {
			pf.close()
			return nil, err
		}
		if strings.HasPrefix(source.Name, "kubejs:") && !worldgenHasMod(snap, "kubejs") {
			pf.close()
			continue
		}
		if strings.HasPrefix(source.Name, "globalpacks:") && !worldgenHasMod(snap, "globalpacks") || strings.HasPrefix(source.Name, "paxi:") && !worldgenHasMod(snap, "paxi") {
			pf.close()
			continue
		}
		if strings.HasPrefix(source.Name, tectonicEmbeddedSource) && !worldgenHasMod(snap, "tectonic") {
			pf.close()
			continue
		}
		if err := collector.readSourcePrefetched(source, pf); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			snap.issue("source_unreadable", source.Name)
		}
	}
	// Fabric lists mod data packs by ID, while global loaders use basenames.
	// Resolved IDs are not unknown-pack diagnostics.
	resolved := map[string]bool{"fabric": snap.Loader == "fabric"}
	for _, mod := range snap.Mods {
		resolved[mod.ID] = true
	}
	for _, source := range input.Sources {
		if (strings.HasPrefix(source.Name, "globalpacks:") && worldgenHasMod(snap, "globalpacks")) || (strings.HasPrefix(source.Name, "paxi:") && worldgenHasMod(snap, "paxi")) {
			resolved[filepath.Base(source.Path)] = true
		}
	}
	for _, id := range input.EnabledPacks {
		if id != "vanilla" && !strings.HasPrefix(id, "mod:") && !strings.HasPrefix(id, "file/") && !resolved[id] {
			snap.issue("pack_order", id)
		}
	}
	keptIssues := snap.Issues[:0]
	for _, issue := range snap.Issues {
		if issue.Code != "pack_order" || !resolved[issue.Source] {
			keptIssues = append(keptIssues, issue)
		}
	}
	snap.Issues = keptIssues
	rewriteTectonicDocuments(collector.documents, parseTectonicConfig(snap.ModConfigs["tectonic.json"]))
	if worldgenHasMod(snap, "structurify") && collector.readConfig != nil {
		if raw, err := collector.readConfig("structurify.json"); err == nil {
			rewriteStructurifyDocuments(collector.documents, raw)
		}
	}
	injectSupplementariesDynamicTags(collector.documents, snap)
	keys := make([]string, 0, len(collector.documents))
	for key := range collector.documents {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		snap.Documents = append(snap.Documents, collector.documents[key])
	}
	if worldgenHasMod(snap, "terrablender") || len(snap.Documents) != 0 {
		emitSeedMapEarly(ctx, snap)
	}
	stage("ingest.regions")
	seedMapProgress(ctx, "regions", "")
	if err := collector.readRegionPrograms(input.Sources); err != nil {
		return nil, err
	}
	stage("ingest.biomes")
	seedMapProgress(ctx, "biomes", "")
	if err := collector.readBiomePrograms(input.Sources); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	stage("ingest.hash")
	defer stage("")
	b, err := json.Marshal(snap)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(b)
	snap.Hash = hex.EncodeToString(hash[:])
	if input.CacheDir != "" {
		p, err := safeJoin(input.CacheDir, snap.Hash+".json")
		if err != nil {
			snap.issue("cache_unavailable", "")
			return snap, nil
		}
		if cached, e := readWorldgenCache(ctx, p, snap.Hash); e == nil {
			return cached, nil
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// Keep snapshots compact: indentation can amplify deeply nested raw JSON.
		cachedBytes, e := json.Marshal(snap)
		if e == nil {
			e = atomicBytes(p, cachedBytes, 0600)
		}
		if e != nil {
			snap.issue("cache_unavailable", "")
		} else {
			trimWorldgenCache(input.CacheDir, p)
		}
	}
	return snap, nil
}

func readWorldgenCache(ctx context.Context, file, hash string) (*WorldgenSnapshot, error) {
	b, err := readWorldgenFile(ctx, file, 256<<20)
	if err != nil {
		return nil, err
	}
	var snap WorldgenSnapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return nil, err
	}
	if snap.Schema != worldgenSchema || snap.Hash != hash {
		return nil, errors.New("worldgen cache version mismatch")
	}
	snap.Hash = ""
	canonical, err := json.Marshal(snap)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(canonical)
	if hex.EncodeToString(digest[:]) != hash {
		return nil, errors.New("worldgen cache content mismatch")
	}
	snap.Hash = hash
	return &snap, nil
}

// Only our content-addressed JSON files are eligible for eviction (four on disk).
func trimWorldgenCache(dir, current string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var files []os.FileInfo
	for _, en := range entries {
		name := en.Name()
		if en.IsDir() || len(name) != 69 || !strings.HasSuffix(name, ".json") {
			continue
		}
		if _, err := hex.DecodeString(name[:64]); err != nil {
			continue
		}
		if info, err := en.Info(); err == nil {
			files = append(files, info)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].ModTime().After(files[j].ModTime()) })
	kept := 1
	for _, file := range files {
		p := filepath.Join(dir, file.Name())
		if p == current {
			continue
		}
		if kept < 4 {
			kept++
			continue
		}
		// Cache eviction is best effort; an unavailable cache cannot block a map.
		_ = os.Remove(p)
	}
}

// A deliberately small, version-gated fallback: placement candidates, never
// synthetic vanilla biomes. Installed vanilla JSON overrides these entries.
func vanillaSeedMapDocuments(version string) []WorldgenDocument {
	parts := strings.Split(version, ".")
	if len(parts) < 2 || len(parts) > 3 || parts[0] != "1" || (parts[1] != "19" && parts[1] != "20" && parts[1] != "21") {
		return nil
	}
	for _, part := range parts {
		if part == "" {
			return nil
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return nil
			}
		}
	}
	if len(parts) == 3 {
		patch, err := strconv.Atoi(parts[2])
		if err != nil || patch > map[string]int{"19": 4, "20": 6, "21": 11}[parts[1]] {
			return nil
		}
	}
	var out []WorldgenDocument
	for _, v := range []struct {
		id                        string
		spacing, separation, salt int
		spread                    string
	}{
		{"villages", 34, 8, 10387312, "linear"},
		{"desert_pyramids", 32, 8, 14357617, "linear"},
		{"jungle_temples", 32, 8, 14357619, "linear"},
		{"swamp_huts", 32, 8, 14357620, "linear"},
		{"ocean_monuments", 32, 5, 10387313, "triangular"},
		{"woodland_mansions", 80, 20, 10387319, "triangular"},
	} {
		data := fmt.Sprintf(`{"structures":[{"structure":"minecraft:%s","weight":1}],"placement":{"type":"minecraft:random_spread","spacing":%d,"separation":%d,"salt":%d,"spread_type":"%s"}}`, v.id, v.spacing, v.separation, v.salt, v.spread)
		out = append(out, WorldgenDocument{ID: "minecraft:" + v.id, Kind: "structure_set", Source: "fallback:" + version, Data: json.RawMessage(data)})
	}
	return out
}

// vanillaClientJar finds the game jar that carries vanilla data: the release
// folder first, then the selected version folder and its "jar"/inheritsFrom
// chain, since launchers such as PCL copy the client jar under the instance
// name. A candidate counts only when it contains vanilla worldgen data.
func vanillaClientJar(root, minecraft, selected string) string {
	seen := map[string]bool{}
	queue := []string{minecraft, selected}
	for len(queue) > 0 && len(seen) < 8 {
		id := queue[0]
		queue = queue[1:]
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if p, e := safeJoin(root, "versions/"+id+"/"+id+".jar"); e == nil && jarHasVanillaWorldgen(p) {
			return p
		}
		p, e := safeJoin(root, "versions/"+id+"/"+id+".json")
		if e != nil {
			continue
		}
		data, e := os.ReadFile(p)
		if e != nil || len(data) > 4<<20 {
			continue
		}
		var meta struct{ Jar, InheritsFrom string }
		if json.Unmarshal(data, &meta) == nil {
			queue = append(queue, meta.Jar, meta.InheritsFrom)
		}
	}
	return ""
}

func jarHasVanillaWorldgen(p string) bool {
	z, err := zip.OpenReader(p)
	if err != nil {
		return false
	}
	defer z.Close()
	for _, f := range z.File {
		if f.Name == "data/minecraft/worldgen/structure_set/strongholds.json" || f.Name == "data/minecraft/worldgen/structure_set/villages.json" {
			return true
		}
	}
	return false
}
