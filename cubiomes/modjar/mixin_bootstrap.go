package modjar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

const biomeSourcesClass = "net/minecraft/world/level/biome/BiomeSources"

// MixinEffect identifies a declared transformation. It is retained separately
// from successful registry execution: a working codec does not prove that
// noise, features, or structure generation have also been transformed.
type MixinEffect struct {
	Config, Class, Target, Method, Descriptor, Kind string
	Plugin, Package                                 string
	Priority                                        int
	Selectors                                       []string
}

type bootstrapHook struct {
	ref             reference
	at              string
	cancellable     bool
	priority, order int
}

func (e *Environment) bootstrapMixins(ctx context.Context, archives []Archive, options Options) error {
	var hooks []bootstrapHook
	if e.vm.weave == nil {
		e.vm.weave = &weaver{applied: map[string]bool{}, enabled: func(s WeaveSpec) bool {
			return e.mixinEnabled(MixinEffect{Config: s.Config, Class: s.Mixin, Target: s.Target, Plugin: s.Plugin, Package: s.Package})
		}}
	}
	for _, archive := range archives {
		configs, err := declaredMixins(ctx, archive.Reader, options)
		if err != nil {
			return fmt.Errorf("%s: %w", archive.Name, err)
		}
		for _, name := range configs {
			raw, err := mixinFile(ctx, archive.Reader, name)
			if err != nil {
				return err
			}
			if raw == nil {
				// Loader-declared libraries can supply a config named by the
				// owning mod. Resolve it through the selected classpath as well.
				if resources := e.vm.host.options.Resources; resources != nil {
					candidates, e := resources(name)
					if e != nil {
						return e
					}
					for _, candidate := range candidates {
						if len(candidate) > 2<<20 {
							return fmt.Errorf("mixin config size limit: %s", name)
						}
						if raw != nil && !bytes.Equal(raw, candidate) {
							return fmt.Errorf("ambiguous mixin config resource: %s", name)
						}
						raw = candidate
					}
				}
			}
			if raw == nil {
				return fmt.Errorf("declared mixin config missing: %s", name)
			}
			cfg := mixinConfig{Priority: 1000, MixinPriority: 1000}
			raw, err = mixinJSON(raw)
			if err == nil {
				err = json.Unmarshal(raw, &cfg)
			}
			if err != nil {
				return fmt.Errorf("mixin config %s: %w", name, err)
			}
			classes := append([]*string(nil), cfg.Mixins...)
			if options.Side == "CLIENT" {
				classes = append(classes, cfg.Client...)
			} else {
				classes = append(classes, cfg.Server...)
			}
			if len(classes) > 32768 {
				return fmt.Errorf("mixin class limit")
			}
			for _, relative := range classes {
				if relative == nil {
					continue
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				className := strings.ReplaceAll(cfg.Package+"."+*relative, ".", "/")
				if cfg.Package == "" {
					continue // Mixin 0.8.7 does not load orphaned mixins without a package.
				}
				// Inspect declarations in their owning archive. Resolve the shared
				// executable definition only when a relevant transformation runs;
				// an unrelated networking library conflict must not disable biomes.
				data, err := mixinFile(ctx, archive.Reader, className+".class")
				if err == nil && data == nil {
					data, err = e.vm.resolveClass(className)
				}
				if err != nil {
					return err
				}
				if data == nil {
					return fmt.Errorf("declared mixin class missing: %s", className)
				}
				c := parseClass(data, e.vm.host.options.Symbols)
				a, ok := findAnnotation(c.annotations, "Lorg/spongepowered/asm/mixin/Mixin;")
				if !ok {
					return fmt.Errorf("declared class is not a mixin: %s", className)
				}
				priority := cfg.MixinPriority
				if n, ok := a.values["priority"].(int32); ok {
					priority = int(n)
				}
				for _, target := range mixinTargets(a) {
					e.vm.weave.add(WeaveSpec{Config: name, Mixin: c.name, Target: target, Plugin: cfg.Plugin, Package: cfg.Package, Priority: priority})
					for _, m := range c.methods {
						for _, inject := range m.annotations {
							if !strings.HasPrefix(inject.typeName, "Lorg/spongepowered/asm/mixin/injection/") && !strings.HasPrefix(inject.typeName, "Lcom/llamalad7/mixinextras/injector/") && inject.typeName != "Lorg/spongepowered/asm/mixin/Overwrite;" {
								continue
							}
							effect := MixinEffect{Config: name, Class: c.name, Target: target, Method: m.name, Descriptor: m.desc, Kind: inject.typeName, Priority: priority, Plugin: cfg.Plugin, Package: cfg.Package}
							for _, selector := range annotationList(inject.values["method"]) {
								if s, ok := selector.(string); ok {
									effect.Selectors = append(effect.Selectors, s)
								}
							}
							if inject.typeName == "Lorg/spongepowered/asm/mixin/Overwrite;" {
								effect.Selectors = []string{m.name + m.desc}
							}
							e.MixinEffects = append(e.MixinEffects, effect)
							if len(e.MixinEffects) > 32768 {
								return fmt.Errorf("mixin effect limit")
							}
							if target != biomeSourcesClass {
								continue
							}
							if !e.mixinEnabled(effect) {
								continue
							}
							hook, err := parseBootstrapHook(c, m, inject, cfg, priority)
							if err != nil {
								return fmt.Errorf("%s: %w", name, err)
							}
							hooks = append(hooks, hook)
						}
					}
				}
			}
		}
	}
	if len(hooks) == 0 || e.weaveOnly {
		return nil
	}
	// MapCodec became the registry element in 1.20.5. Older Codec bootstraps
	// require a different descriptor; never reinterpret their bytecode as this ABI.
	if !slices.Contains([]string{"1.20.5", "1.20.6", "1.21", "1.21.1", "1.21.2", "1.21.3", "1.21.4", "1.21.5", "1.21.6", "1.21.7", "1.21.8", "1.21.9", "1.21.10", "1.21.11"}, options.Minecraft) {
		return fmt.Errorf("biome bootstrap ABI requires a verified version profile: %s", options.Minecraft)
	}
	slices.SortStableFunc(hooks, func(a, b bootstrapHook) int {
		if a.order != b.order {
			return a.order - b.order
		}
		return b.priority - a.priority
	})
	v := e.vm
	registry := v.host.symbol("bootstrap:biome_source", "net/minecraft/core/Registry", biomeSourceRegistry)
	call := func(at string, result any) (any, bool) {
		for _, hook := range hooks {
			if hook.at != at {
				continue
			}
			state := &callbackState{name: "bootstrap", cancellable: hook.cancellable, value: result}
			ci := v.alloc(callbackReturnClass, state)
			args := []any{registry, ci}
			if hook.ref.desc == "(L"+callbackReturnClass+";)V" {
				args = []any{ci}
			}
			v.method(hook.ref, nil, false, args)
			if state.cancelled {
				return state.value, true
			}
		}
		return result, false
	}
	if _, cancelled := call("HEAD", nil); cancelled {
		return nil
	}
	var result any
	for _, id := range []string{"minecraft:fixed", "minecraft:multi_noise", "minecraft:checkerboard", "minecraft:the_end"} {
		result = v.alloc("com/mojang/serialization/MapCodec", nativeBiomeCodec(id))
		e.api.register(biomeSourceRegistry, id, result)
	}
	_, _ = call("RETURN", result)
	return nil
}

type nativeBiomeCodec string

func parseBootstrapHook(c *class, m method, a annotation, cfg mixinConfig, priority int) (bootstrapHook, error) {
	h := bootstrapHook{ref: reference{c.name, m.name, m.desc}, priority: priority, order: 1000}
	if a.typeName != "Lorg/spongepowered/asm/mixin/injection/Inject;" || m.access&8 == 0 {
		return h, fmt.Errorf("unsupported registry transformation %s.%s", c.name, m.name)
	}
	methods := annotationList(a.values["method"])
	if len(methods) != 1 {
		return h, fmt.Errorf("ambiguous bootstrap selector")
	}
	selector, ok := methods[0].(string)
	if !ok || (selector != "bootstrap" && selector != "bootstrap(Lnet/minecraft/core/Registry;)Lcom/mojang/serialization/MapCodec;") {
		return h, fmt.Errorf("unsupported bootstrap selector %v", methods)
	}
	for key, value := range a.values {
		switch key {
		case "method", "at", "cancellable", "remap", "require", "expect", "allow", "order", "id":
		case "locals":
			if v, ok := value.(enumValue); !ok || v.name != "NO_CAPTURE" {
				return h, fmt.Errorf("unsupported bootstrap local capture")
			}
		default:
			return h, fmt.Errorf("unsupported bootstrap injector option %s", key)
		}
	}
	if m.desc != "(Lnet/minecraft/core/Registry;L"+callbackReturnClass+";)V" && m.desc != "(L"+callbackReturnClass+";)V" {
		return h, fmt.Errorf("unsupported bootstrap callback descriptor %s", m.desc)
	}
	points := annotationList(a.values["at"])
	if len(points) != 1 {
		return h, fmt.Errorf("unsupported bootstrap injection points")
	}
	at, ok := points[0].(annotation)
	if !ok {
		return h, fmt.Errorf("invalid bootstrap injection point")
	}
	h.at, _ = at.values["value"].(string)
	if h.at == "TAIL" {
		h.at = "RETURN"
	}
	if h.at != "HEAD" && h.at != "RETURN" {
		return h, fmt.Errorf("unsupported bootstrap point %s", h.at)
	}
	for key, value := range at.values {
		switch key {
		case "value", "remap", "id":
		case "ordinal":
			n, ok := value.(int32)
			if !ok || n < -1 || n > 0 {
				return h, fmt.Errorf("bootstrap return ordinal does not match")
			}
		case "shift":
			if v, ok := value.(enumValue); !ok || v.name != "NONE" {
				return h, fmt.Errorf("shifted bootstrap injection unsupported")
			}
		default:
			return h, fmt.Errorf("unsupported bootstrap point option %s", key)
		}
	}
	require := cfg.Injectors.DefaultRequire
	if n, ok := a.values["require"].(int32); ok && n >= 0 {
		require = int(n)
	}
	if require > 1 {
		return h, fmt.Errorf("bootstrap injection count 1 is below required %d", require)
	}
	if n, ok := a.values["order"].(int32); ok {
		h.order = int(n)
	}
	if n, ok := a.values["cancellable"].(int32); ok {
		h.cancellable = n != 0
	}
	return h, nil
}

// BootstrapBiomeSources executes the registry bootstrap phase only. Consumers
// must additionally validate relevant transformations and initialization state
// before using a captured codec as a prediction program.
func BootstrapBiomeSources(ctx context.Context, archives []Archive, options Options) (env *Environment, err error) {
	var cp *Classpath
	defer func() {
		if p := recover(); p != nil {
			env = nil
			err = fmt.Errorf("worldgen bootstrap: %v", p)
		}
		if ctx.Err() != nil {
			env = nil
			err = ctx.Err()
		}
		if err != nil && cp != nil {
			cp.Close()
		}
	}()
	cp, err = NewClasspath(ctx, archives, options.Loader)
	if err != nil {
		return nil, err
	}
	v := newMachine(ctx, nil)
	if options.Resources == nil {
		options.Resources = cp.Resources
	}
	v.host = &bindings{vm: v, options: options}
	v.resolveClass = cp.Class
	v.reflectionOpen = cp.ReflectionOpen
	if options.Class != nil {
		v.resolveClass = func(name string) ([]byte, error) {
			data, e := cp.Class(name)
			if e != nil || data != nil {
				return data, e
			}
			return options.Class(name)
		}
	}
	api := &biomeProbeBindings{host: v.host}
	v.host.extra = api
	env = &Environment{vm: v, api: api, classpath: cp}
	if err = env.bootstrapMixins(ctx, cp.Archives(), options); err != nil {
		return nil, err
	}
	return env, nil
}
