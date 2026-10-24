package nativeui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestOverviewRetainsContentUntilTargetIsReady(t *testing.T) {
	a, old := homeTester(t, LangZH)
	a.bgWG.Wait()
	old.Frame()
	a.service, a.instStatusErr = nil, ""
	a.contentSwaps = nil
	r := *a.root()
	a.instStatusKey = r.ID + "|" + r.SelectedVersion
	a.instStatus = launcher.InstanceStatus{Dir: r.Path, Mods: 93, EnabledMods: 91}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme)
		ui.Column(c).Fill().Background(chromeBg).Children(func() { a.instanceOverview(c, *a.root()) })
	}, 1536, 1024)
	tt.Advance(400 * time.Millisecond)
	panels := map[string]ui.Rect{}
	for _, label := range []string{"整合包概览", "最近世界", "最近服务器", "游戏截图", "运行环境", "存储空间"} {
		panels[label], _ = tt.Find(label)
	}
	serverPixels := overviewRegionPixels(tt.Image(), panels["最近服务器"])
	beforeSwitch := append([]byte(nil), tt.Image().Pix...)
	a.applySelection(r.ID, "1.21.8")
	a.loadInstanceStatus()
	tt.Frame()
	if !tt.HasText("91 个已启用") || tt.HasText("概览加载占位") {
		t.Fatal("cold selection cleared the outgoing overview")
	}
	for _, wait := range []time.Duration{0, 100 * time.Millisecond, 200 * time.Millisecond} {
		tt.Advance(wait)
		if !bytes.Equal(beforeSwitch, tt.Image().Pix) {
			t.Fatal("cold switch dimmed or covered the retained overview")
		}
	}
	// Holding old contents must be inert, rather than applying old actions to
	// the newly selected instance.
	clickUI(t, tt, "全部存档")
	if a.instTab == "worlds" {
		t.Fatal("outgoing overview was still interactive")
	}
	a.instStatus = launcher.InstanceStatus{Dir: r.Path, Mods: 300, EnabledMods: 300}
	tt.Frame()
	if !tt.HasText("91 个已启用") || tt.HasText("300 个已启用") {
		t.Fatal("incoming text overlapped the outgoing overview")
	}
	tt.Advance(66 * time.Millisecond)
	start, _ := tt.Find("动态数字 stat-value-mods")
	first := append([]byte(nil), tt.Image().Pix...)
	tt.Advance(70 * time.Millisecond)
	middle, _ := tt.Find("动态数字 stat-value-mods")
	if bytes.Equal(first, tt.Image().Pix) {
		t.Fatal("overview switched without animation")
	}
	snapshotUI(t, tt, "overview-transition-middle")
	tt.Advance(330 * time.Millisecond)
	end, _ := tt.Find("动态数字 stat-value-mods")
	if start.W <= 0 || start != middle || middle != end {
		t.Fatalf("number flip moved its fixed slot: %+v %+v %+v", start, middle, end)
	}
	for label, before := range panels {
		if now, _ := tt.Find(label); now != before {
			t.Fatalf("content update moved %s: %+v -> %+v", label, before, now)
		}
	}
	if !bytes.Equal(serverPixels, overviewRegionPixels(tt.Image(), panels["最近服务器"])) {
		t.Fatal("an unchanged server card was animated or repainted")
	}
	if !tt.HasText("300 个已启用") || tt.HasText("91 个已启用") || tt.HasText("概览加载占位") {
		t.Fatal("transition retained stale contents")
	}
	snapshotUI(t, tt, "overview-transition-complete")
}

func overviewRegionPixels(img *image.RGBA, r ui.Rect) []byte {
	var pixels []byte
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		start := img.PixOffset(int(r.X), y)
		pixels = append(pixels, img.Pix[start:start+int(r.W)*4]...)
	}
	return pixels
}

func TestRecentServerEmptyToRecordedKeepsPanelInPlace(t *testing.T) {
	a, previous := homeTester(t, LangZH)
	a.bgWG.Wait()
	previous.Frame()
	a.contentSwaps = nil
	st := launcher.InstanceStatus{Dir: a.root().Path}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Width(370).Height(260).AlignItems(ui.Stretch).Children(func() { a.ovServer(c, st) })
	}, 390, 280)
	tt.Advance(400 * time.Millisecond)
	snapshotUI(t, tt, "server-empty-geometry")
	panel, _ := tt.Find("最近服务器")
	if !tt.HasText("还没有连接记录") || tt.HasText("服务器数据") {
		t.Fatal("empty server card implies a recorded connection")
	}
	address := "friends.example.test:25565"
	st.RecentServer = &launcher.InstanceServer{Address: address, Confirmed: true, Connected: time.Now()}
	a.recentPing.cache = map[string]serverProbe{address: {checked: time.Now(), status: &launcher.ServerStatus{Description: "Welcome", Version: "1.21.1"}}}
	tt.Frame()
	if !tt.HasText("还没有连接记录") {
		t.Fatal("incoming server skipped the outgoing empty-state transition")
	}
	first := append([]byte(nil), tt.Image().Pix...)
	tt.Advance(300 * time.Millisecond)
	snapshotUI(t, tt, "server-filled-geometry")
	if got, _ := tt.Find("最近服务器"); got != panel {
		t.Fatalf("record arrival moved the server panel: %+v -> %+v", panel, got)
	}
	if tt.HasText("还没有连接记录") || !tt.HasText("服务器数据") || !tt.HasText("friends.example.test") {
		t.Fatal("recorded server did not replace the empty state")
	}
	if bytes.Equal(first, tt.Image().Pix) {
		t.Fatal("server details appeared without a transition")
	}
}

func TestOverviewSummaryDoesNotWaitForThumbnailWorkers(t *testing.T) {
	game := t.TempDir()
	dir := filepath.Join(game, "versions", "pack")
	for path, data := range map[string]string{"pack.json": `{"id":"pack","inheritsFrom":"1.21.1"}`, "mods/one.jar": "mod", "saves/world/icon.png": "image"} {
		p := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := launcher.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddRoot(game, "pack"); err != nil {
		t.Fatal(err)
	}
	a, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { a.cancel(); a.bgWG.Wait() }()
	for i := 0; i < cap(a.overviewImageSlots); i++ {
		a.overviewImageSlots <- struct{}{}
	}
	for i := 0; i < cap(a.imageSlots); i++ {
		a.imageSlots <- struct{}{}
	}
	a.loadInstanceStatus()
	started := time.Now()
	tt := ui.NewTester(a.View, 1536, 1024)
	waitUI(t, tt, func() bool { return a.instStatus.Mods == 1 && !a.statusLoading })
	if time.Since(started) > time.Second {
		t.Fatal("thumbnails held the summary behind the worker queue")
	}
	if a.instStatus.Dir == "" || a.instStatus.RecentWorld == nil {
		t.Fatal("summary lost its world while images were pending")
	}
}

// Opt-in QA against an installed pack. Only the temporary launcher's config is
// written; changing the test selection never changes the user's saved choice.
func TestLocalPackOverviewQA(t *testing.T) {
	game := os.Getenv("MCL_LOCAL_INSTANCE_ROOT")
	if game == "" {
		t.Skip("local instance QA not requested")
	}
	data := t.TempDir()
	config := map[string]any{"schema": 1, "activeRoot": "qa", "roots": []launcher.Root{{ID: "qa", Path: game, Name: "Minecraft", SelectedVersion: "project01"}}}
	b, _ := json.Marshal(config)
	if err := os.WriteFile(filepath.Join(data, "config.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := launcher.New(data)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { a.cancel(); a.bgWG.Wait() }()
	a.settings.Language = LangZH
	started := time.Now()
	tt := ui.NewTester(a.View, 1536, 1024)
	waitUI(t, tt, func() bool { return a.instStatus.Dir != "" && !a.instStatus.StoragePending })
	t.Logf("project01 summary and scoped storage ready in %v: %d enabled mods, %.1f MiB", time.Since(started), a.instStatus.EnabledMods, float64(a.instStatus.Total)/(1<<20))
	if a.instStatus.EnabledMods != 300 {
		t.Fatalf("unexpected local mod count: %d", a.instStatus.EnabledMods)
	}
	if mb := a.autoMemoryFor("project01"); mb != 12288 {
		t.Fatalf("300-mod memory estimate = %d", mb)
	}
	tt.Advance(400 * time.Millisecond)
	snapshotUI(t, tt, "project01-ready")
	for _, v := range a.root().Versions {
		if v.ID != "project01" {
			a.applySelection("qa", v.ID)
			a.loadInstanceStatus()
			tt.Frame()
			waitUI(t, tt, func() bool { return a.instStatus.Dir != "" })
			tt.Advance(400 * time.Millisecond)
			a.applySelection("qa", "project01")
			a.loadInstanceStatus()
			tt.Frame()
			for frame := 0; frame < 20; frame++ {
				snapshotUI(t, tt, fmt.Sprintf("project01-switch-%02d", frame))
				tt.Advance(17 * time.Millisecond)
			}
			break
		}
	}
}
