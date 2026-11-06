package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZZP130Noise(t *testing.T) {
	world := os.Getenv("MCL_P128_WORLD")
	if world == "" {
		t.Skip()
	}
	instance := filepath.Dir(filepath.Dir(world))
	root := Root{ID: "p130", Path: filepath.Dir(filepath.Dir(instance)), SelectedVersion: filepath.Base(instance)}
	s := testService(t)
	s.cfg.Roots = []Root{root}
	snap, _, err := s.SeedMapWorldgen(context.Background(), root.ID, "", filepath.Base(world))
	if err != nil {
		t.Fatal(err)
	}
	so := string(snap.SavedDimensions["minecraft:overworld"])
	if i := strings.LastIndex(so, "\"settings\""); i >= 0 {
		t.Logf("settings: %.300s", so[i:])
	}
	t.Logf("type: %.200s", so[len(so)-200:])
	for _, d := range snap.Documents {
		if d.Kind == "noise_settings" {
			t.Logf("%s src=%s len=%d head=%.200s", d.ID, d.Source, len(d.Data), d.Data)
		}
	}
	for _, i := range snap.Issues {
		if i.Code == "invalid_json" || i.Code == "source_unreadable" {
			t.Logf("issue %s %s", i.Code, i.Source)
		}
	}
}
