package modjar

import "strings"

// worldBiomeRegistry marks the biome registry handed to Region.addBiomes.
// Membership is answered from data the game would load: vanilla biomes and
// biome JSONs on the selected loader classpath. Anything else is absent, so a
// mod that rejects absent keys fails its own region rather than guessing.
type worldBiomeRegistry struct{}

var vanillaBiomeIDs = func() map[string]bool {
	ids := map[string]bool{}
	for _, path := range srgVanillaBiomes {
		ids["minecraft:"+path] = true
	}
	return ids
}()

func (h *bindings) biomeRegistered(id string) bool {
	ns, path, ok := strings.Cut(id, ":")
	if !ok || ns == "" || path == "" || strings.Contains(path, "..") {
		return false
	}
	if vanillaBiomeIDs[id] {
		return true
	}
	if h.options.Resources == nil {
		fail("biome registry membership needs the loader classpath")
	}
	docs, err := h.options.Resources("data/" + ns + "/worldgen/biome/" + path + ".json")
	if err != nil {
		fail("biome registry %s: %v", id, err)
	}
	return len(docs) > 0
}

func (h *bindings) worldBiomeRegistryCall(r reference, receiver any, args []any) (any, bool) {
	o, isObject := receiver.(*object)
	if !isObject || o == nil {
		return nil, false
	}
	if _, ok := o.data.(worldBiomeRegistry); !ok {
		return nil, false
	}
	if (r.name == "containsKey" || r.name == "m_142003_") && r.desc == "(Lnet/minecraft/resources/ResourceKey;)Z" {
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "biome key")
		}
		id, ok := objectData(args[0]).(string)
		if !ok {
			fail("biome registry key is unavailable")
		}
		if h.biomeRegistered(id) {
			return int32(1), true
		}
		return int32(0), true
	}
	return nil, false
}
