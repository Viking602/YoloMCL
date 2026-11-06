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
	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// TestJigsawProductionTerrain rebuilds saved jigsaw starts with the
// production asset store (Lithostitched detection, known mod pool edits)
// and the seed map's noise-stage surface, as the structure preview does.
// Opt-in, read-only: MCL_JIGSAW_TERRAIN="instanceDir|world;..." and
// optional MCL_JIGSAW_ONLY substring.
func TestJigsawProductionTerrain(t *testing.T) {
	spec := os.Getenv("MCL_JIGSAW_TERRAIN")
	if spec == "" {
		t.Skip("set MCL_JIGSAW_TERRAIN")
	}
	only := os.Getenv("MCL_JIGSAW_ONLY")
	s := testService(t)
	tectonicPackDisabled = os.Getenv("MCL_TERRAIN_NOTECTONIC") != ""
	defer func() { tectonicPackDisabled = false }()
	for _, item := range strings.Split(spec, ";") {
		inst, world, _ := strings.Cut(item, "|")
		root := Root{ID: "jt", Path: filepath.Dir(filepath.Dir(inst)), SelectedVersion: filepath.Base(inst)}
		s.cfg.Roots = []Root{root}
		snap, level, err := s.SeedMapWorldgen(context.Background(), root.ID, root.SelectedVersion, world)
		if err != nil {
			t.Fatalf("%s: %v", world, err)
		}
		sampler, err := NewLocalSampler(context.Background(), snap, level.RandomSeed)
		if err != nil {
			t.Fatal(err)
		}
		store, err := s.StructureAssetStoreWorld(root.ID, root.SelectedVersion, world)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: litho=%v, %d pool edits", world, store.LithostitchedJigsaw(), len(store.PoolEdits()))
		const dim = "minecraft:overworld"
		// MCL_JIGSAW_ERA=1: each start uses the store and terrain of the
		// generation era DetectEras assigns to its chunk.
		var eraMap *EraMap
		eraStores := []*structure3d.Store{store}
		eraSamplers := []*LocalSampler{sampler}
		if os.Getenv("MCL_JIGSAW_ERA") == "1" {
			if eraMap, err = s.DetectEras(context.Background(), root.ID, root.SelectedVersion, world); err != nil {
				t.Fatal(err)
			}
			for i, e := range eraMap.Eras[1:] {
				st, err := s.StructureAssetStoreEra(root.ID, root.SelectedVersion, world, e.Without)
				if err != nil {
					t.Fatal(err)
				}
				smp, err := s.EraSampler(context.Background(), root.ID, root.SelectedVersion, world, i+1)
				if err != nil {
					t.Fatal(err)
				}
				eraStores, eraSamplers = append(eraStores, st), append(eraSamplers, smp)
			}
		}
		surface := func(x, z int) int {
			// getFirstFreeHeight(WORLD_SURFACE_WG) is one above the top solid
			// block; the noise-stage sea (non-air) caps it at sea level.
			if h, ok := sampler.TerrainSurfaceAt(dim, x, z); ok {
				if info, _ := sampler.TerrainInfo(dim); h+1 < info.SeaLevel {
					return info.SeaLevel
				}
				return h + 1
			}
			return 64
		}
		floor := func(x, z int) int {
			if h, ok := sampler.TerrainSurfaceAt(dim, x, z); ok {
				return h + 1
			}
			return 64
		}
		wdir := filepath.Join(inst, "saves", world)
		paths, _ := mcworld.RegionPaths(wdir, "")
		type tally struct{ n, exact, pieces, ok int }
		stats := map[string]*tally{}
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
					if len(st.Pieces) == 0 || (only != "" && !strings.Contains(st.ID, only)) {
						continue
					}
					useStore := store
					if eraMap != nil {
						v := eraMap.EraAt(st.ChunkX, st.ChunkZ)
						useStore, sampler = eraStores[v.Era], eraSamplers[v.Era]
					}
					got, err := structure3d.PlaceJigsawFloor(useStore, st.ID, level.RandomSeed, st.ChunkX, st.ChunkZ, surface, floor, structure3d.OverworldHeight)
					if err != nil {
						continue
					}
					want := map[string]int{}
					for _, p := range st.Pieces {
						want[fmt.Sprint(p.Template, p.Rotation, p.BB)]++
					}
					ok := 0
					for _, p := range got.Pieces {
						loc := p.Location
						if p.Kind != "single" {
							loc = ""
						}
						for _, bb := range [][6]int{p.BB, p.RawBB} {
							k := fmt.Sprint(loc, structure3d.RotationName(p.Rotation), [6]int32{int32(bb[0]), int32(bb[1]), int32(bb[2]), int32(bb[3]), int32(bb[4]), int32(bb[5])})
							if want[k] > 0 {
								want[k]--
								ok++
								break
							}
						}
					}
					tl := stats[st.ID]
					if tl == nil {
						tl = &tally{}
						stats[st.ID] = tl
					}
					if ok == 0 && len(got.Pieces) > 0 && tl.n < 3 {
						t.Logf("  %s @%d,%d piece0 saved %s %v got %s %v", st.ID, st.ChunkX, st.ChunkZ, st.Pieces[0].Template, st.Pieces[0].BB, got.Pieces[0].Location, got.Pieces[0].BB)
					}
					if eraMap != nil && (ok != len(st.Pieces) || len(got.Pieces) != len(st.Pieces)) {
						t.Logf("  ERA %s @%d,%d %+v pieces %d/%d", st.ID, st.ChunkX, st.ChunkZ, eraMap.EraAt(st.ChunkX, st.ChunkZ), ok, len(st.Pieces))
					}
					tl.n++
					tl.pieces += len(st.Pieces)
					tl.ok += ok
					if ok == len(st.Pieces) && len(got.Pieces) == len(st.Pieces) {
						tl.exact++
					} else if os.Getenv("WORLDGEN_JIGSAW_SEQ") == "1" && tl.n-tl.exact <= 6 {
						// First divergence in placement order (saved pieces keep
						// the generator's order), with both surfaces at that piece.
						n := min(len(st.Pieces), len(got.Pieces))
						for k := 0; k < n; k++ {
							a, b := st.Pieces[k], got.Pieces[k]
							bb := [6]int32{int32(b.BB[0]), int32(b.BB[1]), int32(b.BB[2]), int32(b.BB[3]), int32(b.BB[4]), int32(b.BB[5])}
							raw := [6]int32{int32(b.RawBB[0]), int32(b.RawBB[1]), int32(b.RawBB[2]), int32(b.RawBB[3]), int32(b.RawBB[4]), int32(b.RawBB[5])}
							if a.Template == b.Location && a.Rotation == structure3d.RotationName(b.Rotation) && (a.BB == bb || a.BB == raw) {
								continue
							}
							cx, cz := (int(a.BB[0])+int(a.BB[3]))/2, (int(a.BB[2])+int(a.BB[5]))/2
							t.Logf("  SEQ %s @%d,%d saved %d got %d, first diff #%d (surface@%d,%d=%d)", st.ID, st.ChunkX, st.ChunkZ, len(st.Pieces), len(got.Pieces), k, cx, cz, surface(cx, cz))
							for j := max(0, k-1); j < min(n, k+2); j++ {
								t.Logf("    %d saved %s %s %v | got %s %s %v", j, st.Pieces[j].Template, st.Pieces[j].Rotation, st.Pieces[j].BB, got.Pieces[j].Location, structure3d.RotationName(got.Pieces[j].Rotation), got.Pieces[j].BB)
							}
							break
						}
					}
				}
			}
		}
		ids := make([]string, 0, len(stats))
		for id := range stats {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			tl := stats[id]
			t.Logf("%s %s: starts %d exact %d pieces %d/%d", world, id, tl.n, tl.exact, tl.ok, tl.pieces)
		}
	}
}
