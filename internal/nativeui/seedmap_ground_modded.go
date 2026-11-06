package nativeui

import "strings"

// moddedGround is the hand-read surface of a mod biome whose surface rules
// are not traced: the top block and the block under it, painted with the
// block's own texture from the layered asset store
// (structure3d.TerrainTextures.RegisterBlock). Only mods without traced
// TerraBlender rules (currently the Aether) are listed, so a slab painted
// with them is ≈ only.
type moddedGround struct {
	top, under       string
	topKey, underKey [3]uint8 // ground colours bound to the blocks; unique
	stone            string   // filler below the dirt; "" keeps stone
	stoneKey         [3]uint8
}

// moddedGrounds is keyed by biome id; a "ns:*" entry covers a namespace.
// Mods that register TerraBlender surface rules (Biomes O' Plenty) are not
// listed: their rules are traced and evaluated (seedmap_ground_rules.go).
var moddedGrounds = map[string]moddedGround{
	"aether:*": {"aether:aether_grass_block", "aether:aether_dirt", [3]uint8{0x8b, 0xc9, 0x6f}, [3]uint8{0x8e, 0x8a, 0x77}, "aether:holystone", [3]uint8{0xbd, 0xbf, 0xb9}},
}

func moddedGroundFor(id string) (moddedGround, bool) {
	if strings.HasPrefix(id, "minecraft:") {
		return moddedGround{}, false
	}
	if m, ok := moddedGrounds[id]; ok {
		return m, true
	}
	if ns, _, ok := strings.Cut(id, ":"); ok {
		m, ok := moddedGrounds[ns+":*"]
		return m, ok
	}
	return moddedGround{}, false
}

// registerModdedGrounds binds every table block that the store can texture;
// only those colours are used, so a pack without the mod keeps the
// name-based vanilla ground.
func (g *previewGround) registerModdedGrounds() {
	g.modded = map[string]bool{}
	for _, m := range moddedGrounds {
		if g.tex.RegisterBlock(m.topKey, m.top) {
			g.modded[m.top] = true
		}
		if g.tex.RegisterBlock(m.underKey, m.under) {
			g.modded[m.under] = true
		}
		if m.stone != "" && g.tex.RegisterBlock(m.stoneKey, m.stone) {
			g.modded[m.stone] = true
		}
	}
}

// moddedTop is the mod biome's top colour when its block is textured.
func (g *previewGround) moddedTop(id string) ([3]uint8, bool) {
	m, ok := moddedGroundFor(id)
	if !ok || !g.modded[m.top] {
		return [3]uint8{}, false
	}
	return m.topKey, true
}

func (g *previewGround) moddedUnder(id string) ([3]uint8, bool) {
	m, ok := moddedGroundFor(id)
	if !ok || !g.modded[m.under] {
		return [3]uint8{}, false
	}
	return m.underKey, true
}

func (g *previewGround) moddedStone(id string) ([3]uint8, bool) {
	m, ok := moddedGroundFor(id)
	if !ok || m.stone == "" || !g.modded[m.stone] {
		return [3]uint8{}, false
	}
	return m.stoneKey, true
}
