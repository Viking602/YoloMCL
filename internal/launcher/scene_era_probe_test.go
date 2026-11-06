package launcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// MCL_SCENE_ERA="inst|world|cx,cz;cx,cz" assembles each saved start with
// every era's store and terrain and logs how many saved piece boxes the
// scene reproduces. Read-only.
func TestSceneEraProbe(t *testing.T) {
	spec := os.Getenv("MCL_SCENE_ERA")
	if spec == "" {
		t.Skip("set MCL_SCENE_ERA")
	}
	parts := strings.Split(spec, "|")
	inst, world := parts[0], parts[1]
	s := testService(t)
	ctx := context.Background()
	root := Root{ID: "se", Path: filepath.Dir(filepath.Dir(inst)), SelectedVersion: filepath.Base(inst)}
	s.cfg.Roots = []Root{root}
	m, err := s.DetectEras(ctx, root.ID, root.SelectedVersion, world)
	if err != nil {
		t.Fatal(err)
	}
	_, level, err := s.SeedMapWorldgen(ctx, root.ID, root.SelectedVersion, world)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range strings.Split(parts[2], ";") {
		var cx, cz int
		fmt.Sscanf(p, "%d,%d", &cx, &cz)
		r := savedStarts(m.worldDir, cx, cz)
		for _, st := range r {
			for e := range m.Eras {
				store, err := s.StructureAssetStoreEra(root.ID, root.SelectedVersion, world, m.Eras[e].Without)
				if err != nil {
					t.Fatal(err)
				}
				smp, _ := s.EraSampler(ctx, root.ID, root.SelectedVersion, world, e)
				info, _ := smp.TerrainInfo("minecraft:overworld")
				height := func(x, z int) int {
					if h, ok := smp.TerrainSurfaceAt("minecraft:overworld", x, z); ok {
						return h
					}
					return 63
				}
				sc, err := structure3d.AssembleTerrain(store, st.ID, level.RandomSeed, cx*16, cz*16, structure3d.OverworldHeight, samplerTerrain(smp, "minecraft:overworld", info, height))
				store.Close()
				if err != nil {
					t.Logf("%s @%d,%d era%d: %v", st.ID, cx, cz, e, err)
					continue
				}
				n := scenePiecesMatched(st.Pieces, sc)
				t.Logf("%s @%d,%d era%d: %d/%d saved boxes (scene %d pieces)", st.ID, cx, cz, e, n, len(st.Pieces), len(sc.Pieces))
				if n < len(st.Pieces) {
					for _, sp := range st.Pieces {
						t.Logf("   saved %s %s %v raw %v", sp.Template, sp.Rotation, sp.BB, sp.Raw)
					}
					t.Logf("   scene bounds %v %v", sc.Min, sc.Max)
				}
				if len(sc.Pieces) > 0 {
					t.Logf("   saved0 %s %v | got0 %s %v %v", st.Pieces[0].Template, st.Pieces[0].BB, sc.Pieces[0].Name, sc.Pieces[0].Min, sc.Pieces[0].Max)
				}
			}
		}
	}
}
