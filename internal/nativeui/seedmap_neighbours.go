package nativeui

import (
	"github.com/Viking602/YoloMCL/internal/launcher"
	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// previewNeighbourMax caps how many other structures join one preview slab.
const previewNeighbourMax = 8

// addPreviewNeighbours builds the other structures the seed places inside
// the preview's terrain slab (the same hits the 2D map marks) with the same
// generator and surface heights, and merges them into m, so the slab shows
// the map's villages, temples, outposts and portals where they stand.
func addPreviewNeighbours(store *structure3d.Store, m *structure3d.Mesh, hits []launcher.FeatureHit, subject launcher.FeatureHit, dim string, seed int64, surface structure3d.HeightFunc, sampler *launcher.LocalSampler) int {
	if m == nil {
		return 0
	}
	half := previewSlabSize(m)/2 - 4
	cx, cz := int(m.Center[0]), int(m.Center[2])
	n := 0
	for _, h := range hits {
		if n >= previewNeighbourMax {
			break
		}
		if h.Dimension != "" && h.Dimension != dim {
			continue
		}
		if h.X == subject.X && h.Z == subject.Z && h.ID == subject.ID {
			continue
		}
		if h.X < cx-half || h.X > cx+half || h.Z < cz-half || h.Z > cz+half {
			continue
		}
		target, err := resolveHitTarget(store, h, seed, dim, sampler)
		if err != nil {
			continue
		}
		res, err := launcher.PreviewStructureIn(store, target, dim, seed, h.X, h.Z, surface, 8, 8, structure3d.Camera{Yaw: 0.6, Pitch: 0.55, Zoom: 1})
		if err != nil || res == nil || res.Mesh == nil || res.Unsupported != "" {
			continue
		}
		m.Merge(res.Mesh)
		n++
	}
	return n
}
