package modjar

import (
	"encoding/binary"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/Viking602/YoloMCL/cubiomes"
)

const (
	synthPackage  = "net/minecraft/world/level/levelgen/synth/"
	improvedClass = synthPackage + "ImprovedNoise"
	perlinClass   = synthPackage + "PerlinNoise"
)

// nativeNoiseFields is the vanilla field state of the primitives native
// sampling models. A mixin may only @Shadow fields listed here.
var nativeNoiseFields = map[string]map[string]string{
	improvedClass: {"xo": "D", "yo": "D", "zo": "D", "p": "[B"},
	perlinClass:   {"noiseLevels": "[L" + improvedClass + ";", "amplitudes": "Lit/unimi/dsi/fastutil/doubles/DoubleList;", "lowestFreqInputFactor": "D", "lowestFreqValueFactor": "D"},
}

// skeletonClass is an empty class file named after a game class. The game JAR
// is never loaded, so a transformed primitive is assembled from its mixins
// alone; any vanilla member the mixins rely on is absent and fails closed.
func skeletonClass(name string) []byte {
	utf := func(s string) []byte {
		b := []byte{1, 0, 0}
		binary.BigEndian.PutUint16(b[1:], uint16(len(s)))
		return append(b, s...)
	}
	b := []byte{0xCA, 0xFE, 0xBA, 0xBE, 0, 0, 0, 52, 0, 5}
	b = append(b, utf(name)...)
	b = append(b, 7, 0, 1)
	b = append(b, utf("java/lang/Object")...)
	b = append(b, 7, 0, 3)
	b = append(b, 0, 0x21, 0, 2, 0, 4, 0, 0, 0, 0, 0, 0, 0, 0)
	return b
}

// trivialMixinConstructor accepts a mixin constructor that only calls
// Object.<init> and stores zero/null defaults, which is all Mixin would not
// merge anyway (constructor merging is not performed).
func trivialMixinConstructor(m method) bool {
	code := m.code
	for i := 0; i < len(code); {
		switch op := code[i]; op {
		case 42, 1, 3, 9, 11, 14, 177:
			i++
		case 183, 181:
			i += 3
		default:
			return false
		}
	}
	return true
}

// noiseBindings serves vanilla methods of a primitive no enabled mixin
// touches: the native algorithm is the vanilla algorithm by construction.
type noiseBindings struct {
	vm    *machine
	plain map[string]bool
}

func (n *noiseBindings) static(r reference) (any, bool) { return nil, false }
func (n *noiseBindings) initializes(string) bool        { return false }
func (n *noiseBindings) call(r reference, receiver any, args []any) (any, bool) {
	o, ok := receiver.(*object)
	if !ok || o == nil || !n.plain[o.class] {
		return nil, false
	}
	ref, ok := o.data.(*cubiomes.ReferenceImproved)
	if !ok || o.class != improvedClass {
		return nil, false
	}
	switch r.name + r.desc {
	case "noise(DDD)D":
		return ref.Sample(number(args[0]), number(args[1]), number(args[2])), true
	case "noise(DDDDD)D":
		if number(args[3]) != 0 || number(args[4]) != 0 {
			fail("native ImprovedNoise y-scaled sampling not modelled")
		}
		return ref.Sample(number(args[0]), number(args[1]), number(args[2])), true
	}
	return nil, false
}

// nativeNoiseEquivalent decides whether the enabled mixins on the natively
// modelled noise primitives leave their sampled values unchanged. It builds
// each transformed primitive from its mixins over natively seeded vanilla
// state, runs the @Inject(<init>, RETURN) handlers, and compares the woven
// sampling methods bit for bit with native output across seeds, random kinds,
// octave layouts and coordinates (including the 2^25 wrap boundary). Any
// unsupported mixin shape, missing vanilla member, or difference keeps the
// transformation unresolved.
func (e *Environment) nativeNoiseEquivalent(target string) bool {
	if _, modelled := nativeNoiseFields[target]; !modelled || e.vm.weave == nil {
		return false
	}
	if !e.noiseChecked {
		e.noiseChecked = true
		e.checkNativeNoise()
	}
	return e.noiseReason == ""
}

func (e *Environment) checkNativeNoise() (ok bool) {
	e.noiseReason = "no comparison"
	defer func() {
		if x := recover(); x != nil {
			ok = false
			e.noiseReason = fmt.Sprint(x)
		}
		if ok {
			e.noiseReason = ""
		}
	}()
	v := newMachine(e.vm.ctx, nil)
	v.resolveClass = e.vm.resolveClass
	v.host = &bindings{vm: v, options: e.vm.host.options}
	nb := &noiseBindings{vm: v, plain: map[string]bool{}}
	v.host.extra = nb
	type initHook struct{ ref reference }
	hooks := map[string][]initHook{}
	for _, target := range []string{improvedClass, perlinClass} {
		v.raw[target] = skeletonClass(target)
		t := v.load(target)
		var specs []WeaveSpec
		for _, s := range e.vm.weave.targets[target] {
			if e.accessorOnlyMixin(s.Mixin) {
				// Accessors/invokers only expose existing members; enabled or
				// not, the target computes the same values.
				continue
			}
			if e.weaveSpecEnabled(s) {
				specs = append(specs, s)
			}
		}
		if len(specs) == 0 {
			nb.plain[target] = true
			continue
		}
		slices.SortStableFunc(specs, func(a, b WeaveSpec) int { return a.Priority - b.Priority })
		for _, spec := range specs {
			if !e.vm.fetchClass(spec.Mixin) {
				fail("declared mixin class missing: %s", spec.Mixin)
			}
			m := parseClass(e.vm.raw[spec.Mixin], e.vm.host.options.Symbols)
			if m.access&0x200 != 0 {
				fail("accessor mixin %s on native noise", spec.Mixin)
			}
			if m.super != "java/lang/Object" || len(m.interfaces) > 0 {
				fail("mixin %s changes the type of %s", spec.Mixin, target)
			}
			mw := retarget(m, t)
			for _, f := range m.fields {
				if hasAnnotation(f.annotations, mixinShadow) {
					if nativeNoiseFields[target][f.name] != f.desc {
						fail("mixin %s shadows unmodelled field %s.%s", spec.Mixin, target, f.name)
					}
					// The shadowed vanilla field is declared on the skeleton so
					// field resolution stops at the target as it would in game.
					if !slices.ContainsFunc(t.fields, func(x field) bool { return x.name == f.name }) {
						t.fields = append(t.fields, f)
					}
					continue
				}
				if _, clash := nativeNoiseFields[target][f.name]; clash {
					fail("mixin %s redeclares %s.%s", spec.Mixin, target, f.name)
				}
				t.fields = append(t.fields, f)
			}
			for _, mm := range m.methods {
				copy := mm
				copy.origin = mw
				copy.injects = nil
				switch {
				case mm.name == "<init>":
					if !trivialMixinConstructor(mm) {
						fail("mixin %s constructor initialises state", spec.Mixin)
					}
				case mm.name == "<clinit>":
					t.mixinInits = append(t.mixinInits, copy)
				case mm.access&0x400 != 0 || hasAnnotation(mm.annotations, mixinShadow):
				case hasAnnotation(mm.annotations, mixinOverwrite):
					if j := slices.IndexFunc(t.methods, func(x method) bool { return x.name == mm.name && x.desc == mm.desc }); j >= 0 {
						t.methods[j] = copy
					} else {
						t.methods = append(t.methods, copy)
					}
				default:
					if a, injector := injectorAnnotation(mm.annotations); injector {
						if frameworkName(a.typeName) != mixinInject || mm.access&8 != 0 || mm.desc != "(L"+callbackInfoClass+";)V" {
							fail("mixin %s.%s: unsupported injector on native noise", spec.Mixin, mm.name)
						}
						for key := range a.values {
							if key != "method" && key != "at" && key != "remap" && key != "require" {
								fail("mixin %s.%s: unsupported injector option %s", spec.Mixin, mm.name, key)
							}
						}
						sel := annotationList(a.values["method"])
						at := annotationList(a.values["at"])
						if len(sel) != 1 || len(at) != 1 {
							fail("mixin %s.%s: unsupported injector target", spec.Mixin, mm.name)
						}
						s, _ := sel[0].(string)
						point, _ := at[0].(annotation)
						value, _ := point.values["value"].(string)
						if s != "<init>" && !strings.HasSuffix(s, ";<init>") && !strings.HasPrefix(s, "<init>(") || value != "RETURN" || len(point.values) != 1 {
							fail("mixin %s.%s: only @Inject(<init>, RETURN) is modelled", spec.Mixin, mm.name)
						}
						copy.name = mm.name + "$" + strings.ReplaceAll(m.name, "/", "$")
						t.methods = append(t.methods, copy)
						hooks[target] = append(hooks[target], initHook{reference{target, copy.name, copy.desc}})
						continue
					}
					if slices.ContainsFunc(t.methods, func(x method) bool { return x.name == mm.name && x.desc == mm.desc }) {
						fail("mixin %s duplicates %s.%s", spec.Mixin, target, mm.name)
					}
					t.methods = append(t.methods, copy)
				}
			}
		}
	}
	construct := func(class string, data any, fields map[string]any) *object {
		o := v.alloc(class, data)
		for _, f := range v.load(class).fields {
			if f.access&8 == 0 {
				o.fields[fieldKey(reference{class, f.name, f.desc})] = defaultValue(f.desc)
			}
		}
		for name, value := range fields {
			o.fields[fieldKey(reference{class, name, nativeNoiseFields[class][name]})] = value
		}
		for _, h := range hooks[class] {
			ci := v.alloc(callbackInfoClass, &callbackState{name: "<init>"})
			v.method(h.ref, o, false, []any{ci})
		}
		return o
	}
	improved := func(r *cubiomes.ReferenceImproved) *object {
		p := make([]any, 256)
		for i, b := range r.P {
			p[i] = int32(int8(b))
		}
		return construct(improvedClass, r, map[string]any{"xo": r.Xo, "yo": r.Yo, "zo": r.Zo, "p": &array{values: p, descriptor: "[B"}})
	}
	sample := func(class, name, desc string, o *object, args ...any) float64 {
		if nb.plain[class] {
			fail("unreachable sampling of untransformed %s", class)
		}
		x, isFloat := v.method(reference{class, name, desc}, o, true, args).(float64)
		if !isFloat {
			fail("%s.%s did not produce a double", class, name)
		}
		return x
	}
	coords := []float64{0, 0.5, -1.25, 17.3, -123.456, 1e4 + 0.1, -3.3554432e7 + 3, 3.3554431e7, 1.6777216e7 - 0.5, 9.87654321e8}
	layouts := []struct {
		first int
		amps  []float64
	}{{-7, []float64{1, 0, 2, 1, 0.5}}, {-10, []float64{1.5, 0, 1, 0, 0, 0, 0}}, {-3, []float64{1}}}
	compared := 0
	check := func(want, got float64, what string, seed int64) {
		if math.Float64bits(want) != math.Float64bits(got) {
			fail("%s differs from native at seed %d", what, seed)
		}
		compared++
	}
	for _, seed := range []int64{0, 1, -1, 42, 0x5DEECE66D, -0x123456789ABCDEF} {
		for _, legacy := range []bool{false, true} {
			v.steps, v.objects, v.slots = 0, 0, 0
			if !nb.plain[improvedClass] {
				r := cubiomes.NewReferenceImproved(seed, legacy)
				o := improved(r)
				for a, x := range coords {
					y, z := coords[(a+3)%len(coords)]/7, coords[(a+7)%len(coords)]
					check(r.Sample(x, y, z), sample(improvedClass, "noise", "(DDDDD)D", o, x, y, z, 0.0, 0.0), "ImprovedNoise.noise", seed)
					check(r.Sample(x, 0, z), sample(improvedClass, "noise", "(DDDDD)D", o, x, 0.0, z, 0.0, 0.0), "ImprovedNoise.noise", seed)
				}
			}
			if !nb.plain[perlinClass] {
				for _, layout := range layouts {
					r := cubiomes.NewReferencePerlin(seed, legacy, layout.first, layout.amps)
					levels := make([]any, len(r.Octaves))
					for i, octave := range r.Octaves {
						if octave != nil {
							levels[i] = improved(octave)
						}
					}
					amps := make([]any, len(r.Amplitudes))
					for i, a := range r.Amplitudes {
						amps[i] = a
					}
					list := v.alloc("it/unimi/dsi/fastutil/doubles/DoubleArrayList", nil)
					v.method(reference{"it/unimi/dsi/fastutil/doubles/DoubleArrayList", "<init>", "([D)V"}, list, false, []any{&array{values: amps, descriptor: "[D"}})
					o := construct(perlinClass, nil, map[string]any{"noiseLevels": &array{values: levels, descriptor: "[L" + improvedClass + ";"}, "amplitudes": list, "lowestFreqInputFactor": r.InputFactor, "lowestFreqValueFactor": r.ValueFactor})
					for a, x := range coords {
						y, z := coords[(a+3)%len(coords)]/7, coords[(a+7)%len(coords)]
						check(r.Sample(x, y, z), sample(perlinClass, "getValue", "(DDD)D", o, x, y, z), "PerlinNoise.getValue", seed)
					}
				}
			}
		}
	}
	return compared > 0
}

// weaveSpecEnabled asks the plugin exactly as the transformation check does,
// through the mixin's declared effects, so both see one loader decision.
func (e *Environment) weaveSpecEnabled(s WeaveSpec) bool {
	defer func() {
		if x := recover(); x != nil {
			panic(fmt.Sprintf("mixin plugin decision for %s: %v", s.Mixin, x))
		}
	}()
	declared := false
	for _, effect := range e.MixinEffects {
		if effect.Class == s.Mixin && effect.Target == s.Target {
			declared = true
			if e.mixinEnabled(effect) {
				return true
			}
		}
	}
	return !declared && e.vm.weave.enabled(s)
}

const (
	mixinAccessor = "Lorg/spongepowered/asm/mixin/gen/Accessor;"
	mixinInvoker  = "Lorg/spongepowered/asm/mixin/gen/Invoker;"
)

// accessorOnlyMixin reports whether a mixin is an interface whose every
// method is a bodiless @Accessor or @Invoker and which declares no fields.
func (e *Environment) accessorOnlyMixin(name string) bool {
	if !e.vm.fetchClass(name) {
		return false
	}
	m := parseClass(e.vm.raw[name], e.vm.host.options.Symbols)
	if m.access&0x200 == 0 || len(m.fields) > 0 || len(m.methods) == 0 {
		return false
	}
	for _, method := range m.methods {
		if len(method.code) > 0 || !hasAnnotation(method.annotations, mixinAccessor) && !hasAnnotation(method.annotations, mixinInvoker) {
			return false
		}
	}
	return true
}
