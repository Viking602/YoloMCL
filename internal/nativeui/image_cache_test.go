package nativeui

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"runtime"
	"testing"
	"time"
	"weak"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestImageCacheEvictsUnusedArtwork(t *testing.T) {
	for _, size := range []image.Point{{X: 32, Y: 32}, {X: 960, Y: 540}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			a := &App{}
			insert := func(key string) weak.Pointer[ui.Bitmap] {
				bitmap := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, size.X, size.Y)))
				a.cacheImageLocked(key, bitmap, 0)
				return weak.Make(bitmap)
			}
			old := insert("old")
			insert("visible")
			for i := range imageCacheLimit + 10 {
				if a.cachedImageLocked("visible") == nil {
					t.Fatal("cache pressure discarded visible artwork")
				}
				insert(fmt.Sprint(i))
				if len(a.images) > imageCacheLimit || a.imageBytes > imageCacheBudget {
					t.Fatalf("cache grew without bound: %d images, %d bytes", len(a.images), a.imageBytes)
				}
			}
			if a.images["old"] != nil {
				t.Fatal("unused artwork was retained")
			}
			runtime.GC()
			if old.Value() != nil {
				t.Fatal("evicted bitmap is still retained by cache bookkeeping")
			}
			before := a.imageBytes
			insert("visible")
			if a.imageBytes != before {
				t.Fatal("replacing an image counted its pixels twice")
			}
		})
	}
}

func TestImageContentsSharePixelsAndReleaseLastAlias(t *testing.T) {
	a := &App{}
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 256, 256))); err != nil {
		t.Fatal(err)
	}
	load := func(key string, edge int) ui.ImageSource {
		source, content, err := a.decodeImageContent(context.Background(), pngData.Bytes(), edge)
		if err != nil {
			t.Fatal(err)
		}
		a.cacheImageContentLocked(key, source, pngData.Len(), content)
		return a.images[key]
	}
	first := load("jar-version-1", 96)
	w := weak.Make(first.(*ui.Bitmap))
	before := a.imageBytes
	if next := load("jar-version-2", 96); next != first || a.imageBytes != before {
		t.Fatal("identical artwork was decoded or counted twice")
	}
	load("jar-version-1", 96)
	if a.imageBytes != before || len(a.imageContents) != 1 {
		t.Fatal("replacing an alias leaked its content reference")
	}
	large := load("dialog", 128)
	width, height := large.(*ui.Bitmap).Size()
	if large == first || width != 128 || height != 128 {
		t.Fatal("larger artwork reused an undersized thumbnail")
	}
	// Replacing an alias with a larger thumbnail must retain the smaller
	// resource until the last other alias leaves the LRU.
	load("jar-version-1", 128)
	a.deleteImageLocked("jar-version-2")
	if len(a.imageContents) != 1 {
		t.Fatal("the final small alias retained its pixels")
	}
	first = nil
	runtime.GC()
	if w.Value() != nil {
		t.Fatal("the content index kept evicted pixels alive")
	}
	a.deleteImageLocked("dialog")
	a.deleteImageLocked("jar-version-1")
	if len(a.imageContents) != 0 || a.imageBytes != 0 {
		t.Fatal("empty cache kept an index or incorrect byte total")
	}
}

func TestImageContentAliasPressureStaysBounded(t *testing.T) {
	a := &App{}
	bitmap := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 96, 96)))
	content := imageContentKey{edge: 96}
	for i := range imageCacheLimit + 32 {
		a.cacheImageContentLocked(fmt.Sprint(i), bitmap, 0, content)
	}
	if len(a.images) != imageCacheLimit || len(a.imageContents) != 1 || a.imageBytes != imageCost(bitmap, 0) {
		t.Fatal("aliases bypassed the entry budget or counted pixels repeatedly")
	}
	for key := range a.images {
		a.deleteImageLocked(key)
	}
	if len(a.imageContents) != 0 || a.imageBytes != 0 {
		t.Fatal("LRU eviction retained unreferenced contents")
	}
}

func TestNavigationCancelsQueuedArtworkAndAllowsImmediateRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := &App{ctx: ctx, events: make(chan func(), 4), imagePending: map[string]bool{}, imageSlots: make(chan struct{}, 1)}
	a.imageSlots <- struct{}{} // hold the decoder while navigating away
	a.localIcon("icon", func() ([]byte, error) {
		t.Error("obsolete artwork still performed disk work")
		return nil, nil
	})
	old := a.imageRequests["icon"]
	a.goPage("tasks")
	if old.Err() == nil {
		t.Fatal("navigation did not cancel the queued request")
	}
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 16, 16))); err != nil {
		t.Fatal(err)
	}
	a.localIcon("icon", func() ([]byte, error) { return data.Bytes(), nil })
	a.clearImagePending("icon", old)
	a.imageMu.Lock()
	if !a.imagePendingLocked("icon") || a.imageRequests["icon"] == old {
		t.Fatal("old completion removed the returning page's new request")
	}
	a.imageMu.Unlock()
	<-a.imageSlots
	a.bgWG.Wait()
	if a.images["icon"] == nil || len(a.imagePending) != 0 || len(a.imageRequests) != 0 || len(a.imageRetry) != 0 {
		t.Fatal("retry did not complete cleanly after cancelling offscreen artwork")
	}
	a.resetImageRequests()
	if a.images["icon"] == nil {
		t.Fatal("navigation discarded decoded pixels needed for a quick return")
	}
}

func TestImageCacheBoundsFailedLookups(t *testing.T) {
	a := &App{}
	a.failImageLocked("old", -time.Hour)
	for i := range imageRetryLimit + 10 {
		a.failImageLocked(fmt.Sprint(i), time.Hour)
	}
	if len(a.imageRetry) != imageRetryLimit || !a.imageRetry["old"].IsZero() {
		t.Fatal("failed lookups grow without a bound")
	}
	key := fmt.Sprint(imageRetryLimit + 9)
	if a.imageRetry[key].IsZero() {
		t.Fatal("most recent failure was discarded")
	}
	a.cacheImageLocked(key, ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 8, 8))), 0)
	if !a.imageRetry[key].IsZero() {
		t.Fatal("a successful image retained its failure state")
	}
}

func TestOverviewAndWidgetSharePendingImage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := &App{
		ctx: ctx, events: make(chan func(), 2),
		images: map[string]ui.ImageSource{}, imagePending: map[string]bool{},
		imageSlots: make(chan struct{}, 1), overviewImageSlots: make(chan struct{}, 1),
	}
	started, resume := make(chan struct{}), make(chan struct{})
	shot := launcher.InstanceScreenshot{File: "large.png"}
	key := overviewShotKey(launcher.Instance{Root: launcher.Root{ID: "r", SelectedVersion: "v"}}, shot)
	a.localIcon(key, func() ([]byte, error) {
		close(started)
		select {
		case <-resume:
		case <-ctx.Done():
		}
		return nil, fmt.Errorf("test image")
	})
	<-started
	// The service is deliberately nil: a duplicate preload would reach the
	// disk service instead of observing the widget's in-flight request.
	a.prepareOverviewImages(ctx, "r", "v", launcher.InstanceStatus{LatestShot: &shot})
	if got := a.localIcon(key, func() ([]byte, error) {
		t.Error("the widget started a second decoder")
		return nil, nil
	}); got != nil {
		t.Fatal("unfinished image was published")
	}
	close(resume)
	a.bgWG.Wait()
	if len(a.imagePending) != 0 {
		t.Fatal("completed image retained a pending entry")
	}
}

func TestCancelledImageReleasesPendingRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{ctx: ctx, events: make(chan func(), 1), firstFrame: make(chan struct{}), imagePending: map[string]bool{}}
	a.localIcon("cancelled", func() ([]byte, error) {
		t.Error("cancelled request performed disk work")
		return nil, nil
	})
	cancel()
	a.bgWG.Wait()
	if len(a.imagePending) != 0 || len(a.imageRetry) != 0 {
		t.Fatal("cancelled image retained state that prevents a retry")
	}
}
