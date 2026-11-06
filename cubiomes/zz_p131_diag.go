package cubiomes

// TerraBlenderRegionBiomes is a diagnostic: the selected region and the
// biome every region would give at (x, y, z), plus cumulative weights.
func (s *Sampler) TerraBlenderRegionBiomes(x, y, z int) (int, []string, []int, bool) {
	if s.tb == nil || s.g.climate == nil {
		return 0, nil, nil, false
	}
	p, ok := s.g.climate.target(&s.scratch, x, y, z)
	if !ok {
		return 0, nil, nil, false
	}
	region, _ := s.TerraBlenderRegionIndex(x, z)
	out := make([]string, len(s.g.tb.trees))
	for i := range s.g.tb.trees {
		t := &s.g.tb.trees[i]
		leaf := t.tree.search(&p, -1)
		b := int(t.tree.nodes[leaf].biome)
		if b == t.deferred {
			t0 := &s.g.tb.trees[0]
			leaf = t0.tree.search(&p, -1)
			out[i] = s.g.BiomeName(t0.ids[t0.tree.nodes[leaf].biome])
			continue
		}
		out[i] = s.g.BiomeName(t.ids[b])
	}
	return region, out, append([]int(nil), s.g.tb.weights...), true
}
