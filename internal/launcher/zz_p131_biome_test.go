package launcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
)

// TestZZP131BiomeMatch compares overworld biomes against fully generated
// saved chunks (scratch diagnostic, p131).
func TestZZP131BiomeMatch(t *testing.T) {
	list := os.Getenv("MCL_LOCATION_SAVES")
	if list == "" {
		t.Skip("MCL_LOCATION_SAVES")
	}
	for _, world := range strings.Split(list, ";") {
		instance := filepath.Dir(filepath.Dir(world))
		root := Root{ID: "bm", Path: instance}
		if filepath.Base(filepath.Dir(instance)) == "versions" {
			root.Path, root.SelectedVersion = filepath.Dir(filepath.Dir(instance)), filepath.Base(instance)
		}
		s := testService(t)
		s.cfg.Roots = []Root{root}
		snapshot, level, err := s.SeedMapWorldgen(context.Background(), root.ID, "", filepath.Base(world))
		if err != nil {
			t.Logf("%s: %v", world, err)
			continue
		}
		sampler, err := NewLocalSampler(context.Background(), snapshot, level.RandomSeed)
		if err != nil {
			t.Logf("%s: %v", world, err)
			continue
		}
		if ord, err := forgeOrderRegionPrograms(snapshot); err == nil {
			for _, p := range ord {
				if p.Result == nil {
					continue
				}
				for _, r := range p.Result.Regions {
					t.Logf("  region mod=%s id=%s dim=%v w=%d n=%d deferred=%v", p.ModID, r.ID, r.Dimension, r.Weight, len(r.Parameters), r.Deferred)
				}
			}
		}
		for _, p := range snapshot.RegionPrograms {
			t.Logf("  program %s err=%q nil=%v", p.ModID, p.Error, p.Result == nil)
		}
		for _, m := range snapshot.Mods {
			if m.TerraBlender {
				t.Logf("  tbmod %s %s %s", m.ID, m.Version, m.Loader)
			}
		}
		d := sampler.dimensions["minecraft:overworld"]
		scratch := d.scratch()
		paths, _ := mcworld.RegionPaths(world, "")
		total, match, unknown := 0, 0, 0
		conf := map[string]int{}
		savedN := map[string]int{}
		oursN := map[string]int{}
		regConf := map[string]int{}
		var weights []int
		for _, path := range paths {
			r, err := mcworld.OpenRegion(path)
			if err != nil {
				continue
			}
			for i := 0; i < 1024; i++ {
				c, _ := r.Chunk(i%32, i/32)
				if c == nil || !strings.HasSuffix(c.Status, "full") {
					continue
				}
				for _, o := range [][2]int{{2, 2}, {10, 10}} {
					x, z := int(c.X)*16+o[0], int(c.Z)*16+o[1]
					y := 64
					if h, ok := c.Heightmap("WORLD_SURFACE", x&15, z&15, -64); ok {
						y = h
					}
					sb := c.Biome(x, y, z)
					if sb == "" {
						continue
					}
					b, ok := d.biomeAt(x, y, z, scratch, false)
					if !ok {
						unknown++
						continue
					}
					total++
					savedN[sb]++
					oursN[b]++
					if b == sb {
						match++
					} else {
						conf[b+" -> saved "+sb]++
						if reg, all, w, ok := scratch.TerraBlenderRegionBiomes(x, y, z); ok {
							var hits []int
							for i, n := range all {
								if n == sb {
									hits = append(hits, i)
								}
							}
							regConf[fmt.Sprintf("r%d -> %v", reg, hits)]++
							weights = w
						}
					}
				}
			}
		}
		pct := 0.0
		if total > 0 {
			pct = 100 * float64(match) / float64(total)
		}
		t.Logf("== %s: biome match %d/%d = %.1f%% (unknown %d)", world, match, total, pct, unknown)
		dump := func(name string, m map[string]int, n int) {
			type kv struct {
				k string
				v int
			}
			var xs []kv
			for k, v := range m {
				xs = append(xs, kv{k, v})
			}
			sort.Slice(xs, func(i, j int) bool { return xs[i].v > xs[j].v })
			for i, e := range xs {
				if i >= n {
					break
				}
				t.Logf("  %s %5d %s", name, e.v, e.k)
			}
		}
		dump("CONF", conf, 40)
		t.Logf("  weights %v", weights)
		dump("REG", regConf, 30)
		dump("SAVED", savedN, 40)
		dump("OURS", oursN, 40)
		_ = fmt.Sprint
	}
}
