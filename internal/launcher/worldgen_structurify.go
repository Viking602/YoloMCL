package launcher

import (
	"encoding/json"
	"math"
	"strings"
)

// structurifyConfig is the subset of Structurify's config/structurify.json
// that moves structure-set grids.
type structurifyConfig struct {
	General struct {
		EnableGlobal bool    `json:"enable_global_spacing_and_separation_modifier"`
		Global       float64 `json:"global_spacing_and_separation_modifier"`
	} `json:"general"`
	Structures []struct {
		Name     string `json:"name"`
		Disabled bool   `json:"is_disabled"`
	} `json:"structures"`
	Sets []struct {
		Name       string   `json:"name"`
		Spacing    int      `json:"spacing"`
		Separation int      `json:"separation"`
		Salt       *int64   `json:"salt"`
		Frequency  *float64 `json:"frequency"`
		Override   bool     `json:"override_global_spacing_and_separation_modifier"`
	} `json:"structure_sets"`
}

// rewriteStructurifyDocuments applies Structurify's random-spread rules:
// a listed set takes its salt and frequency; spacing and separation are
// either the listed values (global modifier off, or the set overrides it) or
// int(original*modifier); spacing is at least 1 and separation is clamped to
// [0, spacing-1]. Sets whose every structure is disabled are dropped.
func rewriteStructurifyDocuments(documents map[string]WorldgenDocument, raw []byte) {
	var cfg structurifyConfig
	if len(raw) == 0 || json.Unmarshal(raw, &cfg) != nil {
		return
	}
	disabled := map[string]bool{}
	for _, s := range cfg.Structures {
		if s.Disabled {
			disabled[s.Name] = true
		}
	}
	sets := map[string]int{}
	for i, s := range cfg.Sets {
		sets[s.Name] = i
	}
	for key, doc := range documents {
		if doc.Kind != "structure_set" {
			continue
		}
		var data map[string]json.RawMessage
		if json.Unmarshal(doc.Data, &data) != nil {
			continue
		}
		var structures []struct {
			Structure string `json:"structure"`
		}
		if json.Unmarshal(data["structures"], &structures) == nil && len(structures) > 0 {
			all := true
			for _, s := range structures {
				all = all && disabled[s.Structure]
			}
			if all {
				delete(documents, key)
				continue
			}
		}
		var placement map[string]json.RawMessage
		if json.Unmarshal(data["placement"], &placement) != nil {
			continue
		}
		var kind string
		if json.Unmarshal(placement["type"], &kind) == nil && seedMapSpreadFamilies[seedMapID(kind)].scale != 0 {
			continue // own grid fields: the mixin never sees them
		}
		var spacing, separation int
		if json.Unmarshal(placement["spacing"], &spacing) != nil || json.Unmarshal(placement["separation"], &separation) != nil {
			continue // not a random spread placement
		}
		idx, listed := sets[doc.ID]
		switch {
		case cfg.General.EnableGlobal && !(listed && cfg.Sets[idx].Override):
			spacing = int(float64(spacing) * cfg.General.Global)
			separation = int(float64(separation) * cfg.General.Global)
		case listed:
			spacing, separation = cfg.Sets[idx].Spacing, cfg.Sets[idx].Separation
		default:
			continue
		}
		spacing = max(1, spacing)
		separation = max(0, separation)
		if separation >= spacing {
			separation = spacing - 1
		}
		placement["spacing"], _ = json.Marshal(spacing)
		placement["separation"], _ = json.Marshal(separation)
		if listed {
			if s := cfg.Sets[idx]; s.Salt != nil {
				placement["salt"], _ = json.Marshal(*s.Salt)
			}
			if f := cfg.Sets[idx].Frequency; f != nil && !math.IsNaN(*f) {
				placement["frequency"], _ = json.Marshal(*f)
			}
		}
		data["placement"], _ = json.Marshal(placement)
		if b, err := json.Marshal(data); err == nil {
			doc.Data = b
			if !strings.Contains(doc.Source, "+structurify") {
				doc.Source += "+structurify"
			}
			documents[key] = doc
		}
	}
}
