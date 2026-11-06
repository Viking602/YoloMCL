package modjar

import (
	"context"
	"slices"
	"testing"
)

func TestLifecyclePriorityAndDeferredWork(t *testing.T) {
	v := newMachine(context.Background(), nil)
	h := &bindings{vm: v}
	v.host = h
	var order []string
	const event = "Lnet/minecraftforge/fml/event/lifecycle/FMLCommonSetupEvent;"
	callback := func(label string) *lambda {
		return &lambda{target: reference{"java/util/function/Consumer", "accept", "(Ljava/lang/Object;)V"}, kind: 5,
			captured: []any{&consumer{accept: func(any) { order = append(order, label) }}}, instantiated: "(" + event + ")V"}
	}
	for _, entry := range []struct{ label, priority string }{{"normal-a", "NORMAL"}, {"last", "LOWEST"}, {"first", "HIGHEST"}, {"normal-b", "NORMAL"}} {
		h.addListener([]any{v.alloc("net/neoforged/bus/api/EventPriority", eventPriority(entry.priority)), int32(0), callback(entry.label)})
	}
	deferred := callback("deferred")
	deferred.captured = append(deferred.captured, nil)
	h.call(reference{"net/neoforged/fml/event/lifecycle/FMLCommonSetupEvent", "enqueueWork", "(Ljava/lang/Runnable;)Ljava/util/concurrent/CompletableFuture;"}, nil, []any{deferred})
	if len(order) != 0 {
		t.Fatal("deferred callback ran inside listener registration")
	}
	h.fireEvent(event, nil)
	h.drainWork()
	if !slices.Equal(order, []string{"first", "normal-a", "normal-b", "last", "deferred"}) {
		t.Fatal(order)
	}
}

func TestLoaderSideSelection(t *testing.T) {
	a := annotation{values: map[string]any{"value": []any{enumValue{"Dist", "DEDICATED_SERVER"}}}}
	if annotationSide(a, "value", "") || annotationSide(a, "value", "CLIENT") || !annotationSide(a, "value", "DEDICATED_SERVER") {
		t.Fatal("wrong physical side")
	}
	if !annotationSide(annotation{}, "value", "") {
		t.Fatal("default subscription lost")
	}
}

func TestConfigLoadingEventRetainsRuntimeType(t *testing.T) {
	v := newMachine(context.Background(), nil)
	h := &bindings{vm: v}
	v.host = h
	const base = "net/minecraftforge/fml/event/config/ModConfigEvent"
	config := v.alloc("net/minecraftforge/fml/config/ModConfig", nil)
	event := v.alloc(base+"$Loading", config)
	if !v.instanceOf(event, base) || !v.instanceOf(event, "net/neoforged/fml/event/config/ModConfigEvent$Loading") {
		t.Fatal("config event lost its runtime subtype")
	}
	for _, owner := range []string{base, base + "$Loading"} {
		value, ok := h.call(reference{owner, "getConfig", "()Lnet/minecraftforge/fml/config/ModConfig;"}, event, nil)
		if !ok || value != config {
			t.Fatal("inherited config event accessor is not bound")
		}
	}
}

func TestSharedLoaderConfigAndResourceAPIs(t *testing.T) {
	v := newMachine(context.Background(), nil)
	var name string
	h := &bindings{vm: v, options: Options{ModID: "sample", Mods: map[string]string{"terrablender": "4"}, Config: func(n string) ([]byte, error) { name = n; return nil, nil }}}
	v.host = h
	typeValue, ok := h.static(reference{"net/neoforged/fml/config/ModConfig$Type", "COMMON", ""})
	if !ok {
		t.Fatal("configuration type not bound")
	}
	_, ok = h.call(reference{"net/neoforged/fml/ModContainer", "registerConfig", ""}, nil, []any{typeValue, v.alloc("net/neoforged/neoforge/common/ModConfigSpec", &configSpec{})})
	if !ok || name != "sample-common.toml" {
		t.Fatalf("config filename %s", name)
	}
	for _, owner := range []string{"net/neoforged/fml/ModList", "net/minecraftforge/fml/ModList", "net/fabricmc/loader/api/FabricLoader"} {
		for _, test := range []struct {
			id   string
			want int32
		}{{"terrablender", 1}, {"missing", 0}} {
			value, ok := h.call(reference{owner, "isLoaded", ""}, nil, []any{test.id})
			if !ok || value != test.want {
				t.Fatalf("%s %s: %v", owner, test.id, value)
			}
		}
	}
	for _, test := range []struct {
		name string
		args []any
		want string
	}{{"fromNamespaceAndPath", []any{"sample", "biome"}, "sample:biome"}, {"parse", []any{"sample:biome"}, "sample:biome"}, {"withDefaultNamespace", []any{"plains"}, "minecraft:plains"}} {
		value, ok := h.call(reference{"net/minecraft/resources/ResourceLocation", test.name, ""}, nil, test.args)
		if !ok || objectData(value) != test.want {
			t.Fatalf("resource factory %s: %v", test.name, value)
		}
	}
}
