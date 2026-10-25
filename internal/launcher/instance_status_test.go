package launcher

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInstanceStatusSingleWalkTotalsAndCancellation(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), ".minecraft")
	dir := filepath.Join(root, "versions", "pack")
	for path, data := range map[string]string{
		"pack.json":              `{"id":"pack","inheritsFrom":"1.20.1"}`,
		"mods/one.jar":           "one",
		"mods/two.jar.disabled":  "two",
		"resourcepacks/one.zip":  "resource",
		"shaderpacks/one.zip":    "shader",
		"saves/world/region.dat": "world data",
		"screenshots/one.png":    "image",
		"config/example.cfg":     "config",
	} {
		writeTestFile(t, filepath.Join(dir, filepath.FromSlash(path)), []byte(data))
	}
	r, err := s.AddRoot(root, "pack")
	if err != nil {
		t.Fatal(err)
	}
	st, err := s.InstanceStatusContext(context.Background(), r.ID, "pack")
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != treeBytes(dir) || st.Mods != 2 || st.EnabledMods != 1 || st.Saves != 1 || st.ResourcePacks != 1 || st.Shaders != 1 || st.Screenshots != 1 {
		t.Fatalf("wrong counts or total: %+v", st)
	}
	for _, sub := range []string{"mods", "resourcepacks", "shaderpacks", "saves", "screenshots"} {
		if st.Sizes[sub] != treeBytes(filepath.Join(dir, sub)) {
			t.Fatalf("wrong %s size: %d", sub, st.Sizes[sub])
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.InstanceStatusContext(ctx, r.ID, "pack"); !errors.Is(err, context.Canceled) {
		t.Fatalf("obsolete overview scan did not stop: %v", err)
	}
}

func TestInstanceStorageOnlyVisitsMinecraftAndLauncherFiles(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	dir := filepath.Join(root, "versions", "pack")
	allowed := map[string]string{
		"pack.json":  `{"id":"pack","inheritsFrom":"1.21.1"}`,
		"mods/a.jar": "enabled", "mods/b.jar.disabled": "disabled",
		"config/mod.toml": "config", "kubejs/server_scripts/recipes.js": "recipes",
		"saves/world/region/r.0.0.mca": "blocks", "logs/latest.log": "log",
		"options.txt": "settings", ".mcl/playstats.json": "{}",
		".mcl/backups/mod.jar": "saved mod",
	}
	var expected int64
	for path, data := range allowed {
		writeTestFile(t, filepath.Join(dir, filepath.FromSlash(path)), []byte(data))
		expected += int64(len(data))
	}
	for _, path := range []string{
		"dev/nested/source.bin", "cache/large.bin", ".work/tree/object.bin",
		"downloads/file.zip", "releases/pack.zip", "notes.txt", "foreign.json",
		".mcl/unrelated/source.bin", "mods/dev/tool.jar", "mods/notes.txt",
		"kubejs/node_modules/tool/index.js", "config/.git/objects/hash",
	} {
		writeTestFile(t, filepath.Join(dir, filepath.FromSlash(path)), bytes.Repeat([]byte("unrelated"), 1024))
	}
	r, err := s.AddRoot(root, "pack")
	if err != nil {
		t.Fatal(err)
	}
	brief, err := s.InstanceOverviewContext(context.Background(), r.ID, "pack")
	if err != nil || brief.Mods != 2 || brief.EnabledMods != 1 || !brief.StoragePending || brief.Sizes != nil {
		t.Fatalf("summary waited for or mixed in storage: %+v %v", brief, err)
	}
	full, err := s.CompleteInstanceStorage(context.Background(), brief)
	if err != nil || full.Total != expected || full.StoragePending {
		t.Fatalf("unrelated files entered storage accounting: %d want %d, %v", full.Total, expected, err)
	}
	if brief.RecentWorld.Size != 0 || !brief.StoragePending || brief.Sizes != nil {
		t.Fatal("background storage mutated the published summary")
	}
}

func TestOverviewFindsRecentWorldAndScreenshot(t *testing.T) {
	s, r, first := worldFixture(t)
	second := filepath.Join(r.Path, "saves", "w2")
	recent := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	writeTestFile(t, filepath.Join(second, "level.dat"), nbtLevel(t, "Recent World", 1, recent.UnixMilli()))
	writeTestFile(t, filepath.Join(second, "region", "r.0.0.mca"), []byte("recent blocks"))
	writeTestFile(t, filepath.Join(second, "icon.png"), []byte("icon"))
	writeTestFile(t, filepath.Join(r.Path, "backups", "w2", "w2-20261020-120000.zip"), []byte("backup"))
	// Folder timestamps deliberately disagree with LastPlayed in level.dat.
	if e := os.Chtimes(first, recent, recent); e != nil {
		t.Fatal(e)
	}
	shots := filepath.Join(r.Path, "screenshots")
	for file, stamp := range map[string]time.Time{
		"a-latest.png": recent,
		"z-old.PNG":    recent.Add(-time.Hour),
		".hidden.png":  recent.Add(time.Hour),
	} {
		p := filepath.Join(shots, file)
		writeTestFile(t, p, []byte("shot"))
		if e := os.Chtimes(p, stamp, stamp); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.MkdirAll(filepath.Join(shots, "folder.png"), 0755); e != nil {
		t.Fatal(e)
	}
	st, e := s.InstanceStatus(r.ID, "")
	if e != nil {
		t.Fatal(e)
	}
	w := st.RecentWorld
	if st.Saves != 2 || w == nil || w.Dir != "w2" || w.Name != "Recent World" || w.Mode != "creative" || !w.HasIcon || w.Backups != 1 || !w.LastPlayed.Equal(recent) || w.Size != treeBytes(second) {
		t.Fatalf("wrong recent world: %+v / %+v", st, w)
	}
	if st.Screenshots != 2 || st.LatestShot == nil || st.LatestShot.File != "a-latest.png" || !st.LatestShot.Modified.Equal(recent) {
		t.Fatalf("wrong latest screenshot: %+v", st)
	}
	if st.Total != treeBytes(r.Path) {
		t.Fatalf("overview metadata changed storage accounting: %d", st.Total)
	}
}

func TestOverviewScreenshotIsBoundedAndScoped(t *testing.T) {
	s, r, _ := worldFixture(t)
	var data bytes.Buffer
	if e := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 1920, 1080))); e != nil {
		t.Fatal(e)
	}
	writeTestFile(t, filepath.Join(r.Path, "screenshots", "view.png"), data.Bytes())
	b, e := s.ScreenshotPreview(r.ID, "", "view.png")
	if e != nil {
		t.Fatal(e)
	}
	cfg, _, e := image.DecodeConfig(bytes.NewReader(b))
	if e != nil || cfg.Width != 960 || cfg.Height != 540 {
		t.Fatalf("unbounded preview: %+v %v", cfg, e)
	}
	for _, name := range []string{"../view.png", `..\view.png`, "a/b.png", "view.jpg", ""} {
		if _, e := s.ScreenshotPreview(r.ID, "", name); e == nil {
			t.Fatalf("accepted invalid screenshot %q", name)
		}
	}
	writeTestFile(t, filepath.Join(r.Path, "screenshots", "broken.png"), []byte("not an image"))
	if _, e := s.ScreenshotPreview(r.ID, "", "broken.png"); e == nil {
		t.Fatal("accepted corrupt screenshot")
	}
}

func TestOverviewReusesUnchangedFiles(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "versions", "pack")
	writeTestFile(t, filepath.Join(dir, "pack.json"), []byte(`{"id":"pack","mainClass":"x"}`))
	writeTestFile(t, filepath.Join(dir, "saves", "w", "level.dat"), nbtLevel(t, "Cached", 0, 5))
	writeTestFile(t, filepath.Join(dir, "logs", "latest.log"), []byte("[00:00:00] [main/INFO]: Connecting to example.com, 25565\n"))
	r, err := s.AddRoot(filepath.Dir(filepath.Dir(dir)), "pack")
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.InstanceOverviewContext(context.Background(), r.ID, "pack")
	if err != nil || first.Saves != 1 || first.RecentWorld == nil || first.RecentWorld.Name != "Cached" {
		t.Fatalf("%+v %v", first, err)
	}
	second, err := s.InstanceOverviewContext(context.Background(), r.ID, "pack")
	if err != nil || second.RecentWorld.Name != first.RecentWorld.Name || second.Saves != first.Saves {
		t.Fatalf("cache changed the summary: %+v %v", second, err)
	}
	if &second == &first || (second.RecentWorld == first.RecentWorld) {
		t.Fatal("cached overview aliased its stored value")
	}
	writeTestFile(t, filepath.Join(dir, "saves", "w", "level.dat"), nbtLevel(t, "Changed", 0, 6))
	third, err := s.InstanceOverviewContext(context.Background(), r.ID, "pack")
	if err != nil || third.RecentWorld == nil || third.RecentWorld.Name != "Changed" {
		t.Fatalf("changed level.dat stayed cached: %+v %v", third, err)
	}
}

func TestParseLevelDatStopsAfterNeededFields(t *testing.T) {
	var b bytes.Buffer
	str := func(s string) { _ = binary.Write(&b, binary.BigEndian, uint16(len(s))); b.WriteString(s) }
	b.WriteByte(10)
	str("")
	b.WriteByte(10)
	str("Data")
	b.WriteByte(8)
	str("LevelName")
	str("Early")
	b.WriteByte(3)
	str("GameType")
	_ = binary.Write(&b, binary.BigEndian, int32(2))
	b.WriteByte(1)
	str("hardcore")
	b.WriteByte(1)
	b.WriteByte(4)
	str("LastPlayed")
	_ = binary.Write(&b, binary.BigEndian, int64(9))
	b.Write(bytes.Repeat([]byte{0}, 8<<20))
	payload := len(b.Bytes())
	gz, err := gzip.NewReader(bytes.NewReader(compressedNBT(t, b.Bytes())))
	if err != nil {
		t.Fatal(err)
	}
	p := &nbtReader{r: &limitedReader{r: bufio.NewReader(gz), n: maxLevelData}}
	if p.u8() != 10 {
		t.Fatal("not a compound")
	}
	p.str()
	var li LevelInfo
	got := 0
	p.compound(func(path, name string, tag byte) bool {
		if path != "/Data" {
			return false
		}
		switch {
		case name == "LevelName" && tag == 8:
			li.Name = p.str()
		case name == "GameType" && tag == 3:
			li.GameType = int32(p.u32())
		case name == "hardcore" && tag == 1:
			li.Hardcore = p.u8() != 0
		case name == "LastPlayed" && tag == 4:
			li.LastPlayed = int64(p.u64())
		default:
			return false
		}
		got++
		if got == 4 {
			p.done = true
		}
		return true
	}, "")
	if p.err != nil || li.Name != "Early" || li.LastPlayed != 9 || p.consumed() >= payload {
		t.Fatalf("parsed %+v after reading %d of %d: %v", li, p.consumed(), payload, p.err)
	}
}
