// Package worldgen reproduces vanilla structure-piece randomness so the seed
// map can predict piece layouts. It is pure Go and checked against real saves.
package worldgen

// JavaRandom is java.util.Random (LegacyRandomSource).
type JavaRandom struct{ seed int64 }

const (
	mult = 0x5DEECE66D
	mask = (1 << 48) - 1
)

func (r *JavaRandom) SetSeed(s int64) { r.seed = (s ^ mult) & mask }

func (r *JavaRandom) next(bits uint) int32 {
	r.seed = (r.seed*mult + 0xB) & mask
	return int32(r.seed >> (48 - bits))
}

func (r *JavaRandom) NextIntFull() int32 { return r.next(32) }

func (r *JavaRandom) NextInt(bound int32) int32 {
	if bound&(-bound) == bound {
		return int32((int64(bound) * int64(r.next(31))) >> 31)
	}
	for {
		b := r.next(31)
		v := b % bound
		if b-v+(bound-1) >= 0 {
			return v
		}
	}
}

func (r *JavaRandom) NextLong() int64 {
	return int64(r.next(32))<<32 + int64(r.next(32))
}

func (r *JavaRandom) NextDouble() float64 {
	return float64(int64(r.next(26))<<27+int64(r.next(27))) * (1.0 / (1 << 53))
}

// LargeFeatureRandom is WorldgenRandom.setLargeFeatureSeed(seed, cx, cz), the
// random handed to Structure.findGenerationPoint.
func LargeFeatureRandom(seed int64, cx, cz int32) *JavaRandom {
	r := &JavaRandom{}
	r.SetSeed(seed)
	a, b := r.NextLong(), r.NextLong()
	r.SetSeed(int64(cx)*a ^ int64(cz)*b ^ seed)
	return r
}

// Rotations in vanilla enum order.
var Rotations = [4]string{"NONE", "CLOCKWISE_90", "CLOCKWISE_180", "COUNTERCLOCKWISE_90"}

// IglooLayout is the seeded part of an igloo.
type IglooLayout struct {
	Rotation string
	Basement bool
	Depth    int // ladder segments when Basement
}

// Pieces is the saved piece count (top, plus bottom and depth-1 middles).
func (l IglooLayout) Pieces() int {
	if !l.Basement {
		return 1
	}
	return l.Depth + 1
}

// Igloo mirrors IglooStructure.generatePieces / IglooPieces.addPieces.
func Igloo(seed int64, cx, cz int32) IglooLayout {
	r := LargeFeatureRandom(seed, cx, cz)
	l := IglooLayout{Rotation: Rotations[r.NextInt(4)]}
	if r.NextDouble() < 0.5 {
		l.Basement = true
		l.Depth = int(r.NextInt(8) + 4)
	}
	return l
}

// MineshaftRoomSize mirrors the MineShaftRoom constructor's random X/Z extent
// (BB width-1 and depth-1). skipDouble covers the nextDouble() call made in
// MineshaftStructure.findGenerationPoint before the room is built.
func MineshaftRoomSize(seed int64, cx, cz int32, skipDouble bool) (dx, dy, dz int) {
	r := LargeFeatureRandom(seed, cx, cz)
	if skipDouble {
		r.NextDouble()
	}
	dx = 7 + int(r.NextInt(6))
	dy = 4 + int(r.NextInt(6))
	dz = 7 + int(r.NextInt(6))
	return
}
