package nativeui

import "hash/fnv"

// Mod surface rules (TerraBlender namespaced rules traced from the mods'
// bytecode and evaluated in pure Go by cubiomes.SurfaceRules) decide the
// noise-stage surface of mod biomes. Columns keep only the rule-covered Y
// span as palette indexes; "" (index 0) leaves the vanilla ground.

// ruleColumn is the rule output for one column: palette indexes from lo up.
type ruleColumn struct {
	lo  int
	idx []uint8
}

func (r *ruleColumn) at(y int) uint8 {
	if r == nil || y < r.lo || y-r.lo >= len(r.idx) {
		return 0
	}
	return r.idx[y-r.lo]
}

// ruleVanilla maps vanilla blocks that rules place to the preview's ground
// colours (already texture-bound); grass is tinted by biome.
var ruleVanilla = map[string][3]uint8{
	"minecraft:stone": groundStone, "minecraft:dirt": groundDirt, "minecraft:coarse_dirt": groundDirt,
	"minecraft:rooted_dirt": groundDirt, "minecraft:grass_block": groundGrass, "minecraft:podzol": groundPodzol,
	"minecraft:mycelium": groundMycelium, "minecraft:sand": groundSand, "minecraft:red_sand": groundRedSand,
	"minecraft:sandstone": groundSandstone, "minecraft:red_sandstone": groundSandstone, "minecraft:terracotta": groundTerracot,
	"minecraft:gravel": groundGravel, "minecraft:snow_block": groundSnow, "minecraft:powder_snow": groundSnow,
	"minecraft:deepslate": groundDeepslate, "minecraft:bedrock": groundBedrock, "minecraft:water": groundWater,
	"minecraft:lava": groundLava, "minecraft:air": groundCave,
}

// setupRules builds the rule palette and binds a unique ground colour per
// block, textured from the layered store when the block has a model.
func (g *previewGround) setupRules() {
	blocks := g.sampler.SurfaceRuleBlocks(g.dim)
	if len(blocks) == 0 || len(blocks) > 254 {
		return
	}
	g.rulePalette = append([]string{""}, blocks...)
	g.ruleIndex = map[string]uint8{}
	g.ruleColors = make([][3]uint8, len(g.rulePalette))
	g.ruleOK = make([]bool, len(g.rulePalette))
	used := map[[3]uint8]bool{}
	for _, c := range ruleVanilla {
		used[c] = true
	}
	for _, c := range groundOre {
		used[c], used[deepslateOreColor(c)] = true, true
	}
	for _, m := range moddedGrounds {
		used[m.topKey], used[m.underKey], used[m.stoneKey] = true, true, true
	}
	for i, id := range g.rulePalette[1:] {
		i++
		g.ruleIndex[id] = uint8(i)
		if c, ok := ruleVanilla[id]; ok {
			g.ruleColors[i], g.ruleOK[i] = c, true
			continue
		}
		key := ruleKey(id, used)
		used[key] = true
		if g.tex.RegisterBlock(key, id) {
			g.ruleColors[i], g.ruleOK[i] = key, true
		} else if c, ok := ruleVanilla[id]; ok {
			g.ruleColors[i], g.ruleOK[i] = c, true
		}
	}
	g.rules = true
}

// ruleKey is a stable colour for id that no other ground colour uses.
func ruleKey(id string, used map[[3]uint8]bool) [3]uint8 {
	h := fnv.New32a()
	h.Write([]byte(id))
	n := h.Sum32()
	for {
		c := [3]uint8{uint8(n), uint8(n >> 8), uint8(n >> 16)}
		if !used[c] {
			return c
		}
		n = n*16777619 + 1
	}
}

// ruleColumnAt evaluates the mod rules for a noise column (any goroutine).
func (g *previewGround) ruleColumnAt(x, z int, c *groundColumn) *ruleColumn {
	out := make([]string, g.info.Height)
	if !g.sampler.SurfaceColumn(g.dim, x, z, c.sub, c.top, out) {
		return nil
	}
	lo, hi := -1, -1
	for i, id := range out {
		if id != "" {
			if lo < 0 {
				lo = i
			}
			hi = i
		}
	}
	if lo < 0 {
		return nil
	}
	r := &ruleColumn{lo: g.info.MinY + lo, idx: make([]uint8, hi-lo+1)}
	for i := lo; i <= hi; i++ {
		r.idx[i-lo] = g.ruleIndex[out[i]]
	}
	return r
}

// ruleColor is the rule block's ground colour at x, y, z; the caller holds g.mu.
func (g *previewGround) ruleColor(x, y, z int, biome string) ([3]uint8, bool) {
	if !g.rules || !g.noise {
		return [3]uint8{}, false
	}
	i := g.column(x, z).rules.at(y)
	if i == 0 || !g.ruleOK[i] {
		return [3]uint8{}, false
	}
	c := g.ruleColors[i]
	if g.rulePalette[i] == "minecraft:grass_block" {
		c = tintGrass(c, biome)
	}
	return c, true
}
