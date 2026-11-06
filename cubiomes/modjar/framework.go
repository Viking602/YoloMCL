package modjar

import "strings"

// These are framework API relocations, not mod/version aliases. Consumer class
// names and executable bodies remain untouched. Shared native bindings use one
// canonical vocabulary; casts and lifecycle descriptors use it as well.
var frameworkNames = strings.NewReplacer(
	"net/neoforged/api/distmarker/", "net/minecraftforge/api/distmarker/",
	"net/neoforged/fml/common/EventBusSubscriber", "net/minecraftforge/fml/common/Mod$EventBusSubscriber",
	"net/neoforged/neoforge/common/ModConfigSpec", "net/minecraftforge/common/ForgeConfigSpec",
	"net/neoforged/neoforge/common/NeoForge", "net/minecraftforge/common/MinecraftForge",
	"net/neoforged/bus/api/", "net/minecraftforge/eventbus/api/",
	"net/neoforged/fml/", "net/minecraftforge/fml/",
)

func frameworkName(name string) string { return frameworkNames.Replace(name) }

func frameworkRef(r reference) reference {
	r.owner = frameworkName(r.owner)
	r.desc = frameworkName(r.desc)
	return r
}
