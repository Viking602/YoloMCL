package launcher

import "testing"

// Oracle from p141-es/ESOracle.java (LandmarkStructurePlacement javap). Not save-scored.

func TestESLandmarkPosOracle(t *testing.T) {
	const seed int64 = 12345
	cases := []struct {
		rx, rz, cx, cz int
		id             string
	}{
		{-4, -4, -113, -118, "eternal_starlight:golem_forge"},
		{-4, -3, -106, -74, "eternal_starlight:cursed_garden"},
		{-4, -2, -108, -45, "eternal_starlight:golem_forge"},
		{-4, -1, -105, -22, "eternal_starlight:cursed_garden"},
		{-4, 0, -116, 20, "eternal_starlight:golem_forge"},
		{-4, 1, -118, 41, "eternal_starlight:cursed_garden"},
		{-4, 2, -111, 73, "eternal_starlight:golem_forge"},
		{-4, 3, -117, 115, "eternal_starlight:cursed_garden"},
		{-4, 4, -114, 141, "eternal_starlight:golem_forge"},
		{-3, -4, -82, -112, "eternal_starlight:cursed_garden"},
		{-3, -3, -77, -80, "eternal_starlight:golem_forge"},
		{-3, -2, -86, -42, "eternal_starlight:cursed_garden"},
		{-3, -1, -78, -15, "eternal_starlight:golem_forge"},
		{-3, 0, -80, 17, "eternal_starlight:cursed_garden"},
		{-3, 1, -76, 55, "eternal_starlight:golem_forge"},
		{-3, 2, -87, 77, "eternal_starlight:cursed_garden"},
		{-3, 3, -84, 119, "eternal_starlight:golem_forge"},
		{-3, 4, -85, 137, "eternal_starlight:cursed_garden"},
		{-2, -4, -50, -110, "eternal_starlight:golem_forge"},
		{-2, -3, -46, -82, "eternal_starlight:cursed_garden"},
		{-2, -2, -53, -45, "eternal_starlight:golem_forge"},
		{-2, -1, -46, -19, "eternal_starlight:cursed_garden"},
		{-2, 0, -55, 11, "eternal_starlight:golem_forge"},
		{-2, 1, -51, 50, "eternal_starlight:cursed_garden"},
		{-2, 2, -45, 80, "eternal_starlight:golem_forge"},
		{-2, 3, -45, 107, "eternal_starlight:cursed_garden"},
		{-2, 4, -42, 141, "eternal_starlight:golem_forge"},
		{-1, -4, -13, -115, "eternal_starlight:cursed_garden"},
		{-1, -3, -12, -81, "eternal_starlight:golem_forge"},
		{-1, -2, -18, -48, "eternal_starlight:cursed_garden"},
		{-1, -1, -16, -16, "eternal_starlight:golem_forge"},
		{-1, 0, -23, 21, "eternal_starlight:golem_forge"},
		{-1, 1, -17, 48, "eternal_starlight:golem_forge"},
		{-1, 2, -16, 76, "eternal_starlight:cursed_garden"},
		{-1, 3, -21, 110, "eternal_starlight:golem_forge"},
		{-1, 4, -20, 147, "eternal_starlight:cursed_garden"},
		{0, -4, 16, -108, "eternal_starlight:golem_forge"},
		{0, -3, 17, -86, "eternal_starlight:cursed_garden"},
		{0, -2, 14, -55, "eternal_starlight:golem_forge"},
		{0, -1, 13, -22, "eternal_starlight:golem_forge"},
		{0, 0, 23, 18, "eternal_starlight:golem_forge"},
		{0, 1, 20, 44, "eternal_starlight:cursed_garden"},
		{0, 2, 11, 77, "eternal_starlight:golem_forge"},
		{0, 3, 11, 108, "eternal_starlight:cursed_garden"},
		{0, 4, 18, 146, "eternal_starlight:golem_forge"},
		{1, -4, 51, -113, "eternal_starlight:cursed_garden"},
		{1, -3, 46, -87, "eternal_starlight:golem_forge"},
		{1, -2, 55, -47, "eternal_starlight:cursed_garden"},
		{1, -1, 55, -13, "eternal_starlight:golem_forge"},
		{1, 0, 45, 16, "eternal_starlight:cursed_garden"},
		{1, 1, 48, 55, "eternal_starlight:golem_forge"},
		{1, 2, 44, 83, "eternal_starlight:cursed_garden"},
		{1, 3, 48, 107, "eternal_starlight:golem_forge"},
		{1, 4, 49, 144, "eternal_starlight:cursed_garden"},
		{2, -4, 77, -114, "eternal_starlight:golem_forge"},
		{2, -3, 75, -81, "eternal_starlight:cursed_garden"},
		{2, -2, 87, -54, "eternal_starlight:golem_forge"},
		{2, -1, 81, -22, "eternal_starlight:cursed_garden"},
		{2, 0, 86, 12, "eternal_starlight:golem_forge"},
		{2, 1, 73, 44, "eternal_starlight:cursed_garden"},
		{2, 2, 73, 75, "eternal_starlight:golem_forge"},
		{2, 3, 77, 112, "eternal_starlight:cursed_garden"},
		{2, 4, 84, 140, "eternal_starlight:golem_forge"},
		{3, -4, 119, -113, "eternal_starlight:cursed_garden"},
		{3, -3, 106, -76, "eternal_starlight:golem_forge"},
		{3, -2, 116, -53, "eternal_starlight:cursed_garden"},
		{3, -1, 105, -17, "eternal_starlight:golem_forge"},
		{3, 0, 108, 22, "eternal_starlight:cursed_garden"},
		{3, 1, 119, 42, "eternal_starlight:golem_forge"},
		{3, 2, 108, 87, "eternal_starlight:cursed_garden"},
		{3, 3, 109, 111, "eternal_starlight:golem_forge"},
		{3, 4, 116, 141, "eternal_starlight:cursed_garden"},
		{4, -4, 143, -112, "eternal_starlight:golem_forge"},
		{4, -3, 138, -87, "eternal_starlight:cursed_garden"},
		{4, -2, 144, -52, "eternal_starlight:golem_forge"},
		{4, -1, 139, -16, "eternal_starlight:cursed_garden"},
		{4, 0, 144, 20, "eternal_starlight:golem_forge"},
		{4, 1, 137, 49, "eternal_starlight:cursed_garden"},
		{4, 2, 144, 87, "eternal_starlight:golem_forge"},
		{4, 3, 143, 117, "eternal_starlight:cursed_garden"},
		{4, 4, 148, 140, "eternal_starlight:golem_forge"},
	}
	for _, c := range cases {
		gotX, gotZ := esRegionLandmarkPos(seed, c.rx*32+16, c.rz*32+16)
		if gotX != c.cx || gotZ != c.cz {
			t.Fatalf("region %d,%d pos got %d,%d want %d,%d", c.rx, c.rz, gotX, gotZ, c.cx, c.cz)
		}
		// Same pos from any chunk in the region.
		gotX2, gotZ2 := esRegionLandmarkPos(seed, c.rx*32, c.rz*32)
		if gotX2 != c.cx || gotZ2 != c.cz {
			t.Fatalf("region %d,%d probe0 mismatch", c.rx, c.rz)
		}
		pick := esPickLandmark(seed, c.cx, c.cz)
		if pick != c.id {
			t.Fatalf("region %d,%d pick got %s want %s", c.rx, c.rz, pick, c.id)
		}
		if !esIsLandmarkPlacementChunk(seed, c.cx, c.cz, c.id) {
			t.Fatalf("lock %s rejected center", c.id)
		}
		other := "eternal_starlight:golem_forge"
		if c.id == other {
			other = "eternal_starlight:cursed_garden"
		}
		if esIsLandmarkPlacementChunk(seed, c.cx, c.cz, other) {
			t.Fatalf("other lock %s accepted %s center", other, c.id)
		}
		if !esIsLandmarkCenter(seed, c.cx, c.cz) {
			t.Fatalf("center not marked")
		}
	}
}

func TestESLandmarkUniqueness(t *testing.T) {
	const seed int64 = 12345
	cases := []struct{ rx, rz, n, cx, cz int }{
		{-2, -2, 1, -53, -45},
		{-2, -1, 1, -46, -19},
		{-2, 0, 1, -55, 11},
		{-2, 1, 1, -51, 50},
		{-2, 2, 1, -45, 80},
		{-1, -2, 1, -18, -48},
		{-1, -1, 1, -16, -16},
		{-1, 0, 1, -23, 21},
		{-1, 1, 1, -17, 48},
		{-1, 2, 1, -16, 76},
		{0, -2, 1, 14, -55},
		{0, -1, 1, 13, -22},
		{0, 0, 1, 23, 18},
		{0, 1, 1, 20, 44},
		{0, 2, 1, 11, 77},
		{1, -2, 1, 55, -47},
		{1, -1, 1, 55, -13},
		{1, 0, 1, 45, 16},
		{1, 1, 1, 48, 55},
		{1, 2, 1, 44, 83},
		{2, -2, 1, 87, -54},
		{2, -1, 1, 81, -22},
		{2, 0, 1, 86, 12},
		{2, 1, 1, 73, 44},
		{2, 2, 1, 73, 75},
	}
	for _, c := range cases {
		if c.n != 1 {
			t.Fatalf("region %d,%d expected 1 center, oracle N=%d", c.rx, c.rz, c.n)
		}
		count := 0
		for x := c.rx * 32; x < c.rx*32+32; x++ {
			for z := c.rz * 32; z < c.rz*32+32; z++ {
				if esIsLandmarkCenter(seed, x, z) {
					count++
					if x != c.cx || z != c.cz {
						t.Fatalf("unexpected center %d,%d want %d,%d", x, z, c.cx, c.cz)
					}
				}
			}
		}
		if count != 1 {
			t.Fatalf("region %d,%d count %d", c.rx, c.rz, count)
		}
	}
}

func TestESLandmarkParse(t *testing.T) {
	doc := WorldgenDocument{
		ID: "eternal_starlight:golem_forge", Source: "mod:eternal_starlight",
		Data: []byte(`{"placement":{"type":"eternal_starlight:landmark","landmark":"eternal_starlight:golem_forge"},"structures":[{"structure":"eternal_starlight:golem_forge","weight":1}]}`),
	}
	p, ok := parseSeedMapPlacement(doc, nil)
	if !ok || !p.esLandmark || p.esLock != "eternal_starlight:golem_forge" || p.spacing != 32 {
		t.Fatalf("landmark parse ok=%v es=%v lock=%q spacing=%d", ok, p.esLandmark, p.esLock, p.spacing)
	}
	doc2 := WorldgenDocument{
		ID: "eternal_starlight:stranghoul_den", Source: "mod:eternal_starlight",
		Data: []byte(`{"placement":{"type":"eternal_starlight:avoid_landmark","salt":615391630,"separation":8,"spacing":20},"structures":[{"structure":"eternal_starlight:stranghoul_den","weight":1}]}`),
	}
	p2, ok := parseSeedMapPlacement(doc2, nil)
	if !ok || !p2.esAvoidLandmark || p2.spacing != 20 || p2.separation != 8 || p2.salt != 615391630 {
		t.Fatalf("avoid parse ok=%v avoid=%v sp=%d sep=%d salt=%d", ok, p2.esAvoidLandmark, p2.spacing, p2.separation, p2.salt)
	}
	// avoid rejects landmark centers
	const seed int64 = 12345
	cx, cz := esRegionLandmarkPos(seed, 0, 0)
	x, z := p2.candidate(seed, 0, 0) // random_spread region 0 — may or may not be center
	_ = x
	_ = z
	if p2.placementChunk(seed, cx<<4, cz<<4) {
		t.Fatalf("avoid_landmark accepted landmark center %d,%d", cx, cz)
	}
}

func TestESLandmarkCandidate(t *testing.T) {
	const seed int64 = 12345
	x, z, ok := esLandmarkCandidateBlock(seed, 2, 2, "eternal_starlight:golem_forge")
	cx, cz := esRegionLandmarkPos(seed, 64, 64)
	pick := esPickLandmark(seed, cx, cz)
	if pick == "eternal_starlight:golem_forge" {
		if !ok || x>>4 != cx || z>>4 != cz {
			t.Fatalf("golem candidate ok=%v %d,%d want chunk %d,%d", ok, x>>4, z>>4, cx, cz)
		}
	} else if ok {
		t.Fatalf("golem candidate ok but pick=%s", pick)
	}
}
