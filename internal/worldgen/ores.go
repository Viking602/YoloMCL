package worldgen

import (
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"math/bits"
	"strings"
)

// Ore X-ray: vanilla 1.18+ decoration seeding for the UNDERGROUND_ORES step.
//
// ChunkGenerator.applyBiomeDecoration seeds a WorldgenRandom (Xoroshiro bits
// read through the java.util.Random-style BitRandomSource helpers) with
// setDecorationSeed(worldSeed, chunkMinX, chunkMinZ), then reseeds before each
// placed feature with setFeatureSeed(decorationSeed, featureIndex, step). The
// feature index is the feature's position in FeatureSorter's per-step order.
// Placement modifiers and OreFeature then consume the stream depth-first.

// xoroshiro is Xoroshiro128PlusPlus seeded through RandomSupport.
type xoroshiro struct{ lo, hi uint64 }

func mixStafford13(z uint64) uint64 {
	z = (z ^ z>>30) * 0xbf58476d1ce4e5b9
	z = (z ^ z>>27) * 0x94d049bb133111eb
	return z ^ z>>31
}

func (x *xoroshiro) setSeed(seed int64) {
	l := uint64(seed) ^ 0x6a09e667f3bcc909
	m := l + 0x9e3779b97f4a7c15
	x.lo, x.hi = mixStafford13(l), mixStafford13(m)
	if x.lo == 0 && x.hi == 0 {
		x.lo, x.hi = 0x9e3779b97f4a7c15, 0x6a09e667f3bcc909
	}
}

func (x *xoroshiro) nextLong() uint64 {
	i, j := x.lo, x.hi
	k := bits.RotateLeft64(i+j, 17) + i
	j ^= i
	x.lo = bits.RotateLeft64(i, 49) ^ j ^ j<<21
	x.hi = bits.RotateLeft64(j, 28)
	return k
}

// worldgenRandom mirrors WorldgenRandom wrapping a XoroshiroRandomSource:
// every draw goes through next(bits) = nextLong() >>> (64 - bits).
type worldgenRandom struct{ x xoroshiro }

func (r *worldgenRandom) next(n uint) int32 { return int32(r.x.nextLong() >> (64 - n)) }
func (r *worldgenRandom) nextLong() int64 {
	return int64(r.next(32))<<32 + int64(r.next(32))
}
func (r *worldgenRandom) nextInt(bound int32) int32 {
	if bound&(bound-1) == 0 {
		return int32(int64(bound) * int64(r.next(31)) >> 31)
	}
	for {
		i := r.next(31)
		j := i % bound
		if i-j+(bound-1) >= 0 {
			return j
		}
	}
}
func (r *worldgenRandom) nextFloat() float32 { return float32(r.next(24)) * 5.9604645e-8 }
func (r *worldgenRandom) nextDouble() float64 {
	return float64(int64(r.next(26))<<27+int64(r.next(27))) * float64(float32(1.110223e-16))
}
func (r *worldgenRandom) between(lo, hi int) int { return int(r.nextInt(int32(hi-lo+1))) + lo }

func (r *worldgenRandom) setDecorationSeed(seed int64, x, z int) int64 {
	r.x.setSeed(seed)
	a := r.nextLong() | 1
	b := r.nextLong() | 1
	k := int64(x)*a + int64(z)*b ^ seed
	r.x.setSeed(k)
	return k
}

func (r *worldgenRandom) setFeatureSeed(decoration int64, index, step int) {
	r.x.setSeed(decoration + int64(index) + int64(10000*step))
}

// OreStep is GenerationStep.Decoration.UNDERGROUND_ORES.
const OreStep = 6

type intProvider struct {
	Const    int
	Min, Max int
	Uniform  bool
}

func (p *intProvider) UnmarshalJSON(b []byte) error {
	if json.Unmarshal(b, &p.Const) == nil {
		return nil
	}
	var v struct {
		Type     string
		Min, Max *int
		MinI     *int `json:"min_inclusive"`
		MaxI     *int `json:"max_inclusive"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	if v.Type != "minecraft:uniform" || v.MinI == nil || v.MaxI == nil {
		return fmt.Errorf("unsupported int provider %s", b)
	}
	p.Uniform, p.Min, p.Max = true, *v.MinI, *v.MaxI
	return nil
}

func (p intProvider) sample(r *worldgenRandom) int {
	if p.Uniform {
		return r.between(p.Min, p.Max)
	}
	return p.Const
}

type anchor struct {
	Absolute    *int `json:"absolute"`
	AboveBottom *int `json:"above_bottom"`
	BelowTop    *int `json:"below_top"`
}

func (a anchor) resolve() int {
	switch {
	case a.Absolute != nil:
		return *a.Absolute
	case a.AboveBottom != nil:
		return -64 + *a.AboveBottom
	case a.BelowTop != nil:
		return 319 - *a.BelowTop
	}
	return 0
}

type heightProvider struct {
	Type    string
	Min     anchor `json:"min_inclusive"`
	Max     anchor `json:"max_inclusive"`
	Plateau int
}

func (h heightProvider) sample(r *worldgenRandom) int {
	i, j := h.Min.resolve(), h.Max.resolve()
	if i > j {
		return i
	}
	if h.Type == "minecraft:trapezoid" {
		k := j - i
		if h.Plateau >= k {
			return r.between(i, j)
		}
		l := (k - h.Plateau) / 2
		return i + r.between(0, k-l) + r.between(0, l)
	}
	return r.between(i, j)
}

type placement struct {
	Type   string
	Count  intProvider
	Height heightProvider
	Chance int
}

func (p *placement) UnmarshalJSON(b []byte) error {
	var v struct {
		Type   string
		Count  json.RawMessage
		Height heightProvider
		Chance int
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	p.Type, p.Height, p.Chance = v.Type, v.Height, v.Chance
	switch v.Type {
	case "minecraft:count":
		return json.Unmarshal(v.Count, &p.Count)
	case "minecraft:in_square", "minecraft:biome", "minecraft:rarity_filter":
	case "minecraft:height_range":
		if v.Height.Type != "minecraft:uniform" && v.Height.Type != "minecraft:trapezoid" {
			return fmt.Errorf("unsupported height %q", v.Height.Type)
		}
	default:
		return fmt.Errorf("unsupported placement %q", v.Type)
	}
	return nil
}

// OreFeature is one placed minecraft:ore feature of the ores step.
type OreFeature struct {
	ID        string
	Index     int             // position in FeatureSorter's step order
	Ore       string          // mineral, e.g. "diamond"
	Stone     string          // block placed in stone_ore_replaceables
	Deepslate string          // block placed in deepslate_ore_replaceables
	Size      int             //
	Discard   float32         // discard_chance_on_air_exposure
	Biomes    map[string]bool // biomes whose generation settings list it
	// Approximate marks features whose sorter index depends on biome order the
	// data cannot fix, or whose RNG stream depends on terrain (see Exact).
	Approximate bool
	placement   []placement
}

// Exact reports whether the feature's blob positions follow from the seed
// alone: a fractional air-exposure discard draws once per replaceable block,
// so later blobs depend on the actual terrain.
func (f *OreFeature) Exact() bool {
	return !f.Approximate && (f.Discard <= 0 || f.Discard >= 1)
}

// OreData is the generated per-version ore table.
type OreData struct {
	Version  string
	Features []OreFeature
}

//go:embed oredata/*.json
var oreFS embed.FS

type oreFile struct {
	Version  string
	Features []struct {
		ID          string
		Index       int
		Approximate bool `json:",omitempty"`
		Biomes      []string
		Placement   json.RawMessage
		Config      json.RawMessage
	}
}

// LoadOreData returns the embedded table for a release ("1.21.4") or the
// closest earlier table; exact reports whether the table is for that release.
func LoadOreData(version string) (data *OreData, exact bool, err error) {
	entries, _ := oreFS.ReadDir("oredata")
	best := ""
	for _, e := range entries {
		v := strings.TrimSuffix(e.Name(), ".json")
		if compareRelease(v, version) <= 0 && (best == "" || compareRelease(v, best) > 0) {
			best = v
		}
	}
	if best == "" {
		return nil, false, fmt.Errorf("no ore data for %s", version)
	}
	b, err := oreFS.ReadFile("oredata/" + best + ".json")
	if err != nil {
		return nil, false, err
	}
	d, err := ParseOreData(b)
	return d, best == version, err
}

func compareRelease(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < max(len(pa), len(pb)); i++ {
		var x, y int
		if i < len(pa) {
			fmt.Sscan(pa[i], &x)
		}
		if i < len(pb) {
			fmt.Sscan(pb[i], &y)
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// ParseOreData parses a generated ore table.
func ParseOreData(b []byte) (*OreData, error) {
	var f oreFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	d := &OreData{Version: f.Version}
	for _, x := range f.Features {
		of := OreFeature{ID: x.ID, Index: x.Index, Approximate: x.Approximate, Biomes: map[string]bool{}}
		for _, b := range x.Biomes {
			of.Biomes[b] = true
		}
		if err := json.Unmarshal(x.Placement, &of.placement); err != nil {
			return nil, fmt.Errorf("%s: %w", x.ID, err)
		}
		var cfg struct {
			Type   string
			Config struct {
				Size    int
				Discard float32 `json:"discard_chance_on_air_exposure"`
				Targets []struct {
					State  struct{ Name string }
					Target struct {
						Tag string
					}
				}
			}
		}
		if err := json.Unmarshal(x.Config, &cfg); err != nil {
			return nil, err
		}
		if cfg.Type != "minecraft:ore" {
			continue
		}
		of.Size, of.Discard = cfg.Config.Size, cfg.Config.Discard
		for _, t := range cfg.Config.Targets {
			switch t.Target.Tag {
			case "minecraft:stone_ore_replaceables":
				of.Stone = t.State.Name
			case "minecraft:deepslate_ore_replaceables":
				of.Deepslate = t.State.Name
			}
		}
		of.Ore = strings.TrimSuffix(strings.TrimPrefix(of.Stone, "minecraft:"), "_ore")
		if of.Stone == "" || !strings.HasSuffix(of.Stone, "_ore") || len(cfg.Config.Targets) != 2 {
			continue
		}
		d.Features = append(d.Features, of)
	}
	return d, nil
}

// OreCell is one block an ore feature would place.
type OreCell struct {
	X, Y, Z int
	Feature *OreFeature
	Block   string
}

// OreWorld answers the world questions OreFeature asks. Nil fields fall back
// to seed-only assumptions: every column passes the heightmap test, blocks
// are stone above y 0 and deepslate below, and nothing touches air.
type OreWorld struct {
	Biome func(x, y, z int) string
	// Base returns 1 for stone_ore_replaceables, 2 for deepslate ones, 0 else.
	Base func(x, y, z int) int
	// NearAir reports whether a block touches air.
	NearAir func(x, y, z int) bool
	// OceanFloor is the OCEAN_FLOOR_WG height of a column.
	OceanFloor func(x, z int) int
}

func (w *OreWorld) base(x, y, z int) int {
	if w.Base != nil {
		return w.Base(x, y, z)
	}
	if y < 0 {
		return 2
	}
	return 1
}

// ChunkOres runs the ore features decorating chunk (cx, cz) and reports the
// blocks they place (possibly in neighbouring chunks).
func ChunkOres(seed int64, cx, cz int32, feats []OreFeature, w *OreWorld, emit func(OreCell)) {
	var r worldgenRandom
	x0, z0 := int(cx)*16, int(cz)*16
	deco := r.setDecorationSeed(seed, x0, z0)
	for i := range feats {
		f := &feats[i]
		r.setFeatureSeed(deco, f.Index, OreStep)
		f.place(&r, 0, x0, -64, z0, w, emit)
	}
}

func (f *OreFeature) place(r *worldgenRandom, mod, x, y, z int, w *OreWorld, emit func(OreCell)) {
	if mod == len(f.placement) {
		f.ore(r, x, y, z, w, emit)
		return
	}
	p := &f.placement[mod]
	switch p.Type {
	case "minecraft:count":
		for n := p.Count.sample(r); n > 0; n-- {
			f.place(r, mod+1, x, y, z, w, emit)
		}
	case "minecraft:in_square":
		nx := int(r.nextInt(16)) + x
		nz := int(r.nextInt(16)) + z
		f.place(r, mod+1, nx, y, nz, w, emit)
	case "minecraft:height_range":
		f.place(r, mod+1, x, p.Height.sample(r), z, w, emit)
	case "minecraft:rarity_filter":
		if r.nextFloat() < 1/float32(p.Chance) {
			f.place(r, mod+1, x, y, z, w, emit)
		}
	case "minecraft:biome":
		if w.Biome == nil || f.Biomes[w.Biome(x, y, z)] {
			f.place(r, mod+1, x, y, z, w, emit)
		}
	}
}

func ceilF(f float32) int { return int(math.Ceil(float64(f))) }

// ore mirrors OreFeature.place and doPlace.
func (f *OreFeature) ore(r *worldgenRandom, px, py, pz int, w *OreWorld, emit func(OreCell)) {
	a := r.nextFloat() * float32(math.Pi)
	f1 := float32(f.Size) / 8
	i := ceilF((float32(f.Size)/16*2 + 1) / 2)
	s, c := math.Sin(float64(a))*float64(f1), math.Cos(float64(a))*float64(f1)
	x0, x1 := float64(px)+s, float64(px)-s
	z0, z1 := float64(pz)+c, float64(pz)-c
	y0 := float64(py + int(r.nextInt(3)) - 2)
	y1 := float64(py + int(r.nextInt(3)) - 2)
	k := px - ceilF(f1) - i
	l := py - 2 - i
	i1 := pz - ceilF(f1) - i
	j1 := 2 * (ceilF(f1) + i)
	k1 := 2 * (2 + i)
	if w.OceanFloor != nil {
		ok := false
		for xx := k; xx <= k+j1 && !ok; xx++ {
			for zz := i1; zz <= i1+j1; zz++ {
				if l <= w.OceanFloor(xx, zz) {
					ok = true
					break
				}
			}
		}
		if !ok {
			return
		}
	}
	n := f.Size
	ad := make([]float64, n*4)
	for q := 0; q < n; q++ {
		t := float32(q) / float32(n)
		d0 := x0 + float64(t)*(x1-x0)
		d2 := y0 + float64(t)*(y1-y0)
		d4 := z0 + float64(t)*(z1-z0)
		d6 := r.nextDouble() * float64(n) / 16
		d7 := (float64(mSin(float64(float32(math.Pi)*t))+1)*d6 + 1) / 2
		ad[q*4], ad[q*4+1], ad[q*4+2], ad[q*4+3] = d0, d2, d4, d7
	}
	for q := 0; q < n-1; q++ {
		if ad[q*4+3] <= 0 {
			continue
		}
		for o := q + 1; o < n; o++ {
			if ad[o*4+3] <= 0 {
				continue
			}
			dx, dy, dz := ad[q*4]-ad[o*4], ad[q*4+1]-ad[o*4+1], ad[q*4+2]-ad[o*4+2]
			dr := ad[q*4+3] - ad[o*4+3]
			if dr*dr > dx*dx+dy*dy+dz*dz {
				if dr > 0 {
					ad[o*4+3] = -1
				} else {
					ad[q*4+3] = -1
				}
			}
		}
	}
	seen := map[int]bool{}
	for q := 0; q < n; q++ {
		d9 := ad[q*4+3]
		if d9 < 0 {
			continue
		}
		d10, d11, d13 := ad[q*4], ad[q*4+1], ad[q*4+2]
		ax := max(mFloor(d10-d9), k)
		ay := max(mFloor(d11-d9), l)
		az := max(mFloor(d13-d9), i1)
		bx := max(mFloor(d10+d9), ax)
		by := max(mFloor(d11+d9), ay)
		bz := max(mFloor(d13+d9), az)
		for xx := ax; xx <= bx; xx++ {
			e := (float64(xx) + 0.5 - d10) / d9
			if e*e >= 1 {
				continue
			}
			for yy := ay; yy <= by; yy++ {
				g := (float64(yy) + 0.5 - d11) / d9
				if e*e+g*g >= 1 {
					continue
				}
				for zz := az; zz <= bz; zz++ {
					h := (float64(zz) + 0.5 - d13) / d9
					if e*e+g*g+h*h >= 1 || yy < -64 || yy >= 320 {
						continue
					}
					idx := xx - k + (yy-l)*j1 + (zz-i1)*j1*k1
					if seen[idx] {
						continue
					}
					seen[idx] = true
					block := ""
					switch w.base(xx, yy, zz) {
					case 1:
						block = f.Stone
					case 2:
						block = f.Deepslate
					default:
						continue
					}
					if f.canPlace(r, w, xx, yy, zz) {
						emit(OreCell{xx, yy, zz, f, block})
					}
				}
			}
		}
	}
}

func (f *OreFeature) canPlace(r *worldgenRandom, w *OreWorld, x, y, z int) bool {
	skip := f.Discard <= 0 || (f.Discard < 1 && r.nextFloat() >= f.Discard)
	return skip || w.NearAir == nil || !w.NearAir(x, y, z)
}
