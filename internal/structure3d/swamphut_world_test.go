package structure3d_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/structure3d"
	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// TestSwampHutBlocksAgainstWorld compares the drawn hut with the saved one.
func TestSwampHutBlocksAgainstWorld(t *testing.T) {
	dir := `D:\MC\.minecraft\versions\SF Integrated Modpack\saves\新的世界`
	r, err := mcworld.OpenRegion(dir + `\region\r.-1.0.mca`)
	if err != nil {
		t.Skip(err)
	}
	box := worldgen.Box{-256, 63, 320, -250, 69, 328}
	pred := SwampHutBlocks(box, "north")
	hit, n := 0, 0
	var miss []string
	for k, v := range pred {
		if k[1] < 63 {
			continue
		}
		c, _ := r.Chunk((k[0]>>4)&31, (k[2]>>4)&31)
		if c == nil {
			continue
		}
		got := c.Block(k[0]&15, k[1], k[2]&15)
		n++
		if strings.TrimSuffix(got, "") == v.Name {
			hit++
		} else if len(miss) < 12 {
			miss = append(miss, fmt.Sprint(k, " ", v.Name, " saved ", got))
		}
	}
	t.Logf("swamp hut blocks: %d/%d match; misses %v", hit, n, miss)
}
