package launcher

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func rawVersionArgs(values ...string) []json.RawMessage {
	args := make([]json.RawMessage, len(values))
	for i, value := range values {
		args[i], _ = json.Marshal(value)
	}
	return args
}

func TestRenamedMergedNeoForgeVersionIdentity(t *testing.T) {
	root := t.TempDir()
	v := gameVersion{ID: "project02", MainClass: "net.neoforged.fml.startup.Client", Libraries: []library{{Name: "net.neoforged.fancymodloader:loader:11.0.15"}, {Name: "net.neoforged:srgutils:1.0.10"}}}
	v.Arguments.Game = rawVersionArgs("--fml.neoForgeVersion", "26.1.2.114", "--fml.mcVersion", "26.1.2", "--fml.neoFormVersion", "test")
	v.JavaVersion.MajorVersion = 25
	writeTestFile(t, filepath.Join(root, "versions", v.ID, v.ID+".json"), jsonBytes(t, v))
	versions := scanVersions(root)
	if len(versions) != 1 {
		t.Fatal(versions)
	}
	got := versions[0]
	if got.ID != "project02" || got.Minecraft != "26.1.2" || got.Loader != "neoforge" || got.LoaderVersion != "26.1.2.114" || got.JavaVersion != 25 {
		t.Fatalf("renamed NeoForge metadata was not preserved: %+v", got)
	}
}

func TestLocalInheritedVersionIdentity(t *testing.T) {
	root := t.TempDir()
	base := gameVersion{ID: "1.21.1"}
	base.JavaVersion.MajorVersion = 21
	neo := gameVersion{ID: "neoforge-21.1.264", InheritsFrom: base.ID, Libraries: []library{{Name: "net.neoforged:neoforge:21.1.264"}}}
	custom := gameVersion{ID: "weekend", InheritsFrom: neo.ID}
	for _, v := range []gameVersion{base, neo, custom} {
		writeTestFile(t, filepath.Join(root, "versions", v.ID, v.ID+".json"), jsonBytes(t, v))
	}
	for _, got := range scanVersions(root) {
		if got.ID == "weekend" {
			if got.Loader != "neoforge" || got.LoaderVersion != "21.1.264" || got.Minecraft != "1.21.1" || got.JavaVersion != 21 {
				t.Fatalf("inherited profile lost its loader or Minecraft version: %+v", got)
			}
			return
		}
	}
	t.Fatal("renamed inherited profile missing")
}

func TestVersionIdentityFromConditionalAndJVMArguments(t *testing.T) {
	v := gameVersion{ID: "custom", MainClass: "net.neoforged.fml.startup.Client"}
	v.Arguments.Game = []json.RawMessage{json.RawMessage(`{"rules":[{"action":"allow"}],"value":["--fml.mcVersion=26.1.2"]}`)}
	v.Arguments.JVM = rawVersionArgs("-Dfml.neoForgeVersion=26.1.2.114")
	if detectLoader(v) != "neoforge" || minecraftVersionOf(v) != "26.1.2" || loaderVersion(v, "neoforge", "26.1.2") != "26.1.2.114" {
		t.Fatal("conditional or JVM version arguments were ignored")
	}
}
