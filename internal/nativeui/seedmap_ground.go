package nativeui

import (
	"context"
	"math"
	"runtime"
	"strings"
	"sync"

	"github.com/Viking602/YoloMCL/cubiomes"
	"github.com/Viking602/YoloMCL/internal/launcher"
	"github.com/Viking602/YoloMCL/internal/structure3d"
	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// previewGround colours the 3D preview's ground from the world seed: the
// map's surface heights for relief; grass, sand, dirt and stone on top
// (never the flat biome swatch); inside, rock layers, carver caves and ore
// blobs from the same generators as the map's cave and ore overlays.
//
// Noise caves (cheese, spaghetti, noodle) and aquifers are not generated
// anywhere in the tree yet — the map's cave overlay notes the same gap —
// so they stay absent here. Dirt depth, deepslate and bedrock use a
// seed-mixed per-block roll that follows vanilla's probabilities, not the
// full surface-rule graph.
type previewGround struct {
	ctx     context.Context
	seed    int64
	dim     string
	sampler *launcher.LocalSampler
	surface structure3d.HeightFunc
	step    int
	caves   bool

	mu      sync.Mutex
	carves  seedMapCaves
	heights map[[2]int]int
	biomes  map[[2]int]string
	ores    map[[3]int]string
	tex     structure3d.TerrainTextures // zero: solid colours only
	modded  map[string]bool             // mod surface blocks with a texture
	end     *cubiomes.EndIslands
	endD    map[[2]int]float64

	// noise: the dimension has a compiled final_density; cols caches its
	// noise-stage columns (solid, water, lava, air incl. noise caves).
	noise bool
	info  launcher.SeedMapTerrainInfo
	cols  map[[2]int]*groundColumn

	// rules: the dimension has traced mod surface rules (seedmap_ground_rules.go).
	rules       bool
	rulePalette []string
	ruleIndex   map[string]uint8
	ruleColors  [][3]uint8
	ruleOK      []bool
}

type groundColumn struct {
	sub             []launcher.TerrainSubstance
	solidTop, top   int // highest solid / non-air Y; MinY-1 when none
	floorTop, ceilY int // Nether: top of the floor (lava counts), first solid above it
	rules           *ruleColumn
}

func newPreviewGround(seed int64, dim string, sampler *launcher.LocalSampler, surface structure3d.HeightFunc, step int, caves bool, version string, x0, z0, w, d int, store *structure3d.Store) *previewGround {
	return newPreviewGroundContext(context.Background(), seed, dim, sampler, surface, step, caves, version, x0, z0, w, d, store)
}

func newPreviewGroundContext(ctx context.Context, seed int64, dim string, sampler *launcher.LocalSampler, surface structure3d.HeightFunc, step int, caves bool, version string, x0, z0, w, d int, store *structure3d.Store) *previewGround {
	g := &previewGround{ctx: ctx, seed: seed, dim: dim, sampler: sampler, surface: surface, step: max(step, 1), caves: caves,
		heights: map[[2]int]int{}, biomes: map[[2]int]string{}, ores: map[[3]int]string{}}
	if store != nil {
		g.tex = structure3d.LoadTerrainTextures(store)
		g.registerModdedGrounds()
		if dim == "minecraft:overworld" {
			g.registerOreTextures()
		}
	}
	if dim == "minecraft:the_end" {
		g.end, g.endD = cubiomes.NewEndIslands(seed), map[[2]int]float64{}
	}
	if info, ok := sampler.TerrainInfo(dim); ok {
		g.noise, g.info, g.cols = true, info, map[[2]int]*groundColumn{}
		g.setupRules()
		if g.tex.GrassTop != nil {
			for _, c := range biomeGrass {
				g.tex.TintGrass(c)
			}
			g.tex.TintGrass(grassPlains)
		}
		if w > 0 && d > 0 {
			g.prefetch(x0, z0, w, d)
		}
	}
	if ctx.Err() != nil {
		return g
	}
	if dim != "minecraft:overworld" || version == "" {
		return g
	}
	data, _, err := worldgen.LoadOreData(version)
	if err != nil || data == nil {
		return g
	}
	world := &worldgen.OreWorld{Biome: func(bx, by, bz int) string {
		if id, ok := sampler.BiomeAt(dim, bx, by, bz); ok {
			return id
		}
		return "minecraft:plains"
	}}
	cx0, cz0 := int32(math.Floor(float64(x0)/16))-1, int32(math.Floor(float64(z0)/16))-1
	cx1, cz1 := int32(math.Floor(float64(x0+w)/16))+1, int32(math.Floor(float64(z0+d)/16))+1
	for cz := cz0; cz <= cz1; cz++ {
		for cx := cx0; cx <= cx1; cx++ {
			if ctx.Err() != nil {
				return g
			}
			worldgen.ChunkOres(seed, cx, cz, data.Features, world, func(oc worldgen.OreCell) {
				g.ores[[3]int{oc.X, oc.Y, oc.Z}] = oc.Feature.Ore
			})
		}
	}
	return g
}

var (
	groundGrass     = [3]uint8{0x5d, 0x9c, 0x49}
	groundSnow      = [3]uint8{0xf2, 0xf4, 0xf6}
	groundPodzol    = [3]uint8{0x6a, 0x54, 0x3a}
	groundMycelium  = [3]uint8{0x6f, 0x63, 0x6c}
	groundDirt      = [3]uint8{0x86, 0x60, 0x43}
	groundSand      = [3]uint8{0xdb, 0xd3, 0xa0}
	groundRedSand   = [3]uint8{0xbe, 0x66, 0x24}
	groundSandstone = [3]uint8{0xc9, 0xbc, 0x8a}
	groundTerracot  = [3]uint8{0xa0, 0x58, 0x36}
	groundGravel    = [3]uint8{0x84, 0x7f, 0x7a}
	groundStone     = [3]uint8{0x7d, 0x7d, 0x7d}
	groundDeepslate = [3]uint8{0x4d, 0x4d, 0x52}
	groundBedrock   = [3]uint8{0x33, 0x33, 0x33}
	groundCave      = [3]uint8{0x1f, 0x21, 0x26}
	groundLava      = [3]uint8{0xe3, 0x6b, 0x1c}
	groundWater     = [3]uint8{0x3f, 0x76, 0xe4}
	groundOre       = map[string][3]uint8{
		"coal": {0x3b, 0x3f, 0x45}, "iron": {0xd8, 0xa2, 0x7a}, "copper": {0xe0, 0x70, 0x3c}, "gold": {0xf2, 0xc2, 0x30},
		"redstone": {0xe0, 0x30, 0x2c}, "lapis": {0x2f, 0x5f, 0xd0}, "diamond": {0x3f, 0xe0, 0xd8}, "emerald": {0x2f, 0xc4, 0x5a},
	}
)

// column is the noise-stage column at x, z; the caller holds g.mu.
func (g *previewGround) column(x, z int) *groundColumn {
	k := [2]int{x, z}
	if c := g.cols[k]; c != nil {
		return c
	}
	c := g.newColumn(x, z)
	g.cols[k] = c
	return c
}

func (g *previewGround) newColumn(x, z int) *groundColumn {
	c := &groundColumn{sub: make([]launcher.TerrainSubstance, g.info.Height)}
	c.solidTop, c.top, _ = g.sampler.TerrainColumn(g.dim, x, z, c.sub)
	g.floorCeil(c)
	if g.rules {
		// surface rules read the uncarved column; vanilla carves after surface
		c.rules = g.ruleColumnAt(x, z, c)
	}
	if g.noise && strings.Contains(g.dim, "nether") {
		g.netherCarve(x, z, c)
		g.floorCeil(c)
	}
	return c
}

// netherCarve applies the seed-exact vanilla nether_cave carver to the column.
func (g *previewGround) netherCarve(x, z int, c *groundColumn) {
	fluid := g.sampler.NetherCarverFluid(g.dim)
	if !fluid.OK {
		return
	}
	cx, cz := int32(x>>4), int32(z>>4)
	minY := g.info.MinY
	for dx := int32(-8); dx <= 8; dx++ {
		for dz := int32(-8); dz <= 8; dz++ {
			for _, sh := range g.netherShapes(cx+dx, cz+dz) {
				// Most source-chunk ellipsoids cannot touch this column. Reject
				// them before testing solid blocks, and visit only their Y span.
				b := sh.Bounds()
				if x < b[0] || x > b[3] || z < b[2] || z > b[5] {
					continue
				}
				for y := max(1, minY, b[1]); y <= min(120, minY+len(c.sub)-1, b[4]); y++ {
					i := y - minY
					if c.sub[i] != launcher.TerrainSolid || g.ruleBlock(c, y) == "minecraft:bedrock" || !sh.Carved(x, y, z) {
						continue
					}
					c.sub[i] = fluid.Substance(y)
				}
			}
		}
	}
}

var netherShapeCache sync.Map // [3]int64{seed,cx,cz} -> []worldgen.CarveShape

func (g *previewGround) netherShapes(cx, cz int32) []worldgen.CarveShape {
	k := [3]int64{g.seed, int64(cx), int64(cz)}
	if v, ok := netherShapeCache.Load(k); ok {
		return v.([]worldgen.CarveShape)
	}
	s := worldgen.NetherCarves(g.seed, cx, cz)
	netherShapeCache.Store(k, s)
	return s
}

func (g *previewGround) floorCeil(c *groundColumn) {
	minY := g.info.MinY
	c.floorTop, c.ceilY = minY-1, structure3d.TerrainVoid
	i := 0
	for i < len(c.sub) && c.sub[i] != launcher.TerrainAir {
		i++
	}
	c.floorTop = minY + i - 1
	for ; i < len(c.sub); i++ {
		if c.sub[i] == launcher.TerrainSolid {
			c.ceilY = minY + i
			break
		}
	}
}

// prefetch keeps neighbouring columns on the same worker so noise-cell
// corners stay hot. Bound the workers to leave room for UI/map rendering.
func (g *previewGround) prefetch(x0, z0, w, d int) {
	const tile = 8
	nx, nz := (w-g.step/2)/g.step+1, (d-g.step/2)/g.step+1
	columns := make([]*groundColumn, nx*nz)
	type job struct{ ix, iz int }
	jobs := make(chan job, 16)
	var wg sync.WaitGroup
	for n := min(max(runtime.GOMAXPROCS(0)-1, 1), 8); n > 0; n-- {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				for iz := j.iz; iz < min(j.iz+tile, nz); iz++ {
					for ix := j.ix; ix < min(j.ix+tile, nx); ix++ {
						if g.ctx.Err() != nil {
							return
						}
						x, z := x0+g.step/2+ix*g.step, z0+g.step/2+iz*g.step
						columns[iz*nx+ix] = g.newColumn(x, z)
					}
				}
			}
		}()
	}
dispatch:
	for iz := 0; iz < nz; iz += tile {
		for ix := 0; ix < nx; ix += tile {
			select {
			case jobs <- job{ix, iz}:
			case <-g.ctx.Done():
				break dispatch
			}
		}
	}
	close(jobs)
	wg.Wait()
	if g.ctx.Err() != nil {
		return
	}
	for iz := 0; iz < nz; iz++ {
		for ix := 0; ix < nx; ix++ {
			g.cols[[2]int{x0 + g.step/2 + ix*g.step, z0 + g.step/2 + iz*g.step}] = columns[iz*nx+ix]
		}
	}
}

// substance is the noise-stage block at x, y, z; the caller holds g.mu.
func (g *previewGround) substance(x, y, z int) (launcher.TerrainSubstance, bool) {
	if !g.noise || y < g.info.MinY || y >= g.info.MinY+g.info.Height {
		return 0, false
	}
	return g.column(x, z).sub[y-g.info.MinY], true
}

// WaterTop is the overworld slab height with the noise stage: the top
// non-air block, so seas, lakes and aquifer water show as water.
func (g *previewGround) WaterTop(x, z int) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	c := g.column(x, z)
	if c.top < g.info.MinY {
		return g.height(x, z)
	}
	return c.top
}

func (g *previewGround) height(x, z int) int {
	k := [2]int{x, z}
	if h, ok := g.heights[k]; ok {
		return h
	}
	if g.noise {
		// the prefetched column already holds the final_density top
		if c := g.column(x, z); c.solidTop >= g.info.MinY {
			g.heights[k] = c.solidTop
			return c.solidTop
		}
	}
	h := g.surface(x, z)
	g.heights[k] = h
	return h
}

func (g *previewGround) biome(x, h, z int) string {
	k := [2]int{x, z}
	if id, ok := g.biomes[k]; ok {
		return id
	}
	id, _ := g.sampler.BiomeAt(g.dim, x, h, z)
	g.biomes[k] = id
	return id
}

// Top is the solid surface block colour — grass, sand, gravel — never the
// 2D map's biome swatch (which painted deep_dark / ocean as a flat blue).
func (g *previewGround) Top(x, z, h int) [3]uint8 {
	g.mu.Lock()
	sub, ok := g.substance(x, h, z)
	id := g.biome(x, h, z)
	rule, ruled := g.ruleColor(x, h, z, id)
	g.mu.Unlock()
	if ok {
		switch sub {
		case launcher.TerrainWater:
			return groundWater
		case launcher.TerrainLava:
			return groundLava
		}
	}
	if ruled {
		return rule
	}
	if c, ok := g.moddedTop(id); ok {
		return c
	}
	return tintGrass(surfaceTop(id, h), id)
}

func surfaceTop(id string, h int) [3]uint8 {
	switch {
	case strings.Contains(id, "badlands"):
		return groundTerracot
	case strings.Contains(id, "desert"):
		return groundSand
	case strings.Contains(id, "beach") || strings.Contains(id, "stony_shore"):
		if strings.Contains(id, "stony") {
			return groundStone
		}
		return groundSand
	case strings.Contains(id, "ocean") || strings.Contains(id, "river") || strings.Contains(id, "frozen"):
		if h < gaugeSeaLevel-2 {
			return groundGravel
		}
		return groundSand
	case strings.Contains(id, "mushroom"):
		return groundMycelium
	case strings.Contains(id, "old_growth") || strings.Contains(id, "giant_tree_taiga"):
		return groundPodzol
	case strings.Contains(id, "snowy") || strings.Contains(id, "ice_spikes") || strings.Contains(id, "frozen_peaks") || strings.Contains(id, "jagged_peaks"):
		return groundSnow
	case strings.Contains(id, "stony_peaks") || strings.Contains(id, "windswept_gravelly"):
		return groundStone
	case h > 150:
		return groundSnow
	case h > 110:
		return groundStone
	case h < gaugeSeaLevel:
		return groundGravel
	}
	return groundGrass
}

// Section is the ground at a world block inside a column of the slab.
func (g *previewGround) Section(x, y, z int) [3]uint8 {
	switch g.dim {
	case "minecraft:the_end":
		return g.endSection(x, y, z)
	case "minecraft:the_nether":
		g.mu.Lock()
		defer g.mu.Unlock()
		return g.netherSection(x, y, z)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	h := g.height(x, z)
	if sub, ok := g.substance(x, y, z); ok && sub != launcher.TerrainSolid {
		switch sub {
		case launcher.TerrainWater:
			return groundWater
		case launcher.TerrainLava:
			return groundLava
		}
		return groundCave // air: above ground or a noise cave
	}
	if y > h && !g.noise {
		return groundCave // air / water void above the solid column
	}
	if g.dim != "minecraft:overworld" {
		// Mod dimensions (the Aether, ...): the mod surface when its
		// blocks are textured, three blocks deep (≈), stone below.
		id := g.biome(x, h, z)
		if c, ok := g.ruleColor(x, y, z, id); ok {
			return c
		}
		if y == h {
			if c, ok := g.moddedTop(id); ok {
				return c
			}
		} else if y >= h-3 {
			if c, ok := g.moddedUnder(id); ok {
				return c
			}
		}
		if c, ok := g.moddedStone(id); ok {
			return c
		}
		return groundStone
	}
	// Bedrock floor: five layers, probability (5-depth)/5, seed-mixed per block
	// the same way BedrockRule rolls (y + nextInt(5) <= floor).
	if y <= gaugeBedrock {
		return groundBedrock
	}
	if y < gaugeBedrock+5 {
		depth := y - gaugeBedrock // 1..4
		if int(g.roll(x, y, z, 1)%5) >= depth {
			return groundBedrock
		}
	}
	if g.caves && y < h-1 {
		m := g.carves.mask(g.seed, int32(x>>4), int32(z>>4), y)
		if bit := uint16(1) << (x & 15); (m.cave[z&15]|m.canyon[z&15])&bit != 0 {
			if y <= worldgen.CarverLavaY {
				return groundLava
			}
			return groundCave
		}
	}
	x0, z0 := x-g.step/2, z-g.step/2
	for dx := 0; dx < g.step; dx++ {
		for dz := 0; dz < g.step; dz++ {
			if ore, ok := g.ores[[3]int{x0 + dx, y, z0 + dz}]; ok {
				if c, ok := g.oreColor(ore, x, y, z); ok {
					return c
				}
			}
		}
	}
	// Mod surface rules run before carvers and ores, after the noise stage.
	if c, ok := g.ruleColor(x, y, z, g.biome(x, h, z)); ok {
		return c
	}
	// Deepslate: solid below Y 0; Y 0..8 fades with a per-block roll.
	if g.deepslateLayer(x, y, z) {
		return groundDeepslate
	}
	// Dirt under the surface: 2..4 blocks, seed-mixed depth.
	id := g.biome(x, h, z)
	dirtDepth := 2 + int(g.roll(x, h, z, 3)%3) // 2, 3 or 4
	if y >= h-dirtDepth {
		if y == h {
			if c, ok := g.moddedTop(id); ok {
				return c
			}
		} else if c, ok := g.moddedUnder(id); ok {
			return c
		}
		return tintGrass(surfaceUnder(id, h, y == h), id)
	}
	return groundStone
}

func surfaceUnder(id string, h int, top bool) [3]uint8 {
	if top {
		return surfaceTop(id, h)
	}
	switch {
	case strings.Contains(id, "badlands"):
		return groundTerracot
	case strings.Contains(id, "desert"):
		return groundSandstone
	case strings.Contains(id, "beach") || strings.Contains(id, "ocean") || strings.Contains(id, "river"):
		return groundSand
	case h < gaugeSeaLevel:
		return groundGravel
	}
	return groundDirt
}

// roll is a seed-mixed per-block uint32, stable across runs.
func (g *previewGround) roll(x, y, z, salt int) uint32 {
	n := uint64(g.seed) ^ uint64(uint32(salt))*0x9E3779B97F4A7C15
	n ^= uint64(uint32(x)) * 0xD1B54A32D192ED03
	n ^= uint64(uint32(y)) * 0xABC98388FB8FAC03
	n ^= uint64(uint32(z)) * 0x91E10DA5C79E7A13
	n ^= n >> 32
	n *= 0xd6e8feb86659fd93
	return uint32(n ^ n>>32)
}

func (g *previewGround) textures() structure3d.TerrainTextures { return g.tex }

// ruleBlock is the uncarved surface-rule block at y, "" when none; the nether
// carver's replaceable tag excludes bedrock.
func (g *previewGround) ruleBlock(c *groundColumn, y int) string {
	r := c.rules
	if r == nil || y < r.lo || y >= r.lo+len(r.idx) {
		return ""
	}
	return g.rulePalette[r.idx[y-r.lo]]
}
