package launcher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestEndCubiomesReference(t *testing.T) {
	raw, err := os.ReadFile("../../cubiomes/testdata/end.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Seed        string
		X, Z, Biome int
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	snap, err := WorldgenIngest(context.Background(), WorldgenInput{Minecraft: "1.21.1"})
	if err != nil {
		t.Fatal(err)
	}
	names := map[int]string{9: "minecraft:the_end", 40: "minecraft:small_end_islands", 41: "minecraft:end_midlands", 42: "minecraft:end_highlands", 43: "minecraft:end_barrens"}
	seen := map[int]bool{}
	var s *LocalSampler
	last := ""
	for _, f := range fixtures {
		if f.Seed != last {
			seed, e := strconv.ParseInt(f.Seed, 10, 64)
			if e != nil {
				t.Fatal(e)
			}
			s = testSeedMapSampler(t, snap, seed)
			last = f.Seed
		}
		got, ok := s.BiomeAt("minecraft:the_end", f.X, 64, f.Z)
		if !ok || got != names[f.Biome] {
			t.Fatalf("End seed %s (%d,%d): %s want %s", f.Seed, f.X, f.Z, got, names[f.Biome])
		}
		seen[f.Biome] = true
	}
	if len(seen) != 5 {
		t.Fatalf("fixtures miss End biomes: %v", seen)
	}
	// Vanilla's positive-overflow corner has radial height 100-sqrt(2)*8,
	// clamped to 80. Cubiomes' optimized mapEndBiome skips this quirk, so this
	// analytically derived case is deliberately separate from its fixtures.
	if got, ok := s.BiomeAt("minecraft:the_end", 1048576, 64, -1048576); !ok || got != "minecraft:end_highlands" {
		t.Fatalf("positive End overflow lost: %s", got)
	}
	// The End must evaluate an overridden erosion channel, including custom types.
	snap.Documents = append(snap.Documents, WorldgenDocument{Kind: "noise_settings", ID: "minecraft:end", Data: json.RawMessage(`{"noise_router":{"erosion":1}}`)})
	s = testSeedMapSampler(t, snap, 0)
	if got, ok := s.BiomeAt("minecraft:the_end", 8192, 64, 8192); !ok || got != "minecraft:end_highlands" {
		t.Fatalf("End override ignored: %s", got)
	}
	snap.Documents[len(snap.Documents)-1].Data = json.RawMessage(`{"noise_router":{"erosion":{"type":"pack:java"}}}`)
	if c := testSeedMapSampler(t, snap, 0).Coverage("minecraft:the_end"); !c.Opaque || c.Terrain != "candidates" {
		t.Fatalf("custom End pretended to render: %+v", c)
	}
}

func TestSeedMapNearbySearchBiomesAndCancellation(t *testing.T) {
	snap, err := WorldgenIngest(context.Background(), WorldgenInput{Minecraft: "1.21.1"})
	if err != nil {
		t.Fatal(err)
	}
	s := testSeedMapSampler(t, snap, 42)
	q := SeedMapSearch{Dimension: "minecraft:overworld", Structure: "minecraft:villages", X: -1234, Y: 64, Z: -2345, Radius: 4096, Limit: 12}
	result, err := s.FindStructures(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 12 || result.Limited {
		t.Fatalf("unexpected search: %+v", result)
	}
	prev := float64(-1)
	for _, h := range result.Hits {
		dist := math.Hypot(float64(h.X-q.X), float64(h.Z-q.Z))
		if h.ID != q.Structure || h.Confidence != "biome_candidate" || h.Biome == "" || dist < prev || dist > float64(q.Radius) {
			t.Fatalf("unordered/unverified result: %+v", h)
		}
		prev = dist
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = s.FindStructures(ctx, q); !errors.Is(err, context.Canceled) {
		t.Fatalf("search ignored cancellation: %v", err)
	}
	bad := q
	bad.Radius = 32769
	if _, err = s.FindStructures(context.Background(), bad); err == nil {
		t.Fatal("unbounded search")
	}
	bad = q
	bad.Structure = "minecraft:strongholds"
	if _, err = s.FindStructures(context.Background(), bad); err == nil {
		t.Fatal("unsupported concentric placement searched")
	}
	// A filter belongs in the tile identity, and a tile only exposes that set.
	tile, err := s.Sample(context.Background(), SeedMapTileKey{Dimension: q.Dimension, Structure: q.Structure, X: -1, Z: -1, Y: 64, BlocksPerPixel: 16})
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range tile.Hits {
		if h.ID != q.Structure {
			t.Fatalf("filter leaked %s", h.ID)
		}
	}
	keys, _ := seedMapTileKeys(SeedMapViewport{Dimension: q.Dimension, Structure: q.Structure, Width: 1, Height: 1, BlocksPerPixel: 4})
	if keys[0].Structure != q.Structure {
		t.Fatal("cache key lost structure filter")
	}
	// Search and sampling share one worker. Replacing a blocked search must
	// cancel it before rendering, and it must never publish its stale results.
	tiles := newSeedMapTiles(context.Background(), s.Sample)
	defer tiles.Close()
	started := make(chan struct{})
	stopped := make(chan struct{})
	tiles.find = func(ctx context.Context, _ SeedMapSearch) (SeedMapSearchResult, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return SeedMapSearchResult{}, ctx.Err()
	}
	stale := make(chan struct{}, 1)
	if _, err = tiles.Search(q, func(SeedMapSearchResult, error) { stale <- struct{}{} }); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("search never started")
	}
	frames := make(chan SeedMapFrame, 1)
	if _, err = tiles.Request(SeedMapViewport{Dimension: q.Dimension, Width: 16, Height: 16, BlocksPerPixel: 4}, func(f SeedMapFrame) { frames <- f }); err != nil {
		t.Fatal(err)
	}
	select {
	case <-frames:
	case <-time.After(3 * time.Second):
		t.Fatal("search blocked rendering")
	}
	<-stopped
	select {
	case <-stale:
		t.Fatal("canceled search published")
	default:
	}
}

func TestSeedMapSavedSpawnNBTAndLanguageAssets(t *testing.T) {
	data := map[string]any{"Data": map[string]any{"SpawnX": int32(0), "SpawnY": int32(81), "SpawnZ": int32(-257), "Player": map[string]any{"SpawnX": int32(-12), "SpawnY": int32(64), "SpawnZ": int32(0), "SpawnDimension": "minecraft:the_nether"}, "WorldGenSettings": map[string]any{"seed": int64(-1)}}}
	var b bytes.Buffer
	writeSeedMapTestNBT(t, &b, "", data)
	level, err := ParseLevelDat(bytes.NewReader(compressedNBT(t, b.Bytes())))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(level.WorldSpawn, &SeedMapSpawn{X: 0, Y: 81, Z: -257, Dimension: "minecraft:overworld", Kind: "world"}) || !reflect.DeepEqual(level.PlayerSpawn, &SeedMapSpawn{X: -12, Y: 64, Z: 0, Dimension: "minecraft:the_nether", Kind: "player"}) {
		t.Fatalf("spawn lost zero/negative axes or dimension: %+v %+v", level.WorldSpawn, level.PlayerSpawn)
	}
	delete(data["Data"].(map[string]any), "SpawnY")
	b.Reset()
	writeSeedMapTestNBT(t, &b, "", data)
	level, err = ParseLevelDat(bytes.NewReader(compressedNBT(t, b.Bytes())))
	if err != nil || level.WorldSpawn != nil {
		t.Fatalf("partial spawn accepted: %+v %v", level.WorldSpawn, err)
	}
	dir := t.TempDir()
	mod, pack := filepath.Join(dir, "labels.jar"), filepath.Join(dir, "override")
	writeTestFile(t, mod, zipBytes(t, map[string][]byte{"assets/pack/lang/en_us.json": []byte(`{"structure.pack.tower":"Tower","biome.pack.cold":"Frost","item.pack.ignored":"Drop me"}`)}))
	writeTestFile(t, filepath.Join(pack, "assets/pack/lang/en_us.json"), []byte(`{"structure.pack.tower":"Overridden tower","dimension.pack.moon":"Moon"}`))
	snap, err := WorldgenIngest(context.Background(), WorldgenInput{Minecraft: "1.21.1", Sources: []WorldgenSource{{mod, "mod:labels"}, {pack, "pack:labels"}}})
	if err != nil {
		t.Fatal(err)
	}
	var labels map[string]string
	for _, d := range snap.Documents {
		if d.Kind == "language" && d.ID == "pack:en_us" {
			if err = json.Unmarshal(d.Data, &labels); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(labels) != 3 || labels["structure.pack.tower"] != "Overridden tower" || labels["biome.pack.cold"] != "Frost" || labels["dimension.pack.moon"] != "Moon" {
		t.Fatalf("language priority or directory pruning: %v", labels)
	}
}

func TestSeedMapSpawnEstimateAndLightweightSaves(t *testing.T) {
	ctx := context.Background()
	snap, err := WorldgenIngest(ctx, WorldgenInput{Minecraft: "1.21.1"})
	if err != nil {
		t.Fatal(err)
	}
	s := testSeedMapSampler(t, snap, 0)
	spawn, err := s.EstimateSpawn(ctx)
	if err != nil || spawn == nil || spawn.Kind != "estimate" || spawn.Dimension != "minecraft:overworld" {
		t.Fatalf("spawn estimate: %+v %v", spawn, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = s.EstimateSpawn(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("spawn search ignored cancellation: %v", err)
	}
	service, root, world := worldFixture(t)
	saves, err := service.SeedMapSaves(ctx, root.ID, "")
	if err != nil || len(saves) != 1 || saves[0].Name != "World One" {
		t.Fatalf("save picker: %v %v", saves, err)
	}
	// Empty saves is a normal manual-preview state; io.EOF is not an error.
	if err = os.Rename(world, filepath.Join(filepath.Dir(filepath.Dir(world)), "moved-save")); err != nil {
		t.Fatal(err)
	}
	saves, err = service.SeedMapSaves(ctx, root.ID, "")
	if err != nil || len(saves) != 0 {
		t.Fatalf("empty saves: %v %v", saves, err)
	}
}

func TestSeedMapDenseSearchStartsAtCenter(t *testing.T) {
	// A naive top-left scan hits its budget far from the requested center.
	s := testSeedMapSampler(t, &WorldgenSnapshot{Documents: []WorldgenDocument{
		{Kind: "dimension", ID: "pack:flat", Data: json.RawMessage(`{"generator":{"type":"minecraft:noise","biome_source":{"type":"minecraft:fixed","biome":"pack:red"}}}`)},
		{Kind: "structure_set", ID: "pack:dense", Data: json.RawMessage(`{"structures":[{"structure":"pack:tower","weight":1}],"placement":{"type":"minecraft:random_spread","spacing":1,"separation":0,"salt":1}}`)},
	}}, 0)
	result, err := s.FindStructures(context.Background(), SeedMapSearch{Dimension: "pack:flat", Structure: "pack:dense", Y: 64, Radius: 32768, Limit: 4})
	if err != nil || result.Limited || len(result.Hits) != 4 {
		t.Fatalf("dense search: %+v %v", result, err)
	}
	for _, h := range result.Hits {
		if h.X*h.X+h.Z*h.Z != 128 {
			t.Fatalf("search missed nearest chunk centers: %+v", h)
		}
	}
}
