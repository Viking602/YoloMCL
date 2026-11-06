package worldgen

import "math"

// Data2D maps a HorizontalFaces index to the saved piece "O" value.
var Data2D = [4]int32{2, 3, 0, 1}

// MonumentStart mirrors OceanMonumentStructure.generatePieces: the building
// sits at chunk min − 29 on both axes, Y sea level − 24, 58×23×58, facing a random
// horizontal direction (index into HorizontalFaces).
func MonumentStart(seed int64, cx, cz int32, seaLevel int) (Box, int) {
	r := LargeFeatureRandom(seed, cx, cz)
	dir := int(r.NextInt(4))
	x, z := int(cx)*16-29, int(cz)*16-29
	y := seaLevel - 24
	return Box{x, y, z, x + 57, y + 22, z + 57}, dir
}

// rotationOffset5 is the 5-block corner offset used by woodland mansions and
// end cities when sampling the lowest surface for a rotation.
func rotationOffset5(rot int32) (int, int) {
	switch rot {
	case 1:
		return -5, 5
	case 2:
		return -5, -5
	case 3:
		return 5, -5
	}
	return 5, 5
}

// RotatedStart mirrors the shared findGenerationPoint prefix of
// WoodlandMansionStructure and EndCityStructure: Rotation.getRandom, then the
// start column chunk+(7,7) with the four corners (offset by the rotation)
// whose lowest surface becomes the start Y (rejected below 60).
func RotatedStart(seed int64, cx, cz int32) (rotation string, x, z int, corners [4][2]int) {
	r := LargeFeatureRandom(seed, cx, cz)
	rot := r.NextInt(4)
	x, z = int(cx)*16+7, int(cz)*16+7
	dx, dz := rotationOffset5(rot)
	corners = [4][2]int{{x, z}, {x, z + dz}, {x + dx, z}, {x + dx, z + dz}}
	return Rotations[rot], x, z, corners
}

// ConcentricRings mirrors ChunkGeneratorStructureState.generateRingPositions.
// inBiome reports whether the biome at quart (qx, qz) at Y quart 0 is in the
// placement's preferred_biomes. It returns start chunk coordinates.
func ConcentricRings(seed int64, distance, spread, count int, inBiome func(qx, qz int) bool) [][2]int32 {
	return ConcentricRingsWith(seed, distance, spread, count, nil, inBiome)
}

// ConcentricRingsWith calls begin before each ring's biome search, so the
// caller can reset per-search biome lookup history (MC-241546).
func ConcentricRingsWith(seed int64, distance, spread, count int, begin func(), inBiome func(qx, qz int) bool) [][2]int32 {
	r := &JavaRandom{}
	r.SetSeed(seed)
	angle := r.NextDouble() * math.Pi * 2
	out := make([][2]int32, 0, count)
	inRing, ring := 0, 0
	for n := 0; n < count; n++ {
		dist := float64(4*distance+distance*ring*6) + (r.NextDouble()-0.5)*(float64(distance)*2.5)
		o := int32(math.Floor(math.Cos(angle)*dist + 0.5))
		p := int32(math.Floor(math.Sin(angle)*dist + 0.5))
		fork := &JavaRandom{}
		fork.SetSeed(r.NextLong())
		if begin != nil {
			begin()
		}
		out = append(out, snapRing(o, p, fork, inBiome))
		angle += math.Pi * 2 / float64(spread)
		inRing++
		if inRing == spread {
			ring++
			inRing = 0
			spread += 2 * spread / (ring + 1)
			spread = min(spread, count-n)
			angle += r.NextDouble() * math.Pi * 2
		}
	}
	return out
}

// snapRing is BiomeSource.findBiomeHorizontal(center, 0, radius 112) with
// reservoir choice, then the containing chunk.
func snapRing(o, p int32, r *JavaRandom, inBiome func(qx, qz int) bool) [2]int32 {
	qx, qz := (int(o)*16+8)>>2, (int(p)*16+8)>>2
	const k = 112 >> 2
	found, m := false, 0
	var bx, bz int
	for dz := -k; dz <= k; dz++ {
		for dx := -k; dx <= k; dx++ {
			if !inBiome(qx+dx, qz+dz) {
				continue
			}
			if !found || r.NextInt(int32(m+1)) == 0 {
				bx, bz, found = (qx+dx)*4, (qz+dz)*4, true
			}
			m++
		}
	}
	if !found {
		return [2]int32{o, p}
	}
	return [2]int32{int32(bx >> 4), int32(bz >> 4)}
}

// MansionEntrance is the seeded part of a woodland mansion that is verified
// against saves: the rotation and the entrance template origin (start column
// moved 9 blocks along the rotated −X axis). The room grid is not ported.
func MansionEntrance(seed int64, cx, cz int32) (rotation string, tpx, tpz int) {
	rotation, x, z, _ := RotatedStart(seed, cx, cz)
	switch rotation {
	case "CLOCKWISE_90":
		return rotation, x, z - 9
	case "CLOCKWISE_180":
		return rotation, x + 9, z
	case "COUNTERCLOCKWISE_90":
		return rotation, x, z + 9
	}
	return rotation, x - 9, z
}
