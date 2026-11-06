package cubiomes

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"testing"
)

func TestTerraBlenderProfileOwnership(t *testing.T) {
	first, err := BuiltinTerraBlenderProfile("26.3", "26.3.0.0.15", Overworld)
	if err != nil {
		t.Fatal(err)
	}
	want := first.Regions[1].Parameters[0]
	first.Regions[1].Weight = 0
	first.Regions[1].Parameters[0].Biome = "test:changed"
	first.Regions[1].Parameters[0].Space[0][0]++
	second, err := BuiltinTerraBlenderProfile("26.3", "26.3.0.0.15", Overworld)
	if err != nil || second.Regions[1].Weight != 10 || second.Regions[1].Parameters[0] != want {
		t.Fatalf("caller mutated shared profile: %v", err)
	}
	end, err := BuiltinTerraBlenderEndProfile("26.3", "26.3.0.0.15")
	if err != nil {
		t.Fatal(err)
	}
	end.Highlands.Biomes[0].Weight = 0
	end, err = BuiltinTerraBlenderEndProfile("26.3", "26.3.0.0.15")
	if err != nil || end.Highlands.Biomes[0].Weight != 10 {
		t.Fatalf("caller mutated shared End weights: %v", err)
	}
	toggles, err := BuiltinTerraBlenderBiomeToggles("26.3", "26.3.0.0.15")
	if err != nil {
		t.Fatal(err)
	}
	toggles["auroral_garden_enabled"] = false
	toggles, err = BuiltinTerraBlenderBiomeToggles("26.3", "26.3.0.0.15")
	if err != nil || !toggles["auroral_garden_enabled"] {
		t.Fatalf("caller mutated shared biome toggles: %v", err)
	}
}

func TestTerraBlenderProfileVersionBoundary(t *testing.T) {
	for _, pair := range [][2]string{{"26.2", "26.3.0.0.15"}, {"26.3", "26.3.0.0.14"}, {"26.4", "26.3.0.0.15"}} {
		if _, err := BuiltinTerraBlenderProfile(pair[0], pair[1], Overworld); err == nil {
			t.Fatalf("unverified profile accepted: %v", pair)
		}
	}
}

func TestTerraBlenderBiomeToggleOracle(t *testing.T) {
	t.Run("26.3", func(t *testing.T) { testTerraBlenderBiomeToggleOracle(t, "26.3", "26.3.0.0.15", 86) })
	t.Run("1.21.1", func(t *testing.T) { testTerraBlenderBiomeToggleOracle(t, "1.21.1", "21.1.0.14", 92) })
}

func testTerraBlenderBiomeToggleOracle(t *testing.T, minecraft, bop string, count int) {
	file, err := os.Open("testdata/terrablender-bop-" + minecraft + "-toggles.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var fixture struct {
		Cases []struct {
			Toggles      map[string]bool   `json:"toggles"`
			RegionHashes map[string]string `json:"region_hashes"`
		} `json:"cases"`
	}
	if err := json.NewDecoder(z).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != count {
		t.Fatalf("incomplete reference fixture: %d configurations", len(fixture.Cases))
	}
	for i, sample := range fixture.Cases {
		for _, dimension := range []Dimension{Overworld, Nether} {
			cfg, err := BuiltinTerraBlenderProfileWithToggles(minecraft, bop, dimension, sample.Toggles)
			if err != nil {
				t.Fatal(err)
			}
			for _, region := range cfg.Regions {
				if region.Base {
					continue
				}
				hash := fnv.New64a()
				for _, parameter := range region.Parameters {
					hash.Write([]byte(parameter.Biome))
					hash.Write([]byte{'\n'})
				}
				if got, want := fmt.Sprintf("%016x", hash.Sum64()), sample.RegionHashes[region.ID]; got != want {
					t.Fatalf("config %d / %s: %s != public BOP reference %s", i, region.ID, got, want)
				}
			}
		}
	}
}
