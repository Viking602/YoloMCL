package modjar

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func sessionState() []byte {
	c := &fixtureClass{}
	c.field("value", "I", 9)
	return c.bytes("example/State", "java/lang/Object")
}

func sessionDigit(c *fixtureClass, digit byte) []byte {
	b := c.instruction(178, "example/State", "value", "I")
	b = append(b, 16, 10, 104, 16, digit, 96)
	return append(b, c.instruction(179, "example/State", "value", "I")...)
}

func TestSessionSharesStateHonorsOrderAndDiscardsFailures(t *testing.T) {
	module := func(id string, digit byte, bad bool) Module {
		c := &fixtureClass{}
		c.modAnnotation("Lnet/neoforged/fml/common/Mod;", id)
		code := []byte{42}
		code = append(code, c.instruction(183, "java/lang/Object", "<init>", "()V")...)
		code = append(code, sessionDigit(c, digit)...)
		code = append(code, c.instruction(178, "net/minecraft/core/registries/BuiltInRegistries", "BIOME_SOURCE", "Lnet/minecraft/core/Registry;")...)
		code = append(code, c.string(id+":source")...)
		code = append(code, c.instruction(184, "net/minecraft/resources/ResourceLocation", "parse", "(Ljava/lang/String;)Lnet/minecraft/resources/ResourceLocation;")...)
		code = append(code, c.instruction(178, "com/mojang/serialization/Codec", "STRING", "Lcom/mojang/serialization/Codec;")...)
		code = append(code, c.instruction(184, "net/minecraft/core/Registry", "register", "(Lnet/minecraft/core/Registry;Lnet/minecraft/resources/ResourceLocation;Ljava/lang/Object;)Ljava/lang/Object;")...)
		code = append(code, 87)
		if bad {
			code = append(code, c.instruction(184, "java/lang/Runtime", "getRuntime", "()Ljava/lang/Runtime;")...)
			code = append(code, 87)
		}
		c.method("<init>", "()V", 1, append(code, 177))
		name := "example/" + id
		return Module{ID: id, Archive: fixtureArchive(t, map[string][]byte{name + ".class": c.bytes(name, "java/lang/Object"), "example/State.class": sessionState()})}
	}
	a, b, c := module("first", 1, false), module("second", 2, false), module("third", 3, false)
	b.After = []string{"first"}
	c.After = []string{"second"}
	mods := []Module{c, b, a}
	env, err := Load(context.Background(), mods, Options{Minecraft: "1.21.1", Loader: "neoforge"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(env.Order, []string{"first", "second", "third"}) || env.vm.static(reference{"example/State", "value", "I"}) != int32(123) {
		t.Fatal("mod initialization did not share ordered static state")
	}
	if !slices.Equal(env.BiomeSourceIDs(), []string{"first:source", "second:source", "third:source"}) {
		t.Fatal(env.BiomeSourceIDs())
	}
	bad := module("bad", 4, true)
	bad.After = []string{"third"}
	env, err = Load(context.Background(), append(mods, bad), Options{Minecraft: "1.21.1", Loader: "neoforge"})
	if env != nil || err == nil || !strings.Contains(err.Error(), "forbidden registration API") {
		t.Fatalf("partial state escaped: %v %v", env, err)
	}
	a.After = []string{"third"}
	if _, err = OrderModules([]Module{a, b, c}); err == nil {
		t.Fatal("cycle accepted")
	}
	if _, err = OrderModules([]Module{a, a}); err == nil {
		t.Fatal("duplicate mod accepted")
	}
}

func TestJSONLoaderLifecycleUsesDeclaredEntryForms(t *testing.T) {
	for _, loader := range []string{"fabric", "quilt"} {
		for _, side := range []string{"CLIENT", "DEDICATED_SERVER"} {
			t.Run(loader+"/"+side, func(t *testing.T) {
				c := &fixtureClass{}
				desc := "()V"
				if loader == "quilt" {
					desc = "(Lorg/quiltmc/loader/api/ModContainer;)V"
				}
				for i, name := range []string{"pre", "main", "client", "extension", "server"} {
					d := desc
					if name == "extension" {
						d = "()V"
					}
					c.method(name, d, 9, append(sessionDigit(c, byte(i+1)), 177))
				}
				entries := map[string]any{}
				metadata := map[string]any{"schemaVersion": 1, "id": "sample", "entrypoints": entries}
				filename := "fabric.mod.json"
				if loader == "quilt" {
					filename = "quilt.mod.json"
					metadata = map[string]any{"schema_version": 1, "quilt_loader": map[string]any{"id": "sample", "entrypoints": entries}}
					entries["pre_launch"] = "example.Main::pre"
					entries["init"] = []any{map[string]string{"adapter": "default", "value": "example.Main::main"}}
					entries["client_init"] = "example.Main::client"
					entries["server_init"] = "example.Main::server"
				} else {
					entries["preLaunch"] = []string{"example.Main::pre"}
					entries["main"] = []string{"example.Main::main"}
					entries["client"] = []string{"example.Main::client"}
					entries["server"] = []string{"example.Main::server"}
				}
				entries["terrablender"] = []string{"example.Main::extension"}
				data, _ := json.Marshal(metadata)
				z := fixtureArchive(t, map[string][]byte{filename: data, "example/Main.class": c.bytes("example/Main", "java/lang/Object"), "example/State.class": sessionState()})
				env, err := Load(context.Background(), []Module{{ID: "sample", Archive: z}}, Options{Minecraft: "1.21.1", Loader: loader, Side: side})
				if err != nil {
					t.Fatal(err)
				}
				want := int32(1234)
				if side == "DEDICATED_SERVER" {
					want = 1254
				}
				if got := env.vm.static(reference{"example/State", "value", "I"}); got != want {
					t.Fatalf("lifecycle state %v, want %v", got, want)
				}
			})
		}
	}
}

func TestDefaultEntrypointRejectsAmbiguousMethods(t *testing.T) {
	c := &fixtureClass{}
	c.method("start", "()V", 9, []byte{177})
	c.method("start", "(I)V", 9, []byte{177})
	v := newMachine(context.Background(), map[string][]byte{"example/Main": c.bytes("example/Main", "java/lang/Object")})
	v.host = &bindings{vm: v}
	expectFault(t, "ambiguous", func() {
		defaultEntrypoint(v, "example.Main::start", "net/fabricmc/api/ModInitializer", "onInitialize", "()V")
	})
}

func TestDeferredRegistryUsesEventsAndAttachedObjects(t *testing.T) {
	c := &fixtureClass{}
	c.method("value", "()Ljava/lang/Object;", 9, append(c.string("registered"), 176))
	v := newMachine(context.Background(), map[string][]byte{"example/Factory": c.bytes("example/Factory", "java/lang/Object")})
	h := &bindings{vm: v, options: Options{Loader: "neoforge", ModID: "sample"}}
	v.host = h
	api := &biomeProbeBindings{host: h}
	h.extra = api
	supplier := &lambda{target: reference{"example/Factory", "value", "()Ljava/lang/Object;"}, kind: 6}
	const owner = "net/neoforged/neoforge/registries/DeferredRegister"
	const desc = "(Ljava/lang/String;Ljava/util/function/Supplier;)Lnet/neoforged/neoforge/registries/DeferredHolder;"
	one := v.alloc(owner, deferredRegistry{"sample", biomeSourceRegistry})
	two := v.alloc(owner, deferredRegistry{"sample", biomeSourceRegistry})
	h.call(reference{owner, "register", desc}, one, []any{"first", supplier})
	h.call(reference{owner, "register", desc}, two, []any{"second", supplier})
	h.call(reference{owner, "register", "(Lnet/neoforged/bus/api/IEventBus;)V"}, one, []any{v.alloc("net/neoforged/bus/api/IEventBus", nil)})
	if len(api.registries) != 0 {
		t.Fatal("declaration ran its supplier")
	}
	expectFault(t, "requires a native loader profile", func() { api.fireRegistryEvents(nil) })
	api.fireRegistryEvents([]string{biomeSourceRegistry})
	if api.registries[biomeSourceRegistry]["sample:first"] != "registered" || api.registries[biomeSourceRegistry]["sample:second"] != nil {
		t.Fatal("unattached declaration registered or attached declaration lost")
	}
	expectFault(t, "already attached", func() { h.call(reference{owner, "register", "(Lnet/neoforged/bus/api/IEventBus;)V"}, one, []any{nil}) })
}

func TestModEventBusesAndWorkRetainOwner(t *testing.T) {
	v := newMachine(context.Background(), nil)
	h := &bindings{vm: v, moduleOrder: []string{"a", "b"}}
	v.host = h
	var order []string
	for _, id := range []string{"a", "b"} {
		priority := 0
		if id == "a" {
			priority = 4
		}
		h.listeners = append(h.listeners, eventListener{event: "event", modID: id, priority: priority, native: func(any) {
			order = append(order, h.options.ModID)
			callback := &lambda{kind: 5, target: reference{"java/util/function/Consumer", "accept", "(Ljava/lang/Object;)V"}, captured: []any{&consumer{accept: func(any) { order = append(order, "work:"+h.options.ModID) }}, nil}}
			h.call(reference{"net/neoforged/fml/event/lifecycle/FMLCommonSetupEvent", "enqueueWork", "(Ljava/lang/Runnable;)Ljava/util/concurrent/CompletableFuture;"}, nil, []any{callback})
		}})
	}
	h.options.ModID = "outside"
	h.fireEvent("event", nil)
	h.drainWork()
	if !slices.Equal(order, []string{"a", "b", "work:a", "work:b"}) || h.options.ModID != "outside" {
		t.Fatal(order, h.options.ModID)
	}
	order = nil
	h.fireEventForMod("event", nil, "b")
	if !slices.Equal(order, []string{"b"}) {
		t.Fatal("mod-local config event leaked", order)
	}
}

func TestSessionResolvesDeclaredNestedLibraries(t *testing.T) {
	dependency := &fixtureClass{}
	dependency.method("start", "()V", 9, append(sessionDigit(dependency, 7), 177))
	main := &fixtureClass{}
	main.method("start", "()V", 9, append(main.instruction(184, "example/Dependency", "start", "()V"), 177))
	z := fixtureArchive(t, map[string][]byte{
		"fabric.mod.json":     []byte(`{"schemaVersion":1,"id":"sample","entrypoints":{"main":["example.Main::start"]},"jars":[{"file":"lib.jar"}]}`),
		"example/Main.class":  main.bytes("example/Main", "java/lang/Object"),
		"example/State.class": sessionState(),
		"lib.jar":             nestedArchiveBytes(t, dependency.bytes("example/Dependency", "java/lang/Object"), false),
	})
	env, err := Load(context.Background(), []Module{{ID: "sample", Archive: z}}, Options{Minecraft: "1.21.1", Loader: "fabric"})
	if err != nil {
		t.Fatal(err)
	}
	if got := env.vm.static(reference{"example/State", "value", "I"}); got != int32(7) {
		t.Fatalf("nested dependency was not initialized: %v", got)
	}
}

func TestSpecializedDeferredItemsAreNotSilentlySkipped(t *testing.T) {
	c := &fixtureClass{}
	// An identity Function isolates argument forwarding and event timing from
	// the game's Item implementation, which is not supplied by this fixture.
	c.method("apply", "(Ljava/lang/Object;)Ljava/lang/Object;", 9, []byte{42, 176})
	v := newMachine(context.Background(), map[string][]byte{"example/Factory": c.bytes("example/Factory", "java/lang/Object")})
	h := &bindings{vm: v, options: Options{Loader: "neoforge", ModID: "sample"}}
	v.host = h
	api := &biomeProbeBindings{host: h}
	h.extra = api
	const owner = "net/neoforged/neoforge/registries/DeferredRegister$Items"
	const registry = "net/minecraft/core/registries/Registries.ITEM"
	reg := v.alloc(owner, deferredRegistry{"sample", registry})
	props := v.alloc("net/minecraft/world/item/Item$Properties", nil)
	factory := &lambda{kind: 6, target: reference{"example/Factory", "apply", "(Ljava/lang/Object;)Ljava/lang/Object;"}}
	h.call(reference{owner, "registerItem", "(Ljava/lang/String;Ljava/util/function/Function;Lnet/minecraft/world/item/Item$Properties;)Lnet/neoforged/neoforge/registries/DeferredItem;"}, reg, []any{"test", factory, props})
	h.call(reference{owner, "register", "(Lnet/neoforged/bus/api/IEventBus;)V"}, reg, []any{nil})
	if len(api.registries) != 0 {
		t.Fatal("factory ran before its registry event")
	}
	api.fireRegistryEvents([]string{registry})
	if api.registries[registry]["sample:test"] != props {
		t.Fatal("specialized registration was skipped or lost its argument")
	}
	expectFault(t, "null registry entry", func() { api.register(registry, "sample:null", nil) })
}
