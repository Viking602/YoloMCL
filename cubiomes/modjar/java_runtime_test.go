package modjar

import (
	"context"
	"math"
	"strings"
	"testing"
)

func TestNativePairDispatchWithInstalledLibrary(t *testing.T) {
	c := &fixtureClass{}
	c.field("right", "Ljava/lang/Object;", 1)
	code := append([]byte{42}, c.instruction(180, "org/apache/commons/lang3/tuple/ImmutablePair", "right", "Ljava/lang/Object;")...)
	c.method("getRight", "()Ljava/lang/Object;", 1, append(code, 176))
	v := newMachine(context.Background(), map[string][]byte{"org/apache/commons/lang3/tuple/ImmutablePair": c.bytes("org/apache/commons/lang3/tuple/ImmutablePair", "org/apache/commons/lang3/tuple/Pair")})
	v.host = &bindings{vm: v}
	r := reference{"org/apache/commons/lang3/tuple/Pair", "getRight", "()Ljava/lang/Object;"}
	native := v.alloc("org/apache/commons/lang3/tuple/ImmutablePair", javaPair{nil, "native state"})
	if got := v.method(r, native, true, nil); got != "native state" {
		t.Fatal("native state replaced by uninitialised Java fields", got)
	}
	raw := v.alloc(native.class, nil)
	raw.fields["org/apache/commons/lang3/tuple/ImmutablePair.right:Ljava/lang/Object;"] = "Java state"
	if got := v.method(r, raw, true, nil); got != "Java state" {
		t.Fatal("Java implementation overwritten", got)
	}
}

func TestClassNamesPreserveArrayDescriptors(t *testing.T) {
	v := newMachine(context.Background(), nil)
	v.host = &bindings{vm: v}
	for from, want := range map[string]string{"java/lang/String": "java.lang.String", "[[Ljava/lang/String;": "[[Ljava.lang.String;", "[I": "[I", "int": "int"} {
		if got := v.method(reference{"java/lang/Class", "getName", "()Ljava/lang/String;"}, classLiteral(from), true, nil); got != want {
			t.Fatalf("%s: %v", from, got)
		}
	}
}

func TestServiceDiscoveryLazinessAndClasspathResources(t *testing.T) {
	c := &fixtureClass{interfaces: []string{"sample/Service"}}
	c.field("created", "I", 9)
	code := append([]byte{4}, c.instruction(179, "sample/Provider", "created", "I")...)
	code = append(code, 177)
	c.method("<init>", "()V", 1, code)
	z := fixtureArchive(t, map[string][]byte{"sample/Provider.class": c.bytes("sample/Provider", "java/lang/Object"), "META-INF/services/sample.Service": []byte("# service\nsample.Provider\nsample.Provider # duplicate\n")})
	cp, err := NewClasspath(context.Background(), []Archive{{"module", z}}, "neoforge")
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	v := newMachine(context.Background(), nil)
	v.resolveClass = cp.Class
	reads := 0
	v.host = &bindings{vm: v, options: Options{Resources: func(name string) ([][]byte, error) { reads++; return cp.Resources(name) }}}
	s := v.method(reference{"java/util/ServiceLoader", "load", "(Ljava/lang/Class;)Ljava/util/ServiceLoader;"}, nil, false, []any{classLiteral("sample/Service")})
	if v.initialized["sample/Provider"] {
		t.Fatal("provider initialized during service discovery")
	}
	if reads != 0 {
		t.Fatal("service load read provider files eagerly")
	}
	first := v.method(reference{"java/util/ServiceLoader", "findFirst", "()Ljava/util/Optional;"}, s, true, nil)
	x := v.method(reference{"java/util/Optional", "orElseThrow", "()Ljava/lang/Object;"}, first, true, nil)
	if got := v.static(reference{"sample/Provider", "created", "I"}); got != int32(1) {
		t.Fatal(got)
	}
	it := v.method(reference{"java/util/ServiceLoader", "iterator", "()Ljava/util/Iterator;"}, s, true, nil)
	if next := v.method(reference{"java/util/Iterator", "next", "()Ljava/lang/Object;"}, it, true, nil); next != x {
		t.Fatal("provider not cached")
	}
	if has := v.method(reference{"java/util/Iterator", "hasNext", "()Z"}, it, true, nil); has != int32(0) {
		t.Fatal("duplicate provider")
	}
	expectJavaException(t, "java/util/NoSuchElementException", func() { v.method(reference{"java/util/Iterator", "next", "()Ljava/lang/Object;"}, it, true, nil) })
	empty := v.method(reference{"java/util/Optional", "empty", "()Ljava/util/Optional;"}, nil, false, nil)
	if got := v.method(reference{"java/util/Optional", "orElse", "(Ljava/lang/Object;)Ljava/lang/Object;"}, empty, true, []any{"fallback"}); got != "fallback" {
		t.Fatal(got)
	}
}

func TestNativeCollectionsPreserveMutationAndValueSemantics(t *testing.T) {
	v := newMachine(context.Background(), nil)
	v.host = &bindings{vm: v}
	for _, name := range []string{"java/util/ArrayList", "java/util/concurrent/CopyOnWriteArrayList"} {
		list := v.alloc(name, nil)
		v.method(reference{name, "<init>", "()V"}, list, false, nil)
		v.method(reference{"java/util/List", "add", "(Ljava/lang/Object;)Z"}, list, true, []any{"first"})
		it := v.method(reference{"java/util/List", "iterator", "()Ljava/util/Iterator;"}, list, true, nil)
		v.method(reference{"java/util/List", "add", "(Ljava/lang/Object;)Z"}, list, true, []any{"second"})
		read := func() any {
			return v.method(reference{"java/util/Iterator", "next", "()Ljava/lang/Object;"}, it, true, nil)
		}
		if name == "java/util/ArrayList" {
			expectJavaException(t, "java/util/ConcurrentModificationException", func() { read() })
		} else {
			if read() != "first" {
				t.Fatal("snapshot changed")
			}
			expectJavaException(t, "java/util/NoSuchElementException", func() { read() })
		}
	}
	for _, name := range []string{"java/util/HashMap", "java/util/IdentityHashMap"} {
		m := v.alloc(name, nil)
		v.method(reference{name, "<init>", "()V"}, m, false, nil)
		a, b := v.alloc("java/lang/Integer", int32(99)), v.alloc("java/lang/Integer", int32(99))
		v.method(reference{"java/util/Map", "put", "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;"}, m, true, []any{a, "value"})
		x := v.method(reference{"java/util/Map", "get", "(Ljava/lang/Object;)Ljava/lang/Object;"}, m, true, []any{b})
		if name == "java/util/HashMap" && x != "value" || name == "java/util/IdentityHashMap" && x != nil {
			t.Fatalf("map equality %s %v", name, x)
		}
	}
	a, b := v.alloc("java/lang/Double", math.Copysign(0, -1)), v.alloc("java/lang/Double", float64(0))
	if v.collectionEquals(a, b, false) {
		t.Fatal("boxed signed zero merged")
	}
	a.data = math.Float64frombits(0x7ff8000000000001)
	b.data = math.Float64frombits(0x7ff8000000000002)
	if !v.collectionEquals(a, b, false) {
		t.Fatal("boxed NaNs must compare equal")
	}
}

func TestUUIDAndConfigFactories(t *testing.T) {
	v := newMachine(context.Background(), nil)
	v.host = &bindings{vm: v}
	u := v.method(reference{"java/util/UUID", "fromString", "(Ljava/lang/String;)Ljava/util/UUID;"}, nil, false, []any{"1-1-1-1-1"})
	if got := v.method(reference{"java/util/UUID", "toString", "()Ljava/lang/String;"}, u, true, nil); got != "00000001-0001-0001-0001-000000000001" {
		t.Fatal(got)
	}
	expectJavaException(t, "java/lang/IllegalArgumentException", func() {
		v.method(reference{"java/util/UUID", "fromString", "(Ljava/lang/String;)Ljava/util/UUID;"}, nil, false, []any{"bad"})
	})
	if got := v.host.formatString("%2$s %1$d %% %3$b", []any{v.alloc("java/lang/Integer", int32(42)), "value", nil}); got != "value 42 % false" {
		t.Fatal(got)
	}
	expectFault(t, "conversion", func() { v.host.formatString("%f", []any{v.alloc("java/lang/Double", 1.)}) })
	expectFault(t, "type", func() { v.host.formatString("%d", []any{v.alloc("java/lang/Boolean", int32(1))}) })
	c := &fixtureClass{}
	c.method("apply", "(Ljava/lang/Object;)Ljava/lang/Object;", 9, []byte{42, 176})
	v.raw["sample/Factory"] = c.bytes("sample/Factory", "java/lang/Object")
	owner := "net/minecraftforge/common/ForgeConfigSpec$Builder"
	builder := v.alloc(owner, nil)
	v.method(reference{owner, "<init>", "()V"}, builder, false, nil)
	fn := &lambda{kind: 6, target: reference{"sample/Factory", "apply", "(Ljava/lang/Object;)Ljava/lang/Object;"}}
	p := v.method(reference{owner, "configure", "(Ljava/util/function/Function;)Lorg/apache/commons/lang3/tuple/Pair;"}, builder, true, []any{fn})
	left := v.method(reference{"org/apache/commons/lang3/tuple/Pair", "getLeft", "()Ljava/lang/Object;"}, p, true, nil)
	if left != builder {
		t.Fatal("configuration factory not called")
	}
	var requested string
	v.host.options.Config = func(name string) ([]byte, error) { requested = name; return nil, nil }
	v.host.options.ModID = "example"
	right := v.method(reference{"org/apache/commons/lang3/tuple/Pair", "getRight", "()Ljava/lang/Object;"}, p, true, nil)
	kind := v.host.symbol("server", "net/minecraftforge/fml/config/ModConfig$Type", "SERVER")
	v.method(reference{"net/minecraftforge/fml/ModContainer", "registerConfig", "(Lnet/minecraftforge/fml/config/ModConfig$Type;Lnet/minecraftforge/common/ForgeConfigSpec;)V"}, v.alloc("net/minecraftforge/fml/ModContainer", nil), true, []any{kind, right})
	if requested != "serverconfig/example-server.toml" {
		t.Fatal(requested)
	}
}

func TestImmutableFactoryCollectionsRejectInvalidMutations(t *testing.T) {
	v := newMachine(context.Background(), nil)
	v.host = &bindings{vm: v}
	makeSet := func(args ...any) any {
		return v.method(reference{"java/util/Set", "of", "([Ljava/lang/Object;)Ljava/util/Set;"}, nil, false, []any{&array{descriptor: "[Ljava/lang/Object;", values: args}})
	}
	expectJavaException(t, "java/lang/NullPointerException", func() { makeSet(nil) })
	expectJavaException(t, "java/lang/IllegalArgumentException", func() { makeSet("a", "a") })
	set := makeSet("a")
	expectJavaException(t, "java/lang/UnsupportedOperationException", func() { v.method(reference{"java/util/Set", "add", "(Ljava/lang/Object;)Z"}, set, true, []any{"b"}) })
	m := v.method(reference{"java/util/Map", "of", "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/util/Map;"}, nil, false, []any{"a", "b"})
	if got := v.method(reference{"java/util/Map", "get", "(Ljava/lang/Object;)Ljava/lang/Object;"}, m, true, []any{"a"}); got != "b" {
		t.Fatal(got)
	}
	expectJavaException(t, "java/lang/UnsupportedOperationException", func() {
		v.method(reference{"java/util/Map", "put", "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;"}, m, true, []any{"a", "c"})
	})
	legacy := v.method(reference{"java/util/Collections", "emptyList", "()Ljava/util/List;"}, nil, false, nil)
	v.method(reference{"java/util/List", "clear", "()V"}, legacy, true, nil)
	modern := v.method(reference{"java/util/List", "of", "()Ljava/util/List;"}, nil, false, nil)
	expectJavaException(t, "java/lang/UnsupportedOperationException", func() { v.method(reference{"java/util/List", "clear", "()V"}, modern, true, nil) })
}

func TestCollectionFactoriesCopyInputAndDoNotDropCustomHashing(t *testing.T) {
	c := &fixtureClass{}
	c.method("hashCode", "()I", 1, []byte{4, 172})
	v := newMachine(context.Background(), map[string][]byte{"sample/HashKey": c.bytes("sample/HashKey", "java/lang/Object")})
	v.host = &bindings{vm: v}
	a := &array{descriptor: "[Ljava/lang/Object;", values: []any{"initial", nil}}
	list := v.method(reference{"com/google/common/collect/Lists", "newArrayList", "([Ljava/lang/Object;)Ljava/util/ArrayList;"}, nil, false, []any{a})
	a.values[0] = "changed"
	if got := v.method(reference{"java/util/List", "get", "(I)Ljava/lang/Object;"}, list, true, []any{int32(0)}); got != "initial" {
		t.Fatal(got)
	}
	m := v.alloc("java/util/HashMap", &javaMap{})
	key := v.alloc("sample/HashKey", nil)
	// Custom hashCode must be executed (not ignored). The fixture returns 1.
	if got := v.method(reference{"java/util/Map", "put", "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;"}, m, true, []any{key, "value"}); got != nil {
		t.Fatalf("put: %v", got)
	}
	if got := v.method(reference{"java/util/Map", "get", "(Ljava/lang/Object;)Ljava/lang/Object;"}, m, true, []any{key}); got != "value" {
		t.Fatalf("get: %v", got)
	}
	str := v.alloc("java/lang/String", "same")
	if !v.collectionEquals("same", str, false) || !v.collectionEquals(str, "same", false) {
		t.Fatal("asymmetric string value equality")
	}
}

func TestFoodEffectRegistrationRemainsLazyAndOrdered(t *testing.T) {
	v := newMachine(context.Background(), nil)
	h := &bindings{vm: v}
	v.host = h
	h.extra = &biomeProbeBindings{host: h}
	owner := "net/minecraft/world/food/FoodProperties$Builder"
	b := v.alloc(owner, nil)
	v.method(reference{owner, "<init>", "()V"}, b, false, nil)
	supplier := &lambda{kind: 6, target: reference{"unavailable/Effect", "get", "()Ljava/lang/Object;"}}
	for _, probability := range []float32{0.25, 0.75} {
		v.method(reference{owner, "effect", "(Ljava/util/function/Supplier;F)Lnet/minecraft/world/food/FoodProperties$Builder;"}, b, true, []any{supplier, probability})
	}
	v.method(reference{owner, "alwaysEdible", "()Lnet/minecraft/world/food/FoodProperties$Builder;"}, b, true, nil)
	food := v.method(reference{owner, "build", "()Lnet/minecraft/world/food/FoodProperties;"}, b, true, nil)
	if objectData(food).(map[string][]any)["alwaysEdible"][0] != int32(1) {
		t.Fatal("food flags lost")
	}
	values := objectData(food).(map[string][]any)["effects"]
	if len(values) != 4 || values[0] != supplier || values[1] != float32(0.25) || values[3] != float32(0.75) {
		t.Fatal(values)
	}
	v.method(reference{owner, "effect", "(Ljava/util/function/Supplier;F)Lnet/minecraft/world/food/FoodProperties$Builder;"}, b, true, []any{supplier, float32(1)})
	if len(values) != 4 {
		t.Fatal("built properties changed")
	}
}

func TestRuntimeExceptionIsNotRuntimeProcessAccess(t *testing.T) {
	c := &fixtureClass{}
	code := c.newObject("java/lang/RuntimeException")
	code = append(code, c.string("ordinary exception")...)
	code = append(code, c.instruction(183, "java/lang/RuntimeException", "<init>", "(Ljava/lang/String;)V")...)
	code = append(code, 191)
	end := len(code)
	code = append(code, 87, 16, 42, 172)
	c.method("caught", "()I", 9, code, exceptionHandler{0, end, end, "java/lang/RuntimeException"})
	v := newMachine(context.Background(), map[string][]byte{"sample/Catcher": c.bytes("sample/Catcher", "java/lang/Object")})
	v.host = &bindings{vm: v}
	if got := v.method(reference{"sample/Catcher", "caught", "()I"}, nil, false, nil); got != int32(42) {
		t.Fatal(got)
	}
	for _, name := range []string{"java/lang/Runtime", "java/lang/Runtime$Version", "java/lang/ProcessBuilder", "java/lang/ProcessBuilder$Redirect", "java/lang/System"} {
		if !forbiddenClass(name) {
			t.Fatal("host API allowed", name)
		}
	}
}

func TestInitializationDiagnosticsPreserveOriginalCause(t *testing.T) {
	c := &fixtureClass{}
	code := c.newObject("java/lang/RuntimeException")
	code = append(code, c.string("actual failure")...)
	code = append(code, c.instruction(183, "java/lang/RuntimeException", "<init>", "(Ljava/lang/String;)V")...)
	c.method("<clinit>", "()V", 8, append(code, 191))
	v := newMachine(context.Background(), map[string][]byte{"sample/InitializationFailure": c.bytes("sample/InitializationFailure", "java/lang/Object")})
	v.host = &bindings{vm: v}
	defer func() {
		p := recover()
		e, ok := p.(javaException)
		if !ok || e.class != "java/lang/ExceptionInInitializerError" || !strings.Contains(e.Error(), "java/lang/RuntimeException: actual failure") {
			t.Fatalf("lost cause: %v", p)
		}
	}()
	v.initialize("sample/InitializationFailure")
	t.Fatal("failed initialization succeeded")
}
