package nativeui

// Deepslate ores need a distinct material key while retaining the mineral's
// fallback colour when assets are missing. Ground materials use RGB keys.
func deepslateOreColor(c [3]uint8) [3]uint8 {
	c[2]--
	return c
}

func (g *previewGround) registerOreTextures() {
	for ore, c := range groundOre {
		g.tex.RegisterBlock(c, "minecraft:"+ore+"_ore")
		g.tex.RegisterBlock(deepslateOreColor(c), "minecraft:deepslate_"+ore+"_ore")
	}
}

// deepslateLayer matches the surrounding rock when no surface rule supplied
// a block. Keep the same seed-mixed transition for rock and its ore variant.
func (g *previewGround) deepslateLayer(x, y, z int) bool {
	return y < 0 || y < 8 && int(g.roll(x, y, z, 2)%8) >= y
}

// oreColor is called with g.mu held, like the surrounding Section logic.
func (g *previewGround) oreColor(ore string, x, y, z int) ([3]uint8, bool) {
	c, ok := groundOre[ore]
	if !ok {
		return c, false
	}
	deep := g.deepslateLayer(x, y, z)
	if g.noise && g.rules {
		if block := g.ruleBlock(g.column(x, z), y); block != "" {
			deep = block == "minecraft:deepslate"
		}
	}
	if deep {
		c = deepslateOreColor(c)
	}
	return c, true
}
