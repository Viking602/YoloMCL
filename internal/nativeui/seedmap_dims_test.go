package nativeui

import (
	"context"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// TestSeedMapDiveDimensionShots dives into a village (with its neighbours on
// the same slab), a Nether fortress or bastion and an End city on the live
// 1.21.1 jar, and records the dive's frame times.
func TestSeedMapDiveDimensionShots(t *testing.T) {
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
	tt.SetSize(1440, 900)
	tt.Frame()
	find := func(dim, set string, x, z int, want string) []launcher.FeatureHit {
		res, err := v.sampler.FindStructures(context.Background(), launcher.SeedMapSearch{Dimension: dim, Structure: set, X: x, Y: 64, Z: z, Radius: 6000, Limit: 32})
		if err != nil {
			t.Logf("%s %s: %v", dim, set, err)
			return nil
		}
		var out []launcher.FeatureHit
		for _, h := range res.Hits {
			if want == "" || strings.Contains(h.ID, want) {
				out = append(out, h)
			}
		}
		return out
	}
	cases := []struct {
		name, dim string
		sets      []string
		want      string
		x, z      int
	}{
		{"village", "minecraft:overworld", []string{"minecraft:villages"}, "village", 0, 0},
		{"fortress", "minecraft:the_nether", []string{"minecraft:nether_complexes"}, "", 0, 0},
		{"bastion", "minecraft:the_nether", []string{"minecraft:nether_complexes"}, "", 0, 0},
		{"end-city", "minecraft:the_end", []string{"minecraft:end_cities"}, "", 1600, 0},
	}
	for _, c := range cases {
		if v.viewport.Dimension != c.dim {
			a.seedMapDimension(v, c.dim)
			waitUI(t, tt, func() bool { return !v.pending && v.frame.Viewport.Dimension == c.dim })
		}
		var hits []launcher.FeatureHit
		for _, s := range c.sets {
			hits = append(hits, find(c.dim, s, c.x, c.z, c.want)...)
		}
		if len(hits) == 0 {
			t.Fatalf("%s: no hit", c.name)
		}
		var hit launcher.FeatureHit
		var p *seedMapStructurePreview
		for i, h := range hits {
			hit = h
			if c.name == "fortress" || c.name == "bastion" {
				store, err := a.service.StructureAssetStoreWorld(v.rootID, v.versionID, v.world)
				if err != nil {
					t.Fatal(err)
				}
				id, _ := resolveHitTarget(store, h, v.seed, c.dim, v.sampler)
				store.Close()
				if !strings.Contains(id, c.name) {
					continue
				}
				t.Logf("%s at %d,%d resolves to %s", h.ID, h.X, h.Z, id)
			}
			if c.name == "village" {
				// the map's other surface markers around the village
				for _, st := range v.sampler.Structures(c.dim) {
					s := st.ID
					if !strings.Contains(s, "village") && !strings.Contains(s, "outpost") && !strings.Contains(s, "pyramid") && !strings.Contains(s, "portal") && !strings.Contains(s, "hut") && !strings.Contains(s, "igloo") && !strings.Contains(s, "temple") {
						continue
					}
					v.hits = append(v.hits, find(c.dim, s, hit.X, hit.Z, "")...)
				}
			}
			v.selected = &hit
			v.structurePreview = nil
			tt.Frame()
			clickUI(t, tt, a.L("seedmap.structure_preview"))
			waitUI(t, tt, func() bool { p := v.structurePreview; return p != nil && !p.loading })
			p = v.structurePreview
			if p.terrain && p.err == "" {
				break
			}
			t.Logf("%s %s at %d,%d: unsupported=%q err=%q, next", c.name, hit.ID, hit.X, hit.Z, p.unsupported, p.err)
			if i == len(hits)-1 || i > 12 {
				t.Fatalf("%s: no previewable hit", c.name)
			}
		}
		settle := func() {
			for i := 0; v.structurePreview == p && (p.rendering || p.dirty); i++ {
				if i > 1000 {
					t.Fatal("render did not settle")
				}
				tt.Frame()
				time.Sleep(10 * time.Millisecond)
			}
			tt.Frame()
		}
		worst := 0.0
		for i := 0; i < 14 && p.diving; i++ {
			tt.Frame()
			settle()
			worst = math.Max(worst, p.frameMs)
			if i == 4 {
				snapshotUI(t, tt, "seedmap-dim-"+c.name+"-mid")
			}
			tt.Advance(diveDuration / 10)
		}
		settle()
		t.Logf("%s %s at %d,%d: Y %d..%d, terrain top %d, dive worst frame %.1f ms, faces %d, blocks %d",
			c.name, hit.ID, hit.X, hit.Z, p.structMinY, p.structMaxY, p.terrainTop, worst, p.mesh.Faces(), p.mesh.Blocks)
		snapshotUI(t, tt, "seedmap-dim-"+c.name+"-arrived")
		v.structurePreview = nil
		tt.Frame()
	}
}
