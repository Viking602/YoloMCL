package structure3d

import (
	"encoding/json"

	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// assembleShipwreck places the seed-chosen shipwreck template exactly as
// ShipwreckStructure does (pivot 4,0,15 at the chunk's min corner). The
// vertical fit is approximate: vanilla averages the ocean floor over the
// footprint; we sample the surface at the hull centre.
func assembleShipwreck(store *Store, beached bool, seed int64, x, z int, height HeightFunc) (*Scene, error) {
	tp, rot := worldgen.Shipwreck(seed, int32(x>>4), int32(z>>4), beached)
	t, err := LoadTemplate(store, tp)
	if err != nil {
		return nil, err
	}
	pos := [3]int{x >> 4 << 4, 0, z >> 4 << 4}
	s := placeTemplate(t, pos, "NONE", rot, [3]int{4, 0, 15}, height, beached)
	return s, nil
}

// assembleRuinedPortal follows RuinedPortalStructure.findGenerationPoint:
// setup, template, rotation and mirror from the seed, pivot at the template's
// horizontal centre, origin at the chunk's min corner.
//
// height is the top solid Y of the terrain (no water) and seaLevel
// the dimension's sea level (0: none). Lithostitched's template lists apply
// when the store runs with breaks_seed_parity. ground, when set, answers the
// corner columns of findSuitableY; without it a column is solid up to its
// surface.
func assembleRuinedPortal(store *Store, setups []worldgen.RuinedPortalSetup, seed int64, x, z int, height HeightFunc, dim DimHeight, ground Ground, seaLevel int) (*Scene, error) {
	var standard, giant []string
	if store.LithostitchedJigsaw() {
		standard = store.templateList("ruined_portal/standard")
		giant = store.templateList("ruined_portal/giant")
	}
	p := worldgen.RuinedPortalFull(seed, int32(x>>4), int32(z>>4), setups, standard, giant)
	t, err := LoadTemplate(store, p.Template)
	if err != nil {
		return nil, err
	}
	pos := [3]int{x >> 4 << 4, 0, z >> 4 << 4}
	pivot := [3]int{t.Size[0] / 2, 0, t.Size[2] / 2}
	a := worldgen.TransformMirror([3]int{0, 0, 0}, p.Mirror, p.Rotation, pivot)
	b := worldgen.TransformMirror([3]int{t.Size[0] - 1, 0, t.Size[2] - 1}, p.Mirror, p.Rotation, pivot)
	minX, maxX := pos[0]+min(a[0], b[0]), pos[0]+max(a[0], b[0])
	minZ, maxZ := pos[2]+min(a[2], b[2]), pos[2]+max(a[2], b[2])
	floor := p.Placement == "on_ocean_floor"
	free := func(x, z int) int { // first free Y on the placement heightmap
		if height == nil {
			return 64
		}
		h := height(x, z) + 1
		if !floor && seaLevel > 0 && h < seaLevel {
			h = seaLevel
		}
		return h
	}
	corners := [4][2]int{{minX, minZ}, {maxX, minZ}, {minX, maxZ}, {maxX, maxZ}}
	solid := func(y, c int) bool {
		cx, cz := corners[c][0], corners[c][1]
		if ground != nil {
			switch ground(cx, y, cz) {
			case GroundSolid:
				return true
			case GroundLiquid:
				return !floor
			}
			return false
		}
		return y < free(cx, cz)
	}
	cxm, czm := minX+(maxX-minX+1)/2, minZ+(maxZ-minZ+1)/2 // BoundingBox.getCenter
	y := worldgen.RuinedPortalY(p, dim.MinY, free(cxm, czm)-1, t.Size[1], solid)
	blocks := make([]WorldBlock, 0, len(t.Blocks))
	for _, bl := range t.Blocks {
		w := worldgen.TransformMirror([3]int{bl.X, bl.Y, bl.Z}, p.Mirror, p.Rotation, pivot)
		blocks = append(blocks, WorldBlock{X: pos[0] + w[0], Y: y + w[1], Z: pos[2] + w[2], State: rotateState(mirrorState(bl.State, p.Mirror), p.Rotation)})
	}
	s := BuildScene(blocks)
	s.Seeded = true
	s.Pieces = []PieceBox{{Min: [3]int{minX, y, minZ}, Max: [3]int{maxX, y + t.Size[1] - 1, maxZ}, Name: p.Template}}
	return s, nil
}

// templateList reads a Lithostitched template list
// (data/lithostitched/lithostitched/template_list/<path>.json).
func (s *Store) templateList(path string) []string {
	b, err := s.Read("data/lithostitched/lithostitched/template_list/" + path + ".json")
	if err != nil {
		return nil
	}
	var out []string
	if json.Unmarshal(lenientJSON(b), &out) != nil {
		return nil
	}
	return out
}

func placeTemplate(t *Template, pos [3]int, mirror, rotation string, pivot [3]int, height HeightFunc, sink bool) *Scene {
	c := worldgen.TransformMirror([3]int{t.Size[0] / 2, 0, t.Size[2] / 2}, mirror, rotation, pivot)
	y := 64
	if height != nil {
		y = height(pos[0]+c[0], pos[2]+c[2])
	}
	if sink {
		y -= t.Size[1] / 2
	}
	blocks := make([]WorldBlock, 0, len(t.Blocks))
	for _, b := range t.Blocks {
		w := worldgen.TransformMirror([3]int{b.X, b.Y, b.Z}, mirror, rotation, pivot)
		blocks = append(blocks, WorldBlock{X: pos[0] + w[0], Y: y + w[1], Z: pos[2] + w[2], State: rotateState(mirrorState(b.State, mirror), rotation)})
	}
	s := BuildScene(blocks)
	s.Seeded = true
	return s
}

// mirrorState applies Mirror.FRONT_BACK / LEFT_RIGHT to facing and to
// connection properties.
func mirrorState(st BlockState, mirror string) BlockState {
	a, b := "east", "west"
	switch mirror {
	case "FRONT_BACK":
	case "LEFT_RIGHT":
		a, b = "north", "south"
	default:
		return st
	}
	if len(st.Properties) == 0 {
		return st
	}
	props := make(map[string]string, len(st.Properties))
	for k, v := range st.Properties {
		props[k] = v
	}
	if f := props["facing"]; f == a {
		props["facing"] = b
	} else if f == b {
		props["facing"] = a
	}
	va, oka := st.Properties[a]
	vb, okb := st.Properties[b]
	if oka && okb {
		props[a], props[b] = vb, va
	}
	return BlockState{Name: st.Name, Properties: props}
}
