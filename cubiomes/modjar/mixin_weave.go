package modjar

import (
	"slices"
	"strings"
)

// WeaveSpec names one declared mixin class for one target. A program carries
// only the specs whose plugin decision was already made in the loader session.
type WeaveSpec struct {
	Config, Mixin, Target, Plugin, Package string
	Priority                               int
}

type injectHook struct {
	at          string // HEAD or RETURN
	ref         reference
	static      bool
	params      int // target arguments the handler receives
	cancellable bool
}

// weaver applies declared mixins when their target class is first loaded,
// with Mixin's merge semantics for the supported subset: @Overwrite, merged
// (unique, synthetic) members, static initializer merge and non-capturing
// @Inject at HEAD/RETURN/TAIL. Anything else stays unapplied and remains a
// declared effect for the consumers that refuse unresolved transformations.
type weaver struct {
	targets map[string][]WeaveSpec
	enabled func(WeaveSpec) bool
	applied map[string]bool
	used    []WeaveSpec
	active  map[string]bool
}

func effectKey(class, method, desc string) string { return class + "." + method + desc }

func (w *weaver) add(spec WeaveSpec) {
	for _, s := range w.targets[spec.Target] {
		if s.Mixin == spec.Mixin {
			return
		}
	}
	if w.targets == nil {
		w.targets = map[string][]WeaveSpec{}
	}
	w.targets[spec.Target] = append(w.targets[spec.Target], spec)
}

const (
	mixinShadow    = "Lorg/spongepowered/asm/mixin/Shadow;"
	mixinOverwrite = "Lorg/spongepowered/asm/mixin/Overwrite;"
	mixinInject    = "Lorg/spongepowered/asm/mixin/injection/Inject;"
)

func hasAnnotation(list []annotation, name string) bool {
	_, ok := findAnnotation(list, name)
	return ok
}

func injectorAnnotation(list []annotation) (annotation, bool) {
	for _, a := range list {
		n := frameworkName(a.typeName)
		if strings.HasPrefix(n, "Lorg/spongepowered/asm/mixin/injection/") || strings.HasPrefix(n, "Lcom/llamalad7/mixinextras/injector/") {
			return a, true
		}
	}
	return annotation{}, false
}

// retarget gives the mixin's code the target's identity: every reference to
// the mixin class (owner, descriptor, class literal) resolves to the target,
// exactly as Mixin rewrites merged bytecode.
func retarget(m, t *class) *class {
	mw := *m
	mw.pool = slices.Clone(m.pool)
	for i, k := range mw.pool {
		if k.tag != 1 {
			continue
		}
		s, _ := k.value.(string)
		if s == m.name {
			s = t.name
		} else if strings.Contains(s, "L"+m.name+";") {
			s = strings.ReplaceAll(s, "L"+m.name+";", "L"+t.name+";")
		}
		mw.pool[i].value = s
	}
	mw.name, mw.super, mw.interfaces = t.name, t.super, t.interfaces
	return &mw
}

func (v *machine) weaveClass(t *class) {
	w := v.weave
	if w.active[t.name] {
		fail("mixin decision for %s depends on its own target", t.name)
	}
	if w.active == nil {
		w.active = map[string]bool{}
	}
	w.active[t.name] = true
	defer delete(w.active, t.name)
	specs := slices.Clone(w.targets[t.name])
	// Lower priority applies first; a higher priority overwrite wins.
	slices.SortStableFunc(specs, func(a, b WeaveSpec) int { return a.Priority - b.Priority })
	for _, spec := range specs {
		if !w.enabled(spec) {
			continue
		}
		if !v.fetchClass(spec.Mixin) {
			fail("declared mixin class missing: %s", spec.Mixin)
		}
		var symbols *Symbols
		if v.host != nil {
			symbols = v.host.options.Symbols
		}
		m := parseClass(v.raw[spec.Mixin], symbols)
		if m.access&0x200 != 0 {
			continue // accessor/invoker interfaces are generated, not merged
		}
		v.mergeMixin(t, m, retarget(m, t))
		w.used = append(w.used, spec)
	}
}

func (v *machine) mergeMixin(t, m, mw *class) {
	w := v.weave
	for _, f := range m.fields {
		if hasAnnotation(f.annotations, mixinShadow) {
			continue
		}
		if !slices.ContainsFunc(t.fields, func(x field) bool { return x.name == f.name && x.desc == f.desc }) {
			t.fields = append(t.fields, f)
		}
	}
	for _, i := range m.interfaces {
		if !slices.Contains(t.interfaces, i) {
			t.interfaces = append(t.interfaces, i)
		}
	}
	find := func(name, desc string) int {
		return slices.IndexFunc(t.methods, func(x method) bool { return x.name == name && x.desc == desc })
	}
	for _, mm := range m.methods {
		copy := mm
		copy.origin = mw
		copy.injects = nil
		switch {
		case mm.name == "<init>":
			// Constructor merging (instance field initialisers) is not
			// performed; such members keep their declared default.
			continue
		case mm.name == "<clinit>":
			t.mixinInits = append(t.mixinInits, copy)
			w.applied[effectKey(m.name, mm.name, mm.desc)] = true
			continue
		case mm.access&0x400 != 0 || hasAnnotation(mm.annotations, mixinShadow):
			continue
		case hasAnnotation(mm.annotations, mixinOverwrite):
			j := find(mm.name, mm.desc)
			if j < 0 || t.methods[j].access&8 != mm.access&8 {
				continue
			}
			copy.access = mm.access
			t.methods[j] = copy
			w.applied[effectKey(m.name, mm.name, mm.desc)] = true
			continue
		}
		if a, ok := injectorAnnotation(mm.annotations); ok {
			if frameworkName(a.typeName) != mixinInject {
				continue
			}
			v.applyInject(t, m, mm, copy, a)
			continue
		}
		if find(mm.name, mm.desc) < 0 {
			t.methods = append(t.methods, copy)
		}
	}
}

func (v *machine) applyInject(t, m *class, mm, handler method, a annotation) {
	if locals, ok := a.values["locals"].(enumValue); ok && locals.name != "NO_CAPTURE" {
		return
	}
	for key := range a.values {
		switch key {
		case "method", "at", "cancellable", "remap", "require", "expect", "allow", "id", "locals", "order":
		default:
			return // slice/target-qualified injectors are not interpreted
		}
	}
	points := annotationList(a.values["at"])
	if len(points) == 0 {
		return
	}
	var ats []string
	for _, raw := range points {
		at, ok := raw.(annotation)
		if !ok {
			return
		}
		value, _ := at.values["value"].(string)
		if value != "HEAD" && value != "RETURN" && value != "TAIL" {
			return
		}
		for key, x := range at.values {
			switch key {
			case "value", "remap", "id":
			case "ordinal":
				if n, ok := x.(int32); !ok || n != -1 {
					return
				}
			case "shift":
				if e, ok := x.(enumValue); !ok || e.name != "NONE" {
					return
				}
			default:
				return
			}
		}
		ats = append(ats, value)
	}
	cancellable := false
	if n, ok := a.values["cancellable"].(int32); ok {
		cancellable = n != 0
	}
	hparams, hret := descriptor(mm.desc)
	if hret != "V" || len(hparams) == 0 {
		return
	}
	ci := hparams[len(hparams)-1]
	if ci != "L"+callbackInfoClass+";" && ci != "L"+callbackReturnClass+";" {
		return
	}
	type match struct {
		index int
		at    []string
	}
	var targets []match
	for _, raw := range annotationList(a.values["method"]) {
		sel, ok := raw.(string)
		if !ok || strings.ContainsAny(sel, "*") {
			return
		}
		if strings.HasPrefix(sel, "L") {
			if i := strings.Index(sel, ";"); i > 0 {
				sel = sel[i+1:]
			}
		}
		name, desc := sel, ""
		if i := strings.Index(sel, "("); i >= 0 {
			name, desc = sel[:i], sel[i:]
		}
		for j, tm := range t.methods {
			if tm.name != name || desc != "" && tm.desc != desc {
				continue
			}
			tparams, tret := descriptor(tm.desc)
			if (tm.access&8 != 0) != (mm.access&8 != 0) || (tret == "V") != (ci == "L"+callbackInfoClass+";") {
				return
			}
			if n := len(hparams) - 1; n != 0 && (n != len(tparams) || !slices.Equal(hparams[:n], tparams)) {
				return
			}
			var at []string
			for _, x := range ats {
				switch {
				case x == "HEAD" && tm.name == "<init>":
					return // constructor HEAD runs after super(); not modelled
				case x == "TAIL":
					returns := 0
					methodReferencesChecked(t, tm, func(op byte, _ int) {
						if op >= 172 && op <= 177 {
							returns++
						}
					})
					if tm.origin != nil || returns != 1 {
						return
					}
					at = append(at, "RETURN")
				default:
					at = append(at, x)
				}
			}
			targets = append(targets, match{j, at})
		}
	}
	if len(targets) == 0 {
		return
	}
	// Merged handlers get a unique name, as Mixin renames them.
	handler.name = mm.name + "$" + strings.ReplaceAll(m.name, "/", "$")
	t.methods = append(t.methods, handler)
	ref := reference{t.name, handler.name, handler.desc}
	for _, x := range targets {
		for _, at := range x.at {
			t.methods[x.index].injects = append(t.methods[x.index].injects, injectHook{at: at, ref: ref, static: mm.access&8 != 0, params: len(hparams) - 1, cancellable: cancellable})
		}
	}
	v.weave.applied[effectKey(m.name, mm.name, mm.desc)] = true
}

var primitiveBoxes = map[string][2]string{
	"Z": {"java/lang/Boolean", "(Z)Ljava/lang/Boolean;"}, "B": {"java/lang/Byte", "(B)Ljava/lang/Byte;"},
	"C": {"java/lang/Character", "(C)Ljava/lang/Character;"}, "S": {"java/lang/Short", "(S)Ljava/lang/Short;"},
	"I": {"java/lang/Integer", "(I)Ljava/lang/Integer;"}, "J": {"java/lang/Long", "(J)Ljava/lang/Long;"},
	"F": {"java/lang/Float", "(F)Ljava/lang/Float;"}, "D": {"java/lang/Double", "(D)Ljava/lang/Double;"},
}

// runInjected executes a woven method: HEAD callbacks, the body, merged
// static initialisers (for <clinit>), then RETURN callbacks.
func (v *machine) runInjected(c *class, m *method, receiver any, args []any, merged []method) any {
	_, ret := descriptor(m.desc)
	box, primitive := primitiveBoxes[ret]
	class := callbackInfoClass
	if ret != "V" {
		class = callbackReturnClass
	}
	call := func(h injectHook, value any) (any, bool) {
		if primitive && value != nil {
			value = v.method(reference{box[0], "valueOf", box[1]}, nil, false, []any{value})
		}
		state := &callbackState{name: m.name, cancellable: h.cancellable, value: value}
		ci := v.alloc(class, state)
		hargs := append(append([]any(nil), args[:h.params]...), ci)
		var recv any
		if !h.static {
			recv = receiver
		}
		v.method(h.ref, recv, false, hargs)
		out := state.value
		if primitive && out != nil {
			out = objectData(out)
		}
		return out, state.cancelled
	}
	for _, h := range m.injects {
		if h.at == "HEAD" {
			if x, cancelled := call(h, nil); cancelled {
				return x
			}
		}
	}
	plain := *m
	plain.injects = nil
	var result any
	if len(plain.code) > 0 {
		result = v.run(c, &plain, receiver, args)
	}
	for i := range merged {
		v.run(c, &merged[i], nil, nil)
	}
	for _, h := range m.injects {
		if h.at == "RETURN" {
			if x, cancelled := call(h, result); cancelled {
				return x
			}
		}
	}
	return result
}
