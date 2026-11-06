package cubiomes

import (
	"context"
	"errors"
	"fmt"
	"math"
)

// TerraBlenderWeightedBiome is one ordered entry of an End biome table.
// Duplicate names are valid: they retain their individual weighted intervals.
type TerraBlenderWeightedBiome struct {
	Biome  string `json:"biome"`
	Weight int    `json:"weight"`
}

// TerraBlenderEndTable is one ordered weighted field and its zoom size.
type TerraBlenderEndTable struct {
	Size   int                         `json:"size"`
	Biomes []TerraBlenderWeightedBiome `json:"biomes"`
}

// TerraBlenderEndConfig describes the four independently sized End areas.
// Zero-total-weight tables use Minecraft's ocean biome, as BiomeInitialLayer
// overrides the generic region layer's default index.
type TerraBlenderEndConfig struct {
	Minecraft string               `json:"minecraft"`
	Version   string               `json:"terrablender_version"`
	Highlands TerraBlenderEndTable `json:"highlands"`
	Midlands  TerraBlenderEndTable `json:"midlands"`
	Edge      TerraBlenderEndTable `json:"edge"`
	Islands   TerraBlenderEndTable `json:"islands"`
}

type terraBlenderEnd struct{ areas [4]terraBlender }
type terraBlenderEndScratch struct{ areas [4]*terraBlenderScratch }

// WithTerraBlenderEnd applies the versioned TerraBlender End selection contract. The
// supplied generator retains its erosion density graph, including datapacks.
// TerraBlender evaluates that graph at quart-cell block coordinates; vanilla
// evaluates End biomes at section centers, which gives different edge results.
func WithTerraBlenderEnd(ctx context.Context, base *Generator, cfg TerraBlenderEndConfig) (*Generator, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if base == nil || base.end == nil || base.tbEnd != nil || base.tb != nil {
		return nil, errors.New("TerraBlender End requires an unwrapped End generator")
	}
	verified := cfg.Minecraft == "26.3" && cfg.Version == "26.3.0.0.9" || cfg.Minecraft == "1.21.1" && cfg.Version == "4.1.0.8"
	if !verified || base.version != cfg.Minecraft {
		return nil, fmt.Errorf("unsupported TerraBlender End/Minecraft combination %q/%q (generator %q)", cfg.Version, cfg.Minecraft, base.version)
	}
	if cfg.Version == "4.1.0.8" {
		// This release passes the edge entries to its island area constructor.
		// Preserve that observable game behavior, retaining the island size.
		cfg.Islands.Biomes = cfg.Edge.Biomes
	}
	g := *base
	g.names = append([]string(nil), base.names...)
	g.biomes = append([]string(nil), base.biomes...)
	g.tbEnd = &terraBlenderEnd{}
	palette, reachable := make(map[string]BiomeID, len(g.names)), make(map[string]bool, len(g.biomes))
	for i, name := range g.names {
		if name != "" {
			palette[name] = BiomeID(i)
		}
	}
	for _, name := range g.biomes {
		reachable[name] = true
	}
	add := func(name string) (BiomeID, error) {
		if name == "" || name == terraBlenderDeferred {
			return 0, errors.New("invalid TerraBlender End biome")
		}
		id, ok := palette[name]
		if !ok {
			if len(g.names) > math.MaxUint16 {
				return 0, errors.New("TerraBlender End biome palette is too large")
			}
			id = BiomeID(len(g.names))
			palette[name] = id
			g.names = append(g.names, name)
		}
		if !reachable[name] {
			reachable[name] = true
			g.biomes = append(g.biomes, name)
		}
		return id, nil
	}
	for i, table := range [...]TerraBlenderEndTable{cfg.Highlands, cfg.Midlands, cfg.Edge, cfg.Islands} {
		if table.Size < 2 || table.Size > 6 || len(table.Biomes) > 65536 {
			return nil, errors.New("TerraBlender End table exceeds supported limits")
		}
		area := &g.tbEnd.areas[i]
		area.regionSize = table.Size
		area.weights = make([]int, len(table.Biomes))
		area.values = make([]BiomeID, len(table.Biomes))
		for j, entry := range table.Biomes {
			if j&1023 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			if entry.Weight < 0 || int64(area.total)+int64(entry.Weight) > math.MaxInt32 {
				return nil, errors.New("invalid TerraBlender End weight")
			}
			area.total += entry.Weight
			area.weights[j] = area.total
			id, err := add(entry.Biome)
			if err != nil {
				return nil, err
			}
			area.values[j] = id
		}
		if area.total == 0 {
			id, err := add("minecraft:ocean")
			if err != nil {
				return nil, err
			}
			area.emptyValue = int(id)
		}
		area.foldConstant()
	}
	return &g, nil
}

func (tb *terraBlenderEnd) newScratch(seed int64) *terraBlenderEndScratch {
	s := &terraBlenderEndScratch{}
	for i := range s.areas {
		s.areas[i] = tb.areas[i].newScratch(seed)
	}
	return s
}

func (s *Sampler) terraBlenderEndBiome(x, y, z int) (BiomeID, bool) {
	qx, qy, qz := int32(x)>>2, int32(y)>>2, int32(z)>>2
	sx, sz := int64(qx>>2), int64(qz>>2)
	if sx*sx+sz*sz <= 4096 {
		return biomeTheEnd, true
	}
	s.scratch.at(int(qx<<2), int(qy<<2), int(qz<<2))
	v := s.g.end.erosion.eval(&s.scratch)
	area := 2 // edge; NaN also takes this branch in Java
	switch {
	case v > .25:
		area = 0
	case v >= -.0625:
		area = 1
	case v < -.21875:
		area = 3
	}
	state, field := s.tbEnd.areas[area], &s.g.tbEnd.areas[area]
	return BiomeID(state.region(field, len(state.layers)-1, qx, qz)), true
}
