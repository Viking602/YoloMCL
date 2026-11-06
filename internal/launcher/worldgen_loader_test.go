package launcher

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Viking602/YoloMCL/cubiomes/modjar"
)

func TestWorldgenLoaderExcludesInactiveModResources(t *testing.T) {
	for _, tc := range []struct{ loader, mc, descriptor, metadata string }{
		{"fabric", "1.21.1", "fabric.mod.json", `{"id":"active","version":"1"}`},
		{"quilt", "1.21.1", "quilt.mod.json", `{"schema_version":1,"quilt_loader":{"id":"active","version":"1"}}`},
		{"forge", "1.20.1", "META-INF/mods.toml", "[[mods]]\nmodId=\"active\"\nversion=\"1\""},
		{"neoforge", "1.20.4", "META-INF/mods.toml", "[[mods]]\nmodId=\"active\"\nversion=\"1\""},
		{"neoforge", "1.21.1", "META-INF/neoforge.mods.toml", "[[mods]]\nmodId=\"active\"\nversion=\"1\""},
	} {
		t.Run(tc.loader+"/"+tc.mc, func(t *testing.T) {
			root := t.TempDir()
			active := filepath.Join(root, "active.jar")
			foreign := filepath.Join(root, "foreign.jar")
			pack := filepath.Join(root, "pack.zip")
			writeTestFile(t, active, zipBytes(t, map[string][]byte{tc.descriptor: []byte(tc.metadata), "data/active/worldgen/biome/test.json": []byte(`{}`)}))
			wrong := "fabric.mod.json"
			if tc.loader == "fabric" || tc.loader == "quilt" {
				wrong = "META-INF/neoforge.mods.toml"
			}
			writeTestFile(t, foreign, zipBytes(t, map[string][]byte{wrong: []byte(`invalid inactive metadata`), "data/foreign/worldgen/biome/broken.json": []byte(`invalid inactive resource`)}))
			writeTestFile(t, pack, zipBytes(t, map[string][]byte{"data/shared/worldgen/biome/test.json": []byte(`{}`)}))
			input := WorldgenInput{Minecraft: tc.mc, Loader: tc.loader, LoaderVersion: "1", Sources: []WorldgenSource{{active, "mod:active.jar"}, {foreign, "mod:foreign.jar"}, {pack, "datapack:pack.zip"}}}
			snapshot, err := WorldgenIngest(context.Background(), input)
			if err != nil || hasWorldgenIssue(snapshot, "source_unreadable") || hasWorldgenIssue(snapshot, "invalid_json") || len(snapshot.Mods) != 1 || snapshot.Mods[0].Loader != tc.loader {
				t.Fatalf("inactive file affected %s: %+v %v", tc.loader, snapshot, err)
			}
			worldgenDoc(t, snapshot, "biome", "active:test")
			worldgenDoc(t, snapshot, "biome", "shared:test")
			input.LoaderVersion = "2"
			changed, err := WorldgenIngest(context.Background(), input)
			if err != nil || changed.Hash == snapshot.Hash {
				t.Fatal("loader version reused prediction cache", err)
			}
		})
	}
}

func TestWorldgenFMLKeepsEveryDeclaredModIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "multiple.jar")
	writeTestFile(t, path, zipBytes(t, map[string][]byte{"META-INF/neoforge.mods.toml": []byte("[[mods]]\nmodId=\"first\"\nversion=\"1\"\n[[mods]]\nmodId=\"second\"\nversion=\"2\"\n")}))
	snapshot, err := WorldgenIngest(context.Background(), WorldgenInput{Minecraft: "1.21.1", Loader: "neoforge", Sources: []WorldgenSource{{path, "mod:multiple.jar"}}})
	if err != nil || len(snapshot.Mods) != 2 || snapshot.Mods[0].ID != "first" || snapshot.Mods[1].ID != "second" || snapshot.Mods[1].Version != "2" {
		t.Fatalf("%+v %v", snapshot, err)
	}
}

func TestWorldgenDeclaredLibraryIsNotModPack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.jar")
	writeTestFile(t, path, zipBytes(t, map[string][]byte{
		"META-INF/MANIFEST.MF":                      []byte("Manifest-Version: 1.0\r\nFMLModType: LIB\r\n RARY\r\n\r\n"),
		"probe/Marker.class":                        []byte{0xca, 0xfe, 0xba, 0xbe},
		"data/inactive/worldgen/biome/invalid.json": []byte(`not a mod resource pack`),
	}))
	source := WorldgenSource{path, "mod:library.jar"}
	c := worldgenCollector{ctx: context.Background(), snapshot: &WorldgenSnapshot{Loader: "neoforge", Minecraft: "1.21.1"}, documents: map[string]WorldgenDocument{}}
	if err := c.readSource(source); err != nil {
		t.Fatal(err)
	}
	if len(c.snapshot.Mods) != 0 || len(c.documents) != 0 || len(c.libraries) != 1 {
		t.Fatal("library treated as a mod pack")
	}
	roots, closeRoots, err := c.classArchives([]WorldgenSource{source})
	defer closeRoots()
	if err != nil {
		t.Fatal(err)
	}
	cp, err := modjar.NewClasspath(c.ctx, roots, "neoforge")
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	if data, err := cp.Class("probe/Marker"); err != nil || len(data) != 4 {
		t.Fatal("declared library missing from classpath", err)
	}
}

func TestWorldgenClientDoesNotActivateServerOnlyFabricMod(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.jar")
	writeTestFile(t, path, zipBytes(t, map[string][]byte{"fabric.mod.json": []byte(`{"id":"server_only","version":"1","environment":"server"}`), "data/inactive/worldgen/biome/invalid.json": []byte(`invalid`)}))
	snapshot, err := WorldgenIngest(context.Background(), WorldgenInput{Minecraft: "1.21.1", Loader: "fabric", Sources: []WorldgenSource{{path, "mod:server.jar"}}})
	if err != nil || len(snapshot.Mods) != 0 || hasWorldgenIssue(snapshot, "invalid_json") {
		t.Fatalf("%+v %v", snapshot, err)
	}
}

func TestInstalledLoaderUsesLaunchProfileNotDisplayName(t *testing.T) {
	for _, tc := range []struct {
		v    gameVersion
		want string
	}{
		{gameVersion{ID: "my-forge-neoforge-pack"}, "vanilla"},
		{gameVersion{Libraries: []library{{Name: "net.neoforged:srgutils:1"}, {Name: "net.minecraftforge:eventbus:6"}, {Name: "net.fabricmc:fabric-loader:0.19"}}}, "fabric"},
		{gameVersion{MainClass: "org.quiltmc.loader.impl.launch.knot.KnotClient", Libraries: []library{{Name: "net.fabricmc:fabric-loader:0.19"}}}, "quilt"},
		{gameVersion{MainClass: "net.fabricmc.loader.impl.launch.knot.KnotClient", Libraries: []library{{Name: "net.neoforged.fancymodloader:loader:4"}}}, "fabric"},
		{gameVersion{Libraries: []library{{Name: "net.neoforged:forge:1.20.1-47.1.106"}}}, "neoforge"},
	} {
		if got := installedLoader(tc.v); got != tc.want {
			t.Errorf("%+v: %s, want %s", tc.v, got, tc.want)
		}
	}
}
