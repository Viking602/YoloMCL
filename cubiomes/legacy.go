package cubiomes

import (
	"context"
	"errors"
	"math"
	"math/bits"
	"strconv"
	"strings"
)

func legacyRelease(version string) (int, bool) {
	if version == "b1.7" || version == "b1.7.1" || version == "b1.7.2" || version == "b1.7.3" {
		return mcB1_7, true
	}
	if version == "b1.8" || version == "b1.8.1" {
		return mcB1_8, true
	}
	p := strings.Split(version, ".")
	if len(p) < 2 || len(p) > 3 || p[0] != "1" {
		return 0, false
	}
	for _, s := range p {
		if s == "" {
			return 0, false
		}
		for _, c := range s {
			if c < '0' || c > '9' {
				return 0, false
			}
		}
	}
	m, e := strconv.Atoi(p[1])
	if e != nil || m < 0 || m > 17 {
		return 0, false
	}
	patch := 0
	if len(p) == 3 {
		patch, e = strconv.Atoi(p[2])
		if e != nil {
			return 0, false
		}
	}
	last := [18]int{0, 0, 5, 2, 7, 2, 4, 10, 9, 4, 2, 2, 2, 2, 4, 2, 5, 1}
	if patch > last[m] {
		return 0, false
	}
	versions := [18]int{mc1_0, mc1_1, mc1_2, mc1_3, mc1_4, mc1_5, mc1_6, mc1_7, mc1_8, mc1_9, mc1_10, mc1_11, mc1_12, mc1_13, mc1_14, mc1_15, mc1_16, mc1_17}
	if m == 16 && patch <= 1 {
		return mc1_16_1, true
	}
	return versions[m], true
}

type legacyGenerator struct {
	config                Config
	fixedSeed             bool
	mc                    int
	dimension             Dimension
	layers                *legacyLayers
	temperature, humidity *normalNoise
	end                   *endIslands
	beta                  *betaGenerator
}

func newLegacyGenerator(cfg Config, mc int) (*Generator, error) {
	if cfg.Dimension == End && mc < mc1_0 {
		return nil, errors.New("the End did not exist in this release")
	}
	if cfg.LargeBiomes && mc < mc1_3 {
		return nil, errors.New("large biomes require Minecraft 1.3 or later")
	}
	l := &legacyGenerator{mc: mc, dimension: cfg.Dimension, config: cfg}
	g := &Generator{legacy: l, sha: seedSHA(cfg.Seed), seed: cfg.Seed}
	switch cfg.Dimension {
	case Overworld:
		if mc == mcB1_7 {
			l.beta = newBetaGenerator(cfg.Seed)
		} else {
			l.layers = newLegacyLayers(mc, cfg.Seed, cfg.LargeBiomes)
		}
		for id, name := range legacyBiomeNames {
			if legacyOverworldBiome(mc, id) {
				g.biomes = append(g.biomes, name)
			}
		}
	case Nether:
		g.biomes = []string{legacyBiomeNames[biomeNetherWastes]}
		if mc >= mc1_16_1 {
			l.temperature = newNormalNoise(newNoiseRandom(cfg.Seed, true), -7, []float64{1, 1}, true)
			l.humidity = newNormalNoise(newNoiseRandom(cfg.Seed+1, true), -7, []float64{1, 1}, true)
			for id := biomeSoulSandValley; id <= biomeBasaltDeltas; id++ {
				g.biomes = append(g.biomes, legacyBiomeNames[id])
			}
		}
	case End:
		g.biomes = []string{legacyBiomeNames[biomeTheEnd]}
		if mc >= mc1_9 {
			l.end = newEndIslands(cfg.Seed)
			for id := biomeSmallEndIslands; id <= biomeEndBarrens; id++ {
				g.biomes = append(g.biomes, legacyBiomeNames[id])
			}
		}
	}
	g.initBiomeIDs()
	return g, nil
}

func legacyOverworldBiome(mc, id int) bool {
	if mc == mcB1_7 {
		switch id {
		case 1, 2, 4, 5, 6, 12, 35, 51, 52, 53:
			return true
		}
		return false
	}
	if id < 0 || id >= len(legacyBiomeNames) || legacyBiomeNames[id] == "" {
		return false
	}
	if mc == mcB1_8 {
		switch id {
		case 10, 11, 12, 14, 15:
			return false
		}
	}
	if mc <= mc1_0 {
		switch id {
		case 13, 16, 17, 18, 19, 20:
			return false
		}
	}
	switch {
	case id == 8 || id == 9 || id == 40 || id == 41 || id == 42 || id == 43 || id == 47:
		return false
	case id == 10:
		return mc <= mc1_6 || mc >= mc1_13
	case id == 20:
		return mc <= mc1_6
	case id <= 20:
		return true
	case id <= 22:
		return mc >= mc1_2
	case id <= 39:
		return mc >= mc1_7
	case id >= 44 && id <= 50:
		return mc >= mc1_13
	case id == 155:
		return mc >= mc1_7 && (mc <= mc1_8 || mc >= mc1_11)
	case id >= 129 && id <= 167:
		return mc >= mc1_7
	case id == 168 || id == 169:
		return mc >= mc1_14
	}
	return false
}

func (l *legacyGenerator) noiseBiome(s *Sampler, x, y, z int) BiomeID {
	switch l.dimension {
	case Overworld:
		if l.beta != nil {
			return l.beta.biome(x, z, true)
		}
		s.legacy.used = 0
		s.legacy.ctx = context.Background()
		var out [1]int32
		_ = l.layers.root.area(&s.legacy, out[:], x>>2, z>>2, 1, 1)
		return BiomeID(out[0])
	case Nether:
		if l.temperature == nil {
			return biomeNetherWastes
		}
		t := float32(l.temperature.sample(float64(x>>2), 0, float64(z>>2)))
		h := float32(l.humidity.sample(float64(x>>2), 0, float64(z>>2)))
		points := [5][3]float32{{0, 0, 0}, {0, -.5, 0}, {.4, 0, 0}, {0, .5, .375 * .375}, {-.5, 0, .175 * .175}}
		ids := [5]BiomeID{biomeNetherWastes, biomeSoulSandValley, biomeCrimsonForest, biomeWarpedForest, biomeBasaltDeltas}
		best, dmin := 0, float32(math.MaxFloat32)
		for i, p := range points {
			dx, dy := p[0]-t, p[1]-h
			d := float32(float32(dx*dx)+float32(dy*dy)) + p[2]
			if d < dmin {
				best, dmin = i, d
			}
		}
		return ids[best]
	case End:
		if l.end == nil {
			return biomeTheEnd
		}
		return l.endBiome(s, x>>4, z>>4)
	}
	return 0
}

func (l *legacyGenerator) endBiome(s *Sampler, cx, cz int) BiomeID {
	if int64(cx)*int64(cx)+int64(cz)*int64(cz) <= 4096 {
		return biomeTheEnd
	}
	cache := s.scratch.endCache
	entry := &cache.biomes[(cx*31+cz*17)&255]
	if entry.id != 0 && entry.x == int32(cx) && entry.z == int32(cz) {
		return entry.id
	}
	x, z := 2*cx+1, 2*cz+1
	id := BiomeID(biomeSmallEndIslands)
	if l.mc > mc1_13 && int32(x)*int32(x)+int32(z)*int32(z) < 0 {
		id = biomeEndBarrens
	} else {
		h := int64(14401)
		if x >= -15 && x <= 15 && z >= -15 && z <= 15 {
			h = 64 * (int64(x)*int64(x) + int64(z)*int64(z))
		}
		hx, hz, ox, oz := x/2, z/2, x%2, z%2
		// Every factor is at least 9. An axis distance >=14 contributes at
		// least 14²*9²=15876, beyond the last biome threshold (14400).
		// Omitting these islands preserves all four classifications exactly.
		dx0, dz0 := -6, -6
		if ox < 0 {
			dx0--
		}
		if oz < 0 {
			dz0--
		}
		for dz := dz0; dz < dz0+14; dz++ {
			row, mask := l.end.factorRow(cache, hx+dx0, hx+dx0+14, hz+dz, true)
			for mask != 0 {
				dx := dx0 + bits.TrailingZeros64(mask)
				mask &= mask - 1
				a, b := int64(ox-dx*2), int64(oz-dz*2)
				f := int64(row.values[(hx+dx)&63])
				h = min(h, (a*a+b*b)*f*f)
			}
			if h < 3600 {
				break
			}
		}
		switch {
		case h < 3600:
			id = biomeEndHighlands
		case h <= 10000:
			id = biomeEndMidlands
		case h <= 14400:
			id = biomeEndBarrens
		}
	}
	*entry = endBiome{int32(cx), 0, int32(cz), id}
	return id
}

func (s *Sampler) voronoi(x, y, z int) (int, int, int) {
	l := s.g.legacy
	if l == nil {
		return voronoiAccess(s.g.sha, x, y, z)
	}
	if l.mc >= mc1_15 {
		return voronoiAccess(s.g.sha, x, y, z)
	}
	start, salt := uint64(0), uint64(0)
	if l.layers != nil {
		start, salt = l.layers.voronoiStart, l.layers.voronoiSalt
	} else {
		// The old End zoom never received the world seed.
		salt = 10
		for range 3 {
			salt = voronoiStep(salt, 10)
		}
	}
	x -= 2
	z -= 2
	px, pz := x>>2, z>>2
	var distance [4]int64
	for i := range 4 {
		dx, dz := i&1, i>>1
		cs := legacyChunkSeed(start, (px+dx)*4, (pz+dz)*4)
		a := int64((legacyInt(cs, 1024)-512)*36 + dx*40960 - (x&3)*10240)
		cs = voronoiStep(cs, salt)
		b := int64((legacyInt(cs, 1024)-512)*36 + dz*40960 - (z&3)*10240)
		distance[i] = a*a + b*b
	}
	best := 3
	for i := range 3 {
		less := true
		for j := range 4 {
			if i != j && distance[i] >= distance[j] {
				less = false
			}
		}
		if less {
			best = i
			break
		}
	}
	return px + (best & 1), y >> 2, pz + (best >> 1)
}
