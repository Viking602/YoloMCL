package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZZP133TectonicSnap(t *testing.T) {
	mode := os.Getenv("MCL_P133_TECT")
	if mode == "" {
		t.Skip()
	}
	if mode == "notect" {
		tectonicPackDisabled = true
		t.Cleanup(func() { tectonicPackDisabled = false })
	}
	world := `D:\MC\.minecraft\versions\project01\saves\Codex_Compat_Regression_REI_20260925`
	instance := filepath.Dir(filepath.Dir(world))
	root := Root{ID: "t", Path: filepath.Dir(filepath.Dir(instance)), SelectedVersion: filepath.Base(instance)}
	s := testService(t)
	s.cfg.Roots = []Root{root}
	snap, level, err := s.SeedMapWorldgen(context.Background(), root.ID, "", filepath.Base(world))
	if err != nil {
		t.Fatal(err)
	}
	tect := 0
	for _, d := range snap.Documents {
		if strings.Contains(d.Source, "tectonic") {
			tect++
		}
	}
	sampler, err := NewLocalSampler(context.Background(), snap, level.RandomSeed)
	if err != nil {
		t.Fatal(err)
	}
	d := sampler.dimensions["minecraft:overworld"]
	sc := d.scratch()
	t.Logf("mode=%s tectonicDocs=%d ow=%q", mode, tect, d.mode)
	for _, xz := range [][2]int{{0, 0}, {8, 8}, {13, 8}, {616, 728}, {-200, 0}} {
		b, _ := d.biomeAt(xz[0], 64, xz[1], sc, false)
		_, top, _ := sampler.TerrainColumn("minecraft:overworld", xz[0], xz[1], nil)
		t.Logf("  @%d,%d biome=%s top=%d", xz[0], xz[1], b, top)
	}
	eras, err := s.DetectEras(context.Background(), root.ID, "", filepath.Base(world))
	if err != nil {
		t.Logf("eras: %v", err)
		return
	}
	t.Logf("eras=%v mixed=%v", eras.Eras, eras.Mixed())
	for _, ch := range [][2]int{{0, 0}, {38, 45}, {-13, 0}, {13, 4}, {225, -602}} {
		v := eras.EraAt(ch[0], ch[1])
		t.Logf("  era chunk %v -> era=%d approx=%v n=%d exact=%v", ch, v.Era, v.Approx, v.Samples, v.Exact)
	}
}
