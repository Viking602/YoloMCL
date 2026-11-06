package nativeui

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// TestSeedMapMansionPreviewShots opens the 3D preview of a woodland mansion
// and checks it is labelled layout-seeded (rooms seed-exact, blocks drawn).
func TestSeedMapMansionPreviewShots(t *testing.T) {
	const jar = `D:\MC\.minecraft\versions\1.21.1\1.21.1.jar`
	if _, err := os.Stat(jar); err != nil {
		t.Skip("live 1.21.1 game jar required")
	}
	a, tt, saves := worldsTester(t)
	copyLiveJar(t, jar, saves)
	a.OverrideLanguage(LangEN)
	tab := a.L("instance.tab_prefix") + " " + a.L("seedmap.open")
	waitUI(t, tt, func() bool { _, ok := tt.Find(tab); return ok })
	clickUI(t, tt, tab)
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	v := a.seedMap
	tt.SetSize(1440, 900)
	tt.Frame()
	v.selected = &launcher.FeatureHit{ID: "minecraft:mansion", X: 1240, Z: -856, Confidence: "biome_candidate"}
	tt.Frame()
	clickUI(t, tt, a.L("seedmap.structure_preview"))
	// Advance the dive clock explicitly: waitUI only Frames, so under a loaded
	// machine a still-diving preview never settles inside its wall timeout.
	for i := 0; i < 80 && (v.structurePreview == nil || v.structurePreview.loading); i++ {
		tt.Frame()
		time.Sleep(20 * time.Millisecond)
	}
	if v.structurePreview == nil || v.structurePreview.loading {
		t.Fatalf("mansion preview did not load: %v", tt.Texts())
	}
	p0 := v.structurePreview
	for i := 0; i < 20 && p0.diving; i++ {
		tt.Frame()
		tt.Advance(diveDuration / 10)
	}
	for i := 0; i < 500 && (p0.rendering || p0.dirty); i++ {
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if p0.loading || p0.rendering || p0.dirty || p0.diving || p0.bitmap == nil {
		t.Fatalf("mansion preview did not settle: loading=%v rendering=%v dirty=%v diving=%v bitmap=%v", p0.loading, p0.rendering, p0.dirty, p0.diving, p0.bitmap != nil)
	}
	tt.Advance(600 * time.Millisecond)
	tt.Frame()
	p := v.structurePreview
	if p.err != "" || p.unsupported != "" || !p.seeded || p.blocks == 0 {
		t.Fatalf("mansion preview: err=%q unsupported=%q layout=%v seeded=%v blocks=%d", p.err, p.unsupported, p.layoutSeeded, p.seeded, p.blocks)
	}
	if !tt.HasText(a.L("seedmap.structure_preview.seeded")) {
		t.Fatalf("mansion label missing: %v", tt.Texts())
	}
	snapshotUI(t, tt, "seedmap-mansion-preview")
	p.yaw, p.pitch, p.zoom = 0.6, 1.25, 8
	a.rerenderStructurePreview(v)
	waitUI(t, tt, func() bool { return !v.structurePreview.rendering && !v.structurePreview.dirty })
	tt.Advance(600 * time.Millisecond)
	tt.Frame()
	snapshotUI(t, tt, "seedmap-mansion-preview-top")
	a.OverrideLanguage(LangZH)
	tt.Frame()
	tt.Frame()
	snapshotUI(t, tt, "seedmap-mansion-preview-zh")
	t.Logf("mansion preview %q: %d blocks", p.title, p.blocks)
}

// copyLiveJar puts the real 1.21.1 client jar into the fixture's version
// folder so the preview can read the vanilla structure definitions.
func copyLiveJar(t *testing.T, jar, saves string) {
	t.Helper()
	for dir := saves; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		ver := filepath.Join(dir, "versions", "1.21.1")
		if _, err := os.Stat(filepath.Join(ver, "1.21.1.json")); err != nil {
			continue
		}
		in, err := os.Open(jar)
		if err != nil {
			t.Fatal(err)
		}
		defer in.Close()
		out, err := os.Create(filepath.Join(ver, "1.21.1.jar"))
		if err != nil {
			t.Fatal(err)
		}
		defer out.Close()
		if _, err = io.Copy(out, in); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatalf("no versions/1.21.1 above %s", saves)
}
