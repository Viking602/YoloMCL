package launcher

import (
	"context"
	"os"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	"github.com/Viking602/YoloMCL/internal/worldgen"
)

func TestPortalProbe(t *testing.T) {
	gameRoot := os.Getenv("MCL_PORTAL_PROBE_ROOT")
	if gameRoot == "" {
		t.Skip("set MCL_PORTAL_PROBE_ROOT to inspect a local project01 save")
	}
	world := "Codex_Compat_Regression_REI_20260925"
	s := testService(t)
	root := Root{ID: "se", Path: gameRoot, SelectedVersion: "project01"}
	s.cfg.Roots = []Root{root}
	m, err := s.DetectEras(context.Background(), root.ID, root.SelectedVersion, world)
	if err != nil {
		t.Fatal(err)
	}
	seed, _ := mcworld.LevelSeed(m.worldDir)
	for _, c := range [][2]int{{-26, -33}, {-23, 23}, {-314, -264}, {-276, -275}, {15, -16}, {14, 2}, {218, -553}, {211, -587}, {250, -538}, {260, -583}} {
		for _, st := range savedStarts(m.worldDir, c[0], c[1]) {
			if len(st.Pieces) == 0 {
				continue
			}
			p := st.Pieces[0].Raw
			line := ""
			for skip := 0; skip < 5; skip++ {
				r := worldgen.LargeFeatureRandom(seed, int32(c[0]), int32(c[1]))
				for i := 0; i < skip; i++ {
					r.NextFloat()
				}
				tp := ""
				if r.NextFloat() < 0.05 {
					tp = "giant_portal_" + string(rune('1'+r.NextInt(3)))
				} else {
					r.NextInt(10)
					if skip >= 9 {
						tp = "portal_" + pitoa(1+int(r.NextInt(10)))
					} else {
						tp = "portal_" + pitoa(1+int(r.NextInt(10)))
					}
				}
				rot := worldgen.Rotations[r.NextInt(4)]
				mir := "FRONT_BACK"
				if r.NextFloat() < 0.5 {
					mir = "NONE"
				}
				if "minecraft:ruined_portal/"+tp == p["Template"] && rot == p["Rotation"] && (mir == p["Mirror"] || (mir == "NONE" && p["Mirror"] == nil)) {
					line += " skip" + pitoa(skip)
				}
			}
			t.Logf("%s @%v saved %v %v %v vp=%v air=%v match:%s", st.ID, c, p["Template"], p["Rotation"], p["Mirror"], p["VerticalPlacement"], p["Properties"], line)
		}
	}
}

func pitoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return "1" + string(rune('0'+i-10))
}
