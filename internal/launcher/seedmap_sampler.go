package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"hash/fnv"
	"image"
	"image/color"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/Viking602/YoloMCL/internal/worldgen"

	"github.com/Viking602/YoloMCL/cubiomes"
	"github.com/Viking602/YoloMCL/cubiomes/modjar"
)

// FeatureHit is a seed-derived placement candidate, not a generated building.
// An empty Dimension means that JSON biome/dimension eligibility is unresolved.
type FeatureHit struct {
	ID, Kind, Dimension, Source string
	X, Z                        int
	Confidence                  string
	Biome                       string
	Y                           int
}

// SeedMapCoverage explains exactly which parts of a dimension are sampled.
type SeedMapCoverage struct {
	Terrain                 string // fixed, checkerboard, multi_noise, the_end, vanilla_layered, or candidates
	Opaque                  bool
	Fallback                bool
	JSONEntries, Placements int
	Issues                  []WorldgenIssue
}

// SeedMapBiome is one categorical map color, not a rendered terrain texture.
type SeedMapBiome struct {
	ID    string
	Color color.RGBA
}

type seedMapDimension struct {
	generator, mode, biome string
	biomes                 []string
	scale                  int
	opaque                 bool
	settings, source       json.RawMessage
	dimensionType          string
	engine                 *cubiomes.Generator
	pool                   *sync.Pool
	terrain                *cubiomes.Terrain // final_density program; nil when unsupported
	terrainPool            *sync.Pool
	surface                *cubiomes.SurfaceRules // traced mod surface rules; nil when none
	issues                 []WorldgenIssue
	palette                []SeedMapBiome
	seaLevel               int
	carverFluid            CarverFluid // nether_cave carver output (seedmap_carver_fluid.go)
}

type seedMapPlacement struct {
	id, source, dimension     string
	spacing, separation, salt int
	triangular                bool
	frequency                 float64 // 0 or 1 = every placement chunk
	frequencyMethod           string
	excludes                  []seedMapExclusion // exclusion_zone and mod exclusion zones
	minDistance               int                // blocks from origin; 0 = none
	avoidWater                bool               // Better Temples river/ocean scan
	waterBiomes               map[string]bool
	structures                []string
	allowed                   map[string]bool
	biomesKnown               bool
	dimensions                map[string]bool
	rings                     *seedMapRings // concentric_rings placements only
	landmarkGrid              bool          // twilightforest:landmark_grid
	gridLock                  string        // structure_grid_lock; empty = unlocked
	esLandmark                bool          // eternal_starlight:landmark
	esLock                    string        // landmark ResourceKey id
	esAvoidLandmark           bool          // eternal_starlight:avoid_landmark
	tfAvoidLandmark           bool          // twilightforest:avoid_landmark_grid
	jigsaw                    *seedMapJigsawStart
	probes                    []seedMapProbe // per structure, in set order
}

// LocalSampler compiles immutable definitions into native Go tile sampling.
// It never reads disk, starts a process or makes network calls. Unsupported
// mod logic remains an explicit coverage gap.
type LocalSampler struct {
	seed       int64
	dimensions map[string]seedMapDimension
	colors     map[string]color.RGBA
	placements []seedMapPlacement
	coverage   SeedMapCoverage
	// jigsawPools is set once before the sampler is shared.
	jigsawPools map[string][]JigsawStartPiece
}

// NewLocalSampler compiles a bounded snapshot once per seed-map session.
func NewLocalSampler(ctx context.Context, snapshot *WorldgenSnapshot, seed int64) (*LocalSampler, error) {
	s := &LocalSampler{seed: seed, dimensions: map[string]seedMapDimension{}, colors: map[string]color.RGBA{}}
	if snapshot == nil {
		return nil, errors.New("missing worldgen snapshot")
	}
	s.coverage.Issues = append([]WorldgenIssue(nil), snapshot.Issues...)
	docs := make(map[string]WorldgenDocument, len(snapshot.Documents))
	for _, doc := range snapshot.Documents {
		docs[doc.Kind+"/"+doc.ID] = doc
	}
	// Normal is the manual-preview default. A save's dimension map replaces it
	// below; explicit dimension registry entries override it for manual maps.
	if doc, ok := docs["world_preset/minecraft:normal"]; ok {
		var preset struct {
			Dimensions map[string]json.RawMessage `json:"dimensions"`
		}
		if json.Unmarshal(doc.Data, &preset) == nil && len(preset.Dimensions) <= 256 {
			for id, raw := range preset.Dimensions {
				s.dimensions[id] = parseSeedMapDimension(raw)
			}
		}
	}
	for _, doc := range snapshot.Documents {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !strings.HasPrefix(doc.Source, "fallback:") {
			s.coverage.JSONEntries++
		}
		switch doc.Kind {
		case "dimension":
			d := parseSeedMapDimension(doc.Data)
			s.dimensions[doc.ID] = d
			if d.opaque && !seedMapKnownGenerator(d.generator) {
				s.addIssue("opaque_generator", doc.ID+" / "+d.generator)
			}
		case "biome":
			var data struct {
				Effects struct {
					GrassColor *uint32 `json:"grass_color"`
				} `json:"effects"`
			}
			if json.Unmarshal(doc.Data, &data) == nil && data.Effects.GrassColor != nil && !seedMapPaletteOverGrass[doc.ID] {
				n := *data.Effects.GrassColor
				s.colors[doc.ID] = color.RGBA{byte(n >> 16), byte(n >> 8), byte(n), 255}
			}
		case "structure_set":
			p, ok := parseSeedMapPlacement(doc, snapshot.ModConfigs)
			if ok && len(s.placements) < 512 {
				s.placements = append(s.placements, p)
			} else {
				s.addIssue("placement_unsupported", doc.ID)
			}
		}
	}
	s.linkPlacementExclusions(ctx, docs)
	if snapshot.SavedWorld {
		s.dimensions = map[string]seedMapDimension{}
		for id, raw := range snapshot.SavedDimensions {
			s.dimensions[id] = parseSeedMapDimension(raw)
		}
	}
	for dimension, generator := range snapshot.Generators {
		d := s.dimensions[dimension]
		d.generator = generator
		d.opaque = d.opaque || !seedMapKnownGenerator(generator)
		if _, ok := snapshot.SavedDimensions[dimension]; !ok {
			d.mode = ""
		}
		s.dimensions[dimension] = d
		if d.opaque {
			var st struct {
				Type string `json:"type"`
			}
			_ = json.Unmarshal(d.source, &st)
			s.addIssue("opaque_generator", dimension+" / "+generator+" biome_source="+st.Type)
		}
	}
	if snapshot.SavedWorld && len(snapshot.SavedDimensions) == 0 {
		s.addIssue("saved_settings", "level.dat")
	}
	// Bound compilation separately from ingest. Sort so the same definitions
	// are retained regardless of Go map iteration order.
	dimensionIDs := s.Dimensions()
	rank := func(id string) int {
		switch id {
		case "minecraft:overworld":
			return 0
		case "minecraft:the_nether":
			return 1
		case "minecraft:the_end":
			return 2
		}
		return 3
	}
	sort.SliceStable(dimensionIDs, func(i, j int) bool { return rank(dimensionIDs[i]) < rank(dimensionIDs[j]) })
	const maxDimensions = 128
	for _, id := range dimensionIDs[min(maxDimensions, len(dimensionIDs)):] {
		delete(s.dimensions, id)
		s.addIssue("dimension_limit", id)
	}
	compiledNodes := 0
	for _, id := range dimensionIDs[:min(maxDimensions, len(dimensionIDs))] {
		d := s.dimensions[id]
		blueprintWrap := false
		if original, ok := parseBlueprintModded(d.source); ok {
			// Evaluate the wrapped original (usually multi_noise + TerraBlender),
			// then re-apply Blueprint's weighted slice selection on top.
			d.source = original
			d.opaque = false
			var sourceType struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(original, &sourceType) == nil {
				switch seedMapID(sourceType.Type) {
				case "minecraft:multi_noise":
					d.mode = "multi_noise"
				case "minecraft:the_end":
					d.mode = "the_end"
				default:
					d.opaque = true
					d.mode = ""
				}
			}
			blueprintWrap = d.mode != ""
		}
		var custom *modjar.BiomeProgram
		if d.generator == "minecraft:noise" && d.opaque {
			var source struct{ Type string }
			_ = json.Unmarshal(d.source, &source)
			for _, program := range snapshot.BiomePrograms {
				if program.ID == seedMapID(source.Type) {
					if program.Program != nil {
						custom = program.Program
						d.mode = "custom"
						d.opaque = false
					} else if program.Error != "" {
						d.issues = append(d.issues, WorldgenIssue{"biome_source_unsupported", id + " / " + program.Error})
					}
					break
				}
			}
		}
		if d.mode == "multi_noise" || d.mode == "the_end" || d.mode == "vanilla_layered" || custom != nil {
			if compiledNodes >= 32768 {
				d.mode = ""
				d.issues = append(d.issues, WorldgenIssue{"dimension_limit", id})
				s.dimensions[id] = d
				continue
			}
			definition := cubiomes.Definition{
				Version: snapshot.Minecraft, Seed: seed, Settings: d.settings, BiomeSource: d.source,
			}
			var engine *cubiomes.Generator
			var err error
			if custom != nil {
				engine, err = cubiomes.CompileCustomBiomeSource(ctx, docs, definition, *custom)
			} else {
				engine, err = cubiomes.Compile(ctx, docs, definition)
			}
			if err == nil {
				engine, err = applyNativeWorldgen(ctx, snapshot, id, d, seed, engine)
			}
			if err == nil && blueprintWrap {
				engine, err = applyBlueprintModded(ctx, snapshot, id, seed, engine)
			}
			if err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				code := "climate_invalid"
				var unsupported *cubiomes.CompileError
				if errors.As(err, &unsupported) {
					code, d.opaque = unsupported.Code, true
				}
				d.mode = ""
				d.issues = append(d.issues, WorldgenIssue{code, id + " / " + err.Error()})
			} else {
				d.engine, d.biomes = engine, engine.Biomes()
				if d.mode != "custom" && (id == "minecraft:overworld" || id == "minecraft:the_nether") {
					for _, p := range skippedRegionPrograms(snapshot) {
						d.issues = append(d.issues, WorldgenIssue{"region_skipped", id + " / " + p.ModID + ": " + p.Error})
					}
				}
				d.seaLevel = seedMapSeaLevel(d.settings, docs)
				d.pool = &sync.Pool{New: func() any { return engine.NewSampler() }}
				compiledNodes += engine.NodeCount()
			}
			s.dimensions[id] = d
		}
		if d.generator == "minecraft:noise" && len(d.settings) > 0 {
			def := cubiomes.Definition{Version: snapshot.Minecraft, Seed: seed, Settings: d.settings, BiomeSource: d.source}
			s.compileTerrain(ctx, docs, def, &d, id)
			s.compileSurfaceRules(ctx, snapshot, docs, def, &d, id)
			if id == "minecraft:the_nether" && d.terrain != nil {
				d.carverFluid = compileCarverFluid(docs, "minecraft:nether_cave", d.terrain.MinY)
			}
		}
		s.dimensions[id] = d
	}
	for i := range s.placements {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		s.compilePlacementBiomes(ctx, &s.placements[i], docs)
		if s.placements[i].rings != nil {
			s.compileRingBiomes(ctx, &s.placements[i], docs)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for dimension, d := range s.dimensions {
		for _, id := range append(append([]string(nil), d.biomes...), d.biome) {
			if id != "" {
				s.colors[id] = s.biomeColor(id)
			}
		}
		if d.engine != nil {
			names := d.engine.BiomePalette()
			d.palette = make([]SeedMapBiome, len(names))
			for id, name := range names {
				if name != "" {
					d.palette[id] = SeedMapBiome{name, s.biomeColor(name)}
				}
			}
			s.dimensions[dimension] = d
		}
	}
	if len(s.dimensions) == 0 {
		s.dimensions["minecraft:overworld"] = seedMapDimension{}
	}
	s.bindPlacementDimensions()
	s.coverage.Placements = len(s.placements)
	s.addIssue("candidate_only", "")
	return s, nil
}

func (s *LocalSampler) addIssue(code, source string) {
	if len(s.coverage.Issues) < 256 {
		s.coverage.Issues = append(s.coverage.Issues, WorldgenIssue{code, source})
	}
}

func seedMapKnownGenerator(id string) bool {
	return id == "minecraft:noise" || id == "minecraft:flat" || id == "minecraft:debug"
}

func parseSeedMapDimension(raw []byte) seedMapDimension {
	var data struct {
		Type      json.RawMessage `json:"type"`
		Generator struct {
			Type        string          `json:"type"`
			BiomeSource json.RawMessage `json:"biome_source"`
			Settings    json.RawMessage `json:"settings"`
		} `json:"generator"`
	}
	if json.Unmarshal(raw, &data) != nil {
		return seedMapDimension{}
	}
	var biomeSource struct {
		Type   string   `json:"type"`
		Biome  string   `json:"biome"`
		Biomes []string `json:"biomes"`
		Scale  *int     `json:"scale"`
	}
	g := data.Generator
	// Lithostitched's codec decodes to the delegate; runtime injectors are
	// applied separately after TerraBlender, for manual and saved-world inputs.
	for depth := 0; depth < 32; depth++ {
		var wrapper struct {
			Type     string
			Delegate json.RawMessage
		}
		if json.Unmarshal(g.BiomeSource, &wrapper) != nil || wrapper.Type != "lithostitched:injector" {
			break
		}
		g.BiomeSource = wrapper.Delegate
	}
	g.Type = seedMapID(g.Type)
	d := seedMapDimension{generator: g.Type, opaque: g.Type != "" && !seedMapKnownGenerator(g.Type), scale: 2, settings: g.Settings, source: g.BiomeSource}
	if json.Unmarshal(data.Type, &d.dimensionType) == nil && d.dimensionType != "" {
		d.dimensionType = seedMapID(d.dimensionType)
	}
	if d.opaque {
		return d
	}
	if g.Type == "minecraft:flat" {
		var settings struct {
			Biome string `json:"biome"`
		}
		if json.Unmarshal(g.Settings, &settings) == nil && settings.Biome != "" {
			d.mode, d.biome = "fixed", seedMapID(settings.Biome)
		}
		return d
	}
	if g.Type != "minecraft:noise" {
		return d
	}
	var sourceType struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(g.BiomeSource, &sourceType) == nil && seedMapID(sourceType.Type) == "minecraft:multi_noise" {
		d.mode = "multi_noise"
		return d
	}
	if json.Unmarshal(g.BiomeSource, &biomeSource) != nil {
		return d
	}
	switch seedMapID(biomeSource.Type) {
	case "minecraft:fixed":
		if biomeSource.Biome != "" {
			d.mode, d.biome = "fixed", seedMapID(biomeSource.Biome)
		}
	case "minecraft:checkerboard":
		if biomeSource.Scale != nil {
			d.scale = *biomeSource.Scale
		}
		if len(biomeSource.Biomes) > 0 && len(biomeSource.Biomes) <= 256 && d.scale >= 0 && d.scale <= 16 {
			d.mode, d.biomes = "checkerboard", biomeSource.Biomes
			for i := range d.biomes {
				d.biomes[i] = seedMapID(d.biomes[i])
			}
		}
	case "minecraft:the_end":
		d.mode = "the_end"
	case "minecraft:vanilla_layered":
		d.mode = "vanilla_layered"
	default:
		if biomeSource.Type != "" && biomeSource.Type != "minecraft:the_end" {
			d.opaque = true
		}
	}
	return d
}

func parseSeedMapPlacement(doc WorldgenDocument, configs map[string][]byte) (seedMapPlacement, bool) {
	p := seedMapPlacement{id: doc.ID, source: doc.Source}
	var data struct {
		Structures []struct {
			Structure string `json:"structure"`
			Weight    int    `json:"weight"`
		} `json:"structures"`
		Placement map[string]json.RawMessage `json:"placement"`
	}
	if json.Unmarshal(doc.Data, &data) != nil || len(data.Structures) == 0 || len(data.Structures) > 256 {
		return p, false
	}
	for _, structure := range data.Structures {
		if structure.Structure == "" || structure.Weight <= 0 {
			return p, false
		}
		p.structures = append(p.structures, seedMapID(structure.Structure))
	}
	var kind string
	if raw, ok := data.Placement["type"]; ok && json.Unmarshal(raw, &kind) == nil && isTFLandmarkGridType(kind) {
		lock, ok := parseTFLandmarkGrid(data.Placement)
		if !ok {
			return p, false
		}
		p.landmarkGrid = true
		p.gridLock = lock
		// spacing marks the TF 16-chunk region for search helpers; candidate is overridden.
		p.spacing = 16
		return p, true
	}
	if raw, ok := data.Placement["type"]; ok && json.Unmarshal(raw, &kind) == nil && isESLandmarkType(kind) {
		lock, ok := parseESLandmark(data.Placement)
		if !ok {
			return p, false
		}
		p.esLandmark = true
		p.esLock = lock
		p.spacing = esRegionChunks
		return p, true
	}
	if raw, ok := data.Placement["type"]; ok && json.Unmarshal(raw, &kind) == nil && kind == "minecraft:concentric_rings" {
		rings, ok := parseSeedMapRings(data.Placement)
		if !ok {
			return p, false
		}
		p.rings = rings
		if strings.HasPrefix(doc.Source, "fallback:") {
			p.dimension = "minecraft:overworld"
		}
		return p, true
	}
	var placement struct {
		Type, SpreadType    string
		Spacing, Separation *int
		Salt                *int64
	}
	extra := map[string]json.RawMessage{}
	for key, raw := range data.Placement {
		var err error
		switch key {
		case "type":
			err = json.Unmarshal(raw, &placement.Type)
		case "spread_type":
			err = json.Unmarshal(raw, &placement.SpreadType)
		case "spacing":
			err = json.Unmarshal(raw, &placement.Spacing)
		case "separation":
			err = json.Unmarshal(raw, &placement.Separation)
		case "salt":
			err = json.Unmarshal(raw, &placement.Salt)
		case "frequency":
			var frequency float64
			err = json.Unmarshal(raw, &frequency)
			if frequency <= 0 || frequency > 1 {
				return p, false
			}
			p.frequency = frequency
		case "frequency_reduction_method":
			var method string
			err = json.Unmarshal(raw, &method)
			if _, ok := worldgen.FrequencyPasses(method, 0, 0, 0, 0, 0.5); !ok {
				return p, false
			}
			p.frequencyMethod = method
		case "exclusion_zone":
			zone, ok := parseSeedMapExclusion(raw, 64)
			if !ok {
				return p, false
			}
			p.excludes = append(p.excludes, zone)
		case "locate_offset": // moves only the locate target, never the start chunk
			var offset []int
			err = json.Unmarshal(raw, &offset)
			if len(offset) != 3 {
				return p, false
			}
		case "min_distance_from_world_origin", "super_exclusion_zone", "enhanced_exclusion_zone", "avoid_additional_structures":
			extra[key] = raw
		case "config":
			extra[key] = raw // aquamirae:configurable_random_spread nesting
		default:
			return p, false // exclusions, custom codecs, unknown future rules
		}
		if err != nil {
			return p, false
		}
	}
	family, known := seedMapSpreadFamilies[seedMapID(placement.Type)]
	if !known || placement.Salt == nil {
		return p, false
	}
	if isESAvoidLandmarkType(placement.Type) {
		p.esAvoidLandmark = true
	}
	if isTFAvoidLandmarkGridType(placement.Type) {
		p.tfAvoidLandmark = true
	}
	if seedMapID(placement.Type) == aquamiraeConfigurableSpread {
		raw, ok := extra["config"]
		if !ok {
			return p, false
		}
		sp, se, tri, ok := aquamiraeParseConfig(raw, configs)
		if !ok {
			return p, false
		}
		delete(extra, "config")
		placement.Spacing, placement.Separation = &sp, &se
		if tri {
			placement.SpreadType = "triangular"
		}
	}
	if placement.Spacing == nil || placement.Separation == nil {
		return p, false
	}
	for key, raw := range extra {
		switch {
		case key == "min_distance_from_world_origin":
			if json.Unmarshal(raw, &p.minDistance) != nil || p.minDistance < 0 {
				return p, false
			}
			if !family.minDistance {
				p.minDistance = 0 // codec field the class never reads
			}
		case key == family.zoneKey:
			zones, ok := parseSeedMapExclusionZones(raw, 64)
			if !ok {
				return p, false
			}
			p.excludes = append(p.excludes, zones...)
		case key == "avoid_additional_structures":
			var m map[string]int
			if json.Unmarshal(raw, &m) != nil || len(m) == 0 || len(m) > 64 {
				return p, false
			}
			if !p.tfAvoidLandmark {
				return p, false // codec field only AvoidLandmarkGridPlacement reads
			}
			for id, count := range m {
				if id == "" || count < 1 || count > 64 {
					return p, false
				}
				rawID, err := json.Marshal(seedMapID(id))
				if err != nil {
					return p, false
				}
				p.excludes = append(p.excludes, seedMapExclusion{raw: rawID, chunks: count})
			}
		default:
			return p, false
		}
	}
	p.avoidWater = family.avoidWater
	p.spacing, p.separation, p.salt = *placement.Spacing, *placement.Separation, javaIntValue(*placement.Salt)
	if p.spacing < 1 || p.spacing > 4096 || p.separation < 0 || p.salt < 0 {
		return p, false
	}
	if family.scale != 0 {
		if p.separation >= p.spacing {
			return p, false // the constructor throws
		}
		p.spacing, p.separation = int(math.Floor(float64(p.spacing)*family.scale+0.5)), int(math.Floor(float64(p.separation)*family.scale+0.5))
	}
	if p.separation >= p.spacing && !(family.minDistance && p.spacing == 1 && p.separation == 0) {
		return p, false
	}
	if placement.SpreadType != "" && placement.SpreadType != "linear" && placement.SpreadType != "triangular" {
		return p, false
	}
	p.triangular = placement.SpreadType == "triangular"
	// Historical fallback data predates structure/biome-tag registries. Modern
	// vanilla and mod placements derive eligibility from their effective data.
	if strings.HasPrefix(doc.Source, "fallback:") {
		switch doc.ID {
		case "minecraft:villages", "minecraft:desert_pyramids", "minecraft:jungle_temples", "minecraft:swamp_huts", "minecraft:ocean_monuments", "minecraft:woodland_mansions", "minecraft:igloos", "minecraft:shipwrecks", "minecraft:ocean_ruins", "minecraft:ancient_cities", "minecraft:trail_ruins", "minecraft:trial_chambers", "minecraft:pillager_outposts", "minecraft:buried_treasures":
			p.dimension = "minecraft:overworld"
		case "minecraft:nether_complexes", "minecraft:nether_fossils":
			p.dimension = "minecraft:the_nether"
		case "minecraft:end_cities":
			p.dimension = "minecraft:the_end"
		}
	}
	return p, true
}

// Dimensions returns sorted known dimension IDs, always including Overworld.
func (s *LocalSampler) Dimensions() []string {
	ids := make([]string, 0, len(s.dimensions))
	for id := range s.dimensions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Coverage returns honest terrain and placement support for a dimension.
func (s *LocalSampler) Coverage(dimension string) SeedMapCoverage {
	c := s.coverage
	d := s.dimensions[dimension]
	c.Issues = append(append([]WorldgenIssue(nil), c.Issues...), d.issues...)
	c.Terrain, c.Opaque = d.mode, d.opaque
	if c.Terrain == "" {
		c.Terrain = "candidates"
	}
	c.Placements = 0
	for _, placement := range s.placements {
		if placement.inDimension(dimension) {
			c.Placements++
			c.Fallback = c.Fallback || strings.HasPrefix(placement.source, "fallback:")
		}
	}
	return c
}

// Biomes lists the categorical legend for supported JSON biome sources.
func (s *LocalSampler) Biomes(dimension string) []SeedMapBiome {
	d := s.dimensions[dimension]
	ids := d.biomes
	if d.mode == "fixed" {
		ids = []string{d.biome}
	}
	var out []SeedMapBiome
	seen := map[string]bool{}
	for _, id := range ids {
		if !seen[id] {
			out = append(out, SeedMapBiome{id, s.biomeColor(id)})
			seen[id] = true
		}
	}
	return out
}

func (s *LocalSampler) biomeColor(id string) color.RGBA {
	if c, ok := s.colors[id]; ok {
		return c
	}
	if c, ok := seedMapVanillaColors[id]; ok {
		return c
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(id)) // hash.Write never returns an error
	n := h.Sum32()
	return color.RGBA{byte(70 + n%130), byte(85 + (n>>8)%130), byte(75 + (n>>16)%125), 255}
}

func seedMapFloor(n, d int) int {
	q := n / d
	if n%d < 0 {
		q--
	}
	return q
}

// SeedMapTilePixels is the fixed CPU raster size of each tile.
const SeedMapTilePixels = 128

// SeedMapTileKey identifies a tile in one sampler's seed and snapshot.
type SeedMapTileKey struct {
	X, Z, BlocksPerPixel int
	Y                    int
	Dimension            string
	Structure            string
	Surface              bool
	ContourOverlay       bool // draw fine contours at display resolution
	Terrain              bool // world-map surface colours instead of biome colours
}

// SeedMapTile is immutable once published to the worker's LRU cache.
type SeedMapTile struct {
	Key     SeedMapTileKey
	Image   *image.RGBA
	Hits    []FeatureHit
	Limited bool
	Biomes  []SeedMapBiome
	ids     []cubiomes.BiomeID
	surface []cubiomes.SurfaceSample
	lava    []bool // final_density relief: the ground is a lava sea
	palette []SeedMapBiome
}

// Sample evaluates one tile, checking cancellation at every raster/region row.
func (s *LocalSampler) Sample(ctx context.Context, key SeedMapTileKey) (*SeedMapTile, error) {
	if key.BlocksPerPixel < 1 || key.BlocksPerPixel > 32 || key.X < -250000 || key.X > 250000 || key.Z < -250000 || key.Z > 250000 || key.Y < -2048 || key.Y > 2047 {
		return nil, errors.New("invalid seed map tile")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tile := &SeedMapTile{Key: key, Image: image.NewRGBA(image.Rect(0, 0, SeedMapTilePixels, SeedMapTilePixels))}
	d := s.dimensions[key.Dimension]
	span := SeedMapTilePixels * key.BlocksPerPixel
	baseX, baseZ := key.X*span, key.Z*span
	scratch := d.scratch()
	// Large legacy raster workspaces are intentionally not retained while idle.
	if d.mode != "vanilla_layered" || key.BlocksPerPixel <= 8 {
		defer d.releaseScratch(scratch)
	}
	seenBiomes := map[string]bool{}
	var seenIDs [1024]uint64 // all uint16 IDs, with no per-pixel name hashing
	var biomeIDs []cubiomes.BiomeID
	if d.engine != nil {
		tile.palette = d.palette
		if key.Surface && d.engine.SupportsSurface() {
			var err error
			tile.surface, err = seedMapSurface(ctx, scratch, key, baseX, baseZ, d.seaLevel)
			if err != nil {
				return nil, err
			}
		} else {
			// Close views use the game's block-biome zoom. Overview levels keep
			// quart sampling; no fine pixel is filled by stretching a quart cell.
			grid := cubiomes.Range{X: baseX, Y: key.Y, Z: baseZ, SizeX: SeedMapTilePixels, SizeY: 1, SizeZ: SeedMapTilePixels, StepXZ: key.BlocksPerPixel, StepY: 1, Voronoi: key.BlocksPerPixel < 4}
			biomeIDs = make([]cubiomes.BiomeID, SeedMapTilePixels*SeedMapTilePixels)
			if err := scratch.GenerateIDs(ctx, grid, biomeIDs); err != nil {
				return nil, err
			}
			tile.ids = biomeIDs
			if key.Surface && d.noiseSurface() {
				var err error
				if tile.surface, tile.lava, err = s.seedMapNoiseSurface(ctx, d, key, baseX, baseZ, biomeIDs); err != nil {
					return nil, err
				}
			}
		}
	}
	for py := range SeedMapTilePixels {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		z := baseZ + py*key.BlocksPerPixel
		for px := range SeedMapTilePixels {
			if px%16 == 0 && ctx.Err() != nil {
				return nil, ctx.Err()
			}
			x := baseX + px*key.BlocksPerPixel
			c := color.RGBA{224, 231, 238, 255}
			var biome string
			var ok bool
			if d.engine != nil {
				var id cubiomes.BiomeID
				if tile.surface != nil {
					id = tile.surface[(py+1)*seedMapSurfaceWidth+px+1].Biome
				} else {
					id = biomeIDs[py*SeedMapTilePixels+px]
				}
				entry := d.palette[id]
				c, ok = entry.Color, entry.ID != ""
				if ok {
					mask := uint64(1) << (id & 63)
					if seenIDs[id>>6]&mask == 0 {
						seenIDs[id>>6] |= mask
						tile.Biomes = append(tile.Biomes, entry)
					}
				}
			} else {
				biome, ok = d.biomeAt(x, key.Y, z, scratch, true)
			}
			if ok && d.engine == nil {
				c = s.biomeColor(biome)
				if !seenBiomes[biome] {
					seenBiomes[biome] = true
					tile.Biomes = append(tile.Biomes, SeedMapBiome{biome, c})
				}
			} else if !ok {
				if (seedMapFloor(x, 128)+seedMapFloor(z, 128))&1 == 0 {
					c = color.RGBA{214, 222, 231, 255}
				}
			}
			if tile.lava != nil && ok {
				c = seedMapNoiseShade(c, d.palette[tile.surface[(py+1)*seedMapSurfaceWidth+px+1].Biome].ID, tile.surface, tile.lava, px+1, py+1, key.BlocksPerPixel, key.Terrain, !key.ContourOverlay)
			} else if tile.surface != nil && key.Terrain && ok && d.engine != nil {
				cell := tile.surface[(py+1)*seedMapSurfaceWidth+px+1]
				c = seedMapTerrainBase(d.palette[cell.Biome].ID, float64(cell.Height), d.seaLevel)
				c = seedMapTerrainShade(c, tile.surface, px+1, py+1, key.BlocksPerPixel, d.seaLevel)
			} else if tile.surface != nil {
				c = seedMapRelief(c, tile.surface, px+1, py+1, key.BlocksPerPixel, !key.ContourOverlay)
			}
			offset := py*tile.Image.Stride + px*4
			pixel := tile.Image.Pix[offset : offset+4]
			pixel[0], pixel[1], pixel[2], pixel[3] = c.R, c.G, c.B, c.A
		}
	}
	// Axes are an overlay and do not trigger extra climate evaluations.
	for pixel := range SeedMapTilePixels {
		if baseX <= 0 && baseX+span > 0 {
			tile.Image.SetRGBA(-baseX/key.BlocksPerPixel, pixel, color.RGBA{130, 147, 167, 255})
		}
		if baseZ <= 0 && baseZ+span > 0 {
			tile.Image.SetRGBA(pixel, -baseZ/key.BlocksPerPixel, color.RGBA{130, 147, 167, 255})
		}
	}
	candidateChecks := 0
	for _, p := range s.placements {
		// An overview needs biome-qualified positions. Raw placement grids are
		// still available when a user explicitly selects a structure type.
		if key.Structure == "" && (d.mode == "" || !p.biomesKnown) {
			continue
		}
		if key.Structure != "" && p.id != key.Structure {
			continue
		}
		if !p.inDimension(key.Dimension) {
			continue
		}
		if p.rings != nil {
			chunks, err := s.ringChunks(ctx, p, key.Dimension)
			if err != nil {
				return nil, err
			}
			for _, c := range chunks {
				x, z := int(c[0])*16, int(c[1])*16
				if x < baseX || x >= baseX+span || z < baseZ || z >= baseZ+span {
					continue
				}
				confidence, biome := "ring_candidate", ""
				if p.biomesKnown && d.mode != "" {
					if id, ok := d.biomeAt(x, key.Y, z, scratch, false); ok {
						if !p.allowed[id] {
							continue
						}
						confidence, biome = "biome_candidate", id
					}
				}
				if key.Structure == "" && confidence != "biome_candidate" {
					continue
				}
				if len(tile.Hits) == 256 {
					tile.Limited = true
					return tile, nil
				}
				tile.Hits = append(tile.Hits, FeatureHit{ID: p.id, Kind: "structure_set", Dimension: p.dimension, Source: p.source, X: x, Y: key.Y, Z: z, Confidence: confidence, Biome: biome})
			}
			continue
		}
		if p.landmarkGrid {
			// TF regions are offset by +8 chunks vs spacing-aligned 0..15 grids.
			const regionBlocks = 256
			for rz := seedMapFloor(baseZ+128, regionBlocks); rz <= seedMapFloor(baseZ+span-1+128, regionBlocks); rz++ {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				for rx := seedMapFloor(baseX+128, regionBlocks); rx <= seedMapFloor(baseX+span-1+128, regionBlocks); rx++ {
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
					x, z, ok := tfLandmarkCandidateBlock(s.seed, rx, rz, p.gridLock)
					if !ok {
						continue
					}
					if x < baseX || x >= baseX+span || z < baseZ || z >= baseZ+span {
						continue
					}
					candidateChecks++
					if candidateChecks > 4096 {
						tile.Limited = true
						return tile, nil
					}
					confidence, biome := "candidate", ""
					if p.biomesKnown && d.mode != "" {
						if id, ok, pass := s.biomeCheck(p, d, key.Dimension, x, key.Y, z, scratch); ok {
							if !pass {
								continue
							}
							confidence, biome = "biome_candidate", id
						}
					}
					if key.Structure == "" && confidence != "biome_candidate" {
						continue
					}
					if len(tile.Hits) == 256 {
						tile.Limited = true
						return tile, nil
					}
					tile.Hits = append(tile.Hits, FeatureHit{ID: p.id, Kind: "structure_set", Dimension: p.dimension, Source: p.source, X: x, Y: key.Y, Z: z, Confidence: confidence, Biome: biome})
				}
			}
			continue
		}
		if p.esLandmark {
			const regionBlocks = 512 // 32 chunks
			for rz := seedMapFloor(baseZ, regionBlocks); rz <= seedMapFloor(baseZ+span-1, regionBlocks); rz++ {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				for rx := seedMapFloor(baseX, regionBlocks); rx <= seedMapFloor(baseX+span-1, regionBlocks); rx++ {
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
					x, z, ok := esLandmarkCandidateBlock(s.seed, rx, rz, p.esLock)
					if !ok {
						continue
					}
					if x < baseX || x >= baseX+span || z < baseZ || z >= baseZ+span {
						continue
					}
					candidateChecks++
					if candidateChecks > 4096 {
						tile.Limited = true
						return tile, nil
					}
					confidence, biome := "candidate", ""
					if p.biomesKnown && d.mode != "" {
						if id, ok, pass := s.biomeCheck(p, d, key.Dimension, x, key.Y, z, scratch); ok {
							if !pass {
								continue
							}
							confidence, biome = "biome_candidate", id
						}
					}
					if key.Structure == "" && confidence != "biome_candidate" {
						continue
					}
					if len(tile.Hits) == 256 {
						tile.Limited = true
						return tile, nil
					}
					tile.Hits = append(tile.Hits, FeatureHit{ID: p.id, Kind: "structure_set", Dimension: p.dimension, Source: p.source, X: x, Y: key.Y, Z: z, Confidence: confidence, Biome: biome})
				}
			}
			continue
		}
		regionSpan := p.spacing * 16
		if p.frequency > 0 && p.frequency < 1 && (span/regionSpan+1)*(span/regionSpan+1) > 1<<16 {
			continue // dense low-frequency sets (mineshafts) only at closer zoom
		}
		for rz := seedMapFloor(baseZ, regionSpan); rz <= seedMapFloor(baseZ+span-1, regionSpan); rz++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			for rx := seedMapFloor(baseX, regionSpan); rx <= seedMapFloor(baseX+span-1, regionSpan); rx++ {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				x, z := p.candidate(s.seed, rx, rz)
				if !p.placementChunk(s.seed, x, z) {
					continue
				}
				if x < baseX || x >= baseX+span || z < baseZ || z >= baseZ+span {
					continue
				}
				candidateChecks++
				if candidateChecks > 4096 {
					tile.Limited = true
					return tile, nil
				}
				confidence, biome := "candidate", ""
				if p.biomesKnown && d.mode != "" {
					// A fresh search makes marker filtering independent of raster zoom.
					if id, ok, pass := s.biomeCheck(p, d, key.Dimension, x, key.Y, z, scratch); ok {
						if !pass {
							continue
						}
						confidence, biome = "biome_candidate", id
					}
				}
				if key.Structure == "" && confidence != "biome_candidate" {
					continue
				}
				if len(tile.Hits) == 256 {
					tile.Limited = true
					return tile, nil
				}
				tile.Hits = append(tile.Hits, FeatureHit{ID: p.id, Kind: "structure_set", Dimension: p.dimension, Source: p.source, X: x, Y: key.Y, Z: z, Confidence: confidence, Biome: biome})
			}
		}
	}
	return tile, nil
}

// BiomeAt returns the biome at the containing quart cell. Each call starts a
// fresh RTree search; tile jobs retain the previous leaf in raster order.
func (s *LocalSampler) BiomeAt(dimension string, x, y, z int) (string, bool) {
	biome, ok, _ := s.BiomeAtContext(context.Background(), dimension, x, y, z)
	return biome, ok
}

// BiomeAtContext supports cancellation before a native point query.
func (s *LocalSampler) BiomeAtContext(ctx context.Context, dimension string, x, y, z int) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	d := s.dimensions[dimension]
	scratch := d.scratch()
	defer d.releaseScratch(scratch)
	biome, ok := d.biomeAt(x, y, z, scratch, false)
	return biome, ok, nil
}

func (d seedMapDimension) scratch() *cubiomes.Sampler {
	if d.pool != nil {
		return d.pool.Get().(*cubiomes.Sampler)
	}
	if d.engine != nil {
		return d.engine.NewSampler()
	}
	return nil
}

func (d seedMapDimension) releaseScratch(s *cubiomes.Sampler) {
	if d.pool != nil && s != nil {
		s.Reset()
		d.pool.Put(s)
	}
}

func (d seedMapDimension) biomeAt(x, y, z int, scratch *cubiomes.Sampler, history bool) (string, bool) {
	switch d.mode {
	case "fixed":
		return d.biome, true
	case "checkerboard":
		index := (int64(x) >> uint(d.scale+4)) + (int64(z) >> uint(d.scale+4))
		index %= int64(len(d.biomes))
		if index < 0 {
			index += int64(len(d.biomes))
		}
		return d.biomes[index], true
	case "multi_noise", "the_end", "vanilla_layered", "custom":
		if history {
			return scratch.NextNoiseBiome(x, y, z)
		}
		return scratch.NoiseBiome(x, y, z)
	}
	return "", false
}

func (p seedMapPlacement) candidate(seed int64, regionX, regionZ int) (int, int) {
	if p.landmarkGrid {
		x, z, ok := tfLandmarkCandidateBlock(seed, regionX, regionZ, p.gridLock)
		if !ok {
			// Non-matching lock: return an off-grid sentinel so placementChunk rejects.
			return regionX * 16 * 16, regionZ * 16 * 16
		}
		return x, z
	}
	if p.esLandmark {
		x, z, ok := esLandmarkCandidateBlock(seed, regionX, regionZ, p.esLock)
		if !ok {
			return regionX * esRegionChunks * 16, regionZ * esRegionChunks * 16
		}
		return x, z
	}
	// JSON placement validation and bounded tile/search coordinates satisfy
	// the library's limits before this adapter is reached.
	x, z, _ := (cubiomes.RandomSpread{Spacing: p.spacing, Separation: p.separation, Salt: p.salt, Triangular: p.triangular}).Candidate(seed, regionX, regionZ)
	return x, z
}

// passes applies the placement frequency rule to the candidate's chunk.
func (p seedMapPlacement) passes(seed int64, x, z int) bool {
	if p.frequency <= 0 || p.frequency >= 1 {
		return true
	}
	ok, _ := worldgen.FrequencyPasses(p.frequencyMethod, seed, int32(p.salt), int32(x>>4), int32(z>>4), p.frequency)
	return ok
}

// placementChunk is StructurePlacement.isStructureChunk: the candidate,
// frequency and exclusion-zone rules, without biome checks.
func (p seedMapPlacement) placementChunk(seed int64, x, z int) bool {
	return p.placementChunkDepth(seed, x, z, 0)
}

func (p seedMapPlacement) placementChunkDepth(seed int64, x, z, depth int) bool {
	if p.landmarkGrid {
		return tfIsPlacementChunk(seed, x>>4, z>>4, p.gridLock)
	}
	if p.esLandmark {
		return esIsLandmarkPlacementChunk(seed, x>>4, z>>4, p.esLock)
	}
	if p.minDistance > 0 && int64(x&^15)*int64(x&^15)+int64(z&^15)*int64(z&^15) < int64(p.minDistance)*int64(p.minDistance) {
		return false
	}
	if !p.passes(seed, x, z) {
		return false
	}
	if p.esAvoidLandmark && esIsLandmarkCenter(seed, x>>4, z>>4) {
		return false
	}
	if p.tfAvoidLandmark && tfTooCloseToLandmark(x>>4, z>>4) {
		return false
	}
	if depth > 6 {
		return true
	}
	for _, ex := range p.excludes {
		tooClose := false
		for _, other := range ex.others {
			if other.chunkInRangeDepth(seed, x>>4, z>>4, ex.chunks, depth+1) {
				tooClose = true
				break
			}
		}
		if tooClose {
			return false
		}
		// BZ SuperExclusionZone: optional allowed_chunk_count > chunk_count.
		// If no avoid-set structure is within the allowed range, isPlacementForbidden
		// returns false immediately (remaining avoids are skipped). javap p142-bz.
		if ex.allowed > ex.chunks {
			found := false
			for _, other := range ex.others {
				if other.chunkInRangeDepth(seed, x>>4, z>>4, ex.allowed, depth+1) {
					found = true
					break
				}
			}
			if !found {
				return true
			}
		}
	}
	return true
}

// chunkInRangeDepth checks for a placement chunk within r chunks of (cx, cz),
// bounding recursive placement dependencies.
func (p seedMapPlacement) chunkInRangeDepth(seed int64, cx, cz, r, depth int) bool {
	if p.rings != nil || p.spacing < 1 {
		return false
	}
	if p.landmarkGrid {
		minRX := tfRegionOfChunk(cx - r)
		maxRX := tfRegionOfChunk(cx + r)
		minRZ := tfRegionOfChunk(cz - r)
		maxRZ := tfRegionOfChunk(cz + r)
		for rx := minRX; rx <= maxRX; rx++ {
			for rz := minRZ; rz <= maxRZ; rz++ {
				x, z, ok := tfLandmarkCandidateBlock(seed, rx, rz, p.gridLock)
				if !ok {
					continue
				}
				if dx, dz := x>>4-cx, z>>4-cz; dx < -r || dx > r || dz < -r || dz > r {
					continue
				}
				if p.placementChunkDepth(seed, x, z, depth) {
					return true
				}
			}
		}
		return false
	}
	if p.esLandmark {
		minRX := (cx - r) >> 5
		maxRX := (cx + r) >> 5
		minRZ := (cz - r) >> 5
		maxRZ := (cz + r) >> 5
		for rx := minRX; rx <= maxRX; rx++ {
			for rz := minRZ; rz <= maxRZ; rz++ {
				x, z, ok := esLandmarkCandidateBlock(seed, rx, rz, p.esLock)
				if !ok {
					continue
				}
				if dx, dz := x>>4-cx, z>>4-cz; dx < -r || dx > r || dz < -r || dz > r {
					continue
				}
				if p.placementChunkDepth(seed, x, z, depth) {
					return true
				}
			}
		}
		return false
	}
	for rx := floorDivInt(cx-r, p.spacing); rx <= floorDivInt(cx+r, p.spacing); rx++ {
		for rz := floorDivInt(cz-r, p.spacing); rz <= floorDivInt(cz+r, p.spacing); rz++ {
			x, z := p.candidate(seed, rx, rz)
			if dx, dz := x>>4-cx, z>>4-cz; dx < -r || dx > r || dz < -r || dz > r {
				continue
			}
			if p.placementChunkDepth(seed, x, z, depth) {
				return true
			}
		}
	}
	return false
}

func floorDivInt(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func seedMapID(id string) string {
	if id != "" && !strings.Contains(id, ":") {
		return "minecraft:" + id
	}
	return id
}
