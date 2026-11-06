package launcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// MCL_ERA_TRY="inst|world" places saved starts (MCL_JIGSAW_ONLY id,
// MCL_ERA_STARTS="cx,cz;...") with the terrain of their detected era and
// each candidate store, to find the mod set a structure was generated with:
//   - MCL_ERA_WITHOUT="a,b;c,d": jar exclusion sets (one store each)
//   - MCL_EDIT_POOL=substr: log the code pool edits of matching pools
//   - MCL_EDIT_DROP="src,src|src": also try dropping edits by source
//   - MCL_EDIT_LAST="src|src": also try moving a source's edits last
func TestEraModSetTry(t *testing.T) {
	spec := os.Getenv("MCL_ERA_TRY")
	if spec == "" {
		t.Skip("set MCL_ERA_TRY")
	}
	s := testService(t)
	ctx := context.Background()
	inst, world, _ := strings.Cut(spec, "|")
	root := Root{ID: "et", Path: filepath.Dir(filepath.Dir(inst)), SelectedVersion: filepath.Base(inst)}
	s.cfg.Roots = []Root{root}
	_, level, err := s.SeedMapWorldgen(ctx, root.ID, root.SelectedVersion, world)
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.DetectEras(ctx, root.ID, root.SelectedVersion, world)
	if err != nil {
		t.Fatal(err)
	}
	want := map[[2]int]bool{}
	for _, p := range strings.Split(os.Getenv("MCL_ERA_STARTS"), ";") {
		var x, z int
		if _, err := fmt.Sscanf(p, "%d,%d", &x, &z); err == nil {
			want[[2]int{x, z}] = true
		}
	}
	only := os.Getenv("MCL_JIGSAW_ONLY")
	var starts []mcworld.Start
	paths, _ := mcworld.RegionPaths(filepath.Join(inst, "saves", world), "")
	for _, p := range paths {
		r, err := mcworld.OpenRegion(p)
		if err != nil {
			continue
		}
		for i := 0; i < 1024; i++ {
			c, _ := r.Chunk(i%32, i/32)
			if c == nil {
				continue
			}
			for _, st := range c.Starts() {
				if len(st.Pieces) > 0 && strings.Contains(st.ID, only) && (len(want) == 0 || want[[2]int{st.ChunkX, st.ChunkZ}]) {
					starts = append(starts, st)
				}
			}
		}
	}
	try := func(store *structure3d.Store, label string) {
		line := label
		for _, st := range starts {
			era := m.EraAt(st.ChunkX, st.ChunkZ).Era
			smp, err := s.EraSampler(ctx, root.ID, root.SelectedVersion, world, era)
			if err != nil {
				t.Fatal(err)
			}
			const dim = "minecraft:overworld"
			info, _ := smp.TerrainInfo(dim)
			floor := func(x, z int) int {
				if h, ok := smp.TerrainSurfaceAt(dim, x, z); ok {
					return h + 1
				}
				return 64
			}
			free := func(x, z int) int { return max(floor(x, z), info.SeaLevel) }
			got, err := structure3d.PlaceJigsawFloor(store, st.ID, level.RandomSeed, st.ChunkX, st.ChunkZ, free, floor, structure3d.OverworldHeight)
			if err != nil {
				line += fmt.Sprintf(" @%d,%d err %v", st.ChunkX, st.ChunkZ, err)
				continue
			}
			line += fmt.Sprintf(" @%d,%d %d/%d", st.ChunkX, st.ChunkZ, savedPiecesMatched(st, got), len(st.Pieces))
		}
		t.Log(line)
	}
	bySource := func(e structure3d.PoolEdit, set string) bool {
		for _, d := range strings.Split(set, ",") {
			if d != "" && strings.Contains(strings.ToLower(e.Source), strings.ToLower(d)) {
				return true
			}
		}
		return false
	}
	for _, set := range strings.Split(os.Getenv("MCL_ERA_WITHOUT"), ";") {
		var without []string
		for _, w := range strings.Split(set, ",") {
			if w = strings.TrimSpace(strings.ToLower(w)); w != "" {
				without = append(without, w)
			}
		}
		store, err := s.StructureAssetStoreEra(root.ID, root.SelectedVersion, world, without)
		if err != nil {
			t.Fatal(err)
		}
		all := store.PoolEdits()
		if pool := os.Getenv("MCL_EDIT_POOL"); pool != "" {
			for i, e := range all {
				if strings.Contains(e.Pool, pool) {
					t.Logf("  edit#%d %s <- %s w%d %s", i, e.Pool, e.Source, e.Weight, string(e.Element)[:min(100, len(e.Element))])
				}
			}
		}
		try(store, fmt.Sprintf("without %.40q:", set))
		if v := os.Getenv("MCL_EDIT_DROP"); v != "" {
			for _, ds := range strings.Split(v, "|") {
				var keep []structure3d.PoolEdit
				for _, e := range all {
					if !bySource(e, ds) {
						keep = append(keep, e)
					}
				}
				store.SetPoolEdits(keep)
				try(store, "  drop "+ds+":")
			}
		}
		if v := os.Getenv("MCL_EDIT_LAST"); v != "" {
			for _, ds := range strings.Split(v, "|") {
				var head, tail []structure3d.PoolEdit
				for _, e := range all {
					if bySource(e, ds) {
						tail = append(tail, e)
					} else {
						head = append(head, e)
					}
				}
				store.SetPoolEdits(append(head, tail...))
				try(store, "  last "+ds+":")
				store.SetPoolEdits(append(tail, head...))
				try(store, "  first "+ds+":")
			}
		}
		store.SetPoolEdits(all)
		store.Close()
	}
}
