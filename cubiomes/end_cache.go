package cubiomes

import (
	"math"
	"math/bits"
)

// Each row is a sliding, coordinate-checked interval of at most 64 factors.
// The occupancy mask lets height queries skip empty islands in one operation.
type endFactorRow struct {
	x0, x1, z int
	occupied  uint64
	values    [64]float32
	valid     bool
}

func (n *endIslands) factor(x, z int, legacy bool) float32 {
	if int64(x)*int64(x)+int64(z)*int64(z) <= 4096 || n.simplex(float64(x), float64(z)) >= float64(float32(-.9)) {
		return 0
	}
	v := float32(math.Abs(float64(float32(x))))*3439 + float32(math.Abs(float64(float32(z))))*147
	if legacy {
		return float32(uint32(uint64(v))%13 + 9)
	}
	return float32(math.Mod(float64(v), 13)) + 9
}

func (n *endIslands) factorRow(cache *endCache, x0, x1, z int, legacy bool) (*endFactorRow, uint64) {
	r := &cache.rows[z&63]
	if !r.valid || r.z != z || x0 > r.x1 || x1 < r.x0 {
		*r = endFactorRow{x0: x0, x1: x0, z: z, valid: true}
	}
	fill := func(lo, hi int) {
		for x := lo; x < hi; x++ {
			v := n.factor(x, z, legacy)
			r.values[x&63] = v
			mask := uint64(1) << (x & 63)
			r.occupied &^= mask
			if v != 0 {
				r.occupied |= mask
			}
		}
	}
	if x0 < r.x0 {
		fill(x0, r.x0)
		r.x0 = x0
		r.x1 = min(r.x1, x0+64)
	}
	if x1 > r.x1 {
		fill(r.x1, x1)
		r.x1 = x1
		r.x0 = max(r.x0, x1-64)
	}
	mask := bits.RotateLeft64(r.occupied, -(x0&63)) & ((uint64(1) << (x1 - x0)) - 1)
	return r, mask
}
