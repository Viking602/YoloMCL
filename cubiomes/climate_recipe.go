package cubiomes

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
)

// ClimateParameterRewrite describes a source registration operation. Matching
// uses the original box, before any rewrites, just like parameter-map builders.
// An empty Biome preserves the original biome; later matching rewrites win.
type ClimateParameterRewrite struct {
	Match [7][2]int64           `json:"match"`
	With  TerraBlenderParameter `json:"with"`
}

// RewriteClimateParameters implements the shared copy/replace/append contract
// used by region-registration APIs. It retains insertion order and duplicates,
// which are observable when Minecraft's climate tree resolves distance ties.
func RewriteClimateParameters(ctx context.Context, base []TerraBlenderParameter, replacements []ClimateParameterRewrite, additions []TerraBlenderParameter) ([]TerraBlenderParameter, error) {
	if len(base)+len(additions) > 262144 || len(replacements) > 4096 {
		return nil, fmt.Errorf("climate recipe exceeds parameter limits")
	}
	result := slices.Clone(base)
	bySpace := make(map[[7][2]int64]TerraBlenderParameter, len(replacements))
	for _, rewrite := range replacements {
		bySpace[rewrite.Match] = rewrite.With
	}
	for i, original := range base {
		if i&255 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		if replacement, ok := bySpace[original.Space]; ok {
			result[i].Space = replacement.Space
			if replacement.Biome != "" {
				result[i].Biome = replacement.Biome
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return append(result, additions...), nil
}

var orderedOverworld1201 = sync.OnceValues(func() ([]TerraBlenderParameter, error) {
	raw, err := readSeedMapBuiltin("overworld-1.20.1-parameters.json")
	if err != nil {
		return nil, err
	}
	var parameters []TerraBlenderParameter
	err = json.Unmarshal(raw, &parameters)
	return parameters, err
})

// BuiltinClimateParameters returns an owned, ordered parameter list for source
// adapters that explicitly copy the vanilla builder. This is distinct from a
// base region, which always uses the supplied world's effective datapack tree.
func BuiltinClimateParameters(ctx context.Context, minecraft, preset string) ([]TerraBlenderParameter, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if minecraft != "1.20.1" || preset != "minecraft:overworld" {
		return nil, fmt.Errorf("ordered climate parameters unavailable for %s / %s", minecraft, preset)
	}
	parameters, err := orderedOverworld1201()
	return slices.Clone(parameters), err
}
