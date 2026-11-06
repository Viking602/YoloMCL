package launcher

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The installed-instance matrix is opt-in and read-only. Expectations are
// supplied by the caller so unsupported cases remain visible in diagnostics.
func TestWorldgenInstalledInstances(t *testing.T) {
	path := os.Getenv("MCL_WORLDGEN_INSTANCES")
	if path == "" {
		t.Skip("MCL_WORLDGEN_INSTANCES names an external instance matrix")
	}
	var fixture struct {
		Root  string
		Cases []struct {
			Version, World  string
			JavaPath        string // per-instance runtime, as configured in the launcher
			Seed            int64
			Require         []string
			RegionProviders []string
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, entry := range fixture.Cases {
		t.Run(entry.Version+"/"+entry.World, func(t *testing.T) {
			s := testService(t)
			s.cfg.Roots = []Root{{ID: "matrix", Path: fixture.Root, SelectedVersion: entry.Version}}
			if entry.JavaPath != "" {
				s.cfg.Roots[0].VersionConfigs = map[string]VersionConfig{entry.Version: {JavaPath: entry.JavaPath}}
			}
			start := time.Now()
			snapshot, level, err := s.SeedMapWorldgen(context.Background(), "matrix", "", entry.World)
			if err != nil {
				t.Fatal(err)
			}
			seed := entry.Seed
			if entry.World != "" {
				seed = level.RandomSeed
			}
			sampler, err := NewLocalSampler(context.Background(), snapshot, seed)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("%s/%s seed=%d mods=%d resources=%d load=%s", snapshot.Minecraft, snapshot.Loader, seed, len(snapshot.Mods), len(snapshot.Documents), time.Since(start))
			providers := make(map[string]bool, len(snapshot.RegionPrograms))
			for _, program := range snapshot.RegionPrograms {
				providers[program.ModID] = true
				regions := 0
				if program.Result != nil {
					regions = len(program.Result.Regions)
				}
				t.Logf("registration %s: regions=%d error=%s", program.ModID, regions, program.Error)
			}
			for _, id := range entry.RegionProviders {
				if !providers[id] {
					t.Errorf("missing expected region provider %s", id)
				}
			}
			seenIssues := map[WorldgenIssue]bool{}
			for _, dimension := range sampler.Dimensions() {
				coverage := sampler.Coverage(dimension)
				biome, ok := sampler.BiomeAt(dimension, 34, 64, 129)
				t.Logf("%s: %s sample=%s/%t surface=%t", dimension, coverage.Terrain, biome, ok, sampler.SupportsSurface(dimension))
				for _, issue := range coverage.Issues {
					if issue.Code != "candidate_only" && issue.Code != "placement_unsupported" && !seenIssues[issue] {
						seenIssues[issue] = true
						t.Logf("  %s: %s", issue.Code, issue.Source)
					}
				}
			}
			for _, dimension := range entry.Require {
				tile, err := sampler.Sample(context.Background(), SeedMapTileKey{X: 0, Z: 1, Y: 64, BlocksPerPixel: 1, Dimension: dimension, Surface: sampler.SupportsSurface(dimension)})
				if err != nil || tile == nil || len(tile.Biomes) == 0 {
					t.Errorf("required preview %s unavailable: %v", dimension, err)
				}
			}
			if output := os.Getenv("MCL_WORLDGEN_MATRIX_OUTPUT"); output != "" {
				if err := os.MkdirAll(output, 0700); err != nil {
					t.Fatal(err)
				}
				data, err := json.Marshal(snapshot)
				if err != nil {
					t.Fatal(err)
				}
				name := entry.Version
				if entry.World != "" {
					name += "--" + entry.World
				}
				if err := os.WriteFile(filepath.Join(output, name+".json"), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
