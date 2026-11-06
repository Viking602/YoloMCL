package nativeui

import (
	"context"
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func seedMapReliefFixture(t testing.TB) *seedMapView {
	t.Helper()
	ctx := context.Background()
	snapshot, err := launcher.WorldgenIngest(ctx, launcher.WorldgenInput{Minecraft: "26.3"})
	if err != nil {
		t.Fatal(err)
	}
	sampler, err := launcher.NewLocalSampler(ctx, snapshot, -557181663597411692)
	if err != nil {
		t.Fatal(err)
	}
	// These tests paint terrain and reuse textures; the village filter avoids
	// unrelated full-world stronghold-ring searches when preparing each raster.
	v := &seedMapView{viewport: launcher.SeedMapViewport{X: -100, Z: 240, Width: 192, Height: 128, BlocksPerPixel: 1, Y: 64, Dimension: "minecraft:overworld", Structure: "minecraft:villages", Surface: true, ContourOverlay: true}}
	v.requested = v.viewport
	v.syncCamera()
	v.raster.textures = make(map[launcher.SeedMapTileKey]*seedMapTexture)
	x, z := v.viewport.Origin()
	for tz := z >> 7; tz <= (z+v.viewport.Height-1)>>7; tz++ {
		for tx := x >> 7; tx <= (x+v.viewport.Width-1)>>7; tx++ {
			key := launcher.SeedMapTileKey{X: tx, Z: tz, Y: 64, BlocksPerPixel: 1, Dimension: v.viewport.Dimension, Structure: v.viewport.Structure, Surface: true, ContourOverlay: true}
			tile, err := sampler.Sample(ctx, key)
			if err != nil {
				t.Fatal(err)
			}
			v.raster.textures[key] = &seedMapTexture{tile: tile}
		}
	}
	return v
}

func TestSeedMapCameraAndTextureReuse(t *testing.T) {
	v := seedMapReliefFixture(t)
	now := time.Now()
	before := v.drawTiles(now, 768, 512, 4)
	uploads, uploadBytes := v.raster.uploads, v.raster.uploadBytes
	bitmaps := map[launcher.SeedMapTileKey]*ui.Bitmap{}
	for _, tile := range before {
		bitmaps[tile.texture.tile.Key] = tile.texture.bitmap
	}
	for range 32 {
		v.placeCamera(v.camera.x-.125, v.camera.z+.0625)
	}
	after := v.drawTiles(now.Add(time.Second), 768, 512, 4)
	if v.camera.x != -104 || v.camera.z != 242 || v.raster.uploads != uploads || v.raster.uploadBytes != uploadBytes {
		t.Fatal("fractional pan was rounded away or reuploaded cached textures")
	}
	for _, tile := range after {
		if tile.texture.bitmap != bitmaps[tile.texture.tile.Key] {
			t.Fatal("camera move rebuilt a relief texture")
		}
	}
	if math.Abs(float64(after[0].rect.X-before[0].rect.X-16)) > .01 || math.Abs(float64(after[0].rect.Y-before[0].rect.Y+8)) > .01 {
		t.Fatal("cached map did not immediately follow camera before a sampling result")
	}
	// Terrain slices and dimensions never reuse pixels from a different world layer.
	v.viewport.Y++
	if len(v.drawTiles(now, 768, 512, 4)) != 0 {
		t.Fatal("old Y slice leaked into a different layer")
	}
	// The display cache has a bounded lifetime and cannot grow during long pans.
	for i := range 250 {
		key := launcher.SeedMapTileKey{X: 1000 + i, BlocksPerPixel: 1}
		v.raster.textures[key] = &seedMapTexture{tile: &launcher.SeedMapTile{Key: key, Image: image.NewRGBA(image.Rect(0, 0, 128, 128))}}
	}
	v.raster.prune()
	if len(v.raster.textures) > seedMapTextureCount || v.raster.bytes > seedMapTextureBytes {
		t.Fatal("display cache is unbounded")
	}
}

func TestSeedMapInertiaStopsAndHonoursReducedMotion(t *testing.T) {
	now := time.Now()
	n := seedMapCamera{vx: 900, vz: -300, lastMove: now}
	n.release(now, false, 1)
	x1, _, moving := n.advance(now.Add(50 * time.Millisecond))
	x2, _, _ := n.advance(now.Add(100 * time.Millisecond))
	x3, _, _ := n.advance(now.Add(150 * time.Millisecond))
	if !moving || x1 <= 0 || x3-x2 >= x2-x1 {
		t.Fatal("release did not decelerate")
	}
	_, _, moving = n.advance(now.Add(time.Second))
	if moving || !n.coast.IsZero() {
		t.Fatal("inertia continued requesting frames after settling")
	}
	n.release(now, true, 1)
	if !n.coast.IsZero() {
		t.Fatal("reduced motion still started inertia")
	}
	n.release(now.Add(time.Second), false, 1)
	if !n.coast.IsZero() {
		t.Fatal("holding the pointer still before release started inertia")
	}
}

func TestSeedMapPaintAnimationsSettle(t *testing.T) {
	for _, lessMotion := range []bool{false, true} {
		v := seedMapReliefFixture(t)
		painted, built := 0, 0
		loading := false
		tt := ui.NewTester(func(c *ui.Context) {
			built++
			tiles := v.drawTiles(c.Now(), 384, 256, 2)
			ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
				painted++
				seedMapPaintTiles(p, r, tiles, lessMotion)
			})
			if loading {
				seedMapLoading(c, 1, "Loading map", nil)
			}
		}, 384, 256)
		tt.SetPreferences(ui.Preferences{ReduceMotion: lessMotion})
		start, builds := painted, built
		tt.Advance(time.Second)
		if painted-start > 7 || built != builds {
			t.Fatalf("fade rebuilt UI or exceeded 30 fps cap: paints=%d builds=%d", painted-start, built-builds)
		}
		if lessMotion && painted != start {
			t.Fatal("reduced motion still repainted tile fades")
		}
		start = painted
		tt.Advance(time.Second)
		if painted != start {
			t.Fatal("completed tiles kept repainting an idle map")
		}
		loading = true
		tt.Frame()
		start, builds = painted, built
		tt.Advance(time.Second)
		if painted-start > 13 || built != builds {
			t.Fatal("loading indicator exceeded 12 fps or rebuilt the view")
		}
		if lessMotion && painted != start {
			t.Fatal("reduced-motion loading indicator kept repainting")
		}
		loading = false
		tt.Frame()
		tt.Advance(time.Second)
		start = painted
		tt.Advance(time.Second)
		if painted != start {
			t.Fatal("loading repaint timer survived after completion")
		}
	}
}

func TestSeedMapDeliveryCoalesces(t *testing.T) {
	d := &seedMapDelivery{}
	for i := range 20 {
		if posted := d.put(launcher.SeedMapFrame{Generation: uint64(i), Partial: i < 19}); posted != (i == 0) {
			t.Fatal("more than one UI post queued")
		}
	}
	if frame := d.take(); frame.Generation != 19 || frame.Partial {
		t.Fatal("coalescing lost the completed frame")
	}
	if !d.put(launcher.SeedMapFrame{Generation: 20}) {
		t.Fatal("consumption prevented subsequent delivery")
	}
}

// Compare the previous fine-contour paint loop with cached display tiles using
// identical real terrain. Headless CPU paint time is not hardware GPU time.
func BenchmarkSeedMapReliefPaint(b *testing.B) {
	v := seedMapReliefFixture(b)
	plan := v.drawTiles(time.Now().Add(-time.Second), 768, 512, 4)
	type oldTile struct {
		bitmap *ui.Bitmap
		edges  []launcher.SeedMapContour
		rect   ui.Rect
	}
	var old []oldTile
	primitives := 0
	for _, tile := range plan {
		// The old compositor culled offscreen contours. Keep the baseline to
		// the same visible viewport rather than counting entire padded tiles.
		var edges []launcher.SeedMapContour
		for _, edge := range tile.texture.tile.Contours() {
			x, y := tile.rect.X+float32(edge.X)*4, tile.rect.Y+float32(edge.Z)*4
			w, h := float32(1), float32(4)
			if edge.Horizontal {
				w, h = h, w
			}
			if x < 768 && y < 512 && x+w > 0 && y+h > 0 {
				edges = append(edges, edge)
			}
		}
		old = append(old, oldTile{ui.NewBitmap(tile.texture.tile.Image).Pixelated(), edges, tile.rect})
		primitives += len(edges) + 1
	}
	for _, cached := range []bool{false, true} {
		name := "vector_contours"
		if cached {
			name = "cached_tiles"
		}
		b.Run(name, func(b *testing.B) {
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Box(c).Fill().Clip().Draw(func(p *ui.Painter, r ui.Rect) {
					if cached {
						seedMapPaintTiles(p, r, plan, true)
						return
					}
					for _, tile := range old {
						p.ImageRegion(tile.bitmap, ui.Rect{}, tile.rect, ui.ImageOptions{})
						for _, edge := range tile.edges {
							x, y := tile.rect.X+float32(edge.X)*4, tile.rect.Y+float32(edge.Z)*4
							color := ink.Alpha(.52)
							if edge.Light {
								color = panel.Alpha(.65)
							}
							if edge.Boundary {
								color = ink.Alpha(.8)
							}
							if edge.Horizontal {
								p.Fill(ui.Rect{X: x, Y: y, W: 4, H: 1}, color, 0)
							} else {
								p.Fill(ui.Rect{X: x, Y: y, W: 1, H: 4}, color, 0)
							}
						}
					}
				})
			}, 768, 512)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tt.Frame()
			}
			if cached {
				b.ReportMetric(float64(len(plan)), "primitives/frame")
			} else {
				b.ReportMetric(float64(primitives), "primitives/frame")
			}
		})
	}
}
