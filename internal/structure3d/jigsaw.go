package structure3d

import (
	"encoding/json"
	"strings"

	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// UnsupportedError means a structure uses a piece type we refuse to invent.
type UnsupportedError struct {
	Kind   string // KindCodeGenerated, KindUnsupportedPiece, KindEmpty
	Reason string
}

func (e *UnsupportedError) Error() string { return e.Reason }

// HeightFunc returns surface Y for heightmap projection. Nil means Y=0.
type HeightFunc func(x, z int) int

// StructureDef is the worldgen/structure JSON we understand.
type StructureDef struct {
	Type                    string          `json:"type"`
	StartPool               string          `json:"start_pool"`
	MineshaftType           string          `json:"mineshaft_type"`
	Size                    int             `json:"size"`
	MaxDistanceFromCenter   json.RawMessage `json:"max_distance_from_center"` // int, or {"horizontal","vertical"} in 1.21.2+
	StartHeight             json.RawMessage `json:"start_height"`
	ProjectStartToHeightmap string          `json:"project_start_to_heightmap"`
	UseExpansionHack        bool            `json:"use_expansion_hack"`
	worldgen.OceanRuinConfig
	IsBeached bool                         `json:"is_beached"`
	Setups    []worldgen.RuinedPortalSetup `json:"setups"`
	// Single-template structures (e.g. some mods) may set start_pool empty
	// and use a direct location via custom fields — those are unsupported unless
	// type is jigsaw or we can load a lone template by ID.
}

// Assemble builds the structure whose start is at (originX, originZ) in the
// overworld. seed drives piece selection. height may be nil.
func Assemble(store *Store, structureID string, seed int64, originX, originZ int, height HeightFunc) (*Scene, error) {
	return AssembleIn(store, structureID, seed, originX, originZ, height, OverworldHeight)
}

// AssembleIn is Assemble for a dimension with the given build height.
func AssembleIn(store *Store, structureID string, seed int64, originX, originZ int, height HeightFunc, dim DimHeight) (*Scene, error) {
	return AssembleGround(store, structureID, seed, originX, originZ, height, dim, nil)
}

// AssembleGround is AssembleIn with a terrain sampler for the code-built
// structures (fortress, mineshaft, stronghold, temples) whose vanilla pieces read the
// world: support pillars, skip-air boxes and floor patches over caves.
func AssembleGround(store *Store, structureID string, seed int64, originX, originZ int, height HeightFunc, dim DimHeight, ground Ground) (*Scene, error) {
	return AssembleGroundSea(store, structureID, seed, originX, originZ, height, dim, ground, 0)
}

// AssembleGroundSea is AssembleGround with the dimension's sea level, for
// a height that is the solid noise terrain (no water): jigsaw starts on
// WORLD_SURFACE_WG then rest on the sea surface, OCEAN_FLOOR starts on the
// sea bed. seaLevel 0 treats the height as the start surface everywhere.
func AssembleGroundSea(store *Store, structureID string, seed int64, originX, originZ int, height HeightFunc, dim DimHeight, ground Ground, seaLevel int) (*Scene, error) {
	def, err := loadStructureDef(store, structureID)
	if err != nil {
		return nil, err
	}
	switch {
	case def.Type == "minecraft:jigsaw" || strings.HasSuffix(def.Type, ":jigsaw") || def.Type == "yungsapi:yung_jigsaw" ||
		def.Type == "legendary_monsters:lm_jigsaw" || def.Type == "touhou_little_maid_spell:relic_sanctum":
		return assembleJigsaw(store, structureID, seed, originX, originZ, height, dim, seaLevel)
	case def.Type == "" && def.StartPool != "":
		return assembleJigsaw(store, structureID, seed, originX, originZ, height, dim, seaLevel)
	case def.Type == "minecraft:mineshaft":
		return assembleMineshaft(seed, def.MineshaftType == "mesa", originX, originZ, height, ground)
	case def.Type == "minecraft:shipwreck":
		return assembleShipwreck(store, def.IsBeached, seed, originX, originZ, height)
	case def.Type == "minecraft:ruined_portal" && len(def.Setups) > 0:
		return assembleRuinedPortal(store, def.Setups, seed, originX, originZ, height, dim, ground, seaLevel)
	case def.Type == "minecraft:stronghold":
		return assembleStronghold(seed, decoIndex(store, structureID, "minecraft:stronghold"), originX, originZ, height, ground), nil
	case def.Type == "minecraft:swamp_hut":
		return assembleTemple("minecraft:swamp_hut", decoIndex(store, structureID, "minecraft:swamp_hut"), seed, originX, originZ, height, ground), nil
	case def.Type == "minecraft:desert_pyramid":
		return assembleTemple("minecraft:desert_pyramid", decoIndex(store, structureID, "minecraft:desert_pyramid"), seed, originX, originZ, height, ground), nil
	case def.Type == "minecraft:jungle_temple":
		return assembleTemple("minecraft:jungle_pyramid", decoIndex(store, structureID, "minecraft:jungle_pyramid"), seed, originX, originZ, height, ground), nil
	case def.Type == "minecraft:buried_treasure":
		return assembleBuriedTreasure(seed, originX, originZ, height), nil
	case def.Type == "minecraft:ocean_ruin":
		return assembleOceanRuin(store, def.OceanRuinConfig, seed, originX, originZ, height)
	case def.Type == "minecraft:ocean_monument":
		return assembleMonument(seed, originX, originZ, store.MonumentYOffset()), nil
	case def.Type == "minecraft:woodland_mansion":
		return assembleMansion(store, seed, originX, originZ, height)
	case def.Type == "minecraft:fortress":
		return assembleFortress(seed, originX, originZ, ground), nil
	case def.Type == "minecraft:end_city":
		return assembleEndCity(store, seed, originX, originZ, height)
	case def.Type == "minecraft:igloo":
		return assembleIgloo(store, seed, originX, originZ, height)
	case isModStructureType(def.Type):
		return assembleMod(store, def.Type, seed, originX, originZ, dim, ModTerrain{Height: height, Ground: ground, SeaLevel: seaLevel})
	default:
		// Non-jigsaw types are placed by Java code (vanilla igloo, mod
		// structures). A template sharing the structure id is the only shape
		// we can show without inventing geometry.
		if store.HasTemplate(structureID) {
			t, err := LoadTemplate(store, structureID)
			if err != nil {
				return nil, err
			}
			return templateScene(t, originX, 0, originZ, height), nil
		}
		return nil, &UnsupportedError{Kind: KindCodeGenerated, Reason: "structure type " + def.Type + " is generated by code"}
	}
}

// AssembleTemplate loads one NBT template without jigsaw expansion.
func AssembleTemplate(store *Store, templateID string, originX, originY, originZ int) (*Scene, error) {
	t, err := LoadTemplate(store, templateID)
	if err != nil {
		return nil, err
	}
	blocks := make([]WorldBlock, 0, len(t.Blocks))
	for _, b := range t.Blocks {
		blocks = append(blocks, WorldBlock{X: originX + b.X, Y: originY + b.Y, Z: originZ + b.Z, State: b.State})
	}
	return BuildScene(blocks), nil
}

func loadStructureDef(store *Store, id string) (*StructureDef, error) {
	ns, name := Namespaced(id)
	data, err := store.Read(DataPath(ns, "worldgen/structure", name, ".json"))
	if err != nil {
		return nil, &PreviewError{Kind: KindMissingStructure, ID: id, Err: err}
	}
	var def StructureDef
	if err := json.Unmarshal(lenientJSON(data), &def); err != nil {
		return nil, &PreviewError{Kind: KindBadStructure, ID: id, Err: err}
	}
	return &def, nil
}

// assembleJigsaw places the pieces with PlaceJigsaw and fills them with
// their template blocks. The layout is seed-exact; it stays approximate when
// a piece's height came from the estimated terrain.
func assembleJigsaw(store *Store, structureID string, seed int64, ox, oz int, height HeightFunc, dim DimHeight, seaLevel int) (*Scene, error) {
	var free, floor FreeHeightFunc
	if height != nil {
		floor = func(x, z int) int { return height(x, z) + 1 }
		free = floor
		if seaLevel != 0 {
			// getFirstFreeHeight(WORLD_SURFACE_WG): noise-stage water counts.
			free = func(x, z int) int { return max(height(x, z)+1, seaLevel) }
		}
	}
	layout, err := PlaceJigsawFloor(store, structureID, seed, floorDiv16(ox), floorDiv16(oz), free, floor, dim)
	if err != nil {
		return nil, err
	}
	var blocks []WorldBlock
	seen := map[[3]int]int{}
	ctx := &procCtx{store: store, height: height, sea: seaLevel, tags: map[string]map[string]bool{}}
	ctx.place = func(w [3]int, st BlockState) {
		if _, dup := seen[w]; dup {
			return
		}
		seen[w] = len(blocks)
		blocks = append(blocks, WorldBlock{X: w[0], Y: w[1], Z: w[2], State: st})
	}
	ctx.existing = func(w [3]int) (BlockState, bool) {
		if i, ok := seen[w]; ok {
			return blocks[i].State, true
		}
		return BlockState{}, false
	}
	if len(layout.Pieces) > 0 {
		b := layout.Pieces[0].BB
		ctx.pivot = [3]int{b[0] + (b[3]-b[0]+1)/2, b[1], b[2] + (b[5]-b[2]+1)/2}
	}
	// Repeated pieces share immutable decoded templates within this assembly.
	templates := map[string]*Template{}
	for _, p := range layout.Pieces {
		for _, el := range p.Elements {
			t, loaded := templates[el.Location]
			if !loaded {
				t, _ = LoadTemplate(store, el.Location)
				templates[el.Location] = t
			}
			if t == nil {
				continue // vanilla places nothing for a missing template
			}
			list, approx, _ := loadProcessors(store, el.Processors)
			if approx {
				ctx.approx = true
			}
			procs := pieceProcessors(list, el.Legacy, !el.Rigid)
			rot := RotationName(p.Rotation)
			src := t.Blocks
			if len(t.Jigsaw) > 0 {
				src = make([]PlacedBlock, len(t.Blocks), len(t.Blocks)+len(t.Jigsaw))
				copy(src, t.Blocks)
			}
			for _, j := range t.Jigsaw {
				src = append(src, PlacedBlock{X: j.X, Y: j.Y, Z: j.Z, State: BlockState{Name: "minecraft:jigsaw", Properties: map[string]string{"orientation": j.Orientation}}})
			}
			for i, b := range src {
				local := [3]int{b.X, b.Y, b.Z}
				pb := &procBlock{local: local, world: add(rotPos(local, p.Rotation), p.Pos), state: b.State}
				if i >= len(t.Blocks) {
					pb.nbt = map[string]any{"final_state": t.Jigsaw[i-len(t.Blocks)].FinalState}
				}
				for _, pr := range procs {
					if pb = pr.process(ctx, *pb); pb == nil {
						break
					}
				}
				if pb == nil || pb.state.Name == "minecraft:air" || pb.state.Name == "minecraft:structure_void" {
					continue
				}
				w := pb.world
				if _, dup := seen[w]; dup {
					continue
				}
				seen[w] = len(blocks)
				blocks = append(blocks, WorldBlock{X: w[0], Y: w[1], Z: w[2], State: rotateState(pb.state, rot)})
			}
		}
	}
	if len(blocks) == 0 {
		return nil, &UnsupportedError{Kind: KindEmpty, Reason: "jigsaw assembly produced no blocks"}
	}
	s := BuildScene(blocks)
	for _, p := range layout.Pieces {
		b := p.RawBB
		s.Pieces = append(s.Pieces, PieceBox{Min: [3]int{b[0], b[1], b[2]}, Max: [3]int{b[3], b[4], b[5]}, Name: p.Location, Depth: p.Depth})
	}
	s.LayoutSeeded = true
	s.Seeded = !layout.TerrainDependent && !ctx.approx && !ctx.gravity
	return s, nil
}

func floorDiv16(v int) int { return v >> 4 }

func templateScene(t *Template, ox, oy, oz int, height HeightFunc) *Scene {
	if height != nil {
		oy = height(ox, oz)
	}
	blocks := make([]WorldBlock, 0, len(t.Blocks))
	for _, b := range t.Blocks {
		blocks = append(blocks, WorldBlock{X: ox + b.X, Y: oy + b.Y, Z: oz + b.Z, State: b.State})
	}
	return BuildScene(blocks)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func horizontalDistance(raw json.RawMessage) int {
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return n
	}
	var o struct {
		Horizontal int `json:"horizontal"`
	}
	_ = json.Unmarshal(raw, &o)
	return o.Horizontal
}

// lenientJSON strips // and /* */ comments and trailing commas outside
// strings, matching the lenient reader Minecraft uses for data packs.
func lenientJSON(b []byte) []byte {
	out := make([]byte, 0, len(b))
	inStr, esc := false, false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if inStr {
			out = append(out, c)
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch {
		case c == '"':
			inStr = true
			out = append(out, c)
		case c == '/' && i+1 < len(b) && b[i+1] == '/':
			for i < len(b) && b[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(b) && b[i+1] == '*':
			i += 2
			for i+1 < len(b) && !(b[i] == '*' && b[i+1] == '/') {
				i++
			}
			i++
		case c == '}' || c == ']':
			j := len(out) - 1
			for j >= 0 && (out[j] == ' ' || out[j] == '\t' || out[j] == '\r' || out[j] == '\n') {
				j--
			}
			if j >= 0 && out[j] == ',' {
				out = append(out[:j], out[j+1:]...)
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return out
}
