package structure3d_test

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/Viking602/YoloMCL/internal/launcher"
	"github.com/Viking602/YoloMCL/internal/structure3d"
	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// seedGround is the seed's own pre-structure overworld: the compiled
// final_density + aquifer column with the vanilla cave/canyon carvers cut in
// (carved cells are air, lava at or below worldgen.CarverLavaY). It replaces
// guessing the ground from the save, where the structure has already
// overwritten caves it built into. STRUCTURE_SEED_GROUND="instDir|world".
type seedGround struct {
	sampler *launcher.LocalSampler
	seed    int64
	minY    int
	mu      sync.Mutex
	cols    map[[2]int][]launcher.TerrainSubstance
	shapes  map[[2]int32][]worldgen.CarveShape
}

func newSeedGround(t *testing.T, seed int64) *seedGround {
	spec := os.Getenv("STRUCTURE_SEED_GROUND")
	if spec == "" {
		return nil
	}
	inst, world, _ := strings.Cut(spec, "|")
	s, err := launcher.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, err := s.AddRoot(inst, "seedground")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	snap, _, err := s.SeedMapWorldgen(ctx, root.ID, root.SelectedVersion, world)
	if err != nil {
		t.Fatal(err)
	}
	sm, err := launcher.NewLocalSampler(ctx, snap, seed)
	if err != nil {
		t.Fatal(err)
	}
	info, ok := sm.TerrainInfo("minecraft:overworld")
	if !ok {
		t.Fatal("no overworld terrain")
	}
	return &seedGround{sampler: sm, seed: seed, minY: info.MinY, cols: map[[2]int][]launcher.TerrainSubstance{}, shapes: map[[2]int32][]worldgen.CarveShape{}}
}

func (g *seedGround) kind(x, y, z int) structure3d.GroundKind {
	g.mu.Lock()
	defer g.mu.Unlock()
	k := [2]int{x, z}
	col, ok := g.cols[k]
	if !ok {
		info, _ := g.sampler.TerrainInfo("minecraft:overworld")
		col = make([]launcher.TerrainSubstance, info.Height)
		g.sampler.TerrainColumn("minecraft:overworld", x, z, col)
		g.carve(x, z, col)
		g.cols[k] = col
	}
	i := y - g.minY
	if i < 0 || i >= len(col) {
		return structure3d.GroundAir
	}
	switch col[i] {
	case launcher.TerrainSolid:
		return structure3d.GroundSolid
	case launcher.TerrainWater, launcher.TerrainLava:
		return structure3d.GroundLiquid
	}
	return structure3d.GroundAir
}

func (g *seedGround) carve(x, z int, col []launcher.TerrainSubstance) {
	cx, cz := int32(x>>4), int32(z>>4)
	for dx := int32(-8); dx <= 8; dx++ {
		for dz := int32(-8); dz <= 8; dz++ {
			key := [2]int32{cx + dx, cz + dz}
			ss, ok := g.shapes[key]
			if !ok {
				ss = worldgen.OverworldCarves(g.seed, key[0], key[1])
				g.shapes[key] = ss
			}
			for i := range ss {
				b := ss[i].Bounds()
				if x < b[0] || x > b[3] || z < b[2] || z > b[5] {
					continue
				}
				for y := max(b[1], g.minY); y <= b[4] && y-g.minY < len(col); y++ {
					if col[y-g.minY] == launcher.TerrainSolid && ss[i].Carved(x, y, z) {
						if y <= worldgen.CarverLavaY {
							col[y-g.minY] = launcher.TerrainLava
						} else {
							col[y-g.minY] = launcher.TerrainAir
						}
					}
				}
			}
		}
	}
}
