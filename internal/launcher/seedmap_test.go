package launcher

import (
	"bytes"
	"container/list"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"reflect"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func testSeedMapSampler(t *testing.T, snapshot *WorldgenSnapshot, seed int64) *LocalSampler {
	t.Helper()
	s, err := NewLocalSampler(context.Background(), snapshot, seed)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSeedMapJSONBiomesAndOpaqueGenerator(t *testing.T) {
	snapshot := &WorldgenSnapshot{Documents: []WorldgenDocument{
		{ID: "pack:fixed", Kind: "dimension", Data: json.RawMessage(`{"generator":{"type":"minecraft:noise","settings":"pack:noise","biome_source":{"type":"minecraft:fixed","biome":"pack:red"}}}`)},
		{ID: "pack:checker", Kind: "dimension", Data: json.RawMessage(`{"generator":{"type":"minecraft:noise","biome_source":{"type":"minecraft:checkerboard","scale":0,"biomes":["pack:red","pack:green"]}}}`)},
		{ID: "pack:opaque", Kind: "dimension", Data: json.RawMessage(`{"generator":{"type":"pack:java_chunk_generator","biome_source":{"type":"minecraft:fixed","biome":"pack:red"}}}`)},
		{ID: "pack:noise", Kind: "dimension", Data: json.RawMessage(`{"generator":{"type":"minecraft:noise","biome_source":{"type":"minecraft:multi_noise","preset":"minecraft:overworld"}}}`)},
		{ID: "pack:red", Kind: "biome", Data: json.RawMessage(`{"effects":{"grass_color":16711680}}`)},
		{ID: "pack:green", Kind: "biome", Data: json.RawMessage(`{"effects":{"grass_color":65280}}`)},
	}}
	s := testSeedMapSampler(t, snapshot, 42)
	for _, tc := range []struct {
		dimension string
		x, z      int
		want      color.RGBA
	}{
		{"pack:fixed", 8, 8, color.RGBA{255, 0, 0, 255}},
		{"pack:checker", 8, 8, color.RGBA{255, 0, 0, 255}},
		{"pack:checker", 24, 8, color.RGBA{0, 255, 0, 255}},
		{"pack:checker", -1, 8, color.RGBA{0, 255, 0, 255}},
		{"pack:checker", -1, -1, color.RGBA{255, 0, 0, 255}},
	} {
		key := SeedMapTileKey{X: seedMapFloor(tc.x, 128), Z: seedMapFloor(tc.z, 128), BlocksPerPixel: 1, Dimension: tc.dimension}
		tile, err := s.Sample(context.Background(), key)
		if err != nil {
			t.Fatal(err)
		}
		if got := tile.Image.RGBAAt(tc.x-key.X*128, tc.z-key.Z*128); got != tc.want {
			t.Fatalf("%s (%d,%d): %v want %v", tc.dimension, tc.x, tc.z, got, tc.want)
		}
	}
	if c := s.Coverage("pack:opaque"); !c.Opaque || c.Terrain != "candidates" {
		t.Fatalf("opaque generator got invented terrain: %+v", c)
	}
	if c := s.Coverage("pack:noise"); c.Terrain != "candidates" {
		t.Fatalf("noise incorrectly supported: %+v", c)
	}
	snapshot.SavedWorld = true
	saved := testSeedMapSampler(t, snapshot, 42)
	if saved.Coverage("pack:fixed").Terrain != "candidates" {
		t.Fatal("current JSON was mistaken for saved biome parameters")
	}
}

func TestSeedMapRandomSpreadReferenceVectors(t *testing.T) {
	linear := seedMapPlacement{spacing: 34, separation: 8, salt: 10387312}
	triangular := seedMapPlacement{spacing: 32, separation: 5, salt: 10387313, triangular: true}
	for _, tc := range []struct {
		seed               int64
		rx, rz             int
		linear, triangular image.Point
	}{
		{0, 0, 0, image.Pt(248, 40), image.Pt(344, 296)},
		{0, -1, -1, image.Pt(-136, -536), image.Pt(-472, -312)},
		{-1, -17, 31, image.Pt(-9160, 17112), image.Pt(-8456, 16296)},
		{-9223372036854775808, 22, -33, image.Pt(12360, -17656), image.Pt(11448, -16728)},
	} {
		for _, pair := range []struct {
			p    seedMapPlacement
			want image.Point
		}{{linear, tc.linear}, {triangular, tc.triangular}} {
			x, z := pair.p.candidate(tc.seed, tc.rx, tc.rz)
			if image.Pt(x, z) != pair.want {
				t.Fatalf("seed %d region %d,%d: %d,%d want %v", tc.seed, tc.rx, tc.rz, x, z, pair.want)
			}
		}
	}
}

func TestSeedMapCandidatesAndUnsupportedPlacements(t *testing.T) {
	snapshot := &WorldgenSnapshot{Documents: append(vanillaSeedMapDocuments("1.20.1"), WorldgenDocument{ID: "pack:towers", Kind: "structure_set", Source: "mod:pack.jar", Data: json.RawMessage(seedMapTowerJSON)})}
	a, b := testSeedMapSampler(t, snapshot, 0), testSeedMapSampler(t, snapshot, 1)
	key := SeedMapTileKey{X: -1, Z: -1, BlocksPerPixel: 8, Dimension: "minecraft:overworld"}
	overview, err := a.Sample(context.Background(), key)
	if err != nil || len(overview.Hits) != 0 {
		t.Fatalf("unfiltered candidates flooded an unavailable biome overview: %v", err)
	}
	key.Structure = "pack:towers"
	first, err := a.Sample(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	repeat, err := a.Sample(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	different, err := b.Sample(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Hits, repeat.Hits) || !bytes.Equal(first.Image.Pix, repeat.Image.Pix) || reflect.DeepEqual(first.Hits, different.Hits) {
		t.Fatal("sampling is not deterministic and seed-dependent")
	}
	modHit := false
	for _, hit := range first.Hits {
		if hit.X >= 0 || hit.X < -1024 || hit.Z >= 0 || hit.Z < -1024 || hit.Confidence != "candidate" {
			t.Fatalf("invalid candidate: %+v", hit)
		}
		if hit.ID == "pack:towers" {
			modHit = true
			if hit.Dimension != "" || hit.Source != "mod:pack.jar" {
				t.Fatalf("custom placement provenance: %+v", hit)
			}
		}
	}
	if !modHit {
		t.Fatal("mod JSON never affected the map")
	}
	for _, placement := range []string{
		`{"type":"minecraft:concentric_rings","distance":32,"count":128,"spread":3}`,
		`{"type":"minecraft:random_spread","spacing":10,"separation":10,"salt":1}`,
		`{"type":"minecraft:random_spread","spacing":10,"separation":2,"salt":1,"exclusion_zone":{}}`,
		`{"type":"mod:custom","spacing":10,"separation":2,"salt":1}`,
	} {
		doc := WorldgenDocument{Data: json.RawMessage(`{"structures":[{"structure":"pack:tower","weight":1}],"placement":` + placement + `}`)}
		if _, ok := parseSeedMapPlacement(doc, nil); ok {
			t.Fatalf("unsupported placement accepted: %s", placement)
		}
	}
	if len(vanillaSeedMapDocuments("26.1")) != 0 || len(vanillaSeedMapDocuments("1.20-pre1")) != 0 {
		t.Fatal("unverified Minecraft release used fallback")
	}
}

func awaitSeedMap[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("seed map worker timed out")
		var zero T
		return zero
	}
}

func TestSeedMapWorkerCancelsStaleViewportAndStops(t *testing.T) {
	started, canceled := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	tiles := newSeedMapTiles(context.Background(), func(ctx context.Context, key SeedMapTileKey) (*SeedMapTile, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-ctx.Done()
			close(canceled)
			return nil, ctx.Err()
		}
		return &SeedMapTile{Key: key, Image: image.NewRGBA(image.Rect(0, 0, 128, 128))}, nil
	})
	t.Cleanup(func() { tiles.Close(); awaitSeedMap(t, tiles.Done()) })
	frames := make(chan SeedMapFrame, 4)
	view := SeedMapViewport{X: 64, Z: 64, Width: 128, Height: 128, BlocksPerPixel: 1, Dimension: "minecraft:overworld"}
	if _, err := tiles.Request(view, func(f SeedMapFrame) { frames <- f }); err != nil {
		t.Fatal(err)
	}
	awaitSeedMap(t, started)
	view.X = -1000
	gen, err := tiles.Request(view, func(f SeedMapFrame) { frames <- f })
	if err != nil {
		t.Fatal(err)
	}
	awaitSeedMap(t, canceled)
	frame := awaitSeedMap(t, frames)
	if frame.Generation != gen || frame.Viewport.X != -1000 {
		t.Fatal("stale viewport published")
	}
	select {
	case <-frames:
		t.Fatal("obsolete frame was delivered")
	default:
	}
}

func TestSeedMapWorkerViewportBorderAndIdle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sampled := make(chan SeedMapTileKey, 100)
		tiles := newSeedMapTiles(context.Background(), func(_ context.Context, key SeedMapTileKey) (*SeedMapTile, error) {
			sampled <- key
			return &SeedMapTile{Key: key, Image: image.NewRGBA(image.Rect(0, 0, 128, 128))}, nil
		})
		t.Cleanup(func() { tiles.Close(); awaitSeedMap(t, tiles.Done()) })
		frames := make(chan SeedMapFrame, 2)
		view := SeedMapViewport{X: 64, Z: 64, Width: 128, Height: 128, BlocksPerPixel: 1, Dimension: "minecraft:overworld"}
		if _, err := tiles.Request(view, func(f SeedMapFrame) { frames <- f }); err != nil {
			t.Fatal(err)
		}
		awaitSeedMap(t, frames)
		seen := map[SeedMapTileKey]bool{}
		for range 9 {
			key := awaitSeedMap(t, sampled)
			if key.X < -1 || key.X > 1 || key.Z < -1 || key.Z > 1 {
				t.Fatalf("outside viewport ±1: %+v", key)
			}
			seen[key] = true
		}
		if len(seen) != 9 {
			t.Fatal("duplicate tile work")
		}
		// A sample notification precedes insertion into the LRU. Wait until the
		// worker is blocked on its request channel before checking cache reuse.
		synctest.Wait()
		if _, err := tiles.Request(view, func(f SeedMapFrame) { frames <- f }); err != nil {
			t.Fatal(err)
		}
		awaitSeedMap(t, frames)
		synctest.Wait()
		select {
		case <-sampled:
			t.Fatal("cache hit caused resampling")
		default:
		}
		for _, view := range []SeedMapViewport{{Width: 100000, Height: 100000, BlocksPerPixel: 1}, {Width: 1, Height: 1, BlocksPerPixel: 0}} {
			if _, err := tiles.Request(view, func(SeedMapFrame) {}); err == nil {
				t.Fatal("unbounded viewport accepted")
			}
		}
	})
}

func TestSeedMapTileLRUEvictsLeastRecentlyUsed(t *testing.T) {
	counts := map[SeedMapTileKey]int{}
	tiles := &SeedMapTiles{cache: map[SeedMapTileKey]*list.Element{}, lru: list.New(), sample: func(_ context.Context, key SeedMapTileKey) (*SeedMapTile, error) {
		counts[key]++
		return &SeedMapTile{Key: key}, nil
	}}
	key := func(x int) SeedMapTileKey {
		return SeedMapTileKey{X: x, BlocksPerPixel: 1, Dimension: "minecraft:overworld"}
	}
	get := func(x int) {
		t.Helper()
		if _, err := tiles.tile(context.Background(), key(x)); err != nil {
			t.Fatal(err)
		}
	}
	for x := range seedMapCacheTiles {
		get(x)
	}
	get(0)
	get(seedMapCacheTiles)
	if len(tiles.cache) != seedMapCacheTiles || tiles.cache[key(1)] != nil || tiles.cache[key(0)] == nil {
		t.Fatal("LRU did not preserve recently used tile")
	}
	get(1)
	if counts[key(1)] != 2 || counts[key(0)] != 1 {
		t.Fatal("evicted tile not resampled")
	}
}
