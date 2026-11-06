package launcher

import (
	"encoding/json"

	"github.com/Viking602/YoloMCL/cubiomes"
	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// seedMapProbe is one structure of a set: ChunkGenerator.tryGenerateStructure
// tries the set's structures in a weighted order until one passes its own
// Structure.isValidBiome, so a candidate is a structure chunk when any
// structure's biome check passes at that structure's generation stub.
type seedMapProbe struct {
	allowed map[string]bool
	jigsaw  *seedMapJigsawStart
	// non-jigsaw stub: block offset in the chunk, fixed Y or a heightmap.
	dx, dz  int
	fixedY  *int
	surface string // "", "WORLD_SURFACE_WG" or "OCEAN_FLOOR_WG"
	// lowest5: Structure.getLowestYIn5by5BoxOffset7Blocks after one
	// random.nextDouble (rarity roll) and Rotation.getRandom.
	lowest5 bool
	// uniform + popDouble: BetterMineshaftStructure (javap) samples
	// nextDouble then nextInt(maxY-minY+1)+minY at chunk block (dx,dz).
	uniform    bool
	minY, maxY int
	popDouble  bool
}

func intp(v int) *int { return &v }

// compileProbe places the stub like the vanilla structure types do.
func compileProbe(docs map[string]WorldgenDocument, id string, allowed map[string]bool) seedMapProbe {
	pr := seedMapProbe{allowed: allowed, jigsaw: compileJigsawStart(docs, id)}
	if pr.jigsaw != nil {
		return pr
	}
	var def struct {
		Type        string          `json:"type"`
		StartHeight json.RawMessage `json:"start_height"`
		Project     string          `json:"project_start_to_heightmap"`
	}
	if doc, ok := docs["structure/"+id]; ok {
		_ = json.Unmarshal(doc.Data, &def)
		// Non-jigsaw stubs: absolute start_height fixes Y only when there is
		// no heightmap projection. absolute:0 + WORLD_SURFACE_WG (surface
		// jigsaw JSON on a custom type we do not yet compile) must probe the
		// surface, not Y=0 (cave biomes in 3D multi_noise).
		var abs struct {
			Absolute *int   `json:"absolute"`
			Type     string `json:"type"`
			Value    struct {
				Absolute *int `json:"absolute"`
			} `json:"value"`
		}
		if json.Unmarshal(def.StartHeight, &abs) == nil {
			if abs.Absolute == nil && seedMapID(abs.Type) == "minecraft:constant" {
				abs.Absolute = abs.Value.Absolute
			}
			if def.Project != "" {
				pr.surface = def.Project
			} else if abs.Absolute != nil {
				pr.fixedY = abs.Absolute
			}
		}
	}
	switch seedMapID(def.Type) {
	case "minecraft:desert_pyramid", "minecraft:jungle_temple", "minecraft:swamp_hut", "minecraft:igloo", "minecraft:woodland_mansion":
		// onTopOfChunkCenter(WORLD_SURFACE_WG); mansion: middle at its lowest corner (≈ surface)
		pr.dx, pr.dz, pr.surface = 8, 8, "WORLD_SURFACE_WG"
	case "minecraft:shipwreck":
		pr.dx, pr.dz, pr.surface = 8, 8, "OCEAN_FLOOR_WG"
	case "minecraft:ocean_ruin":
		pr.fixedY = intp(90)
	case "minecraft:buried_treasure":
		pr.dx, pr.dz, pr.fixedY = 9, 9, intp(90)
	case "iceandfire:pixie_village":
		// getLowestYIn5by5BoxOffset7Blocks: chunk block 7,7 at the lowest
		// WORLD_SURFACE_WG of the 5x5 box (javap PixieVillageStructure).
		pr.dx, pr.dz, pr.surface, pr.lowest5 = 7, 7, "WORLD_SURFACE_WG", true
	case "minecraft:nether_fortress":
		pr.fixedY = intp(64)
	case "bettermineshafts:mineshaft":
		// BetterMineshaftStructure.findGenerationPoint (javap): discard
		// nextDouble, Y in CONFIG minY..maxY (defaults -55..30; 逆转 configs match),
		// stub at ChunkPos.getBlockX/Z(3). Biome is checked at that stub.
		pr.dx, pr.dz = 3, 3
		pr.uniform, pr.minY, pr.maxY, pr.popDouble = true, -55, 30, true
	}
	return pr
}

// probePos is the stub position of one structure for the candidate at (x, z).
func (s *LocalSampler) probePos(pr seedMapProbe, dimension string, x, y, z int) (int, int, int) {
	if pr.jigsaw != nil {
		return s.jigsawProbe(pr.jigsaw, dimension, x, y, z)
	}
	// Candidates are chunk centres (+8); every typed stub above is relative
	// to the chunk's min block (ChunkPos.getMinBlockX / getBlockX(n)).
	if pr.dx == 0 && pr.dz == 0 && pr.fixedY == nil && pr.surface == "" && !pr.lowest5 && !pr.uniform {
		return x, y, z
	}
	bx, bz := x&^15+pr.dx, z&^15+pr.dz
	if pr.lowest5 {
		return s.lowestIn5by5(dimension, bx, y, bz)
	}
	if pr.uniform {
		r := worldgen.LargeFeatureRandom(s.seed, int32(bx>>4), int32(bz>>4))
		if pr.popDouble {
			r.NextDouble()
		}
		h := pr.minY + int(r.NextInt(int32(pr.maxY-pr.minY+1)))
		return bx, h, bz
	}
	switch {
	case pr.fixedY != nil:
		return bx, *pr.fixedY, bz
	case pr.surface != "":
		if solidTop, top, ok := s.TerrainColumn(dimension, bx, bz, nil); ok {
			if pr.surface == "OCEAN_FLOOR_WG" {
				return bx, solidTop, bz
			}
			return bx, top, bz
		}
	}
	return bx, y, bz
}

// biomeCheck reports whether any structure of p may start at (x, z), and the
// biome of the first checked stub. known is false without a biome answer.
func (s *LocalSampler) biomeCheck(p seedMapPlacement, d seedMapDimension, dimension string, x, y, z int, scratch *cubiomes.Sampler) (biome string, known, pass bool) {
	if p.avoidWater {
		if b, ok, hit := nearWater(p, d, x, z, scratch); !ok {
			return "", false, true
		} else if hit {
			return b, true, false
		}
	}
	if len(p.probes) == 0 {
		bx, by, bz := s.biomeProbe(p, dimension, x, y, z)
		b, ok := d.biomeAt(bx, by, bz, scratch, false)
		return b, ok, !ok || p.allowed[b]
	}
	for _, pr := range p.probes {
		bx, by, bz := s.probePos(pr, dimension, x, y, z)
		b, ok := d.biomeAt(bx, by, bz, scratch, false)
		if !ok {
			return "", false, true
		}
		if biome == "" {
			biome = b
		}
		if pr.allowed[b] && s.jigsawBiomeRadius(pr, d, dimension, x, z, scratch) {
			return b, true, true
		}
	}
	return biome, true, false
}

// jigsawProbe is JigsawStructure.findGenerationPoint up to isValidBiome:
// start height, then Rotation.getRandom, then the start pool template.
func (s *LocalSampler) jigsawProbe(j *seedMapJigsawStart, dimension string, x, y, z int) (int, int, int) {
	// JigsawPlacement starts at the chunk's min block, not the candidate's
	// chunk centre.
	x, z = x&^15, z&^15
	pieces := s.jigsawPools[j.pool]
	r := worldgen.LargeFeatureRandom(s.seed, int32(x>>4), int32(z>>4))
	h := j.absY
	if j.uniform {
		h = j.minY + int(r.NextInt(int32(j.maxY-j.minY+1)))
	}
	if len(pieces) == 0 { // unbound pool: chunk corner, start height still known
		if j.heightmap {
			if _, top, ok := s.TerrainColumn(dimension, x, z, nil); ok {
				if j.yung {
					// Heightmap yung embeds startHeight for pieces; biome uses occupied
					// column (surface_structures). Buried Y false-rejects as dripstone
					// in our 3D multi_noise (jungle_temple 1->0 regression).
					return x, top + 1, z
				}
				return x, top + 1 + h, z
			}
			return x, y, z
		}
		return x, h, z
	}
	rot := r.NextInt(4) // NONE, CLOCKWISE_90, CLOCKWISE_180, COUNTERCLOCKWISE_90
	piece := pieces[r.NextInt(int32(len(pieces)))]
	if piece.Empty {
		return x, h, z
	}
	sx, sz := piece.Size[0]-1, piece.Size[1]-1
	minX, maxX, minZ, maxZ := x, x+sx, z, z+sz
	switch rot {
	case 1:
		minX, maxX, minZ, maxZ = x-sz, x, z, z+sx
	case 2:
		minX, maxX, minZ, maxZ = x-sx, x, z-sz, z
	case 3:
		minX, maxX, minZ, maxZ = x, x+sz, z-sx, z
	}
	cx, cz := (minX+maxX)/2, (minZ+maxZ)/2
	if j.heightmap {
		if _, top, ok := s.TerrainColumn(dimension, cx, cz, nil); ok {
			if j.yung {
				return cx, top + 1, cz
			}
			return cx, top + 1 + h, cz
		}
		return cx, y, cz
	}
	return cx, h, cz
}

// lowestIn5by5 mirrors Structure.getLowestYIn5by5BoxOffset7Blocks: corner
// offsets (5,5) turned by the rotation, minimum WORLD_SURFACE_WG of the four
// columns, at chunk block (7, 7).
func (s *LocalSampler) lowestIn5by5(dimension string, bx, y, bz int) (int, int, int) {
	r := worldgen.LargeFeatureRandom(s.seed, int32(bx>>4), int32(bz>>4))
	r.NextDouble()
	i, j := 5, 5
	switch r.NextInt(4) { // NONE, CLOCKWISE_90, CLOCKWISE_180, COUNTERCLOCKWISE_90
	case 1:
		i = -5
	case 2:
		i, j = -5, -5
	case 3:
		j = -5
	}
	low, found := 0, false
	for _, c := range [4][2]int{{0, 0}, {0, j}, {i, 0}, {i, j}} {
		if _, top, ok := s.TerrainColumn(dimension, bx+c[0], bz+c[1], nil); ok && (!found || top < low) {
			low, found = top, true
		}
	}
	if !found {
		return bx, y, bz
	}
	return bx, low, bz
}

// jigsawBiomeRadius is integrated_api JigsawStructure.extraSpawningChecks
// (javap, runs before the jigsaw start piece is drawn): y is the start
// height, plus the first free height at the chunk's min block when the start
// is projected; every chunk within the radius must have an allowed biome at
// its min corner at that y (QuartPos.fromSection of the chunk coords). Its
// cannot_spawn_in_liquid check reads the block at the first free height,
// which is never a fluid, so it is not modelled.
func (s *LocalSampler) jigsawBiomeRadius(pr seedMapProbe, d seedMapDimension, dimension string, x, z int, scratch *cubiomes.Sampler) bool {
	j := pr.jigsaw
	if j == nil || j.biomeRadius == 0 || j.uniform {
		return true
	}
	x, z = x&^15, z&^15
	y := j.absY
	if j.heightmap {
		_, top, ok := s.TerrainColumn(dimension, x, z, nil)
		if !ok {
			return true
		}
		if j.yung {
			y = top + 1
		} else {
			y += top + 1
		}
	}
	for cx := x>>4 - j.biomeRadius; cx <= x>>4+j.biomeRadius; cx++ {
		for cz := z>>4 - j.biomeRadius; cz <= z>>4+j.biomeRadius; cz++ {
			if b, ok := d.biomeAt(cx<<4, y, cz<<4, scratch, false); ok && !pr.allowed[b] {
				return false
			}
		}
	}
	return true
}
