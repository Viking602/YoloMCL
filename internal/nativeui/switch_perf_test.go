package nativeui

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// Two versions with 199 mods each: a switch must not block the UI thread
// (the frame right after the click stays fast), the list fills in async,
// and switching back is served from the cache in the same frame.
func TestVersionSwitchStaysOffUIThread(t *testing.T) {
	for _, tab := range []string{"overview", "mods"} {
		t.Run(tab, func(t *testing.T) { testPackSwitchFrames(t, tab) })
	}
}

func testPackSwitchFrames(t *testing.T, tab string) {
	t.Setenv("MCL_DEVELOPMENT", "")
	mc := filepath.Join(t.TempDir(), ".minecraft")
	var screenshot bytes.Buffer
	if tab == "overview" {
		if e := png.Encode(&screenshot, image.NewRGBA(image.Rect(0, 0, 3840, 2160))); e != nil {
			t.Fatal(e)
		}
	}
	for _, v := range []string{"forge-a", "forge-b"} {
		vd := filepath.Join(mc, "versions", v)
		_ = os.MkdirAll(filepath.Join(vd, "mods"), 0755)
		_ = os.WriteFile(filepath.Join(vd, v+".json"), []byte(`{"id":"`+v+`","inheritsFrom":"1.20.1","libraries":[{"name":"net.minecraftforge:forge:1.20.1-47.4.10"}]}`), 0644)
		if tab == "overview" {
			_ = os.MkdirAll(filepath.Join(vd, "screenshots"), 0755)
			if e := os.WriteFile(filepath.Join(vd, "screenshots", "4k.png"), screenshot.Bytes(), 0644); e != nil {
				t.Fatal(e)
			}
		}
		for i := 0; i < 199; i++ {
			var jar bytes.Buffer
			z := zip.NewWriter(&jar)
			w, _ := z.Create("fabric.mod.json")
			fmt.Fprintf(w, `{"schemaVersion":1,"id":"m%d","version":"1.0.%d","name":"Mod %d"}`, i, i, i)
			_ = z.Close()
			_ = os.WriteFile(filepath.Join(vd, "mods", fmt.Sprintf("m%03d.jar", i)), jar.Bytes(), 0644)
		}
	}
	s, e := launcher.New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	root, e := s.AddRoot(mc, "test")
	if e != nil {
		t.Fatal(e)
	}
	// Select before NewTester renders: rendering b first would warm the
	// supposed cold target and hide the expensive first list layout.
	if e := s.Select(root.ID, "", "forge-a"); e != nil {
		t.Fatal(e)
	}
	a, e := New(s)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { a.cancel(); a.bgWG.Wait() }()
	a.instTab = tab
	tt := ui.NewTester(a.View, 1536, 1024)
	tt.Frame()
	ready := func() bool {
		// Summary publication has a bounded thumbnail wait. A slow image
		// decoder must not prevent the pack's counts and controls appearing.
		if tab == "overview" {
			if a.instStatus.Mods != 199 || filepath.Base(a.instStatus.Dir) != a.root().SelectedVersion {
				return false
			}
			shot := a.instStatus.LatestShot
			if shot == nil {
				return false
			}
			a.imageMu.Lock()
			defer a.imageMu.Unlock()
			return a.images[overviewShotKey(*a.root(), *shot)] != nil
		}
		return !a.modsLoading && len(a.mods) == 199 && a.modsKey == a.root().ID+"|"+a.root().SelectedVersion+"|mods"
	}
	waitUI(t, tt, ready)

	measure := func(ver string) (click, filled, worst time.Duration) {
		t0 := time.Now()
		a.selectVersion(root.ID, ver)
		tt.Frame()
		click = time.Since(t0)
		worst = click
		for {
			frame := time.Now()
			tt.Frame()
			worst = max(worst, time.Since(frame))
			if ready() {
				break
			}
			if time.Since(t0) > 5*time.Second {
				t.Fatal("pack list did not finish")
			}
			time.Sleep(5 * time.Millisecond)
		}
		filled = time.Since(t0)
		return
	}
	c1, f1, w1 := measure("forge-b") // cold: scans 199 jars in the background
	c2, f2, w2 := measure("forge-a") // warm: cached
	t.Logf("cold pack: click %v, list %v, worst frame %v; cached: click %v, list %v, worst frame %v", c1, f1, w1, c2, f2, w2)
	if a.pendingOp("select") {
		t.Fatal("switch set busy")
	}
	if w1 > 250*time.Millisecond || w2 > 250*time.Millisecond {
		t.Errorf("switch frame blocked the UI thread: %v / %v", w1, w2)
	}
	snapshotUI(t, tt, "pack-switch-"+tab)
}
