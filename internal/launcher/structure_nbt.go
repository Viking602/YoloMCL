package launcher

import "github.com/Viking602/YoloMCL/internal/structure3d"

// DecodeStructureNBT reads a gzipped or raw structure-template NBT compound.
// Seed-map structure preview uses this path; level.dat streaming stays in
// world_nbt.go's specialized nbtReader.
func DecodeStructureNBT(data []byte) (map[string]any, error) {
	return structure3d.DecodeNBT(data)
}
