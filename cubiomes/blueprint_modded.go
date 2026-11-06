package cubiomes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// BlueprintModdedConfig ports Blueprint's ModdedBiomeSource (1.20.1 / 7.1.x).
// Slices and size come from the datapack registry and blueprint-common.toml;
// seeds match ModdedBiomeSlicesManager (world seed + dimension location hash).
type BlueprintModdedConfig struct {
	Size   int
	DimID  string
	Seed   int64
	Slices []BlueprintModdedSlice
}

// BlueprintModdedSlice is one weighted ModdedBiomeSlice for a dimension.
type BlueprintModdedSlice struct {
	ID       string
	Weight   int
	Provider BlueprintModdedProvider
}

// BlueprintModdedProvider is a Blueprint ModdedBiomeProvider.
// Type is "blueprint:original" or "blueprint:overlay".
type BlueprintModdedProvider struct {
	Type     string
	Overlays []BlueprintModdedOverlay
	// MultiNoise is the blueprint:multi_noise parameter list with areas,
	// only_map_from_areas and registry presence already resolved; entries that
	// resolve to the original source carry BlueprintOriginalSourceMarker.
	MultiNoise json.RawMessage
}

// BlueprintOriginalSourceMarker is BlueprintBiomes.ORIGINAL_SOURCE_MARKER.
const BlueprintOriginalSourceMarker = "blueprint:original_source_marker"

// BlueprintModdedOverlay replaces the original biome when it matches Matches.
type BlueprintModdedOverlay struct {
	Matches []string
	// Biome is used when the nested source is a fixed or single-biome checkerboard.
	Biome string
	// Scale is the checkerboard scale (quart shift); ignored for a fixed biome.
	Scale int
}

type blueprintModded struct {
	inner                      *Generator
	slices                     []blueprintSlice
	size                       int
	slicesSeed, slicesZoomSeed int64
	biomes                     []string
}

type blueprintSlice struct {
	id       string
	weight   int
	provider BlueprintModdedProvider
	tree     *climateTree
}

type blueprintModdedSampler struct {
	src   *blueprintModded
	inner *Sampler
	// previous is the per-slice RTree lastResult (Climate.RTree ThreadLocal).
	previous []int
}

// WithBlueprintModded wraps a compiled (and TerraBlender-applied) generator with
// Blueprint's weighted slice selection. The originals provider and exhausted
// fallbacks call through to the inner generator; overlay providers may replace
// matched biomes. PositionalRandomCache is omitted: computeZoomedPositionalRandom
// is pure in (x, z) so a cache only affects speed.
func WithBlueprintModded(ctx context.Context, base *Generator, cfg BlueprintModdedConfig) (*Generator, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if base == nil {
		return nil, errors.New("blueprint modded biome source requires a compiled generator")
	}
	if cfg.Size <= 0 || cfg.Size > 32 {
		return nil, errors.New("invalid Blueprint modded biome slice size")
	}
	if cfg.DimID == "" || !strings.Contains(cfg.DimID, ":") {
		return nil, errors.New("invalid Blueprint dimension id")
	}
	if len(cfg.Slices) == 0 || len(cfg.Slices) > 256 {
		return nil, errors.New("blueprint modded biome slices exceed supported limits")
	}
	dimHash := int64(resourceLocationHash(cfg.DimID))
	src := &blueprintModded{
		inner:          base,
		size:           cfg.Size,
		slicesSeed:     cfg.Seed + 1791510900 + dimHash,
		slicesZoomSeed: cfg.Seed - 771160217 + dimHash,
		slices:         make([]blueprintSlice, 0, len(cfg.Slices)),
	}
	total := 0
	seen := map[string]bool{}
	for _, name := range base.Biomes() {
		if name != "" && !seen[name] {
			seen[name] = true
			src.biomes = append(src.biomes, name)
		}
	}
	for _, slice := range cfg.Slices {
		var tree *climateTree
		if slice.Weight <= 0 || slice.Weight > 1<<30 {
			return nil, errors.New("invalid Blueprint modded biome slice weight")
		}
		switch slice.Provider.Type {
		case "blueprint:original":
			if len(slice.Provider.Overlays) != 0 {
				return nil, errors.New("blueprint original provider cannot carry overlays")
			}
		case "blueprint:overlay":
			if len(slice.Provider.Overlays) == 0 || len(slice.Provider.Overlays) > 64 {
				return nil, errors.New("invalid Blueprint overlay provider")
			}
			for _, overlay := range slice.Provider.Overlays {
				if overlay.Biome == "" || len(overlay.Matches) == 0 || len(overlay.Matches) > 256 {
					return nil, errors.New("invalid Blueprint overlay entry")
				}
				if overlay.Scale < 0 || overlay.Scale > 16 {
					return nil, errors.New("invalid Blueprint overlay checkerboard scale")
				}
				if !seen[overlay.Biome] {
					seen[overlay.Biome] = true
					src.biomes = append(src.biomes, overlay.Biome)
				}
			}
		case "blueprint:multi_noise":
			if base.climate == nil {
				return nil, errors.New("blueprint multi_noise provider requires a climate sampler")
			}
			t, ids, err := parseClimateList(ctx, slice.Provider.MultiNoise, climateTreeOrder(base.version))
			if err != nil {
				return nil, fmt.Errorf("blueprint multi_noise provider %s: %w", slice.ID, err)
			}
			tree = t
			for _, id := range ids {
				if id != BlueprintOriginalSourceMarker && !seen[id] {
					seen[id] = true
					src.biomes = append(src.biomes, id)
				}
			}
		default:
			return nil, fmt.Errorf("unsupported Blueprint modded biome provider %q", slice.Provider.Type)
		}
		total += slice.Weight
		src.slices = append(src.slices, blueprintSlice{id: slice.ID, weight: slice.Weight, provider: slice.Provider, tree: tree})
	}
	if total <= 0 {
		return nil, errors.New("blueprint modded biome slices have no positive weight")
	}
	if len(src.biomes) == 0 || len(src.biomes) > 32768 {
		return nil, errors.New("invalid Blueprint modded biome palette")
	}
	if _, err := src.NewBiomeSampler(ctx, nil); err != nil {
		return nil, err
	}
	g := *base
	g.tb = nil
	g.tbEnd = nil
	g.injection = nil
	g.custom = src
	g.biomes = append([]string(nil), src.biomes...)
	// Keep the inner climate/TB name table and extend it for overlay biomes.
	// initBiomeIDs would rebuild from climate.root only and drop overlays.
	if g.names == nil {
		g.initBiomeIDs()
	}
	palette := make(map[string]BiomeID, len(g.names))
	for i, name := range g.names {
		if name != "" {
			palette[name] = BiomeID(i)
		}
	}
	g.customIDs = make(map[string]BiomeID, len(src.biomes))
	for _, id := range src.biomes {
		bid, ok := palette[id]
		if !ok {
			bid = BiomeID(len(g.names))
			palette[id] = bid
			g.names = append(g.names, id)
		}
		g.customIDs[id] = bid
	}
	return &g, ctx.Err()
}

func (b *blueprintModded) Biomes() []string { return append([]string(nil), b.biomes...) }

func (b *blueprintModded) NewBiomeSampler(ctx context.Context, _ *NoiseRouterSampler) (CustomBiomeSampler, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &blueprintModdedSampler{src: b, inner: b.inner.NewSampler(), previous: newBlueprintPrevious(len(b.slices))}, nil
}

func (s *blueprintModdedSampler) NoiseBiome(ctx context.Context, x, y, z int32) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return s.src.getNoiseBiome(s, int(x), int(y), int(z))
}

// getNoiseBiome ports ModdedBiomeSource.getNoiseBiome / getSlice(biomes=true).
func newBlueprintPrevious(n int) []int {
	p := make([]int, n)
	for i := range p {
		p[i] = -1
	}
	return p
}

func (b *blueprintModded) getNoiseBiome(s *blueprintModdedSampler, x, y, z int) (string, error) {
	inner := s.inner
	active := append([]blueprintSlice(nil), b.slices...)
	total := 0
	for _, slice := range active {
		total += slice.weight
	}
	random := b.positionalRandom(x, z)
	roll := floorMod64(random, total)
retry:
	for {
		for i := range active {
			slice := &active[i]
			if slice.weight == 0 {
				continue
			}
			roll -= slice.weight
			if roll >= 0 {
				continue
			}
			biome, marker, err := b.providerBiome(s, i, x, y, z)
			if err != nil {
				return "", err
			}
			if marker {
				// Overlay declined: drop this slice and re-roll, matching getSlice.
				total -= slice.weight
				slice.weight = 0
				if total <= 0 {
					return b.originalBiome(inner, x, y, z)
				}
				roll = floorMod64(random, total)
				continue retry
			}
			return biome, nil
		}
		// No live slice accepted the roll (should not happen when total > 0).
		return b.originalBiome(inner, x, y, z)
	}
}

func (b *blueprintModded) providerBiome(s *blueprintModdedSampler, index, x, y, z int) (string, bool, error) {
	inner, p := s.inner, b.slices[index].provider
	switch p.Type {
	case "blueprint:multi_noise":
		// MultiNoiseModdedBiomeProvider.getNoiseBiome: biomes.findValue(sampler.sample(x, y, z))
		// with quart coordinates; the original-source marker declines the slice.
		tree := b.slices[index].tree
		climate, ok := inner.Climate(x*4, y*4, z*4)
		if !ok {
			return "", false, errors.New("blueprint multi_noise climate sample invalid")
		}
		var target climatePoint
		copy(target[:6], climate[:])
		s.previous[index] = tree.search(&target, s.previous[index])
		biome := tree.biomes[tree.nodes[s.previous[index]].biome]
		if biome == BlueprintOriginalSourceMarker {
			return "", true, nil
		}
		return biome, false, nil
	case "blueprint:original":
		biome, err := b.originalBiome(inner, x, y, z)
		return biome, false, err
	case "blueprint:overlay":
		original, err := b.originalBiome(inner, x, y, z)
		if err != nil {
			return "", false, err
		}
		for _, overlay := range p.Overlays {
			if !overlayMatches(overlay.Matches, original) {
				continue
			}
			if overlay.Biome != "" {
				return overlay.Biome, false, nil
			}
		}
		return "", true, nil
	default:
		return "", false, fmt.Errorf("unsupported Blueprint provider %q", p.Type)
	}
}

func overlayMatches(matches []string, biome string) bool {
	for _, id := range matches {
		if id == biome {
			return true
		}
	}
	return false
}

func (b *blueprintModded) originalBiome(inner *Sampler, x, y, z int) (string, error) {
	// BiomeSource.getNoiseBiome takes quart coordinates; Sampler.NoiseBiome takes blocks.
	biome, ok := inner.NoiseBiome(x*4, y*4, z*4)
	if !ok || biome == "" {
		return "", errors.New("blueprint original biome source returned no biome")
	}
	return biome, nil
}

func (b *blueprintModded) positionalRandom(x, z int) int64 {
	return b.computeZoomedPositionalRandom(x, z)
}

// computeZoomedPositionalRandom ports ModdedBiomeSource.computeZoomedPositionalRandom.
func (b *blueprintModded) computeZoomedPositionalRandom(x, z int) int64 {
	// QuartPos.fromBlock on quart coordinates (getNoiseBiome inputs).
	x >>= 2
	z >>= 2
	seed := b.slicesZoomSeed
	for range b.size {
		oddX := x & 1
		oddZ := z & 1
		halfX := x >> 1
		halfZ := z >> 1
		switch {
		case oddX == 0 && oddZ == 0:
			x, z = halfX, halfZ
		case oddX == 0:
			if blueprintNextInt(seed, halfX<<1, halfZ<<1, 2) == 0 {
				z = halfZ
			} else {
				z = (z + 1) >> 1
			}
			x = halfX
		case oddZ == 0:
			if blueprintNextInt(seed, halfX<<1, halfZ<<1, 2) == 0 {
				x = halfX
			} else {
				x = (x + 1) >> 1
			}
			z = halfZ
		default:
			switch blueprintNextInt(seed, halfX<<1, halfZ<<1, 4) {
			case 0:
				x, z = halfX, halfZ
			case 1:
				x, z = (x+1)>>1, halfZ
			case 2:
				x, z = halfX, (z+1)>>1
			default:
				x, z = (x+1)>>1, (z+1)>>1
			}
		}
	}
	return blueprintNext(b.slicesSeed, x, z)
}

func blueprintNextInt(seed int64, x, z, bound int) int {
	return floorMod64(blueprintNext(seed, x, z), bound)
}

// blueprintNext ports ModdedBiomeSource.next (LinearCongruentialGenerator chain).
func blueprintNext(seed int64, x, z int) int64 {
	seed = lcgNext(seed, int64(int32(x)))
	seed = lcgNext(seed, int64(int32(z)))
	seed = lcgNext(seed, int64(int32(x)))
	seed = lcgNext(seed, int64(int32(z)))
	return seed >> 24
}

// lcgNext is net.minecraft.util.LinearCongruentialGenerator.next.
func lcgNext(seed, salt int64) int64 {
	return seed*(seed*6364136223846793005+1442695040888963407) + salt
}

func floorMod64(x int64, m int) int {
	if m <= 0 {
		return 0
	}
	r := int(x % int64(m))
	if r < 0 {
		r += m
	}
	return r
}

// javaStringHash is java.lang.String.hashCode for BMP code points.
func javaStringHash(s string) int32 {
	h := int32(0)
	for _, ch := range s {
		h = h*31 + int32(ch)
	}
	return h
}

// resourceLocationHash is net.minecraft.resources.ResourceLocation.hashCode:
// 31 * namespace.hashCode() + path.hashCode().
func resourceLocationHash(id string) int32 {
	ns, path, ok := strings.Cut(id, ":")
	if !ok {
		return javaStringHash(id)
	}
	return javaStringHash(ns)*31 + javaStringHash(path)
}
