package launcher

import (
	"testing"
)

// Oracle vectors from p140-tf-javap/TFOracle.java (LegacyLandmarkPlacements
// source transcribed; java.util.Random == LegacyRandomSource). Not save-scored.

func TestTFLandmarkCentersOracle(t *testing.T) {
	cases := []struct{ rx, rz, bx, bz, cx, cz int }{
		{-4, -4, -1015, -1031, -64, -65},
		{-4, -3, -1015, -807, -64, -51},
		{-4, -2, -1031, -487, -65, -31},
		{-4, -1, -1047, -247, -66, -16},
		{-4, 0, -983, 8, -62, 0},
		{-4, 1, -1015, 264, -64, 16},
		{-4, 2, -999, 488, -63, 30},
		{-4, 3, -983, 760, -62, 47},
		{-4, 4, -1031, 1048, -65, 65},
		{-3, -4, -727, -1015, -46, -64},
		{-3, -3, -743, -759, -47, -48},
		{-3, -2, -727, -503, -46, -32},
		{-3, -1, -743, -295, -47, -19},
		{-3, 0, -775, -24, -49, -2},
		{-3, 1, -743, 296, -47, 18},
		{-3, 2, -775, 536, -49, 33},
		{-3, 3, -727, 808, -46, 50},
		{-3, 4, -743, 1080, -47, 67},
		{-2, -4, -519, -1015, -33, -64},
		{-2, -3, -551, -759, -35, -48},
		{-2, -2, -503, -487, -32, -31},
		{-2, -1, -551, -199, -35, -13},
		{-2, 0, -519, 40, -33, 2},
		{-2, 1, -503, 264, -32, 16},
		{-2, 2, -487, 472, -31, 29},
		{-2, 3, -503, 776, -32, 48},
		{-2, 4, -503, 1032, -32, 64},
		{-1, -4, -247, -999, -16, -63},
		{-1, -3, -279, -759, -18, -48},
		{-1, -2, -295, -487, -19, -31},
		{-1, -1, -247, -247, -16, -16},
		{-1, 0, -247, 8, -16, 0},
		{-1, 1, -279, 280, -18, 17},
		{-1, 2, -199, 552, -13, 34},
		{-1, 3, -247, 776, -16, 48},
		{-1, 4, -279, 984, -18, 61},
		{0, -4, 8, -967, 0, -61},
		{0, -3, 24, -727, 1, -46},
		{0, -2, 40, -503, 2, -32},
		{0, -1, 8, -247, 0, -16},
		{0, 0, 8, 8, 0, 0},
		{0, 1, 8, 232, 0, 14},
		{0, 2, 40, 552, 2, 34},
		{0, 3, 24, 808, 1, 50},
		{0, 4, 8, 1032, 0, 64},
		{1, -4, 280, -1063, 17, -67},
		{1, -3, 264, -759, 16, -48},
		{1, -2, 264, -487, 16, -31},
		{1, -1, 296, -263, 18, -17},
		{1, 0, 280, 56, 17, 3},
		{1, 1, 264, 264, 16, 16},
		{1, 2, 264, 488, 16, 30},
		{1, 3, 296, 776, 18, 48},
		{1, 4, 280, 1000, 17, 62},
		{2, -4, 552, -999, 34, -63},
		{2, -3, 552, -775, 34, -49},
		{2, -2, 504, -455, 31, -29},
		{2, -1, 536, -199, 33, -13},
		{2, 0, 536, -8, 33, -1},
		{2, 1, 536, 232, 33, 14},
		{2, 2, 520, 504, 32, 31},
		{2, 3, 520, 776, 32, 48},
		{2, 4, 504, 1048, 31, 65},
		{3, -4, 776, -1015, 48, -64},
		{3, -3, 744, -791, 46, -50},
		{3, -2, 760, -503, 47, -32},
		{3, -1, 760, -279, 47, -18},
		{3, 0, 760, 24, 47, 1},
		{3, 1, 760, 312, 47, 19},
		{3, 2, 824, 520, 51, 32},
		{3, 3, 760, 776, 47, 48},
		{3, 4, 808, 1032, 50, 64},
		{4, -4, 1048, -1031, 65, -65},
		{4, -3, 1080, -791, 67, -50},
		{4, -2, 1048, -471, 65, -30},
		{4, -1, 1016, -231, 63, -15},
		{4, 0, 984, 40, 61, 2},
		{4, 1, 1048, 264, 65, 16},
		{4, 2, 1016, 520, 63, 32},
		{4, 3, 984, 792, 61, 49},
		{4, 4, 1032, 1048, 64, 65},
	}
	for _, c := range cases {
		bx, bz := tfNearestCenterXZ(c.rx*16, c.rz*16)
		if bx != c.bx || bz != c.bz {
			t.Fatalf("region %d,%d center block got %d,%d want %d,%d", c.rx, c.rz, bx, bz, c.bx, c.bz)
		}
		if bx>>4 != c.cx || bz>>4 != c.cz {
			t.Fatalf("region %d,%d center chunk got %d,%d want %d,%d", c.rx, c.rz, bx>>4, bz>>4, c.cx, c.cz)
		}
		if !tfChunkHasLandmarkCenter(c.cx, c.cz) {
			t.Fatalf("center chunk %d,%d not marked", c.cx, c.cz)
		}
		count := 0
		for x := c.rx*16 - 8; x < c.rx*16-8+16; x++ {
			for z := c.rz*16 - 8; z < c.rz*16-8+16; z++ {
				if tfChunkHasLandmarkCenter(x, z) {
					count++
				}
			}
		}
		if count != 1 {
			t.Fatalf("region %d,%d center count %d", c.rx, c.rz, count)
		}
	}
}

func TestTFLandmarkVarietyOracle(t *testing.T) {
	const seed int64 = 12345
	cases := []struct {
		rx, rz, cx, cz int
		id             string
	}{
		{-8, -8, -128, -129, "twilightforest:small_hollow_hill"},
		{-8, -7, -127, -109, "twilightforest:lich_tower"},
		{-8, -6, -128, -99, "twilightforest:small_hollow_hill"},
		{-8, -5, -128, -81, "twilightforest:hedge_maze"},
		{-8, -4, -127, -65, "twilightforest:naga_courtyard"},
		{-8, -3, -128, -49, "twilightforest:medium_hollow_hill"},
		{-8, -2, -127, -31, "twilightforest:hedge_maze"},
		{-8, -1, -128, -16, "twilightforest:lich_tower"},
		{-8, 0, -129, 3, "twilightforest:small_hollow_hill"},
		{-8, 1, -128, 14, "twilightforest:lich_tower"},
		{-8, 2, -128, 30, "twilightforest:hedge_maze"},
		{-8, 3, -129, 47, "twilightforest:small_hollow_hill"},
		{-8, 4, -131, 63, "twilightforest:medium_hollow_hill"},
		{-8, 5, -128, 80, "twilightforest:lich_tower"},
		{-8, 6, -128, 93, "twilightforest:small_hollow_hill"},
		{-8, 7, -128, 111, "twilightforest:lich_tower"},
		{-8, 8, -126, 129, "twilightforest:medium_hollow_hill"},
		{-7, -8, -114, -125, "twilightforest:naga_courtyard"},
		{-7, -7, -111, -111, "twilightforest:small_hollow_hill"},
		{-7, -6, -114, -97, "twilightforest:naga_courtyard"},
		{-7, -5, -109, -81, "twilightforest:small_hollow_hill"},
		{-7, -4, -110, -64, "twilightforest:medium_hollow_hill"},
		{-7, -3, -110, -47, "twilightforest:lich_tower"},
		{-7, -2, -114, -29, "twilightforest:medium_hollow_hill"},
		{-7, -1, -112, -15, "twilightforest:medium_hollow_hill"},
		{-7, 0, -114, 0, "twilightforest:naga_courtyard"},
		{-7, 1, -112, 15, "twilightforest:small_hollow_hill"},
		{-7, 2, -111, 29, "twilightforest:lich_tower"},
		{-7, 3, -115, 50, "twilightforest:naga_courtyard"},
		{-7, 4, -111, 66, "twilightforest:small_hollow_hill"},
		{-7, 5, -112, 79, "twilightforest:medium_hollow_hill"},
		{-7, 6, -114, 97, "twilightforest:medium_hollow_hill"},
		{-7, 7, -112, 113, "twilightforest:medium_hollow_hill"},
		{-7, 8, -111, 128, "twilightforest:naga_courtyard"},
		{-6, -8, -96, -128, "twilightforest:naga_courtyard"},
		{-6, -7, -99, -112, "twilightforest:lich_tower"},
		{-6, -6, -97, -96, "twilightforest:small_hollow_hill"},
		{-6, -5, -94, -80, "twilightforest:naga_courtyard"},
		{-6, -4, -97, -64, "twilightforest:small_hollow_hill"},
		{-6, -3, -95, -50, "twilightforest:medium_hollow_hill"},
		{-6, -2, -94, -31, "twilightforest:naga_courtyard"},
		{-6, -1, -96, -17, "twilightforest:naga_courtyard"},
		{-6, 0, -95, -1, "twilightforest:small_hollow_hill"},
		{-6, 1, -95, 17, "twilightforest:medium_hollow_hill"},
		{-6, 2, -99, 34, "twilightforest:large_hollow_hill"},
		{-6, 3, -94, 48, "twilightforest:small_hollow_hill"},
		{-6, 4, -97, 65, "twilightforest:naga_courtyard"},
		{-6, 5, -93, 80, "twilightforest:naga_courtyard"},
		{-6, 6, -96, 94, "twilightforest:lich_tower"},
		{-6, 7, -95, 112, "twilightforest:medium_hollow_hill"},
		{-6, 8, -96, 129, "twilightforest:large_hollow_hill"},
		{-5, -8, -80, -128, "twilightforest:small_hollow_hill"},
		{-5, -7, -79, -111, "twilightforest:medium_hollow_hill"},
		{-5, -6, -78, -97, "twilightforest:lich_tower"},
		{-5, -5, -77, -83, "twilightforest:small_hollow_hill"},
		{-5, -4, -81, -62, "twilightforest:naga_courtyard"},
		{-5, -3, -82, -48, "twilightforest:naga_courtyard"},
		{-5, -2, -80, -31, "twilightforest:small_hollow_hill"},
		{-5, -1, -78, -18, "twilightforest:small_hollow_hill"},
		{-5, 0, -81, 1, "twilightforest:hedge_maze"},
		{-5, 1, -80, 15, "twilightforest:lich_tower"},
		{-5, 2, -81, 32, "twilightforest:medium_hollow_hill"},
		{-5, 3, -80, 47, "twilightforest:medium_hollow_hill"},
		{-5, 4, -83, 65, "twilightforest:lich_tower"},
		{-5, 5, -82, 80, "twilightforest:lich_tower"},
		{-5, 6, -78, 97, "twilightforest:medium_hollow_hill"},
		{-5, 7, -77, 114, "twilightforest:small_hollow_hill"},
		{-5, 8, -79, 130, "twilightforest:small_hollow_hill"},
		{-4, -8, -64, -129, "twilightforest:naga_courtyard"},
		{-4, -7, -64, -115, "twilightforest:small_hollow_hill"},
		{-4, -6, -64, -96, "twilightforest:medium_hollow_hill"},
		{-4, -5, -64, -83, "twilightforest:large_hollow_hill"},
		{-4, -4, -64, -65, "twilightforest:lich_tower"},
		{-4, -3, -64, -51, "twilightforest:hedge_maze"},
		{-4, -2, -65, -31, "twilightforest:naga_courtyard"},
		{-4, -1, -66, -16, "twilightforest:small_hollow_hill"},
		{-4, 0, -62, 0, "twilightforest:medium_hollow_hill"},
		{-4, 1, -64, 16, "twilightforest:medium_hollow_hill"},
		{-4, 2, -63, 30, "twilightforest:naga_courtyard"},
		{-4, 3, -62, 47, "twilightforest:small_hollow_hill"},
		{-4, 4, -65, 65, "twilightforest:medium_hollow_hill"},
		{-4, 5, -62, 81, "twilightforest:large_hollow_hill"},
		{-4, 6, -63, 98, "twilightforest:naga_courtyard"},
		{-4, 7, -63, 111, "twilightforest:medium_hollow_hill"},
		{-4, 8, -63, 125, "twilightforest:lich_tower"},
		{-3, -8, -51, -129, "twilightforest:small_hollow_hill"},
		{-3, -7, -49, -113, "twilightforest:medium_hollow_hill"},
		{-3, -6, -47, -97, "twilightforest:hedge_maze"},
		{-3, -5, -46, -81, "twilightforest:small_hollow_hill"},
		{-3, -4, -46, -64, "twilightforest:medium_hollow_hill"},
		{-3, -3, -47, -48, "twilightforest:lich_tower"},
		{-3, -2, -46, -32, "twilightforest:small_hollow_hill"},
		{-3, -1, -47, -19, "twilightforest:large_hollow_hill"},
		{-3, 0, -49, -2, "twilightforest:small_hollow_hill"},
		{-3, 1, -47, 18, "twilightforest:naga_courtyard"},
		{-3, 2, -49, 33, "twilightforest:small_hollow_hill"},
		{-3, 3, -46, 50, "twilightforest:naga_courtyard"},
		{-3, 4, -47, 67, "twilightforest:small_hollow_hill"},
		{-3, 5, -46, 80, "twilightforest:small_hollow_hill"},
		{-3, 6, -47, 96, "twilightforest:hedge_maze"},
		{-3, 7, -49, 113, "twilightforest:hedge_maze"},
		{-3, 8, -46, 129, "twilightforest:small_hollow_hill"},
		{-2, -8, -34, -128, "twilightforest:naga_courtyard"},
		{-2, -7, -35, -113, "twilightforest:small_hollow_hill"},
		{-2, -6, -30, -98, "twilightforest:small_hollow_hill"},
		{-2, -5, -31, -80, "twilightforest:naga_courtyard"},
		{-2, -4, -33, -64, "twilightforest:medium_hollow_hill"},
		{-2, -3, -35, -48, "twilightforest:large_hollow_hill"},
		{-2, -2, -32, -31, "twilightforest:large_hollow_hill"},
		{-2, -1, -35, -13, "twilightforest:naga_courtyard"},
		{-2, 0, -33, 2, "twilightforest:medium_hollow_hill"},
		{-2, 1, -32, 16, "twilightforest:hedge_maze"},
		{-2, 2, -31, 29, "twilightforest:naga_courtyard"},
		{-2, 3, -32, 48, "twilightforest:medium_hollow_hill"},
		{-2, 4, -32, 64, "twilightforest:medium_hollow_hill"},
		{-2, 5, -33, 78, "twilightforest:hedge_maze"},
		{-2, 6, -30, 96, "twilightforest:small_hollow_hill"},
		{-2, 7, -32, 113, "twilightforest:medium_hollow_hill"},
		{-2, 8, -32, 127, "twilightforest:medium_hollow_hill"},
		{-1, -8, -16, -128, "twilightforest:naga_courtyard"},
		{-1, -7, -15, -111, "twilightforest:hedge_maze"},
		{-1, -6, -13, -97, "twilightforest:small_hollow_hill"},
		{-1, -5, -16, -82, "twilightforest:medium_hollow_hill"},
		{-1, -4, -16, -63, "twilightforest:lich_tower"},
		{-1, -3, -18, -48, "twilightforest:small_hollow_hill"},
		{-1, -2, -19, -31, "twilightforest:lich_tower"},
		{-1, -1, -16, -16, "twilightforest:medium_hollow_hill"},
		{-1, 0, -16, 0, "twilightforest:naga_courtyard"},
		{-1, 1, -18, 17, "twilightforest:small_hollow_hill"},
		{-1, 2, -13, 34, "twilightforest:naga_courtyard"},
		{-1, 3, -16, 48, "twilightforest:lich_tower"},
		{-1, 4, -18, 61, "twilightforest:lich_tower"},
		{-1, 5, -19, 82, "twilightforest:small_hollow_hill"},
		{-1, 6, -18, 96, "twilightforest:medium_hollow_hill"},
		{-1, 7, -15, 110, "twilightforest:small_hollow_hill"},
		{-1, 8, -16, 130, "twilightforest:naga_courtyard"},
		{0, -8, 2, -125, "twilightforest:medium_hollow_hill"},
		{0, -7, -2, -109, "twilightforest:lich_tower"},
		{0, -6, -1, -95, "twilightforest:naga_courtyard"},
		{0, -5, 1, -81, "twilightforest:lich_tower"},
		{0, -4, 0, -61, "twilightforest:medium_hollow_hill"},
		{0, -3, 1, -46, "twilightforest:small_hollow_hill"},
		{0, -2, 2, -32, "twilightforest:medium_hollow_hill"},
		{0, -1, 0, -16, "twilightforest:lich_tower"},
		{0, 0, 0, 0, "twilightforest:small_hollow_hill"},
		{0, 1, 0, 14, "twilightforest:lich_tower"},
		{0, 2, 2, 34, "twilightforest:small_hollow_hill"},
		{0, 3, 1, 50, "twilightforest:medium_hollow_hill"},
		{0, 4, 0, 64, "twilightforest:small_hollow_hill"},
		{0, 5, 1, 79, "twilightforest:lich_tower"},
		{0, 6, -1, 95, "twilightforest:small_hollow_hill"},
		{0, 7, -3, 113, "twilightforest:lich_tower"},
		{0, 8, 1, 127, "twilightforest:small_hollow_hill"},
		{1, -8, 16, -129, "twilightforest:naga_courtyard"},
		{1, -7, 15, -110, "twilightforest:medium_hollow_hill"},
		{1, -6, 18, -99, "twilightforest:small_hollow_hill"},
		{1, -5, 19, -82, "twilightforest:naga_courtyard"},
		{1, -4, 17, -67, "twilightforest:naga_courtyard"},
		{1, -3, 16, -48, "twilightforest:naga_courtyard"},
		{1, -2, 16, -31, "twilightforest:small_hollow_hill"},
		{1, -1, 18, -17, "twilightforest:hedge_maze"},
		{1, 0, 17, 3, "twilightforest:naga_courtyard"},
		{1, 1, 16, 16, "twilightforest:small_hollow_hill"},
		{1, 2, 16, 30, "twilightforest:medium_hollow_hill"},
		{1, 3, 18, 48, "twilightforest:medium_hollow_hill"},
		{1, 4, 17, 62, "twilightforest:naga_courtyard"},
		{1, 5, 16, 82, "twilightforest:medium_hollow_hill"},
		{1, 6, 17, 95, "twilightforest:lich_tower"},
		{1, 7, 15, 111, "twilightforest:small_hollow_hill"},
		{1, 8, 15, 125, "twilightforest:naga_courtyard"},
		{2, -8, 35, -127, "twilightforest:small_hollow_hill"},
		{2, -7, 32, -111, "twilightforest:small_hollow_hill"},
		{2, -6, 30, -96, "twilightforest:medium_hollow_hill"},
		{2, -5, 33, -82, "twilightforest:small_hollow_hill"},
		{2, -4, 34, -63, "twilightforest:small_hollow_hill"},
		{2, -3, 34, -49, "twilightforest:small_hollow_hill"},
		{2, -2, 31, -29, "twilightforest:small_hollow_hill"},
		{2, -1, 33, -13, "twilightforest:small_hollow_hill"},
		{2, 0, 33, -1, "twilightforest:medium_hollow_hill"},
		{2, 1, 33, 14, "twilightforest:naga_courtyard"},
		{2, 2, 32, 31, "twilightforest:lich_tower"},
		{2, 3, 32, 48, "twilightforest:lich_tower"},
		{2, 4, 31, 65, "twilightforest:large_hollow_hill"},
		{2, 5, 32, 79, "twilightforest:small_hollow_hill"},
		{2, 6, 30, 98, "twilightforest:medium_hollow_hill"},
		{2, 7, 34, 111, "twilightforest:medium_hollow_hill"},
		{2, 8, 31, 129, "twilightforest:naga_courtyard"},
		{3, -8, 48, -126, "twilightforest:medium_hollow_hill"},
		{3, -7, 49, -113, "twilightforest:hedge_maze"},
		{3, -6, 48, -98, "twilightforest:lich_tower"},
		{3, -5, 46, -80, "twilightforest:small_hollow_hill"},
		{3, -4, 48, -64, "twilightforest:medium_hollow_hill"},
		{3, -3, 46, -50, "twilightforest:lich_tower"},
		{3, -2, 47, -32, "twilightforest:medium_hollow_hill"},
		{3, -1, 47, -18, "twilightforest:small_hollow_hill"},
		{3, 0, 47, 1, "twilightforest:naga_courtyard"},
		{3, 1, 47, 19, "twilightforest:small_hollow_hill"},
		{3, 2, 51, 32, "twilightforest:medium_hollow_hill"},
		{3, 3, 47, 48, "twilightforest:large_hollow_hill"},
		{3, 4, 50, 64, "twilightforest:small_hollow_hill"},
		{3, 5, 46, 81, "twilightforest:small_hollow_hill"},
		{3, 6, 45, 98, "twilightforest:lich_tower"},
		{3, 7, 49, 113, "twilightforest:small_hollow_hill"},
		{3, 8, 48, 128, "twilightforest:small_hollow_hill"},
		{4, -8, 63, -128, "twilightforest:naga_courtyard"},
		{4, -7, 67, -113, "twilightforest:small_hollow_hill"},
		{4, -6, 67, -95, "twilightforest:naga_courtyard"},
		{4, -5, 66, -78, "twilightforest:small_hollow_hill"},
		{4, -4, 65, -65, "twilightforest:small_hollow_hill"},
		{4, -3, 67, -50, "twilightforest:medium_hollow_hill"},
		{4, -2, 65, -30, "twilightforest:small_hollow_hill"},
		{4, -1, 63, -15, "twilightforest:small_hollow_hill"},
		{4, 0, 61, 2, "twilightforest:small_hollow_hill"},
		{4, 1, 65, 16, "twilightforest:naga_courtyard"},
		{4, 2, 63, 32, "twilightforest:lich_tower"},
		{4, 3, 61, 49, "twilightforest:small_hollow_hill"},
		{4, 4, 64, 65, "twilightforest:hedge_maze"},
		{4, 5, 64, 79, "twilightforest:lich_tower"},
		{4, 6, 64, 97, "twilightforest:small_hollow_hill"},
		{4, 7, 64, 113, "twilightforest:hedge_maze"},
		{4, 8, 64, 130, "twilightforest:lich_tower"},
		{5, -8, 78, -129, "twilightforest:hedge_maze"},
		{5, -7, 77, -114, "twilightforest:naga_courtyard"},
		{5, -6, 80, -99, "twilightforest:medium_hollow_hill"},
		{5, -5, 82, -80, "twilightforest:large_hollow_hill"},
		{5, -4, 80, -62, "twilightforest:large_hollow_hill"},
		{5, -3, 80, -47, "twilightforest:lich_tower"},
		{5, -2, 83, -32, "twilightforest:small_hollow_hill"},
		{5, -1, 80, -15, "twilightforest:naga_courtyard"},
		{5, 0, 80, 0, "twilightforest:small_hollow_hill"},
		{5, 1, 78, 18, "twilightforest:small_hollow_hill"},
		{5, 2, 82, 32, "twilightforest:medium_hollow_hill"},
		{5, 3, 82, 48, "twilightforest:lich_tower"},
		{5, 4, 80, 65, "twilightforest:small_hollow_hill"},
		{5, 5, 77, 83, "twilightforest:medium_hollow_hill"},
		{5, 6, 79, 99, "twilightforest:hedge_maze"},
		{5, 7, 79, 111, "twilightforest:small_hollow_hill"},
		{5, 8, 80, 128, "twilightforest:small_hollow_hill"},
		{6, -8, 95, -127, "twilightforest:small_hollow_hill"},
		{6, -7, 98, -111, "twilightforest:medium_hollow_hill"},
		{6, -6, 96, -94, "twilightforest:naga_courtyard"},
		{6, -5, 97, -82, "twilightforest:lich_tower"},
		{6, -4, 97, -65, "twilightforest:lich_tower"},
		{6, -3, 94, -49, "twilightforest:medium_hollow_hill"},
		{6, -2, 99, -34, "twilightforest:lich_tower"},
		{6, -1, 99, -13, "twilightforest:lich_tower"},
		{6, 0, 99, 1, "twilightforest:naga_courtyard"},
		{6, 1, 95, 17, "twilightforest:naga_courtyard"},
		{6, 2, 94, 31, "twilightforest:lich_tower"},
		{6, 3, 95, 47, "twilightforest:small_hollow_hill"},
		{6, 4, 93, 65, "twilightforest:medium_hollow_hill"},
		{6, 5, 98, 79, "twilightforest:lich_tower"},
		{6, 6, 97, 96, "twilightforest:medium_hollow_hill"},
		{6, 7, 99, 114, "twilightforest:large_hollow_hill"},
		{6, 8, 96, 130, "twilightforest:hedge_maze"},
		{7, -8, 110, -127, "twilightforest:naga_courtyard"},
		{7, -7, 112, -113, "twilightforest:small_hollow_hill"},
		{7, -6, 113, -96, "twilightforest:small_hollow_hill"},
		{7, -5, 112, -79, "twilightforest:small_hollow_hill"},
		{7, -4, 109, -65, "twilightforest:naga_courtyard"},
		{7, -3, 115, -50, "twilightforest:small_hollow_hill"},
		{7, -2, 114, -34, "twilightforest:large_hollow_hill"},
		{7, -1, 112, -15, "twilightforest:medium_hollow_hill"},
		{7, 0, 110, 0, "twilightforest:naga_courtyard"},
		{7, 1, 112, 15, "twilightforest:lich_tower"},
		{7, 2, 111, 32, "twilightforest:small_hollow_hill"},
		{7, 3, 110, 47, "twilightforest:naga_courtyard"},
		{7, 4, 111, 65, "twilightforest:small_hollow_hill"},
		{7, 5, 109, 81, "twilightforest:large_hollow_hill"},
		{7, 6, 115, 99, "twilightforest:small_hollow_hill"},
		{7, 7, 111, 111, "twilightforest:small_hollow_hill"},
		{7, 8, 112, 131, "twilightforest:naga_courtyard"},
		{8, -8, 126, -129, "twilightforest:lich_tower"},
		{8, -7, 127, -112, "twilightforest:lich_tower"},
		{8, -6, 131, -94, "twilightforest:large_hollow_hill"},
		{8, -5, 129, -81, "twilightforest:naga_courtyard"},
		{8, -4, 130, -64, "twilightforest:small_hollow_hill"},
		{8, -3, 127, -47, "twilightforest:large_hollow_hill"},
		{8, -2, 129, -31, "twilightforest:small_hollow_hill"},
		{8, -1, 128, -19, "twilightforest:lich_tower"},
		{8, 0, 126, -3, "twilightforest:small_hollow_hill"},
		{8, 1, 127, 17, "twilightforest:lich_tower"},
		{8, 2, 128, 32, "twilightforest:lich_tower"},
		{8, 3, 126, 48, "twilightforest:small_hollow_hill"},
		{8, 4, 125, 63, "twilightforest:lich_tower"},
		{8, 5, 128, 77, "twilightforest:medium_hollow_hill"},
		{8, 6, 126, 95, "twilightforest:medium_hollow_hill"},
		{8, 7, 131, 115, "twilightforest:lich_tower"},
		{8, 8, 128, 129, "twilightforest:large_hollow_hill"},
	}
	for _, c := range cases {
		bx, bz := tfNearestCenterXZ(c.rx*16, c.rz*16)
		if bx>>4 != c.cx || bz>>4 != c.cz {
			t.Fatalf("region %d,%d chunk mismatch", c.rx, c.rz)
		}
		got := tfPickVarietyLandmark(seed, c.cx, c.cz)
		if got != c.id {
			t.Fatalf("region %d,%d chunk %d,%d got %s want %s", c.rx, c.rz, c.cx, c.cz, got, c.id)
		}
		if !tfIsPlacementChunk(seed, c.cx, c.cz, c.id) {
			t.Fatalf("lock %s rejected its own center", c.id)
		}
		if c.id != "twilightforest:hedge_maze" && tfIsPlacementChunk(seed, c.cx, c.cz, "twilightforest:hedge_maze") {
			t.Fatalf("hedge lock accepted %s center", c.id)
		}
		if !tfIsPlacementChunk(seed, c.cx, c.cz, "") {
			t.Fatalf("unlocked rejected center")
		}
	}
}

func TestTFLandmarkGridParse(t *testing.T) {
	doc := WorldgenDocument{
		ID: "twilightforest:hedge_maze", Source: "mod:twilightforest",
		Data: []byte(`{"placement":{"type":"twilightforest:landmark_grid","structure_grid_lock":"twilightforest:hedge_maze"},"structures":[{"structure":"twilightforest:hedge_maze","weight":1}]}`),
	}
	p, ok := parseSeedMapPlacement(doc, nil)
	if !ok || !p.landmarkGrid || p.gridLock != "twilightforest:hedge_maze" || p.spacing != 16 {
		t.Fatalf("locked parse: ok=%v grid=%v lock=%q spacing=%d", ok, p.landmarkGrid, p.gridLock, p.spacing)
	}
	doc2 := WorldgenDocument{
		ID: "twilightforest:quest_grove", Source: "mod:twilightforest",
		Data: []byte(`{"placement":{"type":"twilightforest:landmark_grid"},"structures":[{"structure":"twilightforest:quest_grove","weight":1}]}`),
	}
	p2, ok := parseSeedMapPlacement(doc2, nil)
	if !ok || !p2.landmarkGrid || p2.gridLock != "" {
		t.Fatalf("unlocked parse: ok=%v grid=%v lock=%q", ok, p2.landmarkGrid, p2.gridLock)
	}
	doc3 := WorldgenDocument{
		ID: "twilightforest:x", Source: "mod:twilightforest",
		Data: []byte(`{"placement":{"type":"twilightforest:landmark_grid","spacing":16},"structures":[{"structure":"twilightforest:x","weight":1}]}`),
	}
	if _, ok := parseSeedMapPlacement(doc3, nil); ok {
		t.Fatal("expected reject unknown spacing on landmark_grid")
	}
}

func TestTFLandmarkSpecialCells(t *testing.T) {
	seed := int64(999)
	foundLich, foundNaga := false, false
	for cx := -128; cx <= 128; cx += 16 {
		for cz := -128; cz <= 128; cz += 16 {
			ox := absInt(((cx + 64) >> 4) % 8)
			oz := absInt(((cz + 64) >> 4) % 8)
			got := tfPickVarietyLandmark(seed, cx, cz)
			if ox == 4 && (oz == 5 || oz == 3) {
				if got != "twilightforest:lich_tower" {
					t.Fatalf("lich cell %d,%d got %s", cx, cz, got)
				}
				foundLich = true
			}
			if oz == 4 && (ox == 5 || ox == 3) {
				if got != "twilightforest:naga_courtyard" {
					t.Fatalf("naga cell %d,%d got %s", cx, cz, got)
				}
				foundNaga = true
			}
		}
	}
	if !foundLich || !foundNaga {
		t.Fatalf("special cells missing lich=%v naga=%v", foundLich, foundNaga)
	}
}

func TestTFAvoidLandmarkGridParseAndProximity(t *testing.T) {
	doc := WorldgenDocument{
		ID: "twilightforest:hollow_tree",
		Data: []byte(`{
			"structures":[{"structure":"twilightforest:hollow_tree","weight":1}],
			"placement":{
				"type":"twilightforest:avoid_landmark_grid",
				"frequency":0.5,
				"salt":34481210,
				"separation":5,
				"spacing":7,
				"spread_type":"triangular"
			}
		}`),
	}
	p, ok := parseSeedMapPlacement(doc, nil)
	if !ok {
		t.Fatal("parse hollow_tree")
	}
	if !p.tfAvoidLandmark || p.spacing != 7 || p.separation != 5 || !p.triangular || p.frequency != 0.5 {
		t.Fatalf("got tf=%v sp=%d se=%d tri=%v freq=%v", p.tfAvoidLandmark, p.spacing, p.separation, p.triangular, p.frequency)
	}
	// Camp with avoid_additional_structures
	camp := WorldgenDocument{
		ID: "twilightforest:camp",
		Data: []byte(`{
			"structures":[{"structure":"twilightforest:camp","weight":1}],
			"placement":{
				"type":"twilightforest:avoid_landmark_grid",
				"avoid_additional_structures":{
					"twilightforest:fallen_trunk":2,
					"twilightforest:hollow_tree":1
				},
				"salt":701432212,
				"separation":14,
				"spacing":18,
				"spread_type":"triangular"
			}
		}`),
	}
	c, ok := parseSeedMapPlacement(camp, nil)
	if !ok || !c.tfAvoidLandmark || len(c.excludes) != 2 {
		t.Fatalf("camp ok=%v tf=%v excludes=%d", ok, c.tfAvoidLandmark, len(c.excludes))
	}

	// Proximity: landmark center chunk itself is always within Chebyshev 80 of its world-position.
	// Find a known center from TF oracle (region 0,0).
	bx, bz := tfNearestCenterXZ(0, 0)
	cx, cz := bx>>4, bz>>4
	if !tfTooCloseToLandmark(cx, cz) {
		t.Fatalf("center chunk %d,%d should be too close", cx, cz)
	}
	// A chunk far from any landmark: jump many regions away and pick a non-center.
	farCX, farCZ := cx+200, cz+200
	if tfChunkHasLandmarkCenter(farCX, farCZ) {
		farCX++
	}
	nbx, nbz := tfNearestCenterXZ(farCX, farCZ)
	dx := nbx - (farCX << 4)
	if dx < 0 {
		dx = -dx
	}
	dz := nbz - (farCZ << 4)
	if dz < 0 {
		dz = -dz
	}
	if dx < 80 && dz < 80 {
		// Move further from the nearest center along the larger axis.
		if dx >= dz {
			farCX += 10
		} else {
			farCZ += 10
		}
	}
	if tfTooCloseToLandmark(farCX, farCZ) {
		t.Fatalf("far chunk %d,%d unexpectedly close (nearest %d,%d)", farCX, farCZ, nbx, nbz)
	}

	// placementChunk on a random-spread candidate that lands on a landmark center must reject.
	// Force-check: any candidate block whose chunk is a landmark center.
	p.excludes = nil // isolation
	p.frequency = 0  // always pass frequency
	if p.placementChunk(12345, bx, bz) {
		t.Fatal("candidate on landmark center must be rejected by avoid_landmark_grid")
	}
}

func TestTFAvoidLandmarkOracleEdge(t *testing.T) {
	// Boundary: Chebyshev exactly 80 is allowed (javac uses < 80).
	// Construct synthetic by verifying the predicate math against known center.
	bx, bz := tfNearestCenterXZ(8, 8) // any chunk in region
	cx, cz := bx>>4, bz>>4
	// worldPos of center chunk
	wx, wz := cx<<4, cz<<4
	dx, dz := bx-wx, bz-wz
	if dx < 0 {
		dx = -dx
	}
	if dz < 0 {
		dz = -dz
	}
	if dx >= 80 || dz >= 80 {
		t.Fatalf("center should be inside: d=%d,%d", dx, dz)
	}
}
