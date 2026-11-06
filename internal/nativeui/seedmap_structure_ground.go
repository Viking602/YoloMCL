package nativeui

import (
	"github.com/Viking602/YoloMCL/internal/launcher"
	"github.com/Viking602/YoloMCL/internal/structure3d"
	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// StructureGround is the noise-stage terrain (with carver caves when the
// seedmap generates them) as the code-built structure pieces see it: they
// skip air, stop at liquid and rest on solid ground. Noise-stage only:
// surface rules, aquifer details and features are not there, so pieces that
// touch the terrain stay approximate.
func (g *previewGround) StructureGround() structure3d.Ground {
	return func(x, y, z int) structure3d.GroundKind {
		g.mu.Lock()
		defer g.mu.Unlock()
		if sub, ok := g.substance(x, y, z); ok {
			switch sub {
			case launcher.TerrainWater, launcher.TerrainLava:
				return structure3d.GroundLiquid
			case launcher.TerrainSolid:
				return g.carvedKind(x, y, z)
			}
			return structure3d.GroundAir
		}
		if y <= g.height(x, z) {
			return g.carvedKind(x, y, z)
		}
		return structure3d.GroundAir
	}
}

// carvedKind is a solid block after the cave and canyon carvers; the
// caller holds g.mu.
func (g *previewGround) carvedKind(x, y, z int) structure3d.GroundKind {
	if g.caves && g.dim == "minecraft:overworld" && y < g.height(x, z)-1 {
		m := g.carves.mask(g.seed, int32(x>>4), int32(z>>4), y)
		if bit := uint16(1) << (x & 15); (m.cave[z&15]|m.canyon[z&15])&bit != 0 {
			if y <= worldgen.CarverLavaY {
				return structure3d.GroundLiquid
			}
			return structure3d.GroundAir
		}
	}
	return structure3d.GroundSolid
}
