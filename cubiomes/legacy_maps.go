package cubiomes

type legacyNeighbors struct{ c, n, e, w, s, nw, ne, sw, se int32 }

func (n legacyNeighbors) has(v int32) bool { return n.n == v || n.e == v || n.w == v || n.s == v }
func (n legacyNeighbors) ocean() bool {
	return legacyOcean(n.n) || legacyOcean(n.e) || legacyOcean(n.w) || legacyOcean(n.s)
}

func (l *legacyLayer) neighbor(n legacyNeighbors, x, z int) int32 {
	v := n.c
	switch l.kind {
	case layerLand:
		corners := [4]int32{n.nw, n.ne, n.sw, n.se}
		allOcean := n.nw == 0 && n.ne == 0 && n.sw == 0 && n.se == 0
		anyOcean := n.nw == 0 || n.ne == 0 || n.sw == 0 || n.se == 0
		if l.mc == mcB1_8 {
			if v == 0 && !allOcean {
				v = int32(legacyInt(legacyChunkSeed(l.start, x, z), 3) / 2)
			} else if v == 1 && (n.nw != 1 || n.ne != 1 || n.sw != 1 || n.se != 1) {
				v = 1 - int32(legacyInt(legacyChunkSeed(l.start, x, z), 5)/4)
			}
		} else if v == 0 && !allOcean {
			cs := legacyChunkSeed(l.start, x, z)
			count := 0
			v = 1
			for _, a := range corners {
				if a != 0 {
					count++
					if legacyInt(cs, count) == 0 {
						v = a
					}
					cs = voronoiStep(cs, l.salt)
				}
			}
			if (l.mc <= mc1_6 || v != biomeForest) && legacyInt(cs, 3) != 0 {
				if l.mc <= mc1_6 && v == biomeSnowyTundra {
					v = biomeFrozenOcean
				} else {
					v = biomeOcean
				}
			}
		} else if v != 0 && anyOcean && (l.mc <= mc1_6 || v != biomeForest) {
			if legacyInt(legacyChunkSeed(l.start, x, z), 5) == 0 {
				if l.mc <= mc1_6 && v == biomeSnowyTundra {
					v = biomeFrozenOcean
				} else {
					v = biomeOcean
				}
			}
		}
	case layerIsland:
		if v == 0 && n.n == 0 && n.e == 0 && n.w == 0 && n.s == 0 && legacyInt(legacyChunkSeed(l.start, x, z), 2) == 0 {
			v = 1
		}
	case layerSnow:
		if l.mc <= mc1_6 {
			if v != 0 {
				v = biomePlains
				if legacyInt(legacyChunkSeed(l.start, x, z), 5) == 0 {
					v = biomeSnowyTundra
				}
			}
		} else if !legacyShallow(v) {
			r := legacyInt(legacyChunkSeed(l.start, x, z), 6)
			switch {
			case r == 0:
				v = 4
			case r == 1:
				v = 3
			default:
				v = 1
			}
		}
	case layerCool:
		if v == 1 && (n.has(3) || n.has(4)) {
			v = 2
		}
	case layerHeat:
		if v == 4 && (n.has(1) || n.has(2)) {
			v = 3
		}
	case layerMushroom:
		if v == 0 && n.nw == 0 && n.ne == 0 && n.sw == 0 && n.se == 0 && legacyInt(legacyChunkSeed(l.start, x, z), 100) == 0 {
			v = biomeMushroomFields
		}
	case layerDeep:
		if legacyShallow(v) && legacyShallow(n.n) && legacyShallow(n.e) && legacyShallow(n.w) && legacyShallow(n.s) {
			v = legacyDeepen(v)
		}
	case layerEdge:
		switch v {
		case biomeWoodedBadlandsPlateau, biomeBadlandsPlateau, biomeGiantTreeTaiga:
			if !legacySimilar(l.mc, n.n, v) || !legacySimilar(l.mc, n.e, v) || !legacySimilar(l.mc, n.w, v) || !legacySimilar(l.mc, n.s, v) {
				if v == biomeGiantTreeTaiga {
					v = biomeTaiga
				} else {
					v = biomeBadlands
				}
			}
		case biomeDesert:
			if n.has(biomeSnowyTundra) {
				v = biomeWoodedMountains
			}
		case biomeSwamp:
			if n.has(biomeDesert) || n.has(biomeSnowyTaiga) || n.has(biomeSnowyTundra) {
				v = biomePlains
			} else if n.has(biomeJungle) || n.has(biomeBambooJungle) {
				v = biomeJungleEdge
			}
		}
	case layerShore:
		v = l.shore(n)
	}
	return v
}

func legacyRiverID(v int32) int32 {
	if v >= 2 {
		return 2 + (v & 1)
	}
	return v
}

var legacyWarmBiomes = [6]int32{biomeDesert, biomeDesert, biomeDesert, biomeSavanna, biomeSavanna, biomePlains}
var legacyLushBiomes = [6]int32{biomeForest, biomeDarkForest, biomeMountains, biomePlains, biomeBirchForest, biomeSwamp}
var legacyColdBiomes = [4]int32{biomeForest, biomeMountains, biomeTaiga, biomePlains}
var legacySnowBiomes = [4]int32{biomeSnowyTundra, biomeSnowyTundra, biomeSnowyTundra, biomeSnowyTaiga}
var legacyOldBiomes = [7]int32{biomeDesert, biomeForest, biomeMountains, biomeSwamp, biomePlains, biomeTaiga, biomeJungle}

func (l *legacyLayer) pickBiome(id int32, cs uint64) int32 {
	special := id&0xf00 != 0
	id &^= 0xf00
	if l.mc <= mc1_6 {
		if id == biomeOcean || id == biomeMushroomFields {
			return id
		}
		n := 7
		if l.mc <= mc1_1 {
			n = 6
		}
		v := legacyOldBiomes[legacyInt(cs, n)]
		if id != biomePlains && (v != biomeTaiga || l.mc <= mc1_2) {
			v = biomeSnowyTundra
		}
		return v
	}
	if legacyOcean(id) || id == biomeMushroomFields {
		return id
	}
	switch id {
	case 1:
		if special {
			if legacyInt(cs, 3) == 0 {
				return biomeBadlandsPlateau
			}
			return biomeWoodedBadlandsPlateau
		}
		return legacyWarmBiomes[legacyInt(cs, 6)]
	case 2:
		if special {
			return biomeJungle
		}
		return legacyLushBiomes[legacyInt(cs, 6)]
	case 3:
		if special {
			return biomeGiantTreeTaiga
		}
		return legacyColdBiomes[legacyInt(cs, 4)]
	case 4:
		return legacySnowBiomes[legacyInt(cs, 4)]
	}
	return biomeMushroomFields
}

func (l *legacyLayer) hill(n legacyNeighbors, noise int32, x, z int) int32 {
	a := n.c
	bn := int32(-1)
	if l.mc >= mc1_7 {
		bn = (noise - 2) % 29
	}
	if bn == 1 && noise >= 2 && !legacyShallow(a) {
		if m := legacyMutated(l.mc, a); m > 0 {
			return m
		}
		return a
	}
	cs := legacyChunkSeed(l.start, x, z)
	if bn != 0 && legacyInt(cs, 3) != 0 {
		return a
	}
	hill := a
	switch a {
	case biomeDesert:
		hill = biomeDesertHills
	case biomeForest:
		hill = biomeWoodedHills
	case biomeBirchForest:
		hill = biomeBirchForestHills
	case biomeDarkForest:
		hill = biomePlains
	case biomeTaiga:
		hill = biomeTaigaHills
	case biomeGiantTreeTaiga:
		hill = biomeGiantTreeTaigaHills
	case biomeSnowyTaiga:
		hill = biomeSnowyTaigaHills
	case biomePlains:
		hill = biomeForest
		if l.mc >= mc1_7 {
			cs = voronoiStep(cs, l.salt)
			if legacyInt(cs, 3) == 0 {
				hill = biomeWoodedHills
			}
		}
	case biomeSnowyTundra:
		hill = biomeSnowyMountains
	case biomeJungle:
		hill = biomeJungleHills
	case biomeBambooJungle:
		hill = biomeBambooJungleHills
	case biomeOcean:
		if l.mc >= mc1_7 {
			hill = biomeDeepOcean
		}
	case biomeMountains:
		if l.mc >= mc1_7 {
			hill = biomeWoodedMountains
		}
	case biomeSavanna:
		hill = biomeSavannaPlateau
	default:
		if legacySimilar(l.mc, a, biomeWoodedBadlandsPlateau) {
			hill = biomeBadlands
		} else if legacyDeep(a) {
			cs = voronoiStep(cs, l.salt)
			if legacyInt(cs, 3) == 0 {
				cs = voronoiStep(cs, l.salt)
				hill = biomeForest
				if legacyInt(cs, 2) == 0 {
					hill = biomePlains
				}
			}
		}
	}
	if bn == 0 && hill != a {
		hill = legacyMutated(l.mc, hill)
		if hill < 0 {
			hill = a
		}
	}
	if hill == a {
		return a
	}
	equal := 0
	for _, v := range [4]int32{n.n, n.e, n.w, n.s} {
		if legacySimilar(l.mc, v, a) {
			equal++
		}
	}
	minimum := 3
	if l.mc <= mc1_6 {
		minimum = 4
	}
	if equal >= minimum {
		return hill
	}
	return a
}

func (l *legacyLayer) shore(n legacyNeighbors) int32 {
	v := n.c
	if v == biomeMushroomFields {
		if n.has(biomeOcean) {
			return biomeMushroomFieldShore
		}
		return v
	}
	if l.mc <= mc1_0 {
		return v
	}
	if l.mc <= mc1_6 {
		if v == biomeMountains {
			if n.n != v || n.e != v || n.w != v || n.s != v {
				return biomeMountainEdge
			}
		} else if v != biomeOcean && v != biomeRiver && v != biomeSwamp && n.has(biomeOcean) {
			return biomeBeach
		}
		return v
	}
	if legacyCategory(l.mc, v) == biomeJungle {
		for _, a := range [4]int32{n.n, n.e, n.w, n.s} {
			if legacyCategory(l.mc, a) != biomeJungle && a != biomeForest && a != biomeTaiga && !legacyOcean(a) {
				return biomeJungleEdge
			}
		}
		if n.ocean() {
			return biomeBeach
		}
		return v
	}
	if v == biomeMountains || v == biomeWoodedMountains {
		if n.ocean() {
			return biomeStoneShore
		}
		return v
	}
	if legacySnowy(v) {
		if !legacyOcean(v) && n.ocean() {
			return biomeSnowyBeach
		}
		return v
	}
	if v == biomeBadlands || v == biomeWoodedBadlandsPlateau {
		if !n.ocean() && (!legacyMesa(n.n) || !legacyMesa(n.e) || !legacyMesa(n.w) || !legacyMesa(n.s)) {
			return biomeDesert
		}
		return v
	}
	if v != biomeOcean && v != biomeDeepOcean && v != biomeRiver && v != biomeSwamp && n.ocean() {
		return biomeBeach
	}
	return v
}

func (l *legacyLayer) oceanMix(s *legacyScratch, out []int32, x, z, w, h int) error {
	if err := l.p2.area(s, out, x, z, w, h); err != nil {
		return err
	}
	x0, z0, x1, z1 := 0, 0, w, h
	for j := range h {
		if j&31 == 0 {
			if err := s.ctx.Err(); err != nil {
				return err
			}
		}
		for i := range w {
			if out[j*w+i] == biomeWarmOcean || out[j*w+i] == biomeFrozenOcean {
				x0 = min(x0, i-8)
				z0 = min(z0, j-8)
				x1 = max(x1, i+9)
				z1 = max(z1, j+9)
			}
		}
	}
	lw, lh := x1-x0, z1-z0
	land := s.take(lw * lh)
	if err := l.p.area(s, land, x+x0, z+z0, lw, lh); err != nil {
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
			a := land[(j-z0)*lw+i-x0]
			if !legacyOcean(a) {
				out[idx] = a
				continue
			}
			replacement := int32(0)
			if v == biomeWarmOcean {
				replacement = biomeLukewarmOcean
			}
			if v == biomeFrozenOcean {
				replacement = biomeColdOcean
			}
			nearLand := false
			if replacement != 0 {
				for dx := -8; dx <= 8 && !nearLand; dx += 4 {
					for dz := -8; dz <= 8; dz += 4 {
						if !legacyOcean(land[(j+dz-z0)*lw+i+dx-x0]) {
							nearLand = true
							break
						}
					}
				}
			}
			if nearLand {
				v = replacement
			} else if a == biomeDeepOcean && v != biomeWarmOcean {
				v = legacyDeepen(v)
			}
			out[idx] = v
		}
	}
	return nil
}
