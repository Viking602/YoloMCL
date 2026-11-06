package cubiomes

import "math"

func (n *densityNode) findTopSurface(p *densitySample) float64 {
	upper := n.b.eval(p)
	if math.IsNaN(upper) || math.IsInf(upper, 0) {
		return math.NaN()
	}
	top := math.Floor(upper/n.q) * n.q
	// Bound work for custom graphs with unbounded upper-bound expressions.
	// Ordinary dimension heights take at most a few hundred samples.
	if (top-n.p)/n.q > 32768 {
		return math.NaN()
	}
	for y := top; y >= n.p; y -= n.q {
		if p.evalAt(n.a, p.x, y, p.z) > 0 {
			return y
		}
	}
	return n.p
}
