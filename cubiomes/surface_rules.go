package cubiomes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
)

// Pure-Go port of Minecraft 1.20.1 SurfaceRules (vanilla codec JSON) with
// TerraBlender's namespaced dispatch: a biome's namespace selects its rule and
// a null result falls back to the base (vanilla) rule, which the caller owns.

// SurfaceRuleSource is one namespace's rule in vanilla surface_rule JSON.
type SurfaceRuleSource struct {
	Namespace string
	Rule      json.RawMessage
}

type surfaceCtx struct {
	x, y, z                int
	stoneAbove, stoneBelow int
	water                  int
	depth                  int
	secondary              float64
	secondaryOK            bool
	minSurface             int
	minSurfaceOK           bool
	biomeY                 int
	biomeID                string
	in                     *SurfaceInput
	s                      *SurfaceRules
}

type surfaceCond func(*surfaceCtx) bool
type surfaceRule func(*surfaceCtx) string

// SurfaceRules is a compiled set of namespaced rules for one dimension.
type SurfaceRules struct {
	MinY, Height       int
	sources            map[string]surfaceRule
	surface, secondary *normalNoise
	random             noisePositional
	blocks             map[string]bool
}

// SurfaceInput is one noise-stage column plus its context.
type SurfaceInput struct {
	X, Z int
	Sub  []Substance // index y-MinY
	// Biome is the biome id at a block; its namespace selects the rule.
	Biome func(y int) string
	// Top is WORLD_SURFACE_WG-1 (top non-air Y) of a column, for steep.
	Top func(x, z int) int
	// Prelim is NoiseChunk.preliminarySurfaceLevel at a block column.
	Prelim func(x, z int) int
}

const surfaceMinInt = math.MinInt32

// Preliminary is NoiseChunk.preliminarySurfaceLevel (4x4 quantised).
func (s *TerrainSampler) Preliminary(x, z int) int { return s.preliminary(x, z) }

// SurfaceBaseNamespace names a source whose rule is the noise settings'
// surface_rule; it decides biomes no namespaced rule covers, as TerraBlender
// falls back to the dimension's base rule.
const SurfaceBaseNamespace = "*base*"

// CompileSurfaceRules compiles namespaced rules with the dimension's seed.
func CompileSurfaceRules(ctx context.Context, docs map[string]Document, def Definition, t *Terrain, sources []SurfaceRuleSource) (*SurfaceRules, error) {
	if t == nil || len(sources) == 0 {
		return nil, errors.New("no surface rules")
	}
	settings := def.Settings
	var ref string
	if json.Unmarshal(settings, &ref) == nil && ref != "" {
		settings = docs["noise_settings/"+seedMapID(ref)].Data
	}
	var data struct {
		Legacy bool            `json:"legacy_random_source"`
		Rule   json.RawMessage `json:"surface_rule"`
	}
	if json.Unmarshal(settings, &data) != nil {
		return nil, errors.New("invalid noise settings")
	}
	c := newDensityCompiler(ctx, docs, def.Seed, data.Legacy)
	c.version = def.Version
	r := &SurfaceRules{MinY: t.MinY, Height: t.Height, sources: map[string]surfaceRule{}, random: c.factory, blocks: map[string]bool{}}
	var err error
	if r.surface, err = c.compileNoise(json.RawMessage(`"minecraft:surface"`)); err != nil {
		return nil, err
	}
	if r.secondary, err = c.compileNoise(json.RawMessage(`"minecraft:surface_secondary"`)); err != nil {
		return nil, err
	}
	sc := &surfaceCompiler{c: c, r: r, terrain: t}
	for _, src := range sources {
		if src.Namespace == SurfaceBaseNamespace {
			if len(data.Rule) == 0 {
				continue
			}
			src.Rule = data.Rule
		}
		rule, err := sc.rule(src.Rule, 0)
		if err != nil {
			return nil, fmt.Errorf("surface rule %s: %w", src.Namespace, err)
		}
		r.sources[src.Namespace] = rule
	}
	return r, nil
}

// Namespaces lists the compiled namespaces, sorted.
func (r *SurfaceRules) Namespaces() []string {
	var out []string
	for k := range r.sources {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Blocks lists every block id a rule can place, sorted.
func (r *SurfaceRules) Blocks() []string {
	var out []string
	for k := range r.blocks {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

type surfaceCompiler struct {
	c       *densityCompiler
	r       *SurfaceRules
	terrain *Terrain
}

func (sc *surfaceCompiler) anchor(raw json.RawMessage) (func() int, error) {
	var a map[string]int
	if json.Unmarshal(raw, &a) != nil || len(a) != 1 {
		return nil, errors.New("invalid vertical anchor")
	}
	minY, top := sc.r.MinY, sc.r.MinY+sc.r.Height-1
	for k, v := range a {
		switch k {
		case "absolute":
			return func() int { return v }, nil
		case "above_bottom":
			return func() int { return minY + v }, nil
		case "below_top":
			return func() int { return top - v }, nil
		}
	}
	return nil, errors.New("invalid vertical anchor")
}

func (sc *surfaceCompiler) cond(raw json.RawMessage, depth int) (surfaceCond, error) {
	if depth > 64 {
		return nil, errors.New("surface condition too deep")
	}
	if err := sc.c.budget(0); err != nil {
		return nil, err
	}
	var d struct {
		Type         string          `json:"type"`
		Biomes       json.RawMessage `json:"biome_is"`
		Noise        string          `json:"noise"`
		Is3D         bool            `json:"is_3d"`
		Min          *float64        `json:"min_threshold"`
		Max          *float64        `json:"max_threshold"`
		RandomName   string          `json:"random_name"`
		TrueAt       json.RawMessage `json:"true_at_and_below"`
		FalseAt      json.RawMessage `json:"false_at_and_above"`
		Anchor       json.RawMessage `json:"anchor"`
		Mult         int             `json:"surface_depth_multiplier"`
		AddStone     bool            `json:"add_stone_depth"`
		Offset       int             `json:"offset"`
		AddSurface   bool            `json:"add_surface_depth"`
		SecondaryRng int             `json:"secondary_depth_range"`
		SurfaceType  string          `json:"surface_type"`
		Invert       json.RawMessage `json:"invert"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	switch seedMapID(d.Type) {
	case "minecraft:biome":
		set, err := sc.biomes(d.Biomes)
		if err != nil {
			return nil, err
		}
		return func(c *surfaceCtx) bool { return set[c.biome()] }, nil
	case "minecraft:noise_threshold":
		if d.Min == nil || d.Max == nil {
			return nil, errors.New("noise_threshold bounds")
		}
		n, err := sc.c.compileNoise(json.RawMessage(strconv.Quote(d.Noise)))
		if err != nil {
			return nil, err
		}
		lo, hi := *d.Min, *d.Max
		return func(c *surfaceCtx) bool {
			y := 0
			if d.Is3D {
				y = c.y
			}
			v := n.sample(float64(c.x), float64(y), float64(c.z))
			return v >= lo && v <= hi
		}, nil
	case "minecraft:vertical_gradient":
		lo, err := sc.anchor(d.TrueAt)
		if err != nil {
			return nil, err
		}
		hi, err := sc.anchor(d.FalseAt)
		if err != nil {
			return nil, err
		}
		// RandomState.getOrCreateRandomFactory: random.fromHashOf(id).forkPositional().
		f := sc.c.factory.fromHash(seedMapID(d.RandomName)).forkPositional()
		return func(c *surfaceCtx) bool {
			i, j := lo(), hi()
			if c.y <= i {
				return true
			}
			if c.y >= j {
				return false
			}
			// Mth.map(y, i, j, 1, 0)
			p := 1 + (float64(c.y)-float64(i))/(float64(j)-float64(i))*(0-1)
			return float64(f.at(int32(c.x), int32(c.y), int32(c.z)).nextFloat()) < p
		}, nil
	case "minecraft:y_above":
		a, err := sc.anchor(d.Anchor)
		if err != nil {
			return nil, err
		}
		return func(c *surfaceCtx) bool {
			y := c.y
			if d.AddStone {
				y += c.stoneAbove
			}
			return y >= a()+c.depth*d.Mult
		}, nil
	case "minecraft:water":
		return func(c *surfaceCtx) bool {
			if c.water == surfaceMinInt {
				return true
			}
			y := c.y
			if d.AddStone {
				y += c.stoneAbove
			}
			return y >= c.water+d.Offset+c.depth*d.Mult
		}, nil
	case "minecraft:stone_depth":
		ceiling := d.SurfaceType == "ceiling"
		if d.SurfaceType != "floor" && !ceiling {
			return nil, errors.New("stone_depth surface_type")
		}
		return func(c *surfaceCtx) bool {
			i := c.stoneAbove
			if ceiling {
				i = c.stoneBelow
			}
			j := 0
			if d.AddSurface {
				j = c.depth
			}
			k := 0
			if d.SecondaryRng != 0 {
				k = int((c.secondaryValue() + 1) / 2 * float64(d.SecondaryRng))
			}
			return i <= 1+d.Offset+j+k
		}, nil
	case "minecraft:not":
		inner, err := sc.cond(d.Invert, depth+1)
		if err != nil {
			return nil, err
		}
		return func(c *surfaceCtx) bool { return !inner(c) }, nil
	case "minecraft:hole":
		return func(c *surfaceCtx) bool { return c.depth <= 0 }, nil
	case "minecraft:above_preliminary_surface":
		if sc.terrain == nil || sc.terrain.prelim == nil {
			return nil, errors.New("missing preliminary surface router")
		}
		return func(c *surfaceCtx) bool { return c.y >= c.minSurfaceLevel() }, nil
	case "minecraft:steep":
		return func(c *surfaceCtx) bool { return c.steep() }, nil
	case "minecraft:temperature":
		climate := biomeClimates(sc.c.docs)
		return func(c *surfaceCtx) bool { return climate.coldEnoughToSnow(c.biome(), c.x, c.y, c.z) }, nil
	}
	return nil, &CompileError{"surface_rule_unsupported", "condition " + d.Type}
}

func (sc *surfaceCompiler) rule(raw json.RawMessage, depth int) (surfaceRule, error) {
	if depth > 64 {
		return nil, errors.New("surface rule too deep")
	}
	if err := sc.c.budget(0); err != nil {
		return nil, err
	}
	var d struct {
		Type     string                `json:"type"`
		Sequence []json.RawMessage     `json:"sequence"`
		If       json.RawMessage       `json:"if_true"`
		Then     json.RawMessage       `json:"then_run"`
		State    struct{ Name string } `json:"result_state"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	switch seedMapID(d.Type) {
	case "minecraft:block":
		if d.State.Name == "" {
			return nil, errors.New("block rule without state")
		}
		id := seedMapID(d.State.Name)
		sc.r.blocks[id] = true
		return func(*surfaceCtx) string { return id }, nil
	case "minecraft:sequence":
		var rules []surfaceRule
		for _, r := range d.Sequence {
			f, err := sc.rule(r, depth+1)
			if err != nil {
				return nil, err
			}
			rules = append(rules, f)
		}
		return func(c *surfaceCtx) string {
			for _, f := range rules {
				if s := f(c); s != "" {
					return s
				}
			}
			return ""
		}, nil
	case "minecraft:condition":
		cond, err := sc.cond(d.If, depth+1)
		if err != nil {
			return nil, err
		}
		then, err := sc.rule(d.Then, depth+1)
		if err != nil {
			return nil, err
		}
		return func(c *surfaceCtx) string {
			if cond(c) {
				return then(c)
			}
			return ""
		}, nil
	case "minecraft:bandlands":
		bands, err := sc.clayBands()
		if err != nil {
			return nil, err
		}
		return bands, nil
	}
	return nil, &CompileError{"surface_rule_unsupported", "rule " + d.Type}
}

func (c *surfaceCtx) biome() string {
	if c.in.Biome == nil {
		return ""
	}
	if c.biomeY != c.y || c.biomeID == "" {
		c.biomeY, c.biomeID = c.y, seedMapID(c.in.Biome(c.y))
	}
	return c.biomeID
}

func (c *surfaceCtx) secondaryValue() float64 {
	if !c.secondaryOK {
		c.secondary, c.secondaryOK = c.s.secondary.sample(float64(c.x), 0, float64(c.z)), true
	}
	return c.secondary
}

// SurfaceSystem.Context.getMinSurfaceLevel.
func (c *surfaceCtx) minSurfaceLevel() int {
	if !c.minSurfaceOK {
		cx, cz := c.x>>4<<4, c.z>>4<<4
		p := c.in.Prelim
		a00, a10, a01, a11 := float64(p(cx, cz)), float64(p(cx+16, cz)), float64(p(cx, cz+16)), float64(p(cx+16, cz+16))
		dx, dz := float64(float32(c.x&15)/16), float64(float32(c.z&15)/16)
		v := surfaceLerp(dz, surfaceLerp(dx, a00, a10), surfaceLerp(dx, a01, a11))
		c.minSurface, c.minSurfaceOK = int(math.Floor(v))+c.depth-8, true
	}
	return c.minSurface
}

func surfaceLerp(t, a, b float64) float64 { return a + t*(b-a) }

// SurfaceRules.Steep: WORLD_SURFACE_WG compared inside the chunk.
func (c *surfaceCtx) steep() bool {
	cx, cz := c.x&^15, c.z&^15
	i, j := c.x&15, c.z&15
	top := c.in.Top
	k, l := max(j-1, 0), min(j+1, 15)
	if top(cx+i, cz+l) >= top(cx+i, cz+k)+4 {
		return true
	}
	k1, l1 := max(i-1, 0), min(i+1, 15)
	return top(cx+k1, cz+j) >= top(cx+l1, cz+j)+4
}

// SurfaceDepth is SurfaceSystem.getSurfaceDepth.
func (r *SurfaceRules) SurfaceDepth(x, z int) int {
	return int(r.surface.sample(float64(x), 0, float64(z))*2.75 + 3 + r.random.at(int32(x), 0, int32(z)).nextDouble()*0.25)
}

// Column applies namespaced rules to a noise-stage column (SurfaceSystem
// buildSurface's per-column loop). out[y-MinY] is the rule's block id, or ""
// where the base (vanilla) rule applies or the block is not default stone.
// top is WORLD_SURFACE_WG-1 of this column.
func (r *SurfaceRules) Column(in *SurfaceInput, top int, out []string) {
	for i := range out {
		out[i] = ""
	}
	if r == nil || in == nil || len(in.Sub) < r.Height || len(out) < r.Height {
		return
	}
	minY := r.MinY
	c := &surfaceCtx{x: in.X, z: in.Z, depth: r.SurfaceDepth(in.X, in.Z), in: in, s: r, biomeY: math.MinInt32}
	stone := func(y int) bool { return y >= minY && in.Sub[y-minY] == Solid }
	water, stoneAbove, ceilingY := surfaceMinInt, 0, math.MaxInt32
	if top >= minY+r.Height {
		top = minY + r.Height - 1
	}
	for y := top; y >= minY; y-- {
		switch in.Sub[y-minY] {
		case Air:
			stoneAbove, water = 0, surfaceMinInt
			continue
		case Water, Lava:
			if water == surfaceMinInt {
				water = y + 1
			}
			continue
		}
		if ceilingY >= y {
			ceilingY = minY - 1 // DimensionType.WAY_BELOW_MIN_Y is never reached: minY-1 is air
			for i := y - 1; i >= minY-1; i-- {
				if !stone(i) {
					ceilingY = i + 1
					break
				}
			}
		}
		stoneAbove++
		c.y, c.stoneAbove, c.stoneBelow, c.water = y, stoneAbove, y-ceilingY+1, water
		b := c.biome()
		ns := "minecraft"
		for i := 0; i < len(b); i++ {
			if b[i] == ':' {
				ns = b[:i]
				break
			}
		}
		rule := r.sources[ns]
		if rule == nil {
			rule = r.sources[SurfaceBaseNamespace]
		}
		if rule != nil {
			out[y-minY] = rule(c)
		}
	}
}

// clayBands is SurfaceSystem.getBand: 192 terracotta bands from
// generateBands(noiseRandom.fromHashOf("minecraft:clay_bands")), offset per
// column by round(clay_bands_offset(x, 0, z) * 4) (verified with javap).
func (sc *surfaceCompiler) clayBands() (surfaceRule, error) {
	off, err := sc.c.compileNoise(json.RawMessage(`"minecraft:clay_bands_offset"`))
	if err != nil {
		return nil, err
	}
	bands := generateClayBands(sc.c.factory.fromHash("minecraft:clay_bands"))
	for _, b := range bands {
		sc.r.blocks[b] = true
	}
	n := len(bands)
	return func(c *surfaceCtx) string {
		i := int(math.Floor(off.sample(float64(c.x), 0, float64(c.z))*4 + 0.5))
		k := (c.y + i + n) % n
		if k < 0 {
			k += n
		}
		return bands[k]
	}, nil
}

func generateClayBands(r *noiseRandom) []string {
	const (
		terracotta = "minecraft:terracotta"
		orange     = "minecraft:orange_terracotta"
		yellow     = "minecraft:yellow_terracotta"
		brown      = "minecraft:brown_terracotta"
		red        = "minecraft:red_terracotta"
		white      = "minecraft:white_terracotta"
		lightGray  = "minecraft:light_gray_terracotta"
	)
	bs := make([]string, 192)
	for i := range bs {
		bs[i] = terracotta
	}
	for i := 0; i < len(bs); i++ {
		i += r.nextInt(5) + 1
		if i < len(bs) {
			bs[i] = orange
		}
	}
	makeClayBands(r, bs, 1, yellow)
	makeClayBands(r, bs, 2, brown)
	makeClayBands(r, bs, 1, red)
	j := r.nextInt(15-9+1) + 9
	for k, l := 0, 0; k < j && l < len(bs); l += r.nextInt(16) + 4 {
		bs[l] = white
		if l-1 > 0 && r.nextBoolean() {
			bs[l-1] = lightGray
		}
		if l+1 < len(bs) && r.nextBoolean() {
			bs[l+1] = lightGray
		}
		k++
	}
	return bs
}

func makeClayBands(r *noiseRandom, out []string, minSize int, state string) {
	n := r.nextInt(15-6+1) + 6
	for j := 0; j < n; j++ {
		k := minSize + r.nextInt(3)
		l := r.nextInt(len(out))
		for m := 0; l+m < len(out) && m < k; m++ {
			out[l+m] = state
		}
	}
}
