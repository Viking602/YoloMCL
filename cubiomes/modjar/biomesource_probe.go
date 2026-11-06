package modjar

import (
	"archive/zip"
	"context"
	"fmt"
	"strings"
)

const biomeSourceClass = "net/minecraft/world/level/biome/BiomeSource"
const recordBuilderClass = "com/mojang/serialization/codecs/RecordCodecBuilder"
const climateSamplerClass = "net/minecraft/world/level/biome/Climate$Sampler"
const densityClass = "net/minecraft/world/level/levelgen/DensityFunction"

// BiomeProbeQuery supplies quart coordinates and constant raw density inputs
// (temperature, humidity, continentalness, erosion, depth, weirdness).
// It tests JAR execution independently of a particular terrain noise engine.
type BiomeProbeQuery struct {
	X, Y, Z int32
	Climate [6]float64
}

type BiomeProbeResult struct {
	Biomes, CodecBiomes []string
	Steps               int
}

// ProbeBiomeSource executes a named source's registry-only MapCodec and query
// methods in Go. This diagnostic does not resolve a game's codec registration,
// run a loader lifecycle, or certify world compatibility. Neither class names
// nor codec fields are guessed from resource IDs. A batch shares one bounded VM;
// exhaustion fails the entire batch without returning partial predictions.
func ProbeBiomeSource(ctx context.Context, z *zip.Reader, options Options, className, codecField string, queries []BiomeProbeQuery) (result BiomeProbeResult, err error) {
	defer func() {
		if p := recover(); p != nil {
			result = BiomeProbeResult{}
			err = fmt.Errorf("biome source probe: %v", p)
		}
		if ctx.Err() != nil {
			result = BiomeProbeResult{}
			err = ctx.Err()
		}
	}()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if options.Minecraft == "" || len(queries) > 4096 {
		return result, fmt.Errorf("invalid biome probe version/query budget")
	}
	v, _, err := archiveMachine(ctx, z, options)
	if err != nil {
		return result, err
	}
	h := &biomeProbeBindings{host: v.host}
	v.host.extra = h
	codec := v.static(reference{className, codecField, "Lcom/mojang/serialization/MapCodec;"})
	recipe, ok := objectData(codec).(recordRecipe)
	if !ok {
		return result, fmt.Errorf("unsupported biome source codec")
	}
	args := make([]any, len(recipe.elements))
	for i, element := range recipe.elements {
		key, ok := objectData(element).(registryElement)
		if !ok {
			return result, fmt.Errorf("unsupported record element")
		}
		result.CodecBiomes = append(result.CodecBiomes, key.id)
		args[i] = v.host.symbol("holder:"+key.id, "net/minecraft/core/Holder", key.id)
	}
	source := v.callLambda(recipe.factory, args)
	if !v.instanceOf(source, biomeSourceClass) {
		return result, fmt.Errorf("codec did not construct a biome source")
	}
	for _, q := range queries {
		sampler := v.alloc(climateSamplerClass, q.Climate)
		x := v.method(reference{biomeSourceClass, "getNoiseBiome", "(IIIL" + climateSamplerClass + ";)Lnet/minecraft/core/Holder;"}, source, true, []any{q.X, q.Y, q.Z, sampler})
		id, ok := objectData(x).(string)
		if !ok || !strings.Contains(id, ":") {
			return BiomeProbeResult{}, fmt.Errorf("biome query returned an invalid holder")
		}
		result.Biomes = append(result.Biomes, id)
	}
	result.Steps = v.steps
	return result, nil
}

type registryElement struct{ id string }
type registryTag struct{ registry, id string }
type recordRecipe struct {
	elements []any
	factory  *lambda
}
type deferredRegistry struct{ namespace, registry string }
type deferredValue struct {
	registry, id string
	supplier     any
	owner        *object
	args         []any
}
type biomeProbeBindings struct {
	host       *bindings
	deferred   []deferredValue
	registries map[string]map[string]any
	attached   map[*object]bool
}

func (h *biomeProbeBindings) initializes(name string) bool {
	if name == neoDeferredRegister || name == neoDeferredRegister+"$Items" || name == neoDeferredRegister+"$Blocks" {
		return true
	}
	return name == biomeSourceClass || name == "net/minecraft/core/particles/SimpleParticleType" || name == "net/minecraft/advancements/critereon/SimpleCriterionTrigger" || name == "net/minecraft/world/level/storage/loot/functions/LootItemConditionalFunction" || name == "net/minecraft/world/level/levelgen/feature/Feature"
}
func (h *biomeProbeBindings) static(r reference) (any, bool) {
	if x, ok := h.platformValueStatic(r); ok {
		return x, true
	}
	if r.owner == "net/minecraft/world/item/ItemStack" && r.name == "EMPTY" && r.desc == "Lnet/minecraft/world/item/ItemStack;" {
		return h.host.symbol(fieldKey(r), r.owner, map[string]any{"count": int32(0)}), true
	}
	if x, ok := h.codecStatic(r); ok {
		return x, true
	}
	if r.owner == "net/minecraft/world/item/CreativeModeTabs" && r.desc == "Lnet/minecraft/resources/ResourceKey;" {
		switch r.name {
		case "BUILDING_BLOCKS", "COLORED_BLOCKS", "NATURAL_BLOCKS", "FUNCTIONAL_BLOCKS", "REDSTONE_BLOCKS", "HOTBAR", "SEARCH", "TOOLS_AND_UTILITIES", "COMBAT", "FOOD_AND_DRINKS", "INGREDIENTS", "SPAWN_EGGS", "OP_BLOCKS", "INVENTORY":
			id := "minecraft:" + strings.ToLower(r.name)
			return h.host.symbol("key:net/minecraft/core/registries/Registries.CREATIVE_MODE_TAB\x00"+id, "net/minecraft/resources/ResourceKey", id), true
		}
	}
	if r.owner == "net/neoforged/neoforge/registries/NeoForgeRegistries" || r.owner == "net/neoforged/neoforge/registries/NeoForgeRegistries$Keys" {
		keys := map[string]string{"ENTITY_DATA_SERIALIZERS": "entity_data_serializers", "GLOBAL_LOOT_MODIFIER_SERIALIZERS": "global_loot_modifier_serializers", "BIOME_MODIFIER_SERIALIZERS": "biome_modifier_serializers", "STRUCTURE_MODIFIER_SERIALIZERS": "structure_modifier_serializers", "FLUID_TYPES": "fluid_type", "HOLDER_SET_TYPES": "holder_set_type", "INGREDIENT_TYPES": "ingredient_serializer", "FLUID_INGREDIENT_TYPES": "fluid_ingredient_type", "CONDITION_CODECS": "condition_codecs", "CONDITION_SERIALIZERS": "condition_codecs", "ATTACHMENT_TYPES": "attachment_types", "BIOME_MODIFIERS": "biome_modifier", "STRUCTURE_MODIFIERS": "structure_modifier"}
		if key := keys[r.name]; key != "" && (r.desc == "Lnet/minecraft/core/Registry;" || r.desc == "Lnet/minecraft/resources/ResourceKey;") {
			if r.desc == "Lnet/minecraft/resources/ResourceKey;" {
				return h.host.registryKey("neoforge:" + key), true
			}
			return h.host.symbol(fieldKey(r), strings.TrimSuffix(r.desc[1:], ";"), "neoforge:"+key), true
		}
	}
	if r.owner == "net/minecraft/core/registries/BuiltInRegistries" && (r.desc == "Lnet/minecraft/core/Registry;" || r.desc == "Lnet/minecraft/core/DefaultedRegistry;") && r.name != "" {
		return h.host.symbol(fieldKey(r), "net/minecraft/core/Registry", "net/minecraft/core/registries/Registries."+r.name), true
	}
	if r.owner == "net/minecraft/world/item/Rarity" && r.desc == "Lnet/minecraft/world/item/Rarity;" && (r.name == "COMMON" || r.name == "UNCOMMON" || r.name == "RARE" || r.name == "EPIC") {
		return h.host.symbol(fieldKey(r), r.owner, r.name), true
	}
	if r.owner == "net/minecraft/world/item/Items" && r.desc == "Lnet/minecraft/world/item/Item;" {
		return h.host.symbol(fieldKey(r), "net/minecraft/world/item/Item", "minecraft:"+strings.ToLower(r.name)), true
	}
	if r.owner == "net/minecraft/world/level/biome/Biomes" && r.desc == "Lnet/minecraft/resources/ResourceKey;" {
		// Mojang named API fields use the resource path in upper case. Actual
		// registry presence must be checked by a future production codec host.
		id := "minecraft:" + strings.ToLower(r.name)
		return h.host.symbol("key:net/minecraft/core/registries/Registries.BIOME\x00"+id, "net/minecraft/resources/ResourceKey", id), true
	}
	return nil, false
}

func (h *biomeProbeBindings) call(r reference, receiver any, args []any) (any, bool) {
	if value, ok := h.deferredCall(r, receiver, args); ok {
		return value, true
	}
	if x, ok := h.platformValueCall(r, receiver, args); ok {
		return x, true
	}
	if x, ok := biomeMath(r, receiver, args); ok {
		return x, true
	}
	if x, ok := callbackCall(r, receiver, args); ok {
		return x, true
	}
	if x, ok := h.codecCall(r, receiver, args); ok {
		return x, true
	}
	v := h.host.vm
	if r.owner == "net/minecraft/core/Registry" && r.name == "key" && r.desc == "()Lnet/minecraft/resources/ResourceKey;" {
		return h.host.registryKey(objectData(receiver).(string)), true
	}
	if r.owner == "net/neoforged/neoforge/registries/RegisterEvent" || r.owner == "net/minecraftforge/registries/RegisterEvent" {
		key := objectData(receiver)
		if r.name == "getRegistryKey" && r.desc == "()Lnet/minecraft/resources/ResourceKey;" {
			return key, true
		}
		if r.name == "register" && (len(args) == 2 || len(args) == 3) {
			if objectData(key) != objectData(args[0]) {
				return nil, true
			}
			registry := objectData(key).(string)
			if len(args) == 3 {
				h.register(registry, objectData(args[1]).(string), v.callLambda(args[2].(*lambda), nil))
			} else {
				v.callLambda(args[1].(*lambda), []any{v.alloc(r.owner+"$RegisterHelper", registry)})
			}
			return nil, true
		}
	}
	if (r.owner == "net/neoforged/neoforge/registries/RegisterEvent$RegisterHelper" || r.owner == "net/minecraftforge/registries/RegisterEvent$RegisterHelper") && r.name == "register" && len(args) == 2 {
		var id string
		if x, ok := args[0].(string); ok {
			id = x
		} else {
			id = objectData(args[0]).(string)
		}
		if !strings.Contains(id, ":") {
			// RegisterHelper resolves bare names in the active mod namespace.
			namespace := h.host.options.ModID
			if namespace == "" {
				namespace = "minecraft"
			}
			id = namespace + ":" + id
		}
		h.register(objectData(receiver).(string), id, args[1])
		return nil, true
	}
	if r.owner == "net/minecraft/sounds/SoundEvent" && r.name == "createVariableRangeEvent" && r.desc == "(Lnet/minecraft/resources/ResourceLocation;)Lnet/minecraft/sounds/SoundEvent;" {
		return v.alloc(r.owner, map[string]any{"location": args[0]}), true
	}
	if r.owner == "net/minecraft/world/level/levelgen/feature/Feature" && r.name == "<init>" && r.desc == "(Lcom/mojang/serialization/Codec;)V" {
		receiver.(*object).data = args[0]
		return nil, true
	}
	if r.owner == "net/minecraft/world/level/storage/loot/functions/LootItemConditionalFunction" && r.name == "commonFields" && r.desc == "(Lcom/mojang/serialization/codecs/RecordCodecBuilder$Instance;)Lcom/mojang/datafixers/Products$P1;" {
		return v.alloc("com/mojang/datafixers/Products$P1", []any{v.alloc(recordBuilderClass, codecRecipe{"native", []any{r.owner, r.name}})}), true
	}
	if r.owner == "net/minecraft/world/level/storage/loot/functions/LootItemFunctionType" && r.name == "<init>" && r.desc == "(Lcom/mojang/serialization/MapCodec;)V" {
		receiver.(*object).data = args[0]
		return nil, true
	}
	if r.owner == "net/minecraft/world/item/crafting/RecipeType" {
		if r.name == "simple" && r.desc == "(Lnet/minecraft/resources/ResourceLocation;)Lnet/minecraft/world/item/crafting/RecipeType;" {
			return v.alloc(r.owner, objectData(args[0])), true
		}
		if r.name == "toString" && r.desc == "()Ljava/lang/String;" {
			return objectData(receiver), true
		}
	}
	if r.owner == "net/minecraft/world/item/ItemStack" && receiver != nil {
		if data, ok := objectData(receiver).(map[string]any); ok && data["count"] == int32(0) {
			switch r.name {
			case "getItem":
				return h.static(reference{"net/minecraft/world/item/Items", "AIR", "Lnet/minecraft/world/item/Item;"})
			case "getCount":
				return int32(0), true
			case "isEmpty":
				return int32(1), true
			case "getComponentsPatch":
				return h.codecStatic(reference{"net/minecraft/core/component/DataComponentPatch", "EMPTY", "Lnet/minecraft/core/component/DataComponentPatch;"})
			}
		}
	}
	if r.owner == "net/minecraft/core/Registry" && r.name == "getKey" && r.desc == "(Ljava/lang/Object;)Lnet/minecraft/resources/ResourceLocation;" {
		if o, isObject := args[0].(*object); !isObject || o == nil {
		} else if id, ok := o.data.(string); ok && strings.Contains(id, ":") {
			return v.alloc("net/minecraft/resources/ResourceLocation", id), true
		}
	}
	if r.owner == "net/minecraft/advancements/critereon/SimpleCriterionTrigger" && r.name == "<init>" && r.desc == "()V" {
		receiver.(*object).data = map[string]any{}
		return nil, true
	}
	if r.owner == "net/minecraft/core/component/DataComponentType" && r.name == "builder" && r.desc == "()Lnet/minecraft/core/component/DataComponentType$Builder;" {
		return v.alloc(r.owner+"$Builder", map[string]any{}), true
	}
	if r.owner == "net/minecraft/core/component/DataComponentType$Builder" {
		data := objectData(receiver).(map[string]any)
		if r.name == "persistent" && r.desc == "(Lcom/mojang/serialization/Codec;)Lnet/minecraft/core/component/DataComponentType$Builder;" || r.name == "networkSynchronized" && r.desc == "(Lnet/minecraft/network/codec/StreamCodec;)Lnet/minecraft/core/component/DataComponentType$Builder;" {
			data[r.name] = args[0]
			return receiver, true
		}
		if r.name == "cacheEncoding" && len(args) == 0 {
			data[r.name] = true
			return receiver, true
		}
		if r.name == "build" && r.desc == "()Lnet/minecraft/core/component/DataComponentType;" {
			if data["persistent"] == nil && data["networkSynchronized"] == nil {
				fail("component requires serialization")
			}
			copy := map[string]any{}
			for k, x := range data {
				copy[k] = x
			}
			return v.alloc("net/minecraft/core/component/DataComponentType", copy), true
		}
	}
	if r.owner == "net/neoforged/neoforge/attachment/AttachmentType" && (r.name == "serializable" || r.name == "builder") && (r.desc == "(Ljava/util/function/Supplier;)Lnet/neoforged/neoforge/attachment/AttachmentType$Builder;" || r.desc == "(Ljava/util/function/Function;)Lnet/neoforged/neoforge/attachment/AttachmentType$Builder;") {
		data := map[string]any{"factory": args[0], "factoryType": r.desc}
		if r.name == "serializable" {
			data["serializer"] = "nbt"
		}
		return v.alloc(r.owner+"$Builder", data), true
	}
	if r.owner == "net/neoforged/neoforge/attachment/AttachmentType$Builder" {
		data := objectData(receiver).(map[string]any)
		if r.name == "sync" && r.desc == "(Lnet/minecraft/network/codec/StreamCodec;)Lnet/neoforged/neoforge/attachment/AttachmentType$Builder;" {
			if args[0] == nil {
				fail("null attachment stream codec")
			}
			data["sync"] = args[0]
			return receiver, true
		}
		if r.name == "serialize" && r.desc == "(Lcom/mojang/serialization/Codec;)Lnet/neoforged/neoforge/attachment/AttachmentType$Builder;" {
			if data["serializer"] != nil || args[0] == nil {
				fail("invalid attachment serializer")
			}
			data["serializer"] = args[0]
			return receiver, true
		}
		if r.name == "copyOnDeath" && len(args) == 0 {
			if data["serializer"] == nil {
				fail("copyOnDeath requires a serializer")
			}
			data["copyOnDeath"] = true
			return receiver, true
		}
		if r.name == "build" && r.desc == "()Lnet/neoforged/neoforge/attachment/AttachmentType;" {
			copy := map[string]any{}
			for k, x := range data {
				copy[k] = x
			}
			return v.alloc("net/neoforged/neoforge/attachment/AttachmentType", copy), true
		}
	}
	if r.owner == "net/minecraft/network/chat/Component" && (r.name == "translatable" && r.desc == "(Ljava/lang/String;)Lnet/minecraft/network/chat/MutableComponent;" || r.name == "literal" && r.desc == "(Ljava/lang/String;)Lnet/minecraft/network/chat/MutableComponent;") {
		return v.alloc("net/minecraft/network/chat/MutableComponent", map[string]any{r.name: args[0]}), true
	}
	if r.owner == "net/minecraft/world/item/CreativeModeTab" && r.name == "builder" && r.desc == "()Lnet/minecraft/world/item/CreativeModeTab$Builder;" {
		return v.alloc(r.owner+"$Builder", map[string]any{}), true
	}
	if r.owner == "net/minecraft/world/item/CreativeModeTab$Builder" {
		data := objectData(receiver).(map[string]any)
		if (r.name == "withTabsBefore" || r.name == "withTabsAfter") && r.desc == "([Lnet/minecraft/resources/ResourceKey;)Lnet/minecraft/world/item/CreativeModeTab$Builder;" {
			data[r.name] = append([]any(nil), args[0].(*array).values...)
			return receiver, true
		}
		if len(args) == 1 && (r.name == "title" && r.desc == "(Lnet/minecraft/network/chat/Component;)Lnet/minecraft/world/item/CreativeModeTab$Builder;" || r.name == "icon" && r.desc == "(Ljava/util/function/Supplier;)Lnet/minecraft/world/item/CreativeModeTab$Builder;" || r.name == "displayItems" && r.desc == "(Lnet/minecraft/world/item/CreativeModeTab$DisplayItemsGenerator;)Lnet/minecraft/world/item/CreativeModeTab$Builder;") {
			data[r.name] = args[0]
			return receiver, true
		}
		if r.name == "build" && r.desc == "()Lnet/minecraft/world/item/CreativeModeTab;" {
			copy := map[string]any{}
			for k, x := range data {
				copy[k] = x
			}
			return v.alloc("net/minecraft/world/item/CreativeModeTab", copy), true
		}
	}
	if strings.HasPrefix(r.owner, "net/minecraft/tags/") && r.name == "create" {
		registry := ""
		var location any
		switch r.owner {
		case "net/minecraft/tags/FluidTags":
			registry = "net/minecraft/core/registries/Registries.FLUID"
		case "net/minecraft/tags/ItemTags":
			registry = "net/minecraft/core/registries/Registries.ITEM"
		case "net/minecraft/tags/BlockTags":
			registry = "net/minecraft/core/registries/Registries.BLOCK"
		}
		if registry != "" && r.desc == "(Lnet/minecraft/resources/ResourceLocation;)Lnet/minecraft/tags/TagKey;" {
			location = args[0]
		}
		if r.owner == "net/minecraft/tags/TagKey" && r.desc == "(Lnet/minecraft/resources/ResourceKey;Lnet/minecraft/resources/ResourceLocation;)Lnet/minecraft/tags/TagKey;" {
			registry = objectData(args[0]).(string)
			location = args[1]
		}
		if location != nil {
			id := objectData(location).(string)
			return h.host.symbol("tag:"+registry+"\x00"+id, "net/minecraft/tags/TagKey", registryTag{registry, id}), true
		}
	}
	if r.owner == "net/minecraft/world/item/crafting/ShapedRecipePattern" {
		state := h.host.symbol("crafting_size", r.owner, [2]int32{3, 3})
		size := state.data.([2]int32)
		switch {
		case r.name == "setCraftingSize" && r.desc == "(II)V":
			state.data = [2]int32{max(size[0], integer(args[0])), max(size[1], integer(args[1]))}
			return nil, true
		case r.name == "getMaxWidth" && r.desc == "()I":
			return size[0], true
		case r.name == "getMaxHeight" && r.desc == "()I":
			return size[1], true
		}
	}
	if r.owner == "net/minecraft/util/StringRepresentable" && r.name == "fromEnum" && r.desc == "(Ljava/util/function/Supplier;)Lnet/minecraft/util/StringRepresentable$EnumCodec;" {
		values := v.callLambda(args[0].(*lambda), nil).(*array)
		names := map[string]any{}
		for _, value := range values.values {
			name := v.method(reference{r.owner, "getSerializedName", "()Ljava/lang/String;"}, value, true, nil).(string)
			if _, exists := names[name]; exists {
				fail("duplicate enum serialized name")
			}
			names[name] = value
		}
		return v.alloc(r.owner+"$EnumCodec", names), true
	}
	if r.owner == "net/minecraft/core/Registry" && r.name == "register" && len(args) == 3 {
		var id string
		if s, ok := args[1].(string); ok {
			id = s
		} else {
			id, _ = objectData(args[1]).(string)
		}
		if !strings.Contains(id, ":") {
			id = "minecraft:" + id
		}
		return h.register(objectData(args[0]).(string), id, args[2]), true
	}
	if (r.owner == "net/neoforged/neoforge/registries/DeferredRegister" || r.owner == "net/minecraftforge/registries/DeferredRegister" || strings.HasPrefix(r.owner, "net/neoforged/neoforge/registries/DeferredRegister$") || strings.HasPrefix(r.owner, "net/minecraftforge/registries/DeferredRegister$")) && r.name == "register" && r.desc == "(Lnet/minecraftforge/eventbus/api/IEventBus;)V" {
		owner := receiver.(*object)
		if h.attached == nil {
			h.attached = map[*object]bool{}
		}
		if h.attached[owner] {
			fail("deferred register already attached")
		}
		h.attached[owner] = true
		if len(h.host.listeners) >= 1024 {
			fail("event listener budget exceeded")
		}
		event := strings.Split(r.owner, "$")[0]
		event = strings.TrimSuffix(event, "DeferredRegister") + "RegisterEvent"
		h.host.listeners = append(h.host.listeners, eventListener{event: "L" + event + ";", priority: 2, modID: h.host.options.ModID, native: func(event any) { h.registerDeferred(owner, objectData(objectData(event)).(string)) }})
		return nil, true
	}
	// Deferred item declarations may share a mod's static initializer with
	// registry keys. Keep the declaration opaque; attempting to use an item
	// or its properties still fails unless that API is explicitly implemented.
	if r.owner == "net/minecraft/world/item/Item$Properties" && r.name == "<init>" && r.desc == "()V" {
		receiver.(*object).data = map[string][]any{}
		return nil, true
	}
	if r.owner == "net/minecraft/world/item/Item$Properties" && r.name == "stacksTo" && r.desc == "(I)Lnet/minecraft/world/item/Item$Properties;" {
		objectData(receiver).(map[string][]any)[r.name] = append([]any(nil), args...)
		return receiver, true
	}
	if r.owner == "net/minecraft/world/item/Item$Properties" && r.name == "rarity" && r.desc == "(Lnet/minecraft/world/item/Rarity;)Lnet/minecraft/world/item/Item$Properties;" {
		objectData(receiver).(map[string][]any)[r.name] = append([]any(nil), args...)
		return receiver, true
	}
	if r.owner == "net/minecraft/world/item/Item$Properties" && (r.name == "craftRemainder" && r.desc == "(Lnet/minecraft/world/item/Item;)Lnet/minecraft/world/item/Item$Properties;" || r.name == "food" && r.desc == "(Lnet/minecraft/world/food/FoodProperties;)Lnet/minecraft/world/item/Item$Properties;") {
		objectData(receiver).(map[string][]any)[r.name] = append([]any(nil), args...)
		return receiver, true
	}
	if r.owner == "net/minecraft/world/food/FoodProperties$Builder" {
		if r.name == "<init>" && r.desc == "()V" {
			receiver.(*object).data = map[string][]any{}
			return nil, true
		}
		if r.name == "nutrition" && r.desc == "(I)Lnet/minecraft/world/food/FoodProperties$Builder;" || r.name == "saturationModifier" && r.desc == "(F)Lnet/minecraft/world/food/FoodProperties$Builder;" {
			objectData(receiver).(map[string][]any)[r.name] = append([]any(nil), args...)
			return receiver, true
		}
		if r.name == "alwaysEdible" && r.desc == "()Lnet/minecraft/world/food/FoodProperties$Builder;" {
			objectData(receiver).(map[string][]any)[r.name] = []any{int32(1)}
			return receiver, true
		}
		if r.name == "effect" && r.desc == "(Ljava/util/function/Supplier;F)Lnet/minecraft/world/food/FoodProperties$Builder;" {
			// NeoForge retains the supplier in a PossibleEffect record. Its
			// execution belongs to effect(), not builder registration or build().
			props := objectData(receiver).(map[string][]any)
			v.reserveCollection(2)
			props["effects"] = append(props["effects"], args...)
			return receiver, true
		}
		if r.name == "build" && r.desc == "()Lnet/minecraft/world/food/FoodProperties;" {
			props := map[string][]any{}
			for k, a := range objectData(receiver).(map[string][]any) {
				props[k] = append([]any(nil), a...)
			}
			return v.alloc("net/minecraft/world/food/FoodProperties", props), true
		}
	}
	if r.name == "createMobBucket" && r.desc == "(Ljava/util/function/Supplier;Ljava/util/function/Supplier;Ljava/util/function/Supplier;)Ljava/util/function/Supplier;" {
		return &lambda{kind: 6, target: reference{"net/minecraft/world/item/Item", "<init>", "(Lnet/minecraft/world/item/Item$Properties;)V"}, captured: nil}, true
	}
	if r.owner == "net/neoforged/neoforge/registries/DeferredRegister$Items" && r.name == "registerSimpleItem" && r.desc == "(Ljava/lang/String;Lnet/minecraft/world/item/Item$Properties;)Lnet/neoforged/neoforge/registries/DeferredItem;" {
		reg := objectData(receiver).(deferredRegistry)
		entry := deferredValue{registry: reg.registry, id: reg.namespace + ":" + args[0].(string), supplier: &lambda{kind: 8, target: reference{"net/minecraft/world/item/Item", "<init>", "(Lnet/minecraft/world/item/Item$Properties;)V"}}, owner: receiver.(*object), args: []any{args[1]}}
		h.deferValue(entry)
		return v.alloc("net/neoforged/neoforge/registries/DeferredItem", entry), true
	}
	if r.owner == "net/neoforged/neoforge/registries/DeferredRegister$Items" && r.name == "registerItem" && r.desc == "(Ljava/lang/String;Ljava/util/function/Function;Lnet/minecraft/world/item/Item$Properties;)Lnet/neoforged/neoforge/registries/DeferredItem;" {
		reg := objectData(receiver).(deferredRegistry)
		entry := deferredValue{registry: reg.registry, id: reg.namespace + ":" + args[0].(string), supplier: asSupplier(args[1]), owner: receiver.(*object), args: []any{args[2]}}
		h.deferValue(entry)
		return v.alloc("net/neoforged/neoforge/registries/DeferredItem", entry), true
	}
	if o, ok := receiver.(*object); ok && o != nil && r.name != "<init>" {
		switch o.data.(type) {
		case deferredRegistry, deferredValue:
			// Native-owned registry state: framework bytecode would read Java
			// fields this object never had. Unhandled queries see an opaque
			// game value; decisions on it fail closed.
			if _, handled := h.nativeDeferredQuery(r, o); handled {
				break
			}
			if e, ok := o.data.(deferredValue); ok && r.name == "getId" && r.desc == "()Lnet/minecraft/resources/ResourceLocation;" {
				// DeferredHolder.getId is key.location(): the registered name.
				ns, path, found := strings.Cut(e.id, ":")
				if !found {
					fail("unbound deferred holder id %s", e.id)
				}
				return v.method(reference{"net/minecraft/resources/ResourceLocation", "fromNamespaceAndPath", "(Ljava/lang/String;Ljava/lang/String;)Lnet/minecraft/resources/ResourceLocation;"}, nil, false, []any{ns, path}), true
			}
			if r.name != "register" && r.name != "createHolder" && r.name != "get" && r.name != "getId" && r.name != "getKey" && r.name != "equals" && r.name != "hashCode" && r.name != "toString" {
				_, ret := descriptor(r.desc)
				switch {
				case ret == "V":
					return nil, true
				case strings.HasPrefix(ret, "L") || strings.HasPrefix(ret, "["):
					return h.host.vm.inert(r.owner+"."+r.name+" result", true), true
				default:
					return defaultValue(ret), true
				}
			}
		}
	}
	// A mod-defined NeoForge registry: the builder's options (sync, max id,
	// default key, callbacks) only matter to networking and runtime lookups,
	// so create() yields the registry identified by its key, exactly the
	// handle DeferredRegister and RegisterEvent already resolve by key.
	if r.owner == "net/neoforged/neoforge/registries/RegistryBuilder" && receiver != nil {
		if r.name == "<init>" && r.desc == "(Lnet/minecraft/resources/ResourceKey;)V" {
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "registryKey")
			}
			id, ok := objectData(args[0]).(string)
			if !ok {
				fail("unbound registry key")
			}
			receiver.(*object).data = id
			return nil, true
		}
		id, ok := objectData(receiver).(string)
		if !ok {
			fail("unbound registry builder")
		}
		if r.name == "create" && r.desc == "()Lnet/minecraft/core/Registry;" {
			return h.host.symbol("custom-registry:"+id, "net/minecraft/core/Registry", id), true
		}
		if _, ret := descriptor(r.desc); ret == "L"+r.owner+";" {
			return receiver, true
		}
	}
	if r.owner == "net/neoforged/neoforge/registries/DeferredRegister" || r.owner == "net/minecraftforge/registries/DeferredRegister" {
		if r.name == "create" && r.desc == "(Lnet/minecraft/core/Registry;Ljava/lang/String;)L"+r.owner+";" {
			return v.alloc(r.owner, deferredRegistry{args[1].(string), objectData(args[0]).(string)}), true
		}
		if (r.name == "createItems" || r.name == "createBlocks") && len(args) == 1 {
			kind, registry := "$Items", "ITEM"
			if r.name == "createBlocks" {
				kind, registry = "$Blocks", "BLOCK"
			}
			return v.alloc(r.owner+kind, deferredRegistry{args[0].(string), "net/minecraft/core/registries/Registries." + registry}), true
		}
		if r.name == "create" && len(args) == 2 && r.desc == "(Lnet/minecraft/resources/ResourceKey;Ljava/lang/String;)L"+r.owner+";" {
			return v.alloc(r.owner, deferredRegistry{args[1].(string), objectData(args[0]).(string)}), true
		}
		if r.name == "create" && r.desc == "(Lnet/minecraft/resources/ResourceLocation;Ljava/lang/String;)L"+r.owner+";" {
			if i := inertOf(args[0]); i != nil {
				// A registry named by a game value (a ResourceKey.location() of
				// a game registry key) is a content registry; worldgen registry
				// keys are framework symbols and never inert.
				return v.alloc(r.owner, deferredRegistry{args[1].(string), "unresolved:" + i.origin}), true
			}
			id := objectData(args[0]).(string)
			name := strings.TrimPrefix(strings.TrimPrefix(id, "minecraft:"), "worldgen/")
			symbol := "net/minecraft/core/registries/Registries." + strings.ToUpper(name)
			if nativeRegistryLocation(symbol) != id {
				symbol = id
			}
			return v.alloc(r.owner, deferredRegistry{args[1].(string), symbol}), true
		}
		if ro, _ := receiver.(*object); ro != nil && r.name == "register" && len(args) == 2 && strings.HasPrefix(r.desc, "(Ljava/lang/String;Ljava/util/function/Supplier;)") && isDeferredRegistry(ro.data) {
			reg := objectData(receiver).(deferredRegistry)
			entry := deferredValue{registry: reg.registry, id: reg.namespace + ":" + args[0].(string), supplier: asSupplier(args[1]), owner: receiver.(*object)}
			h.deferValue(entry)
			return v.alloc(strings.TrimSuffix(strings.Split(r.desc, ")L")[1], ";"), entry), true
		}
	}
	if r.owner == "net/minecraft/resources/RegistryOps" && r.name == "retrieveElement" && len(args) == 1 {
		return v.alloc(recordBuilderClass, registryElement{objectData(args[0]).(string)}), true
	}
	if r.owner == "net/minecraft/world/level/levelgen/feature/stateproviders/NoiseBasedStateProvider" && r.name == "noiseCodec" && strings.HasPrefix(r.desc, "(Lcom/mojang/serialization/codecs/RecordCodecBuilder$Instance;)") {
		return v.alloc("com/mojang/datafixers/Products$P3", []any{
			v.alloc(recordBuilderClass, codecRecipe{"native", []any{"noise", "seed"}}),
			v.alloc(recordBuilderClass, codecRecipe{"native", []any{"noise", "noise"}}),
			v.alloc(recordBuilderClass, codecRecipe{"native", []any{"noise", "scale"}}),
		}), true
	}
	if r.owner == recordBuilderClass && (r.name == "mapCodec" || r.name == "create") && len(args) == 1 {
		builder := v.alloc(recordBuilderClass+"$Instance", nil)
		x := v.callLambda(args[0].(*lambda), []any{builder})
		kind := "com/mojang/serialization/MapCodec"
		if r.name == "create" {
			kind = "com/mojang/serialization/Codec"
		}
		if _, opaque := x.(*inertValue); opaque {
			// The recipe was built from game-owned codec pieces; keep the codec
			// opaque so storing it works and any decode through it fails closed.
			return v.inert(kind+" from opaque record recipe", true), true
		}
		return v.alloc(kind, objectData(x)), true
	}
	if r.owner == recordBuilderClass+"$Instance" {
		switch r.name {
		case "group":
			if len(args) < 1 || len(args) > 16 {
				fail("record codec arity limit")
			}
			return v.alloc(fmt.Sprintf("com/mojang/datafixers/Products$P%d", len(args)), append([]any(nil), args...)), true
		case "stable":
			if len(args) == 1 {
				return v.alloc("com/mojang/datafixers/kinds/App", args[0]), true
			}
		}
	}
	if strings.HasPrefix(r.owner, "com/mojang/datafixers/Products$P") && r.name == "apply" && len(args) == 2 {
		f, ok := args[1].(*lambda)
		if !ok {
			f, ok = objectData(args[1]).(*lambda)
		}
		if !ok {
			fail("unsupported record codec factory")
		}
		return v.alloc("com/mojang/datafixers/kinds/App", recordRecipe{objectData(receiver).([]any), f}), true
	}
	if strings.HasPrefix(r.owner, "com/mojang/datafixers/Products$P") && r.name == "and" && len(args) == 1 {
		values := append([]any(nil), objectData(receiver).([]any)...)
		if more, ok := objectData(args[0]).([]any); ok {
			values = append(values, more...)
		} else {
			values = append(values, args[0])
		}
		if len(values) > 16 {
			fail("record codec arity limit")
		}
		return v.alloc(fmt.Sprintf("com/mojang/datafixers/Products$P%d", len(values)), values), true
	}
	if r.owner == biomeSourceClass && r.name == "<init>" && r.desc == "()V" {
		return nil, true
	}
	if r.owner == climateSamplerClass && r.desc == "()L"+densityClass+";" {
		for i, name := range []string{"temperature", "humidity", "continentalness", "erosion", "depth", "weirdness"} {
			if name == r.name {
				if input, ok := objectData(receiver).(NoiseInput); ok {
					return v.alloc(densityClass, densityInput{input, i}), true
				}
				return v.alloc(densityClass, objectData(receiver).([6]float64)[i]), true
			}
		}
	}
	if r.owner == densityClass+"$SinglePointContext" && r.name == "<init>" && r.desc == "(III)V" {
		receiver.(*object).data = [3]int32{integer(args[0]), integer(args[1]), integer(args[2])}
		return nil, true
	}
	if r.owner == densityClass && r.name == "compute" && r.desc == "(L"+densityClass+"$FunctionContext;)D" {
		if input, ok := objectData(receiver).(densityInput); ok {
			p := objectData(args[0]).([3]int32)
			value, valid := input.router.Compute(input.channel, p[0], p[1], p[2])
			if !valid {
				fail("invalid custom biome density input")
			}
			return value, true
		}
		return objectData(receiver).(float64), true
	}
	if r.owner == "java/lang/Math" && r.name == "floorDiv" && r.desc == "(II)I" {
		a, b := integer(args[0]), integer(args[1])
		if b == 0 {
			raiseJava("java/lang/ArithmeticException", "/ by zero")
		}
		q := a / b
		if a%b != 0 && (a^b) < 0 {
			q--
		}
		return q, true
	}
	if r.owner == "net/minecraft/core/QuartPos" && r.desc == "(I)I" {
		if r.name == "fromBlock" {
			return integer(args[0]) >> 2, true
		}
		if r.name == "toBlock" {
			return integer(args[0]) << 2, true
		}
	}
	if r.owner == "net/minecraft/core/SectionPos" && r.name == "blockToSectionCoord" && r.desc == "(I)I" {
		return integer(args[0]) >> 4, true
	}
	if r.owner == "java/util/stream/Stream" && r.name == "of" && r.desc == "([Ljava/lang/Object;)Ljava/util/stream/Stream;" {
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "Stream.of")
		}
		return v.alloc(r.owner, &javaList{values: append([]any(nil), args[0].(*array).values...), readonly: true}), true
	}
	if r.owner == "java/util/stream/Collectors" && r.name == "toUnmodifiableSet" && len(args) == 0 {
		return v.alloc("java/util/stream/Collector", "unmodifiableSet"), true
	}
	if r.owner == "java/util/stream/Stream" && r.name == "collect" && len(args) == 1 && objectData(args[0]) == "unmodifiableSet" {
		var unique []any
		for _, x := range objectData(receiver).(*javaList).values {
			if x == nil {
				fail("null element in unmodifiable set")
			}
			found := false
			for _, y := range unique {
				v.tick()
				if equal(x, y) {
					found = true
					break
				}
			}
			if !found {
				unique = append(unique, x)
			}
		}
		return v.alloc("java/util/Set", unique), true
	}
	return nil, false
}

func (h *biomeProbeBindings) nativeDeferredQuery(r reference, o *object) (any, bool) {
	return nil, false
}

func isDeferredRegistry(x any) bool {
	_, ok := x.(deferredRegistry)
	return ok
}
