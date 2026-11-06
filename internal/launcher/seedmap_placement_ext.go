package launcher

import (
	"context"
	"encoding/json"

	"github.com/Viking602/YoloMCL/cubiomes"
)

// seedMapExclusion is one "other sets nearby" rule: vanilla exclusion_zone,
// or a mod's super/enhanced exclusion zone (any listed set, id or #tag, with a
// structure chunk within chunks rejects the candidate).
type seedMapExclusion struct {
	raw     json.RawMessage
	chunks  int
	allowed int // 0 = absent; BZ allowed_chunk_count when set
	others  []*seedMapPlacement
}

// seedMapSpreadFamily describes random_spread subclasses whose behaviour was
// checked against their bytecode: same candidate grid as vanilla, plus extras.
type seedMapSpreadFamily struct {
	minDistance bool   // isPlacementChunk rejects (cx*16)^2+(cz*16)^2 < d^2
	zoneKey     string // extra exclusion key
	avoidWater  bool   // Better Temples: no river/ocean biome within 48 blocks at y=120
	// scale multiplies spacing and separation once, Math.round(v*scale), in the
	// constructor (Moog's Structure Lib); 0 means unscaled. Such classes keep
	// their own grid fields, so Structurify's random_spread mixin misses them.
	scale float64
}

var seedMapSpreadFamilies = map[string]seedMapSpreadFamily{
	"minecraft:random_spread":                      {},
	"repurposed_structures:advanced_random_spread": {minDistance: true, zoneKey: "super_exclusion_zone"},
	"mes:advanced_random_spread":                   {minDistance: true, zoneKey: "super_exclusion_zone"},
	"cataclysm:cataclysm_random_spread":            {minDistance: true, zoneKey: "super_exclusion_zone"},
	"goety:random_spread":                          {minDistance: true, zoneKey: "super_exclusion_zone"},
	"goety:minor_random_spread":                    {zoneKey: "super_exclusion_zone"},
	"yungsapi:enhanced_random_spread":              {zoneKey: "enhanced_exclusion_zone"},
	"betterdeserttemples:desert_temple":            {zoneKey: "enhanced_exclusion_zone", avoidWater: true},
	"betterjungletemples:jungle_temple":            {zoneKey: "enhanced_exclusion_zone", avoidWater: true},
	"moogs_structures:advanced_random_spread":      {minDistance: true, zoneKey: "super_exclusion_zone", scale: 1.65},
	"integrated_api:advanced_random_spread":        {minDistance: true, zoneKey: "super_exclusion_zone"},
	"aquamirae:configurable_random_spread":         {},                                                   // ConfigurableStructurePlacement -> RandomSpread (javap)
	"eternal_starlight:avoid_landmark":             {},                                                   // RandomSpread + reject ES landmark center (javap)
	"the_bumblezone:advanced_random_spread":        {minDistance: true, zoneKey: "super_exclusion_zone"}, // list_of_avoids codec
	"twilightforest:avoid_landmark_grid":           {},                                                   // RandomSpread + Chebyshev<80 vs TF landmark (javap)
}

func parseSeedMapExclusion(raw json.RawMessage, maxChunks int) (seedMapExclusion, bool) {
	var zone struct {
		OtherSet   json.RawMessage `json:"other_set"`
		ChunkCount int             `json:"chunk_count"`
	}
	if json.Unmarshal(raw, &zone) != nil || len(zone.OtherSet) == 0 || zone.ChunkCount < 1 || zone.ChunkCount > maxChunks {
		return seedMapExclusion{}, false
	}
	return seedMapExclusion{raw: zone.OtherSet, chunks: zone.ChunkCount}, true
}

// parseBZAvoidData is one SuperExclusionZone.AvoidData entry:
// other_set (HolderSet) + chunk_count + optional allowed_chunk_count.
func parseBZAvoidData(raw json.RawMessage, maxChunks int) (seedMapExclusion, bool) {
	var zone struct {
		OtherSet          json.RawMessage `json:"other_set"`
		ChunkCount        int             `json:"chunk_count"`
		AllowedChunkCount *int            `json:"allowed_chunk_count"`
	}
	if json.Unmarshal(raw, &zone) != nil || len(zone.OtherSet) == 0 || zone.ChunkCount < 1 || zone.ChunkCount > maxChunks {
		return seedMapExclusion{}, false
	}
	ex := seedMapExclusion{raw: zone.OtherSet, chunks: zone.ChunkCount}
	if zone.AllowedChunkCount != nil {
		if *zone.AllowedChunkCount < 1 || *zone.AllowedChunkCount > maxChunks {
			return seedMapExclusion{}, false
		}
		ex.allowed = *zone.AllowedChunkCount
	}
	return ex, true
}

// parseSeedMapExclusionZones accepts either a flat {other_set,chunk_count}
// (vanilla / RS / YUNG) or BZ {list_of_avoids:[AvoidData,...]}.
func parseSeedMapExclusionZones(raw json.RawMessage, maxChunks int) ([]seedMapExclusion, bool) {
	var wrap struct {
		List []json.RawMessage `json:"list_of_avoids"`
	}
	if json.Unmarshal(raw, &wrap) == nil && len(wrap.List) > 0 {
		if len(wrap.List) > 64 {
			return nil, false
		}
		out := make([]seedMapExclusion, 0, len(wrap.List))
		for _, item := range wrap.List {
			z, ok := parseBZAvoidData(item, maxChunks)
			if !ok {
				return nil, false
			}
			out = append(out, z)
		}
		return out, true
	}
	z, ok := parseSeedMapExclusion(raw, maxChunks)
	if !ok {
		return nil, false
	}
	return []seedMapExclusion{z}, true
}

// javaIntValue is Gson's LazilyParsedNumber.intValue for an integral literal:
// (int) Long.parseLong, i.e. the low 32 bits.
func javaIntValue(n int64) int { return int(int32(uint32(uint64(n)))) }

// linkPlacementExclusions resolves every exclusion against all placements,
// including ones that carry exclusions themselves. A named set that is not
// registered cannot be reproduced and drops the placement; tags tolerate
// missing optional members like the game does.
func (s *LocalSampler) linkPlacementExclusions(ctx context.Context, docs map[string]WorldgenDocument) {
	heap := make([]*seedMapPlacement, len(s.placements))
	byID := map[string]*seedMapPlacement{}
	for i := range s.placements {
		cp := s.placements[i]
		cp.excludes = append([]seedMapExclusion(nil), cp.excludes...)
		heap[i] = &cp
		byID[cp.id] = &cp
	}
	water := map[string]bool(nil)
	kept := s.placements[:0]
	for i, h := range heap {
		ok := true
		for j := range h.excludes {
			budget := 4096
			ids, resolved := resolveSeedMapRegistry(ctx, docs, "structure_set", h.excludes[j].raw, map[string]bool{}, &budget)
			if !resolved {
				ok = false
				break
			}
			var single string
			named := json.Unmarshal(h.excludes[j].raw, &single) == nil && len(single) > 0 && single[0] != '#'
			for id := range ids {
				if other := byID[id]; other != nil {
					h.excludes[j].others = append(h.excludes[j].others, other)
				} else if named {
					ok = false
				}
			}
		}
		if ok && h.avoidWater {
			if water == nil {
				budget := 4096
				water, _ = resolveSeedMapRegistry(ctx, docs, "biome", json.RawMessage(`["#minecraft:is_river","#minecraft:is_ocean"]`), map[string]bool{}, &budget)
				if water == nil {
					water = map[string]bool{}
				}
			}
			h.waterBiomes = water
		}
		if !ok {
			s.addIssue("placement_unsupported", h.id)
			continue
		}
		p := s.placements[i]
		p.excludes, p.waterBiomes = h.excludes, h.waterBiomes
		kept = append(kept, p)
	}
	s.placements = kept
}

// nearWater is BiomeSource.findBiomeHorizontal(middle x/z, 120, 48, step 2)
// with an is_river/is_ocean predicate: every quart cell the 2-block lattice
// touches is sampled.
func nearWater(p seedMapPlacement, d seedMapDimension, x, z int, scratch *cubiomes.Sampler) (biome string, known, hit bool) {
	mx, mz := x&^15+7, z&^15+7
	for qx := (mx - 48) >> 2; qx <= (mx+48)>>2; qx++ {
		for qz := (mz - 48) >> 2; qz <= (mz+48)>>2; qz++ {
			b, ok := d.biomeAt(qx<<2, 120, qz<<2, scratch, false)
			if !ok {
				return "", false, false
			}
			if p.waterBiomes[b] {
				return b, true, true
			}
		}
	}
	return "", true, false
}
