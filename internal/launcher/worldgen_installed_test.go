package launcher

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// An opt-in end-to-end regression against a local game save. The case file
// contains independently observed game coordinates, not generated Go answers.
// Only a temporary launcher's cache/config is used; installed files are read-only.
func TestWorldgenInstalledSave(t *testing.T) {
	file := os.Getenv("MCL_WORLDGEN_SAVE_CASE")
	if file == "" {
		t.Skip("MCL_WORLDGEN_SAVE_CASE names an external installed-save case")
	}
	var fixture struct {
		World, Seed      string
		Noise            bool
		AllowClimateTies bool
		Points           []struct {
			Dimension string
			X, Y, Z   int
			Biome     string
		}
		Sources          map[string]string
		AbsentStructures []struct{ Dimension, ID string }
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	world, err := filepath.Abs(fixture.World)
	if err != nil || filepath.Base(filepath.Dir(world)) != "saves" {
		t.Fatal("expected an absolute saves/world directory", err)
	}
	instance := filepath.Dir(filepath.Dir(world))
	root := Root{ID: "worldgen-oracle", Path: instance}
	if filepath.Base(filepath.Dir(instance)) == "versions" {
		root.Path, root.SelectedVersion = filepath.Dir(filepath.Dir(instance)), filepath.Base(instance)
	}
	s := testService(t)
	s.cfg.Roots = []Root{root}
	snapshot, level, err := s.SeedMapWorldgen(context.Background(), root.ID, "", filepath.Base(world))
	if err != nil {
		t.Fatal(err)
	}
	if !level.HasRandomSeed || strconv.FormatInt(level.RandomSeed, 10) != fixture.Seed {
		t.Fatal("save seed differs from game observation")
	}
	if hasWorldgenIssue(snapshot, "source_unreadable") {
		t.Fatalf("installed resource load failed: %+v", snapshot.Issues)
	}
	for key, source := range fixture.Sources {
		kind, id, ok := strings.Cut(key, "/")
		if !ok || worldgenDoc(t, snapshot, kind, id).Source != source {
			t.Fatalf("resource precedence differs: %s", key)
		}
	}
	sampler, err := NewLocalSampler(context.Background(), snapshot, level.RandomSeed)
	if err != nil {
		t.Fatal(err)
	}
	mismatches, ties := 0, 0
	counts := map[string]int{}
	for _, p := range fixture.Points {
		var got string
		var ok bool
		if fixture.Noise {
			d := sampler.dimensions[p.Dimension]
			if d.engine != nil {
				got, ok = d.engine.NewSampler().NextNoiseBiome(p.X, p.Y, p.Z)
			}
		} else {
			got, ok = sampler.BiomeAt(p.Dimension, p.X, p.Y, p.Z)
		}
		counts[p.Biome]++
		if !ok || got != p.Biome {
			if fixture.Noise && fixture.AllowClimateTies {
				d := sampler.dimensions[p.Dimension]
				if d.engine != nil {
					alternatives, _ := d.engine.NewSampler().NearestNoiseBiomes(p.X, p.Y, p.Z)
					for _, alternative := range alternatives {
						if alternative == p.Biome {
							ties++
							ok = true
							got = p.Biome
							break
						}
					}
				}
			}
			if ok && got == p.Biome {
				continue
			}
			mismatches++
			if mismatches <= 15 {
				t.Logf("%s (%d,%d,%d): %s, game %s", p.Dimension, p.X, p.Y, p.Z, got, p.Biome)
			}
		}
	}
	if mismatches > 0 {
		t.Errorf("%d / %d saved biome samples differ", mismatches, len(fixture.Points))
	}
	t.Logf("strict matches=%d; equal-distance boundary alternatives=%d; mismatches=%d", len(fixture.Points)-ties-mismatches, ties, mismatches)
	t.Logf("saved biome distribution: %v", counts)
	for _, absent := range fixture.AbsentStructures {
		for _, structure := range sampler.Structures(absent.Dimension) {
			if structure.ID == absent.ID {
				t.Errorf("disabled structure still displayed: %+v", absent)
			}
		}
	}
	if output := os.Getenv("MCL_WORLDGEN_SAVE_EXPORT"); output != "" {
		data, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("Minecraft %s / %s: %d mod identities, %d resources, %d observed biome points", snapshot.Minecraft, snapshot.Loader, len(snapshot.Mods), len(snapshot.Documents), len(fixture.Points))
}
