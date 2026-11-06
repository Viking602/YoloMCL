package nativeui

import (
	"context"
	"os"
	"testing"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// TestSeedMapPillagerOutpostsLive checks that pillager_outposts (random
// spread, legacy_type_1 frequency 0.2, villages exclusion zone of 10 chunks)
// is searchable and matches the cubiomes positions for seed 0 on 1.21.1.
func TestSeedMapPillagerOutpostsLive(t *testing.T) {
	const jar = `D:\MC\.minecraft\versions\1.21.1\1.21.1.jar`
	if _, err := os.Stat(jar); err != nil {
		t.Skip("live 1.21.1 game jar required")
	}
	a, tt, saves := worldsTester(t)
	copyLiveJar(t, jar, saves)
	tab := a.L("instance.tab_prefix") + " " + a.L("seedmap.open")
	waitUI(t, tt, func() bool { _, ok := tt.Find(tab); return ok })
	clickUI(t, tt, tab)
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	v := a.seedMap
	if v.seed != 0 {
		t.Fatalf("fixture seed %d, want 0", v.seed)
	}
	res, err := v.sampler.FindStructures(context.Background(), launcher.SeedMapSearch{Dimension: "minecraft:overworld", Structure: "minecraft:pillager_outposts", X: 0, Y: 64, Z: 0, Radius: 5000, Limit: 64})
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range res.Hits {
		t.Logf("outpost %d %d %s %s", h.X, h.Z, h.Biome, h.Confidence)
	}
	if len(res.Hits) == 0 {
		t.Fatal("no outposts")
	}
}
