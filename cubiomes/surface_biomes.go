package cubiomes

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// SurfaceRules' biome condition accepts a holder set in 26.2: one id, a list,
// or a biome tag. Older releases use the same list form.
func (sc *surfaceCompiler) biomes(raw json.RawMessage) (map[string]bool, error) {
	var ids []string
	var id string
	if json.Unmarshal(raw, &id) == nil && id != "" {
		ids = []string{id}
	} else if json.Unmarshal(raw, &ids) != nil || ids == nil {
		return nil, errors.New("invalid surface biome set")
	}
	set, active := map[string]bool{}, map[string]bool{}
	var add func(string, bool, int) error
	add = func(id string, required bool, depth int) error {
		if err := sc.c.budget(depth); err != nil {
			return err
		}
		if id == "" || id == "#" {
			return errors.New("empty surface biome id")
		}
		if !strings.HasPrefix(id, "#") {
			set[seedMapID(id)] = true
			return nil
		}
		id = seedMapID(strings.TrimPrefix(id, "#"))
		if active[id] {
			return fmt.Errorf("cyclic surface biome tag %s", id)
		}
		doc, ok := sc.c.docs["biome_tag/"+id]
		if !ok {
			if !required {
				return nil
			}
			return fmt.Errorf("missing surface biome tag %s", id)
		}
		var tag struct {
			Values []json.RawMessage `json:"values"`
		}
		if json.Unmarshal(doc.Data, &tag) != nil || tag.Values == nil {
			return fmt.Errorf("invalid surface biome tag %s", id)
		}
		active[id] = true
		defer delete(active, id)
		for _, value := range tag.Values {
			var entry struct {
				ID       string `json:"id"`
				Required *bool  `json:"required"`
			}
			if json.Unmarshal(value, &entry.ID) != nil && json.Unmarshal(value, &entry) != nil {
				return fmt.Errorf("invalid surface biome tag entry %s", id)
			}
			if err := add(entry.ID, entry.Required == nil || *entry.Required, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	for _, id := range ids {
		if err := add(id, true, 0); err != nil {
			return nil, err
		}
	}
	return set, nil
}
