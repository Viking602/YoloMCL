package cubiomes

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

// Document is one effective registry entry, with optional pack provenance.
// Compile reads these entries synchronously and retains no references to Data.
type Document struct {
	ID, Kind, Source string
	Data             json.RawMessage
}

// Dimension selects a vanilla biome source.
type Dimension uint8

const (
	// Overworld uses the version's layer, Beta climate or modern climate source.
	Overworld Dimension = iota
	// Nether uses the legacy-seeded five-biome climate source.
	Nether
	// End uses the legacy Simplex island density.
	End
)

// Config selects a verified vanilla release and world seed.
type Config struct {
	Version     string
	Seed        int64
	Dimension   Dimension
	LargeBiomes bool // only valid for Overworld
}

// Definition supplies an effective noise settings holder and biome source.
// Settings is a registry ID or inline JSON. BiomeSource is multi_noise,
// the_end, or a supported legacy vanilla_layered source. Registry keys are
// kind/namespace:id. Missing modern registry entries are errors;
// Compile never silently substitutes vanilla definitions for an override.
type Definition struct {
	Version               string
	Seed                  int64
	Settings, BiomeSource json.RawMessage
}

// Generator holds immutable compiled noise and biome lookup data.
// Its zero value is not usable; construct it with New or Compile.
type Generator struct {
	seed      int64
	version   string
	legacy    *legacyGenerator
	climate   *climateSampler
	end       *endBiomeSampler
	biomes    []string
	sha       uint64
	ids       []BiomeID
	names     []string
	tb        *terraBlender
	tbEnd     *terraBlenderEnd
	surface   *densityNode
	surfaceY  []int
	injection *biomeInjection
	custom    CustomBiomeSource
	customIDs map[string]BiomeID
}

// New constructs a vanilla generator. Unsupported releases are rejected rather
// than approximated by a different version. No network, JVM or cgo is used.
func New(ctx context.Context, cfg Config) (*Generator, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if cfg.Dimension > End || (cfg.LargeBiomes && cfg.Dimension != Overworld) {
		return nil, errors.New("invalid dimension or large-biome combination")
	}
	if mc, ok := legacyRelease(cfg.Version); ok {
		return newLegacyGenerator(cfg, mc)
	}
	if _, _, ok := seedMapRelease(cfg.Version); !ok && !latestRelease(cfg.Version) {
		return nil, fmt.Errorf("unsupported Minecraft version %q", cfg.Version)
	}
	// This key names immutable bundled data, so repeated vanilla construction
	// need not copy and hash its entire JSON registry merely to change a seed.
	keyData := append([]byte("vanilla\x00"+cfg.Version+"\x00"), byte(cfg.Dimension))
	if cfg.LargeBiomes {
		keyData = append(keyData, 1)
	}
	key := sha256.Sum256(keyData)
	if g := cachedProgram(key); g != nil {
		return g.WithSeed(ctx, cfg.Seed)
	}
	documents, err := BuiltinDocuments(cfg.Version)
	if err != nil {
		return nil, err
	}
	docs := make(map[string]Document, len(documents))
	for _, doc := range documents {
		docs[doc.Kind+"/"+doc.ID] = doc
	}
	settings, source := `"minecraft:overworld"`, `{"type":"minecraft:multi_noise","preset":"minecraft:overworld"}`
	if cfg.LargeBiomes {
		settings = `"minecraft:large_biomes"`
	}
	switch cfg.Dimension {
	case Nether:
		settings, source = `"minecraft:nether"`, `{"type":"minecraft:multi_noise","preset":"minecraft:nether"}`
	case End:
		settings, source = `"minecraft:end"`, `{"type":"minecraft:the_end"}`
	}
	g, err := Compile(ctx, docs, Definition{Version: cfg.Version, Seed: cfg.Seed, Settings: json.RawMessage(settings), BiomeSource: json.RawMessage(source)})
	if err == nil {
		rememberProgram(key, g)
	}
	return g, err
}

// Compile builds a bounded generator from effective registry JSON. Callers own
// file ingestion and pack priority. A generator can outlive its input documents.
func Compile(ctx context.Context, docs map[string]Document, def Definition) (*Generator, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if mc, ok := legacyRelease(def.Version); ok {
		return compileLegacy(docs, def, mc)
	}
	key, err := programKey(ctx, docs, def)
	if err != nil {
		return nil, err
	}
	if g := cachedProgram(key); g != nil {
		return g.WithSeed(ctx, def.Seed)
	}
	g, err := compileFresh(ctx, docs, def)
	if err == nil {
		rememberProgram(key, g)
	}
	return g, err
}

func compileFresh(ctx context.Context, docs map[string]Document, def Definition) (*Generator, error) {
	var source struct{ Type string }
	if json.Unmarshal(def.BiomeSource, &source) != nil {
		return nil, errors.New("invalid biome source")
	}
	g := &Generator{seed: def.Seed, version: def.Version}
	var err error
	switch seedMapID(source.Type) {
	case "minecraft:multi_noise":
		g.climate, g.biomes, err = compileClimate(ctx, docs, def.Seed, def.Settings, def.BiomeSource, def.Version)
	case "minecraft:the_end":
		if latestFloatRelease(def.Version) {
			g.end, err = compileLatestEndBiome(ctx, docs, def.Seed, def.Settings)
		} else {
			g.end, err = compileEndBiome(ctx, docs, def.Seed, def.Settings)
		}
		g.biomes = []string{"minecraft:the_end", "minecraft:end_highlands", "minecraft:end_midlands", "minecraft:small_end_islands", "minecraft:end_barrens"}
	default:
		return nil, &CompileError{Code: "biome_source_unsupported", Detail: source.Type}
	}
	if err != nil {
		return nil, err
	}
	g.sha = seedSHA(def.Seed)
	if g.climate != nil {
		g.surface = surfaceHeightRoot(g.climate.router[4])
		if g.surface != nil {
			g.surfaceY = surfaceYNodes(g.climate.router)
		}
	}
	g.initBiomeIDs()
	return g, nil
}

func seedSHA(value int64) uint64 {
	var seed [8]byte
	binary.LittleEndian.PutUint64(seed[:], uint64(value))
	sum := sha256.Sum256(seed[:])
	return binary.LittleEndian.Uint64(sum[:8])
}

// Biomes returns a fresh list of biome names reachable by this source.
func (g *Generator) Biomes() []string { return append([]string(nil), g.biomes...) }

// NodeCount reports compiled density nodes or legacy layers for caller budgets.
func (g *Generator) NodeCount() int {
	if g.legacy != nil {
		if g.legacy.layers != nil {
			return g.legacy.layers.n
		}
		return 0
	}
	if g.climate != nil {
		return g.climate.nodes
	}
	return g.end.nodes
}

// Sampler owns mutable scratch and search history. Do not share a Sampler
// between goroutines or copy it; share the Generator and call NewSampler.
type Sampler struct {
	g             *Generator
	scratch       densitySample
	previous      int
	legacy        legacyScratch
	beta          *betaGenerator
	tb            *terraBlenderScratch
	tbEnd         *terraBlenderEndScratch
	blocks        []blockBiomeCell
	surface       *surfaceScratch
	injection     densitySample
	custom        CustomBiomeSampler
	customRouter  *NoiseRouterSampler
	customErr     error
	customContext context.Context
}

// NewSampler allocates reusable work storage for a sequence of queries.
func (g *Generator) NewSampler() *Sampler {
	s := &Sampler{g: g, previous: -1}
	if g.custom != nil {
		s.customRouter = (&NoiseRouter{program: g}).NewSampler()
		s.custom, s.customErr = g.custom.NewBiomeSampler(context.Background(), s.customRouter)
	}
	if g.injection != nil {
		s.injection = densitySample{seen: make([]uint32, g.injection.nodes), values: make([]float64, g.injection.nodes)}
	}
	if g.tb != nil {
		s.tb = g.tb.newScratch(g.seed)
	}
	if g.tbEnd != nil {
		s.tbEnd = g.tbEnd.newScratch(g.seed)
	}
	if g.legacy != nil && g.legacy.beta != nil {
		s.beta = g.legacy.beta
	}
	n := g.NodeCount()
	if g.legacy != nil {
		n = 0
	} // layer scratch uses its own integer arena
	s.scratch = densitySample{seen: make([]uint32, n), values: make([]float64, n)}
	if g.end != nil || (g.climate != nil && g.climate.hasEnd) || (g.legacy != nil && g.legacy.end != nil) {
		s.scratch.endCache = &endCache{}
	}
	if s.customRouter != nil {
		s.scratch.endHeight = s.customRouter.scratch.endHeight
		s.injection.endHeight = s.customRouter.scratch.endHeight
	}
	return s
}

// Reset clears biome search history while retaining allocated scratch.
func (s *Sampler) Reset() {
	s.previous = -1
	if s.custom != nil {
		s.customErr = nil
	}
	if s.tb != nil {
		s.tb.reset()
	}
}

// NoiseBiome returns the containing quart-cell biome at block coordinates.
// Every call starts a fresh tree search, independent of earlier queries.
// False means a custom density produced an invalid climate value.
func (s *Sampler) NoiseBiome(x, y, z int) (string, bool) {
	s.Reset()
	return s.NextNoiseBiome(x, y, z)
}

// NextNoiseBiome retains the previous winning leaf on equal distances, matching
// vanilla's history-dependent climate lookup. Reset at independent raster jobs.
// Coordinates are blocks, including negative values; they must fit Java int32.
func (s *Sampler) NextNoiseBiome(x, y, z int) (string, bool) {
	if s.g.legacy != nil || s.tb != nil || s.tbEnd != nil || s.g.injection != nil || s.g.custom != nil {
		id, ok := s.NextNoiseBiomeID(x, y, z)
		return s.g.BiomeName(id), ok
	}
	if c := s.g.climate; c != nil {
		p, ok := c.target(&s.scratch, x, y, z)
		if !ok {
			return "", false
		}
		s.previous = c.root.search(&p, s.previous)
		return c.root.biomes[c.root.nodes[s.previous].biome], true
	}
	return s.g.end.biome(x, y, z, &s.scratch)
}

// Climate returns temperature, humidity, continentalness, erosion, depth and
// weirdness, quantized as in Java. End sources have no climate vector.
func (s *Sampler) Climate(x, y, z int) ([6]int64, bool) {
	if s.g.climate == nil {
		return [6]int64{}, false
	}
	p, ok := s.g.climate.target(&s.scratch, x, y, z)
	return [6]int64(p[:6]), ok
}

// BiomeAt applies native Beta climate, pre-1.15 2D or post-1.15 3D Voronoi
// lookup. It predicts new-world biomes, not saved chunks or upgrade blending.
func (s *Sampler) BiomeAt(x, y, z int) (string, bool) {
	if s.beta != nil {
		id := s.beta.biome(x, z, false)
		return s.g.BiomeName(id), true
	}
	x, y, z = s.voronoi(x, y, z)
	return s.NoiseBiome(x*4, y*4, z*4)
}
