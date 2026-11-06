package cubiomes

// Beta 1.7 exposes its actual climate biomes (cubiomes NO_BETA_OCEAN mode).
// Sea-level terrain estimation is a separate visualization, not a biome source.
type betaOctave struct {
	noise                 *improvedNoise
	lacunarity, amplitude float64
}
type betaNoise []betaOctave
type betaGenerator struct{ climate [3]betaNoise }

func newBetaNoise(r *noiseRandom, count int, frequency, frequencyMul float64) betaNoise {
	n := make(betaNoise, count)
	amplitude := .55
	for i := range n {
		n[i] = betaOctave{newImprovedNoise(r), frequency, amplitude}
		frequency *= frequencyMul
		amplitude *= 2
	}
	return n
}

func newBetaGenerator(seed int64) *betaGenerator {
	b := &betaGenerator{}
	b.climate[0] = newBetaNoise(newNoiseRandom(seed*9871, true), 4, .025/1.5, .25)
	b.climate[1] = newBetaNoise(newNoiseRandom(seed*39811, true), 4, .05/1.5, 1.0/3)
	b.climate[2] = newBetaNoise(newNoiseRandom(seed*0x84a59, true), 2, .25/1.5, 10.0/17)
	return b
}

func (n betaNoise) simplex(x, z float64) float64 {
	v := 0.0
	for _, o := range n {
		xx := x*o.lacunarity + o.noise.x
		zz := z*o.lacunarity + o.noise.y
		v += o.amplitude * simplex2D(&o.noise.p, xx, zz)
	}
	return v
}

func (b *betaGenerator) biome(x, z int, quart bool) BiomeID {
	if quart {
		x = (x>>2)*4 + 2
		z = (z>>2)*4 + 2
	}
	f := b.climate[2].simplex(float64(x), float64(z))*1.1 + .5
	t := (b.climate[0].simplex(float64(x), float64(z))*.15+.7)*.99 + f*.01
	t = 1 - (1-t)*(1-t)
	t = max(0, min(1, t))
	h := (b.climate[1].simplex(float64(x), float64(z))*.15+.5)*.998 + f*.002
	h = max(0, min(1, h))
	index := int(float32(t)*63) + 64*int(float32(h)*63)
	biomes := [10]BiomeID{biomePlains, biomeDesert, biomeForest, biomeTaiga, biomeSwamp, biomeSnowyTundra, biomeSavanna, biomeSeasonalForest, biomeRainforest, biomeShrubland}
	return biomes[betaBiomeTable[index]-'0']
}
