package launcher

import (
	"context"
	"os"
	"testing"
)

// MCL_CAVE_PROBE=1: noise-stage substances around the saved wrought chamber
// @-325,-312 (BB x -5209..-5191, y 31..37, z -5009..-4991).
func TestCaveProbe(t *testing.T) {
	if os.Getenv("MCL_CAVE_PROBE") == "" {
		t.Skip("set MCL_CAVE_PROBE")
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
	for e := range m.Eras {
		smp, _ := s.EraSampler(ctx, root.ID, root.SelectedVersion, world, e)
		info, _ := smp.TerrainInfo("minecraft:overworld")
		out := make([]TerrainSubstance, info.Height)
		air := 0
		var ex [][3]int
		for x := -5240; x < -5160; x += 2 {
			for z := -5040; z < -4960; z += 2 {
				smp.TerrainColumn("minecraft:overworld", x, z, out)
				for y := 20; y <= 50; y++ {
					if out[y-info.MinY] != TerrainSolid {
						air++
						if len(ex) < 5 {
							ex = append(ex, [3]int{x, y, z})
						}
					}
				}
			}
		}
		d, _ := smp.TerrainDensity("minecraft:overworld", -5200, 33, -5000)
		for z := -4992; z <= -4983; z++ {
			smp.TerrainColumn("minecraft:overworld", -5200, z, out)
			r := ""
			for y := 25; y <= 45; y++ {
				r += string(".#~L"[out[y-info.MinY]])
			}
			d, _ := smp.TerrainDensity("minecraft:overworld", -5200, 31, z)
			t.Logf("era%d x-5200 z%d y25..45 %s dens@31 %.4f", e, z, r, d)
		}
		t.Logf("era%d %s: non-solid y20..50 over 40x40 grid: %d %v; density@room %.4f", e, m.Eras[e].Name, air, ex, d)
	}
}
