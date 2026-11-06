package worldgen

// DecorationRandom is the WorldgenRandom ChunkGenerator.applyBiomeDecoration
// hands a structure's StructureStart.placeInChunk: setDecorationSeed for the
// chunk origin, then setFeatureSeed(index, step) where index is the
// structure's position among the registry's structures in that
// GenerationStep.Decoration and step is the step ordinal.
type DecorationRandom struct{ r worldgenRandom }

// Decoration step ordinals (GenerationStep.Decoration).
const (
	StepUndergroundStructures = 3
	StepSurfaceStructures     = 4
	StepStrongholds           = 5
	StepUndergroundDecoration = 7
)

func NewDecorationRandom(seed int64, cx, cz, index, step int) *DecorationRandom {
	d := &DecorationRandom{}
	deco := d.r.setDecorationSeed(seed, cx*16, cz*16)
	d.r.setFeatureSeed(deco, index, step)
	return d
}

func (d *DecorationRandom) NextInt(n int32) int32 { return d.r.nextInt(n) }
func (d *DecorationRandom) NextFloat() float32    { return d.r.nextFloat() }
func (d *DecorationRandom) NextBoolean() bool     { return d.r.next(1) != 0 }
func (d *DecorationRandom) NextLong() int64       { return d.r.nextLong() }
