package cubiomes

import (
	"crypto/md5" // Minecraft's positional seed hash, not a security primitive.
	"encoding/binary"
	"math"
	"math/bits"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Minecraft's RandomSource and PositionalRandomFactory. Arithmetic deliberately
// wraps at the Java integer widths. See docs/seed-map.md for reference sources.
type noiseRandom struct {
	lo, hi uint64
	legacy bool
}

func newNoiseRandom(seed int64, legacy bool) *noiseRandom {
	if legacy {
		return &noiseRandom{lo: (uint64(seed) ^ 0x5deece66d) & ((1 << 48) - 1), legacy: true}
	}
	lo := uint64(seed) ^ 0x6a09e667f3bcc909
	return &noiseRandom{lo: noiseMix(lo), hi: noiseMix(lo + 0x9e3779b97f4a7c15)}
}

func noiseMix(n uint64) uint64 {
	n = (n ^ (n >> 30)) * 0xbf58476d1ce4e5b9
	n = (n ^ (n >> 27)) * 0x94d049bb133111eb
	return n ^ (n >> 31)
}

func (r *noiseRandom) nextBits(n uint) uint64 {
	r.lo = (r.lo*0x5deece66d + 11) & ((1 << 48) - 1)
	return r.lo >> (48 - n)
}

// Exponentiation of the affine LCG transition gives exactly the same 48-bit
// state as count calls to nextBits, with logarithmic rather than linear work.
func (r *noiseRandom) skipLegacy(count uint64) {
	mul, add := uint64(0x5deece66d), uint64(11)
	for count != 0 {
		if count&1 != 0 {
			r.lo = r.lo*mul + add
		}
		add *= mul + 1
		mul *= mul
		count >>= 1
	}
	r.lo &= (1 << 48) - 1
}

func (r *noiseRandom) nextLong() uint64 {
	if r.legacy {
		a, b := int64(int32(r.nextBits(32))), int64(int32(r.nextBits(32)))
		return uint64((a << 32) + b)
	}
	lo, hi := r.lo, r.hi
	result := bits.RotateLeft64(lo+hi, 17) + lo
	hi ^= lo
	r.lo, r.hi = bits.RotateLeft64(lo, 49)^hi^(hi<<21), bits.RotateLeft64(hi, 28)
	return result
}

func (r *noiseRandom) nextDouble() float64 {
	if r.legacy {
		return float64(r.nextBits(26)<<27+r.nextBits(27)) * 0x1p-53
	}
	return float64(r.nextLong()>>11) * 0x1p-53
}

func (r *noiseRandom) nextInt(bound int) int {
	if r.legacy {
		if bound&(bound-1) == 0 {
			return int((uint64(bound) * r.nextBits(31)) >> 31)
		}
		for {
			n := int32(r.nextBits(31))
			v := n % int32(bound)
			if n-v+int32(bound-1) >= 0 {
				return int(v)
			}
		}
	}
	for {
		product := uint64(uint32(r.nextLong())) * uint64(bound)
		if uint32(product) >= uint32(-uint32(bound))%uint32(bound) {
			return int(product >> 32)
		}
	}
}

// nextBoolean is RandomSource.nextBoolean.
func (r *noiseRandom) nextBoolean() bool {
	if r.legacy {
		return r.nextBits(1) != 0
	}
	return r.nextLong()&1 != 0
}

// nextFloat is RandomSource.nextFloat (24 bits).
func (r *noiseRandom) nextFloat() float32 {
	if r.legacy {
		return float32(r.nextBits(24)) * 0x1p-24
	}
	return float32(r.nextLong()>>40) * 0x1p-24
}

type noisePositional struct {
	lo, hi uint64
	legacy bool
}

func (r *noiseRandom) forkPositional() noisePositional {
	f := noisePositional{lo: r.nextLong(), legacy: r.legacy}
	if !r.legacy {
		f.hi = r.nextLong()
	}
	return f
}

// noiseSeedID mirrors Tectonic's NoisesMixin: tectonic:parameter/<x> noises
// are seeded as minecraft:<x> so they line up with vanilla climate noise.
func noiseSeedID(id string) string {
	if rest, ok := strings.CutPrefix(id, "tectonic:parameter/"); ok {
		return "minecraft:" + rest
	}
	return id
}

func (f noisePositional) fromHash(id string) *noiseRandom {
	if f.legacy {
		var hash int32
		for _, c := range utf16.Encode([]rune(id)) {
			hash = 31*hash + int32(c)
		}
		return newNoiseRandom(int64(f.lo)^int64(hash), true)
	}
	h := md5.Sum([]byte(id))
	return xoroshiroState(f.lo^binary.BigEndian.Uint64(h[:8]), f.hi^binary.BigEndian.Uint64(h[8:]))
}

func xoroshiroState(lo, hi uint64) *noiseRandom {
	if lo|hi == 0 {
		lo, hi = 0x9e3779b97f4a7c15, 0x6a09e667f3bcc909
	}
	return &noiseRandom{lo: lo, hi: hi}
}

func (f noisePositional) at(x, y, z int32) *noiseRandom {
	// Mth.getSeed multiplies X as an int before promoting to a long.
	n := int64(x*3129871) ^ int64(z)*116129781 ^ int64(y)
	n = (n*n*42317861 + n*11) >> 16
	if f.legacy {
		return newNoiseRandom(n^int64(f.lo), true)
	}
	return xoroshiroState(uint64(n)^f.lo, f.hi)
}

type improvedNoise struct {
	x, y, z float64
	p       [256]byte
	iy      int
	yf, ys  float64
}

func newImprovedNoise(r *noiseRandom) *improvedNoise {
	n := &improvedNoise{x: r.nextDouble() * 256, y: r.nextDouble() * 256, z: r.nextDouble() * 256}
	for i := range n.p {
		n.p[i] = byte(i)
	}
	for i := range n.p {
		j := i + r.nextInt(256-i)
		n.p[i], n.p[j] = n.p[j], n.p[i]
	}
	n.iy = int(math.Floor(n.y))
	n.yf = n.y - float64(n.iy)
	n.ys = noiseSmooth(n.yf)
	return n
}

func noiseLerp(t, a, b float64) float64 { return a + t*(b-a) }
func noiseSmooth(t float64) float64     { return t * t * t * (t*(t*6-15) + 10) }

var improvedGradients = [16][3]float64{
	{1, 1, 0}, {-1, 1, 0}, {1, -1, 0}, {-1, -1, 0},
	{1, 0, 1}, {-1, 0, 1}, {1, 0, -1}, {-1, 0, -1},
	{0, 1, 1}, {0, -1, 1}, {0, 1, -1}, {0, -1, -1},
	{1, 1, 0}, {0, -1, 1}, {-1, 1, 0}, {0, -1, -1},
}

func noiseGrad(h int, x, y, z float64) float64 {
	// A shared table avoids a stack copy per corner and unpredictable branches.
	g := &improvedGradients[h&15]
	return g[0]*x + g[1]*y + g[2]*z
}

func (n *improvedNoise) sample(x, y, z float64) float64 {
	x, z = x+n.x, z+n.z
	ix, iz := int(math.Floor(x)), int(math.Floor(z))
	x, z = x-math.Floor(x), z-math.Floor(z)
	// Five climate channels and most shift evaluations are horizontal. Their
	// zero Y input always uses the same seeded permutation row and fade.
	iy, v := n.iy, n.ys
	if y == 0 {
		y = n.yf
	} else {
		y += n.y
		iy = int(math.Floor(y))
		y -= math.Floor(y)
		v = noiseSmooth(y)
	}
	p := func(i int) int { return int(n.p[i&255]) }
	a, b := p(ix), p(ix+1)
	aa, ab, ba, bb := p(a+iy), p(a+iy+1), p(b+iy), p(b+iy+1)
	u, w := noiseSmooth(x), noiseSmooth(z)
	return noiseLerp(w,
		noiseLerp(v, noiseLerp(u, noiseGrad(p(aa+iz), x, y, z), noiseGrad(p(ba+iz), x-1, y, z)), noiseLerp(u, noiseGrad(p(ab+iz), x, y-1, z), noiseGrad(p(bb+iz), x-1, y-1, z))),
		noiseLerp(v, noiseLerp(u, noiseGrad(p(aa+iz+1), x, y, z-1), noiseGrad(p(ba+iz+1), x-1, y, z-1)), noiseLerp(u, noiseGrad(p(ab+iz+1), x, y-1, z-1), noiseGrad(p(bb+iz+1), x-1, y-1, z-1))))
}

type perlinNoise struct {
	octaves           []*improvedNoise
	amplitudes        []float64
	frequency, factor float64
}

func newPerlinNoise(r *noiseRandom, first int, amplitudes []float64, legacyNether bool) perlinNoise {
	n := perlinNoise{octaves: make([]*improvedNoise, len(amplitudes)), amplitudes: amplitudes, frequency: math.Exp2(float64(first)), factor: math.Exp2(float64(len(amplitudes)-1)) / (math.Exp2(float64(len(amplitudes))) - 1)}
	if !legacyNether {
		f := r.forkPositional()
		for i, a := range amplitudes {
			if a != 0 {
				n.octaves[i] = newImprovedNoise(f.fromHash("octave_" + strconv.Itoa(first+i)))
			}
		}
		return n
	}
	zero := newImprovedNoise(r)
	if -first < len(amplitudes) && amplitudes[-first] != 0 {
		n.octaves[-first] = zero
	}
	for i := -first - 1; i >= 0; i-- {
		if i < len(amplitudes) && amplitudes[i] != 0 {
			n.octaves[i] = newImprovedNoise(r)
		} else {
			r.skipLegacy(262)
		}
	}
	return n
}

func noiseWrap(x float64) float64 { return x - math.Floor(x/33554432+0.5)*33554432 }

func (n *perlinNoise) sample(x, y, z float64) float64 {
	result, frequency, factor := 0.0, n.frequency, n.factor
	for i, octave := range n.octaves {
		if octave != nil {
			result += n.amplitudes[i] * octave.sample(noiseWrap(x*frequency), noiseWrap(y*frequency), noiseWrap(z*frequency)) * factor
		}
		frequency *= 2
		factor /= 2
	}
	return result
}

type normalNoise struct {
	first, second perlinNoise
	factor        float64
	spec          *noiseSpec
}

func newNormalNoise(r *noiseRandom, first int, amplitudes []float64, legacyNether bool) *normalNoise {
	n := &normalNoise{first: newPerlinNoise(r, first, amplitudes, legacyNether), second: newPerlinNoise(r, first, amplitudes, legacyNether)}
	lo, hi := len(amplitudes), -1
	for i, a := range amplitudes {
		if a != 0 {
			lo, hi = min(lo, i), max(hi, i)
		}
	}
	if hi >= lo {
		n.factor = (1.0 / 6.0) / (0.1 * (1 + 1/float64(hi-lo+1)))
	}
	return n
}

func (n *normalNoise) sample(x, y, z float64) float64 {
	const scale = 1.0181268882175227
	return (n.first.sample(x, y, z) + n.second.sample(x*scale, y*scale, z*scale)) * n.factor
}
