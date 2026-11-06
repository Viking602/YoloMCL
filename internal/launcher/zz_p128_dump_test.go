package launcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestZZP128DumpRegions(t *testing.T) {
	world := os.Getenv("MCL_P128_WORLD")
	if world == "" {
		t.Skip()
	}
	instance := filepath.Dir(filepath.Dir(world))
	root := Root{ID: "p128", Path: filepath.Dir(filepath.Dir(instance)), SelectedVersion: filepath.Base(instance)}
	s := testService(t)
	s.cfg.Roots = []Root{root}
	snap, _, err := s.SeedMapWorldgen(context.Background(), root.ID, "", filepath.Base(world))
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, d := range snap.Documents {
		if d.Kind == "biome" {
			known[d.ID] = string(d.Data) != "null"
		}
	}
	for _, p := range snap.RegionPrograms {
		if p.Error != "" {
			t.Logf("%s ERR %s", p.ModID, p.Error)
			continue
		}
		if p.Result == nil {
			continue
		}
		for _, r := range p.Result.Regions {
			miss := map[string]int{}
			for _, q := range r.Parameters {
				if !known[q.Biome] {
					miss[q.Biome]++
				}
			}
			t.Logf("%s region %s dim=%v w=%d params=%d unknown=%v", p.ModID, r.ID, r.Dimension, r.Weight, len(r.Parameters), miss)
		}
	}
}
