package cubiomes

import (
	"context"
	"testing"
)

// TestCherryGroveGenerates guards cherry_grove in the 1.20+ overworld trees:
// it must appear on a plain vanilla scan (seed 0) and not in 1.19.
func TestCherryGroveGenerates(t *testing.T) {
	for _, tc := range []struct {
		version string
		want    bool
	}{{"1.19.4", false}, {"1.20.1", true}, {"1.21.1", true}, {"1.21.4", true}} {
		g, err := New(context.Background(), Config{Version: tc.version, Seed: 0, Dimension: Overworld})
		if err != nil {
			t.Fatal(err)
		}
		s := g.NewSampler()
		n, first := 0, [2]int{}
		for x := -4096; x < 4096; x += 64 {
			for z := -4096; z < 4096; z += 64 {
				if id, ok := s.BiomeAt(x, 100, z); ok && id == "minecraft:cherry_grove" {
					if n == 0 {
						first = [2]int{x, z}
					}
					n++
				}
			}
		}
		t.Logf("%s: %d cherry_grove samples, first at %v", tc.version, n, first)
		if (n > 0) != tc.want {
			t.Fatalf("%s: cherry_grove samples %d, want present=%v", tc.version, n, tc.want)
		}
	}
}
