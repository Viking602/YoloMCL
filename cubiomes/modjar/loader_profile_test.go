package modjar

import (
	"context"
	"strings"
	"testing"
)

func TestInstanceLoaderSelectsDescriptor(t *testing.T) {
	z := fixtureArchive(t, map[string][]byte{
		"fabric.mod.json": {}, "quilt.mod.json": {}, "mcmod.info": {},
		"META-INF/mods.toml": {}, "META-INF/neoforge.mods.toml": {},
	})
	for _, tc := range []struct{ loader, mc, want string }{
		{"fabric", "1.21.1", "fabric.mod.json"},
		{"quilt", "1.21.1", "quilt.mod.json"},
		{"forge", "1.12.2", "mcmod.info"},
		{"forge", "1.20.1", "META-INF/mods.toml"},
		{"neoforge", "1.20.1", "META-INF/mods.toml"},
		{"neoforge", "1.20.4", "META-INF/mods.toml"},
		{"neoforge", "1.20.5", "META-INF/neoforge.mods.toml"},
		{"neoforge", "1.21.1", "META-INF/neoforge.mods.toml"},
		{"neoforge", "26.3", "META-INF/neoforge.mods.toml"},
		{"vanilla", "1.21.1", ""},
	} {
		t.Run(tc.loader+"/"+tc.mc, func(t *testing.T) {
			p, err := SelectLoader(tc.loader, tc.mc, "")
			if err != nil {
				t.Fatal(err)
			}
			f, err := p.Descriptor(z)
			var got string
			if f != nil {
				got = f.Name
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q %v, want %q", got, err, tc.want)
			}
		})
	}
	p, _ := SelectLoader("", "1.21.1", "")
	if _, err := p.Descriptor(z); err == nil {
		t.Fatal("ambiguous standalone archive picked a loader")
	}
	if _, err := SelectLoader("unrecognised", "1.21.1", ""); err == nil {
		t.Fatal("unknown loader accepted")
	}
	foreign := fixtureArchive(t, map[string][]byte{"fabric.mod.json": {}})
	for _, loader := range []string{"forge", "neoforge", "vanilla"} {
		p, _ := SelectLoader(loader, "1.21.1", "")
		if f, err := p.Descriptor(foreign); err != nil || f != nil {
			t.Fatalf("%s activated foreign descriptor", loader)
		}
	}
	p, _ = SelectLoader("quilt", "1.21.1", "")
	if f, err := p.Descriptor(foreign); err != nil || f == nil {
		t.Fatal("Quilt lost Fabric compatibility", err)
	}
}

func TestUniversalArchiveDoesNotRunForeignFMLConstructor(t *testing.T) {
	for _, loader := range []string{"forge", "neoforge"} {
		t.Run(loader, func(t *testing.T) {
			files := archiveFiles(t, loaderFixture(t, loader, true))
			foreign := &fixtureClass{}
			annotation := "Lnet/neoforged/fml/common/Mod;"
			if loader == "neoforge" {
				annotation = "Lnet/minecraftforge/fml/common/Mod;"
			}
			foreign.modAnnotation(annotation, "sample")
			code := []byte{42}
			code = append(code, foreign.instruction(183, "java/lang/Object", "<init>", "()V")...)
			code = append(code, foreign.instruction(184, "java/lang/Runtime", "getRuntime", "()Ljava/lang/Runtime;")...)
			foreign.method("<init>", "()V", 1, append(code, 88, 177))
			files["foreign/Entrypoint.class"] = foreign.bytes("foreign/Entrypoint", "java/lang/Object")
			r, err := ReadTerraBlender(context.Background(), fixtureArchive(t, files), Options{Loader: loader, Minecraft: "1.21.1", ModID: "sample"})
			if err != nil || len(r.Regions) != 1 {
				t.Fatalf("foreign entrypoint affected %s: %+v %v", loader, r, err)
			}
			for _, entry := range r.Entrypoints {
				if strings.Contains(entry, "foreign/") {
					t.Fatal(entry)
				}
			}
		})
	}
}

func TestQuiltDedicatedRegistrationUsesQuiltDescriptor(t *testing.T) {
	files := archiveFiles(t, loaderFixture(t, "quilt", true))
	files["fabric.mod.json"] = []byte(`invalid inactive metadata`)
	files["quilt.mod.json"] = []byte(`{"schema_version":1,"quilt_loader":{"id":"sample","version":"1","entrypoints":{"terrablender":["example.Main"]}}}`)
	r, err := ReadTerraBlender(context.Background(), fixtureArchive(t, files), Options{Loader: "quilt", Minecraft: "1.21.1", ModID: "sample"})
	if err != nil || len(r.Regions) != 1 {
		t.Fatalf("Quilt registration: %+v %v", r, err)
	}
}

func TestInstalledLoaderDoesNotGuessStaticEntrypoints(t *testing.T) {
	for _, loader := range []string{"forge", "neoforge"} {
		r, err := ReadTerraBlender(context.Background(), regionFixture(t, "sample", true, false), Options{Loader: loader, Minecraft: "1.21.1", ModID: "sample"})
		if err == nil || len(r.Regions) != 0 {
			t.Fatalf("%s executed undeclared module: %+v %v", loader, r, err)
		}
	}
}
