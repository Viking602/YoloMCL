package cubiomes

import "context"

// The historical layer graph is immutable after construction. Work buffers
// belong to a Sampler, so different map workers never share mutable layer state.
type legacyLayer struct {
	kind        legacyLayerKind
	mc          int
	start, salt uint64
	p, p2       *legacyLayer
	noise       *improvedNoise
}

type legacyLayerKind uint8

const (
	layerContinent legacyLayerKind = iota
	layerFuzzy
	layerZoom
	layerLand
	layerIsland
	layerSnow
	layerCool
	layerHeat
	layerSpecial
	layerMushroom
	layerDeep
	layerBiome
	layerBamboo
	layerNoise
	layerEdge
	layerHills
	layerRiver
	layerSmooth
	layerSunflower
	layerShore
	layerSwampRiver
	layerRiverMix
	layerOceanTemp
	layerOceanMix
)

type legacyLayers struct {
	layers                    [64]legacyLayer
	n                         int
	root                      *legacyLayer
	voronoiSalt, voronoiStart uint64
}

func legacyLayerSeeds(seed, salt uint64) (uint64, uint64) {
	if salt == 0 {
		return 0, 0
	}
	ls := voronoiStep(salt, salt)
	ls = voronoiStep(ls, salt)
	ls = voronoiStep(ls, salt)
	for range 3 {
		seed = voronoiStep(seed, ls)
	}
	return voronoiStep(seed, 0), seed
}

func newLegacyLayers(mc int, seed int64, large bool) *legacyLayers {
	g := &legacyLayers{}
	add := func(kind legacyLayerKind, salt uint64, p, p2 *legacyLayer) *legacyLayer {
		l := &g.layers[g.n]
		g.n++
		l.kind, l.mc, l.p, l.p2 = kind, mc, p, p2
		l.start, l.salt = legacyLayerSeeds(uint64(seed), salt)
		return l
	}
	p := add(layerContinent, 1, nil, nil)
	p = add(layerFuzzy, 2000, p, nil)
	p = add(layerLand, 1, p, nil)
	p = add(layerZoom, 2001, p, nil)
	p = add(layerLand, 2, p, nil)
	if mc == mcB1_8 {
		p = add(layerZoom, 2002, p, nil)
		p = add(layerLand, 3, p, nil)
		p = add(layerZoom, 2003, p, nil)
		p = add(layerLand, 3, p, nil)
		p = add(layerZoom, 2004, p, nil)
		p = add(layerLand, 3, p, nil)
	} else if mc <= mc1_6 {
		p = add(layerSnow, 2, p, nil)
		p = add(layerZoom, 2002, p, nil)
		p = add(layerLand, 3, p, nil)
		p = add(layerZoom, 2003, p, nil)
		p = add(layerLand, 4, p, nil)
		p = add(layerMushroom, 5, p, nil)
	} else {
		p = add(layerLand, 50, p, nil)
		p = add(layerLand, 70, p, nil)
		p = add(layerIsland, 2, p, nil)
		p = add(layerSnow, 2, p, nil)
		p = add(layerLand, 3, p, nil)
		p = add(layerCool, 2, p, nil)
		p = add(layerHeat, 2, p, nil)
		p = add(layerSpecial, 3, p, nil)
		p = add(layerZoom, 2002, p, nil)
		p = add(layerZoom, 2003, p, nil)
		p = add(layerLand, 4, p, nil)
		p = add(layerMushroom, 5, p, nil)
		p = add(layerDeep, 4, p, nil)
	}
	base := p
	p = add(layerBiome, 200, p, nil)
	if mc >= mc1_14 {
		p = add(layerBamboo, 1001, p, nil)
	}
	p = add(layerZoom, 1000, p, nil)
	p = add(layerZoom, 1001, p, nil)
	if mc >= mc1_7 {
		p = add(layerEdge, 1000, p, nil)
	}
	noise := add(layerNoise, 100, base, nil)
	if mc > mc1_0 {
		hills := noise
		if mc <= mc1_12 {
			hills = add(layerZoom, 0, hills, nil)
			hills = add(layerZoom, 0, hills, nil)
		} else {
			hills = add(layerZoom, 1000, hills, nil)
			hills = add(layerZoom, 1001, hills, nil)
		}
		p = add(layerHills, 1000, p, hills)
	}
	if mc >= mc1_7 {
		p = add(layerSunflower, 1001, p, nil)
	}
	p = add(layerZoom, 1000, p, nil)
	p = add(layerLand, 3, p, nil)
	if mc <= mc1_0 {
		p = add(layerShore, 1000, p, nil)
	}
	p = add(layerZoom, 1001, p, nil)
	if mc >= mc1_1 {
		p = add(layerShore, 1000, p, nil)
		if mc <= mc1_6 {
			p = add(layerSwampRiver, 1000, p, nil)
		}
	}
	p = add(layerZoom, 1002, p, nil)
	p = add(layerZoom, 1003, p, nil)
	if large && mc >= mc1_3 {
		p = add(layerZoom, 1004, p, nil)
		p = add(layerZoom, 1005, p, nil)
	}
	p = add(layerSmooth, 1000, p, nil)
	rivers := add(layerZoom, 1000, noise, nil)
	rivers = add(layerZoom, 1001, rivers, nil)
	for i := 0; i < 4; i++ {
		salt := uint64(1000 + i)
		if mc <= mc1_6 {
			salt += 2
		}
		rivers = add(layerZoom, salt, rivers, nil)
	}
	if large && mc >= mc1_3 && mc <= mc1_7 {
		salt := uint64(1004)
		if mc <= mc1_6 {
			salt = 1006
		}
		rivers = add(layerZoom, salt, rivers, nil)
		rivers = add(layerZoom, salt+1, rivers, nil)
	}
	rivers = add(layerRiver, 1, rivers, nil)
	rivers = add(layerSmooth, 1000, rivers, nil)
	p = add(layerRiverMix, 100, p, rivers)
	if mc >= mc1_13 {
		ocean := add(layerOceanTemp, 2, nil, nil)
		ocean.noise = newImprovedNoise(newNoiseRandom(seed, true))
		for i := uint64(2001); i <= 2006; i++ {
			ocean = add(layerZoom, i, ocean, nil)
		}
		p = add(layerOceanMix, 100, p, ocean)
	}
	g.root = p
	g.voronoiStart, g.voronoiSalt = legacyLayerSeeds(uint64(seed), 10)
	return g
}

func legacyChunkSeed(start uint64, x, z int) uint64 {
	s := start + uint64(int64(x))
	s = voronoiStep(s, uint64(int64(z)))
	s = voronoiStep(s, uint64(int64(x)))
	return voronoiStep(s, uint64(int64(z)))
}

func legacyInt(seed uint64, n int) int {
	v := int((int64(seed) >> 24) % int64(n))
	if v < 0 {
		v += n
	}
	return v
}

type legacyScratch struct {
	buf  []int32
	used int
	ctx  context.Context
}

func (s *legacyScratch) take(n int) []int32 {
	start := s.used
	s.used += n
	if s.used > len(s.buf) {
		s.buf = append(s.buf, make([]int32, s.used-len(s.buf))...)
	}
	return s.buf[start:s.used]
}

func (l *legacyLayer) area(s *legacyScratch, out []int32, x, z, w, h int) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	mark := s.used
	defer func() { s.used = mark }()
	switch l.kind {
	case layerContinent:
		for j := range h {
			if j&31 == 0 {
				if err := s.ctx.Err(); err != nil {
					return err
				}
			}
			for i := range w {
				v := int32(0)
				if legacyInt(legacyChunkSeed(l.start, x+i, z+j), 10) == 0 {
					v = 1
				}
				out[j*w+i] = v
			}
		}
		if x > -w && x <= 0 && z > -h && z <= 0 {
			out[-z*w-x] = 1
		}
	case layerFuzzy, layerZoom:
		return l.zoom(s, out, x, z, w, h)
	case layerOceanTemp:
		for j := range h {
			if j&31 == 0 {
				if err := s.ctx.Err(); err != nil {
					return err
				}
			}
			for i := range w {
				t := l.noise.sample(float64(x+i)/8, float64(z+j)/8, 0)
				v := int32(biomeOcean)
				switch {
				case t > 0.4:
					v = biomeWarmOcean
				case t > 0.2:
					v = biomeLukewarmOcean
				case t < -0.4:
					v = biomeFrozenOcean
				case t < -0.2:
					v = biomeColdOcean
				}
				out[j*w+i] = v
			}
		}
	case layerOceanMix:
		return l.oceanMix(s, out, x, z, w, h)
	case layerRiverMix:
		if err := l.p.area(s, out, x, z, w, h); err != nil {
			return err
		}
		rivers := s.take(w * h)
		if err := l.p2.area(s, rivers, x, z, w, h); err != nil {
			return err
		}
		for i, v := range out[:w*h] {
			if i&4095 == 0 {
				if err := s.ctx.Err(); err != nil {
					return err
				}
			}
			if rivers[i] != biomeRiver || v == biomeOcean || (l.mc > mc1_6 && legacyOcean(v)) {
				continue
			}
			switch v {
			case biomeSnowyTundra:
				out[i] = biomeFrozenRiver
			case biomeMushroomFields, biomeMushroomFieldShore:
				out[i] = biomeMushroomFieldShore
			default:
				out[i] = biomeRiver
			}
		}
	case layerSpecial, layerBiome, layerBamboo, layerNoise, layerSunflower, layerSwampRiver:
		if err := l.p.area(s, out, x, z, w, h); err != nil {
			return err
		}
		for j := range h {
			if j&31 == 0 {
				if err := s.ctx.Err(); err != nil {
					return err
				}
			}
			for i := range w {
				idx := j*w + i
				v := out[idx]
				cs := legacyChunkSeed(l.start, x+i, z+j)
				switch l.kind {
				case layerSpecial:
					if v != 0 && legacyInt(cs, 13) == 0 {
						v |= int32(1+legacyInt(voronoiStep(cs, l.salt), 15)) << 8 & 0xf00
					}
				case layerBiome:
					v = l.pickBiome(v, cs)
				case layerBamboo:
					if v == biomeJungle && legacyInt(cs, 10) == 0 {
						v = biomeBambooJungle
					}
				case layerNoise:
					if v > 0 {
						n := 299999
						if l.mc <= mc1_6 {
							n = 2
						}
						v = int32(legacyInt(cs, n) + 2)
					} else {
						v = 0
					}
				case layerSunflower:
					if v == biomePlains && legacyInt(cs, 57) == 0 {
						v = biomeSunflowerPlains
					}
				case layerSwampRiver:
					if v == biomeSwamp {
						if legacyInt(cs, 6) == 0 {
							v = biomeRiver
						}
					} else if v == biomeJungle || v == biomeJungleHills {
						if legacyInt(cs, 8) == 0 {
							v = biomeRiver
						}
					}
				}
				out[idx] = v
			}
		}
	default:
		pw, ph := w+2, h+2
		p := s.take(pw * ph)
		if err := l.p.area(s, p, x-1, z-1, pw, ph); err != nil {
			return err
		}
		if l.kind == layerSmooth {
			return l.smoothArea(s, out, p, x, z, w, h)
		}
		if l.kind == layerRiver {
			return l.riverArea(s, out, p, w, h)
		}
		var rivers []int32
		if l.kind == layerHills {
			rivers = s.take(pw * ph)
			if err := l.p2.area(s, rivers, x-1, z-1, pw, ph); err != nil {
				return err
			}
		}
		for j := range h {
			if j&31 == 0 {
				if err := s.ctx.Err(); err != nil {
					return err
				}
			}
			for i := range w {
				k := (j+1)*pw + i + 1
				n := legacyNeighbors{p[k], p[k-pw], p[k+1], p[k-1], p[k+pw], p[k-pw-1], p[k-pw+1], p[k+pw-1], p[k+pw+1]}
				if l.kind == layerHills {
					out[j*w+i] = l.hill(n, rivers[k], x+i, z+j)
				} else {
					out[j*w+i] = l.neighbor(n, x+i, z+j)
				}
			}
		}
	}
	return nil
}

func legacyChoose4(cs, salt uint64, a, b, c, d int32, fuzzy bool) int32 {
	if !fuzzy {
		ca, cb, cc := 0, 0, 0
		if a == c {
			ca++
		}
		if a == b {
			ca++
		}
		if a == d {
			ca++
		}
		if c == b {
			cb++
		}
		if c == d {
			cb++
		}
		if b == d {
			cc++
		}
		if ca > cb && ca > cc {
			return a
		}
		if cb > ca {
			return c
		}
		if cc > ca {
			return b
		}
	}
	return [4]int32{a, c, b, d}[(voronoiStep(cs, salt)>>24)&3]
}

func (l *legacyLayer) zoom(s *legacyScratch, out []int32, x, z, w, h int) error {
	px, pz := x>>1, z>>1
	pw, ph := ((x+w-1)>>1)-px+2, ((z+h-1)>>1)-pz+2
	p := s.take(pw * ph)
	if err := l.p.area(s, p, px, pz, pw, ph); err != nil {
		return err
	}
	nw, nh := (pw-1)*2, (ph-1)*2
	buf := s.take(nw * nh)
	for j := 0; j < ph-1; j++ {
		if j&31 == 0 {
			if err := s.ctx.Err(); err != nil {
				return err
			}
		}
		p0, p1 := p[j*pw:(j+1)*pw], p[(j+1)*pw:(j+2)*pw]
		upper, lower := buf[j*2*nw:(j*2+1)*nw], buf[(j*2+1)*nw:(j*2+2)*nw]
		for i := 0; i < pw-1; i++ {
			a, b, c, d := p0[i], p1[i], p0[i+1], p1[i+1]
			right, down, diagonal := a, a, a
			if a != b || a != c || a != d {
				cs := legacyChunkSeed(l.start, (i+px)*2, (j+pz)*2)
				if (cs>>24)&1 != 0 {
					down = b
				}
				cs = voronoiStep(cs, l.salt)
				if (cs>>24)&1 != 0 {
					right = c
				}
				diagonal = legacyChoose4(cs, l.salt, a, b, c, d, l.kind == layerFuzzy)
			}
			u, v := upper[i*2:i*2+2], lower[i*2:i*2+2]
			u[0], u[1], v[0], v[1] = a, right, down, diagonal
		}
	}
	for j := range h {
		start := (j+(z&1))*nw + (x & 1)
		copy(out[j*w:(j+1)*w], buf[start:start+w])
	}
	return nil
}
