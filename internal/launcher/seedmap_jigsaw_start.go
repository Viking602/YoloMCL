package launcher

import (
	"encoding/json"
	"strings"
)

// JigsawStartPiece is one entry of a start pool expanded by weight, as
// StructureTemplatePool.getRandomTemplate draws it. Size is the template's
// x/z footprint; Empty marks an empty_pool_element.
type JigsawStartPiece struct {
	Size  [2]int
	Empty bool
}

// seedMapJigsawStart is what JigsawPlacement.addPieces needs before
// Structure.isValidBiome: the biome is sampled at the generation stub, the
// centre of the rotated start piece at the projected start height, not at
// the chunk corner.
type seedMapJigsawStart struct {
	pool       string
	heightmap  bool
	yung       bool // yungsapi:yung_jigsaw; heightmap biome uses occupied column (see jigsawProbe)
	absY       int
	uniform    bool // uniform start_height between minY and maxY
	minY, maxY int
	// integrated_api valid_biome_radius_check (chunks); 0 = none.
	biomeRadius int
}

// seedMapJigsawLike are structure types whose findGenerationPoint draws
// start height, rotation and start piece like JigsawStructure and checks the
// biome at the start piece centre (integrated_api's PieceLimitedJigsawManager,
// verified with javap on ModAdaptiveStructure.findGenerationPoint).
var seedMapJigsawLike = map[string]bool{
	"minecraft:jigsaw":                      true,
	"integrated_api:mod_adaptive_structure": true,
	"integrated_api:generic_structure":      true, // JigsawStructure: same findGenerationPoint shape (javap)
	"goety:crypt":                           true, // CryptStructure._findGenerationPoint -> JigsawPlacement (javap)
	"graveyard:crypt":                       true, // TGJigsawStructure -> JigsawPlacement (javap); also seedMapIsJigsawLike
	"ars_nouveau:wilden_den":                true, // WildenDen.findGenerationPoint -> JigsawPlacement (javap)
	"tlc:lost_castle":                       true, // LostCastle.findGenerationPoint -> JigsawPlacement (javap)
	"legendary_monsters:lm_jigsaw":          true, // CollapsedKingdom / lm_jigsaw -> JigsawPlacement (javap)
	"yungsapi:yung_jigsaw":                  true, // YungJigsawStructure height then YungJigsawManager (javap)
	"touhou_little_maid_spell:land_jigsaw":  true, // LandJigsawStructure -> JigsawPlacement (javap)
}

// seedMapIsJigsawLike is seedMapJigsawLike plus The Graveyard's shared
// TGJigsawStructure types (every graveyard:* structure JSON uses that class).
func seedMapIsJigsawLike(typ string) bool {
	id := seedMapID(typ)
	if seedMapJigsawLike[id] {
		return true
	}
	return strings.HasPrefix(id, "graveyard:")
}

func compileJigsawStart(docs map[string]WorldgenDocument, id string) *seedMapJigsawStart {
	doc, ok := docs["structure/"+id]
	if !ok {
		return nil
	}
	var def struct {
		Type        string          `json:"type"`
		StartPool   string          `json:"start_pool"`
		Project     string          `json:"project_start_to_heightmap"`
		StartJigsaw string          `json:"start_jigsaw_name"`
		StartHeight json.RawMessage `json:"start_height"`
		BiomeRadius int             `json:"valid_biome_radius_check"`
	}
	// start_jigsaw_name only shifts the stub to a named jigsaw block inside the
	// start piece (XZ). Height and rotation still follow JigsawStructure; we
	// keep the piece-centre XZ approximation so absolute Y (ancient_city -27)
	// is not lost to the y=64 fallback.
	if json.Unmarshal(doc.Data, &def) != nil || !seedMapIsJigsawLike(def.Type) || def.StartPool == "" {
		return nil
	}
	// Constant / uniform heights only: other height providers draw from the
	// same random before the pool and rotation.
	var abs struct {
		Absolute *int   `json:"absolute"`
		Type     string `json:"type"`
		Value    struct {
			Absolute *int `json:"absolute"`
		} `json:"value"` // minecraft:constant
		Min struct {
			Absolute *int `json:"absolute"`
		} `json:"min_inclusive"`
		Max struct {
			Absolute *int `json:"absolute"`
		} `json:"max_inclusive"`
	}
	if json.Unmarshal(def.StartHeight, &abs) != nil {
		return nil
	}
	if abs.Absolute == nil && seedMapID(abs.Type) == "minecraft:constant" {
		abs.Absolute = abs.Value.Absolute
	}
	if abs.Absolute == nil {
		if seedMapID(abs.Type) != "minecraft:uniform" || abs.Min.Absolute == nil || abs.Max.Absolute == nil || *abs.Min.Absolute > *abs.Max.Absolute {
			return nil
		}
		yung := seedMapID(def.Type) == "yungsapi:yung_jigsaw"
		return &seedMapJigsawStart{pool: seedMapID(def.StartPool), heightmap: def.Project != "", yung: yung, uniform: true, minY: *abs.Min.Absolute, maxY: *abs.Max.Absolute, biomeRadius: iapiRadius(def.Type, def.BiomeRadius)}
	}
	yung := seedMapID(def.Type) == "yungsapi:yung_jigsaw"
	return &seedMapJigsawStart{pool: seedMapID(def.StartPool), heightmap: def.Project != "", yung: yung, absY: *abs.Absolute, biomeRadius: iapiRadius(def.Type, def.BiomeRadius)}
}

func iapiRadius(typ string, r int) int {
	if r < 1 || r > 8 || !strings.HasPrefix(seedMapID(typ), "integrated_api:") {
		return 0
	}
	return r
}

// JigsawStartPools lists the start pools whose pieces the sampler needs.
func (s *LocalSampler) JigsawStartPools() []string {
	seen := map[string]bool{}
	var out []string
	add := func(j *seedMapJigsawStart) {
		if j != nil && !seen[j.pool] {
			seen[j.pool] = true
			out = append(out, j.pool)
		}
	}
	for _, p := range s.placements {
		add(p.jigsaw)
		// multi-structure sets probe each structure's own start piece
		for _, pr := range p.probes {
			add(pr.jigsaw)
		}
	}
	return out
}

// SetJigsawStartPools supplies resolved start pools. Call it once before the
// sampler is shared; pools left out keep the chunk-corner biome check.
func (s *LocalSampler) SetJigsawStartPools(pools map[string][]JigsawStartPiece) {
	s.jigsawPools = pools
}

// biomeProbe returns where vanilla checks a single-structure set's biome.
func (s *LocalSampler) biomeProbe(p seedMapPlacement, dimension string, x, y, z int) (int, int, int) {
	if p.jigsaw == nil {
		return x, y, z
	}
	return s.jigsawProbe(p.jigsaw, dimension, x, y, z)
}
