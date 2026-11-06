package nativeui

import (
	"github.com/Viking602/YoloMCL/internal/launcher"
	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// resolveHitTarget turns a map hit (a structure-set id) into the structure the
// seed generates there. Sets with several weighted structures (vanilla
// nether_complexes: fortress 2, bastion 3) are resolved per start chunk like
// ChunkGenerator.tryGenerateStructure; anything else falls back to
// ResolvePreviewTarget.
func resolveHitTarget(store *structure3d.Store, hit launcher.FeatureHit, seed int64, dim string, sampler *launcher.LocalSampler) (string, error) {
	var biomeAt func(x, y, z int) string
	if sampler != nil {
		biomeAt = func(x, y, z int) string {
			b, _ := sampler.BiomeAt(dim, x, y, z)
			return b
		}
	}
	if id, ok := launcher.PickSetStructure(store, hit.ID, seed, hit.X, hit.Z, biomeAt); ok {
		return id, nil
	}
	return launcher.ResolvePreviewTarget(store, hit.ID, hit.Biome)
}
