package structure3d

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// TexCoverage reports how many block states in a layered store resolve to a
// textured model, per namespace. It walks assets/<ns>/blockstates/*.json,
// resolves the first variant (or the bare state for multipart), and counts
// the block as textured when at least one face found its PNG.
type TexCoverage struct {
	Blocks, Textured int
	Missing          []string // block ids with no textured face
	ByNS             map[string][2]int
}

// MeasureTexCoverage runs the walk. ns filters to one namespace ("" = all).
func MeasureTexCoverage(store *Store, ns string) TexCoverage {
	cov := TexCoverage{ByNS: map[string][2]int{}}
	c := newModelCache()
	paths := store.List("assets/")
	var keys []string
	for p := range paths {
		if strings.Contains(p, "/blockstates/") && strings.HasSuffix(p, ".json") {
			keys = append(keys, p)
		}
	}
	sort.Strings(keys)
	for _, p := range keys {
		rest := strings.TrimPrefix(p, "assets/")
		pns, name, _ := strings.Cut(rest, "/blockstates/")
		if ns != "" && pns != ns {
			continue
		}
		name = strings.TrimSuffix(name, ".json")
		id := pns + ":" + name
		state := BlockState{Name: id, Properties: firstVariantProps(store, p)}
		m := c.resolve(store, state)
		n := cov.ByNS[pns]
		n[0]++
		cov.Blocks++
		if m != nil && len(m.Elements) > 0 {
			n[1]++
			cov.Textured++
		} else {
			cov.Missing = append(cov.Missing, id)
		}
		cov.ByNS[pns] = n
	}
	return cov
}

func firstVariantProps(store *Store, p string) map[string]string {
	data, err := store.Read(p)
	if err != nil {
		return nil
	}
	var bs struct {
		Variants  map[string]json.RawMessage `json:"variants"`
		Multipart []struct {
			When map[string]any `json:"when"`
		} `json:"multipart"`
	}
	if json.Unmarshal(data, &bs) != nil {
		return nil
	}
	if len(bs.Variants) == 0 {
		// Multipart: a state the first conditional part draws (wall post,
		// fire side, wire dot), as a placed block always carries props.
		for _, part := range bs.Multipart {
			w := part.When
			if or, ok := w["OR"].([]any); ok && len(or) > 0 {
				w, _ = or[0].(map[string]any)
			}
			if len(w) == 0 {
				continue
			}
			props := map[string]string{}
			for k, v := range w {
				if sv, ok := v.(string); ok {
					sv, _, _ = strings.Cut(sv, "|")
					props[k] = sv
				} else {
					props[k] = strings.ToLower(strings.Trim(fmt.Sprint(v), " "))
				}
			}
			return props
		}
		return nil
	}
	var vk []string
	for k := range bs.Variants {
		vk = append(vk, k)
	}
	sort.Strings(vk)
	props := map[string]string{}
	for _, kv := range strings.Split(vk[0], ",") {
		if k, v, ok := strings.Cut(kv, "="); ok {
			props[k] = v
		}
	}
	return props
}
