package modjar

import (
	"context"
	"fmt"

	"github.com/Viking602/YoloMCL/cubiomes"
)

type nativeBiomeSampler struct {
	runtime *BiomeRuntime
	input   *cubiomes.NoiseRouterSampler
}

func (p BiomeProgram) NewBiomeSampler(ctx context.Context, input *cubiomes.NoiseRouterSampler) (cubiomes.CustomBiomeSampler, error) {
	if p.NoiseScope != nil {
		required := input.Requirements()
		if required.NormalNoise && !p.NoiseScope.NormalNoise || required.EndIslands && !p.NoiseScope.EndIslands {
			return nil, fmt.Errorf("noise router exceeds captured transformation scope")
		}
	}
	runtime, err := p.NewRuntime(ctx)
	if err != nil {
		return nil, err
	}
	if p.EndHeight != nil {
		// The stateless proof makes this bounded direct cache valid across
		// adjacent quart queries; no map can grow with a long pan across a world.
		type heightCell struct {
			x, z  int32
			value float32
			valid bool
		}
		cache := make([]heightCell, 1024)
		input.SetEndIslandHeight(func(simplex func(float64, float64) float64, x, z int32) (float32, bool) {
			cell := &cache[(uint32(x)*0x9e3779b1^uint32(z)*0x85ebca77)&1023]
			if cell.valid && cell.x == x && cell.z == z {
				return cell.value, true
			}
			value, ok := runtime.EndHeight(simplex, x, z)
			if ok {
				*cell = heightCell{x, z, value, true}
			}
			return value, ok
		})
	}
	return &nativeBiomeSampler{runtime: runtime, input: input}, nil
}

func (s *nativeBiomeSampler) NoiseBiome(ctx context.Context, x, y, z int32) (string, error) {
	return s.runtime.Query(ctx, x, y, z, s.input)
}
