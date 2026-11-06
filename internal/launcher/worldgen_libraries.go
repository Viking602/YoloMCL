package launcher

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/Viking602/YoloMCL/cubiomes/modjar"
)

// Resolve exactly the installed version inheritance and library selection used
// by launch preparation. This path never downloads or starts a Java runtime.
func seedMapInstalledLibraries(ctx context.Context, root, id string) ([]WorldgenSource, error) {
	v, err := seedMapInstalledVersion(ctx, root, id)
	if err != nil {
		return nil, err
	}
	return seedMapVersionLibraries(ctx, root, v)
}

func seedMapInstalledVersion(ctx context.Context, root, id string) (gameVersion, error) {
	if err := ctx.Err(); err != nil {
		return gameVersion{}, err
	}
	if id == "" {
		return gameVersion{}, nil
	}
	seen := map[string]bool{}
	var resolve func(string, int) (gameVersion, error)
	resolve = func(id string, depth int) (gameVersion, error) {
		if err := ctx.Err(); err != nil {
			return gameVersion{}, err
		}
		if depth > 12 || seen[id] {
			return gameVersion{}, fmt.Errorf("invalid installed version inheritance")
		}
		seen[id] = true
		v, err := readVersion(root, id)
		if err != nil {
			return v, err
		}
		if v.InheritsFrom == "" {
			return v, nil
		}
		base, err := resolve(v.InheritsFrom, depth+1)
		if err != nil {
			return v, err
		}
		return mergeVersion(base, v), nil
	}
	return resolve(id, 0)
}

func seedMapVersionLibraries(ctx context.Context, root string, v gameVersion) ([]WorldgenSource, error) {
	var result []WorldgenSource
	paths := map[string]bool{}
	for _, lib := range v.Libraries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !rulesAllow(lib.Rules) || lib.ClientReq != nil && !*lib.ClientReq {
			continue
		}
		a, err := libraryArtifact(lib)
		if err != nil {
			return nil, err
		}
		if lib.Downloads.Artifact == nil && len(lib.Downloads.Classifiers) > 0 {
			continue
		}
		if a.Path == "" {
			continue
		}
		p, err := safeJoin(root, "libraries/"+a.Path)
		if err != nil {
			return nil, err
		}
		key := p
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if paths[key] {
			continue
		}
		paths[key] = true
		if len(result) >= 1024 {
			return nil, fmt.Errorf("installed library count limit")
		}
		result = append(result, WorldgenSource{Path: p, Name: "library:" + lib.Name})
	}
	// FML locates its own runtime (the loader API and language providers) from
	// the launch arguments instead of the library list. Resolve the same
	// coordinates so loader API classes come from the installed JARs.
	for _, name := range fmlRuntimeArtifacts(v) {
		a, err := mavenPath(name)
		if err != nil {
			continue
		}
		p, err := safeJoin(root, "libraries/"+a)
		if err != nil {
			return nil, err
		}
		key := p
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if paths[key] {
			continue
		}
		if info, err := os.Stat(p); err != nil || info.IsDir() {
			continue
		}
		paths[key] = true
		result = append(result, WorldgenSource{Path: p, Name: "library:" + name})
	}
	return result, nil
}

// fmlRuntimeArtifacts lists the implicit FML runtime coordinates named by the
// launch arguments (--fml.forgeVersion / --fml.neoForgeVersion).
func fmlRuntimeArtifacts(v gameVersion) []string {
	args := map[string]string{}
	var flat []string
	for _, raw := range v.Arguments.Game {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			flat = append(flat, s)
		}
	}
	flat = append(flat, strings.Fields(v.MinecraftArguments)...)
	for i := 0; i+1 < len(flat); i++ {
		if strings.HasPrefix(flat[i], "--fml.") {
			args[strings.TrimPrefix(flat[i], "--fml.")] = flat[i+1]
		}
	}
	mc := args["mcVersion"]
	var out []string
	if neo := args["neoForgeVersion"]; neo != "" {
		out = append(out, "net.neoforged:neoforge:"+neo+":universal")
	}
	if forge := args["forgeVersion"]; forge != "" && mc != "" {
		group := args["forgeGroup"]
		if group == "" {
			group = "net.minecraftforge"
		}
		full := mc + "-" + forge
		out = append(out, group+":forge:"+full+":universal")
		for _, language := range []string{"fmlcore", "javafmllanguage", "lowcodelanguage", "mclanguage"} {
			out = append(out, group+":"+language+":"+full)
		}
	}
	return out
}

// Both registration and biome bootstrapping use this same selected catalogue.
// Libraries contribute classes and service resources, without being discovered
// as mod entrypoints or being mixed into the datapack priority list.
func (c *worldgenCollector) classArchives(sources []WorldgenSource) ([]modjar.Archive, func(), error) {
	if c.classPathErr != nil {
		return nil, func() {}, c.classPathErr
	}
	paths := map[string]string{}
	for _, source := range sources {
		paths[source.Name] = source.Path
	}
	selected := append([]WorldgenSource(nil), c.libraries...)
	for _, mod := range c.snapshot.Mods {
		if c.registrationSources != nil && !c.registrationSources[mod.Source] {
			continue
		}
		selected = append(selected, WorldgenSource{Path: paths[mod.Source], Name: mod.Source})
	}
	var roots []modjar.Archive
	var opened []*zip.ReadCloser
	closeAll := func() {
		for _, z := range opened {
			z.Close()
		}
		opened = nil
	}
	seen := map[string]bool{}
	for _, source := range selected {
		if err := c.ctx.Err(); err != nil {
			closeAll()
			return nil, closeAll, err
		}
		key := source.Path
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		z, err := zip.OpenReader(source.Path)
		if err != nil {
			closeAll()
			return nil, closeAll, fmt.Errorf("%s: %w", source.Name, err)
		}
		opened = append(opened, z)
		roots = append(roots, modjar.Archive{Name: source.Path, Reader: &z.Reader})
	}
	return roots, closeAll, nil
}
