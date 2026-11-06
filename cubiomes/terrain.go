package cubiomes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

// This file evaluates a dimension's noise_router.final_density the way
// NoiseChunk does for a freshly generated chunk: interpolated subtrees are
// sampled at cell corners and lerped (Y, then X, then Z), flat_cache columns at
// quart corners with y 0, blending is off (no old chunks), and the beardifier
// is zero because no structure pieces are known. Aquifers follow
// Aquifer.NoiseBasedAquifer (1.18-1.21.1).

type interpKey struct{ id, x, y, z int }

// packInterp packs a key into 64 bits (id 12, y 12, x 20, z 20) so the hot
// cache uses the fast integer map; keys out of range use the struct map.
func packInterp(k interpKey) (uint64, bool) {
	y := k.y + 2048
	if k.y == math.MinInt32 {
		y = 4095
	}
	if k.id < 0 || k.id >= 1<<12 || y < 0 || y >= 4095 && k.y != math.MinInt32 || k.x < -1<<19 || k.x >= 1<<19 || k.z < -1<<19 || k.z >= 1<<19 {
		return 0, false
	}
	return uint64(k.id)<<52 | uint64(y)<<40 | uint64(k.x+1<<19)<<20 | uint64(k.z+1<<19), true
}

func (p *densitySample) cached(k interpKey, f func() float64) float64 {
	if pk, ok := packInterp(k); ok {
		if v, ok := p.interp64[pk]; ok {
			return v
		}
		if p.interp64 == nil || len(p.interp64) > 1<<18 {
			p.interp64 = make(map[uint64]float64, 4096)
		}
		v := f()
		p.interp64[pk] = v
		return v
	}
	if v, ok := p.interp[k]; ok {
		return v
	}
	if p.interp == nil || len(p.interp) > 1<<18 {
		p.interp = make(map[interpKey]float64, 4096)
	}
	v := f()
	p.interp[k] = v
	return v
}

func (n *densityNode) flatCache(p *densitySample) float64 {
	qx, qz := int(math.Floor(p.x))&^3, int(math.Floor(p.z))&^3
	return p.cached(interpKey{n.id, qx, math.MinInt32, qz}, func() float64 {
		return p.evalAt(n.a, float64(qx), 0, float64(qz))
	})
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

func (n *densityNode) interpolate(p *densitySample) float64 {
	w, h := int(n.p), int(n.q)
	x, y, z := int(math.Floor(p.x)), int(math.Floor(p.y)), int(math.Floor(p.z))
	x0, y0, z0 := floorDiv(x, w)*w, floorDiv(y, h)*h, floorDiv(z, w)*w
	corner := func(dx, dy, dz int) float64 {
		cx, cy, cz := x0+dx*w, y0+dy*h, z0+dz*w
		return p.cached(interpKey{n.id, cx, cy, cz}, func() float64 {
			return p.evalAt(n.a, float64(cx), float64(cy), float64(cz))
		})
	}
	fx, fy, fz := float64(x-x0)/float64(w), float64(y-y0)/float64(h), float64(z-z0)/float64(w)
	v00 := noiseLerp(fy, corner(0, 0, 0), corner(0, 1, 0))
	v10 := noiseLerp(fy, corner(1, 0, 0), corner(1, 1, 0))
	v01 := noiseLerp(fy, corner(0, 0, 1), corner(0, 1, 1))
	v11 := noiseLerp(fy, corner(1, 0, 1), corner(1, 1, 1))
	return noiseLerp(fz, noiseLerp(fx, v00, v10), noiseLerp(fx, v01, v11))
}

// weirdRarity is NoiseRouterData.QuantizedSpaghettiRarity: type_1 is
// getSpaghettiRarity3D, type_2 getSpaghettiRarity2D.
func weirdRarity(v float64, twoD bool) float64 {
	if twoD {
		switch {
		case v < -0.75:
			return 0.5
		case v < -0.5:
			return 0.75
		case v < 0.5:
			return 1
		case v < 0.75:
			return 2
		}
		return 3
	}
	switch {
	case v < -0.5:
		return 0.75
	case v < 0:
		return 1
	case v < 0.5:
		return 1.5
	}
	return 2
}

// sampleSmear is ImprovedNoise.noise(x, y, z, yScale, yMax).
func (n *improvedNoise) sampleSmear(x, y, z, yScale, yMax float64) float64 {
	x, y, z = x+n.x, y+n.y, z+n.z
	fx, fy, fz := math.Floor(x), math.Floor(y), math.Floor(z)
	ix, iy, iz := int(fx), int(fy), int(fz)
	x, y, z = x-fx, y-fy, z-fz
	shift := 0.0
	if yScale != 0 {
		t := y
		if yMax >= 0 && yMax < y {
			t = yMax
		}
		shift = math.Floor(t/yScale+float64(float32(1e-7))) * yScale
	}
	return n.lerpAt(ix, iy, iz, x, y-shift, z, y)
}

func (n *improvedNoise) lerpAt(ix, iy, iz int, x, y, z, yFade float64) float64 {
	p := func(i int) int { return int(n.p[i&255]) }
	a, b := p(ix), p(ix+1)
	aa, ab, ba, bb := p(a+iy), p(a+iy+1), p(b+iy), p(b+iy+1)
	u, v, w := noiseSmooth(x), noiseSmooth(yFade), noiseSmooth(z)
	return noiseLerp(w,
		noiseLerp(v, noiseLerp(u, noiseGrad(p(aa+iz), x, y, z), noiseGrad(p(ba+iz), x-1, y, z)), noiseLerp(u, noiseGrad(p(ab+iz), x, y-1, z), noiseGrad(p(bb+iz), x-1, y-1, z))),
		noiseLerp(v, noiseLerp(u, noiseGrad(p(aa+iz+1), x, y, z-1), noiseGrad(p(ba+iz+1), x-1, y, z-1)), noiseLerp(u, noiseGrad(p(ab+iz+1), x, y-1, z-1), noiseGrad(p(bb+iz+1), x-1, y-1, z-1))))
}

// blendedNoise is BlendedNoise (old_blended_noise), seeded as RandomState
// does: LegacyRandomSource(seed) for legacy settings, else the positional
// "minecraft:terrain" random.
type blendedNoise struct {
	minLimit, maxLimit, main                        perlinNoise
	xzMul, yMul, xzFactor, yFactor, smearMultiplier float64
}

func (c *densityCompiler) compileBlended(fields map[string]json.RawMessage) *blendedNoise {
	num := func(k string) float64 { v, _ := densityNumber(fields[k]); return v }
	var r *noiseRandom
	if c.legacy {
		r = newNoiseRandom(c.seed, true)
	} else {
		r = c.factory.fromHash("minecraft:terrain")
	}
	ones := func(n int) []float64 {
		a := make([]float64, n)
		for i := range a {
			a[i] = 1
		}
		return a
	}
	b := &blendedNoise{}
	b.minLimit = newPerlinNoise(r, -15, ones(16), true)
	b.maxLimit = newPerlinNoise(r, -15, ones(16), true)
	b.main = newPerlinNoise(r, -7, ones(8), true)
	b.xzMul, b.yMul = 684.412*num("xz_scale"), 684.412*num("y_scale")
	b.xzFactor, b.yFactor, b.smearMultiplier = num("xz_factor"), num("y_factor"), num("smear_scale_multiplier")
	return b
}

func (n *perlinNoise) octave(i int) *improvedNoise { return n.octaves[len(n.octaves)-1-i] }

func (b *blendedNoise) compute(x, y, z float64) float64 {
	d0, d1, d2 := x*b.xzMul, y*b.yMul, z*b.xzMul
	d3, d4, d5 := d0/b.xzFactor, d1/b.yFactor, d2/b.xzFactor
	d6 := b.yMul * b.smearMultiplier
	d7 := d6 / b.yFactor
	var lo, hi, mainV float64
	f := 1.0
	for i := 0; i < 8; i++ {
		if o := b.main.octave(i); o != nil {
			mainV += o.sampleSmear(noiseWrap(d3*f), noiseWrap(d4*f), noiseWrap(d5*f), d7*f, d4*f) / f
		}
		f /= 2
	}
	t := (mainV/10 + 1) / 2
	f = 1
	for i := 0; i < 16; i++ {
		x, y, z, s := noiseWrap(d0*f), noiseWrap(d1*f), noiseWrap(d2*f), d6*f
		if t < 1 {
			if o := b.minLimit.octave(i); o != nil {
				lo += o.sampleSmear(x, y, z, s, d1*f) / f
			}
		}
		if t > 0 {
			if o := b.maxLimit.octave(i); o != nil {
				hi += o.sampleSmear(x, y, z, s, d1*f) / f
			}
		}
		f /= 2
	}
	// Mth.clampedLerp
	var v float64
	switch {
	case t < 0:
		v = lo / 512
	case t > 1:
		v = hi / 512
	default:
		v = noiseLerp(t, lo/512, hi/512)
	}
	return v / 128
}

// Terrain is a compiled dimension terrain program.
type Terrain struct {
	MinY, Height, SeaLevel int
	Aquifers               bool
	DefaultFluid           Substance
	final, prelim          *densityNode
	prelimIsHeight         bool
	barrier, flood, spread *densityNode
	lava, erosion, depth   *densityNode
	nodes                  int
	aquiferRandom          noisePositional
	hasEnd                 bool
	cellH                  int
}

// Substance is what noise generation puts in a block.
type Substance uint8

const (
	Air Substance = iota
	Solid
	Water
	Lava
)

// CompileTerrain compiles noise_router.final_density and the aquifer router
// of def.Settings. Supports pre-26.3 JSON graphs; 26.3's float terrain pipeline
// remains unsupported.
func CompileTerrain(ctx context.Context, docs map[string]Document, def Definition) (*Terrain, error) {
	if latestFloatRelease(def.Version) {
		return nil, &CompileError{"terrain_unsupported", "terrain for " + def.Version}
	}
	if _, ok := legacyRelease(def.Version); ok {
		return nil, &CompileError{"terrain_unsupported", "terrain for " + def.Version}
	}
	settings := def.Settings
	var ref string
	if json.Unmarshal(settings, &ref) == nil && ref != "" {
		settings = docs["noise_settings/"+seedMapID(ref)].Data
	}
	var data struct {
		Legacy   bool                       `json:"legacy_random_source"`
		Sea      int                        `json:"sea_level"`
		Aquifers bool                       `json:"aquifers_enabled"`
		Fluid    struct{ Name string }      `json:"default_fluid"`
		Router   map[string]json.RawMessage `json:"noise_router"`
		Noise    struct {
			MinY   int `json:"min_y"`
			Height int `json:"height"`
			SizeH  int `json:"size_horizontal"`
			SizeV  int `json:"size_vertical"`
		} `json:"noise"`
	}
	if json.Unmarshal(settings, &data) != nil || data.Router == nil || data.Noise.Height <= 0 || data.Noise.SizeH < 1 || data.Noise.SizeV < 1 {
		return nil, errors.New("invalid or missing noise settings")
	}
	c := newDensityCompiler(ctx, docs, def.Seed, data.Legacy)
	c.version, c.terrain = def.Version, true
	c.cellW, c.cellH = data.Noise.SizeH*4, data.Noise.SizeV*4
	t := &Terrain{MinY: data.Noise.MinY, Height: data.Noise.Height, SeaLevel: data.Sea, Aquifers: data.Aquifers, cellH: c.cellH}
	switch seedMapID(data.Fluid.Name) {
	case "minecraft:lava":
		t.DefaultFluid = Lava
	case "minecraft:water":
		t.DefaultFluid = Water
	}
	var err error
	get := func(key string) *densityNode {
		if err != nil {
			return nil
		}
		raw, ok := data.Router[key]
		if !ok {
			raw = json.RawMessage("0")
		}
		n, e := c.compile(raw, 0)
		if e != nil {
			err = fmt.Errorf("%s: %w", key, e)
		}
		return n
	}
	t.final = get("final_density")
	if _, ok := data.Router["preliminary_surface_level"]; ok {
		t.prelim, t.prelimIsHeight = get("preliminary_surface_level"), true
	} else if _, ok := data.Router["initial_density_without_jaggedness"]; ok {
		t.prelim = get("initial_density_without_jaggedness")
	} else if t.Aquifers && err == nil {
		err = errors.New("missing preliminary surface router")
	}
	if t.Aquifers {
		t.barrier, t.flood, t.spread = get("barrier"), get("fluid_level_floodedness"), get("fluid_level_spread")
		t.lava, t.erosion, t.depth = get("lava"), get("erosion"), get("depth")
		t.aquiferRandom = c.factory.fromHash("minecraft:aquifer").forkPositional()
	}
	if err != nil {
		return nil, err
	}
	t.nodes, t.hasEnd = c.nodes, c.end != nil
	return t, nil
}

// TerrainSampler is per-worker scratch.
type TerrainSampler struct {
	t        *Terrain
	s        densitySample
	prelim   map[[2]int]int
	aqLoc    map[[3]int][3]int
	aqStatus map[[3]int]fluidStatus
}

func (t *Terrain) NewSampler() *TerrainSampler {
	s := &TerrainSampler{t: t, s: densitySample{seen: make([]uint32, t.nodes), values: make([]float64, t.nodes)}}
	if t.hasEnd {
		s.s.endCache = &endCache{}
	}
	return s
}

func (s *TerrainSampler) eval(n *densityNode, x, y, z int) float64 {
	s.s.at(x, y, z)
	return n.eval(&s.s)
}

// Density is final_density at a block.
func (s *TerrainSampler) Density(x, y, z int) float64 { return s.eval(s.t.final, x, y, z) }

// Block is the noise-stage substance at a block (before surface rules,
// carvers and features). Ore veins are not distinguished from stone.
func (s *TerrainSampler) Block(x, y, z int) Substance {
	if y < s.t.MinY || y >= s.t.MinY+s.t.Height {
		return Air
	}
	d := s.Density(x, y, z)
	if d > 0 {
		return Solid
	}
	if !s.t.Aquifers {
		if y < s.t.SeaLevel {
			return s.t.DefaultFluid
		}
		return Air
	}
	return s.aquifer(x, y, z, d)
}

// SolidTop is the highest Y with final_density > 0, MinY-1 when none. It
// skips the aquifer, which only decides fluid or air below that.
func (s *TerrainSampler) SolidTop(x, z int) int {
	for y := s.t.MinY + s.t.Height - 1; y >= s.t.MinY; y-- {
		if s.Density(x, y, z) > 0 {
			return y
		}
	}
	return s.t.MinY - 1
}

// Column fills out[y-MinY] for the whole build height and returns the top
// solid Y and the top non-air Y (WORLD_SURFACE_WG - 1), MinY-1 when none.
func (s *TerrainSampler) Column(x, z int, out []Substance) (solidTop, top int) {
	solidTop, top = s.t.MinY-1, s.t.MinY-1
	for y := s.t.MinY + s.t.Height - 1; y >= s.t.MinY; y-- {
		b := s.Block(x, y, z)
		if out != nil {
			out[y-s.t.MinY] = b
		}
		if b != Air && top < s.t.MinY {
			top = y
		}
		if b == Solid && solidTop < s.t.MinY {
			solidTop = y
			if out == nil {
				return
			}
		}
	}
	return
}

type fluidStatus struct {
	level int
	fluid Substance
}

func (f fluidStatus) at(y int) Substance {
	if y < f.level {
		return f.fluid
	}
	return Air
}

const wayBelowMinY = -2032 * 16

func (s *TerrainSampler) global(y int) fluidStatus {
	if y < min(-54, s.t.SeaLevel) {
		return fluidStatus{-54, Lava}
	}
	return fluidStatus{s.t.SeaLevel, s.t.DefaultFluid}
}

func (s *TerrainSampler) preliminary(x, z int) int {
	k := [2]int{x &^ 3, z &^ 3}
	if v, ok := s.prelim[k]; ok {
		return v
	}
	if s.prelim == nil || len(s.prelim) > 1<<16 {
		s.prelim = map[[2]int]int{}
	}
	v := math.MaxInt32
	if s.t.prelim != nil {
		if s.t.prelimIsHeight {
			v = int(math.Floor(s.eval(s.t.prelim, k[0], 0, k[1])))
		} else {
			for y := s.t.MinY + s.t.Height; y >= s.t.MinY; y -= s.t.cellH {
				if s.eval(s.t.prelim, k[0], y, k[1]) > 0.390625 {
					v = y
					break
				}
			}
		}
	}
	s.prelim[k] = v
	return v
}

var aquiferOffsets = [13][2]int{{0, 0}, {-2, -1}, {-1, -1}, {0, -1}, {1, -1}, {-3, 0}, {-2, 0}, {-1, 0}, {1, 0}, {-2, 1}, {-1, 1}, {0, 1}, {1, 1}}

func aquiferSimilarity(a, b int) float64 { return 1 - float64(absInt(b-a))/25 }

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (s *TerrainSampler) aquifer(x, y, z int, density float64) Substance {
	if g := s.global(y); g.at(y) == Lava {
		return Lava
	}
	gx, gy, gz := floorDiv(x-5, 16), floorDiv(y+1, 12), floorDiv(z-5, 16)
	d1, d2, d3 := math.MaxInt32, math.MaxInt32, math.MaxInt32
	var p1, p2, p3 [3]int
	for i := 0; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			for k := 0; k <= 1; k++ {
				c := [3]int{gx + i, gy + j, gz + k}
				loc, ok := s.aqLoc[c]
				if !ok {
					r := s.t.aquiferRandom.at(int32(c[0]), int32(c[1]), int32(c[2]))
					loc = [3]int{c[0]*16 + r.nextInt(10), c[1]*12 + r.nextInt(9), c[2]*16 + r.nextInt(10)}
					if s.aqLoc == nil || len(s.aqLoc) > 1<<16 {
						s.aqLoc = map[[3]int][3]int{}
					}
					s.aqLoc[c] = loc
				}
				dx, dy, dz := loc[0]-x, loc[1]-y, loc[2]-z
				d := dx*dx + dy*dy + dz*dz
				switch {
				case d1 >= d:
					d3, p3, d2, p2, d1, p1 = d2, p2, d1, p1, d, loc
				case d2 >= d:
					d3, p3, d2, p2 = d2, p2, d, loc
				case d3 >= d:
					d3, p3 = d, loc
				}
			}
		}
	}
	f1 := s.aquiferStatus(p1)
	sim := aquiferSimilarity(d1, d2)
	b := f1.at(y)
	if sim <= 0 {
		return b
	}
	if b == Water && s.global(y-1).at(y-1) == Lava {
		return b
	}
	barrier := math.NaN()
	f2 := s.aquiferStatus(p2)
	if density+sim*s.pressure(x, y, z, &barrier, f1, f2) > 0 {
		return Solid
	}
	f3 := s.aquiferStatus(p3)
	if s13 := aquiferSimilarity(d1, d3); s13 > 0 && density+sim*s13*s.pressure(x, y, z, &barrier, f1, f3) > 0 {
		return Solid
	}
	if s23 := aquiferSimilarity(d2, d3); s23 > 0 && density+sim*s23*s.pressure(x, y, z, &barrier, f2, f3) > 0 {
		return Solid
	}
	return b
}

func (s *TerrainSampler) pressure(x, y, z int, barrier *float64, a, b fluidStatus) float64 {
	ba, bb := a.at(y), b.at(y)
	if ba == Lava && bb == Water || ba == Water && bb == Lava {
		return 2
	}
	diff := absInt(a.level - b.level)
	if diff == 0 {
		return 0
	}
	mid := 0.5 * float64(a.level+b.level)
	off := float64(y) + 0.5 - mid
	half := float64(diff) / 2
	d9 := half - math.Abs(off)
	var d10 float64
	if off > 0 {
		if v := 0 + d9; v > 0 {
			d10 = v / 1.5
		} else {
			d10 = v / 2.5
		}
	} else {
		if v := 3 + d9; v > 0 {
			d10 = v / 3
		} else {
			d10 = v / 10
		}
	}
	d12 := 0.0
	if d10 >= -2 && d10 <= 2 {
		if math.IsNaN(*barrier) {
			*barrier = s.eval(s.t.barrier, x, y, z)
		}
		d12 = *barrier
	}
	return 2 * (d12 + d10)
}

func (s *TerrainSampler) aquiferStatus(p [3]int) fluidStatus {
	k := [3]int{floorDiv(p[0], 16), floorDiv(p[1], 12), floorDiv(p[2], 16)}
	if v, ok := s.aqStatus[k]; ok {
		return v
	}
	v := s.computeFluid(p[0], p[1], p[2])
	if s.aqStatus == nil || len(s.aqStatus) > 1<<16 {
		s.aqStatus = map[[3]int]fluidStatus{}
	}
	s.aqStatus[k] = v
	return v
}

func (s *TerrainSampler) computeFluid(x, y, z int) fluidStatus {
	global := s.global(y)
	minSurface := math.MaxInt32
	top, bottom := y+12, y-12
	flooded := false
	for _, o := range aquiferOffsets {
		lx, lz := x+o[0]*16, z+o[1]*16
		surface := s.preliminary(lx, lz)
		k1 := surface + 8
		centre := o[0] == 0 && o[1] == 0
		if centre && bottom > k1 {
			return global
		}
		above := top > k1
		if above || centre {
			fs := s.global(k1)
			if fs.at(k1) != Air {
				if centre {
					flooded = true
				}
				if above {
					return fs
				}
			}
		}
		minSurface = min(minSurface, surface)
	}
	level := s.surfaceLevel(x, y, z, global, minSurface, flooded)
	return fluidStatus{level, s.fluidType(x, y, z, global, level)}
}

func (s *TerrainSampler) surfaceLevel(x, y, z int, global fluidStatus, minSurface int, flooded bool) int {
	var d0, d1 float64
	if s.eval(s.t.erosion, x, y, z) < float64(float32(-0.225)) && s.eval(s.t.depth, x, y, z) > float64(float32(0.9)) {
		d0, d1 = -1, -1
	} else {
		i := minSurface + 8 - y
		d2 := 0.0
		if flooded {
			d2 = clampedMap(float64(i), 0, 64, 1, 0)
		}
		d3 := max(-1, min(1, s.eval(s.t.flood, x, y, z)))
		d4 := mapRange(d2, 1, 0, -0.3, 0.8)
		d5 := mapRange(d2, 1, 0, -0.8, 0.4)
		d0, d1 = d3-d5, d3-d4
	}
	switch {
	case d1 > 0:
		return global.level
	case d0 > 0:
		return s.randomizedLevel(x, y, z, minSurface)
	}
	return wayBelowMinY
}

func mapRange(v, a, b, c, d float64) float64 { return noiseLerp((v-a)/(b-a), c, d) }

func clampedMap(v, a, b, c, d float64) float64 {
	t := (v - a) / (b - a)
	if t < 0 {
		return c
	}
	if t > 1 {
		return d
	}
	return noiseLerp(t, c, d)
}

func (s *TerrainSampler) randomizedLevel(x, y, z, minSurface int) int {
	k, l, m := floorDiv(x, 16), floorDiv(y, 40), floorDiv(z, 16)
	base := l*40 + 20
	d0 := s.eval(s.t.spread, k, l, m) * 10
	q := int(math.Floor(d0/3)) * 3
	return min(minSurface, base+q)
}

func (s *TerrainSampler) fluidType(x, y, z int, global fluidStatus, level int) Substance {
	f := global.fluid
	if level <= -10 && level != wayBelowMinY && global.fluid != Lava {
		d0 := s.eval(s.t.lava, floorDiv(x, 64), floorDiv(y, 40), floorDiv(z, 64))
		if math.Abs(d0) > 0.3 {
			f = Lava
		}
	}
	return f
}
