package structure3d

// TerrainTextures are the block textures painted on the ground slab when
// the asset store can supply them. A nil field falls back to the solid
// colour AddTerrain already uses.
type TerrainTextures struct {
	GrassTop, GrassSide  *texel
	Dirt, Stone          *texel
	Deepslate, Bedrock   *texel
	Sand, Gravel, Snow   *texel
	Terracotta           *texel
	Netherrack, SoulSand *texel
	Basalt, Blackstone   *texel
	EndStone             *texel
	ok                   bool // at least one texture loaded
	// grassMask is the untinted grass_block_top; grass maps a registered
	// biome grass colour to the mask tinted with it.
	grassMask *texel
	grass     map[[3]uint8]*texel
	// blocks maps a ground colour to a block's own top texture, so a
	// surface block from any mod or resource pack tiles its real face.
	blocks map[[3]uint8]*texel
	store  *Store
	cache  *modelCache
}

// RegisterBlock binds a ground colour to a block id (any namespace). The
// texture comes from the block's model "up" face in the layered store, so
// a resource pack that retextures the block wins. It reports whether a
// texture was found.
func (t *TerrainTextures) RegisterBlock(c [3]uint8, id string) bool {
	if t.store == nil {
		return false
	}
	if t.blocks == nil {
		t.blocks = map[[3]uint8]*texel{}
	}
	if _, ok := t.blocks[c]; ok {
		return t.blocks[c] != nil
	}
	// A placed block always carries its properties: blockstates with only
	// keyed variants (snowy=false, axis=y) resolve through the first one.
	ns, name := Namespaced(id)
	m := t.cache.resolve(t.store, BlockState{Name: id, Properties: firstVariantProps(t.store, AssetPath(ns, "blockstates", name+".json"))})
	var tx *texel
	if m != nil {
		for _, e := range m.Elements {
			for _, f := range e.Faces {
				if f.Dir == "up" && f.Texture != nil {
					tx = toTexel(f.Texture)
					if f.TintIndex >= 0 {
						tintTexel(tx, c[0], c[1], c[2])
					}
					break
				}
			}
			if tx != nil {
				break
			}
		}
	}
	t.blocks[c] = tx
	if tx != nil {
		t.ok = true
	}
	return tx != nil
}

// TintGrass registers a biome grass colour: a ground top painted with that
// colour then tiles grass_block_top tinted to it, like the game's biome tint.
func (t *TerrainTextures) TintGrass(c [3]uint8) {
	if t.grassMask == nil {
		return
	}
	if t.grass == nil {
		t.grass = map[[3]uint8]*texel{}
	}
	if _, ok := t.grass[c]; ok {
		return
	}
	cp := *t.grassMask
	cp.pix = append([]uint8(nil), t.grassMask.pix...)
	tintTexel(&cp, c[0], c[1], c[2])
	t.grass[c] = &cp
}

// LoadTerrainTextures pulls the common overworld ground textures from the
// layered asset store (client jar, then mods, then resource packs). Missing
// entries stay nil so solid colours remain the fallback.
func LoadTerrainTextures(store *Store) TerrainTextures {
	if store == nil {
		return TerrainTextures{}
	}
	c := newModelCache()
	load := func(id string) *texel {
		img := c.texture(store, id)
		if img == nil {
			return nil
		}
		return toTexel(img)
	}
	// grass_block_top is a grey mask the game tints; bake a temperate
	// plains tint so the slab is green without a full biome-tint pass.
	grassTop := load("minecraft:block/grass_block_top")
	if grassTop != nil {
		tintTexel(grassTop, 0x91, 0xBD, 0x59)
	}
	t := TerrainTextures{
		GrassTop:   grassTop,
		GrassSide:  load("minecraft:block/grass_block_side"),
		Dirt:       load("minecraft:block/dirt"),
		Stone:      load("minecraft:block/stone"),
		Deepslate:  load("minecraft:block/deepslate"),
		Bedrock:    load("minecraft:block/bedrock"),
		Sand:       load("minecraft:block/sand"),
		Gravel:     load("minecraft:block/gravel"),
		Snow:       load("minecraft:block/snow"),
		Terracotta: load("minecraft:block/terracotta"),
		Netherrack: load("minecraft:block/netherrack"),
		SoulSand:   load("minecraft:block/soul_sand"),
		Basalt:     load("minecraft:block/basalt_side"),
		Blackstone: load("minecraft:block/blackstone"),
		EndStone:   load("minecraft:block/end_stone"),
	}
	t.grassMask = load("minecraft:block/grass_block_top")
	t.store, t.cache = store, c
	t.ok = t.GrassTop != nil || t.Stone != nil || t.Dirt != nil || t.Netherrack != nil || t.EndStone != nil
	return t
}

// pickTerrainTex chooses a tiling texture for a solid colour, so the
// ground can keep its material logic and still look like block faces.
func pickTerrainTex(t TerrainTextures, c [3]uint8, top bool) *texel {
	if !t.ok {
		return nil
	}
	if b := t.blocks[c]; b != nil {
		return b
	}
	if g := t.grass[c]; g != nil {
		if top {
			return g
		}
		if t.GrassSide != nil {
			return t.GrassSide
		}
		return t.Dirt
	}
	switch c {
	case [3]uint8{0x5d, 0x9c, 0x49}:
		if top && t.GrassTop != nil {
			return t.GrassTop
		}
		if t.GrassSide != nil {
			return t.GrassSide
		}
		return t.Dirt
	case [3]uint8{0x86, 0x60, 0x43}:
		return t.Dirt
	case [3]uint8{0x7d, 0x7d, 0x7d}:
		return t.Stone
	case [3]uint8{0x4d, 0x4d, 0x52}:
		return t.Deepslate
	case [3]uint8{0x33, 0x33, 0x33}:
		return t.Bedrock
	case [3]uint8{0xdb, 0xd3, 0xa0}, [3]uint8{0xc9, 0xbc, 0x8a}:
		return t.Sand
	case [3]uint8{0x84, 0x7f, 0x7a}:
		return t.Gravel
	case [3]uint8{0xf2, 0xf4, 0xf6}:
		return t.Snow
	case [3]uint8{0xa0, 0x58, 0x36}, [3]uint8{0xbe, 0x66, 0x24}:
		return t.Terracotta
	case [3]uint8{0x6f, 0x36, 0x35}:
		return t.Netherrack
	case [3]uint8{0x51, 0x3e, 0x32}:
		return t.SoulSand
	case [3]uint8{0x50, 0x50, 0x55}:
		return t.Basalt
	case [3]uint8{0x2a, 0x24, 0x2b}:
		return t.Blackstone
	case [3]uint8{0xdb, 0xde, 0x9e}:
		return t.EndStone
	}
	return nil
}

// tintTexel multiplies RGB by a colour (Minecraft foliage/grass style).
func tintTexel(t *texel, tr, tg, tb uint8) {
	for i := 0; i+3 < len(t.pix); i += 4 {
		t.pix[i] = uint8(int(t.pix[i]) * int(tr) / 255)
		t.pix[i+1] = uint8(int(t.pix[i+1]) * int(tg) / 255)
		t.pix[i+2] = uint8(int(t.pix[i+2]) * int(tb) / 255)
	}
}
