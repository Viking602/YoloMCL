package worldgen_test

import (
	"os"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/worldgen"
)

// TestMineshaftChunksAgainstWorld checks the legacy_type_3 frequency rule
// (0.004) against every saved chunk: predicted vs saved mineshaft starts.
func TestMineshaftChunksAgainstWorld(t *testing.T) {
	dirs := os.Getenv("WORLDGEN_VERIFY")
	if dirs == "" {
		t.Skip("set WORLDGEN_VERIFY")
	}
	for _, dir := range splitList(dirs) {
		seed, err := mcworld.LevelSeed(dir)
		if err != nil {
			t.Fatal(err)
		}
		paths, _ := mcworld.RegionPaths(dir, "")
		chunks, tp, fp, fn := 0, 0, 0, 0
		for _, p := range paths {
			r, err := mcworld.OpenRegion(p)
			if err != nil {
				continue
			}
			for lz := 0; lz < 32; lz++ {
				for lx := 0; lx < 32; lx++ {
					c, _ := r.Chunk(lx, lz)
					if c == nil {
						continue
					}
					chunks++
					saved := false
					for _, s := range c.Starts() {
						if s.ID == "minecraft:mineshaft" || s.ID == "minecraft:mineshaft_mesa" {
							saved = true
						}
					}
					cx, cz := int32(c.X), int32(c.Z)
					pred, _ := FrequencyPasses("legacy_type_3", seed, 0, cx, cz, 0.004)
					switch {
					case pred && saved:
						tp++
					case pred:
						fp++
					case saved:
						fn++
					}
				}
			}
		}
		t.Logf("%s: %d chunks, mineshaft starts predicted&saved %d, predicted only %d, saved only %d", dir, chunks, tp, fp, fn)
	}
}
