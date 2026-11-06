package nativeui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestSeedMapOreOverlay(t *testing.T) {
	a, tt, _ := worldsTester(t)
	a.OverrideLanguage(LangEN)
	tab := a.L("instance.tab_prefix") + " " + a.L("seedmap.open")
	waitUI(t, tt, func() bool { _, ok := tt.Find(tab); return ok })
	clickUI(t, tt, tab)
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	v := a.seedMap
	tt.SetSize(1440, 900)
	tt.Frame()
	settle := func() {
		waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport == v.viewport })
		tt.Frame()
		tt.Advance(600 * time.Millisecond)
		tt.Frame()
	}
	setY := func(y string) {
		clickUI(t, tt, a.L("seedmap.y"))
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Key(ui.Ctrl, ui.KeyA)
		tt.Type(y)
		clickUI(t, tt, a.L("seedmap.y_apply"))
		settle()
	}
	clickUI(t, tt, a.L("seedmap.ores.toggle"))
	tt.Frame()
	setY("-54")
	for i := 0; i < 6 && tt.HasText(a.L("seedmap.ores.zoom")); i++ {
		clickUI(t, tt, a.L("seedmap.zoom_in"))
		settle()
	}
	if !v.showOres || !tt.HasText(a.L("seedmap.ores.note")) || tt.HasText(a.L("seedmap.ores.zoom")) {
		t.Fatalf("ore overlay not drawn: %v", tt.Texts())
	}
	n := 0
	for _, cells := range v.ores.cells {
		n += len(cells)
	}
	if n == 0 {
		t.Fatal("no ore cells generated")
	}
	snapshotUI(t, tt, "seedmap-ores-y-54")
	clickUI(t, tt, a.L("seedmap.ores.diamond"))
	tt.Frame()
	if v.oreFilter != "diamond" {
		t.Fatalf("filter = %q", v.oreFilter)
	}
	settle()
	snapshotUI(t, tt, "seedmap-ores-y-54-diamond")
	clickUI(t, tt, a.L("seedmap.ores.diamond"))
	setY("16")
	snapshotUI(t, tt, "seedmap-ores-y16")
	a.OverrideLanguage(LangZH)
	tt.Frame()
	tt.Frame()
	settle()
	snapshotUI(t, tt, "seedmap-ores-y16-zh")
	a.OverrideLanguage(LangEN)
	tt.Frame()
	tt.Frame()
	clickUI(t, tt, a.L("seedmap.ores.toggle"))
	tt.Frame()
	if v.showOres || tt.HasText(a.L("seedmap.ores.note")) {
		t.Fatal("ore overlay did not hide")
	}
}
