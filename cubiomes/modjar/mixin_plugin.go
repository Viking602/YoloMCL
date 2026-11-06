package modjar

import "strings"

// Plugin decisions execute in the same bounded interpreter and classpath as
// registrations. A host capability error is never translated into a Java
// exception: a catch(Throwable) must not accidentally enable a failed plugin.
//
// An UnsatisfiedLinkError from plugin <clinit>/<init> marks the plugin as
// opaque-native: shouldApplyMixin cannot run, so those mixins stay unapplied.
// Registration continues; biome program construction fails closed only when an
// unapplied opaque-native effect targets a reached worldgen class.
func (e *Environment) mixinEnabled(effect MixinEffect) bool {
	if effect.Plugin == "" {
		return true
	}
	v := e.vm
	key := effect.Config + ":" + effect.Plugin
	if e.opaqueNativePlugins[key] {
		return false
	}
	plugin := e.mixinPlugins[key]
	if plugin == nil {
		name := strings.ReplaceAll(effect.Plugin, ".", "/")
		if err := e.constructMixinPlugin(key, name, effect.Package); err != nil {
			if e.opaqueNativePlugins[key] {
				return false
			}
			panic(err)
		}
		if e.opaqueNativePlugins[key] {
			return false
		}
		plugin = e.mixinPlugins[key]
	}
	result := v.method(reference{plugin.class, "shouldApplyMixin", "(Ljava/lang/String;Ljava/lang/String;)Z"}, plugin, true, []any{strings.ReplaceAll(effect.Target, "/", "."), strings.ReplaceAll(effect.Class, "/", ".")})
	return integer(result) != 0
}

func (e *Environment) opaqueNativePlugin(effect MixinEffect) bool {
	if effect.Plugin == "" {
		return false
	}
	return e.opaqueNativePlugins[effect.Config+":"+effect.Plugin]
}

func (e *Environment) constructMixinPlugin(key, name, pkg string) (err any) {
	v := e.vm
	defer func() {
		if p := recover(); p != nil {
			if je, ok := p.(javaException); ok && je.class == "java/lang/UnsatisfiedLinkError" {
				if e.opaqueNativePlugins == nil {
					e.opaqueNativePlugins = map[string]bool{}
				}
				e.opaqueNativePlugins[key] = true
				err = nil
				return
			}
			err = p
		}
	}()
	v.initialize(name)
	plugin := v.alloc(name, nil)
	v.method(reference{name, "<init>", "()V"}, plugin, false, nil)
	v.method(reference{name, "onLoad", "(Ljava/lang/String;)V"}, plugin, true, []any{pkg})
	if extra := v.method(reference{name, "getMixins", "()Ljava/util/List;"}, plugin, true, nil); extra != nil {
		fail("dynamic mixin list requires resolution: %s", name)
	}
	// ASM callbacks can rewrite arbitrary target code. Accept only a proved
	// empty implementation until those transformations have an execution path.
	for _, method := range []string{"preApply", "postApply"} {
		owner := name
		found := false
		for depth := 0; depth < 64 && v.fetchClass(owner); depth++ {
			c := v.load(owner)
			for _, m := range c.methods {
				if m.name == method && m.desc == "(Ljava/lang/String;Lorg/objectweb/asm/tree/ClassNode;Ljava/lang/String;Lorg/spongepowered/asm/mixin/extensibility/IMixinInfo;)V" {
					if len(m.code) != 1 || m.code[0] != 177 {
						// The ASM rewrite has no execution path. Treat the plugin
						// like an opaque-native one: its mixins stay unapplied and
						// program construction fails closed if one targets a
						// reached worldgen class.
						if e.opaqueNativePlugins == nil {
							e.opaqueNativePlugins = map[string]bool{}
						}
						e.opaqueNativePlugins[key] = true
						return nil
					}
					found = true
					break
				}
			}
			if found {
				break
			}
			owner = c.super
		}
		if !found {
			fail("missing mixin plugin callback %s.%s", name, method)
		}
	}
	if e.mixinPlugins == nil {
		e.mixinPlugins = map[string]*object{}
	}
	e.mixinPlugins[key] = plugin
	return nil
}
