package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
)

// TestStrongholdRingsAgainstSaves compares concentric-ring stronghold starts
// computed from the seed and the instance's biome source with saved starts.
func TestStrongholdRingsAgainstSaves(t *testing.T) {
	list := os.Getenv("MCL_STRONGHOLD_SAVES")
	if list == "" {
		t.Skip("MCL_STRONGHOLD_SAVES lists saves/world directories")
	}
	total, hit := 0, 0
	for _, world := range strings.Split(list, ";") {
		instance := filepath.Dir(filepath.Dir(world))
		root := Root{ID: "rings-oracle", Path: instance}
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
		for _, doc := range snapshot.Documents {
			if doc.Kind == "structure_set" && doc.ID == "minecraft:strongholds" {
				t.Logf("%s: placement %s from %s", world, strings.Join(strings.Fields(string(doc.Data)), ""), doc.Source)
			}
		}
		for _, is := range sampler.coverage.Issues {
			if is.Code == "placement_biomes_unknown" || is.Code == "placement_unsupported" {
				t.Logf("issue %+v", is)
			}
		}
		var chunks [][2]int32
		for _, p := range sampler.placements {
			if p.rings != nil && p.inDimension("minecraft:overworld") {
				chunks, err = sampler.ringChunks(context.Background(), p, "minecraft:overworld")
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("%s: %s preferred %d biomes, %d rings, mode %q", world, p.id, len(p.rings.preferred), len(chunks), sampler.dimensions["minecraft:overworld"].mode)
			}
		}
		set := map[[2]int32]bool{}
		for _, c := range chunks {
			set[c] = true
		}
		paths, _ := mcworld.RegionPaths(world, "")
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
					if st.ID != "minecraft:stronghold" {
						continue
					}
					total++
					k := [2]int32{int32(st.ChunkX), int32(st.ChunkZ)}
					if set[k] {
						hit++
					} else {
						best, bd := [2]int32{}, int64(1<<62)
						for _, c := range chunks {
							dx, dz := int64(c[0]-k[0]), int64(c[1]-k[1])
							if dx*dx+dz*dz < bd {
								best, bd = c, dx*dx+dz*dz
							}
						}
						t.Logf("%s: saved stronghold chunk %v not predicted; nearest ring %v", world, k, best)
					}
				}
			}
		}
	}
	t.Logf("strongholds on map: %d/%d saved starts predicted", hit, total)
	if hit != total {
		t.Fail()
	}
}
