package cubiomes

// RandomSpread describes a vanilla linear or triangular placement grid.
// Positions are potential start chunks; biome, frequency, exclusion and terrain
// eligibility remain the caller's responsibility.
type RandomSpread struct {
	Spacing, Separation, Salt int
	Triangular                bool
}

// Candidate returns the block-coordinate center of a candidate chunk in a
// region. False indicates invalid parameters or block coordinates outside int32.
func (p RandomSpread) Candidate(seed int64, regionX, regionZ int) (x, z int, ok bool) {
	if p.Spacing < 1 || p.Spacing > 4096 || p.Separation < 0 || p.Separation >= p.Spacing || p.Salt < 0 || int64(p.Salt) > 1<<31-1 ||
		int64(regionX) < -1<<31 || int64(regionX) > 1<<31-1 || int64(regionZ) < -1<<31 || int64(regionZ) > 1<<31-1 {
		return 0, 0, false
	}
	seed += int64(regionX)*341873128712 + int64(regionZ)*132897987541 + int64(p.Salt)
	// Keep the LCG on the stack: the mixed legacy/xoroshiro constructor is too
	// large to inline and would allocate once per candidate.
	r := noiseRandom{lo: (uint64(seed) ^ 0x5deece66d) & ((1 << 48) - 1), legacy: true}
	offset := func() int {
		n := r.nextInt(p.Spacing - p.Separation)
		if p.Triangular {
			n = (n + r.nextInt(p.Spacing-p.Separation)) / 2
		}
		return n
	}
	x64 := (int64(regionX)*int64(p.Spacing)+int64(offset()))*16 + 8
	z64 := (int64(regionZ)*int64(p.Spacing)+int64(offset()))*16 + 8
	if x64 < -1<<31 || x64 > 1<<31-1 || z64 < -1<<31 || z64 > 1<<31-1 {
		return 0, 0, false
	}
	return int(x64), int(z64), true
}
