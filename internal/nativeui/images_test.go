package nativeui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestResourceThumbnailsPreserveAspectAndPreviewSize(t *testing.T) {
	for _, tc := range []struct {
		name, key          string
		w, h, wantW, wantH int
	}{
		{"small", "icon", 32, 16, 32, 16},
		{"square", "icon", 1024, 1024, 128, 128},
		{"wide", "icon", 1024, 256, 128, 32},
		{"tall", "icon", 256, 1024, 32, 128},
		{"addon", "addon:sha1:mod", 1024, 1024, 96, 96},
		{"screenshot", "screenshot:r:v:large.png", 960, 540, 960, 540},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var data bytes.Buffer
			if err := png.Encode(&data, image.NewNRGBA(image.Rect(0, 0, tc.w, tc.h))); err != nil {
				t.Fatal(err)
			}
			source, err := decodeLocalImage(tc.key, data.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			if w, h := source.(*ui.Bitmap).Size(); w != tc.wantW || h != tc.wantH {
				t.Fatalf("got %dx%d, want %dx%d", w, h, tc.wantW, tc.wantH)
			}
		})
	}
}

func TestLargeImageDecodeQueueCancelsWithoutBlockingSmallIcons(t *testing.T) {
	largeImageDecode <- struct{}{}
	defer func() { <-largeImageDecode }()
	var big, small bytes.Buffer
	if err := png.Encode(&big, image.NewNRGBA(image.Rect(0, 0, 1025, 1025))); err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(&small, image.NewNRGBA(image.Rect(0, 0, 256, 256))); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := decodeThumbnailContext(ctx, big.Bytes(), 96); done <- err }()
	if _, err := decodeThumbnail(small.Bytes(), 96); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled decode: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("superseded large image kept waiting for the decoder")
	}
}

func TestResourceThumbnailKeepsTransparency(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 512, 512))
	for y := range 512 {
		for x := range 512 {
			src.SetNRGBA(x, y, color.NRGBA{200, 100, 40, 128})
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, src); err != nil {
		t.Fatal(err)
	}
	source, err := decodeResourceImage(data.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	render := func(source ui.ImageSource) color.RGBA {
		return ui.Render(func(c *ui.Context) {
			ui.Box(c).Fill().Background(ui.Hex("#123456")).Children(func() { ui.Image(c, source).Size(128, 128) })
		}, 128, 128, 1).RGBAAt(64, 64)
	}
	if got, want := render(source), render(ui.NewBitmap(src)); got != want {
		t.Fatalf("thumbnail changed premultiplied transparency: %v vs %v", got, want)
	}
}

func TestResourceThumbnailKeepsJPEGOrientation(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 512, 256))
	colors := []color.RGBA{{240, 30, 20, 255}, {20, 230, 40, 255}, {20, 40, 240, 255}, {230, 220, 20, 255}}
	for y := range 256 {
		for x := range 512 {
			src.SetRGBA(x, y, colors[(y/128)*2+x/256])
		}
	}
	var data bytes.Buffer
	if err := jpeg.Encode(&data, src, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		for orientation := 1; orientation <= 8; orientation++ {
			tiff := make([]byte, 26)
			copy(tiff, "II")
			if order == binary.BigEndian {
				copy(tiff, "MM")
			}
			order.PutUint16(tiff[2:], 42)
			order.PutUint32(tiff[4:], 8)
			order.PutUint16(tiff[8:], 1)
			order.PutUint16(tiff[10:], 0x0112)
			order.PutUint16(tiff[12:], 3)
			order.PutUint32(tiff[14:], 1)
			order.PutUint16(tiff[18:], uint16(orientation))
			encoded := append([]byte{0xff, 0xd8, 0xff, 0xe1, 0, 34}, []byte("Exif\x00\x00")...)
			encoded = append(encoded, tiff...)
			encoded = append(encoded, data.Bytes()[2:]...)
			thumb, err := decodeResourceImage(encoded)
			if err != nil {
				t.Fatal(err)
			}
			original, err := ui.DecodeBitmap(encoded)
			if err != nil {
				t.Fatal(err)
			}
			w, h := thumb.(*ui.Bitmap).Size()
			ow, oh := original.Size()
			if w*oh != h*ow || max(w, h) != 128 {
				t.Fatalf("orientation %d changed aspect: %dx%d vs %dx%d", orientation, w, h, ow, oh)
			}
			render := func(source ui.ImageSource) *image.RGBA {
				return ui.Render(func(c *ui.Context) { ui.Image(c, source).Size(float32(w), float32(h)) }, w, h, 1)
			}
			got, want := render(thumb), render(original)
			for _, point := range []image.Point{{X: w / 4, Y: h / 4}, {X: 3 * w / 4, Y: h / 4}, {X: w / 4, Y: 3 * h / 4}, {X: 3 * w / 4, Y: 3 * h / 4}} {
				if got.RGBAAt(point.X, point.Y) != want.RGBAAt(point.X, point.Y) {
					t.Fatalf("orientation %d changed a JPEG quadrant", orientation)
				}
			}
		}
	}
}

// Include transparent PNGs: keeping both a decoded NRGBA and a full-sized
// renderer bitmap used to double the transient cost of these small UI icons.
func BenchmarkResourceImageDecode(b *testing.B) {
	for _, edge := range []int{64, 512, 1024} {
		b.Run(fmt.Sprint(edge), func(b *testing.B) {
			img := image.NewNRGBA(image.Rect(0, 0, edge, edge))
			for y := range edge {
				for x := range edge {
					img.SetNRGBA(x, y, color.NRGBA{uint8(x), uint8(y), 120, 180})
				}
			}
			var encoded bytes.Buffer
			if err := png.Encode(&encoded, img); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				source, err := decodeResourceImage(encoded.Bytes())
				if err != nil {
					b.Fatal(err)
				}
				w, h := source.(*ui.Bitmap).Size()
				b.ReportMetric(float64(w*h*4), "retained-pixel-B/icon")
				runtime.KeepAlive(source)
			}
		})
	}
}

func TestNativeResourceImagesUseDiskCacheAndClearFailedLoads(t *testing.T) {
	s, e := launcher.New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a, e := New(s)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { a.cancel(); a.bgWG.Wait() }()
	raw := "https://cdn.example.test/real-icon.png"
	sum := sha256.Sum256([]byte(raw))
	file := filepath.Join(a.snapshot.DataDir, "cache", "resource-icons", hex.EncodeToString(sum[:])+".img")
	_ = os.MkdirAll(filepath.Dir(file), 0700)
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetNRGBA(x, y, color.NRGBA{90, 190, 120, 255})
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	if e = os.WriteFile(file, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	tt := ui.NewTester(a.View, 1320, 860)
	a.remoteImage(raw)
	waitUI(t, tt, func() bool {
		a.imageMu.Lock()
		defer a.imageMu.Unlock()
		return a.images[raw] != nil && !a.imagePending[raw]
	})
	a.imageMu.Lock()
	pending := a.imagePending[raw]
	a.imageMu.Unlock()
	if pending {
		t.Fatal("successful cached icon is still pending")
	}
	// Browsing other artwork can evict this bitmap. Returning to the card
	// must reload its disk copy even though the CDN is unreachable in this test.
	a.imageMu.Lock()
	for i := range 8 {
		a.cacheImageLocked(fmt.Sprint(i), ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 960, 540))), 0)
	}
	evicted := a.images[raw] == nil
	a.imageMu.Unlock()
	if !evicted {
		t.Fatal("cache pressure did not evict the old remote icon")
	}
	a.remoteImage(raw)
	waitUI(t, tt, func() bool {
		a.imageMu.Lock()
		defer a.imageMu.Unlock()
		return a.images[raw] != nil && !a.imagePending[raw]
	})
	bad := "not-an-https-url"
	a.remoteImage(bad)
	waitUI(t, tt, func() bool {
		a.imageMu.Lock()
		defer a.imageMu.Unlock()
		return !a.imagePending[bad]
	})
	a.imageMu.Lock()
	retry := a.imageRetry[bad]
	a.imageMu.Unlock()
	if !retry.After(time.Now()) {
		t.Fatal("failed image load did not enter a retryable state")
	}
	if source, e := decodeResourceImage([]byte(`<svg`)); e == nil || source != nil {
		t.Fatal("invalid SVG became a non-nil native image")
	}
	if source, e := decodeResourceImage([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><circle cx="5" cy="5" r="4" fill="#fff"/></svg>`)); e != nil || source == nil {
		t.Fatal("SVG resource icons are not supported", e)
	}
}

func TestRemoteThumbnailUpgradesWithoutBlinkingOrDuplicateCache(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := &App{ctx: ctx, page: "discover", events: make(chan func(), 8), imageSlots: make(chan struct{}, 1), imagePending: map[string]bool{}}
	a.snapshot.DataDir = t.TempDir()
	raw := "https://cdn.example.test/shared.png"
	sum := sha256.Sum256([]byte(raw))
	file := filepath.Join(a.snapshot.DataDir, "cache", "resource-icons", hex.EncodeToString(sum[:])+".img")
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewNRGBA(image.Rect(0, 0, 256, 128))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	// No service: both reads must use the existing disk cache, including an
	// upgrade from a native source smaller than the requested decode edge.
	if a.remoteImageSize(raw, 96) != nil {
		t.Fatal("uncached image did not load asynchronously")
	}
	a.bgWG.Wait()
	if !a.imageUses[raw].discover {
		t.Fatal("completed artwork was not associated with its requesting page")
	}
	small := a.remoteImageSize(raw, 96)
	if w, h := small.(*ui.Bitmap).Size(); w != 96 || h != 48 {
		t.Fatalf("feed thumbnail = %dx%d", w, h)
	}
	a.imageSlots <- struct{}{} // keep the upgrade pending for both requests
	for range 2 {
		if a.remoteImage(raw) != small {
			<-a.imageSlots
			t.Fatal("an in-flight upgrade hid the already displayed thumbnail")
		}
	}
	<-a.imageSlots
	a.bgWG.Wait()
	large := a.remoteImage(raw)
	if w, h := large.(*ui.Bitmap).Size(); w != 128 || h != 64 {
		t.Fatalf("dialog thumbnail = %dx%d", w, h)
	}
	if len(a.images) != 1 || a.remoteImageSize(raw, 96) != large || len(a.imagePending) != 0 {
		t.Fatal("a smaller request duplicated, downgraded or restarted the cached image")
	}
	if len(a.events) != 2 {
		t.Fatal("reusing the image requested another decode")
	}
	a.page = "home"
	if a.remoteImage(raw) != large || a.imageUses[raw].discover {
		t.Fatal("another page did not retain its shared artwork for Discover expiry")
	}
}

// Enable explicitly for the real API -> CDN -> decoder -> native widget check.
func TestLiveModrinthResourceIcons(t *testing.T) {
	if os.Getenv("MCL_LIVE_ICON_QA") != "1" {
		t.Skip("explicit live network check")
	}
	s, e := launcher.New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	result, e := s.Search(ctx, launcher.SearchQuery{Source: "modrinth", Type: "mod", Query: "Sodium"})
	if e != nil {
		t.Fatal(e)
	}
	if len(result.Projects) < 3 {
		t.Fatal("expected live Modrinth results")
	}
	result.Projects = result.Projects[:3]
	a, e := New(s)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { a.cancel(); a.bgWG.Wait() }()
	a.page = "discover"
	a.resourceType = "模组"
	a.query = "Sodium"
	a.searchResult = result
	tt := ui.NewTester(a.View, 1320, 860)
	deadline := time.Now().Add(40 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		all := true
		a.imageMu.Lock()
		for _, p := range result.Projects {
			if a.images[p.Icon] == nil {
				all = false
				break
			}
		}
		a.imageMu.Unlock()
		if all {
			tt.Advance(650 * time.Millisecond) // include the completed card/icon entrances
			snapshotUI(t, tt, "resources")
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("live resource icons did not reach native widgets")
}
