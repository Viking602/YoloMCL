package launcher

import (
	"context"
	"errors"
	"sort"
)

// SeedMapStructure describes a supported placement rule and its alternatives.
type SeedMapStructure struct {
	ID, Dimension, Source string
	Structures            []string
}

// Structures lists placements from the effective registry that can occur in a
// dimension. It shares the same eligibility rules as map markers and searches.
func (s *LocalSampler) Structures(dimension string) []SeedMapStructure {
	var result []SeedMapStructure
	for _, p := range s.placements {
		if !p.inDimension(dimension) {
			continue
		}
		result = append(result, SeedMapStructure{p.id, p.dimension, p.source, append([]string(nil), p.structures...)})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

type SeedMapSearch struct {
	Dimension, Structure   string
	X, Y, Z, Radius, Limit int
}

type SeedMapSearchResult struct {
	Hits    []FeatureHit
	Limited bool
}

// FindStructures searches a bounded area without rendering terrain tiles. The
// results are biome-eligible placement candidates at the selected height.
func (s *LocalSampler) FindStructures(ctx context.Context, q SeedMapSearch) (SeedMapSearchResult, error) {
	result := SeedMapSearchResult{}
	if q.Radius < 16 || q.Radius > 32768 || q.Limit < 1 || q.Limit > 64 || q.Y < -2048 || q.Y > 2047 || q.X < -29900000 || q.X > 29900000 || q.Z < -29900000 || q.Z > 29900000 {
		return result, errors.New("invalid structure search")
	}
	d, ok := s.dimensions[q.Dimension]
	if !ok {
		return result, errors.New("unknown dimension")
	}
	var placement *seedMapPlacement
	for i := range s.placements {
		if s.placements[i].id == q.Structure && s.placements[i].inDimension(q.Dimension) {
			placement = &s.placements[i]
			break
		}
	}
	if placement == nil {
		return result, errors.New("unsupported structure placement")
	}
	p := *placement
	scratch := d.scratch()
	defer d.releaseScratch(scratch)
	if p.rings != nil {
		return s.findRings(ctx, p, d, q)
	}
	if p.landmarkGrid {
		return s.findLandmarkGrid(ctx, p, d, q)
	}
	if p.esLandmark {
		return s.findESLandmark(ctx, p, d, q)
	}
	checks := 0
	span := p.spacing * 16
	distance := func(h FeatureHit) int64 { dx, dz := int64(h.X-q.X), int64(h.Z-q.Z); return dx*dx + dz*dz }
	less := func(a, b FeatureHit) bool {
		da, db := distance(a), distance(b)
		if da != db {
			return da < db
		}
		if a.X != b.X {
			return a.X < b.X
		}
		return a.Z < b.Z
	}
	minX, maxX := seedMapFloor(q.X-q.Radius, span), seedMapFloor(q.X+q.Radius, span)
	minZ, maxZ := seedMapFloor(q.Z-q.Radius, span), seedMapFloor(q.Z+q.Radius, span)
	cx, cz := seedMapFloor(q.X, span), seedMapFloor(q.Z, span)
	var searchErr error
	visit := func(rx, rz int) bool {
		if rx < minX || rx > maxX || rz < minZ || rz > maxZ {
			return false
		}
		if err := ctx.Err(); err != nil {
			searchErr = err
			return true
		}
		checks++
		if checks > 65536 {
			result.Limited = true
			return true
		}
		x, z := p.candidate(s.seed, rx, rz)
		if !p.placementChunk(s.seed, x, z) {
			return false
		}
		h := FeatureHit{ID: p.id, Kind: "structure_set", Dimension: p.dimension, Source: p.source, X: x, Y: q.Y, Z: z, Confidence: "candidate"}
		if distance(h) > int64(q.Radius)*int64(q.Radius) {
			return false
		}
		if p.biomesKnown && d.mode != "" {
			if biome, ok, pass := s.biomeCheck(p, d, q.Dimension, x, q.Y, z, scratch); ok {
				if !pass {
					return false
				}
				h.Biome, h.Confidence = biome, "biome_candidate"
			}
		}
		result.Hits = append(result.Hits, h)
		sort.Slice(result.Hits, func(i, j int) bool { return less(result.Hits[i], result.Hits[j]) })
		if len(result.Hits) > q.Limit {
			result.Hits = result.Hits[:q.Limit]
		}
		return false
	}
	// Search outwards so a capped dense placement still finds nearby sites.
	// A completed square also bounds every unvisited region's distance: stop
	// once none can beat the farthest retained hit (strictly, to keep ties).
	maxRing := max(cx-minX, maxX-cx, cz-minZ, maxZ-cz)
	for ring := 0; ring <= maxRing; ring++ {
		if ring == 0 {
			if visit(cx, cz) {
				return result, searchErr
			}
		} else {
			for offset := -ring; offset <= ring; offset++ {
				for edge, region := range [4][2]int{{cx + offset, cz - ring}, {cx + offset, cz + ring}, {cx - ring, cz + offset}, {cx + ring, cz + offset}} {
					if edge >= 2 && (offset == -ring || offset == ring) {
						continue
					}
					if visit(region[0], region[1]) {
						return result, searchErr
					}
				}
			}
		}
		remaining := int64(min(q.X-(cx-ring)*span, (cx+ring+1)*span-q.X, q.Z-(cz-ring)*span, (cz+ring+1)*span-q.Z))
		if len(result.Hits) == q.Limit && distance(result.Hits[len(result.Hits)-1]) < remaining*remaining {
			break
		}
	}
	return result, nil
}

// findRings returns the nearest biome-eligible concentric-ring starts.
func (s *LocalSampler) findRings(ctx context.Context, p seedMapPlacement, d seedMapDimension, q SeedMapSearch) (SeedMapSearchResult, error) {
	result := SeedMapSearchResult{}
	chunks, err := s.ringChunks(ctx, p, q.Dimension)
	if err != nil {
		return result, err
	}
	if !p.rings.preferredKnown {
		return result, errors.New("unsupported structure placement")
	}
	scratch := d.scratch()
	defer d.releaseScratch(scratch)
	r2 := int64(q.Radius) * int64(q.Radius)
	for _, c := range chunks {
		x, z := int(c[0])*16, int(c[1])*16
		dx, dz := int64(x-q.X), int64(z-q.Z)
		if dx*dx+dz*dz > r2 {
			continue
		}
		h := FeatureHit{ID: p.id, Kind: "structure_set", Dimension: p.dimension, Source: p.source, X: x, Y: q.Y, Z: z, Confidence: "ring_candidate"}
		if p.biomesKnown && d.mode != "" {
			if biome, ok := d.biomeAt(x, q.Y, z, scratch, false); ok {
				if !p.allowed[biome] {
					continue
				}
				h.Biome, h.Confidence = biome, "biome_candidate"
			}
		}
		result.Hits = append(result.Hits, h)
	}
	dist := func(h FeatureHit) int64 { dx, dz := int64(h.X-q.X), int64(h.Z-q.Z); return dx*dx + dz*dz }
	sort.Slice(result.Hits, func(i, j int) bool {
		a, b := result.Hits[i], result.Hits[j]
		if dist(a) != dist(b) {
			return dist(a) < dist(b)
		}
		if a.X != b.X {
			return a.X < b.X
		}
		return a.Z < b.Z
	})
	if len(result.Hits) > q.Limit {
		result.Hits = result.Hits[:q.Limit]
	}
	return result, nil
}

// findLandmarkGrid returns nearest twilightforest:landmark_grid starts.
func (s *LocalSampler) findLandmarkGrid(ctx context.Context, p seedMapPlacement, d seedMapDimension, q SeedMapSearch) (SeedMapSearchResult, error) {
	result := SeedMapSearchResult{}
	scratch := d.scratch()
	defer d.releaseScratch(scratch)
	r2 := int64(q.Radius) * int64(q.Radius)
	const regionBlocks = 256
	minRX := seedMapFloor(q.X-q.Radius+128, regionBlocks)
	maxRX := seedMapFloor(q.X+q.Radius+128, regionBlocks)
	minRZ := seedMapFloor(q.Z-q.Radius+128, regionBlocks)
	maxRZ := seedMapFloor(q.Z+q.Radius+128, regionBlocks)
	checks := 0
	for rz := minRZ; rz <= maxRZ; rz++ {
		for rx := minRX; rx <= maxRX; rx++ {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			checks++
			if checks > 65536 {
				result.Limited = true
				return result, nil
			}
			x, z, ok := tfLandmarkCandidateBlock(s.seed, rx, rz, p.gridLock)
			if !ok {
				continue
			}
			dx, dz := int64(x-q.X), int64(z-q.Z)
			if dx*dx+dz*dz > r2 {
				continue
			}
			h := FeatureHit{ID: p.id, Kind: "structure_set", Dimension: p.dimension, Source: p.source, X: x, Y: q.Y, Z: z, Confidence: "candidate"}
			if p.biomesKnown && d.mode != "" {
				if biome, ok, pass := s.biomeCheck(p, d, q.Dimension, x, q.Y, z, scratch); ok {
					if !pass {
						continue
					}
					h.Biome, h.Confidence = biome, "biome_candidate"
				}
			}
			result.Hits = append(result.Hits, h)
		}
	}
	dist := func(h FeatureHit) int64 { dx, dz := int64(h.X-q.X), int64(h.Z-q.Z); return dx*dx + dz*dz }
	sort.Slice(result.Hits, func(i, j int) bool {
		a, b := result.Hits[i], result.Hits[j]
		da, db := dist(a), dist(b)
		if da != db {
			return da < db
		}
		if a.X != b.X {
			return a.X < b.X
		}
		return a.Z < b.Z
	})
	if len(result.Hits) > q.Limit {
		result.Hits = result.Hits[:q.Limit]
	}
	return result, nil
}

// findESLandmark returns nearest eternal_starlight:landmark starts.
func (s *LocalSampler) findESLandmark(ctx context.Context, p seedMapPlacement, d seedMapDimension, q SeedMapSearch) (SeedMapSearchResult, error) {
	result := SeedMapSearchResult{}
	scratch := d.scratch()
	defer d.releaseScratch(scratch)
	r2 := int64(q.Radius) * int64(q.Radius)
	const regionBlocks = 512
	minRX := seedMapFloor(q.X-q.Radius, regionBlocks)
	maxRX := seedMapFloor(q.X+q.Radius, regionBlocks)
	minRZ := seedMapFloor(q.Z-q.Radius, regionBlocks)
	maxRZ := seedMapFloor(q.Z+q.Radius, regionBlocks)
	checks := 0
	for rz := minRZ; rz <= maxRZ; rz++ {
		for rx := minRX; rx <= maxRX; rx++ {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			checks++
			if checks > 65536 {
				result.Limited = true
				return result, nil
			}
			x, z, ok := esLandmarkCandidateBlock(s.seed, rx, rz, p.esLock)
			if !ok {
				continue
			}
			dx, dz := int64(x-q.X), int64(z-q.Z)
			if dx*dx+dz*dz > r2 {
				continue
			}
			h := FeatureHit{ID: p.id, Kind: "structure_set", Dimension: p.dimension, Source: p.source, X: x, Y: q.Y, Z: z, Confidence: "candidate"}
			if p.biomesKnown && d.mode != "" {
				if biome, ok, pass := s.biomeCheck(p, d, q.Dimension, x, q.Y, z, scratch); ok {
					if !pass {
						continue
					}
					h.Biome, h.Confidence = biome, "biome_candidate"
				}
			}
			result.Hits = append(result.Hits, h)
		}
	}
	dist := func(h FeatureHit) int64 { dx, dz := int64(h.X-q.X), int64(h.Z-q.Z); return dx*dx + dz*dz }
	sort.Slice(result.Hits, func(i, j int) bool {
		a, b := result.Hits[i], result.Hits[j]
		da, db := dist(a), dist(b)
		if da != db {
			return da < db
		}
		if a.X != b.X {
			return a.X < b.X
		}
		return a.Z < b.Z
	})
	if len(result.Hits) > q.Limit {
		result.Hits = result.Hits[:q.Limit]
	}
	return result, nil
}
