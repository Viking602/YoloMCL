package launcher

import (
	"context"
	"errors"
	"sync"

	"github.com/Viking602/YoloMCL/cubiomes"
)

// TerrainSubstance identifies the noise-stage material before surface rules,
// carvers and features.
type TerrainSubstance = cubiomes.Substance

const (
	TerrainAir   = cubiomes.Air
	TerrainSolid = cubiomes.Solid
	TerrainWater = cubiomes.Water
	TerrainLava  = cubiomes.Lava
)

// SeedMapTerrainInfo describes a dimension's noise volume.
type SeedMapTerrainInfo struct {
	MinY, Height, SeaLevel int
}

func (s *LocalSampler) compileTerrain(ctx context.Context, docs map[string]WorldgenDocument, def cubiomes.Definition, d *seedMapDimension, id string) {
	t, err := cubiomes.CompileTerrain(ctx, docs, def)
	if err != nil {
		code := "terrain_invalid"
		var unsupported *cubiomes.CompileError
		if errors.As(err, &unsupported) {
			code = unsupported.Code
		}
		d.issues = append(d.issues, WorldgenIssue{code, id + " / " + err.Error()})
		return
	}
	d.terrain = t
	d.terrainPool = &sync.Pool{New: func() any { return t.NewSampler() }}
}

// TerrainInfo reports whether dimension has a compiled final_density.
func (s *LocalSampler) TerrainInfo(dimension string) (SeedMapTerrainInfo, bool) {
	if s == nil {
		return SeedMapTerrainInfo{}, false
	}
	d, ok := s.dimensions[dimension]
	if !ok || d.terrain == nil {
		return SeedMapTerrainInfo{}, false
	}
	return SeedMapTerrainInfo{d.terrain.MinY, d.terrain.Height, d.terrain.SeaLevel}, true
}

// TerrainColumn fills out (len >= Height, index y-MinY) with the noise-stage
// column at x, z and returns the top solid Y and the top non-air Y
// (WORLD_SURFACE_WG-1). out may be nil to find only the solid top quickly.
func (s *LocalSampler) TerrainColumn(dimension string, x, z int, out []TerrainSubstance) (solidTop, top int, ok bool) {
	if s == nil {
		return 0, 0, false
	}
	d, found := s.dimensions[dimension]
	if !found || d.terrain == nil {
		return 0, 0, false
	}
	sampler := d.terrainPool.Get().(*cubiomes.TerrainSampler)
	defer d.terrainPool.Put(sampler)
	solidTop, top = sampler.Column(x, z, out)
	return solidTop, top, true
}

// TerrainDensity is final_density at one block.
func (s *LocalSampler) TerrainDensity(dimension string, x, y, z int) (float64, bool) {
	if s == nil {
		return 0, false
	}
	d, found := s.dimensions[dimension]
	if !found || d.terrain == nil {
		return 0, false
	}
	sampler := d.terrainPool.Get().(*cubiomes.TerrainSampler)
	defer d.terrainPool.Put(sampler)
	return sampler.Density(x, y, z), true
}

// TerrainSurfaceAt is the highest solid noise-stage block at x, z (the
// final_density surface before surface rules and features).
func (s *LocalSampler) TerrainSurfaceAt(dimension string, x, z int) (int, bool) {
	if s == nil {
		return 0, false
	}
	d, found := s.dimensions[dimension]
	if !found || d.terrain == nil {
		return 0, false
	}
	sampler := d.terrainPool.Get().(*cubiomes.TerrainSampler)
	solidTop := sampler.SolidTop(x, z)
	d.terrainPool.Put(sampler)
	if info, _ := s.TerrainInfo(dimension); solidTop < info.MinY {
		return 0, false
	}
	return solidTop, true
}
