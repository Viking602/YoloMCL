package modjar

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"strings"
	"testing"
)

func (c *fixtureClass) modAnnotation(descriptor, id string) {
	data := words(1, c.utf(descriptor), 1, c.utf("value"))
	data = append(data, 's')
	data = append(data, words(c.utf(id))...)
	attr := words(c.utf("RuntimeVisibleAnnotations"))
	attr = binary.BigEndian.AppendUint32(attr, uint32(len(data)))
	c.attrs = append(c.attrs, append(attr, data...))
}

func archiveFiles(t testing.TB, z *zip.Reader) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte, len(z.File))
	for _, entry := range z.File {
		f, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name] = data
	}
	return files
}

// Relocate UTF8 constants, retaining instruction offsets and pool indices.
func relocateFixture(data []byte, rewrite *strings.Replacer) []byte {
	r := reader{b: data}
	result := bytes.Clone(r.take(8))
	n := r.u2()
	result = append(result, words(n)...)
	for i := 1; i < n; i++ {
		tag := r.u1()
		result = append(result, byte(tag))
		switch tag {
		case 1:
			s := rewrite.Replace(string(r.take(r.u2())))
			result = append(result, words(len(s))...)
			result = append(result, s...)
		case 3, 4, 9, 10, 11, 12, 17, 18:
			result = append(result, r.take(4)...)
		case 5, 6:
			result = append(result, r.take(8)...)
			i++
		case 7, 8, 16, 19, 20:
			result = append(result, r.take(2)...)
		case 15:
			result = append(result, r.take(3)...)
		default:
			fail("fixture constant %d", tag)
		}
	}
	return append(result, data[r.p:]...)
}

func loaderFixture(t testing.TB, loader string, enabled bool) *zip.Reader {
	files := archiveFiles(t, regionFixture(t, "sample", enabled, false))
	c := &fixtureClass{}
	code := []byte{42}
	code = append(code, c.instruction(183, "java/lang/Object", "<init>", "()V")...)
	if loader == "fabric" || loader == "quilt" {
		c.interfaces = []string{"terrablender/api/TerraBlenderApi"}
		c.method("<init>", "()V", 1, append(code, 177))
		code = []byte{1}
		code = append(code, c.instruction(184, "example/Registration", "setup", "(Lnet/minecraftforge/fml/event/lifecycle/FMLCommonSetupEvent;)V")...)
		c.method("onTerraBlenderInitialized", "()V", 1, append(code, 177))
		files["fabric.mod.json"] = []byte(`{"schemaVersion":1,"id":"sample","version":"1","entrypoints":{"terrablender":[{"adapter":"default","value":"example.Main"}]}}`)
	} else {
		c.modAnnotation("Lnet/minecraftforge/fml/common/Mod;", "sample")
		code = append(code, c.instruction(184, "example/Registration", "initialize", "()V")...)
		desc := "()V"
		if loader == "neoforge" {
			desc = "(Lnet/minecraftforge/eventbus/api/IEventBus;Lnet/minecraftforge/fml/ModContainer;)V"
		}
		c.method("<init>", desc, 1, append(code, 177))
	}
	files["example/Main.class"] = c.bytes("example/Main", "java/lang/Object")
	if loader == "neoforge" {
		rewrite := strings.NewReplacer("net/minecraftforge/eventbus/api/", "net/neoforged/bus/api/", "net/minecraftforge/fml/", "net/neoforged/fml/")
		for name, data := range files {
			if strings.HasSuffix(name, ".class") {
				files[name] = relocateFixture(data, rewrite)
			}
		}
	}
	return fixtureArchive(t, files)
}

func TestLoaderProtocolsAcrossVersions(t *testing.T) {
	// This tests entrypoint/API compatibility, not whole-game prediction parity.
	versions := []string{"1.18", "1.18.2", "1.19", "1.19.2", "1.19.3", "1.19.4", "1.20", "1.20.1", "1.20.2", "1.20.4", "1.20.6", "1.21", "1.21.1", "1.21.3", "1.21.4", "1.21.5", "1.21.6", "1.21.8", "1.21.11", "26.1", "26.2", "26.3"}
	for _, loader := range []string{"forge", "neoforge", "fabric", "quilt"} {
		for _, enabled := range []bool{false, true} {
			for _, version := range versions {
				if loader == "neoforge" && compareMaven(mavenVersion(version), mavenVersion("1.20.1")) < 0 {
					continue // NeoForge did not exist before 1.20.1.
				}
				fixtureLoader := loader
				if loader == "neoforge" && version == "1.20.1" {
					fixtureLoader = "forge" // Its first release retained the Forge API namespace.
				}
				z := loaderFixture(t, fixtureLoader, enabled)
				result, err := ReadTerraBlender(context.Background(), z, Options{Minecraft: version, Loader: loader, ModID: "sample"})
				want := 0
				if enabled {
					want = 1
				}
				if err != nil || len(result.Regions) != want || len(result.Entrypoints) == 0 {
					t.Fatalf("%s %s enabled=%v: %+v %v", loader, version, enabled, result, err)
				}
				if enabled && (result.Regions[0].ID != "sample:region" || result.Regions[0].Parameters[0].Biome != "sample:biome") {
					t.Fatal(result)
				}
			}
		}
	}
}

func TestLoaderEntrypointIdentity(t *testing.T) {
	for _, loader := range []string{"forge", "neoforge", "fabric", "quilt"} {
		_, err := ReadTerraBlender(context.Background(), loaderFixture(t, loader, true), Options{Minecraft: "1.21.1", Loader: loader, ModID: "different"})
		if err == nil {
			t.Errorf("%s ran a different mod's entrypoint", loader)
		}
	}
}
