package nativeui

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestSeedMapNativeFlowAndLifecycle(t *testing.T) {
	a, tt, saves := worldsTester(t)
	a.OverrideLanguage(LangEN)
	tt.Frame()
	mapTab, ok := tt.Find(a.L("instance.tab_prefix") + " " + a.L("seedmap.open"))
	if !ok {
		t.Fatal("missing top-level seed map tab")
	}
	overviewTab, ok := tt.Find(a.L("instance.tab_prefix") + " " + a.L("instance.overview"))
	if !ok || mapTab.Y != overviewTab.Y || mapTab.X <= overviewTab.X {
		t.Fatal("seed map is not beside overview")
	}
	clickUI(t, tt, a.L("instance.tab_prefix")+" "+a.L("seedmap.open"))
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	tt.Advance(400 * time.Millisecond)
	v := a.seedMap
	if v.viewport.BlocksPerPixel != 1 {
		t.Fatal("map did not open at block detail")
	}
	// Use an overview for the structure-selection flow below.
	v.viewport.BlocksPerPixel = 4
	tt.Frame()
	waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport == v.viewport })
	if a.instTab != "seedmap" || v.seed != 0 || len(v.frame.Hits) == 0 || !tt.HasText(a.L("seedmap.surface_active")) {
		t.Fatalf("manual map not wired: %v", tt.Texts())
	}
	textures := make(map[launcher.SeedMapTileKey]*ui.Bitmap)
	for key, texture := range v.raster.textures {
		textures[key] = texture.bitmap
	}
	clickUI(t, tt, a.L("seedmap.y"))
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("-31")
	clickUI(t, tt, a.L("seedmap.y_apply"))
	waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport.Y == -31 })
	if v.yText != "-31" || v.raster.textures == nil {
		t.Fatal("exact Y slice did not publish display tiles")
	}
	clickUI(t, tt, a.L("seedmap.y"))
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("64")
	clickUI(t, tt, a.L("seedmap.y_apply"))
	waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport.Y == 64 })
	for key, texture := range v.raster.textures {
		if old := textures[key]; old != nil && texture.bitmap != old {
			t.Fatal("returning to a cached Y slice recreated its texture")
		}
	}
	var hit launcher.FeatureHit
	for _, candidate := range v.frame.Hits {
		if _, ok := tt.Find(a.seedMapHitLabel(candidate)); ok {
			hit = candidate
			break
		}
	}
	if hit.ID == "" {
		t.Fatal("fixture has no individual visible structure marker")
	}
	clickUI(t, tt, a.seedMapHitLabel(hit))
	if v.selected == nil || *v.selected != hit {
		t.Fatal("marker did not expose its candidate provenance")
	}
	stage, ok := tt.Find(a.L("seedmap.canvas"))
	if !ok {
		t.Fatal("missing map canvas")
	}
	x, y := stage.X+stage.W/2, stage.Y+stage.H/2
	tt.Scroll(x, y, 0, -40)
	waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport.BlocksPerPixel == 2 })
	if !v.mapReady() || v.frame.Image != nil {
		t.Fatal("zoom did not use immutable tiles without a composite upload")
	}
	before := v.viewport
	// Use a corner free of markers so the map itself owns the drag.
	x, y = stage.X+20, stage.Y+20
	tt.Press(x, y)
	tt.Move(x+37, y+23)
	tt.Release(x+37, y+23)
	waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport == v.viewport })
	if v.viewport.X == before.X || v.viewport.Z == before.Z {
		t.Fatal("canvas drag did not pan")
	}
	clickUI(t, tt, a.L("seedmap.seed"))
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("-9223372036854775808")
	clickUI(t, tt, a.L("seedmap.apply"))
	waitUI(t, tt, func() bool { return !v.loading && !v.pending && v.mapReady() && v.seed == -9223372036854775808 })
	if v.seedText != "-9223372036854775808" {
		t.Fatal("seed lost 64-bit precision")
	}
	snapshotUI(t, tt, "seedmap-multi-noise")
	gen := v.generation
	clickUI(t, tt, a.L("seedmap.seed"))
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("9223372036854775808")
	clickUI(t, tt, a.L("seedmap.apply"))
	if v.generation != gen || v.err != a.L("seedmap.invalid_seed") {
		t.Fatal("overflowing seed accepted")
	}
	oldTiles := v.tiles
	clickUI(t, tt, a.L("instance.tab_prefix")+" "+a.L("storage.label.saves"))
	select {
	case <-oldTiles.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("closed map worker leaked")
	}
	tt.Frame()
	if a.seedMap != nil {
		t.Fatal("late callback reopened closed map")
	}

	// A saved modern seed is read from level.dat, without any region files.
	seed := int64(-1234567890123456789)
	var raw bytes.Buffer
	str := func(s string) { raw.Write(binary.BigEndian.AppendUint16(nil, uint16(len(s)))); raw.WriteString(s) }
	raw.WriteByte(10)
	str("")
	raw.WriteByte(10)
	str("Data")
	raw.WriteByte(10)
	str("WorldGenSettings")
	raw.WriteByte(4)
	str("seed")
	raw.Write(binary.BigEndian.AppendUint64(nil, uint64(seed)))
	raw.WriteByte(0) // WorldGenSettings
	for _, axis := range []struct {
		name  string
		value int32
	}{{"SpawnX", 96}, {"SpawnY", 80}, {"SpawnZ", -192}} {
		raw.WriteByte(3)
		str(axis.name)
		raw.Write(binary.BigEndian.AppendUint32(nil, uint32(axis.value)))
	}
	raw.WriteByte(10)
	str("Player")
	for _, axis := range []struct {
		name  string
		value int32
	}{{"SpawnX", 32}, {"SpawnY", 64}, {"SpawnZ", -16}} {
		raw.WriteByte(3)
		str(axis.name)
		raw.Write(binary.BigEndian.AppendUint32(nil, uint32(axis.value)))
	}
	raw.WriteByte(8)
	str("SpawnDimension")
	str("minecraft:the_nether")
	raw.Write([]byte{0, 0, 0})
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	if _, err := gz.Write(raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saves, "world-one", "level.dat"), compressed.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	clickUI(t, tt, fmt.Sprintf(a.L("seedmap.open_world"), "world-one"))
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	if a.instTab != "seedmap" || a.seedMap.seed != seed {
		t.Fatalf("saved seed not loaded: %d", a.seedMap.seed)
	}
	v = a.seedMap
	if len(v.spawns()) != 2 || v.viewport.X != 96 || v.viewport.Z != -192 || !tt.HasText(a.L("seedmap.spawn.world")) {
		t.Fatalf("saved spawn not displayed: %v", v.spawns())
	}
	clickUI(t, tt, a.L("seedmap.spawn.player"))
	waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport.Dimension == "minecraft:the_nether" })
	if v.viewport.X != 32 || v.viewport.Z != -16 || v.selectedSpawn == nil || v.selectedSpawn.Kind != "player" {
		t.Fatal("respawn did not navigate across dimensions")
	}
	clickUI(t, tt, a.L("seedmap.seed"))
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("42")
	clickUI(t, tt, a.L("seedmap.apply"))
	waitUI(t, tt, func() bool { return !v.loading && !v.pending && v.seed == 42 })
	for _, sp := range v.spawns() {
		if sp.Kind != "estimate" {
			t.Fatal("old save spawn persisted after manual seed change")
		}
	}
	oldTiles = a.seedMap.tiles
	a.setInstTab("overview")
	tt.Frame()
	select {
	case <-oldTiles.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("navigation did not cancel sampling")
	}
	if a.seedMap != nil {
		t.Fatal("map retained after navigation")
	}
}

// Optional read-only reproduction from a user's effective registry snapshot.
// The fixture UI and launcher settings remain inside t.TempDir().
func TestSeedMapInstalledSnapshotPreview(t *testing.T) {
	path := os.Getenv("MCL_WORLDGEN_SNAPSHOT")
	if path == "" {
		t.Skip("MCL_WORLDGEN_SNAPSHOT points to an external registry snapshot")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot launcher.WorldgenSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	a, tt, _ := worldsTester(t)
	clickUI(t, tt, a.L("instance.tab_prefix")+" "+a.L("seedmap.open"))
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	v := a.seedMap
	v.snapshot = &snapshot
	if seed := os.Getenv("MCL_WORLDGEN_SEED"); seed != "" {
		v.seedText = seed
	}
	v.viewport.X, v.viewport.Z = 664, 280
	v.xText, v.zText = "664", "280"
	a.loadSeedMap(v, false)
	waitUI(t, tt, func() bool { return !v.loading && !v.pending && v.mapReady() })
	tt.Advance(400 * time.Millisecond)
	snapshotUI(t, tt, "seedmap-installed")
	coverage := v.sampler.Coverage(v.viewport.Dimension)
	if coverage.Opaque || coverage.Terrain == "candidates" || len(v.frame.Biomes) == 0 {
		t.Fatalf("installed snapshot still has no biome preview: %+v", coverage)
	}
	t.Logf("Minecraft %s, %d mods: %d visible biomes, %d candidates, %d marker groups", snapshot.Minecraft, len(snapshot.Mods), len(v.frame.Biomes), len(v.frame.Hits), len(v.markerLayout.markers))
	clickUI(t, tt, a.L("seedmap.hide_markers"))
	if !v.hideMarkers {
		t.Fatal("structure visibility control did not update")
	}
	snapshotUI(t, tt, "seedmap-installed-biomes")
	for range 3 {
		v.zoom(1)
	}
	tt.Frame()
	waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport == v.viewport })
	snapshotUI(t, tt, "seedmap-installed-blocks")
}

func TestSeedMapRefreshLoadsInstalledStructures(t *testing.T) {
	a, tt, saves := worldsTesterProfile(t, "1.21.1", "net.fabricmc.loader.impl.launch.knot.KnotClient")
	clickUI(t, tt, a.L("instance.tab_prefix")+" "+a.L("seedmap.open"))
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	tt.Advance(400 * time.Millisecond)
	v := a.seedMap
	mods := filepath.Join(filepath.Dir(saves), "mods")
	if err := os.MkdirAll(mods, 0755); err != nil {
		t.Fatal(err)
	}
	writeZip(t, filepath.Join(mods, "dynamic.jar"), map[string]string{
		"fabric.mod.json": `{"id":"dynamic_fixture","version":"1"}`,
		"data/a_dynamic/worldgen/structure_set/observatory.json": `{"structures":[{"structure":"a_dynamic:observatory","weight":1}],"placement":{"type":"minecraft:random_spread","spacing":16,"separation":4,"salt":1}}`,
		"data/a_dynamic/worldgen/structure/observatory.json":     `{"biomes":"minecraft:plains"}`,
		"assets/a_dynamic/lang/zh_cn.json":                       `{"structure.a_dynamic.observatory":"动态天文台"}`,
	})
	refresh := func() {
		t.Helper()
		clickUI(t, tt, a.L("seedmap.refresh"))
		waitUI(t, tt, func() bool { return !v.loading && !v.pending && v.mapReady() })
	}
	before := v.viewport
	refresh()
	if v.viewport != before || v.seed != 0 {
		t.Fatal("resource refresh changed the seed or viewport")
	}
	label := a.L("seedmap.structures") + " 动态天文台"
	clickUI(t, tt, label)
	waitUI(t, tt, func() bool { return !v.pending })
	if v.viewport.Structure != "a_dynamic:observatory" {
		t.Fatal("new installed structure did not become selectable")
	}
	for _, structure := range v.sampler.Structures("minecraft:the_nether") {
		if structure.ID == "a_dynamic:observatory" {
			t.Fatal("surface-only structure leaked into Nether")
		}
	}
	if err := a.service.ToggleMod(v.rootID, "dynamic.jar"); err != nil {
		t.Fatal(err)
	}
	refresh()
	if _, exists := tt.Find(label); exists || v.viewport.Structure != "" {
		t.Fatal("disabled structure or its stale filter remained visible")
	}
	if err := a.service.ToggleMod(v.rootID, "dynamic.jar.disabled"); err != nil {
		t.Fatal(err)
	}
	refresh()
	if _, exists := tt.Find(label); !exists {
		t.Fatal("re-enabled structure did not return to the list")
	}
	if err := os.Remove(filepath.Join(mods, "dynamic.jar")); err != nil {
		t.Fatal(err)
	}
	refresh()
	if _, exists := tt.Find(label); exists {
		t.Fatal("removed mod's structure remained visible")
	}
}

func TestSeedMapSearchDimensionsAndFullCanvas(t *testing.T) {
	a, tt, saves := worldsTester(t)
	modDimension := filepath.Join(filepath.Dir(saves), "datapacks/fixture/data/pack/dimension/hidden.json")
	if err := os.MkdirAll(filepath.Dir(modDimension), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modDimension, []byte(`{"generator":{"type":"pack:java"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	clickUI(t, tt, a.L("instance.tab_prefix")+" "+a.L("seedmap.open"))
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	v := a.seedMap
	if got := seedMapDimensions(v.sampler); len(got) != 4 || got[3] != "pack:hidden" {
		t.Fatalf("dimension picker should include custom dimensions after vanilla: %v", got)
	}
	tt.Advance(400 * time.Millisecond)
	for _, language := range []string{"zh-CN", LangEN} {
		a.OverrideLanguage(language)
		tt.Frame()
		clickUI(t, tt, a.L("seedmap.search"))
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type("村庄")
		tt.Frame()
		kind := a.L("seedmap.structures") + " " + a.seedMapName("structure", "minecraft:villages", a.isEN())
		if !tt.HasText("Village") || !tt.HasText("村庄") {
			t.Fatalf("bilingual labels missing: %v", tt.Texts())
		}
		clickUI(t, tt, kind)
		waitUI(t, tt, func() bool { return !v.pending && !v.searching && v.frame.Viewport.Structure == "minecraft:villages" })
		if len(v.results) == 0 {
			t.Fatal("choosing a structure lost its automatic nearby search during map refresh")
		}
		if v.selected == nil || v.viewport.X != v.selected.X || v.viewport.Z != v.selected.Z {
			t.Fatal("choosing a structure did not locate its nearest result")
		}
		// A radius choice refreshes the current structure without another
		// click on Find nearby. Opening the selector alone does no work.
		generation := v.searchGeneration
		radius := "8192"
		if v.radius == radius {
			radius = "4096"
		}
		clickUI(t, tt, a.L("seedmap.radius"))
		if v.searchGeneration != generation {
			t.Fatal("opening the radius selector launched a redundant search")
		}
		clickUI(t, tt, a.L("seedmap.radius")+" "+radius)
		if v.searchGeneration == generation || v.radius != radius {
			t.Fatal("changing the radius did not request a new nearby search")
		}
		waitUI(t, tt, func() bool { return !v.searching && len(v.results) > 0 })
		// A raster refresh can arrive before a queued search callback. The
		// same query must resume after it, and the cancelled callback is stale.
		a.searchSeedMap(v)
		a.startSeedMapSearch(v)
		a.requestSeedMap(v)
		waitUI(t, tt, func() bool { return !v.pending && !v.searching })
		if len(v.results) == 0 || v.searchQueued {
			t.Fatal("raster refresh lost the in-flight nearby search")
		}
		hit := v.results[0]
		resultLabel := a.L("seedmap.jump") + " " + fmt.Sprintf("X %d  Z %d", hit.X, hit.Z)
		waitUI(t, tt, func() bool { _, ok := tt.Find(resultLabel); return ok })
		clickUI(t, tt, resultLabel)
		waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport.X == hit.X && v.frame.Viewport.Z == hit.Z })
		if v.selected == nil || *v.selected != hit {
			t.Fatal("search result did not select the map marker")
		}
		snapshotUI(t, tt, "seedmap-search-"+language)
		for _, size := range [][2]int{{900, 600}, {1536, 1024}, {1920, 1080}} {
			tt.SetSize(size[0], size[1])
			tt.Frame()
			waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport == v.viewport })
			stage, ok := tt.Find(a.L("seedmap.canvas"))
			if !ok || stage.W < 140 || stage.H < 110 || stage.X < 0 || stage.Y < 42 || stage.X+stage.W > float32(size[0])+.5 || stage.Y+stage.H > float32(size[1])+.5 {
				t.Fatalf("map escaped viewport %v: %+v", size, stage)
			}
			if delta := math.Abs(float64(stage.W/stage.H - float32(v.frame.Viewport.Width)/float32(v.frame.Viewport.Height))); delta > .015 {
				t.Fatalf("raster letterboxes canvas %v: %+v / %+v", size, stage, v.frame.Viewport)
			}
			if size[0] == 900 {
				snapshotUI(t, tt, "seedmap-compact-"+language)
			}
		}
		clickUI(t, tt, a.L("seedmap.all_structures"))
		waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport.Structure == "" })
		if v.searching || v.searchQueued || v.jumpNearest || len(v.results) != 0 {
			t.Fatal("clearing the structure retained the previous search")
		}
	}
	for _, id := range []string{"minecraft:the_nether", "minecraft:the_end"} {
		a.seedMapDimension(v, id)
		tt.Frame()
		waitUI(t, tt, func() bool { return v.mapReady() && !v.pending && v.frame.Viewport.Dimension == id })
		if v.sampler.Coverage(id).Terrain == "candidates" {
			t.Fatalf("vanilla dimension did not render: %s", id)
		}
		snapshotUI(t, tt, "seedmap-"+id[len("minecraft:"):])
	}
}

func TestSeedMapSpawnSourceIdentity(t *testing.T) {
	v := &seedMapView{world: "save", seed: 1, level: launcher.LevelInfo{RandomSeed: 1, HasRandomSeed: true, WorldSpawn: &launcher.SeedMapSpawn{X: 0, Dimension: "minecraft:overworld", Kind: "world"}}, estimate: &launcher.SeedMapSpawn{X: 512, Dimension: "minecraft:overworld", Kind: "estimate"}}
	if spawns := v.spawns(); len(spawns) != 1 || spawns[0].Kind != "world" {
		t.Fatal("estimate replaced actual spawn")
	}
	v.seed = 2
	if spawns := v.spawns(); len(spawns) != 1 || spawns[0].Kind != "estimate" {
		t.Fatal("seed change retained saved coordinates")
	}
	v.seed = 1
	v.world = ""
	if spawns := v.spawns(); len(spawns) != 1 || spawns[0].Kind != "estimate" {
		t.Fatal("manual source retained saved coordinates")
	}
}

func TestSeedMapJSONPreviewAndOpaqueBadge(t *testing.T) {
	a, tt, saves := worldsTester(t)
	game := filepath.Dir(saves)
	file := filepath.Join(game, "datapacks", "fixture", "data", "minecraft", "dimension", "overworld.json")
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	definition := `{"generator":{"type":"minecraft:noise","biome_source":{"type":"minecraft:checkerboard","scale":2,"biomes":["pack:forest","pack:desert","pack:lake"]}}}`
	if err := os.WriteFile(file, []byte(definition), 0600); err != nil {
		t.Fatal(err)
	}
	clickUI(t, tt, a.L("instance.tab_prefix")+" "+a.L("seedmap.open"))
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	if !tt.HasText(a.L("seedmap.coverage.checkerboard")) {
		t.Fatalf("JSON not connected to UI: %v", tt.Texts())
	}
	snapshotUI(t, tt, "seedmap-json")
	clickUI(t, tt, a.L("instance.tab_prefix")+" "+a.L("instance.overview"))
	if err := os.WriteFile(file, []byte(`{"generator":{"type":"pack:opaque"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	clickUI(t, tt, a.L("instance.tab_prefix")+" "+a.L("seedmap.open"))
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	if !tt.HasText(a.L("seedmap.coverage.opaque")) {
		t.Fatal("opaque generator has no visible badge")
	}
	snapshotUI(t, tt, "seedmap-opaque")
}

func TestSeedMapCoordsParse(t *testing.T) {
	for query, want := range map[string][2]int{"120 -340": {120, -340}, "120, -340": {120, -340}, "X 5 Z -6": {5, -6}, "x=1 y=64 z=2": {1, 2}, "10，20": {10, 20}, "1 64 2": {1, 2}} {
		x, z, ok := parseSeedMapCoords(query)
		if !ok || x != want[0] || z != want[1] {
			t.Fatalf("%q: got %d %d %v", query, x, z, ok)
		}
	}
	for _, query := range []string{"plains", "12", "village 3", "99999999 0"} {
		if _, _, ok := parseSeedMapCoords(query); ok {
			t.Fatalf("%q parsed as coordinates", query)
		}
	}
}

func TestSeedMapNavigationAndUnifiedSearch(t *testing.T) {
	a, tt, _ := worldsTester(t)
	a.OverrideLanguage(LangEN)
	tab := a.L("instance.tab_prefix") + " " + a.L("seedmap.open")
	waitUI(t, tt, func() bool { _, ok := tt.Find(tab); return ok })
	clickUI(t, tt, tab)
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	v := a.seedMap
	tt.Advance(400 * time.Millisecond)
	tt.Frame()
	stage, ok := tt.Find(a.L("seedmap.canvas"))
	if !ok {
		t.Fatal("no canvas")
	}
	worldAt := func(px, py float32) (float64, float64) {
		scale := max(stage.W/float32(v.viewport.Width), stage.H/float32(v.viewport.Height))
		units := float64(scale / float32(v.viewport.BlocksPerPixel))
		return v.camera.x + float64(px-stage.X-stage.W/2)/units, v.camera.z + float64(py-stage.Y-stage.H/2)/units
	}
	px, py := stage.X+stage.W*.8, stage.Y+stage.H*.3
	for _, dy := range []float32{-120, 120, 120} {
		bx, bz := worldAt(px, py)
		before := v.viewport.BlocksPerPixel*16 + 16/max(1, v.pixelsPerBlock)
		tt.Move(px, py)
		tt.Scroll(px, py, 0, dy)
		tt.Frame()
		tt.Frame()
		if after := v.viewport.BlocksPerPixel*16 + 16/max(1, v.pixelsPerBlock); after == before {
			t.Fatalf("wheel %v did not zoom", dy)
		}
		ax, az := worldAt(px, py)
		if math.Abs(ax-bx) > 2 || math.Abs(az-bz) > 2 {
			t.Fatalf("wheel zoom moved the point under the cursor: %.1f,%.1f -> %.1f,%.1f", bx, bz, ax, az)
		}
	}
	tt.Click(a.L("seedmap.canvas"))
	tt.Frame()
	startX, startZ := v.camera.x, v.camera.z
	tt.KeyDown(ui.KeyRight)
	tt.KeyDown(ui.KeyS)
	for i := 0; i < 30; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	tt.KeyUp(ui.KeyRight)
	tt.KeyUp(ui.KeyS)
	tt.Frame()
	if v.camera.x <= startX+10 || v.camera.z <= startZ+10 {
		t.Fatalf("keyboard pan did not move: %.1f,%.1f -> %.1f,%.1f", startX, startZ, v.camera.x, v.camera.z)
	}
	stopX := v.camera.x
	for i := 0; i < 5; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
	if v.camera.x != stopX {
		t.Fatal("keyboard pan continued after key up")
	}
	zoomBefore := v.viewport.BlocksPerPixel
	tt.Key(0, ui.KeyMinus)
	tt.Frame()
	if v.viewport.BlocksPerPixel == zoomBefore && v.pixelsPerBlock == 1 {
		t.Fatal("minus key did not zoom out")
	}
	tt.Key(0, ui.KeyEqual)
	tt.Frame()

	clickUI(t, tt, a.L("seedmap.search"))
	tt.Type("120 -340")
	tt.Frame()
	if !tt.HasText(fmt.Sprintf(a.L("seedmap.goto_coords"), 120, -340)) {
		t.Fatalf("coordinate result missing: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if v.viewport.X != 120 || v.viewport.Z != -340 {
		t.Fatalf("Enter did not jump: %d %d", v.viewport.X, v.viewport.Z)
	}
	waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport == v.viewport })
	tt.Advance(600 * time.Millisecond)
	tt.Frame()
	snapshotUI(t, tt, "seedmap-ux-coords")

	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("desert")
	tt.Frame()
	biome := a.L("seedmap.biome_nearest") + " " + a.seedMapName("biome", "minecraft:desert", true)
	waitUI(t, tt, func() bool { _, ok := tt.Find(biome); return ok })
	snapshotUI(t, tt, "seedmap-ux-search-list")
	clickUI(t, tt, biome)
	waitUI(t, tt, func() bool { return v.biomeSearch == "" && v.biomeStatus != "" })
	if v.biomeStatusWarn {
		t.Fatalf("desert not found: %s", v.biomeStatus)
	}
	if id, _ := v.sampler.BiomeAt(v.viewport.Dimension, v.viewport.X, v.viewport.Y, v.viewport.Z); id != "minecraft:desert" {
		t.Fatalf("biome jump landed on %s (%s)", id, v.biomeStatus)
	}
	waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport == v.viewport })
	tt.Advance(600 * time.Millisecond)
	tt.Frame()
	snapshotUI(t, tt, "seedmap-ux-biome")
}
