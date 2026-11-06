package cubiomes

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/bits"
)

// End uses legacy-seeded Simplex and the noise router's erosion channel. This
// also respects datapack changes to that channel instead of painting a fixed
// End biome ring. Reference: DensityFunctions.EndIslandDensityFunction.
type endIslands struct{ permutation [256]byte }

// Coordinate-checked direct caches stay bounded, including long random walks.
// Unlike a map that stops accepting entries, they retain locality indefinitely.
type endCache struct {
	rows   [64]endFactorRow
	biomes [256]endBiome
}

type endBiome struct {
	x, y, z int32
	id      BiomeID
}

func newEndIslands(seed int64) *endIslands {
	r := newNoiseRandom(seed, true)
	r.skipLegacy(17292)
	return &endIslands{newImprovedNoise(r).p}
}

func (n *endIslands) simplex(x, z float64) float64 {
	return simplex2D(&n.permutation, x, z)
}

func simplex2D(permutation *[256]byte, x, z float64) float64 {
	f2, g2 := .5*(math.Sqrt(3)-1), (3-math.Sqrt(3))/6
	skew := (x + z) * f2
	i, j := int(math.Floor(x+skew)), int(math.Floor(z+skew))
	unskew := float64(i+j) * g2
	x0, z0 := x-(float64(i)-unskew), z-(float64(j)-unskew)
	dx, dz := 0, 1
	if x0 > z0 {
		dx, dz = 1, 0
	}
	p := func(i int) int { return int(permutation[i&255]) }
	gradients := [12][2]float64{{1, 1}, {-1, 1}, {1, -1}, {-1, -1}, {1, 0}, {-1, 0}, {1, 0}, {-1, 0}, {0, 1}, {0, -1}, {0, 1}, {0, -1}}
	corner := func(index int, a, b float64) float64 {
		t := .5 - a*a - b*b
		if t < 0 {
			return 0
		}
		t *= t
		g := gradients[index%12]
		return t * t * (g[0]*a + g[1]*b)
	}
	return 70 * (corner(p(i+p(j)), x0, z0) + corner(p(i+dx+p(j+dz)), x0-float64(dx)+g2, z0-float64(dz)+g2) + corner(p(i+1+p(j+1)), x0-1+2*g2, z0-1+2*g2))
}

func (n *endIslands) height(x, z int, p *densitySample) float32 {
	if p.endHeight != nil {
		value, ok := p.endHeight(n.simplex, int32(x), int32(z))
		if !ok {
			return float32(math.NaN())
		}
		return value
	}
	hx, hz, ox, oz := x/2, z/2, x%2, z%2
	// Java int multiplication overflow is part of the far-End ring behavior.
	r2 := int32(x)*int32(x) + int32(z)*int32(z)
	height := max(float32(-100), min(float32(80), 100-float32(math.Sqrt(float64(float32(r2))))*8))
	if math.IsNaN(float64(height)) {
		return height
	}
	if p.endCache == nil {
		p.endCache = &endCache{}
	}
	for dz := -12; dz <= 12; dz++ {
		row, mask := n.factorRow(p.endCache, hx-12, hx+13, hz+dz, false)
		for mask != 0 {
			dx := bits.TrailingZeros64(mask) - 12
			mask &= mask - 1
			factor := row.values[(hx+dx)&63]
			a, b := float32(ox-dx*2), float32(oz-dz*2)
			v := 100 - float32(float32(math.Sqrt(float64(a*a+b*b)))*factor)
			height = max(height, max(float32(-100), min(float32(80), v)))
		}
	}
	return height
}

type endBiomeSampler struct {
	erosion *densityNode
	nodes   int
}

func compileEndBiome(ctx context.Context, docs map[string]Document, seed int64, settings json.RawMessage) (*endBiomeSampler, error) {
	var ref string
	if json.Unmarshal(settings, &ref) == nil {
		settings = docs["noise_settings/"+seedMapID(ref)].Data
	}
	var data struct {
		Legacy bool                       `json:"legacy_random_source"`
		Router map[string]json.RawMessage `json:"noise_router"`
	}
	if json.Unmarshal(settings, &data) != nil || data.Router == nil {
		return nil, errors.New("missing End noise settings")
	}
	c := newDensityCompiler(ctx, docs, seed, data.Legacy)
	n, err := c.compile(data.Router["erosion"], 0)
	if err != nil {
		return nil, err
	}
	return &endBiomeSampler{n, c.nodes}, nil
}

func (e *endBiomeSampler) biome(x, y, z int, p *densitySample) (string, bool) {
	id, ok := e.biomeID(x, y, z, p)
	return legacyBiomeNames[id], ok
}

func (e *endBiomeSampler) biomeID(x, y, z int, p *densitySample) (BiomeID, bool) {
	cx, cz := x>>4, z>>4
	if int64(cx)*int64(cx)+int64(cz)*int64(cz) <= 4096 {
		return biomeTheEnd, true
	}
	if p.endCache == nil {
		p.endCache = &endCache{}
	}
	cy := y >> 2
	cached := &p.endCache.biomes[(cx*31+cz*17+cy)&255]
	if cached.id != 0 && cached.x == int32(cx) && cached.y == int32(cy) && cached.z == int32(cz) {
		return cached.id, true
	}
	p.at((cx*2+1)*8, (y>>2)<<2, (cz*2+1)*8)
	v := e.erosion.eval(p)
	id := BiomeID(biomeEndBarrens)
	switch {
	case math.IsNaN(v):
		id = biomeEndBarrens
	case math.IsInf(v, 0):
		return 0, false
	case v > .25:
		id = biomeEndHighlands
	case v >= -.0625:
		id = biomeEndMidlands
	case v < -.21875:
		id = biomeSmallEndIslands
	}
	*cached = endBiome{int32(cx), int32(cy), int32(cz), id}
	return id, true
}
