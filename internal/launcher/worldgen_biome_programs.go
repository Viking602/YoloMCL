package launcher

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/Viking602/YoloMCL/cubiomes"
	"github.com/Viking602/YoloMCL/cubiomes/modjar"
)

type WorldgenBiomeProgram struct {
	ID      string
	Program *modjar.BiomeProgram `json:",omitempty"`
	Error   string               `json:",omitempty"`
}

// Only effective dimensions request executable sources. All installed archives
// remain available for declared registration and dependencies; no mod ID or
// guessed Java class name selects an implementation.
func (c *worldgenCollector) readBiomePrograms(sources []WorldgenSource) error {
	wanted := map[string]bool{}
	scopes := map[string]cubiomes.NoiseRequirements{}
	unknownScope := map[string]bool{}
	docs := map[string]cubiomes.Document{}
	for _, doc := range c.snapshot.Documents {
		docs[doc.Kind+"/"+doc.ID] = doc
	}
	inspect := func(raw json.RawMessage) {
		d := parseSeedMapDimension(raw)
		if d.generator != "minecraft:noise" || !d.opaque {
			return
		}
		var source struct{ Type string }
		if json.Unmarshal(d.source, &source) == nil && source.Type != "" {
			id := seedMapID(source.Type)
			wanted[id] = true
			router, err := cubiomes.CompileNoiseRouter(c.ctx, docs, cubiomes.Definition{Version: c.snapshot.Minecraft, Settings: d.settings})
			if err != nil {
				unknownScope[id] = true
			} else {
				scope := scopes[id]
				next := router.Requirements()
				scope.NormalNoise = scope.NormalNoise || next.NormalNoise
				scope.EndIslands = scope.EndIslands || next.EndIslands
				scopes[id] = scope
			}
		}
	}
	if c.snapshot.SavedWorld {
		for _, raw := range c.snapshot.SavedDimensions {
			inspect(raw)
		}
	} else {
		for _, doc := range c.snapshot.Documents {
			switch doc.Kind {
			case "dimension":
				inspect(doc.Data)
			case "world_preset":
				var preset struct{ Dimensions map[string]json.RawMessage }
				if json.Unmarshal(doc.Data, &preset) == nil {
					for _, raw := range preset.Dimensions {
						inspect(raw)
					}
				}
			}
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	ids := make([]string, 0, len(wanted))
	for id := range wanted {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	roots, closeArchives, bootErr := c.classArchives(sources)
	defer closeArchives()
	mods := map[string]string{}
	loader, loaderErr := c.selectedLoader()
	for _, mod := range c.snapshot.Mods {
		mods[mod.ID] = mod.Version
	}
	if bootErr == nil {
		bootErr = loaderErr
	}
	var env *modjar.Environment
	if bootErr == nil {
		env, bootErr = modjar.BootstrapBiomeSources(c.ctx, roots, modjar.Options{Minecraft: c.snapshot.Minecraft, Symbols: c.symbols, JavaMajor: c.snapshot.JavaMajor, Loader: loader, LoaderVersion: c.snapshot.LoaderVersion, Mods: mods, Side: "DEDICATED_SERVER", Config: c.registrationConfig})
	}
	if env != nil {
		defer env.Close()
	}
	for _, id := range ids {
		result := WorldgenBiomeProgram{ID: id}
		err := bootErr
		if err == nil {
			var p modjar.BiomeProgram
			if unknownScope[id] {
				p, err = env.BiomeProgram(id)
			} else {
				p, err = env.BiomeProgram(id, scopes[id])
			}
			if err == nil {
				result.Program = &p
			}
		}
		if err != nil && bootErr == nil && strings.HasPrefix(err.Error(), "unregistered biome source ") {
			// Entrypoint registration (RegisterEvent / DeferredRegister) is
			// the loader path the bootstrap phase does not run. The registry
			// namespace names the owning mod, as the loader requires.
			if p, rerr := c.registeredBiomeProgram(id, sources, roots, loader, mods, unknownScope[id], scopes[id]); rerr == nil {
				result.Program, err = &p, nil
			} else {
				err = fmt.Errorf("%v; entrypoint registration: %v", err, rerr)
			}
		}
		if err != nil {
			result.Error = err.Error()
		}
		c.snapshot.BiomePrograms = append(c.snapshot.BiomePrograms, result)
	}
	return c.ctx.Err()
}

const biomeSourceRegistryKey = "net/minecraft/core/registries/Registries.BIOME_SOURCE"

// registeredBiomeProgram runs the namespace owner's entrypoints and fires only
// the biome-source RegisterEvent. Other registries stay unpopulated, so a codec
// that reads them fails closed instead of observing guessed contents.
func (c *worldgenCollector) registeredBiomeProgram(id string, sources []WorldgenSource, roots []modjar.Archive, loader string, mods map[string]string, unknown bool, scope cubiomes.NoiseRequirements) (modjar.BiomeProgram, error) {
	namespace, _, ok := strings.Cut(id, ":")
	if !ok || namespace == "" {
		return modjar.BiomeProgram{}, fmt.Errorf("invalid biome source id")
	}
	var source string
	for _, mod := range c.snapshot.Mods {
		if mod.ID == namespace {
			source = mod.Source
		}
	}
	paths := map[string]string{}
	for _, s := range sources {
		paths[s.Name] = s.Path
	}
	if source == "" || paths[source] == "" {
		return modjar.BiomeProgram{}, fmt.Errorf("no installed mod owns namespace %s", namespace)
	}
	z, err := zip.OpenReader(paths[source])
	if err != nil {
		return modjar.BiomeProgram{}, err
	}
	defer z.Close()
	cp, err := modjar.NewClasspath(c.ctx, roots, loader)
	if err != nil {
		return modjar.BiomeProgram{}, err
	}
	defer cp.Close()
	options := modjar.Options{Minecraft: c.snapshot.Minecraft, Symbols: c.symbols, JavaMajor: c.snapshot.JavaMajor, Loader: loader, LoaderVersion: c.snapshot.LoaderVersion, Mods: mods, Side: "DEDICATED_SERVER", Config: c.registrationConfig, Class: cp.Class, Resources: cp.Resources, ReflectionOpen: cp.ReflectionOpen, RegistryOrder: []string{biomeSourceRegistryKey}, PartialRegistries: true}
	env, err := modjar.LoadRegistries(c.ctx, []modjar.Module{{ID: namespace, Archive: &z.Reader}}, options)
	if err != nil {
		return modjar.BiomeProgram{}, err
	}
	defer env.Close()
	if unknown {
		return env.BiomeProgram(id)
	}
	return env.BiomeProgram(id, scope)
}
