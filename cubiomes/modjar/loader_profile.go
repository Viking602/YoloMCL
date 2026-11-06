package modjar

import (
	"archive/zip"
	"context"
	"fmt"
	"strings"
)

// LoaderProfile is selected by the installed instance before inspecting mods.
// Compatibility here describes a loader contract, not individual mod names.
type LoaderProfile struct {
	Loader, Minecraft, Version string
	Namespace                  string
	Descriptors                []string
}

func (p LoaderProfile) LegacyForge() bool {
	return p.Loader == "forge" && strings.HasPrefix(p.Minecraft, "1.") && compareMaven(mavenVersion(p.Minecraft), mavenVersion("1.13")) < 0
}

// Library identifies FML library files that belong on the classpath, not in mod
// resource packs. A foreign loader descriptor alone does not activate a file.
func (p LoaderProfile) Library(ctx context.Context, z *zip.Reader) (bool, error) {
	if p.Loader != "forge" && p.Loader != "neoforge" || p.LegacyForge() {
		return false, nil
	}
	raw, err := mixinFile(ctx, z, "META-INF/MANIFEST.MF")
	if err != nil {
		return false, err
	}
	manifest := strings.ReplaceAll(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n ", "")
	for line := range strings.SplitSeq(manifest, "\n") {
		if line == "" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(key, "FMLModType") {
			switch strings.TrimSpace(value) {
			case "LIBRARY", "GAMELIBRARY", "LANGPROVIDER":
				return true, nil
			case "MOD":
				return false, nil
			default:
				return false, fmt.Errorf("unknown FMLModType %q", value)
			}
		}
	}
	return false, nil
}

func SelectLoader(loader, minecraft, version string) (LoaderProfile, error) {
	p := LoaderProfile{Loader: loader, Minecraft: minecraft, Version: version, Namespace: "named"}
	before := func(release string) bool { return compareMaven(mavenVersion(minecraft), mavenVersion(release)) < 0 }
	switch loader {
	case "vanilla":
		p.Namespace = "official"
	case "fabric":
		p.Namespace = "intermediary"
		p.Descriptors = []string{"fabric.mod.json"}
	case "quilt":
		p.Namespace = "intermediary"
		// Quilt explicitly supports Fabric metadata; Fabric does not support Quilt.
		p.Descriptors = []string{"quilt.mod.json", "fabric.mod.json"}
	case "forge":
		p.Namespace = "srg"
		p.Descriptors = []string{"META-INF/mods.toml"}
		if strings.HasPrefix(minecraft, "1.") && before("1.13") {
			p.Descriptors = []string{"mcmod.info"}
		}
	case "neoforge":
		p.Descriptors = []string{"META-INF/neoforge.mods.toml"}
		if minecraft != "" && before("1.20.5") {
			p.Descriptors = []string{"META-INF/mods.toml"}
			if before("1.20.2") {
				p.Namespace = "srg"
			}
		}
	case "":
		// Standalone inspection may not have an installed profile. Selection
		// below accepts exactly one descriptor; it cannot pick a loader by order.
		p.Descriptors = []string{"META-INF/neoforge.mods.toml", "META-INF/mods.toml", "quilt.mod.json", "fabric.mod.json", "mcmod.info"}
	default:
		return p, fmt.Errorf("unknown loader %q", loader)
	}
	return p, nil
}

func (p LoaderProfile) Descriptor(z *zip.Reader) (*zip.File, error) {
	var selected *zip.File
	for _, name := range p.Descriptors {
		var found *zip.File
		for _, f := range z.File {
			if f.Name == name {
				if found != nil {
					return nil, fmt.Errorf("duplicate loader descriptor %s", name)
				}
				found = f
			}
		}
		if found == nil {
			continue
		}
		if p.Loader != "" {
			return found, nil
		}
		if selected != nil {
			return nil, fmt.Errorf("an instance loader is required for a multi-loader archive")
		}
		selected = found
	}
	return selected, nil
}

func DescriptorLoader(name string) string {
	switch name {
	case "fabric.mod.json":
		return "fabric"
	case "quilt.mod.json":
		return "quilt"
	case "META-INF/neoforge.mods.toml":
		return "neoforge"
	case "META-INF/mods.toml", "mcmod.info":
		return "forge"
	}
	return ""
}

// Native API aliases can share an implementation; discovery must still only
// activate annotations belonging to the selected loader. Universal archives
// can contain both Forge and NeoForge entrypoint classes.
func (p LoaderProfile) annotation(values []annotation, name string) (annotation, bool) {
	neo := p.Loader == "neoforge" && p.Namespace != "srg"
	for _, a := range values {
		if strings.HasPrefix(a.typeName, "Lnet/neoforged/") != neo {
			continue
		}
		if frameworkName(a.typeName) == name {
			return a, true
		}
	}
	return annotation{}, false
}
