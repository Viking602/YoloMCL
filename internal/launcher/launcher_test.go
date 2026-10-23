package launcher

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func testService(t *testing.T) *Service {
	t.Helper()
	t.Setenv("CURSEFORGE_API_KEY", "")
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func fakeHTTP(t *testing.T, s *Service, routes map[string][]byte) {
	t.Helper()
	s.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		b, ok := routes[r.URL.String()]
		if !ok {
			return nil, fmt.Errorf("unexpected network request: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(b)), Request: r}, nil
	})}
}
func jsonBytes(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func writeTestFile(t *testing.T, path string, b []byte) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, b, 0644); e != nil {
		t.Fatal(e)
	}
}
func newTestRoot(t *testing.T, s *Service, name string) Root {
	t.Helper()
	parent := t.TempDir()
	p := filepath.Join(parent, name, ".minecraft")
	if e := os.MkdirAll(p, 0755); e != nil {
		t.Fatal(e)
	}
	r, e := s.AddRoot(filepath.Dir(p), name)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func waitJob(t *testing.T, s *Service, id string) Job {
	t.Helper()
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		j := s.jobs[id].Job
		s.mu.Unlock()
		if j.State != "running" {
			return j
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return Job{}
}
func zipBytes(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, data := range files {
		f, e := w.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = f.Write(data); e != nil {
			t.Fatal(e)
		}
	}
	if e := w.Close(); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func fabricJar(t *testing.T, id, version string) []byte {
	return zipBytes(t, map[string][]byte{"fabric.mod.json": jsonBytes(t, map[string]string{"id": id, "name": id, "version": version})})
}
func hashes(b []byte) (string, string) {
	a, c := sha1.Sum(b), sha512.Sum512(b)
	return hex.EncodeToString(a[:]), hex.EncodeToString(c[:])
}
func installFabricFixture(t *testing.T, s *Service, r Root) {
	t.Helper()
	v := gameVersion{ID: "fabric-test", InheritsFrom: "1.21.1", Libraries: []library{{Name: "net.fabricmc:fabric-loader:0.16.0"}}}
	writeTestFile(t, filepath.Join(r.Path, "versions", v.ID, v.ID+".json"), jsonBytes(t, v))
	if e := s.Select(r.ID, "", v.ID); e != nil {
		t.Fatal(e)
	}
}

func TestOfflineIdentityPersistenceAndEmptySnapshot(t *testing.T) {
	s := testService(t)
	b, e := s.Dispatch(context.Background(), "snapshot", "{}")
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(b, `"accounts":null`) {
		t.Fatal("empty accounts must serialize as an array")
	}
	a, e := s.AddOffline("Notch")
	if e != nil {
		t.Fatal(e)
	}
	if a.UUID != "b50ad385829d3141a2167e7d7539ba7f" {
		t.Fatalf("offline UUID mismatch: %s", a.UUID)
	}
	if _, e = s.AddOffline("Notch"); e != nil {
		t.Fatal(e)
	}
	if len(s.Snapshot().Accounts) != 1 {
		t.Fatal("duplicate offline account")
	}
	if _, e = s.AddOffline("bad name!"); e == nil {
		t.Fatal("invalid username accepted")
	}
	loaded, e := New(s.dir)
	if e != nil {
		t.Fatal(e)
	}
	if loaded.Snapshot().Accounts[0].UUID != a.UUID {
		t.Fatal("UUID changed after restart")
	}
	token, e := loaded.accountToken(context.Background(), a)
	if e != nil || token.Access != "0" {
		t.Fatal("offline login must not require a credential or network")
	}
	if _, e = os.Stat(loaded.secretPath(a.ID)); !os.IsNotExist(e) {
		t.Fatal("offline accounts should not store authentication credentials")
	}
}
func TestFolderIsolationAndPathBoundaries(t *testing.T) {
	s := testService(t)
	a, b := newTestRoot(t, s, "Alpha"), newTestRoot(t, s, "Beta")
	writeTestFile(t, filepath.Join(a.Path, "mods", "example.jar"), fabricJar(t, "example", "1"))
	mods, e := s.ListMods(b.ID)
	if e != nil || len(mods) != 0 {
		t.Fatal("mod files leaked between roots")
	}
	duplicate, e := s.AddRoot(a.Path, "")
	if e != nil || duplicate.ID != a.ID || len(s.Snapshot().Instances) != 2 {
		t.Fatal("direct .minecraft path should deduplicate the parent path")
	}
	for _, path := range []string{"../escape.jar", "mods/../../escape", "C:/escape", "C:escape", "mods\\..\\escape", "/absolute", "\\server\\share", "mods/CON.jar", "mods/x.jar:stream", "mods/end. /x"} {
		if _, e := safeJoin(a.Path, path); e == nil {
			t.Errorf("unsafe path accepted: %q", path)
		}
	}
	if e = s.RemoveRoot(a.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(a.Path, "mods", "example.jar")); e != nil {
		t.Fatal("removing a registration deleted user files")
	}
}
func TestDownloadIntegrityPreservesExistingFile(t *testing.T) {
	s := testService(t)
	dest := filepath.Join(t.TempDir(), "mod.jar")
	writeTestFile(t, dest, []byte("known good"))
	expected, _ := hashes([]byte("new verified data"))
	fakeHTTP(t, s, map[string][]byte{"https://fixture.test/file": []byte("corrupt")})
	e := s.download(context.Background(), DownloadFile{Name: "mod.jar", URL: "https://fixture.test/file", SHA1: expected}, dest)
	if e == nil {
		t.Fatal("checksum failure was ignored")
	}
	b, _ := os.ReadFile(dest)
	if string(b) != "known good" {
		t.Fatal("failed download replaced the original")
	}
	files, _ := os.ReadDir(filepath.Dir(dest))
	if len(files) != 1 {
		t.Fatal("temporary download files were not cleaned up")
	}
}
func TestModUpdateRollsBackAndPreservesDisabledState(t *testing.T) {
	s := testService(t)
	r := newTestRoot(t, s, "Mods")
	installFabricFixture(t, s, r)
	old, newMod, dependency := fabricJar(t, "example", "1"), fabricJar(t, "example", "2"), fabricJar(t, "dependency", "1")
	oldHash, _ := hashes(old)
	newHash, _ := hashes(newMod)
	depHash, _ := hashes(dependency)
	writeTestFile(t, filepath.Join(r.Path, "mods", "old.jar.disabled"), old)
	if e := atomicJSON(filepath.Join(r.Path, ".mcl", "mods.json"), map[string]modRecord{"old.jar": {SHA1: oldHash, Source: "modrinth", ProjectID: "project", VersionID: "old"}}); e != nil {
		t.Fatal(e)
	}
	makeVersion := func(id, project, name, hash string, deps []map[string]string) []byte {
		return jsonBytes(t, map[string]any{"id": id, "project_id": project, "version_number": id, "name": id, "game_versions": []string{"1.21.1"}, "loaders": []string{"fabric"}, "files": []map[string]any{{"filename": name, "url": "https://fixture.test/" + name, "hashes": map[string]string{"sha1": hash}}}, "dependencies": deps})
	}
	fakeHTTP(t, s, map[string][]byte{modrinthAPI + "/version/new": makeVersion("new", "project", "new.jar", newHash, []map[string]string{{"project_id": "dep", "version_id": "depv", "dependency_type": "required"}}), modrinthAPI + "/version/depv": makeVersion("depv", "dep", "dep.jar", depHash, nil), "https://fixture.test/new.jar": newMod, "https://fixture.test/dep.jar": dependency})
	if e := os.Mkdir(filepath.Join(r.Path, "mods", "new.jar.disabled"), 0755); e != nil {
		t.Fatal(e)
	}
	id, e := s.InstallMod(r.ID, "modrinth", "project", "new", "old.jar.disabled")
	if e != nil {
		t.Fatal(e)
	}
	j := waitJob(t, s, id)
	if j.State != "failed" {
		t.Fatalf("expected commit failure, got %s", j.State)
	}
	if !checkHashes(filepath.Join(r.Path, "mods", "old.jar.disabled"), oldHash, "") {
		t.Fatal("rollback did not restore the original mod")
	}
	if _, e = os.Stat(filepath.Join(r.Path, "mods", "dep.jar")); !os.IsNotExist(e) {
		t.Fatal("rollback left a partially installed dependency")
	}
	if e = os.Remove(filepath.Join(r.Path, "mods", "new.jar.disabled")); e != nil {
		t.Fatal(e)
	}
	id, e = s.InstallMod(r.ID, "modrinth", "project", "new", "old.jar.disabled")
	if e != nil {
		t.Fatal(e)
	}
	j = waitJob(t, s, id)
	if j.State != "completed" {
		t.Fatal(j.Message)
	}
	if !checkHashes(filepath.Join(r.Path, "mods", "new.jar.disabled"), newHash, "") {
		t.Fatal("update lost the disabled state")
	}
	if !checkHashes(filepath.Join(r.Path, "mods", "dep.jar"), depHash, "") {
		t.Fatal("required dependency was not installed and enabled")
	}
	matches, _ := filepath.Glob(filepath.Join(r.Path, ".mcl", "backups", "*", "old.jar.disabled"))
	if len(matches) == 0 {
		t.Fatal("old mod was not backed up")
	}
}
func TestMrpackImportLayersAndLockVerification(t *testing.T) {
	s := testService(t)
	payload := fabricJar(t, "example", "1")
	sha, sha512 := hashes(payload)
	index := map[string]any{"formatVersion": 1, "game": "minecraft", "versionId": "pack-v1", "name": "Fixture Pack", "dependencies": map[string]string{"minecraft": "1.21.1"}, "files": []map[string]any{{"path": "mods/example.jar", "hashes": map[string]string{"sha1": sha, "sha512": sha512}, "downloads": []string{"https://fixture.test/mod"}}, {"path": "server-only.jar", "hashes": map[string]string{"sha1": sha, "sha512": sha512}, "downloads": []string{"https://fixture.test/never"}, "env": map[string]string{"client": "unsupported"}}}}
	packPath := filepath.Join(t.TempDir(), "fixture.mrpack")
	writeTestFile(t, packPath, zipBytes(t, map[string][]byte{"modrinth.index.json": jsonBytes(t, index), "overrides/config/settings.txt": []byte("common"), "client-overrides/config/settings.txt": []byte("client"), "server-overrides/config/settings.txt": []byte("server")}))
	// the import now downloads the whole game, so the version has a client jar
	version := jsonBytes(t, gameVersion{ID: "1.21.1", Downloads: map[string]artifact{"client": {URL: "https://fixture.test/client", SHA1: sha}}})
	vh, _ := hashes(version)
	manifest := jsonBytes(t, map[string]any{"versions": []map[string]string{{"id": "1.21.1", "url": "https://fixture.test/version", "sha1": vh}}})
	fakeHTTP(t, s, map[string][]byte{manifestURL: manifest, "https://fixture.test/version": version, "https://fixture.test/mod": payload, "https://fixture.test/client": payload})
	parent := t.TempDir()
	id, e := s.ImportPack(ImportRequest{Path: packPath, Parent: parent, Name: "Imported"})
	if e != nil {
		t.Fatal(e)
	}
	job := waitJob(t, s, id)
	if job.State != "completed" {
		t.Fatal(job.Message)
	}
	root := s.Snapshot().Instances[0]
	b, e := os.ReadFile(filepath.Join(root.Path, "config", "settings.txt"))
	if e != nil || string(b) != "client" {
		t.Fatalf("client override layering failed: %s, %v", b, e)
	}
	if _, e = os.Stat(filepath.Join(root.Path, "server-only.jar")); !os.IsNotExist(e) {
		t.Fatal("server-only file was installed on client")
	}
	report, e := s.CheckPack(context.Background(), root.ID)
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range report.Files {
		if f.Status != "original" {
			t.Fatalf("imported file not recorded correctly: %+v", f)
		}
	}
	writeTestFile(t, filepath.Join(root.Path, "mods", "example.jar"), []byte("locally changed"))
	report, e = s.CheckPack(context.Background(), root.ID)
	if e != nil {
		t.Fatal(e)
	}
	if report.Files[0].Status != "modified" {
		t.Fatal("pack did not detect locally modified mod")
	}
}
func TestMaliciousPackNeverEscapesStaging(t *testing.T) {
	s := testService(t)
	sha, sha512 := hashes([]byte("bad"))
	index := map[string]any{"formatVersion": 1, "game": "minecraft", "name": "bad", "dependencies": map[string]string{"minecraft": "1.21.1"}, "files": []map[string]any{{"path": "../../escaped.jar", "hashes": map[string]string{"sha1": sha, "sha512": sha512}, "downloads": []string{"https://fixture.test/bad"}}}}
	parent := t.TempDir()
	pack := filepath.Join(t.TempDir(), "bad.mrpack")
	writeTestFile(t, pack, zipBytes(t, map[string][]byte{"modrinth.index.json": jsonBytes(t, index)}))
	fakeHTTP(t, s, map[string][]byte{})
	id, e := s.ImportPack(ImportRequest{Path: pack, Parent: parent, Name: "Bad"})
	if e != nil {
		t.Fatal(e)
	}
	if waitJob(t, s, id).State != "failed" {
		t.Fatal("unsafe pack accepted")
	}
	entries, e := os.ReadDir(parent)
	if e != nil {
		t.Fatal(e)
	}
	for _, en := range entries {
		if !strings.HasPrefix(en.Name(), ".mcl-import-") { // the stage is kept for 重试
			t.Fatalf("failed pack import left %s outside staging", en.Name())
		}
	}
	if e := s.RemoveJob(id); e != nil {
		t.Fatal(e)
	}
	if entries, e = os.ReadDir(parent); e != nil || len(entries) != 0 {
		t.Fatal("移除 did not clean the import stage")
	}
}
func TestOfflineLaunchWithRealJVM(t *testing.T) {
	java, e := exec.LookPath("java")
	if e != nil {
		t.Skip("Java is unavailable")
	}
	javac, e := exec.LookPath("javac")
	if e != nil {
		t.Skip("JDK compiler is unavailable")
	}
	jar, e := exec.LookPath("jar")
	if e != nil {
		t.Skip("JDK jar tool is unavailable")
	}
	s := testService(t)
	r := newTestRoot(t, s, "Offline Path With Spaces")
	work := t.TempDir()
	source := filepath.Join(work, "Fixture.java")
	writeTestFile(t, source, []byte(`public class Fixture { public static void main(String[] args) { for (String arg : args) System.out.println("ARG=" + arg); System.out.println("DIR=" + System.getProperty("user.dir")); } }`))
	if b, e := exec.Command(javac, source).CombinedOutput(); e != nil {
		t.Fatalf("javac: %v %s", e, b)
	}
	client := filepath.Join(r.Path, "versions", "fixture", "fixture.jar")
	if e = os.MkdirAll(filepath.Dir(client), 0755); e != nil {
		t.Fatal(e)
	}
	if b, e := exec.Command(jar, "cf", client, "-C", work, "Fixture.class").CombinedOutput(); e != nil {
		t.Fatalf("jar: %v %s", e, b)
	}
	v := gameVersion{ID: "fixture", MainClass: "Fixture", Type: "release"}
	j, e := inspectJava(context.Background(), java)
	if e != nil {
		t.Fatal(e)
	}
	v.JavaVersion.MajorVersion = j.Major
	for _, arg := range []string{"--username", "${auth_player_name}", "--uuid", "${auth_uuid}", "--accessToken", "${auth_access_token}", "--userType", "${user_type}", "--gameDir", "${game_directory}"} {
		v.Arguments.Game = append(v.Arguments.Game, jsonBytes(t, arg))
	}
	writeTestFile(t, filepath.Join(r.Path, "versions", "fixture", "fixture.json"), jsonBytes(t, v))
	if e = s.UpdateRoot(r.ID, r.Name, java, 512); e != nil {
		t.Fatal(e)
	}
	if e = s.Select(r.ID, "", "fixture"); e != nil {
		t.Fatal(e)
	}
	a, e := s.AddOffline("MCL_Tester")
	if e != nil {
		t.Fatal(e)
	}
	fakeHTTP(t, s, map[string][]byte{})
	id, e := s.Launch(r.ID, a.ID)
	if e != nil {
		t.Fatal(e)
	}
	if job := waitJob(t, s, id); job.State != "completed" {
		t.Fatal(job.Message)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		lines, e := s.Logs(r.ID)
		if e == nil && strings.Contains(strings.Join(lines, "\n"), "游戏已退出") {
			joined := strings.Join(lines, "\n")
			for _, expected := range []string{"ARG=MCL_Tester", "ARG=" + a.UUID, "ARG=0", "ARG=legacy", "ARG=" + r.Path, "DIR=" + r.Path} {
				if !strings.Contains(joined, expected) {
					t.Errorf("missing JVM argument %q in %s", expected, joined)
				}
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("offline JVM did not exit or persist its log")
}
func TestCredentialProtectionAndLogRedaction(t *testing.T) {
	if runtime.GOOS == "windows" {
		s := testService(t)
		secret := "test-only-secret-123456"
		if e := s.putSecret("fixture", secret); e != nil {
			t.Fatal(e)
		}
		b, e := os.ReadFile(s.secretPath("fixture"))
		if e != nil {
			t.Fatal(e)
		}
		if bytes.Contains(b, []byte(secret)) {
			t.Fatal("credential was stored as plaintext")
		}
		var restored string
		if e = s.getSecret("fixture", &restored); e != nil || restored != secret {
			t.Fatal("credential roundtrip failed")
		}
	}
	log := &gameLog{secret: "sensitive-token"}
	_, _ = log.Write([]byte("hello sensitive-"))
	_, _ = log.Write([]byte("token world\n"))
	if strings.Contains(strings.Join(log.read(), "\n"), "sensitive-token") {
		t.Fatal("split token was leaked in logs")
	}
}

func TestCachedMavenLibrariesSupportOfflinePreparation(t *testing.T) {
	s := testService(t)
	r := newTestRoot(t, s, "Cached")
	v := gameVersion{ID: "fixture", Jar: "fixture", Libraries: []library{{Name: "example:cached:1.0", URL: "https://fixture.test/"}}}
	client := filepath.Join(r.Path, "versions", "fixture", "fixture.jar")
	writeTestFile(t, client, zipBytes(t, map[string][]byte{"fixture.txt": []byte("client")}))
	lib := filepath.Join(r.Path, "libraries", "example", "cached", "1.0", "cached-1.0.jar")
	writeTestFile(t, lib, zipBytes(t, map[string][]byte{"fixture.txt": []byte("library")}))
	fakeHTTP(t, s, map[string][]byte{})
	cp, natives, e := s.prepareVersion(context.Background(), r.Path, v, func(string, int, int) {})
	if e != nil {
		t.Fatal(e)
	}
	if len(cp) != 2 || cp[0] != lib {
		t.Fatalf("cached library absent from classpath: %v", cp)
	}
	if natives != "" {
		os.RemoveAll(natives)
	}
}
