package modjar

import (
	"context"
	"testing"
)

func expectJavaException(t *testing.T, class string, run func()) {
	t.Helper()
	defer func() {
		e, ok := recover().(javaException)
		if !ok || e.class != class {
			t.Fatalf("expected %s, got %#v", class, e)
		}
	}()
	run()
}

func TestJavaCatchPreservesControlFlowButCannotCatchHostFailures(t *testing.T) {
	c := &fixtureClass{}
	c.method("divide", "()I", 9, []byte{4, 3, 108, 172, 87, 16, 42, 172}, exceptionHandler{0, 4, 4, "java/lang/RuntimeException"})
	forbidden := c.instruction(184, "java/lang/Runtime", "getRuntime", "()Ljava/lang/Runtime;")
	forbidden = append(forbidden, 87, 3, 172, 87, 4, 172)
	c.method("forbidden", "()I", 9, forbidden, exceptionHandler{0, 6, 6, "java/lang/Throwable"})
	throw := c.newObject("java/lang/IllegalArgumentException")
	throw = append(throw, c.string("expected")...)
	throw = append(throw, c.instruction(183, "java/lang/IllegalArgumentException", "<init>", "(Ljava/lang/String;)V")...)
	throw = append(throw, 191)
	c.method("throwException", "()V", 9, throw)
	caller := c.instruction(184, "example/Exceptions", "throwException", "()V")
	caller = append(caller, 3, 172, 87, 16, 43, 172)
	c.method("caller", "()I", 9, caller, exceptionHandler{0, 3, 5, "java/lang/RuntimeException"})
	v := newMachine(context.Background(), map[string][]byte{"example/Exceptions": c.bytes("example/Exceptions", "java/lang/Object")})
	v.host = &bindings{vm: v}
	for name, want := range map[string]int32{"divide": 42, "caller": 43} {
		if got := v.method(reference{"example/Exceptions", name, "()I"}, nil, false, nil); got != want {
			t.Fatalf("%s=%v", name, got)
		}
	}
	expectFault(t, "forbidden registration API", func() { v.method(reference{"example/Exceptions", "forbidden", "()I"}, nil, false, nil) })
}

func TestFailedJavaInitializationCannotExposePartialStatics(t *testing.T) {
	c := &fixtureClass{}
	c.field("value", "I", 9)
	code := []byte{4}
	code = append(code, c.instruction(179, "example/Failed", "value", "I")...)
	code = append(code, 4, 3, 108, 87, 177)
	c.method("<clinit>", "()V", 8, code)
	v := newMachine(context.Background(), map[string][]byte{"example/Failed": c.bytes("example/Failed", "java/lang/Object")})
	v.host = &bindings{vm: v}
	expectJavaException(t, "java/lang/ExceptionInInitializerError", func() { v.initialize("example/Failed") })
	expectJavaException(t, "java/lang/NoClassDefFoundError", func() { v.static(reference{"example/Failed", "value", "I"}) })
}

func TestJavaArrayNarrowingCloneAndStoreChecks(t *testing.T) {
	v := newMachine(context.Background(), nil)
	v.host = &bindings{vm: v}
	for _, test := range []struct {
		kind, store, load byte
		want              int32
	}{{8, 84, 51, -1}, {5, 85, 52, 65535}, {9, 86, 53, -1}, {4, 84, 51, 1}} {
		code := []byte{4, 188, test.kind, 89, 3, 2, test.store, 3, test.load, 172}
		got := v.run(&class{name: "Array"}, &method{name: "run", access: 8, stack: 5, code: code}, nil, nil)
		if got != test.want {
			t.Fatalf("array kind %d=%v", test.kind, got)
		}
	}
	a := &array{descriptor: "[B", values: []any{int32(-1)}}
	b := v.method(reference{"[B", "clone", "()Ljava/lang/Object;"}, a, true, nil).(*array)
	b.values[0] = int32(1)
	if a.values[0] != int32(-1) || b.descriptor != "[B" {
		t.Fatal("array clone lost type or aliased storage")
	}
	wrong := &array{descriptor: "[Ljava/lang/String;", values: []any{nil}}
	expectJavaException(t, "java/lang/ArrayStoreException", func() {
		v.run(&class{name: "Store"}, &method{name: "run", access: 8, stack: 3, locals: 2, code: []byte{42, 3, 43, 83, 177}}, nil, []any{wrong, v.alloc("java/lang/Integer", int32(1))})
	})
	expectJavaException(t, "java/lang/NullPointerException", func() { arrayAccess(nil, 0) })
	expectJavaException(t, "java/lang/ArrayIndexOutOfBoundsException", func() { arrayAccess(a, -1) })
}

func TestBase64AndLocaleAreExplicit(t *testing.T) {
	v := newMachine(context.Background(), nil)
	h := &bindings{vm: v}
	v.host = h
	decoder := v.method(reference{"java/util/Base64", "getDecoder", "()Ljava/util/Base64$Decoder;"}, nil, false, nil)
	decode := reference{"java/util/Base64$Decoder", "decode", "(Ljava/lang/String;)[B"}
	for _, s := range []string{"/w==", "/w"} {
		a := v.method(decode, decoder, true, []any{s}).(*array)
		if len(a.values) != 1 || a.values[0] != int32(-1) {
			t.Fatal(a)
		}
	}
	expectJavaException(t, "java/lang/IllegalArgumentException", func() { v.method(decode, decoder, true, []any{"Zg==\n"}) })
	lower := reference{"java/lang/String", "toLowerCase", "()Ljava/lang/String;"}
	expectFault(t, "default locale", func() { v.method(lower, "I", true, nil) })
	h.options.Language = "tr"
	if got := v.method(lower, "Ii", true, nil); got != "ıi" {
		t.Fatal(got)
	}
}
