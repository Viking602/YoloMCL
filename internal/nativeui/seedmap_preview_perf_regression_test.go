package nativeui

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestPreviewGroundPrefetchCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g := &previewGround{ctx: ctx, step: 1, cols: map[[2]int]*groundColumn{}}
	done := make(chan struct{})
	go func() {
		g.prefetch(0, 0, 512, 512)
		close(done)
	}()
	// Cancel during dispatch, including when the work queue is full.
	time.Sleep(time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled column workers did not exit")
	}
}

func TestStructurePreviewCancelledWithMap(t *testing.T) {
	for _, closeMap := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		v := &seedMapView{structurePreview: &seedMapStructurePreview{cancel: cancel}}
		v.viewport.Dimension = "minecraft:overworld"
		a := &App{seedMap: v}
		a.seedMapDimension(v, v.viewport.Dimension)
		if ctx.Err() != nil || v.structurePreview == nil {
			t.Fatal("unchanged dimension discarded the preview")
		}
		if closeMap {
			a.closeSeedMap()
		} else {
			a.seedMapDimension(v, "minecraft:the_nether")
		}
		if ctx.Err() == nil || v.structurePreview != nil {
			t.Fatal("leaving the map/dimension kept the background build alive")
		}
		cancel()
	}
}

// Compare the bounded carver loop with the original exhaustive evaluation,
// including negative coordinates, fluids, air and protected bedrock rules.
func checkNetherCarverBounds(t *testing.T, sampler *launcher.LocalSampler, seed int64) {
	t.Helper()
	const dim = "minecraft:the_nether"
	g := &previewGround{seed: seed, dim: dim, sampler: sampler, info: launcher.SeedMapTerrainInfo{MinY: 0, Height: 128},
		rulePalette: []string{"", "minecraft:bedrock"}}
	fluid := sampler.NetherCarverFluid(dim)
	if !fluid.OK {
		t.Fatal("fixture lacks the nether carver")
	}
	for x := -32; x <= 32; x += 16 {
		for z := -31; z <= 33; z += 16 {
			c := &groundColumn{sub: make([]launcher.TerrainSubstance, 128), rules: &ruleColumn{idx: make([]uint8, 128)}}
			for y := range c.sub {
				c.sub[y] = launcher.TerrainSolid
			}
			c.sub[20], c.sub[40] = launcher.TerrainLava, launcher.TerrainAir
			c.rules.idx[1], c.rules.idx[70] = 1, 1
			want := slices.Clone(c.sub)
			for dx := int32(-8); dx <= 8; dx++ {
				for dz := int32(-8); dz <= 8; dz++ {
					for _, sh := range g.netherShapes(int32(x>>4)+dx, int32(z>>4)+dz) {
						for y := 1; y <= 120; y++ {
							if want[y] == launcher.TerrainSolid && g.ruleBlock(c, y) != "minecraft:bedrock" && sh.Carved(x, y, z) {
								want[y] = fluid.Substance(y)
							}
						}
					}
				}
			}
			g.netherCarve(x, z, c)
			if !slices.Equal(c.sub, want) {
				t.Fatalf("bounded carver changed column %d,%d", x, z)
			}
		}
	}
}
