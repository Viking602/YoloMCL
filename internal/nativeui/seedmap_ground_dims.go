package nativeui

import (
	"math"
	"strings"

	"github.com/Viking602/YoloMCL/internal/launcher"

	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// Nether and End ground for the 3D preview.
//
// End: island columns come from vanilla's end_islands density function for
// the seed (exact), shaped by the End noise settings' two slides — top:
// y_clamped_gradient 56→312 towards -23.4375, bottom: 4→32 towards
// -0.234375 — so a column is end stone where (h-8)/128 beats both slides,
// void elsewhere. The small base_3d_noise (old_blended_noise) term is not
// implemented, so island edges are smooth: approximate.
//
// Nether: the floor, the lava sea at Y 31 (vanilla sea level), the bedrock
// floor and ceiling rolls and per-biome lake beds (soul sand valley, basalt
// deltas) follow vanilla. Nether relief needs the full final_density graph
// (old_blended_noise), which the tree does not have yet, so the floor is
// drawn at the lava sea and the ceiling as a flat netherrack layer from
// Y 120: approximate.

const (
	netherLava    = 31  // vanilla nether sea level (lava)
	netherCeiling = 120 // drawn ceiling floor; relief is not generated
	netherTop     = 127
)

var (
	groundNetherrack = [3]uint8{0x6f, 0x36, 0x35}
	groundSoulSand   = [3]uint8{0x51, 0x3e, 0x32}
	groundBasalt     = [3]uint8{0x50, 0x50, 0x55}
	groundBlackstone = [3]uint8{0x2a, 0x24, 0x2b}
	groundEndStone   = [3]uint8{0xdb, 0xde, 0x9e}
	groundVoid       = [3]uint8{0x0b, 0x0b, 0x12}
)

func endTop(d float64) int    { return int(math.Floor(56 + 256*d/(d+23.4375))) }
func endBottom(d float64) int { return int(math.Ceil(4 + 28*0.234375/(d+0.234375))) }

func (g *previewGround) endDensity(x, z int) float64 {
	k := [2]int{x, z}
	if d, ok := g.endD[k]; ok {
		return d
	}
	d := g.end.Density(x, z)
	g.endD[k] = d
	return d
}

// EndHeight is the island top at a column, or TerrainVoid.
func (g *previewGround) EndHeight(x, z int) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.noise {
		if c := g.column(x, z); c.solidTop >= g.info.MinY {
			return c.solidTop
		}
		return structure3d.TerrainVoid
	}
	if d := g.endDensity(x, z); d > 0 {
		return endTop(d)
	}
	return structure3d.TerrainVoid
}

// EndFloor is the island underside at a column.
func (g *previewGround) EndFloor(x, z int) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.noise {
		c := g.column(x, z)
		for i, s := range c.sub {
			if s == launcher.TerrainSolid {
				return g.info.MinY + i
			}
		}
		return 0
	}
	if d := g.endDensity(x, z); d > 0 {
		return endBottom(d)
	}
	return 0
}

// EndSurface is the placement height structures use in the End: the
// island top, or 0 over the void.
func (g *previewGround) EndSurface(x, z int) int {
	if h := g.EndHeight(x, z); h != structure3d.TerrainVoid {
		return h
	}
	return 0
}

// NetherFloor is the top of the Nether floor (lava sea included) and
// NetherCeiling the first solid block above it, from final_density.
func (g *previewGround) NetherFloor(x, z int) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.column(x, z).floorTop
}

func (g *previewGround) NetherCeiling(x, z int) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.column(x, z).ceilY
}

// NetherCeilingTop is the ceiling layer's top, or no layer at all when the
// column has no ceiling.
func (g *previewGround) NetherCeilingTop(x, z int) int {
	if g.NetherCeiling(x, z) == structure3d.TerrainVoid {
		return structure3d.TerrainVoid
	}
	return netherTop
}

// NetherNoiseTop colours a floor or ceiling top from the noise column.
func (g *previewGround) NetherNoiseTop(x, z, h int) [3]uint8 {
	g.mu.Lock()
	defer g.mu.Unlock()
	if s, ok := g.substance(x, h, z); ok && s == launcher.TerrainLava {
		return groundLava
	}
	return g.netherSolid(x, h, z, true)
}

// netherSolid is a solid Nether block: bedrock rolls, valley / delta
// floors, netherrack.
func (g *previewGround) netherSolid(x, y, z int, surface bool) [3]uint8 {
	// the dimension's surface_rule (nylium, soul sand / soil, basalt,
	// blackstone, gravel, bedrock rolls), textured from the modpack
	if c, ok := g.ruleColor(x, y, z, ""); ok {
		return c
	}
	if y <= 0 || y >= netherTop {
		return groundBedrock
	}
	if y < 5 && int(g.roll(x, y, z, 1)%5) >= y {
		return groundBedrock
	}
	if y > netherTop-5 && int(g.roll(x, y, z, 4)%5) >= netherTop-y {
		return groundBedrock
	}
	if surface {
		id := g.biome(x, y, z)
		switch {
		case strings.Contains(id, "soul_sand_valley"):
			return groundSoulSand
		case strings.Contains(id, "basalt_deltas"):
			if g.roll(x, y, z, 5)%2 == 0 {
				return groundBlackstone
			}
			return groundBasalt
		}
	}
	return groundNetherrack
}

func (g *previewGround) netherSection(x, y, z int) [3]uint8 {
	if g.noise {
		s, _ := g.substance(x, y, z)
		switch s {
		case launcher.TerrainLava:
			return groundLava
		case launcher.TerrainAir, launcher.TerrainWater:
			return groundCave
		}
		above, _ := g.substance(x, y+1, z)
		return g.netherSolid(x, y, z, above != launcher.TerrainSolid)
	}
	if y <= 0 || y >= netherTop {
		return groundBedrock
	}
	// Bedrock floor Y 0..4 and roof 123..127 roll like the overworld floor.
	if y < 5 && int(g.roll(x, y, z, 1)%5) >= y {
		return groundBedrock
	}
	if y > netherTop-5 && int(g.roll(x, y, z, 4)%5) >= netherTop-y {
		return groundBedrock
	}
	if y > netherLava-4 && y <= netherLava {
		return groundLava
	}
	if y > netherLava-8 && y <= netherLava-4 { // lake bed under the lava sea
		id := g.biome(x, netherLava, z)
		switch {
		case strings.Contains(id, "soul_sand_valley"):
			return groundSoulSand
		case strings.Contains(id, "basalt_deltas"):
			if g.roll(x, y, z, 5)%2 == 0 {
				return groundBlackstone
			}
			return groundBasalt
		}
	}
	return groundNetherrack
}

// NetherTop colours the floor (the lava sea) and the ceiling's top.
func (g *previewGround) NetherTop(_, _, h int) [3]uint8 {
	if h <= netherLava {
		return groundLava
	}
	return groundNetherrack
}

func (g *previewGround) endSection(x, y, z int) [3]uint8 {
	if g.noise {
		g.mu.Lock()
		s, _ := g.substance(x, y, z)
		g.mu.Unlock()
		if s == launcher.TerrainSolid {
			return groundEndStone
		}
		return groundVoid
	}
	if h := g.EndHeight(x, z); h == structure3d.TerrainVoid || y > h || y < g.EndFloor(x, z) {
		return groundVoid
	}
	return groundEndStone
}

// Vanilla grass colours (grass colormap at each biome's temperature and
// downfall, plus the fixed biome overrides). Swamp and dark forest use
// their mean colour: approximate.
var (
	grassPlains = [3]uint8{0x91, 0xbd, 0x59}
	biomeGrass  = map[string][3]uint8{
		"forest": {0x79, 0xc0, 0x5a}, "flower_forest": {0x79, 0xc0, 0x5a},
		"birch_forest": {0x88, 0xbb, 0x67}, "old_growth_birch_forest": {0x88, 0xbb, 0x67},
		"dark_forest": {0x50, 0x7a, 0x32}, "pale_garden": {0x77, 0x82, 0x72},
		"taiga": {0x86, 0xb7, 0x83}, "old_growth_pine_taiga": {0x86, 0xb8, 0x7f}, "old_growth_spruce_taiga": {0x86, 0xb8, 0x7f},
		"snowy_taiga": {0x80, 0xb4, 0x97}, "snowy_plains": {0x80, 0xb4, 0x97}, "ice_spikes": {0x80, 0xb4, 0x97},
		"grove": {0x80, 0xb4, 0x97}, "snowy_slopes": {0x80, 0xb4, 0x97}, "frozen_peaks": {0x80, 0xb4, 0x97},
		"jagged_peaks": {0x80, 0xb4, 0x97}, "snowy_beach": {0x83, 0xb5, 0x93}, "frozen_river": {0x80, 0xb4, 0x97},
		"jungle": {0x59, 0xc9, 0x3c}, "bamboo_jungle": {0x59, 0xc9, 0x3c}, "sparse_jungle": {0x64, 0xc7, 0x3f},
		"savanna": {0xbf, 0xb7, 0x55}, "savanna_plateau": {0xbf, 0xb7, 0x55}, "windswept_savanna": {0xbf, 0xb7, 0x55},
		"desert": {0xbf, 0xb7, 0x55}, "badlands": {0x90, 0x81, 0x4d}, "eroded_badlands": {0x90, 0x81, 0x4d}, "wooded_badlands": {0x90, 0x81, 0x4d},
		"swamp": {0x6a, 0x70, 0x39}, "mangrove_swamp": {0x6a, 0x70, 0x39},
		"meadow": {0x83, 0xbb, 0x6d}, "cherry_grove": {0xb6, 0xdb, 0x61},
		"windswept_hills": {0x8a, 0xb6, 0x89}, "windswept_gravelly_hills": {0x8a, 0xb6, 0x89}, "windswept_forest": {0x8a, 0xb6, 0x89},
		"stony_shore": {0x8a, 0xb6, 0x89}, "stony_peaks": {0x9a, 0xbe, 0x4b},
		"mushroom_fields": {0x55, 0xc9, 0x3f}, "river": {0x8e, 0xb9, 0x71}, "ocean": {0x8e, 0xb9, 0x71}, "deep_ocean": {0x8e, 0xb9, 0x71},
		"lukewarm_ocean": {0x8e, 0xb9, 0x71}, "warm_ocean": {0x8e, 0xb9, 0x71}, "cold_ocean": {0x8e, 0xb9, 0x71},
	}
)

// tintGrass swaps the plain grass swatch for the biome's grass colour;
// other materials pass through.
func tintGrass(c [3]uint8, id string) [3]uint8 {
	if c != groundGrass {
		return c
	}
	if t, ok := biomeGrass[strings.TrimPrefix(id, "minecraft:")]; ok {
		return t
	}
	return grassPlains
}
