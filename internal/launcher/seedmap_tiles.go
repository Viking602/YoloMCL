package launcher

import (
	"cmp"
	"container/list"
	"context"
	"errors"
	"image"
	"image/draw"
	"slices"
	"sync"
)

// Raster limits allow large windows to render without stretching a small map.
// The cache also holds the largest visible viewport and its prefetch border.
const (
	SeedMapMaxWidth   = 1536
	SeedMapMaxHeight  = 1024
	seedMapCacheTiles = 192
)

// SeedMapViewport is a bounded raster centered on block coordinates X, Z.
type SeedMapViewport struct {
	X, Z, Width, Height, BlocksPerPixel int
	Y                                   int
	Dimension                           string
	Structure                           string
	Surface                             bool
	ContourOverlay                      bool
	Terrain                             bool // world-map surface colours (surface view only)
}

// Origin is the world position of the first displayed sample. Align it to the
// tile lattice, including odd viewport sizes and negative centers.
func (v SeedMapViewport) Origin() (int, int) {
	step := max(1, v.BlocksPerPixel)
	x, z := v.X-v.Width*step/2, v.Z-v.Height*step/2
	return -seedMapFloor(-x, step) * step, -seedMapFloor(-z, step) * step
}

// SeedMapFrame contains only the visible raster and its placement candidates.
type SeedMapFrame struct {
	Viewport   SeedMapViewport
	Generation uint64
	Image      *image.RGBA
	Hits       []FeatureHit
	Limited    bool
	Biomes     []SeedMapBiome
	Err        error
	Partial    bool
	tiles      []*SeedMapTile // immutable tiles also serve point inspection
}

// Tiles returns the completed tiles in this frame. The slice, tiles and their
// images are immutable and can be retained after the next delivery.
func (f SeedMapFrame) Tiles() []*SeedMapTile { return f.tiles }

type seedMapRequest struct {
	ctx         context.Context
	viewport    SeedMapViewport
	generation  uint64
	deliver     func(SeedMapFrame)
	search      *SeedMapSearch
	found       func(SeedMapSearchResult, error)
	progressive bool
	tiled       bool
}

// SeedMapTiles owns one worker and a 192-tile LRU (12 MiB of raster pixels).
// Request replaces pending work and cancels stale sampling. No timers, polling,
// JVM startup, network calls, region reads, or goroutines per tile are used.
// Callbacks run on the worker;
// UI clients must post them to the UI thread and check their generation there.
type SeedMapTiles struct {
	ctx        context.Context
	stop       context.CancelFunc
	done       chan struct{}
	requests   chan seedMapRequest
	mu         sync.Mutex
	cancel     context.CancelFunc
	generation uint64
	sample     func(context.Context, SeedMapTileKey) (*SeedMapTile, error)
	find       func(context.Context, SeedMapSearch) (SeedMapSearchResult, error)
	cache      map[SeedMapTileKey]*list.Element
	lru        *list.List
}

// NewSeedMapTiles starts a single blocking worker for this immutable sampler.
func NewSeedMapTiles(ctx context.Context, sampler *LocalSampler) *SeedMapTiles {
	t := newSeedMapTiles(ctx, sampler.Sample)
	t.find = sampler.FindStructures
	return t
}

func newSeedMapTiles(ctx context.Context, sample func(context.Context, SeedMapTileKey) (*SeedMapTile, error)) *SeedMapTiles {
	ctx, stop := context.WithCancel(ctx)
	t := &SeedMapTiles{ctx: ctx, stop: stop, done: make(chan struct{}), requests: make(chan seedMapRequest, 1), sample: sample, cache: map[SeedMapTileKey]*list.Element{}, lru: list.New()}
	go t.run()
	return t
}

// Request queues only the newest viewport, returning its generation. Width and
// height are capped to bound CPU, UI upload, cache memory and tile count.
func (t *SeedMapTiles) Request(view SeedMapViewport, deliver func(SeedMapFrame)) (uint64, error) {
	return t.request(view, deliver, false, false)
}

// RequestProgressive publishes immutable partial frames while uncached tiles
// are generated, followed by one complete frame. Partial images never change
// after delivery, and cancellation suppresses all remaining frames.
func (t *SeedMapTiles) RequestProgressive(view SeedMapViewport, deliver func(SeedMapFrame)) (uint64, error) {
	return t.request(view, deliver, true, false)
}

// RequestTiles publishes immutable tile references without allocating or copying
// a composite Image. Renderers can retain each tile's texture across camera moves.
func (t *SeedMapTiles) RequestTiles(view SeedMapViewport, deliver func(SeedMapFrame)) (uint64, error) {
	return t.request(view, deliver, true, true)
}

func (t *SeedMapTiles) request(view SeedMapViewport, deliver func(SeedMapFrame), progressive, tiled bool) (uint64, error) {
	if view.Width < 1 || view.Height < 1 || view.Width > SeedMapMaxWidth || view.Height > SeedMapMaxHeight || view.BlocksPerPixel < 1 || view.BlocksPerPixel > 32 || view.X < -29900000 || view.X > 29900000 || view.Z < -29900000 || view.Z > 29900000 || view.Y < -2048 || view.Y > 2047 || deliver == nil {
		return 0, errors.New("invalid seed map viewport")
	}
	return t.enqueue(seedMapRequest{viewport: view, deliver: deliver, progressive: progressive, tiled: tiled})
}

// Search shares the tile worker: querying never creates another worker, and
// cancels obsolete border prefetch. A new viewport cancels an old search too.
func (t *SeedMapTiles) Search(query SeedMapSearch, found func(SeedMapSearchResult, error)) (uint64, error) {
	if t.find == nil || found == nil {
		return 0, errors.New("invalid seed map search")
	}
	return t.enqueue(seedMapRequest{search: &query, found: found})
}

func (t *SeedMapTiles) enqueue(req seedMapRequest) (uint64, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.ctx.Err(); err != nil {
		return 0, err
	}
	if t.cancel != nil {
		t.cancel()
	}
	ctx, cancel := context.WithCancel(t.ctx)
	t.cancel = cancel
	t.generation++
	req.ctx, req.generation = ctx, t.generation
	select {
	case <-t.requests:
	default:
	}
	t.requests <- req
	return req.generation, nil
}

// Close cancels work; Done closes when the worker has exited and released its cache.
func (t *SeedMapTiles) Close() { t.stop() }

// Done allows callers to join the worker without polling.
func (t *SeedMapTiles) Done() <-chan struct{} { return t.done }

func seedMapTileKeys(view SeedMapViewport) (visible, border []SeedMapTileKey) {
	span := SeedMapTilePixels * view.BlocksPerPixel
	x, z := view.Origin()
	minX, maxX := seedMapFloor(x, span), seedMapFloor(x+view.Width*view.BlocksPerPixel-1, span)
	minZ, maxZ := seedMapFloor(z, span), seedMapFloor(z+view.Height*view.BlocksPerPixel-1, span)
	for tz := minZ - 1; tz <= maxZ+1; tz++ {
		for tx := minX - 1; tx <= maxX+1; tx++ {
			key := SeedMapTileKey{X: tx, Y: view.Y, Z: tz, BlocksPerPixel: view.BlocksPerPixel, Dimension: view.Dimension, Structure: view.Structure, Surface: view.Surface, ContourOverlay: view.ContourOverlay, Terrain: view.Terrain && view.Surface}
			if tx >= minX && tx <= maxX && tz >= minZ && tz <= maxZ {
				visible = append(visible, key)
			} else {
				border = append(border, key)
			}
		}
	}
	return
}

func (t *SeedMapTiles) tile(ctx context.Context, key SeedMapTileKey) (*SeedMapTile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if e := t.cache[key]; e != nil {
		t.lru.MoveToFront(e)
		return e.Value.(*SeedMapTile), nil
	}
	tile, err := t.sample(ctx, key)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	t.cache[key] = t.lru.PushFront(tile)
	if t.lru.Len() > seedMapCacheTiles {
		e := t.lru.Back()
		delete(t.cache, e.Value.(*SeedMapTile).Key)
		t.lru.Remove(e)
	}
	return tile, nil
}

func (t *SeedMapTiles) run() {
	defer close(t.done)
	defer func() { clear(t.cache); t.lru.Init() }()
	for {
		select {
		case <-t.ctx.Done():
			return
		case req := <-t.requests:
			if req.search != nil {
				result, err := t.find(req.ctx, *req.search)
				if req.ctx.Err() == nil {
					req.found(result, err)
				}
			} else {
				t.render(req)
			}
		}
	}
}

func (t *SeedMapTiles) render(req seedMapRequest) {
	visible, border := seedMapTileKeys(req.viewport)
	frame := SeedMapFrame{Viewport: req.viewport, Generation: req.generation}
	if !req.tiled {
		frame.Image = image.NewRGBA(image.Rect(0, 0, req.viewport.Width, req.viewport.Height))
	}
	v := req.viewport
	x, z := v.Origin()
	span := SeedMapTilePixels * v.BlocksPerPixel
	if req.progressive {
		distance := func(key SeedMapTileKey) int64 {
			dx, dz := int64(key.X*span+span/2-v.X), int64(key.Z*span+span/2-v.Z)
			return dx*dx + dz*dz
		}
		slices.SortStableFunc(visible, func(a, b SeedMapTileKey) int { return cmp.Compare(distance(a), distance(b)) })
	}
	biomes := map[string]bool{}
	newTiles := 0
	// Bound full-frame copies for large canvases while keeping first-tile
	// delivery immediate. At most sixteen intermediate frames per request.
	partialEvery := max(4, (len(visible)+14)/15)
	for index, key := range visible {
		cached := t.cache[key] != nil
		tile, err := t.tile(req.ctx, key)
		if err != nil {
			if req.ctx.Err() == nil {
				frame.Err = err
				req.deliver(frame)
			}
			return
		}
		left, top := seedMapFloor(key.X*span-x, v.BlocksPerPixel), seedMapFloor(key.Z*span-z, v.BlocksPerPixel)
		if frame.Image != nil {
			draw.Draw(frame.Image, image.Rect(left, top, left+SeedMapTilePixels, top+SeedMapTilePixels), tile.Image, image.Point{}, draw.Src)
		}
		frame.tiles = append(frame.tiles, tile)
		frame.Limited = frame.Limited || tile.Limited
		for _, biome := range tile.Biomes {
			if !biomes[biome.ID] {
				biomes[biome.ID] = true
				frame.Biomes = append(frame.Biomes, biome)
			}
		}
		for _, hit := range tile.Hits {
			if hit.X >= x && hit.X < x+v.Width*v.BlocksPerPixel && hit.Z >= z && hit.Z < z+v.Height*v.BlocksPerPixel {
				frame.Hits = append(frame.Hits, hit)
			}
		}
		if !cached {
			newTiles++
		}
		if req.progressive && !cached && (newTiles == 1 || newTiles%partialEvery == 0) && index+1 < len(visible) && req.ctx.Err() == nil {
			partial := frame
			partial.Partial = true
			if frame.Image != nil {
				partial.Image = image.NewRGBA(frame.Image.Rect)
				copy(partial.Image.Pix, frame.Image.Pix)
			}
			partial.Biomes = append([]SeedMapBiome(nil), frame.Biomes...)
			partial.Hits = nil // the UI overlays markers after the frame completes
			partial.tiles = append([]*SeedMapTile(nil), frame.tiles...)
			req.deliver(partial)
		}
	}
	if req.ctx.Err() != nil {
		return
	}
	if len(frame.Hits) > 2048 {
		frame.Hits = spreadSeedMapHits(frame.Hits, 2048)
		frame.Limited = true
	}
	req.deliver(frame)
	// Only viewport ± one tile. Prefetch never delays the visible frame.
	for _, key := range border {
		if _, err := t.tile(req.ctx, key); err != nil {
			return
		}
	}
}

// Bound the completed viewport, not the first center-out tiles. Sorting by
// world position makes the sample independent of progressive/cache order and
// keeps dense maps from losing all markers along their outer edges.
func spreadSeedMapHits(hits []FeatureHit, limit int) []FeatureHit {
	if len(hits) <= limit {
		return hits
	}
	slices.SortFunc(hits, func(a, b FeatureHit) int {
		return cmp.Or(cmp.Compare(a.Z, b.Z), cmp.Compare(a.X, b.X), cmp.Compare(a.ID, b.ID))
	})
	result := make([]FeatureHit, limit)
	for i := range result {
		result[i] = hits[(2*i+1)*len(hits)/(2*limit)]
	}
	return result
}
