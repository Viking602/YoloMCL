package modjar

import (
	"context"
	"testing"
)

func TestDeferredHolderVirtualFactoryAndTypes(t *testing.T) {
	v := newMachine(context.Background(), nil)
	h := &bindings{vm: v}
	v.host = h
	api := &biomeProbeBindings{host: h}
	h.extra = api
	supplier := &lambda{kind: 6, target: reference{"missing/Factory", "value", "()Ljava/lang/Object;"}}
	for i, test := range []struct{ owner, holder string }{
		{neoDeferredRegister, neoDeferredHolder},
		{neoDeferredRegister + "$Items", "net/neoforged/neoforge/registries/DeferredItem"},
		{neoDeferredRegister + "$Blocks", "net/neoforged/neoforge/registries/DeferredBlock"},
	} {
		reg := v.alloc(test.owner, deferredRegistry{"sample", "sample:registry"})
		result := v.method(reference{neoDeferredRegister, "register", "(Ljava/lang/String;Ljava/util/function/Supplier;)L" + neoDeferredHolder + ";"}, reg, true, []any{string(rune('a' + i)), supplier}).(*object)
		if result.class != test.holder || !v.instanceOf(result, neoDeferredHolder) || !v.instanceOf(result, "java/util/function/Supplier") {
			t.Fatalf("erased registration returned %s", result.class)
		}
		if test.holder != neoDeferredHolder && !v.instanceOf(result, "net/minecraft/world/level/ItemLike") {
			t.Fatal("item-like holder lost interface")
		}
		if len(api.registries) != 0 {
			t.Fatal("supplier executed before registry event")
		}
	}
}

func TestDeferredCustomFactoryAndSuperDispatch(t *testing.T) {
	c := &fixtureClass{}
	c.field("calls", "I", 9)
	code := append(c.instruction(178, "sample/Register", "calls", "I"), 4, 96)
	code = append(code, c.instruction(179, "sample/Register", "calls", "I")...)
	code = append(code, 42, 43, 44)
	code = append(code, c.instruction(183, neoDeferredRegister, "createHolder", holderFactoryDescriptor)...)
	c.method("createHolder", holderFactoryDescriptor, 1, append(code, 176))
	v := newMachine(context.Background(), map[string][]byte{"sample/Register": c.bytes("sample/Register", neoDeferredRegister)})
	h := &bindings{vm: v}
	v.host = h
	h.extra = &biomeProbeBindings{host: h}
	reg := v.alloc("sample/Register", deferredRegistry{"sample", "sample:registry"})
	supplier := &lambda{kind: 6, target: reference{"missing/Factory", "value", "()Ljava/lang/Object;"}}
	result := v.method(reference{neoDeferredRegister, "register", "(Ljava/lang/String;Ljava/util/function/Function;)L" + neoDeferredHolder + ";"}, reg, true, []any{"entry", supplier}).(*object)
	if v.static(reference{"sample/Register", "calls", "I"}) != int32(1) {
		t.Fatal("custom factory skipped")
	}
	state := result.data.(deferredValue)
	if len(state.args) != 1 || objectData(state.args[0]) != "sample:entry" {
		t.Fatal("function did not receive entry resource location")
	}
	items := v.alloc(neoDeferredRegister+"$Items", deferredRegistry{"sample", "sample:registry"})
	base := v.method(reference{neoDeferredRegister, "createHolder", holderFactoryDescriptor}, items, false, []any{h.registryKey("sample:registry"), state.args[0]}).(*object)
	if base.class != neoDeferredHolder {
		t.Fatal("invokespecial was virtually dispatched")
	}
}

func TestTransitiveInterfaceAssignability(t *testing.T) {
	parent := &fixtureClass{access: 0x601}
	child := &fixtureClass{access: 0x601, interfaces: []string{"sample/Parent"}}
	impl := &fixtureClass{interfaces: []string{"sample/Child"}}
	v := newMachine(context.Background(), map[string][]byte{"sample/Parent": parent.bytes("sample/Parent", "java/lang/Object"), "sample/Child": child.bytes("sample/Child", "java/lang/Object"), "sample/Impl": impl.bytes("sample/Impl", "java/lang/Object")})
	o := v.alloc("sample/Impl", nil)
	if !v.instanceOf(o, "sample/Parent") || v.instanceOf(o, "sample/Unrelated") {
		t.Fatal("transitive interface graph mismatch")
	}
}

func TestDeferredHolderMapKeysUseRegistryIdentity(t *testing.T) {
	v := newMachine(context.Background(), nil)
	v.host = &bindings{vm: v}
	m := v.alloc("java/util/HashMap", &javaMap{})
	first := v.alloc(neoDeferredHolder, deferredValue{registry: "minecraft:attribute", id: "sample:value"})
	second := v.alloc(neoDeferredHolder, deferredValue{registry: "minecraft:attribute", id: "sample:value"})
	otherRegistry := v.alloc(neoDeferredHolder, deferredValue{registry: "minecraft:item", id: "sample:value"})
	v.method(reference{"java/util/Map", "put", "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;"}, m, true, []any{first, "registered"})
	for _, key := range []any{first, second} {
		if got := v.method(reference{"java/util/Map", "get", "(Ljava/lang/Object;)Ljava/lang/Object;"}, m, true, []any{key}); got != "registered" {
			t.Fatal("equal holder keys diverged", got)
		}
	}
	if got := v.method(reference{"java/util/Map", "get", "(Ljava/lang/Object;)Ljava/lang/Object;"}, m, true, []any{otherRegistry}); got != nil {
		t.Fatal("different registries collapsed", got)
	}
	if got := v.method(reference{"java/util/HashMap", "isEmpty", "()Z"}, m, true, nil); got != int32(0) {
		t.Fatal(got)
	}
}
