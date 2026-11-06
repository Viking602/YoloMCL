package modjar

import (
	"context"
	"testing"
)

func TestClassLiteralArrayAndReflectiveConstruction(t *testing.T) {
	c := &fixtureClass{}
	c.field("value", "Ljava/lang/String;", 17)
	code := append([]byte{42, 43}, c.instruction(181, "sample/Constructed", "value", "Ljava/lang/String;")...)
	c.method("<init>", "(Ljava/lang/String;)V", 1, append(code, 177))
	v := newMachine(context.Background(), map[string][]byte{"sample/Constructed": c.bytes("sample/Constructed", "java/lang/Object")})
	v.host = &bindings{vm: v}
	types := &array{descriptor: "[Ljava/lang/Class;", values: make([]any, 1)}
	store := &method{name: "store", access: 8, stack: 3, locals: 2, code: []byte{42, 3, 43, 83, 177}}
	v.run(&class{name: "sample/Store"}, store, nil, []any{types, classLiteral("java/lang/String")})
	if types.values[0] != classLiteral("java/lang/String") || !v.instanceOf(types, "[Ljava/lang/reflect/Type;") {
		t.Fatal("class literal array covariance")
	}
	expectJavaException(t, "java/lang/ArrayStoreException", func() { v.run(&class{name: "sample/Store"}, store, nil, []any{types, "not a Class"}) })
	lookup := reference{"java/lang/Class", "getConstructor", "([Ljava/lang/Class;)Ljava/lang/reflect/Constructor;"}
	ctor := v.method(lookup, classLiteral("sample/Constructed"), true, []any{types})
	if v.initialized["sample/Constructed"] {
		t.Fatal("constructor lookup initialized class")
	}
	create := reference{"java/lang/reflect/Constructor", "newInstance", "([Ljava/lang/Object;)Ljava/lang/Object;"}
	obj := v.method(create, ctor, true, []any{&array{descriptor: "[Ljava/lang/Object;", values: []any{"kept"}}}).(*object)
	if obj.fields[fieldKey(reference{"sample/Constructed", "value", "Ljava/lang/String;"})] != "kept" {
		t.Fatal("constructor did not execute")
	}
	expectJavaException(t, "java/lang/IllegalArgumentException", func() { v.method(create, ctor, true, []any{nil}) })
	expectJavaException(t, "java/lang/IllegalArgumentException", func() {
		v.method(create, ctor, true, []any{&array{descriptor: "[Ljava/lang/Object;", values: []any{v.alloc("java/lang/Object", nil)}}})
	})
	expectJavaException(t, "java/lang/NoSuchMethodException", func() { v.method(lookup, classLiteral("sample/Constructed"), true, []any{nil}) })
}

func TestReflectedConstructorAccessAndHostFaults(t *testing.T) {
	private := &fixtureClass{}
	private.method("<init>", "()V", 2, []byte{177})
	bad := &fixtureClass{}
	bad.method("<init>", "()V", 1, append(bad.instruction(184, "java/lang/Runtime", "getRuntime", "()Ljava/lang/Runtime;"), 87, 177))
	throws := &fixtureClass{}
	code := throws.newObject("java/lang/IllegalArgumentException")
	code = append(code, throws.instruction(183, "java/lang/IllegalArgumentException", "<init>", "()V")...)
	throws.method("<init>", "()V", 1, append(code, 191))
	v := newMachine(context.Background(), map[string][]byte{"sample/Private": private.bytes("sample/Private", "java/lang/Object"), "sample/Host": bad.bytes("sample/Host", "java/lang/Object"), "sample/Throws": throws.bytes("sample/Throws", "java/lang/Object")})
	v.host = &bindings{vm: v}
	lookup := func(owner, method string) any {
		return v.method(reference{"java/lang/Class", method, "([Ljava/lang/Class;)Ljava/lang/reflect/Constructor;"}, classLiteral(owner), true, []any{nil})
	}
	create := reference{"java/lang/reflect/Constructor", "newInstance", "([Ljava/lang/Object;)Ljava/lang/Object;"}
	ctor := lookup("sample/Private", "getDeclaredConstructor")
	expectJavaException(t, "java/lang/NoSuchMethodException", func() { lookup("sample/Private", "getConstructor") })
	expectFault(t, "nonpublic", func() { v.method(create, ctor, true, []any{nil}) })
	access := reference{"java/lang/reflect/Constructor", "setAccessible", "(Z)V"}
	v.method(access, ctor, true, []any{int32(1)})
	if got := v.method(create, ctor, true, []any{nil}).(*object); got.class != "sample/Private" {
		t.Fatal(got)
	}
	v.reflectionOpen = func(string) (bool, error) { return false, nil }
	expectFault(t, "named module", func() { v.method(access, ctor, true, []any{int32(1)}) })
	v.reflectionOpen = nil
	throwing := lookup("sample/Throws", "getConstructor")
	expectJavaException(t, "java/lang/reflect/InvocationTargetException", func() { v.method(create, throwing, true, []any{nil}) })
	host := lookup("sample/Host", "getConstructor")
	expectFault(t, "forbidden registration API", func() { v.method(create, host, true, []any{nil}) })
}

func TestReflectiveInitializationIsNotConstructorFailure(t *testing.T) {
	c := &fixtureClass{}
	c.method("<init>", "()V", 1, []byte{177})
	c.method("<clinit>", "()V", 8, []byte{4, 3, 108, 87, 177})
	v := newMachine(context.Background(), map[string][]byte{"sample/Init": c.bytes("sample/Init", "java/lang/Object")})
	v.host = &bindings{vm: v}
	ctor := v.method(reference{"java/lang/Class", "getConstructor", "([Ljava/lang/Class;)Ljava/lang/reflect/Constructor;"}, classLiteral("sample/Init"), true, []any{nil})
	expectJavaException(t, "java/lang/ExceptionInInitializerError", func() {
		v.method(reference{"java/lang/reflect/Constructor", "newInstance", "([Ljava/lang/Object;)Ljava/lang/Object;"}, ctor, true, []any{nil})
	})
}

func TestLinkedHashMapConstructionAndReplacement(t *testing.T) {
	v := newMachine(context.Background(), nil)
	v.host = &bindings{vm: v}
	m := v.alloc("java/util/LinkedHashMap", nil)
	v.method(reference{m.class, "<init>", "()V"}, m, false, nil)
	put := reference{"java/util/Map", "put", "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;"}
	for _, key := range []string{"b", "a", "b"} {
		v.method(put, m, true, []any{key, "value"})
	}
	if got := v.method(reference{"java/util/Map", "size", "()I"}, m, true, nil); got != int32(2) {
		t.Fatal(got)
	}
	if !v.instanceOf(m, "java/util/HashMap") {
		t.Fatal("native map superclass")
	}
	values := v.method(reference{"java/util/Map", "values", "()Ljava/util/Collection;"}, m, true, nil)
	if values == nil || !v.instanceOf(values, "java/util/Collection") {
		t.Fatal(values)
	}
}
