package launcher

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// seedMapRings is a minecraft:concentric_rings placement (strongholds). The
// ring positions depend only on the seed and the biome source, so they are
// computed once per dimension and shared by tiles and searches.
type seedMapRings struct {
	distance, spread, count int
	preferredRaw            json.RawMessage
	preferred               map[string]bool
	preferredKnown          bool

	mu    sync.Mutex
	cache map[string][][2]int32
}

func parseSeedMapRings(placement map[string]json.RawMessage) (*seedMapRings, bool) {
	r := &seedMapRings{cache: map[string][][2]int32{}}
	var distance, spread, count *int
	for key, raw := range placement {
		var err error
		switch key {
		case "type", "salt":
		case "distance":
			err = json.Unmarshal(raw, &distance)
		case "spread":
			err = json.Unmarshal(raw, &spread)
		case "count":
			err = json.Unmarshal(raw, &count)
		case "preferred_biomes":
			r.preferredRaw = raw
		case "frequency":
			var f float64
			err = json.Unmarshal(raw, &f)
			if f != 1 {
				return nil, false
			}
		case "locate_offset":
			var offset []int
			err = json.Unmarshal(raw, &offset)
			if len(offset) != 3 || offset[0] != 0 || offset[1] != 0 || offset[2] != 0 {
				return nil, false
			}
		default:
			return nil, false
		}
		if err != nil {
			return nil, false
		}
	}
	// preferred_biomes is required by the vanilla codec.
	if distance == nil || spread == nil || count == nil || r.preferredRaw == nil {
		return nil, false
	}
	r.distance, r.spread, r.count = *distance, *spread, *count
	if r.distance < 0 || r.distance > 1023 || r.spread < 1 || r.spread > 1023 || r.count < 1 || r.count > 4095 {
		return nil, false
	}
	return r, true
}

// ringChunks returns the biome-snapped start chunks for one dimension.
func (s *LocalSampler) ringChunks(ctx context.Context, p seedMapPlacement, dimension string) ([][2]int32, error) {
	r := p.rings
	if r == nil || !r.preferredKnown {
		return nil, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.cache[dimension]; ok {
		return c, nil
	}
	d, ok := s.dimensions[dimension]
	if !ok {
		return nil, nil
	}
	scratch := d.scratch()
	defer d.releaseScratch(scratch)
	var cancelled error
	calls := 0
	inBiome := func(qx, qz int) bool {
		if cancelled != nil {
			return false
		}
		if calls++; calls&1023 == 0 {
			if err := ctx.Err(); err != nil {
				cancelled = err
				return false
			}
		}
		// Vanilla's search keeps the previous climate leaf between cells.
		id, ok := d.biomeAt(qx*4, 0, qz*4, scratch, scratch != nil)
		return ok && r.preferred[id]
	}
	if d.mode == "" {
		inBiome = func(int, int) bool { return false }
	}
	begin := func() {
		if scratch != nil {
			scratch.Reset()
		}
	}
	chunks := worldgen.ConcentricRingsWith(s.seed, r.distance, r.spread, r.count, begin, inBiome)
	if cancelled != nil {
		return nil, cancelled
	}
	r.cache[dimension] = chunks
	return chunks, nil
}

func (s *LocalSampler) compileRingBiomes(ctx context.Context, p *seedMapPlacement, docs map[string]WorldgenDocument) {
	budget := 32768
	values, ok := resolveSeedMapBiomes(ctx, docs, p.rings.preferredRaw, map[string]bool{}, &budget)
	var tag string
	if !ok && json.Unmarshal(p.rings.preferredRaw, &tag) == nil && strings.HasPrefix(tag, "#") {
		// The game binds a reference to an undefined tag as an empty set, so
		// every ring keeps its unsnapped position (seen in saved packs).
		if _, found := docs["biome_tag/"+seedMapID(tag[1:])]; !found && hasBiomeTags(docs) {
			values, ok = map[string]bool{}, true
			s.addIssue("placement_tag_empty", p.id+" / "+tag)
		}
	}
	if !ok {
		s.addIssue("placement_biomes_unknown", p.id)
		return
	}
	p.rings.preferred, p.rings.preferredKnown = values, true
}

// hasBiomeTags reports whether the snapshot carries the biome tag registry,
// so an absent tag is really undefined rather than simply not ingested.
func hasBiomeTags(docs map[string]WorldgenDocument) bool {
	for key := range docs {
		if strings.HasPrefix(key, "biome_tag/") {
			return true
		}
	}
	return false
}
