package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

type seedMapTag struct {
	Replace bool              `json:"replace"`
	Values  []json.RawMessage `json:"values"`
	Remove  []json.RawMessage `json:"remove,omitempty"` // NeoForge: applied after all values
}

func mergeSeedMapTag(previous, current json.RawMessage) (json.RawMessage, error) {
	var a, b seedMapTag
	if string(current) == "null" || json.Unmarshal(current, &b) != nil || b.Values == nil {
		return nil, errors.New("invalid biome tag")
	}
	if !b.Replace && len(previous) > 0 {
		if string(previous) == "null" || json.Unmarshal(previous, &a) != nil {
			return nil, errors.New("invalid lower priority biome tag")
		}
		b.Values = append(a.Values, b.Values...)
		b.Remove = append(a.Remove, b.Remove...)
	}
	if len(b.Values) > 16384 {
		return nil, errors.New("biome tag entry limit")
	}
	return json.Marshal(b)
}

func resolveSeedMapBiomes(ctx context.Context, docs map[string]WorldgenDocument, raw json.RawMessage, active map[string]bool, budget *int) (map[string]bool, bool) {
	return resolveSeedMapRegistry(ctx, docs, "biome", raw, active, budget)
}

func resolveSeedMapRegistry(ctx context.Context, docs map[string]WorldgenDocument, kind string, raw json.RawMessage, active map[string]bool, budget *int) (map[string]bool, bool) {
	*budget--
	if ctx.Err() != nil || *budget < 0 || len(active) > 64 {
		return nil, false
	}
	var id string
	if json.Unmarshal(raw, &id) == nil && id != "" {
		if !strings.HasPrefix(id, "#") {
			return map[string]bool{seedMapID(id): true}, true
		}
		id = seedMapID(strings.TrimPrefix(id, "#"))
		if active[id] {
			return nil, false
		}
		var tag seedMapTag
		doc, found := docs[kind+"_tag/"+id]
		if !found {
			// MappedRegistry.bindTags binds every referenced but undefined
			// tag to an empty set: the structure exists but never matches.
			return map[string]bool{}, true
		}
		if json.Unmarshal(doc.Data, &tag) != nil || tag.Values == nil {
			return nil, false
		}
		active[id] = true
		defer delete(active, id)
		out := map[string]bool{}
		for _, entry := range tag.Values {
			var value string
			required := true
			if json.Unmarshal(entry, &value) != nil {
				var object struct {
					ID       string `json:"id"`
					Required *bool  `json:"required"`
				}
				if json.Unmarshal(entry, &object) != nil || object.ID == "" {
					return nil, false
				}
				value = object.ID
				if object.Required != nil {
					required = *object.Required
				}
			}
			if value == "" {
				return nil, false
			}
			if !required {
				key := kind + "/" + seedMapID(value)
				if strings.HasPrefix(value, "#") {
					key = kind + "_tag/" + seedMapID(value[1:])
				}
				if _, exists := docs[key]; !exists {
					continue
				}
			}
			if strings.HasPrefix(value, "#") {
				if _, exists := docs[kind+"_tag/"+seedMapID(value[1:])]; !exists {
					// TagLoader drops a tag with a missing required reference.
					return map[string]bool{}, true
				}
			}
			b, err := json.Marshal(value)
			if err != nil {
				return nil, false
			}
			values, ok := resolveSeedMapRegistry(ctx, docs, kind, b, active, budget)
			if !ok {
				return nil, false
			}
			for value := range values {
				out[value] = true
			}
		}
		for _, entry := range tag.Remove {
			if values, ok := resolveSeedMapRegistry(ctx, docs, kind, entry, active, budget); ok {
				for value := range values {
					delete(out, value)
				}
			}
		}
		return out, true
	}
	if out, handled, ok := resolveSeedMapHolderSetObject(ctx, docs, kind, raw, active, budget); handled {
		return out, ok
	}
	var entries []json.RawMessage
	if json.Unmarshal(raw, &entries) != nil || entries == nil || len(entries) > 16384 {
		return nil, false
	}
	out := map[string]bool{}
	for _, entry := range entries {
		values, ok := resolveSeedMapRegistry(ctx, docs, kind, entry, active, budget)
		if !ok {
			return nil, false
		}
		for value := range values {
			out[value] = true
		}
	}
	return out, true
}

func (s *LocalSampler) compilePlacementBiomes(ctx context.Context, p *seedMapPlacement, docs map[string]WorldgenDocument) {
	p.allowed = map[string]bool{}
	budget := 32768
	for _, id := range p.structures {
		var structure struct {
			Biomes json.RawMessage `json:"biomes"`
		}
		doc, found := docs["structure/"+id]
		if !found || json.Unmarshal(doc.Data, &structure) != nil {
			s.addIssue("structure_biomes_unknown", p.id)
			p.probes = nil
			return
		}
		values, ok := resolveSeedMapBiomes(ctx, docs, structure.Biomes, map[string]bool{}, &budget)
		if !ok {
			s.addIssue("structure_biomes_unknown", p.id+" / "+id+" "+seedMapMissingTag(docs, "biome", structure.Biomes, 0))
			p.probes = nil
			return
		}
		one := map[string]bool{}
		for value := range values {
			p.allowed[value] = true
			one[value] = true
		}
		p.probes = append(p.probes, compileProbe(docs, id, one))
	}
	// Union of all weighted alternatives: never guess which building won.
	p.biomesKnown = len(p.structures) > 0
	if len(p.structures) == 1 {
		p.jigsaw = compileJigsawStart(docs, p.structures[0])
	}
}

// Resolve once per immutable registry, not once per UI frame or candidate.
// Namespaces are not mod IDs: an enabled pack may add or override another
// namespace, and one structure set may legitimately span several dimensions.
func (s *LocalSampler) bindPlacementDimensions() {
	for i := range s.placements {
		p := &s.placements[i]
		p.dimensions = make(map[string]bool)
		for id, d := range s.dimensions {
			if p.dimension != "" && p.dimension != id {
				continue
			}
			if p.biomesKnown {
				if len(p.allowed) == 0 {
					continue
				}
				if d.mode != "" {
					matches := p.allowed[d.biome]
					for _, biome := range d.biomes {
						matches = matches || p.allowed[biome]
					}
					if !matches {
						continue
					}
				}
			}
			p.dimensions[id] = true
		}
	}
}

func (p seedMapPlacement) inDimension(dimension string) bool {
	if p.dimensions != nil {
		return p.dimensions[dimension]
	}
	return p.dimension == "" || p.dimension == dimension
}

// resolveSeedMapHolderSetObject evaluates NeoForge's composite holder sets
// (neoforge:and/or/not/any) over the registry entries the snapshot carries.
func resolveSeedMapHolderSetObject(ctx context.Context, docs map[string]WorldgenDocument, kind string, raw json.RawMessage, active map[string]bool, budget *int) (map[string]bool, bool, bool) {
	var object struct {
		Type   string            `json:"type"`
		Values []json.RawMessage `json:"values"`
		Value  json.RawMessage   `json:"value"`
	}
	if len(raw) == 0 || raw[0] != '{' || json.Unmarshal(raw, &object) != nil {
		return nil, false, false
	}
	universe := func() map[string]bool {
		all := map[string]bool{}
		for key := range docs {
			if strings.HasPrefix(key, kind+"/") {
				all[key[len(kind)+1:]] = true
			}
		}
		return all
	}
	kind2 := object.Type
	if strings.HasPrefix(kind2, "forge:") {
		kind2 = "neo" + kind2
	}
	switch kind2 {
	case "neoforge:any":
		return universe(), true, true
	case "neoforge:not":
		inner, ok := resolveSeedMapRegistry(ctx, docs, kind, object.Value, active, budget)
		if !ok {
			return nil, true, false
		}
		out := universe()
		for id := range inner {
			delete(out, id)
		}
		return out, true, true
	case "neoforge:and", "neoforge:or":
		var out map[string]bool
		for _, v := range object.Values {
			set, ok := resolveSeedMapRegistry(ctx, docs, kind, v, active, budget)
			if !ok {
				return nil, true, false
			}
			switch {
			case out == nil:
				out = set
			case strings.HasSuffix(object.Type, "forge:or"):
				for id := range set {
					out[id] = true
				}
			default:
				for id := range out {
					if !set[id] {
						delete(out, id)
					}
				}
			}
		}
		if out == nil {
			out = map[string]bool{}
		}
		return out, true, true
	}
	return nil, true, false
}

// seedMapMissingTag names the first required tag reference the snapshot
// cannot resolve, for coverage diagnostics.
func seedMapMissingTag(docs map[string]WorldgenDocument, kind string, raw json.RawMessage, depth int) string {
	if depth > 32 {
		return "cycle"
	}
	var id string
	if json.Unmarshal(raw, &id) == nil {
		if !strings.HasPrefix(id, "#") {
			return ""
		}
		doc, found := docs[kind+"_tag/"+seedMapID(id[1:])]
		var tag seedMapTag
		if !found || json.Unmarshal(doc.Data, &tag) != nil || tag.Values == nil {
			return id
		}
		for _, entry := range tag.Values {
			var object struct {
				ID       string `json:"id"`
				Required *bool  `json:"required"`
			}
			if json.Unmarshal(entry, &object) == nil && object.ID != "" {
				if object.Required != nil && !*object.Required {
					continue
				}
				entry, _ = json.Marshal(object.ID)
			}
			if m := seedMapMissingTag(docs, kind, entry, depth+1); m != "" {
				return m
			}
		}
		return ""
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) == nil {
		for _, e := range list {
			if m := seedMapMissingTag(docs, kind, e, depth+1); m != "" {
				return m
			}
		}
		return ""
	}
	return "holder_set:" + string(raw[:min(len(raw), 60)])
}
