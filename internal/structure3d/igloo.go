package structure3d

import "github.com/Viking602/YoloMCL/internal/worldgen"

// assembleIgloo places igloo top/middle/bottom exactly as IglooStructure does
// for the world seed (verified piece-for-piece against saved worlds).
func assembleIgloo(store *Store, seed int64, originX, originZ int, height HeightFunc) (*Scene, error) {
	pieces := worldgen.IglooPieces(seed, int32(originX>>4), int32(originZ>>4))
	shift := 0
	if height != nil && len(pieces) > 0 {
		t := pieces[len(pieces)-1].World([3]int{3, 0, 0}) // top is placed last
		shift = height(t[0], t[2]) - 91
	}
	var blocks []WorldBlock
	for _, p := range pieces {
		t, err := LoadTemplate(store, p.Template)
		if err != nil {
			return nil, err
		}
		for _, b := range t.Blocks {
			w := p.World([3]int{b.X, b.Y, b.Z})
			blocks = append(blocks, WorldBlock{X: w[0], Y: w[1] + shift, Z: w[2], State: rotateState(b.State, p.Rotation)})
		}
	}
	s := BuildScene(blocks)
	s.Seeded = true
	return s, nil
}

var horizontal = []string{"north", "east", "south", "west"}

func rotateState(st BlockState, rotation string) BlockState {
	steps := map[string]int{"CLOCKWISE_90": 1, "CLOCKWISE_180": 2, "COUNTERCLOCKWISE_90": 3}[rotation]
	if steps == 0 || len(st.Properties) == 0 {
		return st
	}
	props := make(map[string]string, len(st.Properties))
	for k, v := range st.Properties {
		props[k] = v
	}
	if f, ok := props["facing"]; ok {
		for i, h := range horizontal {
			if h == f {
				props["facing"] = horizontal[(i+steps)%4]
			}
		}
	}
	if a, ok := props["axis"]; ok && steps%2 == 1 {
		if a == "x" {
			props["axis"] = "z"
		} else if a == "z" {
			props["axis"] = "x"
		}
	}
	return BlockState{Name: st.Name, Properties: props}
}
