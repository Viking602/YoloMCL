package launcher

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// Twilight Forest landmark_grid (LandmarkGridPlacement + LegacyLandmarkPlacements).
// Centers are seed-independent; variety/lock picks use the overworld WorldOptions seed.
const tfLandmarkGridType = "twilightforest:landmark_grid"
const tfAvoidLandmarkGridType = "twilightforest:avoid_landmark_grid"
const tfAvoidLandmarkBlockRadius = 80 // AvoidLandmarkGridPlacement.isPlacementChunk

// Variety landmarks and weights match LegacyLandmarkPlacements.VARIETY_LANDMARKS
// (SimpleWeightedRandomList builder order).
var tfVarietyLandmarks = []struct {
	id string
	w  int
}{
	{"twilightforest:small_hollow_hill", 6},
	{"twilightforest:medium_hollow_hill", 3},
	{"twilightforest:large_hollow_hill", 1},
	{"twilightforest:hedge_maze", 2},
	{"twilightforest:naga_courtyard", 2},
	{"twilightforest:lich_tower", 2},
}

const tfVarietyTotalWeight = 16

// tfJavaRound mirrors Java Math.round(float): (int) Math.floor(a + 0.5f).
func tfJavaRound(a float32) int {
	return int(math.Floor(float64(a) + 0.5))
}

// tfNearestCenterXZ mirrors LegacyLandmarkPlacements.getNearestCenterXZ.
// Returns block coordinates of the landmark center for the 16-chunk region.
func tfNearestCenterXZ(chunkX, chunkZ int) (blockX, blockZ int) {
	regionX := (chunkX + 8) >> 4
	regionZ := (chunkZ + 8) >> 4
	seed := int64(regionX)*3129871 ^ int64(regionZ)*116129781
	seed = seed*seed*42317861 + seed*7
	num0 := int(seed >> 12 & 3)
	num1 := int(seed >> 15 & 3)
	num2 := int(seed >> 18 & 3)
	num3 := int(seed >> 21 & 3)
	centerX := 8 + num0 - num1
	centerZ := 8 + num2 - num3
	if regionZ >= 0 {
		blockZ = (regionZ*16+centerZ-8)*16 + 8
	} else {
		blockZ = (regionZ*16+(16-centerZ)-8)*16 + 9
	}
	if regionX >= 0 {
		blockX = (regionX*16+centerX-8)*16 + 8
	} else {
		blockX = (regionX*16+(16-centerX)-8)*16 + 9
	}
	return blockX, blockZ
}

func tfChunkHasLandmarkCenter(chunkX, chunkZ int) bool {
	bx, bz := tfNearestCenterXZ(chunkX, chunkZ)
	return chunkX == bx>>4 && chunkZ == bz>>4
}

// tfRegionOfChunk is (chunk + 8) >> 4 — TF's 16-chunk region index.
func tfRegionOfChunk(chunk int) int { return (chunk + 8) >> 4 }

// tfPickVarietyLandmark mirrors LegacyLandmarkPlacements.pickVarietyLandmark.
// worldSeed is WorldUtil.getOverworldSeed() (== WorldOptions.seed()).
func tfPickVarietyLandmark(worldSeed int64, chunkX, chunkZ int) string {
	chunkX = tfJavaRound(float32(chunkX)/16) * 16
	chunkZ = tfJavaRound(float32(chunkZ)/16) * 16
	regionOffsetX := absInt(((chunkX + 64) >> 4) % 8)
	regionOffsetZ := absInt(((chunkZ + 64) >> 4) % 8)
	if (regionOffsetX == 4 && regionOffsetZ == 5) || (regionOffsetX == 4 && regionOffsetZ == 3) {
		return "twilightforest:lich_tower"
	}
	if (regionOffsetX == 5 && regionOffsetZ == 4) || (regionOffsetX == 3 && regionOffsetZ == 4) {
		return "twilightforest:naga_courtyard"
	}
	r := &worldgen.JavaRandom{}
	r.SetSeed(worldSeed + int64(chunkX)*25117 + int64(chunkZ)*151121)
	i := int(r.NextInt(tfVarietyTotalWeight))
	for _, e := range tfVarietyLandmarks {
		i -= e.w
		if i < 0 {
			return e.id
		}
	}
	return tfVarietyLandmarks[0].id
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// parseTFLandmarkGrid reads type twilightforest:landmark_grid and optional
// structure_grid_lock. Empty lock = unlocked (every center is a candidate).
func parseTFLandmarkGrid(placement map[string]json.RawMessage) (lock string, ok bool) {
	for key, raw := range placement {
		switch key {
		case "type":
			var kind string
			if json.Unmarshal(raw, &kind) != nil || seedMapID(kind) != tfLandmarkGridType {
				return "", false
			}
		case "structure_grid_lock":
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
			// Parent StructurePlacement fields; LandmarkGridPlacement ignores them.
		default:
			return "", false
		}
	}
	return lock, true
}

// tfIsPlacementChunk mirrors LandmarkGridPlacement.isPlacementChunk.
func tfIsPlacementChunk(worldSeed int64, chunkX, chunkZ int, lock string) bool {
	if !tfChunkHasLandmarkCenter(chunkX, chunkZ) {
		return false
	}
	if lock == "" {
		return true
	}
	return tfPickVarietyLandmark(worldSeed, chunkX, chunkZ) == lock
}

// tfLandmarkCandidateBlock returns the structure start block (chunk origin) for
// one TF region index (rx, rz), or false when the lock rejects the center.
func tfLandmarkCandidateBlock(worldSeed int64, regionX, regionZ int, lock string) (x, z int, ok bool) {
	bx, bz := tfNearestCenterXZ(regionX*16, regionZ*16)
	cx, cz := bx>>4, bz>>4
	if !tfIsPlacementChunk(worldSeed, cx, cz, lock) {
		return 0, 0, false
	}
	return cx << 4, cz << 4, true
}

func isTFLandmarkGridType(kind string) bool {
	return seedMapID(strings.TrimSpace(kind)) == tfLandmarkGridType
}

func isTFAvoidLandmarkGridType(kind string) bool {
	return seedMapID(strings.TrimSpace(kind)) == tfAvoidLandmarkGridType
}

// tfTooCloseToLandmark mirrors AvoidLandmarkGridPlacement.isPlacementChunk's
// landmark proximity check: Chebyshev distance from ChunkPos.getWorldPosition
// (chunkX<<4, chunkZ<<4) to getNearestCenterXZ must be < 80 blocks to reject.
func tfTooCloseToLandmark(chunkX, chunkZ int) bool {
	bx, bz := tfNearestCenterXZ(chunkX, chunkZ)
	dx := bx - (chunkX << 4)
	if dx < 0 {
		dx = -dx
	}
	dz := bz - (chunkZ << 4)
	if dz < 0 {
		dz = -dz
	}
	return dx < tfAvoidLandmarkBlockRadius && dz < tfAvoidLandmarkBlockRadius
}
