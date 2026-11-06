package launcher

import (
	"context"
	"encoding/json"

	"github.com/Viking602/YoloMCL/cubiomes"
)

// surfaceRuleCategory maps a dimension to TerraBlender's RuleCategory.
func surfaceRuleCategory(dimension string) string {
	switch dimension {
	case "minecraft:overworld":
		return "OVERWORLD"
	case "minecraft:the_nether":
		return "NETHER"
	}
	return ""
}

// compileSurfaceRules compiles mod surface rules traced from TerraBlender
// SurfaceRuleManager.addSurfaceRules, in Forge mod order (a later namespace
// registration replaces an earlier one, as in SurfaceRuleManager's map).
func (s *LocalSampler) compileSurfaceRules(ctx context.Context, snapshot *WorldgenSnapshot, docs map[string]WorldgenDocument, def cubiomes.Definition, d *seedMapDimension, id string) {
	category := surfaceRuleCategory(id)
	if d.terrain == nil || category == "" || snapshot == nil {
		return
	}
	ordered, err := forgeOrderRegionPrograms(snapshot)
	if err != nil {
		ordered = snapshot.RegionPrograms
	}
	index := map[string]int{}
	var sources []cubiomes.SurfaceRuleSource
	for _, program := range ordered {
		if program.Result == nil {
			continue
		}
		if program.Result.SurfaceRulesError != "" {
			d.issues = append(d.issues, WorldgenIssue{"surface_rules_untraced", program.ModID + " / " + program.Result.SurfaceRulesError})
		}
		for _, set := range program.Result.SurfaceRules {
			if set.Category != category {
				continue
			}
			src := cubiomes.SurfaceRuleSource{Namespace: set.Namespace, Rule: json.RawMessage(set.Rule)}
			if i, ok := index[set.Namespace]; ok {
				sources[i] = src
				continue
			}
			index[set.Namespace] = len(sources)
			sources = append(sources, src)
		}
	}
	if category == "NETHER" || category == "OVERWORLD" { // the dimension's own surface_rule decides vanilla biomes
		sources = append(sources, cubiomes.SurfaceRuleSource{Namespace: cubiomes.SurfaceBaseNamespace})
	}
	if len(sources) == 0 {
		return
	}
	rules, err := cubiomes.CompileSurfaceRules(ctx, docs, def, d.terrain, sources)
	if err != nil {
		d.issues = append(d.issues, WorldgenIssue{"surface_rules_invalid", id + " / " + err.Error()})
		return
	}
	d.surface = rules
}

// SurfaceRulesActive reports whether dimension has compiled mod surface rules.
func (s *LocalSampler) SurfaceRulesActive(dimension string) bool {
	if s == nil {
		return false
	}
	d, ok := s.dimensions[dimension]
	return ok && d.surface != nil && d.terrain != nil
}

// SurfaceColumn applies the compiled mod surface rules to a noise-stage column
// (sub from TerrainColumn, top its top non-air Y). out[y-MinY] is a block id,
// or "" where the vanilla rule applies. Biomes go through the post-1.15
// voronoi zoom per block, as BiomeManager.getBiome does in SurfaceSystem.
func (s *LocalSampler) SurfaceColumn(dimension string, x, z int, sub []TerrainSubstance, top int, out []string) bool {
	if !s.SurfaceRulesActive(dimension) {
		return false
	}
	d := s.dimensions[dimension]
	sampler := d.terrainPool.Get().(*cubiomes.TerrainSampler)
	defer d.terrainPool.Put(sampler)
	scratch := d.scratch()
	defer d.releaseScratch(scratch)
	biomes := map[int]string{}
	tops := map[[2]int]int{}
	in := &cubiomes.SurfaceInput{X: x, Z: z, Sub: sub,
		Biome: func(y int) string {
			if b, ok := biomes[y]; ok {
				return b
			}
			var b string
			if scratch != nil && d.mode != "fixed" && d.mode != "checkerboard" {
				b, _ = scratch.BiomeAt(x, y, z)
			} else {
				b, _ = d.biomeAt(x, y, z, scratch, false)
			}
			biomes[y] = b
			return b
		},
		Top: func(nx, nz int) int {
			if nx == x && nz == z {
				return top
			}
			k := [2]int{nx, nz}
			if v, ok := tops[k]; ok {
				return v
			}
			_, v := sampler.Column(nx, nz, nil)
			tops[k] = v
			return v
		},
		Prelim: sampler.Preliminary,
	}
	d.surface.Column(in, top, out)
	return true
}

// SurfaceRuleBlocks lists the block ids the dimension's mod rules can place.
func (s *LocalSampler) SurfaceRuleBlocks(dimension string) []string {
	if !s.SurfaceRulesActive(dimension) {
		return nil
	}
	return s.dimensions[dimension].surface.Blocks()
}
