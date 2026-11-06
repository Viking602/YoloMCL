package launcher

import (
	"context"
	"math"
)

// SeedMapBiomeHit is the nearest cell of a biome found by FindBiome. Step is
// the coarse scan spacing: patches narrower than it can be missed, so the
// result is the nearest patch found on that grid, refined to quart cells.
type SeedMapBiomeHit struct {
	X, Z, Step int
	Found      bool
}

const seedMapBiomeScanStep = 64

// FindBiome scans square rings outward from (x, z) at layer y on a coarse
// grid, then refines around the closest coarse hit at quart resolution.
func (s *LocalSampler) FindBiome(ctx context.Context, dimension, biome string, x, y, z, radius int) (SeedMapBiomeHit, error) {
	d, ok := s.dimensions[dimension]
	if !ok {
		return SeedMapBiomeHit{}, nil
	}
	scratch := d.scratch()
	defer d.releaseScratch(scratch)
	step := seedMapBiomeScanStep
	calls := 0
	at := func(bx, bz int) (bool, error) {
		if calls++; calls&1023 == 0 {
			if err := ctx.Err(); err != nil {
				return false, err
			}
		}
		id, ok := d.biomeAt(bx, y, bz, scratch, false)
		return ok && id == biome, nil
	}
	best, bestX, bestZ := math.Inf(1), 0, 0
	rings := max(1, radius/step)
	for r := 0; r <= rings; r++ {
		if float64((r-1)*step) > best {
			break
		}
		for i := -r; i <= r; i++ {
			for _, c := range [][2]int{{i, -r}, {i, r}, {-r, i}, {r, i}} { // corners repeat; harmless
				bx, bz := x+c[0]*step, z+c[1]*step
				hit, err := at(bx, bz)
				if err != nil {
					return SeedMapBiomeHit{}, err
				}
				if dist := math.Hypot(float64(bx-x), float64(bz-z)); hit && dist < best && dist <= float64(radius) {
					best, bestX, bestZ = dist, bx, bz
				}
			}
		}
	}
	if math.IsInf(best, 1) {
		return SeedMapBiomeHit{Step: step}, ctx.Err()
	}
	// Refine: the closest quart of this biome within one coarse cell.
	rx, rz := bestX, bestZ
	for qz := bestZ - step; qz <= bestZ+step; qz += 4 {
		for qx := bestX - step; qx <= bestX+step; qx += 4 {
			hit, err := at(qx, qz)
			if err != nil {
				return SeedMapBiomeHit{}, err
			}
			if dist := math.Hypot(float64(qx-x), float64(qz-z)); hit && dist < best {
				best, rx, rz = dist, qx, qz
			}
		}
	}
	return SeedMapBiomeHit{X: rx, Z: rz, Step: step, Found: true}, nil
}
