package cubiomes

import (
	"context"
	"errors"
)

// A dense block slice repeatedly visits the same quart cells. Cache the
// coordinate-derived jitter and climate, not the winning biome: climate trees
// must still see every query in raster order to preserve previous-leaf ties.
type blockBiomeCell struct {
	rx, ry, rz int64
	point      climatePoint
	region     int
	id         BiomeID
	ready      bool
}

func (s *Sampler) generateBlocks(ctx context.Context, r Range, dst []string, ids []BiomeID) (bool, error) {
	x0, z0 := (r.X-2)>>2, (r.Z-2)>>2
	x1, z1 := ((r.X+(r.SizeX-1)*r.StepXZ-2)>>2)+1, ((r.Z+(r.SizeZ-1)*r.StepXZ-2)>>2)+1
	w, h := x1-x0+1, z1-z0+1
	// At most 3 MiB per sampler; sparse or very wide grids use the point
	// path. Only two quart Y planes are needed for each output slice.
	const maxCorners = 32768
	if w > maxCorners/2 || h > maxCorners/(2*w) {
		return false, nil
	}
	plane := w * h
	if cap(s.blocks) < 2*plane {
		s.blocks = make([]blockBiomeCell, 2*plane)
	}
	cells := s.blocks[:2*plane]
	lastY, haveY := 0, false
	i := 0
	for iy := range r.SizeY {
		y := r.Y + iy*r.StepY - 2
		qy := y >> 2
		if !haveY || qy != lastY {
			for cy := range 2 {
				for cz := range h {
					for cx := range w {
						j := (cy*h+cz)*w + cx
						if j&63 == 0 && ctx.Err() != nil {
							return true, ctx.Err()
						}
						cell := &cells[j]
						cell.rx, cell.ry, cell.rz = voronoiCell(s.g.sha, x0+cx, qy+cy, z0+cz)
						cell.ready = false
					}
				}
			}
			lastY, haveY = qy, true
		}
		dy := int64(y&3) * 10240
		for iz := range r.SizeZ {
			z := r.Z + iz*r.StepXZ - 2
			qz, dz := z>>2, int64(z&3)*10240
			for ix := range r.SizeX {
				if i&63 == 0 && ctx.Err() != nil {
					return true, ctx.Err()
				}
				x := r.X + ix*r.StepXZ - 2
				qx, dx := x>>2, int64(x&3)*10240
				base := (qz-z0)*w + qx - x0
				best, winner := int64(1<<63-1), 0
				for corner := range 8 {
					bx, by, bz := (corner>>2)&1, (corner>>1)&1, corner&1
					cell := &cells[base+bx+by*plane+bz*w]
					rx := cell.rx + dx - 40960*int64(bx)
					ry := cell.ry + dy - 40960*int64(by)
					rz := cell.rz + dz - 40960*int64(bz)
					if d := rx*rx + ry*ry + rz*rz; d < best {
						best, winner = d, corner
					}
				}
				bx, by, bz := (winner>>2)&1, (winner>>1)&1, winner&1
				cell := &cells[base+bx+by*plane+bz*w]
				if !cell.ready {
					x, y, z := (qx+bx)*4, (qy+by)*4, (qz+bz)*4
					var ok bool
					// Custom sources resolve the full biome (and any injection)
					// themselves; their climate router has no parameter tree.
					if s.g.climate != nil && s.g.custom == nil {
						cell.point, ok = s.g.climate.target(&s.scratch, x, y, z)
						if s.tb != nil {
							cell.region = s.tb.region(s.g.tb, len(s.tb.layers)-1, int32(qx+bx), int32(qz+bz))
						}
					} else {
						cell.id, ok = s.NextNoiseBiomeID(x, y, z)
					}
					if !ok {
						return true, errors.New("invalid climate sample")
					}
					cell.ready = true
				}
				id := cell.id
				if s.g.custom != nil {
				} else if s.tb != nil {
					id = s.terraBlenderClimate(&cell.point, cell.region)
				} else if c := s.g.climate; c != nil {
					s.previous = c.root.search(&cell.point, s.previous)
					id = s.g.ids[c.root.nodes[s.previous].biome]
				}
				if s.g.injection != nil && s.g.custom == nil {
					id = s.injectBiome(id, &cell.point, (qx+bx)*4, (qy+by)*4, (qz+bz)*4)
				}
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
