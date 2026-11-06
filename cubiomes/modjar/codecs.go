package modjar

import "strings"

// Codecs remain executable descriptions until a registry value is decoded.
// Factory/getter callbacks are retained, not evaluated with guessed inputs.
type codecRecipe struct {
	kind string
	args []any
}

func (h *biomeProbeBindings) codecStatic(r reference) (any, bool) {
	if r.owner == "net/minecraft/util/ExtraCodecs" && r.name == "POSITIVE_FLOAT" && r.desc == "Lcom/mojang/serialization/Codec;" {
		return h.host.symbol(fieldKey(r), "com/mojang/serialization/Codec", codecRecipe{"native", []any{r.owner, r.name}}), true
	}
	if r.owner == "net/minecraft/core/component/DataComponentPatch" && r.name == "EMPTY" && r.desc == "Lnet/minecraft/core/component/DataComponentPatch;" {
		return h.host.symbol(fieldKey(r), r.owner, map[string]any{}), true
	}
	if r.desc == "Lcom/mojang/serialization/Codec;" || r.desc == "Lnet/minecraft/network/codec/StreamCodec;" {
		fields := map[string][]string{
			"net/minecraft/world/level/levelgen/feature/configurations/NoneFeatureConfiguration": {"CODEC"},
			"net/minecraft/world/level/storage/loot/providers/number/NumberProviders":            {"CODEC"},
			"net/minecraft/core/component/DataComponentPatch":                                    {"CODEC", "STREAM_CODEC"},
			"net/minecraft/resources/ResourceLocation":                                           {"CODEC", "STREAM_CODEC"},
			"net/minecraft/world/item/ItemStack":                                                 {"CODEC", "OPTIONAL_CODEC", "STRICT_CODEC", "SINGLE_ITEM_CODEC", "STRICT_SINGLE_ITEM_CODEC", "STREAM_CODEC", "OPTIONAL_STREAM_CODEC"},
			"net/minecraft/world/item/crafting/Ingredient":                                       {"CODEC", "CODEC_NONEMPTY", "CONTENTS_STREAM_CODEC"},
		}
		for _, field := range fields[r.owner] {
			if field == r.name {
				return h.host.symbol(fieldKey(r), strings.TrimSuffix(r.desc[1:], ";"), codecRecipe{"native", []any{r.owner, r.name}}), true
			}
		}
	}
	if r.owner == "net/neoforged/neoforge/fluids/FluidStack" && (r.name == "FLUID_NON_EMPTY_CODEC" || r.name == "CODEC" || r.name == "OPTIONAL_CODEC" || r.name == "STREAM_CODEC" || r.name == "OPTIONAL_STREAM_CODEC") && (r.desc == "Lcom/mojang/serialization/Codec;" || r.desc == "Lnet/minecraft/network/codec/StreamCodec;") {
		return h.host.symbol(fieldKey(r), strings.TrimSuffix(r.desc[1:], ";"), codecRecipe{"native", []any{r.owner, r.name}}), true
	}
	if r.owner == "net/minecraft/util/ExtraCodecs" && r.desc == "Lcom/mojang/serialization/Codec;" && (r.name == "POSITIVE_INT" || r.name == "NON_NEGATIVE_INT") {
		lo := int32(0)
		if r.name == "POSITIVE_INT" {
			lo = 1
		}
		return h.host.symbol(fieldKey(r), "com/mojang/serialization/Codec", codecRecipe{"intRange", []any{lo, int32(2147483647)}}), true
	}
	if r.owner == "net/neoforged/neoforge/fluids/crafting/FluidIngredient" && r.name == "MAP_CODEC_NONEMPTY" && r.desc == "Lcom/mojang/serialization/MapCodec;" {
		return h.host.symbol(fieldKey(r), "com/mojang/serialization/MapCodec", codecRecipe{"native", []any{r.owner, r.name}}), true
	}
	if r.owner == "net/minecraft/world/item/component/ItemAttributeModifiers" && (r.name == "CODEC" && r.desc == "Lcom/mojang/serialization/Codec;" || r.name == "STREAM_CODEC" && r.desc == "Lnet/minecraft/network/codec/StreamCodec;") {
		return h.host.symbol(fieldKey(r), strings.TrimSuffix(r.desc[1:], ";"), codecRecipe{"native", []any{r.owner, r.name}}), true
	}
	if r.owner == "net/minecraft/network/codec/ByteBufCodecs" && r.desc == "Lnet/minecraft/network/codec/StreamCodec;" {
		switch r.name {
		case "BOOL", "BYTE", "SHORT", "INT", "VAR_INT", "VAR_LONG", "LONG", "FLOAT", "DOUBLE", "STRING_UTF8":
			return h.host.symbol(fieldKey(r), "net/minecraft/network/codec/StreamCodec", codecRecipe{r.name, nil}), true
		}
	}
	if r.owner == "com/mojang/serialization/Codec" && (r.desc == "Lcom/mojang/serialization/Codec;" || r.desc == "Lcom/mojang/serialization/codecs/PrimitiveCodec;") {
		switch r.name {
		case "BOOL", "BYTE", "SHORT", "INT", "LONG", "FLOAT", "DOUBLE", "STRING":
			return h.host.symbol(fieldKey(r), r.owner, codecRecipe{r.name, nil}), true
		}
	}
	// generic minecraft codec statics: absent game JAR fields stay descriptive stubs
	if (r.desc == "Lcom/mojang/serialization/Codec;" || r.desc == "Lcom/mojang/serialization/MapCodec;" || r.desc == "Lcom/mojang/serialization/codecs/PrimitiveCodec;" || r.desc == "Lnet/minecraft/network/codec/StreamCodec;") && (strings.HasPrefix(r.owner, "net/minecraft/") || strings.HasPrefix(r.owner, "net/neoforged/") || strings.HasPrefix(r.owner, "net/minecraftforge/")) {
		return h.host.symbol(fieldKey(r), strings.TrimSuffix(r.desc[1:], ";"), codecRecipe{"native", []any{r.owner, r.name}}), true
	}
	return nil, false
}

func (h *biomeProbeBindings) codecCall(r reference, receiver any, args []any) (any, bool) {
	v := h.host.vm
	if x, ok := h.dynamicCodecCall(r, receiver, args); ok {
		return x, true
	}
	if r.owner == "net/minecraft/resources/ResourceKey" && r.name == "codec" && r.desc == "(Lnet/minecraft/resources/ResourceKey;)Lcom/mojang/serialization/Codec;" {
		return v.alloc("com/mojang/serialization/Codec", codecRecipe{"resourceKey", []any{args[0]}}), true
	}
	if r.owner == "net/minecraft/network/codec/StreamCodec" && r.name == "of" && r.desc == "(Lnet/minecraft/network/codec/StreamEncoder;Lnet/minecraft/network/codec/StreamDecoder;)Lnet/minecraft/network/codec/StreamCodec;" {
		return v.alloc(r.owner, codecRecipe{"of", append([]any(nil), args...)}), true
	}
	if r.owner == "net/minecraft/util/ExtraCodecs" && r.name == "intRange" && r.desc == "(II)Lcom/mojang/serialization/Codec;" {
		return v.alloc("com/mojang/serialization/Codec", codecRecipe{"intRange", append([]any(nil), args...)}), true
	}
	if r.owner == "net/minecraft/network/codec/ByteBufCodecs" && r.name == "registry" && r.desc == "(Lnet/minecraft/resources/ResourceKey;)Lnet/minecraft/network/codec/StreamCodec;" {
		return v.alloc("net/minecraft/network/codec/StreamCodec", codecRecipe{"registry", append([]any(nil), args...)}), true
	}
	if r.owner == "net/minecraft/network/codec/StreamCodec" && r.name == "composite" && len(args) >= 3 && len(args) <= 17 && len(args)%2 == 1 {
		return v.alloc(r.owner, codecRecipe{"composite", append([]any(nil), args...)}), true
	}
	if r.owner == "net/minecraft/tags/TagKey" && r.name == "codec" && r.desc == "(Lnet/minecraft/resources/ResourceKey;)Lcom/mojang/serialization/Codec;" {
		return v.alloc("com/mojang/serialization/Codec", codecRecipe{"tag", append([]any(nil), args...)}), true
	}
	if r.owner == "net/neoforged/neoforge/common/util/NeoForgeExtraCodecs" && r.name == "optionalFieldAlwaysWrite" && r.desc == "(Lcom/mojang/serialization/Codec;Ljava/lang/String;Ljava/lang/Object;)Lcom/mojang/serialization/MapCodec;" {
		return v.alloc("com/mojang/serialization/MapCodec", codecRecipe{"optionalAlwaysWrite", append([]any(nil), args...)}), true
	}
	if (r.owner == "net/minecraft/core/Registry" || r.owner == "net/minecraft/core/DefaultedRegistry") && r.name == "byNameCodec" && r.desc == "()Lcom/mojang/serialization/Codec;" {
		return v.alloc("com/mojang/serialization/Codec", codecRecipe{"registry", []any{objectData(receiver)}}), true
	}
	if r.owner == "net/minecraft/network/codec/StreamCodec" && r.name == "map" && r.desc == "(Ljava/util/function/Function;Ljava/util/function/Function;)Lnet/minecraft/network/codec/StreamCodec;" {
		return v.alloc(r.owner, codecRecipe{"map", []any{receiver, args[0], args[1]}}), true
	}
	if r.owner == "com/mojang/serialization/Codec" || r.owner == "com/mojang/serialization/MapCodec" {
		_, ret := descriptor(r.desc)
		class := strings.TrimSuffix(strings.TrimPrefix(ret, "L"), ";")
		switch r.name {
		case "either", "mapEither", "withAlternative":
			if receiver == nil && len(args) == 2 {
				return v.alloc(class, codecRecipe{r.name, append([]any(nil), args...)}), true
			}
		case "lazyInitialized", "unit":
			if receiver == nil && len(args) == 1 {
				return v.alloc(class, codecRecipe{r.name, append([]any(nil), args...)}), true
			}
		case "comapFlatMap", "flatXmap", "flatComapMap":
			if len(args) == 2 {
				return v.alloc(class, codecRecipe{r.name, []any{receiver, args[0], args[1]}}), true
			}
		case "validate":
			if len(args) == 1 {
				return v.alloc(class, codecRecipe{r.name, []any{receiver, args[0]}}), true
			}
		case "intRange", "floatRange", "doubleRange":
			if receiver == nil && len(args) == 2 {
				return v.alloc(class, codecRecipe{r.name, append([]any(nil), args...)}), true
			}
		case "fieldOf":
			if len(args) == 1 {
				return v.alloc(class, codecRecipe{"field", []any{receiver, args[0]}}), true
			}
		case "optionalFieldOf":
			if len(args) == 1 || len(args) == 2 {
				return v.alloc(class, codecRecipe{"optional", append([]any{receiver}, args...)}), true
			}
		case "xmap":
			if len(args) == 2 {
				return v.alloc(class, codecRecipe{"xmap", []any{receiver, args[0], args[1]}}), true
			}
		case "listOf":
			if len(args) == 0 {
				return v.alloc(class, codecRecipe{"list", []any{receiver}}), true
			}
		case "forGetter":
			if len(args) == 1 {
				return v.alloc(recordBuilderClass, codecRecipe{"getter", []any{receiver, args[0]}}), true
			}
		case "codec":
			if r.owner == "com/mojang/serialization/MapCodec" && len(args) == 0 {
				return v.alloc(class, objectData(receiver)), true
			}
		}
	}
	return nil, false
}
