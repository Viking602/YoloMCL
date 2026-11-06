package structure3d

import (
	_ "embed"
	"encoding/json"
)

// knownModPoolEdits lists template-pool additions that mods make from code
// (NeoForge ServerAboutToStart / registry events, ported from bytecode), in
// mod load order: Immersive Engineering, Farmer's Delight, Iron's Spells,
// Kaleidoscope Cookery, Touhou Little Maid Spell, Tsuki, TofuCraft.
//
//go:embed modpools.json
var knownModPoolEdits []byte

// ApplyKnownModPoolEdits adds each known code-side pool edit whose content
// the store can resolve (its template, or its MergeFrom pool), so only mods
// that are actually loaded contribute. Mod configs that disable villages
// are not read. Call after mods are opened. Returns the edits added.
func ApplyKnownModPoolEdits(s *Store) int {
	var all []PoolEdit
	if json.Unmarshal(knownModPoolEdits, &all) != nil {
		return 0
	}
	var add []PoolEdit
	for _, e := range all {
		if e.MergeFrom != "" {
			ns, name := Namespaced(e.MergeFrom)
			if _, err := s.Read(DataPath(ns, "worldgen/template_pool", name, ".json")); err == nil {
				add = append(add, e)
			}
			continue
		}
		var el struct {
			Location string `json:"location"`
		}
		if json.Unmarshal(e.Element, &el) == nil && el.Location != "" && s.HasTemplate(el.Location) {
			add = append(add, e)
		}
	}
	s.AddPoolEdits(add...)
	return len(add)
}
