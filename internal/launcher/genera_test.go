package launcher

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestClassifyEra(t *testing.T) {
	for _, c := range []struct {
		exact  []int
		n      int
		region bool
		era    int
		ok     bool
	}{
		{[]int{900, 100}, 1000, true, 0, true},
		{[]int{100, 950}, 1000, true, 1, true},
		{[]int{800, 700}, 1000, true, 0, false}, // under 85%
		{[]int{900, 700}, 1000, true, 0, false}, // lead under 30
		{[]int{60, 0}, 60, true, 0, false},      // too few samples
		{[]int{3, 1}, 4, false, 0, true},
		{[]int{1, 4}, 4, false, 1, true},
		{[]int{4, 2}, 4, false, 0, false},
		{[]int{2, 2}, 4, false, 0, false},
		{[]int{0, 0}, 0, false, 0, false},
	} {
		era, ok := classifyEra(c.exact, c.n, c.region)
		if ok != c.ok || (ok && era != c.era) {
			t.Errorf("classifyEra(%v,%d,%v) = %d,%v want %d,%v", c.exact, c.n, c.region, era, ok, c.era, c.ok)
		}
	}
	if !eraExcluded("Lithostitched-1.4.11-neoforge.jar", []string{"lithostitched"}) || eraExcluded("create.jar", []string{"lithostitched"}) {
		t.Error("eraExcluded")
	}
	day := func(d int) time.Time { return time.Date(2026, 9, d, 0, 0, 0, 0, time.UTC) }
	eras := eraCandidates([]eraJar{{"tectonic-3.0.jar", day(14)}, {"Lithostitched-x.jar", day(14)}, {"a.jar", day(1)},
		{"[x] kaleidoscope_end-1.jar", day(12)}, {"kaleidoscopecookery-1.6.jar", day(30)}, {"Tsuki-1.7.0.jar", day(24)}, {"ab-2.jar", day(20)}})
	if len(eras) != 2 || strings.Join(eras[1].Without, ",") != "tectonic,lithostitched,tsuki-1.7.0.jar,ab-2.jar" {
		t.Errorf("eraCandidates = %+v", eras)
	}
	if len(eraCandidates([]eraJar{{"a.jar", day(1)}})) != 1 {
		t.Error("eraCandidates without worldgen mods")
	}
}

// MCL_ERA_DETECT="inst|world;..." prints the detected era map of saves.
func TestDetectErasWorld(t *testing.T) {
	spec := os.Getenv("MCL_ERA_DETECT")
	if spec == "" {
		t.Skip("set MCL_ERA_DETECT")
	}
	s := testService(t)
	for _, item := range strings.Split(spec, ";") {
		inst, world, _ := strings.Cut(item, "|")
		root := Root{ID: "et", Path: filepath.Dir(filepath.Dir(inst)), SelectedVersion: filepath.Base(inst)}
		s.cfg.Roots = []Root{root}
		m, err := s.DetectEras(context.Background(), root.ID, root.SelectedVersion, world)
		if err != nil {
			t.Fatalf("%s: %v", world, err)
		}
		t.Logf("%s: eras %+v", world, m.Eras)
		var keys [][2]int
		for k := range m.Regions {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i][0]*1000+keys[i][1] < keys[j][0]*1000+keys[j][1] })
		for _, k := range keys {
			t.Logf("  r.%d.%d %+v", k[0], k[1], m.Regions[k])
		}
		count := map[[2]int]int{}
		for _, v := range m.Chunks {
			approx := 0
			if v.Approx {
				approx = 1
			}
			count[[2]int{v.Era, approx}]++
		}
		t.Logf("  mixed-region chunks: %v", count)
	}
}
