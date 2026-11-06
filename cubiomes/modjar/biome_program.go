package modjar

import (
	"context"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/Viking602/YoloMCL/cubiomes"
)

// NoiseInput supplies unquantized RandomState density functions at the exact
// block coordinates selected by a loaded biome source.
type NoiseInput interface {
	Compute(channel int, x, y, z int32) (float64, bool)
}
type densityInput struct {
	router  NoiseInput
	channel int
}

type ProgramMethod struct{ Class, Name, Descriptor string }

func (m ProgramMethod) ref() reference        { return reference{m.Class, m.Name, m.Descriptor} }
func programMethod(r reference) ProgramMethod { return ProgramMethod{r.owner, r.name, r.desc} }

// BiomeProgram is a bounded, self-contained numerical program captured from an
// executed registry codec. All class bytes are inputs, not generated Java code.
// Registry-holder inputs are checked against the effective datapack at bind time.
type BiomeProgram struct {
	Symbols       *Symbols `json:",omitempty"`
	ID, Minecraft string
	Factory       ProgramMethod
	FactoryKind   int
	Holders       []string
	Classes       map[string][]byte
	State         map[string]string
	EndHeight     *ProgramMethod              `json:",omitempty"`
	NoiseScope    *cubiomes.NoiseRequirements `json:",omitempty"`
	// Mixins are the woven mixins whose plugin decision the loader made.
	Mixins []WeaveSpec `json:",omitempty"`
}

const endIslandClass = "net/minecraft/world/level/levelgen/DensityFunctions$EndIslandDensityFunction"
const endHeightDescriptor = "(L" + simplexClass + ";II)F"
const biomeQueryDescriptor = "(IIIL" + climateSamplerClass + ";)Lnet/minecraft/core/Holder;"

// BiomeProgram captures only sources whose complete query graph is stateless.
// Unrelated item/gameplay initialization cannot change this subset's immutable
// holder inputs. Unsupported relevant transformations prevent compilation.
func (e *Environment) BiomeProgram(id string, scope ...cubiomes.NoiseRequirements) (program BiomeProgram, err error) {
	defer func() {
		if p := recover(); p != nil {
			program = BiomeProgram{}
			err = fmt.Errorf("biome program %s: %v", id, p)
		}
	}()
	codec, ok := e.api.registries[biomeSourceRegistry][id]
	if !ok {
		return program, fmt.Errorf("unregistered biome source %s", id)
	}
	recipe, ok := objectData(codec).(recordRecipe)
	if !ok {
		return program, fmt.Errorf("unsupported registry codec %s", id)
	}
	if len(recipe.factory.captured) != 0 {
		return program, fmt.Errorf("biome codec captures initialization state")
	}
	program = BiomeProgram{ID: id, Minecraft: e.vm.host.options.Minecraft, Factory: programMethod(recipe.factory.target), FactoryKind: recipe.factory.kind, Symbols: e.vm.host.options.Symbols}
	if len(scope) > 0 {
		s := scope[0]
		program.NoiseScope = &s
	}
	args := make([]any, len(recipe.elements))
	for i, element := range recipe.elements {
		key, ok := objectData(element).(registryElement)
		if !ok || !strings.Contains(key.id, ":") {
			return BiomeProgram{}, fmt.Errorf("biome codec requires unsupported dynamic input")
		}
		program.Holders = append(program.Holders, key.id)
		args[i] = e.vm.host.symbol("holder:"+key.id, "net/minecraft/core/Holder", key.id)
	}
	source := e.vm.callLambda(recipe.factory, args)
	if !e.vm.instanceOf(source, biomeSourceClass) {
		return BiomeProgram{}, fmt.Errorf("codec did not construct a biome source")
	}
	program.State = immutableBiomeState(source.(*object))
	root := reference{source.(*object).class, "getNoiseBiome", biomeQueryDescriptor}
	seen := map[reference]bool{}
	proveBiomeQuery(e.vm, root, seen)
	// Validate overwrites first so helper classes in their call graphs also
	// participate in transformation checks, independent of archive order.
	for _, effect := range e.MixinEffects {
		if effect.Target == endIslandClass {
			if program.NoiseScope != nil && !program.NoiseScope.EndIslands {
				continue
			}
			if !e.mixinEnabled(effect) {
				continue
			}
			if effect.Kind != "Lorg/spongepowered/asm/mixin/Overwrite;" || effect.Method != "getHeightValue" || effect.Descriptor != endHeightDescriptor || program.EndHeight != nil {
				return BiomeProgram{}, fmt.Errorf("unresolved density transformation %s.%s", effect.Class, effect.Method)
			}
			method := ProgramMethod{effect.Class, effect.Method, effect.Descriptor}
			proveBiomeQuery(e.vm, method.ref(), seen)
			program.EndHeight = &method
		}
	}
	owners := map[string]bool{}
	for method := range seen {
		owners[method.owner] = true
	}
	for _, effect := range e.MixinEffects {
		if effect.Target == endIslandClass {
			continue
		}
		if w := e.vm.weave; w != nil && w.applied[effectKey(effect.Class, effect.Method, effect.Descriptor)] {
			continue // merged into the target; the program reproduces it
		}
		if program.NoiseScope != nil && !program.NoiseScope.NormalNoise {
			switch effect.Target {
			case "net/minecraft/world/level/levelgen/synth/PerlinNoise", "net/minecraft/world/level/levelgen/synth/ImprovedNoise", "net/minecraft/world/level/levelgen/synth/NormalNoise", "net/minecraft/world/level/levelgen/synth/BlendedNoise":
				continue
			}
		}
		// A helper's unrelated methods do not execute on the proved query path.
		if owners[effect.Target] && len(effect.Selectors) > 0 {
			reached := false
			for _, selector := range effect.Selectors {
				if strings.Contains(selector, "<init>") {
					reached = true
				}
				// The query graph rejects static reads (proveBiomeQuery), so a
				// class initializer can only matter through state it produced
				// during this session. One that never ran produced none.
				if strings.Contains(selector, "<clinit>") && e.vm.initialized[effect.Target] {
					reached = true
				}
			}
			for method := range seen {
				if method.owner != effect.Target {
					continue
				}
				for _, selector := range effect.Selectors {
					if selector == method.name || selector == method.name+method.desc || strings.ContainsAny(selector, "*;") {
						reached = true
					}
				}
			}
			if !reached {
				continue
			}
		}
		// DensityFunctions$* and RandomState mixins only matter when that class is
		// on the proved query path. C2ME DFC Inject/WrapOperation handlers compile
		// density functions to output-equivalent bytecode or vectorize fillArray;
		// when those classes are never queried, biome preview must not fail closed.
		densityFnLike := strings.HasPrefix(effect.Target, densityClass+"$") || strings.HasPrefix(effect.Target, "net/minecraft/world/level/levelgen/DensityFunctions$")
		if (densityFnLike || effect.Target == "net/minecraft/world/level/levelgen/RandomState") && !owners[effect.Target] {
			continue
		}
		if owners[effect.Target] || effect.Target == biomeSourceClass || effect.Target == climateSamplerClass || effect.Target == densityClass || densityFnLike || strings.HasPrefix(effect.Target, "net/minecraft/world/level/levelgen/synth/") || effect.Target == "net/minecraft/world/level/levelgen/RandomState" {
			if !e.mixinEnabled(effect) {
				if e.opaqueNativePlugin(effect) {
					return BiomeProgram{}, fmt.Errorf("unresolved biome transformation %s.%s (opaque native mixin plugin %s)", effect.Class, effect.Method, effect.Plugin)
				}
				continue
			}
			if strings.HasPrefix(effect.Target, synthPackage) && e.nativeNoiseEquivalent(effect.Target) {
				continue // woven primitive is value-identical to the native one
			}
			if _, modelled := nativeNoiseFields[effect.Target]; modelled && e.noiseReason != "" {
				why := e.noiseReason
				return BiomeProgram{}, fmt.Errorf("unresolved biome transformation %s.%s (native equivalence: %s)", effect.Class, effect.Method, why)
			}
			return BiomeProgram{}, fmt.Errorf("unresolved biome transformation %s.%s", effect.Class, effect.Method)
		}
	}
	program.Classes = maps.Clone(e.vm.raw)
	if w := e.vm.weave; w != nil {
		for _, spec := range w.used {
			if program.Classes[spec.Target] != nil {
				program.Mixins = append(program.Mixins, spec)
			}
		}
	}
	return program, nil
}

// BiomeRuntime belongs to one sampling worker. The compiler rejects mutable
// fields, array writes, and unresolved calls in every branch of its query graph.
type BiomeRuntime struct {
	vm             *machine
	source         any
	climate        *object
	allowed        map[string]bool
	endHeight      *ProgramMethod
	objects, slots int
}

func (p BiomeProgram) NewRuntime(ctx context.Context) (runtime *BiomeRuntime, err error) {
	defer func() {
		if x := recover(); x != nil {
			runtime = nil
			err = fmt.Errorf("biome runtime: %v", x)
		}
	}()
	if len(p.Classes) > 1024 || len(p.Holders) > 256 || p.FactoryKind != 6 && p.FactoryKind != 8 {
		return nil, fmt.Errorf("invalid biome program")
	}
	size := 0
	for _, data := range p.Classes {
		size += len(data)
		if len(data) > 2<<20 || size > 16<<20 {
			return nil, fmt.Errorf("biome program class budget")
		}
	}
	v := newMachine(ctx, maps.Clone(p.Classes))
	v.host = &bindings{vm: v, options: Options{Minecraft: p.Minecraft, Symbols: p.Symbols}}
	v.host.extra = &biomeProbeBindings{host: v.host}
	if len(p.Mixins) > 0 {
		w := &weaver{applied: map[string]bool{}, enabled: func(WeaveSpec) bool { return true }}
		for _, spec := range p.Mixins {
			w.add(spec)
		}
		v.weave = w
	}
	args := make([]any, len(p.Holders))
	allowed := map[string]bool{}
	for i, id := range p.Holders {
		allowed[id] = true
		args[i] = v.host.symbol("holder:"+id, "net/minecraft/core/Holder", id)
	}
	source := v.callLambda(&lambda{target: p.Factory.ref(), kind: p.FactoryKind}, args)
	if !v.instanceOf(source, biomeSourceClass) {
		return nil, fmt.Errorf("invalid biome source factory")
	}
	if !maps.Equal(p.State, immutableBiomeState(source.(*object))) {
		return nil, fmt.Errorf("biome source factory depends on uncaptured initialization state")
	}
	seen := map[reference]bool{}
	proveBiomeQuery(v, reference{source.(*object).class, "getNoiseBiome", biomeQueryDescriptor}, seen)
	if p.EndHeight != nil {
		proveBiomeQuery(v, p.EndHeight.ref(), seen)
		v.initialize(p.EndHeight.Class)
	}
	climate := v.alloc(climateSamplerClass, nil)
	return &BiomeRuntime{vm: v, source: source, climate: climate, allowed: allowed, endHeight: p.EndHeight, objects: v.objects, slots: v.slots}, nil
}

// Query fields are proved final and primitive/holder-valued. Recreating a
// factory must reproduce these values exactly, including signed zero, rather
// than silently using default mod/configuration conditions in a fresh worker.
func immutableBiomeState(source *object) map[string]string {
	state := map[string]string{}
	for key, value := range source.fields {
		switch x := value.(type) {
		case int32:
			state[key] = fmt.Sprintf("I:%d", x)
		case int64:
			state[key] = fmt.Sprintf("J:%d", x)
		case float32:
			state[key] = fmt.Sprintf("F:%08x", math.Float32bits(x))
		case float64:
			state[key] = fmt.Sprintf("D:%016x", math.Float64bits(x))
		case string:
			state[key] = "S:" + x
		case nil:
			state[key] = "null"
		case *object:
			if x != nil && x.class == "net/minecraft/core/Holder" {
				if id, ok := x.data.(string); ok {
					state[key] = "H:" + id
				}
			}
		}
	}
	return state
}

func (r *BiomeRuntime) Query(ctx context.Context, x, y, z int32, input NoiseInput) (id string, err error) {
	defer func() {
		r.climate.data = nil
		if p := recover(); p != nil {
			id = ""
			err = fmt.Errorf("biome query: %v", p)
		}
		if ctx.Err() != nil {
			id = ""
			err = ctx.Err()
		}
	}()
	if err = ctx.Err(); err != nil {
		return "", err
	}
	v := r.vm
	v.ctx = ctx
	v.steps = 0
	v.objects = r.objects
	v.slots = r.slots
	r.climate.data = input
	value := v.method(reference{biomeSourceClass, "getNoiseBiome", biomeQueryDescriptor}, r.source, true, []any{x, y, z, r.climate})
	id, _ = objectData(value).(string)
	if !r.allowed[id] {
		return "", fmt.Errorf("biome query returned a holder outside its codec inputs")
	}
	return id, nil
}

func (r *BiomeRuntime) EndHeight(simplex func(float64, float64) float64, x, z int32) (height float32, ok bool) {
	defer func() {
		if recover() != nil {
			height, ok = 0, false
		}
	}()
	if r.endHeight == nil {
		return 0, false
	}
	v := r.vm
	noise := v.alloc(simplexClass, simplex)
	value := v.method(r.endHeight.ref(), nil, false, []any{noise, x, z})
	height, ok = value.(float32)
	return
}

func (p BiomeProgram) Biomes() []string {
	ids := slices.Clone(p.Holders)
	slices.Sort(ids)
	return slices.Compact(ids)
}
