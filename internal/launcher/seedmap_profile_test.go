package launcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSeedMapOpenProfile times the seed-map open pipeline per instance.
// MCL_SEEDMAP_PROFILE="<instance dir>|<world>;..." (opt-in, read-only).
func TestSeedMapOpenProfile(t *testing.T) {
	spec := os.Getenv("MCL_SEEDMAP_PROFILE")
	if spec == "" {
		t.Skip("set MCL_SEEDMAP_PROFILE")
	}
	s := testService(t)
	// Mirror the real app: mapping downloads already on disk (parse cost only).
	if src := os.Getenv("MCL_SEEDMAP_MAPPINGS"); src != "" {
		dst := filepath.Join(s.dir, "cache", "game-mappings")
		os.MkdirAll(dst, 0o755)
		entries, _ := os.ReadDir(src)
		for _, e := range entries {
			if b, err := os.ReadFile(filepath.Join(src, e.Name())); err == nil {
				os.WriteFile(filepath.Join(dst, e.Name()), b, 0o644)
			}
		}
	}
	for _, item := range strings.Split(spec, ";") {
		inst, world, _ := strings.Cut(item, "|")
		root := Root{ID: "profile", Path: filepath.Dir(filepath.Dir(inst)), SelectedVersion: filepath.Base(inst)}
		s.cfg.Roots = []Root{root}
		for pass := 1; pass <= 2; pass++ {
			ctx := context.Background()
			t0 := time.Now()
			snap, level, err := s.SeedMapWorldgen(ctx, root.ID, root.SelectedVersion, world)
			t1 := time.Now()
			if err != nil {
				t.Logf("%s pass%d worldgen err %v (%v)", filepath.Base(inst), pass, err, t1.Sub(t0))
				continue
			}
			sampler, err := NewLocalSampler(ctx, snap, level.RandomSeed)
			t2 := time.Now()
			var spawnErr error
			if err == nil {
				_, spawnErr = sampler.EstimateSpawn(ctx)
			}
			t3 := time.Now()
			var dims []string
			if sampler != nil {
				for id, d := range sampler.dimensions {
					dims = append(dims, fmt.Sprintf("%s=%s/opaque=%v/engine=%v", id, d.mode, d.opaque, d.engine != nil))
				}
			}
			t.Logf("%s pass%d ingest=%v sampler=%v spawn=%v total=%v hash=%.12s err=%v spawnErr=%v dims=%.300v", filepath.Base(inst), pass, t1.Sub(t0).Round(time.Millisecond), t2.Sub(t1).Round(time.Millisecond), t3.Sub(t2).Round(time.Millisecond), t3.Sub(t0).Round(time.Millisecond), snap.Hash, err, spawnErr, dims)
		}
	}
}
