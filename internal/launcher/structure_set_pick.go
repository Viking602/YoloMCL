package launcher

import (
	"encoding/json"
	"strings"

	"github.com/Viking602/YoloMCL/internal/structure3d"
	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// PickSetStructure mirrors ChunkGenerator.tryGenerateStructure's choice for a
// structure set with several weighted structures: a large-feature random
// seeded for the start chunk draws by weight; a structure whose biome tag does
// not hold the biome at the start position is dropped and the draw repeats.
// biomeAt returns the biome id at a block position (y is the start height).
// ok is false when the set has a single structure or cannot be read.
func PickSetStructure(store *structure3d.Store, setID string, seed int64, x, z int, biomeAt func(x, y, z int) string) (string, bool) {
	ns, name := structure3d.Namespaced(setID)
	data, err := store.Read(structure3d.DataPath(ns, "worldgen/structure_set", name, ".json"))
	if err != nil {
		return "", false
	}
	var set struct {
		Structures []struct {
			Structure string `json:"structure"`
			Weight    int    `json:"weight"`
		} `json:"structures"`
	}
	if json.Unmarshal(data, &set) != nil || len(set.Structures) < 2 {
		return "", false
	}
	type entry struct {
		id string
		w  int
	}
	var list []entry
	total := 0
	for _, s := range set.Structures {
		w := max(s.Weight, 1)
		list = append(list, entry{s.Structure, w})
		total += w
	}
	cx, cz := int32(x>>4), int32(z>>4)
	r := worldgen.LargeFeatureRandom(seed, cx, cz)
	for len(list) > 0 {
		k := int(r.NextInt(int32(total)))
		i := 0
		for ; i < len(list); i++ {
			k -= list[i].w
			if k < 0 {
				break
			}
		}
		e := list[i]
		if structureBiomeOK(store, e.id, int(cx)*16, int(cz)*16, biomeAt) {
			return e.id, true
		}
		list = append(list[:i:i], list[i+1:]...)
		total -= e.w
	}
	return "", false
}

func structureBiomeOK(store *structure3d.Store, id string, x, z int, biomeAt func(x, y, z int) string) bool {
	if biomeAt == nil {
		return true
	}
	ns, name := structure3d.Namespaced(id)
	data, err := store.Read(structure3d.DataPath(ns, "worldgen/structure", name, ".json"))
	if err != nil {
		return true
	}
	var def struct {
		Biomes      json.RawMessage `json:"biomes"`
		StartHeight struct {
			Absolute *int `json:"absolute"`
		} `json:"start_height"`
	}
	if json.Unmarshal(data, &def) != nil {
		return true
	}
	y := 64
	if def.StartHeight.Absolute != nil {
		y = *def.StartHeight.Absolute
	}
	b := biomeAt(x, y, z)
	if b == "" {
		return true
	}
	return biomeInList(store, def.Biomes, b, 0)
}

func biomeInList(store *structure3d.Store, raw json.RawMessage, biome string, depth int) bool {
	if depth > 8 || len(raw) == 0 {
		return true
	}
	var one string
	if json.Unmarshal(raw, &one) == nil {
		if strings.HasPrefix(one, "#") {
			ns, name := structure3d.Namespaced(one[1:])
			data, err := store.Read(structure3d.DataPath(ns, "tags/worldgen/biome", name, ".json"))
			if err != nil {
				return true
			}
			var tag struct {
				Values []json.RawMessage `json:"values"`
			}
			if json.Unmarshal(data, &tag) != nil {
				return true
			}
			for _, v := range tag.Values {
				var obj struct {
					ID string `json:"id"`
				}
				if json.Unmarshal(v, &obj) == nil && obj.ID != "" {
					v, _ = json.Marshal(obj.ID)
				}
				if biomeInList(store, v, biome, depth+1) {
					return true
				}
			}
			return false
		}
		return normID(one) == normID(biome)
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		for _, s := range many {
			v, _ := json.Marshal(s)
			if biomeInList(store, v, biome, depth+1) {
				return true
			}
		}
		return false
	}
	return true
}

func normID(s string) string {
	if !strings.Contains(s, ":") {
		return "minecraft:" + s
	}
	return s
}
