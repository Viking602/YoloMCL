package launcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	"github.com/Viking602/YoloMCL/internal/worldgen"
)

func TestZZP133DiagPlacement(t *testing.T) {
	id := os.Getenv("MCL_P133_DIAG")
	if id == "" {
		t.Skip()
	}
	world := `D:\MC\.minecraft\versions\project01\saves\Codex_Compat_Regression_REI_20260925`
	instance := filepath.Dir(filepath.Dir(world))
	root := Root{ID: "d", Path: filepath.Dir(filepath.Dir(instance)), SelectedVersion: filepath.Base(instance)}
	s := testService(t)
	s.cfg.Roots = []Root{root}
	snapshot, level, err := s.SeedMapWorldgen(context.Background(), root.ID, "", filepath.Base(world))
	if err != nil {
		t.Fatal(err)
	}
	sampler, err := NewLocalSampler(context.Background(), snapshot, level.RandomSeed)
	if err != nil {
		t.Fatal(err)
	}
	var p *seedMapPlacement
	for i := range sampler.placements {
		for _, st := range sampler.placements[i].structures {
			if st == id {
				p = &sampler.placements[i]
			}
		}
	}
	if p == nil {
		t.Fatal("no placement for " + id)
	}
	t.Logf("placement id=%s spacing=%d sep=%d salt=%d freq=%v method=%q", p.id, p.spacing, p.separation, p.salt, p.frequency, p.frequencyMethod)
	seed := level.RandomSeed
	paths, _ := mcworld.RegionPaths(world, "")
	passN, failN := 0, 0
	for _, path := range paths {
		r, err := mcworld.OpenRegion(path)
		if err != nil {
			continue
		}
		for i := 0; i < 1024; i++ {
			c, _ := r.Chunk(i%32, i/32)
			if c == nil {
				continue
			}
			for _, st := range c.Starts() {
				if st.ID != id || len(st.Pieces) == 0 {
					continue
				}
				ch := [2]int{st.ChunkX, st.ChunkZ}
				rx, rz := floorDivInt(ch[0], p.spacing), floorDivInt(ch[1], p.spacing)
				cx, cz := p.candidate(seed, rx, rz)
				ok := cx>>4 == ch[0] && cz>>4 == ch[1]
				pass := p.placementChunk(seed, cx, cz)
				freq, _ := worldgen.FrequencyPasses(p.frequencyMethod, seed, int32(p.salt), int32(cx>>4), int32(cz>>4), p.frequency)
				if pass {
					passN++
				} else {
					failN++
					if failN <= 8 {
						t.Logf("MISS store@%d,%d start=%v cand=%d,%d ok=%v freq=%v excl=%v bb0=%v", c.X, c.Z, ch, cx>>4, cz>>4, ok, freq, len(p.excludes), st.Pieces[0].BB)
					}
				}
			}
		}
	}
	t.Logf("total pass=%d fail=%d", passN, failN)
}
