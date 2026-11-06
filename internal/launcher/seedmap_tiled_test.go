package launcher

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"testing"
)

func testSeedMapTileImage(_ context.Context, key SeedMapTileKey) (*SeedMapTile, error) {
	img := image.NewRGBA(image.Rect(0, 0, SeedMapTilePixels, SeedMapTilePixels))
	for y := range SeedMapTilePixels {
		for x := range SeedMapTilePixels {
			img.SetRGBA(x, y, color.RGBA{uint8(key.X*128 + x), uint8(key.Z*128 + y), 90, 255})
		}
	}
	return &SeedMapTile{Key: key, Image: img}, nil
}

func TestSeedMapTiledFramesMatchComposite(t *testing.T) {
	worker := newSeedMapTiles(context.Background(), testSeedMapTileImage)
	defer func() { worker.Close(); <-worker.Done() }()
	frames := make(chan SeedMapFrame, 64)
	view := SeedMapViewport{X: -113, Z: 91, Width: 383, Height: 211, BlocksPerPixel: 4}
	if _, err := worker.RequestTiles(view, func(frame SeedMapFrame) { frames <- frame }); err != nil {
		t.Fatal(err)
	}
	var first, final SeedMapFrame
	for {
		f := awaitSeedMap(t, frames)
		if f.Image != nil || f.Err != nil || len(f.Tiles()) == 0 {
			t.Fatalf("invalid tile frame: %+v", f)
		}
		if len(first.Tiles()) == 0 {
			first = f
		}
		if !f.Partial {
			final = f
			break
		}
	}
	if !first.Partial || len(first.Tiles()) != 1 {
		t.Fatal("first tile snapshot was mutated or delayed")
	}
	img := image.NewRGBA(image.Rect(0, 0, view.Width, view.Height))
	x, z := view.Origin()
	for _, tile := range final.Tiles() {
		span := SeedMapTilePixels * tile.Key.BlocksPerPixel
		left, top := (tile.Key.X*span-x)/view.BlocksPerPixel, (tile.Key.Z*span-z)/view.BlocksPerPixel
		draw.Draw(img, image.Rect(left, top, left+128, top+128), tile.Image, image.Point{}, draw.Src)
	}
	if _, err := worker.Request(view, func(frame SeedMapFrame) { frames <- frame }); err != nil {
		t.Fatal(err)
	}
	composite := awaitSeedMap(t, frames)
	if !bytes.Equal(img.Pix, composite.Image.Pix) {
		t.Fatal("tile rendering changed pixels at an unaligned negative origin")
	}
	if first.Tiles()[0] != final.Tiles()[0] {
		t.Fatal("progressive publication copied or replaced immutable tiles")
	}
}

// Same warm tile cache and camera path. Measures composition/publication work,
// not terrain generation, the UI event loop or actual hardware GPU execution.
func BenchmarkSeedMapCachedPan(b *testing.B) {
	for _, tiled := range []bool{false, true} {
		name := "composite"
		if tiled {
			name = "tiles"
		}
		b.Run(name, func(b *testing.B) {
			worker := newSeedMapTiles(context.Background(), testSeedMapTileImage)
			defer func() { worker.Close(); <-worker.Done() }()
			view := SeedMapViewport{Width: 768, Height: 512, BlocksPerPixel: 1}
			render := func() {
				worker.render(seedMapRequest{ctx: context.Background(), viewport: view, tiled: tiled, deliver: func(SeedMapFrame) {}})
			}
			for i := range 128 {
				view.X = i
				render()
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				view.X = i % 128
				render()
			}
		})
	}
}
