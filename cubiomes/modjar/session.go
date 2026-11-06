package modjar

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"
)

// Module supplies an installed archive and loader ordering constraints. The
// caller retains ownership of the archive until the returned environment closes.
type Module struct {
	ID            string
	Archive       *zip.Reader
	Before, After []string
}

// Environment is one shared registration session. Resource identifiers come
// from executed registry calls, never filenames or Java class names.
type Environment struct {
	vm           *machine
	noiseChecked bool
	noiseReason  string
	api          *biomeProbeBindings
	Order        []string
	classpath    *Classpath
	MixinEffects []MixinEffect
	mixinPlugins map[string]*object
	// opaqueNativePlugins are mixin config plugins whose implementation is
	// native-protected (clinit/init throw UnsatisfiedLinkError). Their mixins
	// cannot be decided or applied; worldgen fails closed only if such an
	// unapplied effect targets a reached worldgen class.
	opaqueNativePlugins map[string]bool
	// weaveOnly registers class weaving without running biome-source hooks.
	weaveOnly bool
}

// Close releases nested archive streams. Installed root archives remain owned
// by the caller. Registry identifiers already captured remain readable.
func (e *Environment) Close() error {
	if e == nil || e.classpath == nil {
		return nil
	}
	return e.classpath.Close()
}

// OrderModules performs a stable topological sort. Constraints on absent mods
// are optional ordering relationships; required dependency validation belongs
// to the loader's metadata resolver. Cycles and duplicate identities are errors.
func OrderModules(modules []Module) ([]Module, error) {
	index := make(map[string]int, len(modules))
	for i, m := range modules {
		if m.ID == "" || m.Archive == nil {
			return nil, fmt.Errorf("invalid module")
		}
		if _, exists := index[m.ID]; exists {
			return nil, fmt.Errorf("duplicate module %s", m.ID)
		}
		index[m.ID] = i
	}
	edges := make([]map[int]bool, len(modules))
	in := make([]int, len(modules))
	add := func(a, b string) {
		i, ai := index[a]
		j, bj := index[b]
		if !ai || !bj {
			return
		}
		if edges[i] == nil {
			edges[i] = map[int]bool{}
		}
		if !edges[i][j] {
			edges[i][j] = true
			in[j]++
		}
	}
	for _, m := range modules {
		for _, before := range m.Before {
			add(m.ID, before)
		}
		for _, after := range m.After {
			add(after, m.ID)
		}
	}
	var ordered []Module
	for len(ordered) < len(modules) {
		ready := -1
		for i, n := range in {
			if n == 0 {
				ready = i
				break
			}
		}
		if ready < 0 {
			return nil, fmt.Errorf("module loading order contains a cycle")
		}
		in[ready] = -1
		ordered = append(ordered, modules[ready])
		for next := range edges[ready] {
			in[next]--
		}
	}
	return ordered, nil
}

// Load executes loader entrypoints against one classloader, static heap and
// registry. Unknown APIs abort the session atomically. This bounded host is not
// a complete replacement for a game loader or an arbitrary JVM.
func Load(ctx context.Context, modules []Module, options Options) (env *Environment, err error) {
	return load(ctx, modules, options, false)
}

// LoadRegistries stops before common setup. Its output is usable only by a
// compiler that proves its inputs do not depend on mutable initialization
// state; it is not evidence of complete mod initialization.
func LoadRegistries(ctx context.Context, modules []Module, options Options) (*Environment, error) {
	return load(ctx, modules, options, true)
}

func load(ctx context.Context, modules []Module, options Options, registriesOnly bool) (env *Environment, err error) {
	options.fullLifecycle = true
	var ownedClasspath *Classpath
	defer func() {
		if p := recover(); p != nil {
			env = nil
			err = fmt.Errorf("worldgen initialization: %v", p)
		}
		if ctx.Err() != nil {
			env = nil
			err = ctx.Err()
		}
		if err != nil && ownedClasspath != nil {
			ownedClasspath.Close()
		}
	}()
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if options.Minecraft == "" {
		return nil, fmt.Errorf("registration requires a Minecraft version")
	}
	ordered, err := OrderModules(modules)
	if err != nil {
		return nil, err
	}
	if len(ordered) == 0 {
		return nil, fmt.Errorf("no worldgen modules")
	}
	switch options.Loader {
	case "forge", "neoforge", "fabric", "quilt":
	default:
		return nil, fmt.Errorf("unbound loader %q", options.Loader)
	}
	v := newMachine(ctx, nil)
	v.host = &bindings{vm: v, options: options}
	var roots []Archive
	seenArchives := map[*zip.Reader]bool{}
	for _, m := range ordered {
		if !seenArchives[m.Archive] {
			roots = append(roots, Archive{Name: m.ID, Reader: m.Archive})
			seenArchives[m.Archive] = true
		}
	}
	classpath, e := NewClasspath(ctx, roots, options.Loader)
	if e != nil {
		return nil, e
	}
	ownedClasspath = classpath
	v.reflectionOpen = classpath.ReflectionOpen
	if v.host.options.Resources == nil {
		v.host.options.Resources = classpath.Resources
	}
	locals := make([]*machine, len(ordered))
	classNames := make([][]string, len(ordered))
	plans := make([]registrationPlan, len(ordered))
	for i, m := range ordered {
		o := options
		o.ModID = m.ID
		local, names, e := archiveMachine(ctx, m.Archive, o)
		if e != nil {
			return nil, e
		}
		locals[i], classNames[i] = local, names
	}
	v.resolveClass = func(name string) ([]byte, error) {
		var found []byte
		reads := []func(string) ([]byte, error){classpath.Class}
		if options.Class != nil {
			reads = append(reads, options.Class)
		}
		for _, read := range reads {
			b, e := read(name)
			if e != nil {
				return nil, e
			}
			if found != nil && b != nil && !bytes.Equal(found, b) {
				return nil, fmt.Errorf("conflicting class %s", name)
			}
			if b != nil {
				found = b
			}
		}
		return found, nil
	}
	for i, m := range ordered {
		o := options
		o.ModID = m.ID
		v.inspectSource = locals[i].inspectSource
		plans[i] = discoverEntrypoints(v, m.Archive, classNames[i], o)
	}
	h := v.host
	for _, m := range ordered {
		h.moduleOrder = append(h.moduleOrder, m.ID)
	}
	api := &biomeProbeBindings{host: h, registries: map[string]map[string]any{}, attached: map[*object]bool{}}
	h.extra = api
	env = &Environment{vm: v, api: api, classpath: classpath}
	if err = env.bootstrapMixins(ctx, classpath.Archives(), options); err != nil {
		return nil, err
	}
	for i, m := range ordered {
		h.options.ModID = m.ID
		for _, entry := range plans[i].pre {
			entry.run(v)
		}
	}
	for i, m := range ordered {
		h.options.ModID = m.ID
		env.Order = append(env.Order, m.ID)
		p := plans[i]
		for _, entry := range p.before {
			entry.run(v)
		}
		h.listeners = append(h.listeners, p.subscribers...)
	}
	api.fireRegistryEvents(options.RegistryOrder)
	if registriesOnly {
		return env, nil
	}
	for _, spec := range h.configs {
		config := v.alloc("net/minecraftforge/fml/config/ModConfig", spec)
		event := v.alloc("net/minecraftforge/fml/event/config/ModConfigEvent$Loading", config)
		h.fireEventForMod("Lnet/minecraftforge/fml/event/config/ModConfigEvent$Loading;", event, h.configOwners[spec])
	}
	for i, m := range ordered {
		h.options.ModID = m.ID
		for _, entry := range plans[i].setup {
			entry.run(v)
		}
	}
	h.fireEvent("Lnet/minecraftforge/fml/event/lifecycle/FMLCommonSetupEvent;", v.alloc("net/minecraftforge/fml/event/lifecycle/FMLCommonSetupEvent", nil))
	h.drainWork()
	for i, m := range ordered {
		h.options.ModID = m.ID
		for _, entry := range plans[i].after {
			entry.run(v)
		}
	}
	return env, nil
}

const biomeSourceRegistry = "net/minecraft/core/registries/Registries.BIOME_SOURCE"

func (e *Environment) BiomeSourceIDs() []string {
	var ids []string
	for id := range e.api.registries[biomeSourceRegistry] {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func (h *biomeProbeBindings) register(registry, id string, value any) any {
	if value == nil {
		fail("null registry entry %s/%s", registry, id)
	}
	if h.registries == nil {
		h.registries = map[string]map[string]any{}
	}
	entries := h.registries[registry]
	if entries == nil {
		entries = map[string]any{}
		h.registries[registry] = entries
	}
	if _, exists := entries[id]; exists {
		fail("duplicate registry key %s/%s", registry, id)
	}
	if len(entries) >= 32768 {
		fail("registry entry budget exceeded")
	}
	entries[id] = value
	return value
}

func (h *biomeProbeBindings) deferValue(entry deferredValue) {
	if len(h.deferred) >= 32768 {
		fail("deferred registration budget exceeded")
	}
	for _, other := range h.deferred {
		h.host.vm.tick()
		if other.registry == entry.registry && other.id == entry.id {
			fail("duplicate deferred registry key")
		}
	}
	h.deferred = append(h.deferred, entry)
}

func (h *biomeProbeBindings) registerDeferred(owner *object, registry string) {
	for _, entry := range h.deferred {
		if entry.owner != owner || entry.registry != registry {
			continue
		}
		func() {
			defer func() {
				if p := recover(); p != nil {
					fail("registry %s/%s: %v", entry.registry, entry.id, p)
				}
			}()
			h.register(entry.registry, entry.id, h.host.vm.callSupplier(entry.supplier, entry.args))
		}()
	}
}

func (h *biomeProbeBindings) fireRegistryEvents(order []string) {
	needed := false
	for _, listener := range h.host.listeners {
		needed = needed || strings.HasSuffix(listener.event, "/registries/RegisterEvent;")
	}
	if needed && len(order) == 0 {
		fail("registry event sequence requires a native loader profile")
	}
	seen := map[string]bool{}
	for _, key := range order {
		if key == "" || seen[key] {
			fail("invalid registry event sequence")
		}
		seen[key] = true
	}
	for owner := range h.attached {
		if h.host.options.PartialRegistries {
			break
		}
		if !seen[objectData(owner).(deferredRegistry).registry] {
			fail("registry event sequence omits an attached register")
		}
	}
	for _, key := range order {
		registry := h.host.registryKey(key)
		class := "net/minecraftforge/registries/RegisterEvent"
		if h.host.options.Loader == "neoforge" {
			class = "net/neoforged/neoforge/registries/RegisterEvent"
		}
		h.host.fireEvent("L"+class+";", h.host.vm.alloc(class, registry))
	}
}
