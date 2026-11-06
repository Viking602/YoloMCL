package structure3d

import (
	"fmt"
)

// BlockState is one palette entry.
type BlockState struct {
	Name       string
	Properties map[string]string
}

// PlacedBlock is one block in a template, after palette resolve.
type PlacedBlock struct {
	X, Y, Z int
	State   BlockState
}

// JigsawConn is a jigsaw block connection point inside a template.
type JigsawConn struct {
	X, Y, Z                   int
	Name, Target, Pool, Joint string
	Orientation               string // facing property when present
	PlacementPriority         int
	SelectionPriority         int
	FinalState                string // jigsaw final_state (JigsawReplacementProcessor)
}

// Template is a decoded structure NBT.
type Template struct {
	ID     string
	Size   [3]int
	Blocks []PlacedBlock
	Jigsaw []JigsawConn
}

// templatePaths are the jar paths a template id may live at: structure/ (1.21+)
// then the pre-1.21 structures/ folder that many mods still ship.
func templatePaths(id string) []string {
	ns, name := Namespaced(id)
	return []string{DataPath(ns, "structure", name, ".nbt"), DataPath(ns, "structures", name, ".nbt")}
}

// HasTemplate reports whether any layer holds the template id.
func (s *Store) HasTemplate(id string) bool {
	for _, p := range templatePaths(id) {
		if _, err := s.Read(p); err == nil {
			return true
		}
	}
	return false
}

// LoadTemplate reads data/<ns>/structure/<path>.nbt (or structures/).
func LoadTemplate(store *Store, id string) (*Template, error) {
	candidates := templatePaths(id)
	var data []byte
	var err error
	for _, p := range candidates {
		data, err = store.Read(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, &PreviewError{Kind: KindMissingTemplate, ID: id, Err: err}
	}
	root, err := DecodeNBT(data)
	if err != nil {
		return nil, &PreviewError{Kind: KindBadTemplate, ID: id, Err: err}
	}
	t := &Template{ID: id}
	if size, ok := root["size"].([]any); ok && len(size) >= 3 {
		t.Size = [3]int{asInt(size[0]), asInt(size[1]), asInt(size[2])}
	}
	palette := []BlockState{}
	rawPalette, ok := root["palette"].([]any)
	if !ok {
		// Multi-palette templates (shipwrecks) store "palettes"; vanilla
		// picks one per placement, we show the first.
		if ps, ok2 := root["palettes"].([]any); ok2 && len(ps) > 0 {
			rawPalette, ok = ps[0].([]any)
		}
	}
	if raw := rawPalette; ok {
		for _, e := range raw {
			m, _ := e.(map[string]any)
			st := BlockState{Name: asString(m["Name"]), Properties: map[string]string{}}
			if props, ok := m["Properties"].(map[string]any); ok {
				for k, v := range props {
					st.Properties[k] = asString(v)
				}
			}
			palette = append(palette, st)
		}
	}
	blocks, _ := root["blocks"].([]any)
	for _, e := range blocks {
		m, _ := e.(map[string]any)
		pos, _ := m["pos"].([]any)
		if len(pos) < 3 {
			continue
		}
		si := asInt(m["state"])
		if si < 0 || si >= len(palette) {
			continue
		}
		st := palette[si]
		x, y, z := asInt(pos[0]), asInt(pos[1]), asInt(pos[2])
		if st.Name == "minecraft:air" || st.Name == "minecraft:structure_void" {
			continue
		}
		if st.Name == "minecraft:jigsaw" {
			nbt, _ := m["nbt"].(map[string]any)
			t.Jigsaw = append(t.Jigsaw, JigsawConn{
				X: x, Y: y, Z: z,
				Name: asString(nbt["name"]), Target: asString(nbt["target"]),
				Pool: asString(nbt["pool"]), Joint: asString(nbt["joint"]),
				Orientation:       st.Properties["orientation"],
				PlacementPriority: asInt(nbt["placement_priority"]),
				SelectionPriority: asInt(nbt["selection_priority"]),
				FinalState:        asString(nbt["final_state"]),
			})
			continue
		}
		t.Blocks = append(t.Blocks, PlacedBlock{X: x, Y: y, Z: z, State: st})
	}
	return t, nil
}

func asInt(v any) int {
	switch n := v.(type) {
	case int32:
		return int(n)
	case int64:
		return int(n)
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}

func asString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	default:
		return fmt.Sprint(v)
	}
}
