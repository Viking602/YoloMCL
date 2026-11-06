package modjar

import "strings"

const neoDeferredRegister = "net/neoforged/neoforge/registries/DeferredRegister"
const neoDeferredHolder = "net/neoforged/neoforge/registries/DeferredHolder"
const holderFactoryDescriptor = "(Lnet/minecraft/resources/ResourceKey;Lnet/minecraft/resources/ResourceLocation;)L" + neoDeferredHolder + ";"

func nativeDeferredHolder(name string) bool {
	return name == neoDeferredHolder || name == "net/neoforged/neoforge/registries/DeferredItem" || name == "net/neoforged/neoforge/registries/DeferredBlock"
}

// A covariant register call still uses the receiver's createHolder override.
// Picking a holder from the call site's erased return type changes Java casts.
func (h *biomeProbeBindings) deferredCall(r reference, receiver any, args []any) (any, bool) {
	if r.owner != neoDeferredRegister && r.owner != neoDeferredRegister+"$Items" && r.owner != neoDeferredRegister+"$Blocks" {
		return nil, false
	}
	v := h.host.vm
	if r.name == "createHolder" {
		kind := ""
		switch r.owner {
		case neoDeferredRegister:
			kind = neoDeferredHolder
		case neoDeferredRegister + "$Items":
			kind = "net/neoforged/neoforge/registries/DeferredItem"
		case neoDeferredRegister + "$Blocks":
			kind = "net/neoforged/neoforge/registries/DeferredBlock"
		}
		if kind == "" {
			fail("unbound deferred holder class")
		}
		if r.desc != holderFactoryDescriptor && r.desc != "(Lnet/minecraft/resources/ResourceKey;Lnet/minecraft/resources/ResourceLocation;)L"+kind+";" {
			return nil, false
		}
		if args[0] == nil || args[1] == nil {
			raiseJava("java/lang/NullPointerException", "null deferred key")
		}
		return v.alloc(kind, deferredValue{registry: objectData(args[0]).(string), id: objectData(args[1]).(string)}), true
	}
	if r.name != "register" {
		return nil, false
	}
	parameters, returns := descriptor(r.desc)
	if len(parameters) != 2 || parameters[0] != "Ljava/lang/String;" || (parameters[1] != "Ljava/util/function/Supplier;" && parameters[1] != "Ljava/util/function/Function;") {
		return nil, false
	}
	if returns != "L"+neoDeferredHolder+";" && returns != "Lnet/neoforged/neoforge/registries/DeferredItem;" && returns != "Lnet/neoforged/neoforge/registries/DeferredBlock;" {
		return nil, false
	}
	if args[0] == nil || args[1] == nil {
		raiseJava("java/lang/NullPointerException", "null deferred registration")
	}
	reg, native := objectData(receiver).(deferredRegistry)
	if !native {
		return nil, false
	}
	location := v.method(reference{"net/minecraft/resources/ResourceLocation", "fromNamespaceAndPath", "(Ljava/lang/String;Ljava/lang/String;)Lnet/minecraft/resources/ResourceLocation;"}, nil, false, []any{reg.namespace, javaString(args[0])})
	key := h.host.registryKey(reg.registry)
	owner := receiver.(*object).class
	for n := 0; n < 64; n++ {
		if owner == neoDeferredRegister || owner == neoDeferredRegister+"$Items" || owner == neoDeferredRegister+"$Blocks" {
			break
		}
		owner = v.load(owner).super
	}
	holder := v.method(reference{owner, "createHolder", holderFactoryDescriptor}, receiver, true, []any{key, location}).(*object)
	entry, ok := holder.data.(deferredValue)
	if !ok || entry.registry != reg.registry || entry.id != objectData(location) {
		fail("unbound deferred holder state")
	}
	entry.supplier = asSupplier(args[1])
	entry.owner = receiver.(*object)
	if strings.Contains(parameters[1], "Function;") {
		entry.args = []any{location}
	}
	h.deferValue(entry)
	holder.data = entry
	return holder, true
}

func asSupplier(x any) any {
	if x == nil {
		raiseJava("java/lang/NullPointerException", "null deferred supplier")
	}
	if _, ok := x.(*lambda); ok {
		return x
	}
	if _, ok := objectData(x).(*lambda); ok {
		return x
	}
	return x
}

func (v *machine) callSupplier(supplier any, args []any) any {
	if supplier == nil {
		raiseJava("java/lang/NullPointerException", "null deferred supplier")
	}
	if l, ok := supplier.(*lambda); ok {
		return v.callLambda(l, args)
	}
	if l, ok := objectData(supplier).(*lambda); ok {
		return v.callLambda(l, args)
	}
	if len(args) == 0 {
		return v.method(reference{"java/util/function/Supplier", "get", "()Ljava/lang/Object;"}, supplier, true, nil)
	}
	return v.method(reference{"java/util/function/Function", "apply", "(Ljava/lang/Object;)Ljava/lang/Object;"}, supplier, true, args)
}
