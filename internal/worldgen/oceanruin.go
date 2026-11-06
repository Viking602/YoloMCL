package worldgen

import "strconv"

// ScatteredSize is the ScatteredFeaturePiece footprint (width, height, depth)
// of the code-built temples.
var ScatteredSize = map[string][3]int{
	"minecraft:desert_pyramid": {21, 15, 21},
	"minecraft:jungle_pyramid": {12, 10, 15},
	"minecraft:swamp_hut":      {7, 7, 9},
}

// ScatteredBox mirrors ScatteredFeaturePiece.makeBoundingBox: the piece sits at
// the chunk's min corner; east/west orientations swap width and depth.
func ScatteredBox(cx, cz int32, y int, size [3]int, orientation string) Box {
	x, z := int(cx)*16, int(cz)*16
	w, d := size[0], size[2]
	if orientation == "east" || orientation == "west" {
		w, d = d, w
	}
	return Box{x, y, z, x + w - 1, y + size[1] - 1, z + d - 1}
}

// BuriedTreasurePos is BuriedTreasurePieces' chest column (chunk + 9, 9).
func BuriedTreasurePos(cx, cz int32) (int, int) { return int(cx)*16 + 9, int(cz)*16 + 9 }

// OceanRuinConfig is the ocean_ruin structure JSON.
type OceanRuinConfig struct {
	BiomeTemp          string  `json:"biome_temp"`
	LargeProbability   float32 `json:"large_probability"`
	ClusterProbability float32 `json:"cluster_probability"`
}

// OceanRuinPiece is one placed ruin template (rotation pivot 0,0,0).
type OceanRuinPiece struct {
	Template  string
	Rotation  string
	Pos       [3]int
	Integrity float32
	Large     bool
}

func nums(prefix string, ids ...int) []string {
	out := make([]string, len(ids))
	for i, n := range ids {
		out[i] = "minecraft:underwater_ruin/" + prefix + strconv.Itoa(n)
	}
	return out
}

var (
	ruinWarm     = nums("warm_", 1, 2, 3, 4, 5, 6, 7, 8)
	ruinBigWarm  = nums("big_warm_", 4, 5, 6, 7)
	ruinBrick    = nums("brick_", 1, 2, 3, 4, 5, 6, 7, 8)
	ruinCracked  = nums("cracked_", 1, 2, 3, 4, 5, 6, 7, 8)
	ruinMossy    = nums("mossy_", 1, 2, 3, 4, 5, 6, 7, 8)
	ruinBigBrick = nums("big_brick_", 1, 2, 3, 8)
	ruinBigCrack = nums("big_cracked_", 1, 2, 3, 8)
	ruinBigMossy = nums("big_mossy_", 1, 2, 3, 8)
)

// mthNextInt is Mth.nextInt(random, min, max) (inclusive).
func mthNextInt(r *JavaRandom, lo, hi int) int {
	if lo >= hi {
		return lo
	}
	return int(r.NextInt(int32(hi-lo+1))) + lo
}

// OceanRuin mirrors OceanRuinStructure.generatePieces / OceanRuinPieces.
func OceanRuin(seed int64, cx, cz int32, cfg OceanRuinConfig) []OceanRuinPiece {
	r := LargeFeatureRandom(seed, cx, cz)
	pos := [3]int{int(cx) * 16, 90, int(cz) * 16}
	rot := Rotations[r.NextInt(4)]
	large := r.NextFloat() <= cfg.LargeProbability
	integrity := float32(0.8)
	if large {
		integrity = 0.9
	}
	var out []OceanRuinPiece
	add := func(p [3]int, rot string, large bool, integrity float32) {
		if cfg.BiomeTemp == "warm" {
			list := ruinWarm
			if large {
				list = ruinBigWarm
			}
			out = append(out, OceanRuinPiece{list[r.NextInt(int32(len(list)))], rot, p, integrity, large})
			return
		}
		b, c, m := ruinBrick, ruinCracked, ruinMossy
		if large {
			b, c, m = ruinBigBrick, ruinBigCrack, ruinBigMossy
		}
		i := r.NextInt(int32(len(b)))
		out = append(out, OceanRuinPiece{b[i], rot, p, integrity, large}, OceanRuinPiece{c[i], rot, p, 0.7, large}, OceanRuinPiece{m[i], rot, p, 0.5, large})
	}
	add(pos, rot, large, integrity)
	if large && r.NextFloat() <= cfg.ClusterProbability {
		far := Transform([3]int{15, 0, 15}, rot, [3]int{})
		far = [3]int{far[0] + pos[0], 90, far[2] + pos[2]}
		big := Box{min(pos[0], far[0]), 90, min(pos[2], far[2]), max(pos[0], far[0]), 90, max(pos[2], far[2])}
		bx, bz := big[0], big[2]
		n := func(a, b int) int { return mthNextInt(r, a, b) }
		var list [][3]int
		for _, o := range [][4]int{{-16, 1, 8, 0}, {-16, 1, 8, 1}, {-16, 1, 8, 2}, {0, 1, 7, 0}, {0, 1, 7, 3}, {16, 1, 7, 4}, {16, 1, 7, 1}, {16, 1, 7, 2}} {
			x := bx + o[0] + n(o[1], o[2])
			var z int
			switch o[3] {
			case 0:
				z = bz + 16 + n(1, 7)
			case 1:
				z = bz + n(1, 7)
			case 2:
				z = bz - 16 + n(4, 8)
			case 3:
				z = bz - 16 + n(4, 6)
			case 4:
				z = bz + 16 + n(3, 8)
			}
			list = append(list, [3]int{x, 90, z})
		}
		count := n(4, 8)
		for j := 0; j < count && len(list) > 0; j++ {
			k := r.NextInt(int32(len(list)))
			p := list[k]
			list = append(list[:k], list[k+1:]...)
			rot1 := Rotations[r.NextInt(4)]
			e := Transform([3]int{5, 0, 6}, rot1, [3]int{})
			e = [3]int{e[0] + p[0], 90, e[2] + p[2]}
			small := Box{min(p[0], e[0]), 90, min(p[2], e[2]), max(p[0], e[0]), 90, max(p[2], e[2])}
			if !small.Intersects(big) {
				add(p, rot1, false, 0.8)
			}
		}
	}
	return out
}
