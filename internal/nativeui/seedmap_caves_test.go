package nativeui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestSeedMapCaveOverlay(t *testing.T) {
	a, tt, _ := worldsTester(t)
	a.OverrideLanguage(LangEN)
	tab := a.L("instance.tab_prefix") + " " + a.L("seedmap.open")
	waitUI(t, tt, func() bool { _, ok := tt.Find(tab); return ok })
	clickUI(t, tt, tab)
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	v := a.seedMap
	if !v.seedMapCavesSupported() {
		t.Fatalf("caves unsupported for %q", v.snapshot.Minecraft)
	}
	tt.SetSize(1440, 900)
	tt.Frame()
	carved := func() int {
		n := 0
		for _, m := range v.caves.masks {
			for z := range 16 {
				n += popcount16(m.cave[z] | m.canyon[z])
			}
		}
		return n
	}
	setY := func(y string) {
		clickUI(t, tt, a.L("seedmap.y"))
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Key(ui.Ctrl, ui.KeyA)
		tt.Type(y)
		clickUI(t, tt, a.L("seedmap.y_apply"))
		waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport == v.viewport })
	}
	clickUI(t, tt, a.L("seedmap.caves.toggle"))
	tt.Frame()
	setY("-30")
	tt.Frame()
	if !v.showCaves || !tt.HasText(a.L("seedmap.caves.note")) {
		t.Fatalf("cave overlay not shown: %v", tt.Texts())
	}
	if carved() == 0 {
		t.Fatal("no carved blocks in view at y -30")
	}
	tt.Advance(600 * time.Millisecond)
	tt.Frame()
	snapshotUI(t, tt, "seedmap-caves-y-30")
	clickUI(t, tt, a.L("seedmap.zoom_in"))
	waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport == v.viewport })
	tt.Frame()
	tt.Advance(600 * time.Millisecond)
	tt.Frame()
	snapshotUI(t, tt, "seedmap-caves-y-30-zoom")
	setY("-57")
	tt.Frame()
	if !tt.HasText(a.L("seedmap.caves.lava")) {
		t.Fatal("lava legend missing at y -57")
	}
	tt.Advance(600 * time.Millisecond)
	tt.Frame()
	snapshotUI(t, tt, "seedmap-caves-y-57-lava")
	setY("30")
	tt.Frame()
	tt.Advance(600 * time.Millisecond)
	tt.Frame()
	snapshotUI(t, tt, "seedmap-caves-y30")
	a.OverrideLanguage(LangZH)
	tt.Frame()
	tt.Frame()
	tt.Advance(600 * time.Millisecond)
	tt.Frame()
	snapshotUI(t, tt, "seedmap-caves-y30-zh")
	a.OverrideLanguage(LangEN)
	tt.Frame()
	tt.Frame()
	clickUI(t, tt, a.L("seedmap.caves.toggle"))
	tt.Frame()
	if v.showCaves || tt.HasText(a.L("seedmap.caves.note")) {
		t.Fatal("cave overlay did not hide")
	}
}

func popcount16(x uint16) int {
	n := 0
	for ; x != 0; x &= x - 1 {
		n++
	}
	return n
}
