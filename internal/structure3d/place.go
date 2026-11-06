package structure3d

import "github.com/Viking602/YoloMCL/internal/worldgen"

// appendTemplate appends the template's blocks at templatePosition with the
// given rotation and mirror (StructureTemplate.transform, pivot 0). Structure
// blocks are skipped to match BlockIgnoreProcessor.STRUCTURE_BLOCK.
func appendTemplate(store *Store, id string, pos [3]int, rotation, mirror string, dst *[]WorldBlock) error {
	t, err := LoadTemplate(store, id)
	if err != nil {
		return err
	}
	for _, b := range t.Blocks {
		if b.State.Name == "minecraft:structure_block" {
			continue
		}
		w := worldgen.TransformMirror([3]int{b.X, b.Y, b.Z}, mirror, rotation, [3]int{})
		*dst = append(*dst, WorldBlock{
			X: pos[0] + w[0], Y: pos[1] + w[1], Z: pos[2] + w[2],
			State: rotateState(mirrorState(b.State, mirror), rotation),
		})
	}
	return nil
}
