package structure3d_test

import (
	"os"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/structure3d"
)

func TestAssembleFullOnly(t *testing.T) {
	if os.Getenv("ASM_FULL") == "" {
		t.Skip()
	}
	jar := `D:\MC\.minecraft\versions\1.21.1\1.21.1.jar`
	store := &Store{}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.OpenJar("game", jar); err != nil {
		t.Fatal(err)
	}
	dy := -30
	if v := os.Getenv("ASM_MONUMENT_DY"); v != "" {
		n, sign := 0, 1
		for i, c := range v {
			if i == 0 && c == '-' {
				sign = -1
				continue
			}
			n = n*10 + int(c-'0')
		}
		dy = sign * n
	}
	store.SetMonumentYOffset(dy)
	world := os.Getenv("ASM_WORLD")
	if world == "" {
		world = `D:\MC\.minecraft\versions\project01\saves\Codex_Compat_Regression_REI_20260925`
	}
	dir := world + `\region`
	seed, err := mcworld.LevelSeed(world)
	if err != nil {
		t.Fatal(err)
	}
	cc := &regionCache{dir: dir, regions: map[[2]int]*mcworld.Region{}, chunks: map[[2]int]*mcworld.Chunk{}}
	want := map[string]bool{"minecraft:monument": true, "minecraft:mansion": true}
	type tally struct{ solid, solidOK, starts, skipped int }
	per := map[string]*tally{}
	miss := map[string]int{}
	statusCount := map[string]int{}
	paths, _ := mcworld.RegionPaths(world, "")
	t.Logf("world=%s regions=%d seed=%d dy=%d", world, len(paths), seed, dy)
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
				statusCount[c.Status]++
				full := c.Status == "minecraft:full" || c.Status == "full"
				for _, s := range c.Starts() {
					if !want[s.ID] || len(s.Pieces) == 0 {
						continue
					}
					tl := per[s.ID]
					if tl == nil {
						tl = &tally{}
						per[s.ID] = tl
					}
					if !full {
						tl.skipped++
						continue
					}
					ox, oz := s.ChunkX*16, s.ChunkZ*16
					y0 := int(s.Pieces[0].BB[1])
					for _, pc := range s.Pieces {
						if tp, ok := pc.Raw["TPY"]; ok {
							switch v := tp.(type) {
							case int32:
								y0 = int(v)
							case int64:
								y0 = int(v)
							}
							break
						}
					}
					height := func(x, z int) int { return y0 }
					sc, aerr := Assemble(store, s.ID, seed, ox+8, oz+8, height)
					if aerr != nil || sc == nil {
						t.Logf("%s @%d,%d: %v", s.ID, s.ChunkX, s.ChunkZ, aerr)
						continue
					}
					tl.starts++
					for _, b := range sc.Blocks {
						if b.Y < -64 || b.Y > 320 {
							continue
						}
						name := b.State.Name
						got, ok := cc.block(b.X, b.Y, b.Z)
						if !ok || isAirName(name) {
							continue
						}
						tl.solid++
						if got == name {
							tl.solidOK++
						} else {
							miss[s.ID+": "+name+" -> "+got]++
						}
					}
				}
			}
		}
	}
	t.Logf("chunk statuses: %v", statusCount)
	for id, tl := range per {
		pct := 0.0
		if tl.solid > 0 {
			pct = 100 * float64(tl.solidOK) / float64(tl.solid)
		}
		t.Logf("%s: full_starts=%d skipped_nonfull=%d placed %d/%d (%.2f%%)", id, tl.starts, tl.skipped, tl.solidOK, tl.solid, pct)
	}
	type kv struct {
		k string
		n int
	}
	var top []kv
	for k, n := range miss {
		top = append(top, kv{k, n})
	}
	for i := 0; i < len(top); i++ {
		for j := i + 1; j < len(top); j++ {
			if top[j].n > top[i].n {
				top[i], top[j] = top[j], top[i]
			}
		}
	}
	for i, e := range top {
		if i >= 12 {
			break
		}
		t.Logf("  mismatch %s: %d", e.k, e.n)
	}
}
