package modjar

import "testing"

func TestGenericConfigOverloadUsesDefaultAndFailsClosedOnOverride(t *testing.T) {
	v := &machine{}
	h := &bindings{vm: v}
	v.host = h
	const owner = "net/minecraftforge/common/ForgeConfigSpec$Builder"
	builder := v.alloc(owner, &configBuilder{})
	def := v.alloc("java/util/ArrayList", &javaList{})
	ref := reference{owner, "defineList", "(Ljava/lang/String;Ljava/util/List;Ljava/util/function/Predicate;)Lnet/minecraftforge/common/ForgeConfigSpec$ConfigValue;"}
	x, ok := h.call(ref, builder, []any{"biomes", def, nil})
	if !ok {
		t.Fatal("defineList not accepted")
	}
	cv := objectData(x).(*configValue)
	if cv.value != any(def) || !cv.opaque {
		t.Fatal("default list not preserved")
	}
	h.options.Config = func(string) ([]byte, error) { return []byte("biomes = [\"a\"]\n"), nil }
	spec := v.alloc("net/minecraftforge/common/ForgeConfigSpec", &configSpec{[]*configValue{cv}})
	h.configure("x.toml", spec)
	if _, inert := cv.value.(*inertValue); !inert {
		t.Fatal("user override of unvalidated entry must be opaque")
	}
}
