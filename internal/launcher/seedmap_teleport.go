package launcher

import "github.com/Viking602/YoloMCL/cubiomes"

// TeleportYAt returns a standing height above the noise-stage surface, including
// water. Roofed dimensions use a floor below nearY with two air blocks above it.
// Surface features, structures and later player edits are not part of this query.
func (s *LocalSampler) TeleportYAt(dimension string, x, z, nearY int) (int, bool) {
	if s == nil {
		return 0, false
	}
	d, ok := s.dimensions[dimension]
	if !ok {
		return 0, false
	}
	if d.terrain != nil {
		t := d.terrainPool.Get().(*cubiomes.TerrainSampler)
		defer d.terrainPool.Put(t)
		minY, maxY := d.terrain.MinY, d.terrain.MinY+d.terrain.Height-1
		if t.Block(x, maxY, z) == cubiomes.Solid {
			for y := min(maxY-2, nearY); y >= minY; y-- {
				if t.Block(x, y, z) != cubiomes.Air && t.Block(x, y+1, z) == cubiomes.Air && t.Block(x, y+2, z) == cubiomes.Air {
					return y + 1, true
				}
			}
			return 0, false
		}
		_, top := t.Column(x, z, nil)
		if top < minY || top+2 > maxY {
			return 0, false
		}
		return top + 2, true
	}
	if h, ok := s.SurfaceHeightAt(dimension, x, z); ok {
		// Older/unsupported density graphs can still expose the depth estimate.
		// Keep the default above sea level rather than at an ocean-floor estimate.
		return max(h, d.seaLevel) + 2, true
	}
	return 0, false
}
