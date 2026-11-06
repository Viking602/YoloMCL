package structure3d_test

import (
	"os"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/structure3d"
)

func TestAssembleDiag(t *testing.T) {
	if os.Getenv("ASM_DIAG") == "" {
		t.Skip()
	}
	jar := `D:\MC\.minecraft\versions\1.21.1\1.21.1.jar`
	store := &Store{}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.OpenJar("game", jar); err != nil {
		t.Fatal(err)
	}
	store.SetMonumentYOffset(-30)
	world := `D:\MC\.minecraft\versions\project01\saves\Codex_Compat_Regression_REI_20260925`
	seed, err := mcworld.LevelSeed(world)
	if err != nil {
		t.Fatal(err)
	}
	r, err := mcworld.OpenRegion(world + `\region\r.0.0.mca`)
	if err != nil {
		// find any region with monument
		t.Log(err)
	}
	_ = r
	// Hardcode a known monument from count - scan first few
	paths, _ := mcworld.RegionPaths(world, "")
	found := 0
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
					if s.ID != "minecraft:monument" || len(s.Pieces) == 0 {
						continue
					}
					ox, oz := s.ChunkX*16, s.ChunkZ*16
					y0 := int(s.Pieces[0].BB[1])
					height := func(x, z int) int { return y0 }
					sc, err := Assemble(store, s.ID, seed, ox+8, oz+8, height)
					n := 0
					if sc != nil {
						n = len(sc.Blocks)
					}
					t.Logf("monument @%d,%d y0=%d assemble_err=%v blocks=%d", s.ChunkX, s.ChunkZ, y0, err, n)
					found++
					if found >= 3 {
						return
					}
				}
			}
		}
	}
	t.Logf("found=%d", found)
}
