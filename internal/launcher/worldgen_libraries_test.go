package launcher

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Viking602/YoloMCL/cubiomes/modjar"
)

func TestWorldgenInstalledLibrarySelection(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "versions/base/base.json"), []byte(`{"id":"base","libraries":[{"name":"sample:replaced:1"},{"name":"sample:retained:1"}]}`))
	writeTestFile(t, filepath.Join(root, "versions/profile/profile.json"), []byte(`{"id":"profile","inheritsFrom":"base","libraries":[{"name":"sample:replaced:2"},{"name":"sample:replaced:2"},{"name":"sample:server:1","clientreq":false},{"name":"sample:denied:1","rules":[{"action":"disallow"}]},{"name":"sample:native:1","downloads":{"classifiers":{"natives-windows":{"path":"native.dll"}}}}]}`))
	got, err := seedMapInstalledLibraries(context.Background(), root, "profile")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, source := range got {
		names = append(names, source.Name)
	}
	if !slices.Equal(names, []string{"library:sample:retained:1", "library:sample:replaced:2"}) {
		t.Fatal(names)
	}
	if got[1].Path != filepath.Join(root, "libraries/sample/replaced/2/replaced-2.jar") {
		t.Fatal(got[1])
	}
	writeTestFile(t, filepath.Join(root, "versions/base/base.json"), []byte(`{"id":"base","inheritsFrom":"profile"}`))
	if _, err = seedMapInstalledLibraries(context.Background(), root, "profile"); err == nil {
		t.Fatal("inheritance cycle accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = seedMapInstalledLibraries(ctx, root, "profile"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err = seedMapInstalledLibraries(context.Background(), root, ""); err != nil {
		t.Fatal("profile-less data input changed", err)
	}
}

func TestWorldgenSharedLibraryCatalogue(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "library.jar")
	mod := filepath.Join(root, "mod.jar")
	// Classpath reads raw bytes lazily. The parser executes only requested code.
	writeTestFile(t, library, zipBytes(t, map[string][]byte{"example/Dependency.class": []byte("library class"), "META-INF/services/example.Service": []byte("example.Dependency\n"), "fabric.mod.json": []byte(`{"schemaVersion":1,"id":"not_a_mod","version":"1"}`), "data/hidden/worldgen/biome/test.json": []byte(`{}`)}))
	writeTestFile(t, mod, zipBytes(t, map[string][]byte{"example/Main.class": []byte("mod class"), "data/sample/worldgen/biome/test.json": []byte(`{}`)}))
	source := WorldgenSource{Path: mod, Name: "mod:sample"}
	libraries := []WorldgenSource{{Path: library, Name: "library:sample"}, {Path: library, Name: "library:duplicate"}}
	c := &worldgenCollector{ctx: context.Background(), libraries: libraries, snapshot: &WorldgenSnapshot{Mods: []WorldgenMod{{ID: "sample", Source: source.Name}, {ID: "second", Source: source.Name}}}, registrationSources: map[string]bool{source.Name: true}}
	roots, closeRoots, err := c.classArchives([]WorldgenSource{source})
	if err != nil {
		t.Fatal(err)
	}
	defer closeRoots()
	if len(roots) != 2 {
		t.Fatal("duplicate archives", len(roots))
	}
	cp, err := modjar.NewClasspath(context.Background(), roots, "fabric")
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	if raw, err := cp.Class("example/Dependency"); err != nil || string(raw) != "library class" {
		t.Fatal(string(raw), err)
	}
	if resources, err := cp.Resources("META-INF/services/example.Service"); err != nil || len(resources) != 1 {
		t.Fatal(resources, err)
	}
	snapshot, err := WorldgenIngest(context.Background(), WorldgenInput{Minecraft: "1.21.1", Libraries: libraries})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Mods) != 0 || slices.ContainsFunc(snapshot.Documents, func(d WorldgenDocument) bool { return d.ID == "hidden:test" }) {
		t.Fatal("library was treated as a mod or datapack")
	}
	c.classPathErr = errors.New("manifest unavailable")
	if _, closeFailed, err := c.classArchives(nil); !errors.Is(err, c.classPathErr) {
		closeFailed()
		t.Fatal("partial classpath accepted", err)
	}
}
