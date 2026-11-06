package launcher

import (
	"encoding/json"
	"testing"

	"github.com/Viking602/YoloMCL/cubiomes"
)

// Oracle: p142-bz/BZOracle.java (AdvancedRandomSpread.getPotentialStructureChunk
// == vanilla RandomSpread via setLargeFeatureWithSalt additive mix). Cubiomes refs confirmed. Not save-scored.

func TestBZPotentialChunkOracle(t *testing.T) {
	const seed int64 = 12345
	// main_structures: spacing 31 separation 15 salt 578754875 linear
	p := seedMapPlacement{spacing: 31, separation: 15, salt: 578754875}
	cases := []struct{ rx, rz, bx, bz int }{
		{-2, -2, -856, -952},
		{-2, -1, -952, -360},
		{-2, 0, -872, 232},
		{-1, -1, -296, -248},
		{0, 0, 184, 248},
		{0, 1, 72, 504},
		{1, 0, 504, 200},
		{2, 2, 1112, 1208},
	}
	for _, c := range cases {
		gotX, gotZ := p.candidate(seed, c.rx, c.rz)
		if gotX != c.bx || gotZ != c.bz {
			t.Fatalf("main r %d,%d got %d,%d want %d,%d", c.rx, c.rz, gotX, gotZ, c.bx, c.bz)
		}
		refX, refZ, ok := (cubiomes.RandomSpread{Spacing: 31, Separation: 15, Salt: 578754875}).Candidate(seed, c.rx, c.rz)
		if !ok || refX != c.bx || refZ != c.bz {
			t.Fatalf("cubiomes mismatch r %d,%d", c.rx, c.rz)
		}
	}
	// hanging_structures: 48/25 salt 223986734
	h := seedMapPlacement{spacing: 48, separation: 25, salt: 223986734}
	hanging := []struct{ rx, rz, bx, bz int }{
		{-1, -1, -424, -536},
		{0, 0, 328, 56},
		{1, 1, 968, 1128},
	}
	for _, c := range hanging {
		gotX, gotZ := h.candidate(seed, c.rx, c.rz)
		if gotX != c.bx || gotZ != c.bz {
			t.Fatalf("hanging r %d,%d got %d,%d want %d,%d", c.rx, c.rz, gotX, gotZ, c.bx, c.bz)
		}
	}
}

func TestBZParseListOfAvoids(t *testing.T) {
	doc := WorldgenDocument{
		ID: "the_bumblezone:main_structures",
		Data: []byte(`{
			"structures":[{"structure":"the_bumblezone:overgrown_flower","weight":50}],
			"placement":{
				"type":"the_bumblezone:advanced_random_spread",
				"salt":578754875,
				"spacing":31,
				"separation":15,
				"super_exclusion_zone":{
					"list_of_avoids":[
						{"other_set":["the_bumblezone:cell_maze","the_bumblezone:pollinated_stream"],"chunk_count":5},
						{"other_set":["the_bumblezone:sempiternal_sanctums"],"chunk_count":4}
					]
				}
			}
		}`),
	}
	p, ok := parseSeedMapPlacement(doc, nil)
	if !ok {
		t.Fatal("parse failed")
	}
	if p.spacing != 31 || p.separation != 15 || p.salt != 578754875 {
		t.Fatalf("grid got %d/%d salt %d", p.spacing, p.separation, p.salt)
	}
	if len(p.excludes) != 2 {
		t.Fatalf("excludes=%d want 2", len(p.excludes))
	}
	if p.excludes[0].chunks != 5 || p.excludes[1].chunks != 4 {
		t.Fatalf("chunk counts %d %d", p.excludes[0].chunks, p.excludes[1].chunks)
	}
	if p.excludes[0].allowed != 0 || p.excludes[1].allowed != 0 {
		t.Fatal("allowed should be absent")
	}
	var sets []string
	if json.Unmarshal(p.excludes[0].raw, &sets) != nil || len(sets) != 2 {
		t.Fatalf("other_set0=%s", string(p.excludes[0].raw))
	}
}

func TestBZParseMinDistanceAndAllowed(t *testing.T) {
	doc := WorldgenDocument{
		ID: "the_bumblezone:throne_pillar",
		Data: []byte(`{
			"structures":[{"structure":"the_bumblezone:throne_pillar","weight":1}],
			"placement":{
				"type":"the_bumblezone:advanced_random_spread",
				"salt":17240798,
				"spacing":75,
				"separation":55,
				"min_distance_from_world_origin":500,
				"super_exclusion_zone":{
					"list_of_avoids":[
						{"other_set":["the_bumblezone:sempiternal_sanctums"],"chunk_count":5,"allowed_chunk_count":12}
					]
				}
			}
		}`),
	}
	p, ok := parseSeedMapPlacement(doc, nil)
	if !ok {
		t.Fatal("parse failed")
	}
	if p.minDistance != 500 {
		t.Fatalf("minDistance=%d", p.minDistance)
	}
	if len(p.excludes) != 1 || p.excludes[0].chunks != 5 || p.excludes[0].allowed != 12 {
		t.Fatalf("ex=%+v", p.excludes)
	}
	// Oracle: region 0,0 candidate chunk 7,7 — (7*16)^2*2 = 25088 < 500^2 → reject.
	const seed int64 = 12345
	bx, bz := p.candidate(seed, 0, 0)
	if bx != 136 || bz != 152 {
		t.Fatalf("candidate got %d,%d", bx, bz)
	}
	if p.placementChunk(seed, bx, bz) {
		t.Fatal("origin region should fail minDistance")
	}
	// region 0,1: block 8,1496 — outside minDistance
	bx, bz = p.candidate(seed, 0, 1)
	if bx != 8 || bz != 1496 {
		t.Fatalf("r0,1 got %d,%d", bx, bz)
	}
	if !p.placementChunk(seed, bx, bz) {
		t.Fatal("far candidate should pass without linked excludes")
	}
}

func TestBZAllowedChunkCountBytecode(t *testing.T) {
	// Synthetic using TF landmark_grid placements as range oracles:
	// near = unlocked landmark_grid (centers exist); far = lock that never matches.
	bx, bz := tfNearestCenterXZ(0, 0)
	cx, cz := bx>>4, bz>>4
	blockX, blockZ := (cx<<4)+8, (cz<<4)+8
	near := &seedMapPlacement{id: "near", landmarkGrid: true, spacing: 16}
	far := &seedMapPlacement{id: "far", landmarkGrid: true, spacing: 16, gridLock: "never:matches"}
	p := seedMapPlacement{
		spacing: 10, separation: 5, salt: 3,
		excludes: []seedMapExclusion{
			{chunks: 2, allowed: 5, others: []*seedMapPlacement{far}}, // nothing in allowed range
			{chunks: 64, others: []*seedMapPlacement{near}},           // would forbid if reached
		},
	}
	if !p.placementChunkDepth(0, blockX, blockZ, 0) {
		t.Fatal("allowed early-exit should skip the forbidding second avoid")
	}
	p2 := seedMapPlacement{
		spacing: 10, separation: 5, salt: 3,
		excludes: []seedMapExclusion{
			{chunks: 2, others: []*seedMapPlacement{far}},
			{chunks: 64, others: []*seedMapPlacement{near}},
		},
	}
	if p2.placementChunkDepth(0, blockX, blockZ, 0) {
		t.Fatal("without allowed, second avoid should forbid")
	}
	// Within chunk_count of near: forbid even when allowed is set.
	p3 := seedMapPlacement{
		spacing: 10, separation: 5, salt: 3,
		excludes: []seedMapExclusion{
			{chunks: 1, allowed: 5, others: []*seedMapPlacement{near}},
		},
	}
	if p3.placementChunkDepth(0, blockX, blockZ, 0) {
		t.Fatal("within chunk_count of near must forbid even when allowed is set")
	}
}

func TestBZFlatSuperZoneStillParses(t *testing.T) {
	// RS-shaped flat zone must keep working for non-BZ families and for zones helper.
	zones, ok := parseSeedMapExclusionZones(json.RawMessage(`{"other_set":"minecraft:villages","chunk_count":10}`), 64)
	if !ok || len(zones) != 1 || zones[0].chunks != 10 {
		t.Fatalf("flat ok=%v zones=%v", ok, zones)
	}
}
