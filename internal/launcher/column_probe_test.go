package launcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
)

// MCL_COLUMN_PROBE="inst|world|cx,cz" logs saved heightmaps, the top
// blocks and every era's noise surface at the chunk centre. Read-only.
func TestColumnProbe(t *testing.T) {
	spec := os.Getenv("MCL_COLUMN_PROBE")
	if spec == "" {
		t.Skip("set MCL_COLUMN_PROBE")
	}
	parts := strings.Split(spec, "|")
	inst, world := parts[0], parts[1]
	var cx, cz int
	fmt.Sscanf(parts[2], "%d,%d", &cx, &cz)
	s := testService(t)
	ctx := context.Background()
	root := Root{ID: "cp", Path: filepath.Dir(filepath.Dir(inst)), SelectedVersion: filepath.Base(inst)}
	s.cfg.Roots = []Root{root}
	m, err := s.DetectEras(ctx, root.ID, root.SelectedVersion, world)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("era verdict %+v", m.EraAt(cx, cz))
	r, err := mcworld.OpenRegion(filepath.Join(inst, "saves", world, "region", fmt.Sprintf("r.%d.%d.mca", cx>>5, cz>>5)))
	if err != nil {
		t.Fatal(err)
	}
	c, _ := r.Chunk(cx&31, cz&31)
	if c == nil {
		t.Fatal("no chunk")
	}
	for _, lx := range []int{0, 8, 15} {
		for _, lz := range []int{0, 8, 15} {
			line := fmt.Sprintf("@%d,%d", cx*16+lx, cz*16+lz)
			for _, k := range []string{"WORLD_SURFACE_WG", "WORLD_SURFACE", "OCEAN_FLOOR", "MOTION_BLOCKING"} {
				h, ok := c.Heightmap(k, lx, lz, -64)
				line += fmt.Sprintf(" %s=%d/%v", k, h, ok)
			}
			hs, _ := c.Heightmap("WORLD_SURFACE", lx, lz, -64)
			for y := hs - 1; y > hs-6; y-- {
				line += " " + strings.TrimPrefix(c.Block(lx, y, lz), "minecraft:")
			}
			line += " biome=" + c.Biome(lx, 64, lz)
			for e := range m.Eras {
				smp, err := s.EraSampler(ctx, root.ID, root.SelectedVersion, world, e)
				if err != nil {
					continue
				}
				h, ok := smp.TerrainSurfaceAt("minecraft:overworld", cx*16+lx, cz*16+lz)
				line += fmt.Sprintf(" era%d=%d/%v", e, h, ok)
			}
			t.Log(line)
		}
	}
}
