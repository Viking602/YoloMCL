package cubiomes_test

import (
	"context"
	"fmt"

	"github.com/Viking602/YoloMCL/cubiomes"
)

func ExampleNew() {
	g, err := cubiomes.New(context.Background(), cubiomes.Config{
		Version: "1.20.1", Seed: 0, Dimension: cubiomes.Overworld,
	})
	if err != nil {
		panic(err)
	}
	worker := g.NewSampler()
	biome, ok := worker.NoiseBiome(0, 64, 0)
	fmt.Println(biome, ok)
	// Output: minecraft:river true
}

func ExampleSampler_Generate() {
	g, err := cubiomes.New(context.Background(), cubiomes.Config{Version: "1.20.1", Seed: 0, Dimension: cubiomes.End})
	if err != nil {
		panic(err)
	}
	worker := g.NewSampler()
	biomes := make([]string, 16*16)
	err = worker.Generate(context.Background(), cubiomes.Range{
		X: 0, Y: 64, Z: 0, SizeX: 16, SizeY: 1, SizeZ: 16, StepXZ: 4, StepY: 4,
	}, biomes)
	fmt.Println(biomes[0], biomes[255], err)
	// Output: minecraft:the_end minecraft:the_end <nil>
}
