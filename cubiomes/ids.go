package cubiomes

import "context"

// BiomeID uses cubiomes' stable numeric IDs for vanilla biomes. Custom registry
// names receive IDs starting at 256, local to their Generator.
type BiomeID uint16

var builtinBiomeIDs = func() map[string]BiomeID {
	m := make(map[string]BiomeID)
	for id, name := range legacyBiomeNames {
		if name != "" {
			m[name] = BiomeID(id)
		}
	}
	for id := 0; id < 256; id++ {
		if name := BiomeName(id); name != "" {
			m[name] = BiomeID(id)
		}
	}
	return m
}()

func (g *Generator) initBiomeIDs() {
	g.names = make([]string, 256)
	if g.legacy != nil {
		copy(g.names, legacyBiomeNames[:])
	} else {
		for i := range 256 {
			g.names[i] = BiomeName(i)
		}
	}
	biomes := g.biomes
	if g.climate != nil && g.climate.root != nil {
		biomes = g.climate.root.biomes
	}
	g.ids = make([]BiomeID, len(biomes))
	for i, name := range biomes {
		id, ok := builtinBiomeIDs[name]
		// A datapack may use a retired vanilla name alongside its replacement.
		// Those are distinct registry entries, despite sharing a historical ID.
		if ok && g.names[id] != "" && g.names[id] != name {
			ok = false
		}
		if !ok {
			id = BiomeID(len(g.names))
			g.names = append(g.names, name)
		}
		g.ids[i] = id
		g.names[id] = name
	}
}

// BiomeName resolves a numeric ID in this Generator, including custom biomes.
// Unknown IDs return an empty string. Historical names use cubiomes' 1.17 names.
func (g *Generator) BiomeName(id BiomeID) string {
	if int(id) >= len(g.names) {
		return ""
	}
	return g.names[id]
}

// BiomePalette returns an owned table indexed by BiomeID. Empty entries are
// unused IDs. Raster consumers can derive a color or metadata lookup once,
// avoiding string-keyed map lookups in their per-cell loops.
func (g *Generator) BiomePalette() []string {
	return append([]string(nil), g.names...)
}

// NoiseBiomeID is NoiseBiome without constructing or returning biome names.
func (s *Sampler) NoiseBiomeID(x, y, z int) (BiomeID, bool) {
	s.Reset()
	return s.NextNoiseBiomeID(x, y, z)
}

// NextNoiseBiomeID retains climate lookup history, like NextNoiseBiome.
func (s *Sampler) NextNoiseBiomeID(x, y, z int) (BiomeID, bool) {
	if s.g.custom != nil {
		id, ok := s.customBiome(x, y, z)
		if !ok || s.g.injection == nil {
			return id, ok
		}
		p, ok := s.g.climate.target(&s.customRouter.scratch, x, y, z)
		if !ok {
			return 0, false
		}
		return s.injectBiome(id, &p, x, y, z), true
	}
	if s.g.injection != nil {
		c := s.g.climate
		p, ok := c.target(&s.scratch, x, y, z)
		if !ok {
			return 0, false
		}
		var id BiomeID
		if s.tb != nil {
			region := s.tb.region(s.g.tb, len(s.tb.layers)-1, int32(x>>2), int32(z>>2))
			id = s.terraBlenderClimate(&p, region)
		} else {
			s.previous = c.root.search(&p, s.previous)
			id = s.g.ids[c.root.nodes[s.previous].biome]
		}
		return s.injectBiome(id, &p, x, y, z), true
	}
	if s.tb != nil {
		return s.terraBlenderBiome(x, y, z)
	}
	if s.tbEnd != nil {
		return s.terraBlenderEndBiome(x, y, z)
	}
	if l := s.g.legacy; l != nil {
		return l.noiseBiome(s, x, y, z), true
	}
	if c := s.g.climate; c != nil {
		p, ok := c.target(&s.scratch, x, y, z)
		if !ok {
			return 0, false
		}
		s.previous = c.root.search(&p, s.previous)
		return s.g.ids[c.root.nodes[s.previous].biome], true
	}
	return s.g.end.biomeID(x, y, z, &s.scratch)
}

// BiomeAtID applies the release's block-biome zoom before a fresh lookup.
func (s *Sampler) BiomeAtID(x, y, z int) (BiomeID, bool) {
	if s.beta != nil {
		return s.beta.biome(x, z, false), true
	}
	x, y, z = s.voronoi(x, y, z)
	return s.NoiseBiomeID(x*4, y*4, z*4)
}

// GenerateIDs fills caller-owned numeric output using the same coordinates,
// raster order, cancellation and history rules as Generate.
func (s *Sampler) GenerateIDs(ctx context.Context, r Range, dst []BiomeID) error {
	return s.generate(ctx, r, nil, dst)
}
