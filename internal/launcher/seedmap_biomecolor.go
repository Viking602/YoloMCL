package launcher

import "image/color"

// BiomeColor is the 2D seed map's colour for a biome, so other views (the
// 3D structure preview's ground) match the map.
func (s *LocalSampler) BiomeColor(id string) color.RGBA { return s.biomeColor(id) }
