package nativeui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestJavaDiscoveryWaitsForFirstFrameAndSurvivesInstanceSwitches(t *testing.T) {
	s, err := launcher.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), ".minecraft")
	for _, id := range []string{"pack-one", "pack-two"} {
		dir := filepath.Join(root, "versions", id)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(`{"id":"`+id+`","inheritsFrom":"1.21.1"}`), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.AddRoot(root, "packs"); err != nil {
		t.Fatal(err)
	}
	a, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { a.cancel(); a.bgWG.Wait() }()
	a.ShowPage("settings")
	tt := ui.NewTester(a.View, 1200, 820)
	cache := filepath.Join(a.snapshot.DataDir, "cache", "java-list.json")
	if a.javaScanRevision != 0 || a.javaScanning || a.javaGet.listBusy {
		t.Fatal("initial settings view started Java work before first presentation")
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatal("initial settings view wrote Java discovery cache")
	}
	a.startBackgroundJava()
	a.startBackgroundJava() // repeated lifecycle notifications must be harmless
	waitUI(t, tt, func() bool {
		return a.javaScannedOnce && !a.javaScanning && a.javaGet.list != nil && !a.javaGet.listBusy
	})
	a.bgWG.Wait()
	before, err := os.Stat(cache)
	if err != nil {
		t.Fatal(err)
	}
	if a.javaScanRevision != 1 {
		t.Fatalf("first presentation started %d scans", a.javaScanRevision)
	}
	for i := 0; i < 8; i++ {
		a.ShowPage("home")
		r := a.root()
		a.selectVersion(r.ID, r.Versions[i%len(r.Versions)].ID)
		tt.Frame()
		a.openDialog("root")
		tt.Frame()
		a.closeDialog()
		a.ShowPage("settings")
		tt.Frame()
	}
	a.bgWG.Wait()
	tt.Frame()
	after, err := os.Stat(cache)
	if err != nil || !before.ModTime().Equal(after.ModTime()) || a.javaScanRevision != 1 {
		t.Fatalf("instance/settings switches repeated Java detection: revision=%d err=%v", a.javaScanRevision, err)
	}
	a.refreshJavaList(true)
	waitUI(t, tt, func() bool { return !a.javaScanning && a.javaGet.listRevision == 2 })
	if a.javaScanRevision != 2 {
		t.Fatal("explicit rescan is unavailable")
	}
}
