package launcher

import (
	"context"
	"maps"
	"slices"
	"sort"
)

type seedMapEarlyKey struct{}
type seedMapEarlyIngestKey struct{}

// SeedMapEarly receives a provisional snapshot as soon as every data/resource
// pack has been read, before mod registration code runs. Only dimensions
// reported by SeedMapEarlyDimensions are guaranteed to sample identically to
// the final snapshot; everything else must wait for the final result.
type SeedMapEarly func(provisional *WorldgenSnapshot, level LevelInfo)

// WithSeedMapEarly attaches an early-result callback to an open request.
func WithSeedMapEarly(ctx context.Context, f SeedMapEarly) context.Context {
	return context.WithValue(ctx, seedMapEarlyKey{}, f)
}

func seedMapEarlyFor(ctx context.Context, level LevelInfo) context.Context {
	f, ok := ctx.Value(seedMapEarlyKey{}).(SeedMapEarly)
	if !ok || f == nil {
		return ctx
	}
	return context.WithValue(ctx, seedMapEarlyIngestKey{}, func(p *WorldgenSnapshot) { f(p, level) })
}

// emitSeedMapEarly publishes an isolated copy of the post-source snapshot when
// later stages (registration programs, biome programs) remain to run.
func emitSeedMapEarly(ctx context.Context, snap *WorldgenSnapshot) {
	f, ok := ctx.Value(seedMapEarlyIngestKey{}).(func(*WorldgenSnapshot))
	if !ok || f == nil {
		return
	}
	p := *snap
	p.Hash = ""
	p.Documents = slices.Clone(snap.Documents)
	p.Mods = slices.Clone(snap.Mods)
	p.Issues = slices.Clone(snap.Issues)
	p.ModConfigs = maps.Clone(snap.ModConfigs)
	p.RegionPrograms, p.BiomePrograms = nil, nil
	f(&p)
}

// SeedMapEarlyDimensions lists the dimensions of a provisional sampler whose
// output cannot depend on the stages skipped for it: not opaque (custom biome
// sources need biome programs), and not a TerraBlender target when TerraBlender
// is installed (regions need registration programs). Only a rank-ordered
// prefix is returned so the compiled-node budget sees the same history.
func SeedMapEarlyDimensions(ctx context.Context, provisional *WorldgenSnapshot, s *LocalSampler) []string {
	if provisional == nil || s == nil {
		return nil
	}
	tb := worldgenHasMod(provisional, "terrablender")
	ids := s.Dimensions()
	rank := func(id string) int {
		switch id {
		case "minecraft:overworld":
			return 0
		case "minecraft:the_nether":
			return 1
		case "minecraft:the_end":
			return 2
		}
		return 3
	}
	sort.SliceStable(ids, func(i, j int) bool { return rank(ids[i]) < rank(ids[j]) })
	var ready []string
	for _, id := range ids {
		d := s.dimensions[id]
		ok := !d.opaque && d.mode != ""
		if ok && tb && (d.mode == "multi_noise" || d.mode == "the_end") {
			_, applies, err := terraBlenderDimension(ctx, provisional, d)
			ok = err == nil && !applies && d.mode != "the_end"
		}
		if !ok {
			break
		}
		ready = append(ready, id)
	}
	return ready
}
