package launcher

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestSeedMapEarlyEquivalence checks that every dimension offered from the
// provisional snapshot samples byte-identically to the final snapshot, and
// reports time-to-first-map. Opt-in via MCL_SEEDMAP_PROFILE (read-only).
func TestSeedMapEarlyEquivalence(t *testing.T) {
	spec := os.Getenv("MCL_SEEDMAP_PROFILE")
	if spec == "" {
		t.Skip("set MCL_SEEDMAP_PROFILE")
	}
	s := testService(t)
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
		var provisional *WorldgenSnapshot
		var early time.Duration
		var earlyDims []string
		var earlySampler *LocalSampler
		t0 := time.Now()
		ctx := WithSeedMapEarly(context.Background(), func(p *WorldgenSnapshot, level LevelInfo) {
			provisional = p
			sm, err := NewLocalSampler(context.Background(), p, level.RandomSeed)
			if err == nil {
				earlySampler = sm
				earlyDims = SeedMapEarlyDimensions(context.Background(), p, sm)
			}
			early = time.Since(t0)
		})
		snap, level, err := s.SeedMapWorldgen(ctx, root.ID, root.SelectedVersion, world)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(inst), err)
			continue
		}
		full, err := NewLocalSampler(context.Background(), snap, level.RandomSeed)
		total := time.Since(t0)
		if err != nil {
			t.Errorf("%s full sampler: %v", filepath.Base(inst), err)
			continue
		}
		if provisional == nil || earlySampler == nil {
			t.Logf("%s: no early snapshot (total %v hash %.12s)", filepath.Base(inst), total.Round(time.Millisecond), snap.Hash)
			continue
		}
		mismatch := 0
		tiles := 0
		for _, dim := range earlyDims {
			for _, surface := range []bool{false, true} {
				if surface && !full.SupportsSurface(dim) {
					continue
				}
				if earlySampler.SupportsSurface(dim) != full.SupportsSurface(dim) {
					mismatch++
				}
				for _, bpp := range []int{1, 4, 16} {
					for _, xz := range [][2]int{{0, 0}, {-2048, 1024}, {3000, -5000}, {-20000, -20000}} {
						key := SeedMapTileKey{X: xz[0], Z: xz[1], BlocksPerPixel: bpp, Y: 64, Dimension: dim, Surface: surface}
						a, errA := earlySampler.Sample(context.Background(), key)
						b, errB := full.Sample(context.Background(), key)
						tiles++
						if (errA == nil) != (errB == nil) || errA == nil && (!bytes.Equal(a.Image.Pix, b.Image.Pix) || !reflect.DeepEqual(a.Biomes, b.Biomes) || !reflect.DeepEqual(a.Hits, b.Hits)) {
							mismatch++
							t.Errorf("%s %s %+v differs", filepath.Base(inst), dim, key)
						}
					}
				}
			}
			if !reflect.DeepEqual(earlySampler.Structures(dim), full.Structures(dim)) {
				mismatch++
				t.Errorf("%s %s structures differ", filepath.Base(inst), dim)
			}
		}
		spawnSame := "n/a"
		if len(earlyDims) > 0 && earlyDims[0] == "minecraft:overworld" {
			a, _ := earlySampler.EstimateSpawn(context.Background())
			b, _ := full.EstimateSpawn(context.Background())
			spawnSame = fmt.Sprint(reflect.DeepEqual(a, b))
			if !reflect.DeepEqual(a, b) {
				mismatch++
				t.Errorf("%s spawn differs", filepath.Base(inst))
			}
		}
		sum := sha256.Sum256([]byte(strings.Join(earlyDims, ",")))
		t.Logf("%s early=%v total=%v hash=%.12s earlyDims=%d/%d %v tiles=%d mismatches=%d spawnSame=%s (%x)", filepath.Base(inst), early.Round(time.Millisecond), total.Round(time.Millisecond), snap.Hash, len(earlyDims), len(full.Dimensions()), earlyDims, tiles, mismatch, spawnSame, sum[:3])
	}
}
