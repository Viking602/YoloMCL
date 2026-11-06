package cubiomes

import "math"

// Single-precision lattice sampling follows the MIT-licensed C2ME 26.3 math
// module. Coordinates remain doubles; lattice offsets, fades and interpolation
// use float at every step. See data/latest-LICENSE-C2ME.txt and provenance.
func latestLerp(t, a, b float32) float32 { return a + float32(t*float32(b-a)) }
func latestSmooth(t float32) float32 {
	return float32(float32(t*t)*t) * float32(float32(t*float32(float32(t*6)-15))+10)
}

func (n *improvedNoise) latestSample(x, y, z float64) float32 {
	x, y, z = noiseWrap(x)+n.x, noiseWrap(y)+n.y, noiseWrap(z)+n.z
	ix, iy, iz := int(math.Floor(x)), int(math.Floor(y)), int(math.Floor(z))
	a, b, c := float32(x-math.Floor(x)), float32(y-math.Floor(y)), float32(z-math.Floor(z))
	p := func(i int) int { return int(n.p[i&255]) }
	grad := func(h int, x, y, z float32) float32 {
		g := &improvedGradients[h&15]
		return float32(float32(g[0])*x+float32(g[1])*y) + float32(g[2])*z
	}
	x0, x1 := p(ix), p(ix+1)
	xy00, xy01, xy10, xy11 := p(x0+iy), p(x0+iy+1), p(x1+iy), p(x1+iy+1)
	u, v, w := latestSmooth(a), latestSmooth(b), latestSmooth(c)
	return latestLerp(w,
		latestLerp(v, latestLerp(u, grad(p(xy00+iz), a, b, c), grad(p(xy10+iz), a-1, b, c)), latestLerp(u, grad(p(xy01+iz), a, b-1, c), grad(p(xy11+iz), a-1, b-1, c))),
		latestLerp(v, latestLerp(u, grad(p(xy00+iz+1), a, b, c-1), grad(p(xy10+iz+1), a-1, b, c-1)), latestLerp(u, grad(p(xy01+iz+1), a, b-1, c-1), grad(p(xy11+iz+1), a-1, b-1, c-1))))
}

// 26.3 sums interleaved octave components in float precision. Official vanilla
// noise definitions preserve the earlier octave amplitudes; the compiler only
// enables this conversion for the exact verified definitions.
func (n *normalNoise) latestSample(x, y, z float64) float32 {
	var result float32
	if n.spec != nil && n.spec.legacyNether {
		// The legacy Nether stack rounds each normalization factor before
		// multiplying, and visits the first complete stack before the second.
		// Keeping that order matters when the float climate is quantized.
		for stackIndex, stack := range []*perlinNoise{&n.first, &n.second} {
			frequency := stack.frequency
			if stackIndex != 0 {
				frequency *= 337.0 / 331.0
			}
			amplitude := float32(n.factor) * float32(stack.factor)
			for i, octave := range stack.octaves {
				if octave != nil {
					result += float32(amplitude * float32(stack.amplitudes[i]) * octave.latestSample(x*frequency, y*frequency, z*frequency))
				}
				frequency *= 2
				amplitude *= .5
			}
		}
		return result
	}
	frequency, amplitude := n.first.frequency, n.first.factor
	for i, first := range n.first.octaves {
		if first != nil {
			weight := float32(n.factor * float64(n.first.amplitudes[i]*amplitude))
			result += float32(weight * first.latestSample(x*frequency, y*frequency, z*frequency))
			if second := n.second.octaves[i]; second != nil {
				f := frequency * (337.0 / 331.0)
				result += float32(weight * second.latestSample(x*f, y*f, z*f))
			}
		}
		frequency *= 2
		amplitude *= .5
	}
	return result
}
