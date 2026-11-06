package cubiomes

// ReferenceImproved exposes one natively seeded ImprovedNoise: the vanilla
// field state (xo, yo, zo, p) and the native sampler that generation uses.
// It lets transformed Java noise classes be checked against native output.
type ReferenceImproved struct {
	Xo, Yo, Zo float64
	P          [256]byte
	Sample     func(x, y, z float64) float64
}

// ReferencePerlin exposes a natively seeded PerlinNoise with its vanilla
// field state (noiseLevels, amplitudes, lowestFreqInputFactor,
// lowestFreqValueFactor) and the native sampler.
type ReferencePerlin struct {
	Octaves                  []*ReferenceImproved
	Amplitudes               []float64
	InputFactor, ValueFactor float64
	Sample                   func(x, y, z float64) float64
}

func referenceImproved(n *improvedNoise) *ReferenceImproved {
	if n == nil {
		return nil
	}
	return &ReferenceImproved{Xo: n.x, Yo: n.y, Zo: n.z, P: n.p, Sample: n.sample}
}

func NewReferenceImproved(seed int64, legacy bool) *ReferenceImproved {
	return referenceImproved(newImprovedNoise(newNoiseRandom(seed, legacy)))
}

func NewReferencePerlin(seed int64, legacy bool, first int, amplitudes []float64) *ReferencePerlin {
	n := newPerlinNoise(newNoiseRandom(seed, legacy), first, append([]float64(nil), amplitudes...), false)
	out := &ReferencePerlin{Amplitudes: n.amplitudes, InputFactor: n.frequency, ValueFactor: n.factor, Sample: n.sample}
	for _, o := range n.octaves {
		out.Octaves = append(out.Octaves, referenceImproved(o))
	}
	return out
}
