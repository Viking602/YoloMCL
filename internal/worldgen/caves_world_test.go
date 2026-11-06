package worldgen_test

import (
	"os"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/worldgen"
)

func openBlock(b string) bool {
	switch b {
	case "minecraft:air", "minecraft:cave_air", "minecraft:water", "minecraft:lava":
		return true
	}
	return false
}

// TestCarversAgainstWorld checks predicted carver blocks against saved
// overworld chunks: predicted cells should be open (air/water/lava) far more
// often than the baseline, since carvers only fail on non-replaceable blocks,
// aquifer barriers and later features (ores, dripstone, lush cave plants...).
func TestCarversAgainstWorld(t *testing.T) {
	dirs := os.Getenv("WORLDGEN_CAVES")
	if dirs == "" {
		t.Skip("set WORLDGEN_CAVES")
	}
	for _, dir := range strings.Split(dirs, ";") {
		seed, err := mcworld.LevelSeed(dir)
		if err != nil {
			continue
		}
		control := os.Getenv("WORLDGEN_CAVES_CONTROL") != ""
		if control {
			seed++ // negative control: a wrong seed must drop to baseline
		}
		paths, _ := mcworld.RegionPaths(dir, "")
		cache := map[[2]int32][]CarveShape{}
		shapes := func(cx, cz int32) []CarveShape {
			k := [2]int32{cx, cz}
			if s, ok := cache[k]; ok {
				return s
			}
			s := OverworldCarves(seed, cx, cz)
			cache[k] = s
			return s
		}
		chunks, pred, predOpen, cells, open, caveAir, caveAirPred := 0, 0, 0, 0, 0, 0, 0
		for _, p := range paths {
			r, err := mcworld.OpenRegion(p)
			if err != nil {
				continue
			}
			for i := 0; i < 1024 && chunks < 400; i++ {
				c, _ := r.Chunk(i%32, i/32)
				if c == nil || !strings.HasSuffix(c.Status, "full") {
					continue
				}
				chunks++
				var local []*CarveShape
				box := Box{c.X * 16, -64, c.Z * 16, c.X*16 + 15, 320, c.Z*16 + 15}
				for dx := int32(-8); dx <= 8; dx++ {
					for dz := int32(-8); dz <= 8; dz++ {
						ss := shapes(int32(c.X)+dx, int32(c.Z)+dz)
						for j := range ss {
							if ss[j].Bounds().Intersects(box) {
								local = append(local, &ss[j])
							}
						}
					}
				}
				for y := -58; y <= 40; y++ {
					for z := 0; z < 16; z++ {
						for x := 0; x < 16; x++ {
							b := c.Block(x, y, z)
							o := openBlock(b)
							cells++
							if o {
								open++
							}
							carved := false
							for _, s := range local {
								if s.Carved(c.X*16+x, y, c.Z*16+z) {
									carved = true
									break
								}
							}
							if b == "minecraft:cave_air" {
								caveAir++
								if carved {
									caveAirPred++
								}
							}
							if carved {
								pred++
								if o {
									predOpen++
								}
							}
						}
					}
				}
			}
		}
		if open*2 < cells && !control {
			prec := float64(predOpen) / float64(max(pred, 1))
			if pred < 10000 || prec < 0.85 {
				t.Errorf("carver precision %.3f over %d cells too low", prec, pred)
			}
		}
		t.Logf("SAVE carvers %s: %d chunks, y -58..40; predicted carved %d cells, open in save %d (%.1f%%); baseline open %.1f%%; cave_air %d, predicted %d (%.1f%%)",
			dir, chunks, pred, predOpen, 100*float64(predOpen)/float64(max(pred, 1)), 100*float64(open)/float64(max(cells, 1)), caveAir, caveAirPred, 100*float64(caveAirPred)/float64(max(caveAir, 1)))
	}
}
