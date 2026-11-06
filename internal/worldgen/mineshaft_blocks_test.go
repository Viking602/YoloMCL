package worldgen_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/mcworld"
	. "github.com/Viking602/YoloMCL/internal/worldgen"
)

type chunkCache struct {
	dir     string
	regions map[[2]int]*mcworld.Region
	chunks  map[[2]int]*mcworld.Chunk
}

func (c *chunkCache) block(x, y, z int) (string, bool) {
	ck := [2]int{x >> 4, z >> 4}
	ch, ok := c.chunks[ck]
	if !ok {
		rk := [2]int{ck[0] >> 5, ck[1] >> 5}
		r, ok := c.regions[rk]
		if !ok {
			r, _ = mcworld.OpenRegion(filepath.Join(c.dir, "region", fmt.Sprintf("r.%d.%d.mca", rk[0], rk[1])))
			c.regions[rk] = r
		}
		if r != nil {
			ch, _ = r.Chunk(ck[0]&31, ck[1]&31)
			if ch != nil && ch.Status != "minecraft:full" && ch.Status != "full" {
				ch = nil
			}
		}
		c.chunks[ck] = ch
	}
	if ch == nil {
		return "", false
	}
	return ch.Block(x&15, y, z&15), true
}

// TestMineshaftBlocksAgainstWorld checks the drawn corridor geometry (rail
// line, support posts and caps) against saved blocks in fully generated chunks.
func TestMineshaftBlocksAgainstWorld(t *testing.T) {
	dirs := os.Getenv("WORLDGEN_VERIFY")
	if dirs == "" {
		t.Skip("set WORLDGEN_VERIFY")
	}
	var railLine, railHit, railOff, railNoRailCorr, posts, postHit, caps, capHit int
	for _, dir := range splitList(dirs) {
		seed, err := mcworld.LevelSeed(dir)
		if err != nil {
			continue
		}
		cc := &chunkCache{dir: dir, regions: map[[2]int]*mcworld.Region{}, chunks: map[[2]int]*mcworld.Chunk{}}
		paths, _ := mcworld.RegionPaths(dir, "")
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
					for _, s := range c.Starts() {
						if s.ID != "minecraft:mineshaft" || len(s.Pieces) == 0 {
							continue
						}
						l := Mineshaft(seed, int32(s.ChunkX), int32(s.ChunkZ), true)
						l.MoveBelowSeaLevel(63, -64)
						for _, p := range l.Pieces {
							if p.Kind != MineshaftCorridor {
								continue
							}
							onLine := map[[3]int]bool{}
							for z := 0; z <= p.CorridorLength(); z++ {
								wx, wy, wz := p.World(1, 0, z)
								onLine[[3]int{wx, wy, wz}] = true
								b, ok := cc.block(wx, wy, wz)
								if !ok || !p.Rails {
									continue
								}
								railLine++
								if strings.HasSuffix(b, ":rail") {
									railHit++
								}
							}
							for z := 0; z <= p.CorridorLength(); z++ {
								for x := 0; x <= 2; x++ {
									wx, wy, wz := p.World(x, 0, z)
									b, ok := cc.block(wx, wy, wz)
									if !ok || !strings.HasSuffix(b, ":rail") {
										continue
									}
									if !p.Rails {
										railNoRailCorr++
									} else if !onLine[[3]int{wx, wy, wz}] {
										railOff++
									}
								}
							}
							for _, z := range p.SupportZ() {
								for _, x := range []int{0, 2} {
									for y := 0; y < 2; y++ {
										wx, wy, wz := p.World(x, y, z)
										if b, ok := cc.block(wx, wy, wz); ok {
											posts++
											if strings.HasSuffix(b, "_fence") {
												postHit++
											}
										}
									}
								}
								for x := 0; x <= 2; x++ {
									wx, wy, wz := p.World(x, 2, z)
									if b, ok := cc.block(wx, wy, wz); ok {
										caps++
										if strings.HasSuffix(b, "_planks") {
											capHit++
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	pct := func(a, b int) float64 { return 100 * float64(a) / float64(max(b, 1)) }
	t.Logf("rail line cells with a saved rail: %d/%d (%.1f%%); saved rails off the predicted line: %d; rails in no-rail corridors: %d", railHit, railLine, pct(railHit, railLine), railOff, railNoRailCorr)
	t.Logf("support posts are fences: %d/%d (%.1f%%); support caps are planks: %d/%d (%.1f%%)", postHit, posts, pct(postHit, posts), capHit, caps, pct(capHit, caps))
}
