package modjar

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestInstalledMixinGraph(t *testing.T) {
	dir := os.Getenv("MCL_MIXIN_DIRECTORY")
	if dir == "" {
		t.Skip("MCL_MIXIN_DIRECTORY names read-only installed archives")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var archives []Archive
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jar") {
			continue
		}
		z, e := zip.OpenReader(filepath.Join(dir, entry.Name()))
		if e != nil {
			t.Fatal(e)
		}
		defer z.Close()
		archives = append(archives, Archive{Name: entry.Name(), Reader: &z.Reader})
	}
	env, err := BootstrapBiomeSources(context.Background(), archives, Options{Minecraft: "1.21.1", Loader: "neoforge", Side: "CLIENT", JavaMajor: 21})
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	if path := os.Getenv("MCL_MIXIN_EXPORT"); path != "" {
		data, err := json.MarshalIndent(env.MixinEffects, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("%d declared injection effects, %d registered biome sources", len(env.MixinEffects), len(env.BiomeSourceIDs()))
}

// Opt-in diagnostics of the loader-declared entrypoints in an installed JAR.
// This inspects the same annotations and metadata used by Load, without
// substituting an inferred class name for an actual registry identifier.
func TestInstalledLoaderEntrypoints(t *testing.T) {
	path := os.Getenv("MCL_LOADER_JAR")
	if path == "" {
		t.Skip("MCL_LOADER_JAR names an installed loader input")
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	options := Options{Minecraft: "1.21.1", Loader: os.Getenv("MCL_LOADER_KIND"), ModID: os.Getenv("MCL_LOADER_MOD"), fullLifecycle: true}
	v, names, err := archiveMachine(context.Background(), &z.Reader, options)
	if err != nil {
		t.Fatal(err)
	}
	p := discoverEntrypoints(v, &z.Reader, names, options)
	for _, entry := range append(append(p.before, p.setup...), p.after...) {
		t.Logf("entry %s.%s%s", entry.ref.owner, entry.ref.name, entry.ref.desc)
	}
	for _, listener := range p.subscribers {
		t.Logf("subscriber %s %s.%s", listener.event, listener.callback.target.owner, listener.callback.target.name)
	}
	if len(p.before)+len(p.setup)+len(p.after)+len(p.subscribers) == 0 {
		t.Fatal("installed module has no declared entrypoint")
	}
	for _, name := range names {
		if !strings.Contains(strings.ToLower(name), "mixin") {
			continue
		}
		data, e := v.inspectSource(name)
		if e != nil {
			t.Fatal(e)
		}
		c := parseClass(data)
		for _, a := range c.annotations {
			if a.typeName != "Lorg/spongepowered/asm/mixin/Mixin;" {
				continue
			}
			text := strings.ReplaceAll(fmt.Sprint(a.values), ".", "/")
			if !strings.Contains(text, "world/level/biome/") && !strings.Contains(text, "world/level/levelgen/") {
				continue
			}
			t.Logf("mixin %s %v", c.name, a.values)
			for _, m := range c.methods {
				if len(m.annotations) > 0 {
					t.Logf("  hook %s%s %v calls=%d", m.name, m.desc, m.annotations, len(methodReferences(c, m)))
				}
			}
		}
	}
}

func TestInstalledBiomeSourceBootstrap(t *testing.T) {
	path := os.Getenv("MCL_BIOME_SOURCE_JAR")
	if path == "" {
		t.Skip("MCL_BIOME_SOURCE_JAR names the installed oracle archive")
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	env, err := BootstrapBiomeSources(context.Background(), []Archive{{Name: path, Reader: &z.Reader}}, Options{Minecraft: "1.21.1", Loader: "neoforge", Side: "DEDICATED_SERVER"})
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	ids := env.BiomeSourceIDs()
	t.Logf("actual registry IDs: %v", ids)
	if !slices.Contains(ids, "c6c:new_end_biome_source") {
		t.Fatal("installed bootstrap did not register the dimension's biome source")
	}
	program, err := env.BiomeProgram("c6c:new_end_biome_source")
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := program.NewRuntime(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if runtime == nil || program.EndHeight == nil {
		t.Fatal("missing immutable source or density overwrite")
	}
	t.Logf("captured %d class definitions, %d biome holders and the declared density overwrite", len(program.Classes), len(program.Holders))
	for _, effect := range env.MixinEffects {
		if strings.HasSuffix(effect.Target, "$EndIslandDensityFunction") {
			c := env.vm.load(effect.Class)
			for _, m := range c.methods {
				if m.name == effect.Method {
					t.Logf("density APIs: %v", methodReferences(c, m))
				}
			}
		}
	}
}
