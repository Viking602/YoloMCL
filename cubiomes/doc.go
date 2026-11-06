// Package cubiomes predicts Minecraft Java Edition biomes in pure Go, from
// Beta 1.7 through release 26.3. It implements historical layer generators,
// modern climate trees, the Nether and End, and versioned large biomes.
//
// Generators are immutable and may be shared. Create one Sampler per goroutine
// to reuse scratch storage without locks or warm-query allocations. Biome queries
// do not generate terrain blocks or prove that a structure will generate.
// Beta 1.7 exposes climate biomes without sea-level estimation. JSON overrides
// are supported by the modern compiler; unsupported legacy overrides fail.
// Full structure finders and terrain block generation are outside this API.
// Modern Overworld depth graphs also support approximate elevation rasters.
package cubiomes
