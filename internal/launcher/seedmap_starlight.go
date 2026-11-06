package launcher

import (
	"encoding/json"
	"math"
	"strings"
)

// Eternal Starlight landmark / avoid_landmark (LandmarkStructurePlacement +
// AvoidLandmarkStructurePlacement, javap 0.6.3+1.21.1).

const (
	esLandmarkType      = "eternal_starlight:landmark"
	esAvoidLandmarkType = "eternal_starlight:avoid_landmark"
	esRegionChunks      = 32
)

// Hard-coded STRUCTURES list order from LandmarkStructurePlacement.<clinit>.
var esLandmarkInfos = []struct {
	id  string
	min int // minSpawnDistance in chunks
}{
	{"eternal_starlight:golem_forge", 20},
	{"eternal_starlight:cursed_garden", 40},
}

// esRegionLandmarkPos mirrors getRegionLandmarkPos(state, chunkX, chunkZ).
// worldSeed is ChunkGeneratorStructureState.getLevelSeed().
func esRegionLandmarkPos(worldSeed int64, chunkX, chunkZ int) (cx, cz int) {
	x := (chunkX>>5)*32 + 16
	z := (chunkZ>>5)*32 + 16
	// All mixes are Java int32 arithmetic (silent wrap).
	seed := int32(worldSeed)
	xi, zi := int32(x), int32(z)
	seedX := seed - xi*20090707 + zi*2024
	seedZ := seed + xi*10370 - zi*1274
	fx := math.Sin(float64(seedX))*10000 - math.Floor(math.Sin(float64(seedX))*10000)
	fz := math.Sin(float64(seedZ))*10000 - math.Floor(math.Sin(float64(seedZ))*10000)
	x = x + int(-8.0+fx*16.0) // Java (int) truncates toward zero
	z = z + int(-8.0+fz*16.0)
	return x, z
}

func esFloorMod(x, y int) int {
	if y <= 0 {
		return 0
	}
	r := x % y
	if r < 0 {
		r += y
	}
	return r
}

// esEligibleLandmarks filters STRUCTURES by chunk distance from origin.
func esEligibleLandmarks(chunkX, chunkZ int) []string {
	d2 := float64(chunkX)*float64(chunkX) + float64(chunkZ)*float64(chunkZ)
	var out []string
	for _, info := range esLandmarkInfos {
		if d2 >= math.Pow(float64(info.min), 2) {
			out = append(out, info.id)
		}
	}
	return out
}

// esPickLandmark returns the landmark id at a center chunk, or "".
func esPickLandmark(worldSeed int64, chunkX, chunkZ int) string {
	eligible := esEligibleLandmarks(chunkX, chunkZ)
	if len(eligible) == 0 {
		return ""
	}
	cx, cz := esRegionLandmarkPos(worldSeed, chunkX, chunkZ)
	if cx != chunkX || cz != chunkZ {
		return ""
	}
	return eligible[esFloorMod((chunkX>>5)+(chunkZ>>5), len(eligible))]
}

// esIsLandmarkPlacementChunk mirrors LandmarkStructurePlacement.isPlacementChunk
// for a placement locked to landmarkID.
func esIsLandmarkPlacementChunk(worldSeed int64, chunkX, chunkZ int, landmarkID string) bool {
	return esPickLandmark(worldSeed, chunkX, chunkZ) == seedMapID(landmarkID)
}

// esIsLandmarkCenter is true when (chunkX,chunkZ) is the region's landmark chunk
// (used by avoid_landmark regardless of which structure wins the pick).
func esIsLandmarkCenter(worldSeed int64, chunkX, chunkZ int) bool {
	cx, cz := esRegionLandmarkPos(worldSeed, chunkX, chunkZ)
	return cx == chunkX && cz == chunkZ
}

// esLandmarkCandidateBlock returns the start block for one 32-chunk region when
// the lock matches. regionX/Z are chunk>>5 indices.
func esLandmarkCandidateBlock(worldSeed int64, regionX, regionZ int, lock string) (x, z int, ok bool) {
	probeX, probeZ := regionX*esRegionChunks, regionZ*esRegionChunks
	cx, cz := esRegionLandmarkPos(worldSeed, probeX, probeZ)
	if !esIsLandmarkPlacementChunk(worldSeed, cx, cz, lock) {
		return 0, 0, false
	}
	return cx << 4, cz << 4, true
}

func parseESLandmark(placement map[string]json.RawMessage) (lock string, ok bool) {
	for key, raw := range placement {
		switch key {
		case "type":
			var kind string
			if json.Unmarshal(raw, &kind) != nil || seedMapID(kind) != esLandmarkType {
				return "", false
			}
		case "landmark":
			if json.Unmarshal(raw, &lock) != nil || lock == "" {
				return "", false
			}
			lock = seedMapID(lock)
		case "frequency":
			var f float64
			if json.Unmarshal(raw, &f) != nil || f != 1 {
				return "", false
			}
		case "salt", "locate_offset":
		default:
			return "", false
		}
	}
	if lock == "" {
		return "", false // codec fieldOf("landmark") is required
	}
	return lock, true
}

func isESLandmarkType(kind string) bool {
	return seedMapID(strings.TrimSpace(kind)) == esLandmarkType
}

func isESAvoidLandmarkType(kind string) bool {
	return seedMapID(strings.TrimSpace(kind)) == esAvoidLandmarkType
}
