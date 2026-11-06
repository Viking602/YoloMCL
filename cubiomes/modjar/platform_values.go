package modjar

import "strings"

// These are game API constants, shared by every mod. Methods requiring the
// registries' contents remain unbound until those inputs are available.
func (h *biomeProbeBindings) platformValueStatic(r reference) (any, bool) {
	if r.owner == "net/minecraft/world/entity/npc/VillagerType" && r.name == "BY_BIOME" && r.desc == "Ljava/util/Map;" {
		return h.host.symbol(fieldKey(r), "java/util/HashMap", &javaMap{baseline: fieldKey(r)}), true
	}
	if r.desc != "L"+r.owner+";" {
		return nil, false
	}
	var names string
	switch r.owner {
	case "net/minecraft/world/entity/MobCategory":
		names = "MONSTER CREATURE AMBIENT AXOLOTLS UNDERGROUND_WATER_CREATURE WATER_CREATURE WATER_AMBIENT MISC"
	case "net/minecraft/world/entity/npc/VillagerType":
		names = "DESERT JUNGLE PLAINS SAVANNA SNOW SWAMP TAIGA"
	default:
		return nil, false
	}
	for _, name := range strings.Fields(names) {
		if r.name == name {
			return h.host.symbol(fieldKey(r), r.owner, r.name), true
		}
	}
	return nil, false
}

// Registry keys have a stable symbol identity and a separate serialized ID.
// In particular worldgen registries do not use their field's lower-case name.
func nativeRegistryLocation(symbol string) string {
	name, ok := strings.CutPrefix(symbol, "net/minecraft/core/registries/Registries.")
	if !ok {
		return ""
	}
	switch name {
	case "BIOME", "BIOME_SOURCE", "FEATURE", "CONFIGURED_FEATURE", "PLACED_FEATURE", "DENSITY_FUNCTION", "NOISE", "NOISE_SETTINGS", "CHUNK_GENERATOR", "STRUCTURE", "STRUCTURE_TYPE", "STRUCTURE_SET", "TEMPLATE_POOL", "PROCESSOR_LIST", "WORLD_PRESET", "FLAT_LEVEL_GENERATOR_PRESET":
		return "minecraft:worldgen/" + strings.ToLower(name)
	case "ITEM", "BLOCK", "FLUID", "DATA_COMPONENT_TYPE", "CREATIVE_MODE_TAB", "RECIPE_TYPE", "RECIPE_SERIALIZER", "PARTICLE_TYPE", "SOUND_EVENT", "ENTITY_TYPE", "BLOCK_ENTITY_TYPE", "MOB_EFFECT", "ATTRIBUTE", "POTION", "ENCHANTMENT", "DIMENSION_TYPE", "DIMENSION":
		return "minecraft:" + strings.ToLower(name)
	}
	return ""
}

func (h *biomeProbeBindings) platformValueCall(r reference, receiver any, args []any) (any, bool) {
	if r.owner == "net/neoforged/neoforge/common/world/chunk/TicketController" {
		if r.name == "<init>" && (r.desc == "(Lnet/minecraft/resources/ResourceLocation;)V" || r.desc == "(Lnet/minecraft/resources/ResourceLocation;Lnet/neoforged/neoforge/common/world/chunk/LoadingValidationCallback;)V") {
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "null ticket controller ID")
			}
			var callback any
			if len(args) == 2 {
				callback = args[1]
			}
			receiver.(*object).data = javaPair{args[0], callback}
			return nil, true
		}
		if r.name == "id" && r.desc == "()Lnet/minecraft/resources/ResourceLocation;" {
			return objectData(receiver).(javaPair).left, true
		}
		if r.name == "callback" && r.desc == "()Lnet/neoforged/neoforge/common/world/chunk/LoadingValidationCallback;" {
			return objectData(receiver).(javaPair).right, true
		}
	}
	if r.owner == "net/minecraft/core/particles/SimpleParticleType" && r.name == "<init>" && r.desc == "(Z)V" {
		receiver.(*object).data = args[0]
		return nil, true
	}
	// BlockSetType / WoodType only need their registered names during mod
	// bootstrap; sound/animation fields stay unbound.
	if r.owner == "net/minecraft/world/level/block/state/properties/BlockSetType" {
		// All BlockSetType constructors take the registered name first; sound and
		// sensitivity arguments are host game values and stay unused for worldgen.
		if r.name == "<init>" && strings.HasPrefix(r.desc, "(Ljava/lang/String;") && strings.HasSuffix(r.desc, ")V") {
			receiver.(*object).data = javaString(args[0])
			return nil, true
		}
		if r.name == "name" && r.desc == "()Ljava/lang/String;" {
			s, ok := objectData(receiver).(string)
			if !ok || s == "" {
				fail("BlockSetType name was not captured")
			}
			return s, true
		}
		if r.name == "register" && r.desc == "(Lnet/minecraft/world/level/block/state/properties/BlockSetType;)Lnet/minecraft/world/level/block/state/properties/BlockSetType;" {
			return args[0], true
		}
	}
	if r.owner == "net/minecraft/world/level/block/state/properties/WoodType" {
		if r.name == "<init>" && strings.HasPrefix(r.desc, "(Ljava/lang/String;") && strings.HasSuffix(r.desc, ")V") {
			receiver.(*object).data = javaPair{javaString(args[0]), args[1]}
			return nil, true
		}
		if r.name == "name" && r.desc == "()Ljava/lang/String;" {
			s, ok := objectData(receiver).(javaPair).left.(string)
			if !ok || s == "" {
				fail("WoodType name was not captured")
			}
			return s, true
		}
		if r.name == "register" && r.desc == "(Lnet/minecraft/world/level/block/state/properties/WoodType;)Lnet/minecraft/world/level/block/state/properties/WoodType;" {
			return args[0], true
		}
	}
	return nil, false
}
