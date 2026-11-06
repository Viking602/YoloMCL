package structure3d_test

import (
	"os"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
)

func TestMonumentStatuses(t *testing.T) {
	if os.Getenv("ASM_STAT") == "" {
		t.Skip()
	}
	world := `D:\MC\.minecraft\versions\project01\saves\Codex_Compat_Regression_REI_20260925`
	paths, _ := mcworld.RegionPaths(world, "")
	for _, path := range paths {
		rg, err := mcworld.OpenRegion(path)
		if err != nil {
			continue
		}
		for lx := 0; lx < 32; lx++ {
			for lz := 0; lz < 32; lz++ {
				c, _ := rg.Chunk(lx, lz)
				if c == nil {
					continue
				}
				for _, s := range c.Starts() {
					if s.ID != "minecraft:monument" && s.ID != "minecraft:mansion" {
						continue
					}
					t.Logf("%s start@%d,%d status=%q pieces=%d BB0=%v", s.ID, s.ChunkX, s.ChunkZ, c.Status, len(s.Pieces), s.Pieces[0].BB)
				}
			}
		}
	}
}
