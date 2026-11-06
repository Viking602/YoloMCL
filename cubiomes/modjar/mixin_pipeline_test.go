package modjar

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

func TestDeclaredMixinResourceUsesSharedClasspath(t *testing.T) {
	declaring := fixtureArchive(t, map[string][]byte{"META-INF/neoforge.mods.toml": []byte("[[mixins]]\nconfig='shared.json'\n")})
	dependency := fixtureArchive(t, map[string][]byte{"shared.json": []byte(`{"package":"sample","mixins":[]}`)})
	roots := []Archive{{"declaration", declaring}, {"dependency", dependency}}
	env, err := BootstrapBiomeSources(context.Background(), roots, Options{Loader: "neoforge", Minecraft: "1.21.1"})
	if err != nil {
		t.Fatal(err)
	}
	env.Close()
	conflict := fixtureArchive(t, map[string][]byte{"shared.json": []byte(`{"package":"different","mixins":[]}`)})
	if _, err = BootstrapBiomeSources(context.Background(), append(roots, Archive{"conflicting", conflict}), Options{Loader: "neoforge", Minecraft: "1.21.1"}); err == nil || !strings.Contains(err.Error(), "ambiguous mixin config") {
		t.Fatal("conflicting resource silently selected", err)
	}
	if _, err = BootstrapBiomeSources(context.Background(), roots[:1], Options{Loader: "neoforge", Minecraft: "1.21.1"}); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatal("missing config silently ignored", err)
	}
}

func TestMixinLenientConfigPreservesStringsAndArrayHoles(t *testing.T) {
	data, err := mixinJSON([]byte(`{ // first
"package":"example", "mixins":["A",, "B",], # comment
"plugin":"https://example/*not-comment*/" /* last */}`))
	if err != nil {
		t.Fatal(err)
	}
	var cfg mixinConfig
	if err = json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Mixins) != 4 || cfg.Mixins[1] != nil || cfg.Mixins[3] != nil || cfg.Plugin != "https://example/*not-comment*/" {
		t.Fatalf("Gson-compatible inputs changed: %+v", cfg)
	}
}

func TestMixinMetadataUsesDeclaredLoaderAndSide(t *testing.T) {
	z := fixtureArchive(t, map[string][]byte{"fabric.mod.json": []byte(`{"mixins":["common.json",{"config":"client.json","environment":"client"}]}`), "META-INF/MANIFEST.MF": []byte("Manifest-Version: 1.0\r\nMixinConfigs: \r\n\r\n"), "META-INF/neoforge.mods.toml": []byte("[[mixins]]\nconfig='other.json'\nrequiredMods=['dependency']\n")})
	for _, item := range []struct {
		loader, side string
		mods         map[string]string
		want         int
	}{{"fabric", "CLIENT", nil, 2}, {"fabric", "DEDICATED_SERVER", nil, 1}, {"neoforge", "CLIENT", nil, 0}, {"neoforge", "CLIENT", map[string]string{"dependency": "1"}, 1}} {
		got, err := declaredMixins(context.Background(), z, Options{Loader: item.loader, Side: item.side, Mods: item.mods})
		if err != nil || len(got) != item.want {
			t.Fatalf("%+v: %v %v", item, got, err)
		}
	}
}

func TestClassCompatibilityFailureUsesJavaCatchRules(t *testing.T) {
	c := &fixtureClass{}
	c.method("<clinit>", "()V", 8, []byte{177})
	raw := c.bytes("example/Future", "java/lang/Object")
	binary.BigEndian.PutUint16(raw[6:8], 66)
	v := newMachine(context.Background(), map[string][]byte{"example/Future": raw})
	v.host = &bindings{vm: v, options: Options{JavaMajor: 21}}
	ref := reference{"java/lang/Class", "forName", "(Ljava/lang/String;)Ljava/lang/Class;"}
	expectJavaException(t, "java/lang/UnsupportedClassVersionError", func() { v.method(ref, nil, false, []any{"example.Future"}) })
	if v.initialized["example/Future"] {
		t.Fatal("incompatible class initialized")
	}
	if !v.exceptionAssignable(javaException{class: "java/lang/UnsupportedClassVersionError"}, "java/lang/LinkageError") {
		t.Fatal("wrong error hierarchy")
	}
	v.host.options.JavaMajor = 22
	if got := v.method(ref, nil, false, []any{"example.Future"}); got != classLiteral("example/Future") {
		t.Fatal(got)
	}
	expectFault(t, "forbidden reflected class", func() { v.method(ref, nil, false, []any{"java.lang.Runtime"}) })
}

func TestPluginStringAndArrayOperations(t *testing.T) {
	v := newMachine(context.Background(), nil)
	v.host = &bindings{vm: v}
	a := v.method(reference{"java/lang/String", "split", "(Ljava/lang/String;)[Ljava/lang/String;"}, "example.module.mixin", true, []any{"\\."}).(*array)
	b := v.method(reference{"java/util/Arrays", "copyOf", "([Ljava/lang/Object;I)[Ljava/lang/Object;"}, nil, false, []any{a, int32(2)}).(*array)
	if !v.instanceOf(b, "[Ljava/lang/CharSequence;") {
		t.Fatal("lost reference array covariance")
	}
	got := v.method(reference{"java/lang/String", "join", "(Ljava/lang/CharSequence;[Ljava/lang/CharSequence;)Ljava/lang/String;"}, nil, false, []any{".", b})
	if got != "example.module" {
		t.Fatal(got)
	}
	if len(a.values) != 3 {
		t.Fatal("array copy mutated source")
	}
	expectFault(t, "split pattern", func() {
		v.method(reference{"java/lang/String", "split", "(Ljava/lang/String;)[Ljava/lang/String;"}, "x", true, []any{"[x]+"})
	})
}

func TestMixinPluginExecutesCompatibilityDecisionOnce(t *testing.T) {
	const name = "example/ConditionalPlugin"
	c := &fixtureClass{}
	c.field("enabled", "Z", 2)
	c.method("<init>", "()V", 1, append(append(append([]byte{42}, c.instruction(183, "java/lang/Object", "<init>", "()V")...), 42, 4), append(c.instruction(181, name, "enabled", "Z"), 177)...))
	code := append(c.string("example.Future"), c.instruction(184, "java/lang/Class", "forName", "(Ljava/lang/String;)Ljava/lang/Class;")...)
	code = append(code, 87, 177)
	catch := len(code)
	code = append(code, 87, 42, 3)
	code = append(code, c.instruction(181, name, "enabled", "Z")...)
	code = append(code, 177)
	c.method("onLoad", "(Ljava/lang/String;)V", 1, code, exceptionHandler{start: 0, end: catch, pc: catch, catch: "java/lang/UnsupportedClassVersionError"})
	c.method("getMixins", "()Ljava/util/List;", 1, []byte{1, 176})
	for _, method := range []string{"preApply", "postApply"} {
		c.method(method, "(Ljava/lang/String;Lorg/objectweb/asm/tree/ClassNode;Ljava/lang/String;Lorg/spongepowered/asm/mixin/extensibility/IMixinInfo;)V", 1, []byte{177})
	}
	c.method("shouldApplyMixin", "(Ljava/lang/String;Ljava/lang/String;)Z", 1, append(append([]byte{42}, c.instruction(180, name, "enabled", "Z")...), 172))
	future := (&fixtureClass{}).bytes("example/Future", "java/lang/Object")
	binary.BigEndian.PutUint16(future[6:8], 66)
	raw := c.bytes(name, "java/lang/Object")
	for _, major := range []int{21, 22} {
		v := newMachine(context.Background(), map[string][]byte{name: raw, "example/Future": future})
		v.host = &bindings{vm: v, options: Options{JavaMajor: major}}
		e := &Environment{vm: v}
		effect := MixinEffect{Config: "test.json", Plugin: "example.ConditionalPlugin", Package: "example.mixin", Class: "example/mixin/Noise", Target: densityClass}
		if got := e.mixinEnabled(effect); got != (major == 22) {
			t.Fatalf("Java%d: enabled=%v", major, got)
		}
		first := e.mixinPlugins["test.json:example.ConditionalPlugin"]
		if got := e.mixinEnabled(effect); got != (major == 22) || e.mixinPlugins["test.json:example.ConditionalPlugin"] != first {
			t.Fatal("plugin was reconstructed between target decisions")
		}
	}
}

func TestAbortedInitializationCannotBecomeSuccessful(t *testing.T) {
	c := &fixtureClass{}
	c.method("<clinit>", "()V", 8, append(c.instruction(184, "example/Unavailable", "call", "()V"), 177))
	v := newMachine(context.Background(), map[string][]byte{"example/Initializer": c.bytes("example/Initializer", "java/lang/Object")})
	v.host = &bindings{vm: v}
	for i := 0; i < 2; i++ {
		expectFault(t, "unbound registration method", func() { v.initialize("example/Initializer") })
	}
}

func TestStringConcatFactoryExecutesRecipe(t *testing.T) {
	c := &fixtureClass{}
	handle := c.cp(15, append([]byte{6}, words(c.ref(10, "java/lang/invoke/StringConcatFactory", "makeConcatWithConstants", "(Ljava/lang/invoke/MethodHandles$Lookup;Ljava/lang/String;Ljava/lang/invoke/MethodType;Ljava/lang/String;[Ljava/lang/Object;)Ljava/lang/invoke/CallSite;"))...))
	recipe := c.cp(8, words(c.utf("sample:\x01/\x01/\x02")))
	constant := c.cp(8, words(c.utf("tail")))
	c.boots = words(1, handle, 2, recipe, constant)
	desc := "(ILjava/lang/String;)Ljava/lang/String;"
	nt := c.cp(12, words(c.utf("makeConcatWithConstants"), c.utf(desc)))
	indy := c.cp(18, words(0, nt))
	code := append([]byte{26, 43, 186}, words(indy)...)
	c.method("format", desc, 9, append(code, 0, 0, 176))
	raw := c.bytes("example/Format", "java/lang/Object")
	v := newMachine(context.Background(), map[string][]byte{"example/Format": raw})
	v.host = &bindings{vm: v}
	if got := v.method(reference{"example/Format", "format", desc}, nil, false, []any{int32(-2147483648), "地图"}); got != "sample:-2147483648/地图/tail" {
		t.Fatal(got)
	}
	if got := v.method(reference{"example/Format", "format", desc}, nil, false, []any{int32(0), nil}); got != "sample:0/null/tail" {
		t.Fatal(got)
	}
	parsed := v.load("example/Format")
	expectFault(t, "recipe arity", func() {
		javaConcat(parsed, bootstrap{args: []int{recipe, constant}}, "()Ljava/lang/String;", nil, true)
	})
}

func TestReflectionReadsOnlyPublicInputFields(t *testing.T) {
	c := &fixtureClass{}
	c.field("enabled", "Z", 9)
	c.field("privateFlag", "Z", 10)
	c.field("count", "I", 9)
	c.method("<clinit>", "()V", 8, append(append([]byte{4}, c.instruction(179, "example/Config", "enabled", "Z")...), 177))
	v := newMachine(context.Background(), map[string][]byte{"example/Config": c.bytes("example/Config", "java/lang/Object")})
	v.host = &bindings{vm: v}
	lookup := reference{"java/lang/Class", "getDeclaredField", "(Ljava/lang/String;)Ljava/lang/reflect/Field;"}
	getter := reference{"java/lang/reflect/Field", "getBoolean", "(Ljava/lang/Object;)Z"}
	f := v.method(lookup, classLiteral("example/Config"), true, []any{"enabled"})
	v.method(reference{"java/lang/reflect/Field", "setAccessible", "(Z)V"}, f, true, []any{int32(1)})
	if v.initialized["example/Config"] {
		t.Fatal("metadata lookup initialized class")
	}
	if got := v.method(getter, f, true, []any{nil}); got != int32(1) {
		t.Fatal(got)
	}
	boxed := v.method(reference{"java/lang/reflect/Field", "get", "(Ljava/lang/Object;)Ljava/lang/Object;"}, f, true, []any{nil}).(*object)
	if boxed.class != "java/lang/Boolean" || boxed.data != int32(1) {
		t.Fatal("reflection must box primitive fields", boxed)
	}
	expectJavaException(t, "java/lang/NoSuchFieldException", func() { v.method(lookup, classLiteral("example/Config"), true, []any{"absent"}) })
	private := v.method(lookup, classLiteral("example/Config"), true, []any{"privateFlag"})
	expectFault(t, "nonpublic", func() { v.method(getter, private, true, []any{nil}) })
	v.method(reference{"java/lang/reflect/Field", "setAccessible", "(Z)V"}, private, true, []any{int32(1)})
	if got := v.method(getter, private, true, []any{nil}); got != int32(0) {
		t.Fatal(got)
	}
	v.reflectionOpen = func(string) (bool, error) { return false, nil }
	expectFault(t, "named module", func() {
		v.method(reference{"java/lang/reflect/Field", "setAccessible", "(Z)V"}, private, true, []any{int32(1)})
	})
	count := v.method(lookup, classLiteral("example/Config"), true, []any{"count"})
	expectJavaException(t, "java/lang/IllegalArgumentException", func() { v.method(getter, count, true, []any{nil}) })
	expectFault(t, "forbidden", func() {
		v.method(reference{"java/lang/reflect/Field", "setBoolean", "(Ljava/lang/Object;Z)V"}, f, true, []any{nil, int32(0)})
	})
}
