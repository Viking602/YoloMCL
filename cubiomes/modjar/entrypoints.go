package modjar

import (
	"archive/zip"
	"bytes"
	"strings"
)

type registrationEntry struct {
	ref                   reference
	instance, constructor bool
	field                 *reference
}

type registrationPlan struct {
	pre                  []registrationEntry
	before, setup, after []registrationEntry
	subscribers          []eventListener
}

func (e registrationEntry) run(v *machine) {
	var receiver any
	if e.field != nil {
		receiver = v.static(*e.field)
	} else if e.instance || e.constructor {
		v.initialize(e.ref.owner)
		receiver = v.alloc(e.ref.owner, nil)
		if !e.constructor {
			v.method(reference{e.ref.owner, "<init>", "()V"}, receiver, false, nil)
		}
	}
	var args []any
	types, _ := descriptor(e.ref.desc)
	for _, name := range types {
		name = frameworkName(name)
		switch name {
		case "Lnet/minecraftforge/eventbus/api/IEventBus;",
			"Lnet/minecraftforge/fml/ModContainer;",
			"Lnet/minecraftforge/fml/javafmlmod/FMLJavaModLoadingContext;",
			"Lnet/minecraftforge/fml/event/lifecycle/FMLCommonSetupEvent;":
			args = append(args, v.alloc(strings.TrimSuffix(name[1:], ";"), nil))
		case "Lorg/quiltmc/loader/api/ModContainer;":
			args = append(args, v.host.symbol("container:"+v.host.options.ModID, "org/quiltmc/loader/api/ModContainer", v.host.options.ModID))
		default:
			fail("unbound loader entrypoint argument %s in %s.%s", name, e.ref.owner, e.ref.name)
		}
	}
	v.method(e.ref, receiver, e.instance || e.field != nil, args)
}

func discoverEntrypoints(v *machine, z *zip.Reader, names []string, options Options) registrationPlan {
	// A caller without loader metadata requests the original static-module API.
	// Keep that source-verified contract distinct from full loader discovery.
	if options.Loader == "" {
		return discoverStaticModules(v, names)
	}
	var plan registrationPlan
	if options.Loader == "fabric" || options.Loader == "quilt" {
		return discoverJSONEntrypoints(v, z, options)
	}
	profile, err := SelectLoader(options.Loader, options.Minecraft, options.LoaderVersion)
	if err != nil {
		fail("entrypoint loader: %v", err)
	}
	// Resolve annotated constructors and subscribers from class metadata. API
	// package relocation is shared by Forge/NeoForge and independent of MC patch.
	for _, name := range names {
		data := v.inspectClass(name)
		if !bytes.Contains(data, []byte("/fml/common/")) {
			continue
		}
		c := v.load(name)
		if a, ok := profile.annotation(c.annotations, "Lnet/minecraftforge/fml/common/Mod;"); ok {
			id, _ := a.values["value"].(string)
			if options.ModID != "" && id != options.ModID {
				continue
			}
			if !annotationSide(a, "dist", options.Side) {
				continue
			}
			var constructors []registrationEntry
			for _, m := range c.methods {
				if m.name == "<init>" && m.access&1 != 0 {
					constructors = append(constructors, registrationEntry{ref: reference{c.name, m.name, m.desc}, constructor: true})
				}
			}
			if len(constructors) != 1 {
				fail("ambiguous mod constructor for %s", id)
			}
			plan.before = append(plan.before, constructors[0])
		}
		if a, ok := profile.annotation(c.annotations, "Lnet/minecraftforge/fml/common/Mod$EventBusSubscriber;"); ok {
			id, _ := a.values["modid"].(string)
			if id != "" && options.ModID != "" && id != options.ModID {
				continue
			}
			if !annotationSide(a, "value", options.Side) {
				continue
			}
			bus, _ := a.values["bus"].(enumValue)
			if bus.name != "MOD" {
				continue
			}
			for _, m := range c.methods {
				annotation, ok := findAnnotation(m.annotations, "Lnet/minecraftforge/eventbus/api/SubscribeEvent;")
				if !ok {
					continue
				}
				args, _ := descriptor(m.desc)
				if m.access&8 != 0 && len(args) == 1 {
					priority, _ := annotation.values["priority"].(enumValue)
					plan.subscribers = append(plan.subscribers, eventListener{callback: &lambda{target: reference{c.name, m.name, m.desc}, kind: 6, instantiated: m.desc}, event: frameworkName(args[0]), priority: eventPriority(priority.name), modID: options.ModID})
				}
			}
		}
	}
	return plan
}

func discoverStaticModules(v *machine, names []string) registrationPlan {
	var plan registrationPlan
	// Explicit standalone module inspection has no installed loader lifecycle.
	// Installed instances never enter this path as a missing-entrypoint fallback.
	for _, name := range names {
		data := v.inspectClass(name)
		if !bytes.Contains(data, []byte("/eventbus/api/IEventBus")) && !bytes.Contains(data, []byte("/bus/api/IEventBus")) && !bytes.Contains(data, []byte("/fml/ModLoadingContext")) {
			continue
		}
		c := v.load(name)
		for _, m := range c.methods {
			if m.access&8 == 0 || m.desc != "()V" || m.name == "<clinit>" {
				continue
			}
			ref := reference{name, m.name, m.desc}
			if calls(c, m, "net/minecraftforge/fml/ModLoadingContext", "registerConfig") {
				plan.before = append(plan.before, registrationEntry{ref: ref})
			} else if calls(c, m, "net/minecraftforge/eventbus/api/IEventBus", "addListener") && reachesRegions(v, ref, map[reference]bool{}) {
				plan.setup = append(plan.setup, registrationEntry{ref: ref})
			}
		}
	}
	return plan
}
