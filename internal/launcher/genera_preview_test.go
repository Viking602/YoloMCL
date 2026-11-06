package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// MCL_PREVIEW_SEA="inst|world" (MCL_JIGSAW_ONLY filters ids) runs saved
// jigsaw starts through the live preview path: era store and sampler from
// DetectEras, structure3d.AssembleGroundSea with the solid noise top and the
// sea level, compared piece box by piece box with the save.
func TestPreviewSeaProduction(t *testing.T) {
	spec := os.Getenv("MCL_PREVIEW_SEA")
	if spec == "" {
		t.Skip("set MCL_PREVIEW_SEA")
	}
	only := os.Getenv("MCL_JIGSAW_ONLY")
	s := testService(t)
	ctx := context.Background()
	const dim = "minecraft:overworld"
	for _, item := range strings.Split(spec, ";") {
		inst, world, _ := strings.Cut(item, "|")
		root := Root{ID: "ps", Path: filepath.Dir(filepath.Dir(inst)), SelectedVersion: filepath.Base(inst)}
		s.cfg.Roots = []Root{root}
		_, level, err := s.SeedMapWorldgen(ctx, root.ID, root.SelectedVersion, world)
		if err != nil {
			t.Fatal(err)
		}
		m, err := s.DetectEras(ctx, root.ID, root.SelectedVersion, world)
		if err != nil {
			t.Fatal(err)
		}
		stores := map[int]*structure3d.Store{}
		paths, _ := mcworld.RegionPaths(filepath.Join(inst, "saves", world), "")
		type tally struct{ n, exact, approx int }
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
					if len(st.Pieces) == 0 || only == "" || !strings.Contains(st.ID, only) {
						continue
					}
					ev := m.EraAt(st.ChunkX, st.ChunkZ)
					// The store the live preview uses: era plus the mod set
					// that reproduces this start.
					store, exactSet, err := s.StartStore(ctx, root.ID, root.SelectedVersion, world, st.ID, st.ChunkX, st.ChunkZ)
					if err != nil {
						t.Fatal(err)
					}
					// One store per start: close the previous one.
					if open := stores[0]; open != nil {
						open.Close()
					}
					stores[0] = store
					smp, err := s.EraSampler(ctx, root.ID, root.SelectedVersion, world, ev.Era)
					if err != nil {
						t.Fatal(err)
					}
					info, _ := smp.TerrainInfo(dim)
					height := func(x, z int) int {
						if h, ok := smp.TerrainSurfaceAt(dim, x, z); ok {
							return h
						}
						return 63
					}
					scene, err := structure3d.AssembleGroundSea(store, st.ID, level.RandomSeed, st.ChunkX*16, st.ChunkZ*16, height, structure3d.OverworldHeight, nil, info.SeaLevel)
					if err != nil {
						t.Logf("  %s @%d,%d: %v", st.ID, st.ChunkX, st.ChunkZ, err)
						continue
					}
					// Compare the layout the scene was built from: template,
					// rotation and either box (saved BB may be the raw one).
					floor := func(x, z int) int { return height(x, z) + 1 }
					free := func(x, z int) int { return max(floor(x, z), info.SeaLevel) }
					lay, lerr := structure3d.PlaceJigsawFloor(store, st.ID, level.RandomSeed, st.ChunkX, st.ChunkZ, free, floor, structure3d.OverworldHeight)
					if lerr != nil {
						continue // code-built structure: no jigsaw layout to compare
					}
					ok := savedPiecesMatched(st, lay)
					_ = scene
					tl := stats[st.ID]
					if tl == nil {
						tl = &tally{}
						stats[st.ID] = tl
					}
					tl.n++
					if ev.Approx {
						tl.approx++
					}
					if ok == len(st.Pieces) && len(lay.Pieces) == len(st.Pieces) {
						tl.exact++
					} else {
						t.Logf("  %s @%d,%d era %+v set-exact=%v pieces %d/%d (got %d)", st.ID, st.ChunkX, st.ChunkZ, ev, exactSet, ok, len(st.Pieces), len(lay.Pieces))
					}
				}
			}
		}
		for id, tl := range stats {
			t.Logf("%s %s: starts %d exact %d (≈ era %d)", world, id, tl.n, tl.exact, tl.approx)
		}
	}
}
