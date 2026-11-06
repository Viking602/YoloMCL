package structure3d_test

import (
	"os"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/structure3d"
)

func TestAssembleDiagCache(t *testing.T) {
	if os.Getenv("ASM_DIAG2") == "" {
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
	dir := world + `\region`
	seed, _ := mcworld.LevelSeed(world)
	cc := &regionCache{dir: dir, regions: map[[2]int]*mcworld.Region{}, chunks: map[[2]int]*mcworld.Chunk{}}
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
					if err != nil || sc == nil {
						t.Logf("assemble fail %v", err)
						continue
					}
					okN, missN, sample := 0, 0, ""
					for i, b := range sc.Blocks {
						if i > 5000 && okN+missN > 200 {
							break
						}
						got, ok := cc.block(b.X, b.Y, b.Z)
						if ok {
							okN++
							if sample == "" {
								sample = got
							}
						} else {
							missN++
						}
					}
					// Also probe chunk presence at origin
					ch := cc.chunk(s.ChunkX, s.ChunkZ)
					t.Logf("monument @%d,%d y0=%d blocks=%d cache_ok=%d miss=%d sample=%q chunk_nil=%v",
						s.ChunkX, s.ChunkZ, y0, len(sc.Blocks), okN, missN, sample, ch == nil)
					found++
					if found >= 2 {
						return
					}
				}
			}
		}
	}
}
