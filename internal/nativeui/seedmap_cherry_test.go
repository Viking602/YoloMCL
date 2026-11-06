package nativeui

import (
	"os"
	"testing"
)

// TestSeedMapCherryGroveLive: on the live 1.21.1 jar (seed 0), the map's
// sampler must report cherry_grove where cubiomes does (-4032, -1792), list
// it among the biomes, and give it its own colour.
func TestSeedMapCherryGroveLive(t *testing.T) {
	const jar = `D:\MC\.minecraft\versions\1.21.1\1.21.1.jar`
	if _, err := os.Stat(jar); err != nil {
		t.Skip("live 1.21.1 game jar required")
	}
	a, tt, saves := worldsTester(t)
	copyLiveJar(t, jar, saves)
	a.OverrideLanguage(LangZH)
	tab := a.L("instance.tab_prefix") + " " + a.L("seedmap.open")
	waitUI(t, tt, func() bool { _, ok := tt.Find(tab); return ok })
	clickUI(t, tt, tab)
	waitUI(t, tt, func() bool { return a.seedMap != nil && a.seedMap.mapReady() && !a.seedMap.pending })
	v := a.seedMap
	t.Logf("seed %d version %v coverage %+v", v.seed, v.snapshot.Minecraft, v.sampler.Coverage("minecraft:overworld"))
	id, ok := v.sampler.BiomeAt("minecraft:overworld", -4032, 100, -1792)
	t.Logf("biome at -4032,100,-1792: %q %v", id, ok)
	listed := false
	for _, b := range v.sampler.Biomes("minecraft:overworld") {
		if b.ID == "minecraft:cherry_grove" {
			listed = true
			t.Logf("listed: %+v colour %v", b, v.sampler.BiomeColor(b.ID))
		}
	}
	if c := v.sampler.BiomeColor("minecraft:cherry_grove"); c.R < c.G+30 || c == v.sampler.BiomeColor("minecraft:plains") {
		t.Fatalf("cherry_grove colour %v is not pink: lost among plains", c)
	}
	if id != "minecraft:cherry_grove" || !listed {
		t.Fatalf("cherry_grove: got %q listed=%v", id, listed)
	}
}
