package launcher

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
)

// MCL_START_DUMP="cx,cz;..." logs the saved start and pieces of each chunk.
func TestModStartDump(t *testing.T) {
	spec := os.Getenv("MCL_START_DUMP")
	if spec == "" {
		t.Skip("set MCL_START_DUMP")
	}
	world := "Codex_Compat_Regression_REI_20260925"
	s := testService(t)
	root := Root{ID: "se", Path: `D:\MC\.minecraft`, SelectedVersion: "project01"}
	s.cfg.Roots = []Root{root}
	m, err := s.DetectEras(context.Background(), root.ID, root.SelectedVersion, world)
	if err != nil {
		t.Fatal(err)
	}
	seed, _ := mcworld.LevelSeed(m.worldDir)
	t.Logf("seed %d", seed)
	for _, c := range strings.Split(spec, ";") {
		a, b, _ := strings.Cut(c, ",")
		cx, _ := strconv.Atoi(a)
		cz, _ := strconv.Atoi(b)
		for _, st := range savedStarts(m.worldDir, cx, cz) {
			t.Logf("%s @%d,%d full=%v era=%d pieces=%d", st.ID, cx, cz, savedChunkFull(m.worldDir, cx, cz), m.EraAt(cx, cz).Era, len(st.Pieces))
			for _, p := range st.Pieces {
				t.Logf("   %v", p.Raw)
			}
		}
	}
}
