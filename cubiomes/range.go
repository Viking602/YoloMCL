package cubiomes

import (
	"context"
	"errors"
	"math"
)

// Range describes a block-coordinate grid. Steps are positive block distances,
// not cubiomes C scale values. Results are indexed (iy*SizeZ+iz)*SizeX+ix.
// Voronoi selects block-biome zoom; otherwise samples use containing quart cells.
type Range struct {
	X, Y, Z             int
	SizeX, SizeY, SizeZ int
	StepXZ, StepY       int
	Voronoi             bool
}

// Generate fills caller-owned output in Y/Z/X order without per-cell allocations.
// It resets climate history once per grid, retains ties in raster order, checks
// cancellation at least every 64 cells, and can leave partial output on error.
// A maximum of 16,777,216 cells prevents unbounded synchronous jobs.
func (s *Sampler) Generate(ctx context.Context, r Range, dst []string) error {
	return s.generate(ctx, r, dst, nil)
}

func (s *Sampler) generate(ctx context.Context, r Range, dst []string, ids []BiomeID) error {
	if s.custom != nil {
		s.customContext = ctx
		defer func() { s.customContext = nil }()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	const maxCells = 1 << 24
	if r.SizeX <= 0 || r.SizeY <= 0 || r.SizeZ <= 0 || r.StepXZ <= 0 || r.StepY <= 0 ||
		r.SizeX > maxCells || r.SizeY > maxCells/r.SizeX || r.SizeZ > maxCells/(r.SizeX*r.SizeY) {
		return errors.New("invalid or excessive biome range")
	}
	count := r.SizeX * r.SizeY * r.SizeZ
	if max(len(dst), len(ids)) < count {
		return errors.New("biome destination is too short")
	}
	// Division before multiplication avoids overflow for attacker-sized steps.
	valid := func(start, count, step int) bool {
		return int64(start) >= math.MinInt32+4 && int64(start) <= math.MaxInt32-4 &&
			(count == 1 || int64(step) <= (math.MaxInt32-4-int64(start))/int64(count-1))
	}
	if !valid(r.X, r.SizeX, r.StepXZ) || !valid(r.Z, r.SizeZ, r.StepXZ) || !valid(r.Y, r.SizeY, r.StepY) {
		return errors.New("biome coordinates exceed Java integer range")
	}
	s.Reset()
	if r.Voronoi && r.StepXZ < 4 && s.g.legacy == nil {
		if done, err := s.generateBlocks(ctx, r, dst, ids); done {
			return err
		}
	}
	if s.g.legacy != nil && s.g.legacy.layers != nil {
		if done, err := s.generateLegacy(ctx, r, dst, ids); done {
			return err
		}
	}
	i := 0
	for iy := range r.SizeY {
		for iz := range r.SizeZ {
			for ix := range r.SizeX {
				if i&63 == 0 {
					if err := ctx.Err(); err != nil {
						return err
					}
				}
				x, y, z := r.X+ix*r.StepXZ, r.Y+iy*r.StepY, r.Z+iz*r.StepXZ
				if r.Voronoi && s.beta != nil {
					id := s.beta.biome(x, z, false)
					if ids != nil {
						ids[i] = id
					} else {
						dst[i] = s.g.BiomeName(id)
					}
					i++
					continue
				}
				if r.Voronoi {
					x, y, z = s.voronoi(x, y, z)
					x, y, z = x*4, y*4, z*4
				}
				var ok bool
				if ids != nil {
					ids[i], ok = s.NextNoiseBiomeID(x, y, z)
				} else {
					dst[i], ok = s.NextNoiseBiome(x, y, z)
				}
				if !ok {
					if s.customErr != nil {
						return s.customErr
					}
					return errors.New("invalid climate sample")
				}
				i++
			}
		}
	}
	return ctx.Err()
}

// A whole layer raster reuses each parent rectangle. Sparse queries fall back
// to points rather than allocating a world-sized bounding rectangle.
func (s *Sampler) generateLegacy(ctx context.Context, r Range, dst []string, ids []BiomeID) (bool, error) {
	x0, z0 := r.X>>2, r.Z>>2
	x1, z1 := (r.X+(r.SizeX-1)*r.StepXZ)>>2, (r.Z+(r.SizeZ-1)*r.StepXZ)>>2
	if r.Voronoi {
		x0, z0 = (r.X-2)>>2, (r.Z-2)>>2
		x1, z1 = ((r.X+(r.SizeX-1)*r.StepXZ-2)>>2)+1, ((r.Z+(r.SizeZ-1)*r.StepXZ-2)>>2)+1
	}
	w, h := x1-x0+1, z1-z0+1
	if w > 1<<20 || h > (1<<20)/w {
		return false, nil
	}
	s.legacy.used = 0
	s.legacy.ctx = ctx
	grid := s.legacy.take(w * h)
	if err := s.g.legacy.layers.root.area(&s.legacy, grid, x0, z0, w, h); err != nil {
		return true, err
	}
	if !r.Voronoi && r.StepXZ == 4 {
		// The requested quart raster already is the layer output. Avoid
		// recomputing coordinates and rectangular indexes for every cell.
		for iy := range r.SizeY {
			base := iy * len(grid)
			if ids != nil {
				target := ids[base : base+len(grid)]
				for i, id := range grid {
					if i&63 == 0 && ctx.Err() != nil {
						return true, ctx.Err()
					}
					target[i] = BiomeID(id)
				}
			} else {
				target := dst[base : base+len(grid)]
				for i, id := range grid {
					if i&63 == 0 && ctx.Err() != nil {
						return true, ctx.Err()
					}
					target[i] = s.g.BiomeName(BiomeID(id))
				}
			}
		}
		return true, ctx.Err()
	}
	i := 0
	for iy := range r.SizeY {
		for iz := range r.SizeZ {
			for ix := range r.SizeX {
				if i&63 == 0 {
					if err := ctx.Err(); err != nil {
						return true, err
					}
				}
				x, y, z := r.X+ix*r.StepXZ, r.Y+iy*r.StepY, r.Z+iz*r.StepXZ
				if r.Voronoi {
					x, _, z = s.voronoi(x, y, z)
				} else {
					x >>= 2
					z >>= 2
				}
				id := BiomeID(grid[(z-z0)*w+x-x0])
				if ids != nil {
					ids[i] = id
				} else {
					dst[i] = s.g.BiomeName(id)
				}
				i++
			}
		}
	}
	return true, ctx.Err()
}
