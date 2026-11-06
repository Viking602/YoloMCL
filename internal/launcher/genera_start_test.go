package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
)

// MCL_START_TRY="inst|world" resolves every saved jigsaw start (or the
// MCL_JIGSAW_ONLY id) through StartStore and logs which are exact.
func TestStartStoreSaves(t *testing.T) {
	spec := os.Getenv("MCL_START_TRY")
	if spec == "" {
		t.Skip("set MCL_START_TRY")
	}
	s := testService(t)
	ctx := context.Background()
	inst, world, _ := strings.Cut(spec, "|")
	root := Root{ID: "st", Path: filepath.Dir(filepath.Dir(inst)), SelectedVersion: filepath.Base(inst)}
	s.cfg.Roots = []Root{root}
	only := os.Getenv("MCL_JIGSAW_ONLY")
	paths, _ := mcworld.RegionPaths(filepath.Join(inst, "saves", world), "")
	exact, total := 0, 0
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
				if len(st.Pieces) == 0 || !strings.Contains(st.ID, only) {
					continue
				}
				store, ok, err := s.StartStore(ctx, root.ID, root.SelectedVersion, world, st.ID, st.ChunkX, st.ChunkZ)
				if err != nil {
					t.Logf("%s @%d,%d: %v", st.ID, st.ChunkX, st.ChunkZ, err)
					continue
				}
				store.Close()
				startEditMu.Lock()
				d := startEditCache[strings.Join([]string{root.ID, root.SelectedVersion, world, st.ID}, "|")+"|"+strconv.Itoa(st.ChunkX)+","+strconv.Itoa(st.ChunkZ)]
				startEditMu.Unlock()
				total++
				if ok {
					exact++
				}
				t.Logf("START %s @%d,%d exact=%v drop=%v", st.ID, st.ChunkX, st.ChunkZ, ok, d)
			}
		}
	}
	t.Logf("SUMMARY exact %d/%d", exact, total)
}
