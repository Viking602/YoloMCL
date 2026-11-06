package launcher

import (
	"math/rand"
	"testing"

	"github.com/Viking602/YoloMCL/cubiomes"
	"github.com/Viking602/YoloMCL/cubiomes/modjar"
)

// Without latest.log the region order must not depend on the order region
// programs were collected in.
func TestRegionFallbackOrderDeterministic(t *testing.T) {
	mk := func(id, file string) WorldgenRegionProgram {
		return WorldgenRegionProgram{ModID: id, Source: "mod:" + file, Result: &modjar.Result{Regions: []modjar.Registration{{ID: id + ":r", Dimension: cubiomes.Overworld, Weight: 1}}}}
	}
	base := []WorldgenRegionProgram{mk("b", "B.jar"), mk("a", "c.jar"), mk("c", "a.jar"), mk("d", "a.jar"), mk("e", "z.jar")}
	mods := []WorldgenMod{{ID: "e", Before: []string{"c"}}}
	want := ""
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 50; i++ {
		progs := append([]WorldgenRegionProgram(nil), base...)
		r.Shuffle(len(progs), func(a, b int) { progs[a], progs[b] = progs[b], progs[a] })
		got, err := forgeOrderRegionPrograms(&WorldgenSnapshot{Mods: mods, RegionPrograms: progs})
		if err != nil {
			t.Fatal(err)
		}
		s := ""
		for _, p := range got {
			s += p.ModID
		}
		if want == "" {
			want = s
			if s != "dbaec" {
				t.Fatalf("order %s, want dbaec", s)
			}
		} else if s != want {
			t.Fatalf("order %s differs from %s", s, want)
		}
	}
	regions := []cubiomes.TerraBlenderRegion{{ID: "minecraft:overworld"}, {ID: "x:r"}}
	if applyTerraBlenderRegionLog(&WorldgenSnapshot{}, cubiomes.Overworld, regions) {
		t.Fatal("applied without log")
	}
	partial := &WorldgenSnapshot{ModConfigs: map[string][]byte{terraBlenderRegionLogKey: []byte("OVERWORLD minecraft:overworld 0\n")}}
	if applyTerraBlenderRegionLog(partial, cubiomes.Overworld, regions) {
		t.Fatal("applied incomplete log")
	}
}
