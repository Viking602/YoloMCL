package mcworld

// Heightmap returns the first free Y of a saved heightmap (WORLD_SURFACE,
// OCEAN_FLOOR, ...) at chunk-local x, z. minY is the dimension's lowest Y.
func (c *Chunk) Heightmap(kind string, x, z, minY int) (int, bool) {
	hm, _ := c.Root["Heightmaps"].(map[string]any)
	data, _ := hm[kind].([]int64)
	if len(data) == 0 {
		return 0, false
	}
	// 256 entries packed without spanning longs; the width follows the
	// dimension height (9 bits for 384).
	b := 0
	for w := 1; w <= 32; w++ {
		if (256+64/w-1)/(64/w) == len(data) {
			b = w
			break
		}
	}
	if b == 0 {
		return 0, false
	}
	per := 64 / b
	idx := (z&15)*16 + (x & 15)
	w := idx / per
	if w >= len(data) {
		return 0, false
	}
	v := int((uint64(data[w]) >> (uint(idx%per) * uint(b))) & (1<<uint(b) - 1))
	return v + minY, true
}
