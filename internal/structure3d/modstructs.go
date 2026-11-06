package structure3d

import (
	"math"
	"strings"

	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// Adapters for big mods whose structures are placed by Java code, ported
// from their bytecode: AE2 meteorites (appeng.worldgen.meteorite),
// Mowzie's Mobs wroughtnaut chambers and Ice and Fire dragon roosts and
// siren islands. Each reproduces the saved StructureStart piece box.

// ModTerrain is the noise-stage world a code-built structure reads while it
// picks its position. Height is the top solid block, Ground the base column
// (getBaseColumn: stone, fluid, air), Column the top solid and top non-air
// block (OCEAN_FLOOR_WG-1, WORLD_SURFACE_WG-1), Biome the noise biome.
// Column and Biome may be nil.
type ModTerrain struct {
	Height   HeightFunc
	Ground   Ground
	SeaLevel int
	Column   func(x, z int) (solidTop, top int)
	Biome    func(x, y, z int) string
}

// cols is the top solid and top non-air Y at x, z.
func (t ModTerrain) cols(x, z int) (solidTop, top int) {
	if t.Column != nil {
		return t.Column(x, z)
	}
	h := 63
	if t.Height != nil {
		h = t.Height(x, z)
	}
	top = h
	if t.SeaLevel > 0 && top < t.SeaLevel-1 {
		top = t.SeaLevel - 1
	}
	return h, top
}

// solid is BlockState.isSolid on the base column.
func (t ModTerrain) solid(x, y, z int) bool {
	if t.Ground != nil {
		return t.Ground(x, y, z) == GroundSolid
	}
	s, _ := t.cols(x, z)
	return y <= s
}

func isModStructureType(typ string) bool {
	switch typ {
	case "ae2:ae2mtrt", "mowziesmobs:wrought_chamber", "iceandfire:fire_dragon_roost", "iceandfire:ice_dragon_roost",
		"iceandfire:lightning_dragon_roost", "iceandfire:siren_island":
		return true
	}
	return false
}

// AssembleTerrain is AssembleGroundSea with the richer ModTerrain the mod
// adapters read (exact WORLD_SURFACE_WG, biome).
func AssembleTerrain(store *Store, structureID string, seed int64, originX, originZ int, dim DimHeight, t ModTerrain) (*Scene, error) {
	def, err := loadStructureDef(store, structureID)
	if err != nil {
		return nil, err
	}
	if isModStructureType(def.Type) {
		return assembleMod(store, def.Type, seed, originX, originZ, dim, t)
	}
	return AssembleGroundSea(store, structureID, seed, originX, originZ, t.Height, dim, t.Ground, t.SeaLevel)
}

func assembleMod(store *Store, typ string, seed int64, originX, originZ int, dim DimHeight, t ModTerrain) (*Scene, error) {
	cx, cz := originX>>4, originZ>>4
	switch typ {
	case "ae2:ae2mtrt":
		return assembleMeteorite(store, seed, cx, cz, dim, t), nil
	case "mowziesmobs:wrought_chamber":
		return assembleWroughtChamber(store, cx, cz, t)
	}
	return assembleIafPoint(store, typ, seed, cx, cz, t), nil
}

// assembleIafPoint is DragonRoostStructure / SirenIslandStructure
// .findGenerationPoint: random.nextDouble() (the spawn chance), a random
// rotation, then Structure.getLowestYIn5by5BoxOffset7Blocks; the piece box is
// that single block. The roost or island itself is drawn by postProcess code.
func assembleIafPoint(store *Store, typ string, seed int64, cx, cz int, t ModTerrain) *Scene {
	r := worldgen.LargeFeatureRandom(seed, int32(cx), int32(cz))
	r.NextDouble()
	rot := worldgen.Rotations[r.NextInt(4)]
	i, j := 5, 5
	switch rot {
	case "CLOCKWISE_90":
		i = -5
	case "CLOCKWISE_180":
		i, j = -5, -5
	case "COUNTERCLOCKWISE_90":
		j = -5
	}
	x, z := cx*16+7, cz*16+7
	y := math.MaxInt
	for _, c := range [4][2]int{{x, z}, {x, z + j}, {x + i, z}, {x + i, z + j}} {
		_, top := t.cols(c[0], c[1]) // getFirstOccupiedHeight(WORLD_SURFACE_WG)
		y = min(y, top)
	}
	s := &Scene{Seeded: true, Min: [3]int{x, y, z}, Max: [3]int{x, y, z}}
	s.Pieces = []PieceBox{{Min: [3]int{x, y, z}, Max: [3]int{x, y, z}, Name: typ}}
	iafPostProcessBox(store, s, typ, seed, cx, cz)
	return s
}

// Mowzie's Mobs ferrous_wroughtnaut generation_config height_max/height_min
// (mowziesmobs-common.toml).
const wroughtHeightMax, wroughtHeightMin = 50, 20

// assembleWroughtChamber is WroughtnautChamberStructure.generatePieces and
// tryWroughtChamber: from the chunk middle, scan an 8x8 grid (step 2) of base
// columns for the first air below min(height_max, OCEAN_FLOOR_WG top), the
// floor below it, then walk N/E/S/W up to 8 blocks for a 4-high wall; the
// wall's direction gives the rotation, and the chamber template sits 9
// blocks back from it (BlockPos(0,0,-9).rotate).
func assembleWroughtChamber(store *Store, cx, cz int, t ModTerrain) (*Scene, error) {
	mx, mz := cx*16+8, cz*16+8 // getMiddleBlockX/Z
	surf, _ := t.cols(mx, mz)
	hmax, hmin := wroughtHeightMax, wroughtHeightMin
	if hmax > surf {
		hmax = surf
	}
	dirs := [4]struct {
		dx, dz int
		rot    string
	}{{0, -1, "COUNTERCLOCKWISE_90"}, {1, 0, "NONE"}, {0, 1, "CLOCKWISE_90"}, {-1, 0, "CLOCKWISE_180"}}
	for dx := -8; dx < 8; dx += 2 {
		for dz := -8; dz < 8; dz += 2 {
			x, z := mx+dx, mz+dz
			air := math.MinInt
			for y := hmax; y > hmin; y-- {
				if !t.solid(x, y, z) {
					air = y
					break
				}
			}
			if air == math.MinInt {
				continue
			}
			floor := math.MinInt
			for y := air; y > hmin; y-- {
				if t.solid(x, y, z) {
					floor = y
					break
				}
			}
			if floor == math.MinInt {
				continue
			}
			for _, d := range dirs {
				px, py, pz := x, floor+1, z
				for i := 1; i <= 8; i++ {
					px += d.dx
					pz += d.dz
					y0 := py - 1
					for k := 1; t.solid(px, py, pz); k++ {
						if k == 4 {
							return placeWroughtChamber(store, px, y0, pz, d.rot)
						}
						py++
					}
				}
			}
		}
	}
	return &Scene{Seeded: true}, nil // no chamber in this chunk
}

func placeWroughtChamber(store *Store, x, y, z int, rot string) (*Scene, error) {
	off := worldgen.Transform([3]int{0, 0, -9}, rot, [3]int{})
	x, z = x+off[0], z+off[2]
	const id = "mowziesmobs:wroughtnaut_chamber"
	tp, err := LoadTemplate(store, id)
	if err != nil {
		return nil, err
	}
	blocks := make([]WorldBlock, 0, len(tp.Blocks))
	for _, b := range tp.Blocks {
		if b.State.Name == "minecraft:structure_block" { // BlockIgnoreProcessor.STRUCTURE_BLOCK
			continue
		}
		w := worldgen.Transform([3]int{b.X, b.Y, b.Z}, rot, [3]int{})
		blocks = append(blocks, WorldBlock{X: x + w[0], Y: y + w[1], Z: z + w[2], State: rotateState(b.State, rot)})
	}
	a := worldgen.Transform([3]int{0, 0, 0}, rot, [3]int{})
	c := worldgen.Transform([3]int{tp.Size[0] - 1, 0, tp.Size[2] - 1}, rot, [3]int{})
	s := BuildScene(blocks)
	s.Seeded = true
	s.Pieces = []PieceBox{{Name: id,
		Min: [3]int{x + min(a[0], c[0]), y, z + min(a[2], c[2])},
		Max: [3]int{x + max(a[0], c[0]), y + tp.Size[1] - 1, z + max(a[2], c[2])}}}
	return s, nil
}

// assembleMeteorite is MeteoriteStructure.generatePieces: a random centre in
// the chunk and radius 2..8, the crater floor from the mean (pulled toward
// the minimum on rough ground) of the base heights around it, on
// OCEAN_FLOOR_WG in c:is_ocean biomes and WORLD_SURFACE_WG elsewhere. The
// piece box is the chunk grown by 64 at the centre's Y. The blocks are the
// sky stone ball MeteoritePlacer puts at the centre (craters and fallout,
// which depend on the placed world, are not drawn).
func assembleMeteorite(store *Store, seed int64, cx, cz int, dim DimHeight, t ModTerrain) *Scene {
	r := worldgen.LargeFeatureRandom(seed, int32(cx), int32(cz))
	x := cx*16 + int(r.NextInt(16))
	z := cz*16 + int(r.NextInt(16))
	radius := r.NextFloat()*6 + 2
	crater := int(math.Ceil(float64(radius))) + 1
	ocean := false
	if t.Biome != nil {
		ocean = biomeInTag(store, t.Biome(x, t.SeaLevel, z), "c:is_ocean")
	} else {
		s, _ := t.cols(x, z)
		ocean = t.SeaLevel > 0 && s < t.SeaLevel-1 // no biome source: water over the centre
	}
	rr := int(max(1, radius*2))
	var n, mean, m2 float64 // Guava StatsAccumulator (Welford)
	lo := math.Inf(1)
	for i := -rr; i <= rr; i++ {
		for j := -rr; j <= rr; j++ {
			s, top := t.cols(x+i, z+j)
			h := top + 1
			if ocean {
				h = s + 1
			}
			v := float64(h)
			n++
			d := v - mean
			mean += d / n
			m2 += d * (v - mean)
			lo = min(lo, v)
		}
	}
	y := int(mean)
	if m2/n > 5 {
		y = int(float64(y) - (mean-lo)*0.75)
	}
	y -= crater
	y = max(dim.MinY+crater, y)
	var blocks []WorldBlock
	rad2 := float64(radius) * float64(radius)
	ir := int(math.Ceil(float64(radius)))
	for i := -ir; i <= ir; i++ {
		for j := -ir; j <= ir; j++ {
			for k := -ir; k <= ir; k++ {
				if float64(i*i+j*j+k*k) <= rad2 {
					blocks = append(blocks, WorldBlock{X: x + i, Y: y + j, Z: z + k, State: BlockState{Name: "ae2:sky_stone_block"}})
				}
			}
		}
	}
	s := BuildScene(blocks)
	s.Seeded = true
	s.Pieces = []PieceBox{{Name: "ae2:meteorite", Min: [3]int{cx*16 - 64, y, cz*16 - 64}, Max: [3]int{cx*16 + 15 + 64, y, cz*16 + 15 + 64}}}
	return s
}

// biomeInTag resolves a worldgen biome tag (nested tags included) over the
// store's layers; c:is_ocean falls back to minecraft:is_ocean when no layer
// defines it (NeoForge's tag includes the vanilla one).
func biomeInTag(store *Store, biome, tag string) bool {
	seen := map[string]bool{}
	var in func(tag string) (bool, bool)
	in = func(tag string) (found, defined bool) {
		if seen[tag] {
			return false, false
		}
		seen[tag] = true
		ns, name := Namespaced(tag)
		vals := store.ReadTagValues(DataPath(ns, "tags/worldgen/biome", name, ".json"))
		for _, v := range vals {
			v = strings.TrimSuffix(v, "?")
			if strings.HasPrefix(v, "#") {
				if f, _ := in(v[1:]); f {
					return true, true
				}
			} else if v == biome {
				return true, true
			}
		}
		return false, len(vals) > 0
	}
	found, defined := in(tag)
	if !defined && tag == "c:is_ocean" {
		found, _ = in("minecraft:is_ocean")
	}
	return found
}
