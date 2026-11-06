package modjar

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Loader-provided mod IDs never have a constructor in the mod catalogue.
var platformModIDs = map[string]bool{"minecraft": true, "forge": true, "neoforge": true, "fml": true, "javafml": true, "terrablender": true}

// mandatoryDependencies reads the required [[dependencies.<modId>]] entries
// a Forge/NeoForge descriptor declares for modID.
func mandatoryDependencies(ctx context.Context, z *zip.Reader, modID string) []string {
	var out []string
	for _, f := range z.File {
		if f.Name != "META-INF/mods.toml" && f.Name != "META-INF/neoforge.mods.toml" {
			continue
		}
		b, err := archiveBytes(ctx, f, 1<<20)
		if err != nil {
			continue
		}
		var meta struct {
			Dependencies map[string][]struct {
				ModID     string `toml:"modId"`
				Mandatory *bool  `toml:"mandatory"`
				Type      string `toml:"type"`
			} `toml:"dependencies"`
		}
		if toml.Unmarshal(b, &meta) != nil {
			continue
		}
		for _, d := range meta.Dependencies[modID] {
			required := d.Mandatory != nil && *d.Mandatory || strings.EqualFold(d.Type, "required")
			if required && d.ModID != "" && !platformModIDs[d.ModID] {
				out = append(out, d.ModID)
			}
		}
	}
	return out
}

// dependencyConstructors finds the @Mod constructors of the target mod's
// required library mods, dependencies first. FML constructs a mod only after
// its required dependencies, so a library's static state (for example
// Zeta's ZetaMod.ZETA) exists before the dependent constructor reads it.
func dependencyConstructors(ctx context.Context, v *machine, z *zip.Reader, archives []Archive, options Options) []registrationEntry {
	profile, err := SelectLoader(options.Loader, options.Minecraft, options.LoaderVersion)
	if err != nil {
		return nil
	}
	byID := map[string]Archive{}
	cp := &Classpath{ctx: ctx}
	for _, a := range archives {
		for _, id := range cp.archiveModIDs(a) {
			if _, ok := byID[id]; !ok {
				byID[id] = a
			}
		}
	}
	var entries []registrationEntry
	seen := map[string]bool{options.ModID: true}
	var visit func(r *zip.Reader, id string, depth int)
	visit = func(r *zip.Reader, id string, depth int) {
		if depth > 8 {
			return
		}
		for _, dep := range mandatoryDependencies(ctx, r, id) {
			if seen[dep] {
				continue
			}
			seen[dep] = true
			a, ok := byID[dep]
			if !ok {
				continue
			}
			visit(a.Reader, dep, depth+1)
			if e, ok := modConstructor(ctx, v, a, dep, profile, options.Side); ok {
				entries = append(entries, e)
			}
		}
	}
	visit(z, options.ModID, 0)
	return entries
}

func modConstructor(ctx context.Context, v *machine, a Archive, id string, profile LoaderProfile, side string) (registrationEntry, bool) {
	quoted := []byte(id)
	for _, f := range a.Reader.File {
		if !strings.HasSuffix(f.Name, ".class") || f.UncompressedSize64 > 1<<20 {
			continue
		}
		b, err := archiveBytes(ctx, f, 1<<20)
		if err != nil || !bytes.Contains(b, []byte("/fml/common/Mod;")) || !bytes.Contains(b, quoted) {
			continue
		}
		c := v.load(strings.TrimSuffix(f.Name, ".class"))
		an, ok := profile.annotation(c.annotations, "Lnet/minecraftforge/fml/common/Mod;")
		if !ok {
			continue
		}
		if value, _ := an.values["value"].(string); value != id || !annotationSide(an, "dist", side) {
			continue
		}
		var found []registrationEntry
		for _, m := range c.methods {
			if m.name == "<init>" && m.access&1 != 0 {
				found = append(found, registrationEntry{ref: reference{c.name, m.name, m.desc}, constructor: true})
			}
		}
		if len(found) == 1 {
			return found[0], true
		}
		return registrationEntry{}, false
	}
	return registrationEntry{}, false
}
