package nativeui

import (
	"context"
	"image"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	imageCacheBudget = 16 << 20
	imageCacheLimit  = 256
	imageRetryLimit  = 512
)

type imageUse struct {
	serial   uint64
	edge     int // requested decode size; zero is an unsized/vector source
	data     *imageResource
	discover bool // last used by Discover; shared artwork can change ownership
}

// Paths, jar hashes and URLs are aliases. Different mod versions often contain
// exactly the same artwork, so the encoded contents and decode size own pixels.
type imageContentKey struct {
	sum  [32]byte
	edge int
}

type imageResource struct {
	source  ui.ImageSource
	bytes   int
	refs    int
	content imageContentKey
	indexed bool
}

// A navigation cancels unfinished artwork, not the decoded cache. Returning to
// a page reuses its pixels without restarting offscreen disk/network work.
func (a *App) resetImageRequests() {
	a.imageMu.Lock()
	defer a.imageMu.Unlock()
	if a.imageCancel != nil {
		a.imageCancel()
	}
	a.imageScope, a.imageCancel = nil, nil
}

func (a *App) imageContextLocked() context.Context {
	if a.imageScope == nil {
		a.imageScope, a.imageCancel = context.WithCancel(a.ctx)
	}
	return a.imageScope
}

func (a *App) imagePendingLocked(key string) bool {
	ctx := a.imageRequests[key]
	return a.imagePending[key] && (ctx == nil || ctx.Err() == nil)
}

func (a *App) reserveImageLocked(key string, ctx context.Context) {
	if a.imageRequests == nil {
		a.imageRequests = make(map[string]context.Context)
	}
	a.imagePending[key], a.imageRequests[key] = true, ctx
}

func (a *App) clearImagePending(key string, ctx context.Context) {
	a.imageMu.Lock()
	// A returning page may already have replaced the cancelled request.
	if a.imageRequests[key] == ctx {
		delete(a.imagePending, key)
		delete(a.imageRequests, key)
	}
	// A decoder can be borrowing a shared source before publishing its alias.
	// Only the final completion schedules retirement, back on the UI thread.
	release := len(a.imageRequests) == 0 && len(a.retiredImages) > 0 && !a.imageReleasePosted
	if release {
		a.imageReleasePosted = true
	}
	a.imageMu.Unlock()
	if release {
		a.post(func() {
			a.imageMu.Lock()
			a.imageReleasePosted = false
			released := a.releaseRetiredImagesLocked()
			a.imageMu.Unlock()
			if released {
				a.cacheRelease.Add(1)
			}
		})
	}
}

// MyGo's layout scratch can retain an old Bitmap after its element disappears.
// Drop its pixel storage through the public API once no alias or decoder needs
// it. Call only on the UI thread with imageMu held; active frames keep their
// shared images, and pending decoders finish before this mutates a source.
func (a *App) releaseRetiredImagesLocked() bool {
	if len(a.retiredImages) == 0 || len(a.imageRequests) != 0 {
		return false
	}
	for _, source := range a.images {
		if bitmap, ok := source.(*ui.Bitmap); ok {
			delete(a.retiredImages, bitmap)
		}
	}
	released := len(a.retiredImages) > 0
	for bitmap := range a.retiredImages {
		// A different size also drops MyGo's cached mip levels.
		bitmap.Update(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	}
	a.retiredImages = nil
	return released
}

// Image sources can keep both their full pixels and smaller render levels.
// Bound the shared cache by estimated bytes as well as entries, retaining the
// visible artwork on access. All helpers below require imageMu.
func (a *App) cachedImageLocked(key string) ui.ImageSource {
	return a.cachedPageImageLocked(key, false)
}

func (a *App) cachedPageImageLocked(key string, discover bool) ui.ImageSource {
	source := a.images[key]
	if source != nil {
		a.imageSerial++
		if a.imageUses == nil {
			a.imageUses = make(map[string]imageUse)
		}
		use := a.imageUses[key]
		use.serial = a.imageSerial
		use.discover = discover
		a.imageUses[key] = use
	}
	return source
}

func (a *App) cacheImageLocked(key string, source ui.ImageSource, encodedBytes int, decodeEdge ...int) {
	if source == nil {
		return
	}
	edge := 0
	if len(decodeEdge) > 0 {
		edge = decodeEdge[0]
	}
	a.cacheImageResourceLocked(key, &imageResource{source: source, bytes: imageCost(source, encodedBytes)}, edge)
}

func (a *App) cacheImageContentLocked(key string, source ui.ImageSource, encodedBytes int, content imageContentKey) {
	if source == nil {
		return
	}
	if a.imageContents == nil {
		a.imageContents = make(map[imageContentKey]*imageResource)
	}
	data := a.imageContents[content]
	if data == nil {
		data = &imageResource{source: source, bytes: imageCost(source, encodedBytes), content: content, indexed: true}
		a.imageContents[content] = data
	}
	// A simultaneous decode can finish before the first worker publishes.
	// Use its already-published resource, allowing the redundant pixels to die.
	a.cacheImageResourceLocked(key, data, content.edge)
}

func imageCost(source ui.ImageSource, encodedBytes int) int {
	// SVGs retain parsed paths instead of pixels; reserve space proportional
	// to their bounded source, with room for the renderer's derived geometry.
	cost := max(64<<10, encodedBytes*16)
	if bitmap, ok := source.(*ui.Bitmap); ok {
		w, h := bitmap.Size()
		cost = 0
		for {
			cost += 4 * w * h
			if w <= 1 && h <= 1 {
				break
			}
			w, h = max(1, (w+1)/2), max(1, (h+1)/2)
		}
	}
	return cost
}

func (a *App) cacheImageResourceLocked(key string, data *imageResource, edge int) {
	if a.images == nil {
		a.images = make(map[string]ui.ImageSource)
	}
	if a.imageUses == nil {
		a.imageUses = make(map[string]imageUse)
	}
	if data.refs == 0 {
		a.imageBytes += data.bytes
	}
	data.refs++ // pin before replacing an alias to this same resource
	a.deleteImageLocked(key)
	a.imageSerial++
	a.images[key] = data.source
	a.imageUses[key] = imageUse{serial: a.imageSerial, data: data}
	if _, bitmap := data.source.(*ui.Bitmap); bitmap {
		use := a.imageUses[key]
		use.edge = edge
		a.imageUses[key] = use
	}
	delete(a.imageRetry, key)
	for len(a.images) > imageCacheLimit || a.imageBytes > imageCacheBudget {
		oldest := key
		for candidate, use := range a.imageUses {
			if use.serial < a.imageUses[oldest].serial {
				oldest = candidate
			}
		}
		a.deleteImageLocked(oldest)
	}
}

func (a *App) deleteImageLocked(key string) {
	if data := a.imageUses[key].data; data != nil {
		data.refs--
		if data.refs == 0 {
			a.imageBytes -= data.bytes
			if data.indexed {
				delete(a.imageContents, data.content)
			}
		}
	}
	delete(a.images, key)
	delete(a.imageUses, key)
}

func (a *App) failImageLocked(key string, retry time.Duration) {
	if a.imageRetry == nil {
		a.imageRetry = make(map[string]time.Time)
	}
	a.imageRetry[key] = time.Now().Add(retry)
	for len(a.imageRetry) > imageRetryLimit {
		oldest := key
		for candidate, until := range a.imageRetry {
			if candidate != key && (oldest == key || until.Before(a.imageRetry[oldest])) {
				oldest = candidate
			}
		}
		delete(a.imageRetry, oldest)
	}
}
