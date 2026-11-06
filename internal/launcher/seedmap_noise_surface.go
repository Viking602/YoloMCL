package launcher

import (
	"context"
	"image/color"
	"math"
	"runtime"
	"strings"
	"sync"

	"github.com/Viking602/YoloMCL/cubiomes"
)

// Relief for dimensions without the Overworld depth graph (the Nether, the
// End and modded noise dimensions), read from the seed's final_density.
//
// Roofed dimensions (a solid top layer, like the Nether) show the floor below
// the chosen Y slice: the first non-air block under the first air below Y,
// lava seas included. Open dimensions (the End) show the top solid block;
// columns with none are void, stored as NaN height.

// noiseSurface reports whether relief comes from final_density.
func (d seedMapDimension) noiseSurface() bool {
	return d.terrain != nil && d.engine != nil && !d.engine.SupportsSurface()
}

// noiseSurfaceHeight is the displayed ground Y at x, z and its substance;
// ok is false over the void.
func noiseSurfaceHeight(t *cubiomes.TerrainSampler, minY, height, sliceY, x, z int) (int, cubiomes.Substance, bool) {
	top := minY + height - 1
	air := top
	if t.Block(x, top, z) == cubiomes.Solid {
		// roofed: start at the slice and leave the solid it sits in
		start := max(minY, min(sliceY, top))
		air = start
		for air >= minY && t.Block(x, air, z) != cubiomes.Air {
			air--
		}
		if air < minY {
			// solid all the way down: the ground is at the slice
			return start, cubiomes.Solid, true
		}
	} else if b := t.Block(x, top, z); b != cubiomes.Air {
		return top, b, true
	}
	// air is a known air block. Step down 4 at a time, then read the
	// last bracket block by block from the top.
	for air-4 >= minY && t.Block(x, air-4, z) == cubiomes.Air {
		air -= 4
	}
	for y := air - 1; y >= max(minY, air-4); y-- {
		if b := t.Block(x, y, z); b != cubiomes.Air {
			return y, b, true
		}
	}
	return 0, cubiomes.Air, false
}

// seedMapNoiseSurface fills a haloed height raster (NaN = void) and a
// matching lava mask for one tile from final_density.
func (s *LocalSampler) seedMapNoiseSurface(ctx context.Context, d seedMapDimension, key SeedMapTileKey, x, z int, ids []cubiomes.BiomeID) ([]cubiomes.SurfaceSample, []bool, error) {
	out := make([]cubiomes.SurfaceSample, seedMapSurfaceWidth*seedMapSurfaceWidth)
	lava := make([]bool, len(out))
	minY, height := d.terrain.MinY, d.terrain.Height
	rows := make(chan int)
	var wg sync.WaitGroup
	for range max(1, min(8, runtime.NumCPU()-1)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t := d.terrainPool.Get().(*cubiomes.TerrainSampler)
			defer d.terrainPool.Put(t)
			for r := range rows {
				if ctx.Err() != nil {
					continue
				}
				bz := z + (r-1)*key.BlocksPerPixel
				for c := range seedMapSurfaceWidth {
					bx := x + (c-1)*key.BlocksPerPixel
					i := r*seedMapSurfaceWidth + c
					// halo cells take the nearest inner biome
					ix, iz := max(0, min(SeedMapTilePixels-1, c-1)), max(0, min(SeedMapTilePixels-1, r-1))
					out[i].Biome = ids[iz*SeedMapTilePixels+ix]
					h, sub, ok := noiseSurfaceHeight(t, minY, height, key.Y, bx, bz)
					if !ok {
						out[i].Height = float32(math.NaN())
						continue
					}
					out[i].Height, lava[i] = float32(h), sub == cubiomes.Lava
				}
			}
		}()
	}
	for r := range seedMapSurfaceWidth {
		rows <- r
	}
	close(rows)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return out, lava, nil
}

var (
	seedMapVoid = color.RGBA{11, 11, 18, 255}
	seedMapLava = color.RGBA{207, 92, 16, 255}
)

// seedMapNoiseTerrainBase is a top-block colour for Nether / End ground.
func seedMapNoiseTerrainBase(id string, lava bool) color.RGBA {
	if lava {
		return seedMapLava
	}
	name := id[strings.LastIndex(id, ":")+1:]
	switch {
	case strings.Contains(name, "soul_sand"):
		return color.RGBA{81, 62, 50, 255}
	case strings.Contains(name, "basalt"):
		return color.RGBA{80, 80, 85, 255}
	case strings.Contains(name, "crimson"):
		return color.RGBA{130, 31, 31, 255}
	case strings.Contains(name, "warped"):
		return color.RGBA{43, 114, 111, 255}
	case strings.Contains(name, "end"):
		return color.RGBA{219, 222, 158, 255}
	}
	return color.RGBA{111, 54, 53, 255} // netherrack
}

// noiseSurfaceAt is SurfaceHeightAt for final_density dimensions, from the
// top of the build height (the highest floor under a roof).
func (d seedMapDimension) noiseSurfaceAt(x, z int) (int, bool) {
	t := d.terrainPool.Get().(*cubiomes.TerrainSampler)
	defer d.terrainPool.Put(t)
	h, _, ok := noiseSurfaceHeight(t, d.terrain.MinY, d.terrain.Height, d.terrain.MinY+d.terrain.Height-1, x, z)
	return h, ok
}

// seedMapNoiseShade colours one final_density relief pixel: void stays void,
// the terrain toggle swaps the biome colour for a top-block colour, then the
// shared hillshade and contours apply.
func seedMapNoiseShade(c color.RGBA, id string, cells []cubiomes.SurfaceSample, lava []bool, x, z, step int, terrain, contours bool) color.RGBA {
	i := z*seedMapSurfaceWidth + x
	if math.IsNaN(float64(cells[i].Height)) {
		return seedMapVoid
	}
	if terrain || lava[i] {
		c = seedMapNoiseTerrainBase(id, lava[i])
	}
	return seedMapRelief(c, cells, x, z, step, contours)
}
