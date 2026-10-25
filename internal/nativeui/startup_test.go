package nativeui

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestFirstFrameGateSupportsCancellationAndHeadlessRendering(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	if !waitForFirstFrame(ctx, nil) {
		t.Fatal("headless render waited for a nonexistent window")
	}
	ready := make(chan struct{})
	finished := make(chan bool, 1)
	go func() { finished <- waitForFirstFrame(ctx, ready) }()
	cancel()
	if <-finished {
		t.Fatal("cancelled startup worker continued")
	}
	close(ready)
	if !waitForFirstFrame(context.Background(), ready) {
		t.Fatal("presentation did not release background work")
	}
}

func TestInitialViewReusesRequestedModList(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".minecraft", "versions", "pack")
	if err := os.MkdirAll(filepath.Join(dir, "mods"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pack.json"), []byte(`{"id":"pack","inheritsFrom":"1.21.1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := launcher.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddRoot(filepath.Dir(filepath.Dir(dir)), "pack"); err != nil {
		t.Fatal(err)
	}
	for page, scans := range map[string]int{"home": 0, "home/mods": 1} {
		t.Run(page, func(t *testing.T) {
			a, err := New(s)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { a.cancel(); a.bgWG.Wait() }()
			a.ShowPage(page)
			requested := a.modsGen
			tt := ui.NewTester(a.View, 1200, 820)
			waitUI(t, tt, func() bool { return !a.modsLoading })
			if requested != scans || a.modsGen != requested {
				t.Fatalf("first view requested unnecessary mod scans: %d -> %d, want %d", requested, a.modsGen, scans)
			}
		})
	}
}
