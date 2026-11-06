package modjar

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	"github.com/Viking602/YoloMCL/cubiomes"
)

// Options supplies native data/API bindings. Config is restricted to named
// TOML resources requested by Forge's registerConfig API. Nil uses defaults.
type Options struct {
	Symbols       *Symbols
	fullLifecycle bool
	Minecraft     string
	// JavaMajor is the configured game's class-file compatibility, not a host JVM.
	// Zero leaves version-dependent reflection unavailable.
	JavaMajor     int
	Loader, ModID string
	LoaderVersion string
	Mods          map[string]string
	Side          string
	// Language supplies Java's default locale language for locale-sensitive
	// calls. Empty does not authorize guessing a process-global default.
	Language string
	// RegistryOrder is supplied by a versioned native loader profile. Entries
	// are the registry identities used by ResourceKey bindings. A session with
	// registry listeners must not infer this sequence from referenced classes.
	RegistryOrder []string
	// PartialRegistries fires only RegistryOrder's events. Registers attached
	// to any other registry stay unpopulated, so reading them fails closed.
	PartialRegistries bool
	// Class resolves named dependencies from the installed JAR catalogue.
	Class  func(string) ([]byte, error)
	Config func(string) ([]byte, error)
	// Resources returns every matching resource in the selected loader classpath.
	// Service configuration is data; providers still execute in the bounded VM.
	Resources func(string) ([][]byte, error)
	// ReflectionOpen resolves module access for classes in the supplied catalogue.
	// Without it, reflective access is limited to this archive's known class space.
	ReflectionOpen func(string) (bool, error)
	// MixinArchives supplies the selected loader classpath so declared mixins
	// are woven into classes the registration program loads.
	MixinArchives func() ([]Archive, error)
}

type Registration struct {
	ID         string
	Dimension  cubiomes.Dimension
	Weight     int
	Parameters []cubiomes.TerraBlenderParameter
	// Deferred marks Regions.register inside a FMLCommonSetupEvent.enqueueWork
	// task. The DeferredWorkQueue runs after every mod's parallel setup
	// listener, so direct registrations from all mods come first.
	Deferred bool `json:",omitempty"`
}

// Result is pure data. Compiled engine sampling never executes bytecode.
type Result struct {
	Regions       []Registration
	ConfigNames   []string
	ProgramSHA256 string
	Steps         int
	Entrypoints   []string `json:",omitempty"`
	// SkippedWork lists deferred setup tasks that needed unmodelled gameplay
	// content registries (see contentRegistryGap).
	SkippedWork []string `json:",omitempty"`
	// ConstructorFaults lists mod constructors that hit an interpreter gap or
	// threw. The installed game constructed them, so annotated subscribers and
	// setup still run; regions they register are kept.
	ConstructorFaults []string `json:",omitempty"`
	// SurfaceRules are traced SurfaceRuleManager.addSurfaceRules calls.
	SurfaceRules      []SurfaceRuleSet `json:",omitempty"`
	SurfaceRulesError string           `json:",omitempty"`
}

// ReadTerraBlender interprets framework registration entrypoints. It discovers
// loader metadata and config/setup listeners through their framework calls,
// executes their control flow, and captures Region.addBiomes through native API
// bindings. Unknown calls/opcodes return an error, never fabricated parameters.
// This is a registration interpreter, not an arbitrary Java/mod-loader runtime.
func ReadTerraBlender(ctx context.Context, z *zip.Reader, options Options) (result Result, err error) {
	defer func() {
		if p := recover(); p != nil {
			if f, ok := p.(fault); ok {
				err = fmt.Errorf("mod registration: %s", f.message)
			} else if e, ok := p.(javaException); ok {
				err = fmt.Errorf("mod registration threw: %w", e)
			} else {
				err = fmt.Errorf("invalid registration program: %v", p)
			}
			result = Result{}
		}
		if ctx.Err() != nil {
			err = ctx.Err()
			result = Result{}
		}
	}()
	if err = ctx.Err(); err != nil {
		return
	}
	if strings.TrimSpace(options.Minecraft) == "" {
		return result, fmt.Errorf("registration requires a Minecraft version")
	}
	v, names, err := archiveMachine(ctx, z, options)
	if err != nil {
		return result, err
	}
	h := v.host
	h.extra = &biomeProbeBindings{host: h}
	plan := discoverEntrypoints(v, z, names, options)
	if options.MixinArchives != nil {
		archives, e := options.MixinArchives()
		if e != nil {
			return result, e
		}
		env := &Environment{vm: v, weaveOnly: true}
		if e = env.bootstrapMixins(ctx, archives, options); e != nil {
			return result, e
		}
		if options.Loader == "forge" || options.Loader == "neoforge" {
			for _, entry := range dependencyConstructors(ctx, v, z, archives, options) {
				runDependency(v, entry)
			}
		}
	}
	if len(plan.before)+len(plan.setup)+len(plan.after)+len(plan.subscribers) == 0 {
		return result, fmt.Errorf("no supported registration entrypoint")
	}
	for _, entry := range plan.before {
		if msg := runConstructor(v, entry); msg != "" {
			result.ConstructorFaults = append(result.ConstructorFaults, entry.ref.owner+": "+msg)
			continue
		}
		result.Entrypoints = append(result.Entrypoints, entry.ref.owner+"."+entry.ref.name)
	}
	h.listeners = append(h.listeners, plan.subscribers...)
	// Forge/NeoForge TerraBlender constructors publish CONFIG before any
	// dependent mod's setup, so EndBiomeRegistry.<clinit> can read it there.
	ensureTerraBlenderConfig(v, h)
	for _, spec := range h.configs {
		config := v.alloc("net/minecraftforge/fml/config/ModConfig", spec)
		event := v.alloc("net/minecraftforge/fml/event/config/ModConfigEvent$Loading", config)
		h.fireEvent("Lnet/minecraftforge/fml/event/config/ModConfigEvent$Loading;", event)
	}
	for _, entry := range plan.setup {
		entry.run(v)
		result.Entrypoints = append(result.Entrypoints, entry.ref.owner+"."+entry.ref.name)
	}
	h.tolerateListeners = true
	h.fireEvent("Lnet/minecraftforge/fml/event/lifecycle/FMLCommonSetupEvent;", v.alloc("net/minecraftforge/fml/event/lifecycle/FMLCommonSetupEvent", nil))
	h.tolerateListeners = false
	result.ConstructorFaults = append(result.ConstructorFaults, h.lifecycleFaults...)
	h.drainWork()
	result.SkippedWork = h.skippedWork
	// TerraBlenderFabric.onInitialize publishes CONFIG before API callbacks.
	ensureTerraBlenderConfig(v, h)
	for _, entry := range plan.after {
		entry.run(v)
		result.Entrypoints = append(result.Entrypoints, entry.ref.owner+"."+entry.ref.name)
	}
	seen := map[string]bool{}
	parameterCount := 0
	for _, r := range h.regions {
		o := r.instance
		if seen[r.id] {
			fail("duplicate region registration %s", r.id)
		}
		seen[r.id] = true
		weight := v.method(reference{"terrablender/api/Region", "getWeight", "()I"}, o, true, nil)
		registration := Registration{ID: r.id, Dimension: r.dimension, Weight: int(integer(weight)), Deferred: r.deferred}
		mapper := &consumer{accept: func(x any) {
			p, ok := x.(cubiomes.TerraBlenderParameter)
			if !ok {
				if o, isObj := x.(*object); isObj && o != nil {
					if pair, isPair := objectData(o).(javaPair); isPair {
						space, okSpace := objectData(pair.left).([7][2]int64)
						biome, okBiome := objectData(pair.right).(string)
						if okSpace && okBiome {
							p = cubiomes.TerraBlenderParameter{Biome: biome, Space: space}
							ok = true
						}
					}
				}
			}
			if !ok {
				fail("invalid biome mapper value")
			}
			parameterCount++
			if parameterCount > 262144 {
				fail("region parameter limit")
			}
			registration.Parameters = append(registration.Parameters, p)
		}}
		v.method(reference{o.class, "addBiomes", "(Lnet/minecraft/core/Registry;Ljava/util/function/Consumer;)V"}, o, true, []any{v.alloc("net/minecraft/core/Registry", worldBiomeRegistry{}), mapper})
		result.Regions = append(result.Regions, registration)
	}
	// Hash only classes that participated in discovery/execution, with names
	// and explicit lengths to avoid ambiguous concatenations.
	hash := sha256.New()
	loaded := make([]string, 0, len(v.classes))
	for name := range v.classes {
		loaded = append(loaded, name)
	}
	slices.Sort(loaded)
	for _, name := range loaded {
		fmt.Fprintf(hash, "%d:%s:%d:", len(name), name, len(v.raw[name]))
		hash.Write(v.raw[name])
	}
	result.ProgramSHA256 = hex.EncodeToString(hash.Sum(nil))
	result.ConfigNames = slices.Clone(h.configNames)
	result.Steps = v.steps
	return
}

// Walk instruction boundaries when looking for registration API references;
// matching arbitrary bytes inside an operand would invent entry points.
func calls(c *class, m method, owner, name string) bool {
	for _, ref := range methodReferences(c, m) {
		ref = frameworkRef(ref)
		if ref.owner == owner && ref.name == name {
			return true
		}
	}
	return false
}

func reachesRegions(v *machine, ref reference, seen map[reference]bool) bool {
	if ref.owner == "terrablender/api/Regions" && ref.name == "register" {
		return true
	}
	if seen[ref] || !v.fetchClass(ref.owner) {
		return false
	}
	if len(seen) >= 2048 {
		fail("registration call-graph budget exceeded")
	}
	seen[ref] = true
	c := v.load(ref.owner)
	for _, m := range c.methods {
		if m.name == ref.name && m.desc == ref.desc {
			for _, child := range methodReferences(c, m) {
				if reachesRegions(v, child, seen) {
					return true
				}
			}
		}
	}
	return false
}

func methodReferences(c *class, m method) (refs []reference) {
	return methodReferencesChecked(c, m, nil)
}

func methodReferencesChecked(c *class, m method, visit func(byte, int)) (refs []reference) {
	r := reader{b: m.code}
	for r.p < len(r.b) {
		op := r.u1()
		if visit != nil {
			visit(byte(op), r.p)
		}
		switch {
		case op >= 182 && op <= 185:
			ref := c.ref(r.u2())
			if op == 185 {
				r.take(2)
			}
			refs = append(refs, ref)
		case op == 186:
			dynamic := c.cp(r.u2(), 18)
			r.take(2)
			if dynamic.a >= len(c.boots) {
				fail("invalid bootstrap index")
			}
			for _, arg := range c.boots[dynamic.a].args {
				if arg <= 0 || arg >= len(c.pool) {
					fail("invalid bootstrap argument")
				}
				if h := c.pool[arg]; h.tag == 15 && h.a >= 5 && h.a <= 9 {
					refs = append(refs, c.ref(h.b))
				}
			}
		case op == 170:
			for r.p%4 != 0 {
				r.u1()
			}
			r.u4()
			lo, hi := int64(int32(r.u4())), int64(int32(r.u4()))
			if hi < lo || hi-lo > 16384 {
				fail("invalid switch")
			}
			r.take(int(hi-lo+1) * 4)
		case op == 171:
			for r.p%4 != 0 {
				r.u1()
			}
			r.u4()
			n := int32(r.u4())
			if n < 0 || n > 16384 {
				fail("invalid switch")
			}
			r.take(int(n) * 8)
		case op == 196:
			next := r.u1()
			r.take(2)
			if next == 132 {
				r.take(2)
			}
		case op == 16 || op == 18 || op >= 21 && op <= 25 || op >= 54 && op <= 58 || op == 169 || op == 188:
			r.take(1)
		case op == 17 || op == 19 || op == 20 || op == 132 || op >= 153 && op <= 168 || op >= 178 && op <= 181 || op == 187 || op == 189 || op == 192 || op == 193 || op == 198 || op == 199:
			r.take(2)
		case op == 197:
			r.take(3)
		case op == 200 || op == 201:
			r.take(4)
		case op > 201:
			fail("invalid instruction")
		}
	}
	return refs
}

func ensureTerraBlenderConfig(v *machine, h *bindings) {
	cfgRef := reference{"terrablender/core/TerraBlender", "CONFIG", "Lterrablender/config/TerraBlenderConfig;"}
	if x, ok := v.host.static(cfgRef); ok && x != nil {
		return
	}
	if v.statics["terrablender/core/TerraBlender"] != nil {
		if x := v.statics["terrablender/core/TerraBlender"][fieldKey(cfgRef)]; x != nil {
			return
		}
	}
	if !v.fetchClass("terrablender/config/TerraBlenderConfig") {
		return
	}
	path := h.newPath(javaPath{p: "config/terrablender.toml"})
	cfg := v.alloc("terrablender/config/TerraBlenderConfig", nil)
	v.method(reference{"terrablender/config/TerraBlenderConfig", "<init>", "(Ljava/nio/file/Path;)V"}, cfg, false, []any{path})
	v.method(reference{"terrablender/core/TerraBlender", "setConfig", "(Lterrablender/config/TerraBlenderConfig;)V"}, nil, false, []any{cfg})
}

// runDependency constructs a required library mod. A library whose own
// setup leaves the interpreter's modelled API does not invalidate the
// dependent mod; it surfaces only if the dependent reads that state.
func runDependency(v *machine, entry registrationEntry) {
	defer func() {
		if p := recover(); p != nil {
			switch p.(type) {
			case fault, javaException:
			default:
				panic(p)
			}
		}
	}()
	entry.run(v)
}

// runConstructor runs one mod constructor. A Java exception or unmodelled
// API inside it is reported rather than aborting the lifecycle (the installed
// game did construct the mod); budget and cancellation faults still abort.
func runConstructor(v *machine, entry registrationEntry) string {
	return tolerate(func() { entry.run(v) })
}

// tolerate runs f, reporting a Java exception or unmodelled API instead of
// propagating it. Budget and cancellation faults still abort.
func tolerate(f func()) (msg string) {
	defer func() {
		p := recover()
		if p == nil {
			return
		}
		switch e := p.(type) {
		case javaException:
			msg = e.Error()
		case fault:
			if strings.Contains(e.message, "budget") || strings.Contains(e.message, "cancel") || strings.Contains(e.message, "deadline") {
				panic(p)
			}
			msg = e.message
		default:
			panic(p)
		}
	}()
	f()
	return ""
}
