package launcher

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"reflect"
	"testing"
	"time"
)

func TestSeedMapDenseHitsCoverEntireViewport(t *testing.T) {
	worker := newSeedMapTiles(context.Background(), func(_ context.Context, key SeedMapTileKey) (*SeedMapTile, error) {
		tile := &SeedMapTile{Key: key, Image: image.NewRGBA(image.Rect(0, 0, 128, 128))}
		for z := 4; z < 128; z += 8 {
			for x := 4; x < 128; x += 8 {
				tile.Hits = append(tile.Hits, FeatureHit{ID: "test:site", X: key.X*128 + x, Z: key.Z*128 + z})
			}
		}
		return tile, nil
	})
	defer func() { worker.Close(); <-worker.Done() }()
	view := SeedMapViewport{X: 384, Z: 256, Width: 768, Height: 512, BlocksPerPixel: 1}
	frames := make(chan SeedMapFrame, 64)
	if _, err := worker.RequestProgressive(view, func(f SeedMapFrame) { frames <- f }); err != nil {
		t.Fatal(err)
	}
	var first SeedMapFrame
	for {
		first = awaitSeedMap(t, frames)
		if !first.Partial {
			break
		}
	}
	if first.Err != nil || !first.Limited || len(first.Hits) != 2048 {
		t.Fatalf("dense viewport: hits=%d limited=%v err=%v", len(first.Hits), first.Limited, first.Err)
	}
	tiles := map[[2]int]int{}
	for _, h := range first.Hits {
		tiles[[2]int{h.X / 128, h.Z / 128}]++
	}
	if len(tiles) != 24 {
		t.Fatalf("center-first clipping lost outer tiles: %v", tiles)
	}
	for k, n := range tiles {
		if n < 50 {
			t.Fatalf("tile %v retained only %d markers", k, n)
		}
	}
	if _, err := worker.Request(view, func(f SeedMapFrame) { frames <- f }); err != nil {
		t.Fatal(err)
	}
	second := awaitSeedMap(t, frames)
	if !reflect.DeepEqual(first.Hits, second.Hits) {
		t.Fatal("cached/non-progressive request changed the retained markers")
	}
}

func TestSeedMapSamplingFailureReachesFrame(t *testing.T) {
	failure := errors.New("sampling failed")
	worker := newSeedMapTiles(context.Background(), func(context.Context, SeedMapTileKey) (*SeedMapTile, error) { return nil, failure })
	defer func() { worker.Close(); <-worker.Done() }()
	frames := make(chan SeedMapFrame, 1)
	_, err := worker.Request(SeedMapViewport{Width: 8, Height: 8, BlocksPerPixel: 4}, func(frame SeedMapFrame) { frames <- frame })
	if err != nil {
		t.Fatal(err)
	}
	select {
	case frame := <-frames:
		if !errors.Is(frame.Err, failure) || len(frame.Biomes) != 0 {
			t.Fatalf("sampling failure hidden: %+v", frame)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("worker swallowed sampling failure")
	}
}

func TestSeedMapMarkersDoNotChangeBiomeRaster(t *testing.T) {
	fill := color.RGBA{30, 150, 80, 255}
	worker := newSeedMapTiles(context.Background(), func(_ context.Context, key SeedMapTileKey) (*SeedMapTile, error) {
		img := image.NewRGBA(image.Rect(0, 0, SeedMapTilePixels, SeedMapTilePixels))
		draw.Draw(img, img.Rect, image.NewUniform(fill), image.Point{}, draw.Src)
		return &SeedMapTile{Key: key, Image: img, Hits: []FeatureHit{{ID: "test:site", X: 64, Z: 64}}}, nil
	})
	defer func() { worker.Close(); <-worker.Done() }()
	frames := make(chan SeedMapFrame, 1)
	if _, err := worker.Request(SeedMapViewport{X: 64, Z: 64, Width: 128, Height: 128, BlocksPerPixel: 1}, func(frame SeedMapFrame) { frames <- frame }); err != nil {
		t.Fatal(err)
	}
	select {
	case frame := <-frames:
		if len(frame.Hits) != 1 || frame.Image.RGBAAt(64, 64) != fill {
			t.Fatal("structure marker was painted into the biome raster")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("map worker did not complete")
	}
}

func TestSeedMapProgressiveFramesAreImmutable(t *testing.T) {
	worker := newSeedMapTiles(context.Background(), func(_ context.Context, key SeedMapTileKey) (*SeedMapTile, error) {
		img := image.NewRGBA(image.Rect(0, 0, SeedMapTilePixels, SeedMapTilePixels))
		draw.Draw(img, img.Rect, image.NewUniform(color.RGBA{30, 150, 80, 255}), image.Point{}, draw.Src)
		return &SeedMapTile{Key: key, Image: img}, nil
	})
	defer func() { worker.Close(); <-worker.Done() }()
	frames := make(chan SeedMapFrame, 64)
	view := SeedMapViewport{Width: 260, Height: 132, BlocksPerPixel: 1}
	if _, err := worker.RequestProgressive(view, func(frame SeedMapFrame) { frames <- frame }); err != nil {
		t.Fatal(err)
	}
	var first *image.RGBA
	var saved []byte
	for {
		select {
		case frame := <-frames:
			if first == nil {
				if !frame.Partial {
					t.Fatal("uncached viewport did not publish a partial frame")
				}
				first, saved = frame.Image, append([]byte(nil), frame.Image.Pix...)
			}
			if !frame.Partial {
				if !bytes.Equal(first.Pix, saved) || bytes.Equal(frame.Image.Pix, saved) {
					t.Fatal("partial image mutated or final frame never completed")
				}
				return
			}
		case <-time.After(5 * time.Second):
			t.Fatal("progressive viewport did not complete")
		}
	}
}

// Both modes recompute the same viewport with an empty tile cache. The extra
// metric measures the first immutable image available to the UI, excluding
// the UI thread's scheduling and GPU upload.
func BenchmarkSeedMapViewportFirstPaint(b *testing.B) {
	ctx := context.Background()
	snapshot, err := WorldgenIngest(ctx, WorldgenInput{Minecraft: "1.20.1"})
	if err != nil {
		b.Fatal(err)
	}
	sampler, err := NewLocalSampler(ctx, snapshot, 42)
	if err != nil {
		b.Fatal(err)
	}
	for _, mode := range []struct {
		name        string
		step        int
		progressive bool
		surface     bool
	}{{"complete", 4, false, false}, {"progressive", 4, true, false}, {"complete-block", 1, false, false}, {"progressive-block", 1, true, false}, {"surface-overview", 4, true, true}, {"surface-detail", 1, true, true}} {
		b.Run(mode.name, func(b *testing.B) {
			var firstTotal, completeTotal time.Duration
			b.ReportAllocs()
			for b.Loop() {
				start := time.Now()
				worker := NewSeedMapTiles(ctx, sampler)
				view := SeedMapViewport{X: 8192, Z: -8192, Y: 64, Width: 640, Height: 384, BlocksPerPixel: mode.step, Dimension: "minecraft:overworld", Surface: mode.surface}
				var first time.Duration
				var frameErr error
				deliver := func(frame SeedMapFrame) {
					if first == 0 {
						first = time.Since(start)
					}
					if !frame.Partial {
						frameErr = frame.Err
						worker.Close()
					}
				}
				if mode.progressive {
					_, err = worker.RequestProgressive(view, deliver)
				} else {
					_, err = worker.Request(view, deliver)
				}
				if err != nil {
					worker.Close()
					b.Fatal(err)
				}
				<-worker.Done()
				completeTotal += time.Since(start)
				if frameErr != nil {
					b.Fatal(frameErr)
				}
				firstTotal += first
			}
			b.ReportMetric(float64(firstTotal.Nanoseconds())/float64(b.N), "first-paint-ns/op")
			b.ReportMetric(float64(completeTotal.Nanoseconds())/float64(b.N), "complete-ns/op")
		})
	}
}
