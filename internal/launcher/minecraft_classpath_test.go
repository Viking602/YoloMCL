package launcher

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestPrepareVersionDeduplicatesClasspath(t *testing.T) {
	s := testService(t)
	root := t.TempDir()
	gson := library{Name: "com.google.code.gson:gson:2.10.1"}
	gson.Downloads.Artifact = &artifact{Path: "com/google/code/gson/gson/2.10.1/gson-2.10.1.jar"}
	guava := library{Name: "com.google.guava:guava:32.1.2-jre"}
	guava.Downloads.Artifact = &artifact{Path: "com/google/guava/guava/32.1.2-jre/guava-32.1.2-jre.jar"}
	newGson := library{Name: "com.google.code.gson:gson:2.11.0"}
	newGson.Downloads.Artifact = &artifact{Path: "com/google/code/gson/gson/2.11.0/gson-2.11.0.jar"}
	// Different coordinates and path separators can still identify the same JAR.
	alias := library{Name: "example:gson-alias:1.0"}
	alias.Downloads.Artifact = &artifact{Path: strings.ReplaceAll(gson.Downloads.Artifact.Path, "/", "\\")}
	v := gameVersion{ID: "merged-neoforge", Libraries: []library{gson, guava, gson, newGson, guava, alias}}
	if runtime.GOOS == "windows" {
		upper := library{Name: gson.Name}
		upper.Downloads.Artifact = &artifact{Path: strings.ToUpper(gson.Downloads.Artifact.Path)}
		v.Libraries = append(v.Libraries, upper)
	}
	var want []string
	for _, l := range []library{gson, guava, newGson} {
		path := filepath.Join(root, "libraries", filepath.FromSlash(l.Downloads.Artifact.Path))
		writeTestFile(t, path, zipBytes(t, map[string][]byte{"fixture.txt": []byte(l.Name)}))
		want = append(want, path)
	}
	client := filepath.Join(root, "versions", v.ID, v.ID+".jar")
	writeTestFile(t, client, zipBytes(t, map[string][]byte{"fixture.txt": []byte("client")}))
	want = append(want, client)
	fakeHTTP(t, s, map[string][]byte{})

	cp, _, err := s.prepareVersion(context.Background(), root, v, func(string, int, int) {})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cp, want) {
		t.Fatalf("classpath must contain each JAR once in first-seen order, keeping distinct versions:\ngot  %v\nwant %v", cp, want)
	}
}

func TestPrepareVersionDuplicateLibraryKeepsNatives(t *testing.T) {
	s := testService(t)
	root := t.TempDir()
	lib := library{Name: "example:library:1.0"}
	lib.Downloads.Artifact = &artifact{Path: "example/library/1.0/library-1.0.jar"}
	withNatives := lib
	withNatives.Natives = map[string]string{osName(): "natives-test"}
	nativeBytes := zipBytes(t, map[string][]byte{"fixture-native.bin": []byte("native payload")})
	nativeSHA, _ := hashes(nativeBytes)
	native := artifact{Path: "example/library/1.0/library-1.0-natives-test.jar", SHA1: nativeSHA, URL: "https://fixture.test/natives"}
	withNatives.Downloads.Classifiers = map[string]artifact{"natives-test": native}
	// A disallowed declaration must not hide a later allowed declaration.
	disallowed := lib
	disallowed.Rules = []rule{{Action: "disallow"}}
	serverOnly := lib
	clientReq := false
	serverOnly.ClientReq = &clientReq
	v := gameVersion{ID: "fixture", Libraries: []library{disallowed, serverOnly, lib, withNatives}}
	libPath := filepath.Join(root, "libraries", filepath.FromSlash(lib.Downloads.Artifact.Path))
	writeTestFile(t, libPath, zipBytes(t, map[string][]byte{"fixture.txt": []byte("library")}))
	writeTestFile(t, filepath.Join(root, "libraries", filepath.FromSlash(native.Path)), nativeBytes)
	client := filepath.Join(root, "versions", v.ID, v.ID+".jar")
	writeTestFile(t, client, zipBytes(t, map[string][]byte{"fixture.txt": []byte("client")}))
	fakeHTTP(t, s, map[string][]byte{})

	cp, natives, err := s.prepareVersion(context.Background(), root, v, func(string, int, int) {})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{libPath, client}; !slices.Equal(cp, want) {
		t.Fatalf("unexpected classpath: got %v, want %v", cp, want)
	}
	payload, err := os.ReadFile(filepath.Join(natives, "fixture-native.bin"))
	if err != nil || string(payload) != "native payload" {
		t.Fatalf("duplicate library lost its native artifact: %q, %v", payload, err)
	}
}
