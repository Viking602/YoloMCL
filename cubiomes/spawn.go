package cubiomes

import (
	"context"
	"math"
)

// Position is a horizontal block position, without a terrain height.
type Position struct{ X, Z int }

// EstimateSpawn searches vanilla's spawn climate targets. It does not perform
// final surface or spawn-chunk checks. A nil position means no valid target.
func (g *Generator) EstimateSpawn(ctx context.Context) (*Position, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c := g.climate
	if c == nil || len(c.spawnTargets) == 0 {
		return nil, nil
	}
	p := g.NewSampler().scratch
	fitness := func(x, z int) int64 {
		target, ok := c.target(&p, x, 0, z)
		if !ok {
			return math.MaxInt64
		}
		target[4] = 0
		best := int64(math.MaxInt64)
		for _, node := range c.spawnTargets {
			best = min(best, node.distance(target))
		}
		d := float64(int64(x)*int64(x)+int64(z)*int64(z)) / (2500 * 2500)
		return best + int64(100000000*d*d)
	}
	x, z := 0, 0
	best := fitness(0, 0)
	for _, search := range [][2]float32{{2048, 512}, {512, 32}} {
		cx, cz := x, z
		angle, radius := float32(0), search[1]
		for radius <= search[0] {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			px, pz := cx+int(math.Sin(float64(angle))*float64(radius)), cz+int(math.Cos(float64(angle))*float64(radius))
			if value := fitness(px, pz); value < best {
				x, z, best = px, pz, value
			}
			angle += search[1] / radius
			if float64(angle) > math.Pi*2 {
				angle = 0
				radius += search[1]
			}
		}
	}
	if best == math.MaxInt64 {
		return nil, nil
	}
	return &Position{X: x, Z: z}, nil
}
