package launcher

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/Viking602/YoloMCL/cubiomes"
)

func TestSeedMapSurfaceSeamsAndInspection(t *testing.T) {
	ctx := context.Background()
	snap, err := WorldgenIngest(ctx, WorldgenInput{Minecraft: "26.3"})
	if err != nil {
		t.Fatal(err)
	}
	s := testSeedMapSampler(t, snap, -557181663597411692)
	if !s.SupportsSurface("minecraft:overworld") || s.SupportsSurface("minecraft:the_nether") {
		t.Fatal("surface capability")
	}
	for _, step := range []int{1, 4} {
		key := SeedMapTileKey{X: -2, Z: 1, BlocksPerPixel: step, Y: 64, Dimension: "minecraft:overworld", Surface: true}
		left, err := s.Sample(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		key.X++
		right, err := s.Sample(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		for z := 1; z <= 128; z++ {
			if left.surface[z*seedMapSurfaceWidth+128].Height != right.surface[z*seedMapSurfaceWidth].Height || left.surface[z*seedMapSurfaceWidth+129].Height != right.surface[z*seedMapSurfaceWidth+1].Height {
				t.Fatal("terrain halo differs across tile seam")
			}
		}
		// Compare the whole left tile's rendered heights to a wider independent
		// native raster; no edge copies or clamped derivatives are allowed.
		d := s.dimensions[key.Dimension]
		ref := make([]cubiomes.SurfaceSample, 258*130)
		if err := d.engine.NewSampler().GenerateSurface(ctx, cubiomes.SurfaceRange{X: -256*step - step, Z: 128*step - step, SizeX: 258, SizeZ: 130, Step: step, SeaLevel: 63, Voronoi: step < 4}, ref); err != nil {
			t.Fatal(err)
		}
		for z := 1; z <= 128; z++ {
			for x := 1; x <= 128; x++ {
				if left.surface[z*130+x].Height != ref[z*258+x].Height {
					t.Fatal("height depends on tile subdivision")
				}
			}
		}
		key.Surface = false
		slice, err := s.Sample(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(slice.Image.Pix, right.Image.Pix) {
			t.Fatal("relief did not change raster")
		}
		view := SeedMapViewport{X: -128 * step, Z: 192 * step, Width: 256, Height: 128, BlocksPerPixel: step, Y: 64, Dimension: key.Dimension, Surface: true}
		frame := SeedMapFrame{Viewport: view, tiles: []*SeedMapTile{left, right}}
		for _, px := range []int{0, 127, 128, 255} {
			p, ok := frame.PointAt(px, 40)
			if !ok || !p.Surface || p.X != (-256+px)*step || p.Z != 168*step || p.Biome.ID == "" {
				t.Fatalf("point %d: %+v %v", px, p, ok)
			}
		}
		partial := frame
		partial.tiles = []*SeedMapTile{left}
		if _, ok := partial.PointAt(200, 40); ok {
			t.Fatal("unrendered tile has point data")
		}
		if _, ok := frame.PointAt(-1, 40); ok {
			t.Fatal("out-of-bounds point")
		}
	}
}

func TestSeedMapUnalignedCoordinates(t *testing.T) {
	ctx := context.Background()
	snapshot, err := WorldgenIngest(ctx, WorldgenInput{Minecraft: "1.21.1"})
	if err != nil {
		t.Fatal(err)
	}
	sampler, err := NewLocalSampler(ctx, snapshot, 42)
	if err != nil {
		t.Fatal(err)
	}
	tiles := NewSeedMapTiles(ctx, sampler)
	defer func() { tiles.Close(); <-tiles.Done() }()
	// An odd-sized viewport whose center is off the quart lattice. The visible
	// samples must agree with both the image and coordinates across negative tiles.
	view := SeedMapViewport{Dimension: "minecraft:overworld", X: -130, Y: 64, Z: 7, Width: 255, Height: 129, BlocksPerPixel: 4}
	frames := make(chan SeedMapFrame, 1)
	if _, err := tiles.Request(view, func(frame SeedMapFrame) { frames <- frame }); err != nil {
		t.Fatal(err)
	}
	var frame SeedMapFrame
	select {
	case frame = <-frames:
	case <-time.After(10 * time.Second):
		t.Fatal("map request timed out")
	}
	if frame.Err != nil {
		t.Fatal(frame.Err)
	}
	for _, pixel := range [][2]int{{0, 0}, {1, 1}, {128, 64}, {254, 128}} {
		point, ok := frame.PointAt(pixel[0], pixel[1])
		if !ok || point.X != -640+pixel[0]*4 || point.Z != -248+pixel[1]*4 {
			t.Fatalf("pixel %v point %+v/%v", pixel, point, ok)
		}
		if point.X != 0 && point.Z != 0 && frame.Image.RGBAAt(pixel[0], pixel[1]) != point.Biome.Color {
			t.Fatalf("pixel %v displays a different biome than the point inspector", pixel)
		}
	}
}
