package cubiomes

// EndIslands samples vanilla's end_islands density function
// (EndIslandDensityFunction) for one seed. Not safe for concurrent use.
type EndIslands struct {
	n *endIslands
	p densitySample
}

// NewEndIslands builds the End island field for a world seed.
func NewEndIslands(seed int64) *EndIslands { return &EndIslands{n: newEndIslands(seed)} }

// Density is end_islands at block (x, z): (height-8)/128, positive on land.
// Java's integer division by 8 is kept.
func (e *EndIslands) Density(x, z int) float64 {
	return (float64(e.n.height(x/8, z/8, &e.p)) - 8) / 128
}
