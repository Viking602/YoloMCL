package nativeui

import (
	"encoding/json"

	"github.com/Viking602/YoloMCL/internal/launcher"
	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// bindJigsawStarts reads the start pools the sampler's jigsaw placements use
// so their biome checks sample the start piece centre like vanilla.
func bindJigsawStarts(store *structure3d.Store, sampler *launcher.LocalSampler) {
	pools := map[string][]launcher.JigsawStartPiece{}
	for _, id := range sampler.JigsawStartPools() {
		ns, name := structure3d.Namespaced(id)
		data, err := store.Read(structure3d.DataPath(ns, "worldgen/template_pool", name, ".json"))
		if err != nil {
			continue
		}
		var pool struct {
			Elements []struct {
				Weight  int `json:"weight"`
				Element struct {
					Type     string          `json:"element_type"`
					Location json.RawMessage `json:"location"`
				} `json:"element"`
			} `json:"elements"`
		}
		if json.Unmarshal(data, &pool) != nil {
			continue
		}
		var pieces []launcher.JigsawStartPiece
		ok := true
		for _, e := range pool.Elements {
			var piece launcher.JigsawStartPiece
			switch e.Element.Type {
			case "minecraft:empty_pool_element", "empty_pool_element":
				piece.Empty = true
			case "minecraft:single_pool_element", "single_pool_element", "minecraft:legacy_single_pool_element", "legacy_single_pool_element",
				"yungsapi:yung_single_element", "yung_single_element",
				"yungsapi:max_count_single_element", "max_count_single_element",
				"yungsapi:max_count_legacy_single_element", "max_count_legacy_single_element":
				var loc string
				if json.Unmarshal(e.Element.Location, &loc) != nil {
					ok = false
					break
				}
				t, err := structure3d.LoadTemplate(store, loc)
				if err != nil {
					ok = false
					break
				}
				piece.Size = [2]int{t.Size[0], t.Size[2]}
			default:
				ok = false
			}
			if !ok || e.Weight < 1 || e.Weight > 150 || len(pieces) > 4096 {
				ok = false
				break
			}
			for range e.Weight {
				pieces = append(pieces, piece)
			}
		}
		if ok && len(pieces) > 0 {
			pools[id] = pieces
		}
	}
	sampler.SetJigsawStartPools(pools)
}
