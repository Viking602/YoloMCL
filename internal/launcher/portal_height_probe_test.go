package launcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// MCL_PORTAL_H=1: which centre surface height reproduces the saved Y of the
// underground portal @211,-587, versus our samplers and the saved heightmaps.
func TestPortalHeightProbe(t *testing.T) {
	if os.Getenv("MCL_PORTAL_H") == "" {
		t.Skip("set MCL_PORTAL_H")
	}
	world := "Codex_Compat_Regression_REI_20260925"
	s := testService(t)
	root := Root{ID: "se", Path: `D:\MC\.minecraft`, SelectedVersion: "project01"}
	s.cfg.Roots = []Root{root}
	ctx := context.Background()
	m, err := s.DetectEras(ctx, root.ID, root.SelectedVersion, world)
	if err != nil {
		t.Fatal(err)
	}
	seed, _ := mcworld.LevelSeed(m.worldDir)
	cx, cz := 211, -587
	for e := range m.Eras {
		store, err := s.StructureAssetStoreEra(root.ID, root.SelectedVersion, world, m.Eras[e].Without)
		if err != nil {
			t.Fatal(err)
		}
		var hits []int
		for h := -60; h < 320; h++ {
			hh := h
			sc, err := structure3d.AssembleGroundSea(store, "minecraft:ruined_portal", seed, cx*16, cz*16, func(x, z int) int { return hh }, structure3d.OverworldHeight, nil, -1)
			if err == nil && len(sc.Pieces) > 0 && sc.Pieces[0].Min[1] == 28 {
				hits = append(hits, h)
			}
		}
		store.Close()
		smp, _ := s.EraSampler(ctx, root.ID, root.SelectedVersion, world, e)
		ours := map[string]int{}
		for _, p := range [][2]int{{3393, -9387}, {3394, -9386}, {3393, -9386}, {3394, -9387}} {
			h, _ := smp.TerrainSurfaceAt("minecraft:overworld", p[0], p[1])
			ours[fmt.Sprint(p)] = h
		}
		t.Logf("era%d (%s): surface heights giving Y=28: %v; our sampler at centre: %v", e, m.Eras[e].Name, hits, ours)
	}
	r, _ := mcworld.OpenRegion(filepath.Join(m.worldDir, "region", "r.6.-19.mca"))
	c, _ := r.Chunk(212&31, -587&31)
	for _, k := range []string{"WORLD_SURFACE", "WORLD_SURFACE_WG", "OCEAN_FLOOR", "MOTION_BLOCKING"} {
		h, ok := c.Heightmap(k, 3393&15, -9387&15, -64)
		t.Logf("saved %s at centre: %d %v", k, h, ok)
	}
}
