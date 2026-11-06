package cubiomes

import (
	"math"
	"sort"
)

// NearestNoiseBiomes enumerates equal-fitness climate winners for diagnostics.
// Minecraft retains a thread's previous R-tree winner on ties, so the same
// coordinate can have multiple valid results depending on query history.
// This is deliberately independent of the fast sampler's previous winner.
func (s *Sampler) NearestNoiseBiomes(x, y, z int) ([]string, bool) {
	if s.g.climate == nil {
		return nil, false
	}
	p, ok := s.g.climate.target(&s.scratch, x, y, z)
	if !ok {
		return nil, false
	}
	closest := func(tree *climateTree) []uint16 {
		best := int64(math.MaxInt64)
		var leaves []uint16
		for i := range tree.nodes {
			n := &tree.nodes[i]
			if n.count != 0 {
				continue
			}
			d := n.distance(&p)
			if d < best {
				best = d
				leaves = leaves[:0]
			}
			if d == best {
				leaves = append(leaves, n.biome)
			}
		}
		return leaves
	}
	ids := map[BiomeID]bool{}
	if s.tb != nil {
		region, _ := s.TerraBlenderRegionIndex(x, z)
		tree := &s.g.tb.trees[region]
		for _, id := range closest(tree.tree) {
			if int(id) == tree.deferred {
				base := &s.g.tb.trees[0]
				for _, baseID := range closest(base.tree) {
					ids[base.ids[baseID]] = true
				}
			} else {
				ids[tree.ids[id]] = true
			}
		}
	} else {
		for _, id := range closest(s.g.climate.root) {
			ids[s.g.ids[id]] = true
		}
	}
	names := map[string]bool{}
	for id := range ids {
		names[s.g.BiomeName(s.injectBiome(id, &p, x, y, z))] = true
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, true
}
