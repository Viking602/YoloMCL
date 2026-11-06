package nativeui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTerrainModsHideAllSeedMapEntrances(t *testing.T) {
	a, tt, saves := worldsTester(t)
	r := *a.root()
	mods := filepath.Join(filepath.Dir(saves), "mods")
	if err := os.MkdirAll(mods, 0755); err != nil {
		t.Fatal(err)
	}
	writeZip(t, filepath.Join(mods, "renamed.jar"), map[string]string{"fabric.mod.json": `{"id":"tectonic","version":"1"}`})
	if err := a.service.ToggleMod(r.ID, "renamed.jar"); err != nil {
		t.Fatal(err)
	}
	a.snapshot = a.service.Snapshot()
	tt.Frame()
	tab := a.L("instance.tab_prefix") + " " + a.L("seedmap.open")
	world := fmt.Sprintf(a.L("seedmap.open_world"), "world-one")
	if _, ok := tt.Find(tab); !ok {
		t.Fatal("a disabled terrain mod should not hide the map")
	}
	if err := a.service.ToggleMod(r.ID, "renamed.jar.disabled"); err != nil {
		t.Fatal(err)
	}
	a.snapshot = a.service.Snapshot()
	tt.Frame()
	waitUI(t, tt, func() bool { return !a.root().Versions[0].SeedMapCheckPending && len(a.pendingRequests) == 0 })
	for _, label := range []string{tab, world} {
		if _, ok := tt.Find(label); ok {
			t.Fatalf("terrain mod retained map entrance %q", label)
		}
	}
	a.openSeedMap("world-one")
	a.setInstTab("seedmap")
	tt.Frame()
	if a.seedMap != nil || a.instTab == "seedmap" {
		t.Fatal("a stale action reopened a blocked map")
	}
	// An external change also closes a map that was already displayed.
	if err := a.service.ToggleMod(r.ID, "renamed.jar"); err != nil {
		t.Fatal(err)
	}
	a.snapshot = a.service.Snapshot()
	a.openSeedMap("")
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	old := a.seedMap.tiles
	if err := a.service.ToggleMod(r.ID, "renamed.jar.disabled"); err != nil {
		t.Fatal(err)
	}
	a.snapshot = a.service.Snapshot()
	tt.Frame()
	if a.seedMap != nil || a.instTab != "overview" {
		t.Fatal("enabling terrain generation retained a stale map")
	}
	select {
	case <-old.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("hiding the map did not stop its sampler")
	}
}
