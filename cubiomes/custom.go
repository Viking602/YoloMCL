package cubiomes

import (
	"context"
	"errors"
	"fmt"
)

// CustomBiomeSource supplies a validated, immutable biome algorithm. Each
// worker receives its own state and the world's seeded raw climate functions.
// Implementations must not return vanilla approximations on execution failure.
type CustomBiomeSource interface {
	Biomes() []string
	NewBiomeSampler(context.Context, *NoiseRouterSampler) (CustomBiomeSampler, error)
}
type CustomBiomeSampler interface {
	NoiseBiome(context.Context, int32, int32, int32) (string, error)
}

func CompileCustomBiomeSource(ctx context.Context, docs map[string]Document, def Definition, source CustomBiomeSource) (*Generator, error) {
	if source == nil {
		return nil, errors.New("missing custom biome source")
	}
	router, err := CompileNoiseRouter(ctx, docs, def)
	if err != nil {
		return nil, err
	}
	biomes := source.Biomes()
	if len(biomes) == 0 || len(biomes) > 32768 {
		return nil, errors.New("invalid custom biome palette")
	}
	for _, id := range biomes {
		if _, ok := docs["biome/"+id]; !ok {
			return nil, fmt.Errorf("custom biome holder is absent from effective registry: %s", id)
		}
	}
	if _, err = source.NewBiomeSampler(ctx, router.NewSampler()); err != nil {
		return nil, err
	}
	g := router.program
	g.custom = source
	g.biomes = append([]string(nil), biomes...)
	g.sha = seedSHA(def.Seed)
	g.initBiomeIDs()
	g.customIDs = make(map[string]BiomeID, len(biomes))
	for i, id := range biomes {
		g.customIDs[id] = g.ids[i]
	}
	return g, ctx.Err()
}

func (s *Sampler) customBiome(x, y, z int) (BiomeID, bool) {
	if s.custom == nil || s.customErr != nil {
		return 0, false
	}
	ctx := s.customContext
	if ctx == nil {
		ctx = context.Background()
	}
	id, err := s.custom.NoiseBiome(ctx, int32(x>>2), int32(y>>2), int32(z>>2))
	if err != nil {
		s.customErr = err
		return 0, false
	}
	result, ok := s.g.customIDs[id]
	if !ok {
		s.customErr = fmt.Errorf("custom biome outside compiled palette: %s", id)
	}
	return result, ok
}
