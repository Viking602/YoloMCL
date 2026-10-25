package buildinfo

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Exercise the real pinned MyGo updater in an isolated child executable. No
// user launcher, settings or game directories are involved in this test.
func TestArchivesInstallWithMyGoAndRejectTampering(t *testing.T) {
	openssl := os.Getenv("OPENSSL")
	if openssl == "" {
		openssl = "openssl"
	}
	for _, tool := range []string{"bash", openssl} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip("run make check with Bash and OpenSSL to exercise script-generated updates")
		}
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	assets := t.TempDir()
	server := httptest.NewServer(http.FileServer(http.Dir(assets)))
	defer server.Close()
	work := t.TempDir()
	source := filepath.Join(work, "probe.go")
	if err := os.WriteFile(source, []byte(`package main
import ("context"; "fmt"; "os"; "time"; "github.com/egoist/mygo")
func main() {
 mygo.App.SetVersion("1.0.0")
 ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second); defer cancel()
 up, err := mygo.Updater.Check(ctx)
 if err == nil && up == nil { err = fmt.Errorf("no update offered") }
 if err == nil { err = up.Install(ctx, nil) }
 if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	name := "YoloMCL"
	if runtime.GOOS == "windows" {
		name += ".exe"
	} else if runtime.GOOS == "darwin" {
		name = "YoloMCL.app/Contents/MacOS/YoloMCL"
	}
	exe := filepath.Join(work, "installed", filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	flags := "-X github.com/egoist/mygo.production=1 -X github.com/egoist/mygo.packageUpdateFeed=" + server.URL + "/feed.json" +
		" -X github.com/egoist/mygo.packageUpdateKey=" + base64.StdEncoding.EncodeToString(public)
	build := exec.CommandContext(ctx, "go", "build", "-tags=mygo_noinspector", "-ldflags", flags, "-o", exe, source)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build isolated updater: %v\n%s", err, output)
	}
	original, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(work, "stage")
	publisher := filepath.Join(work, "publisher")
	for path, content := range map[string][]byte{
		filepath.Join(stage, filepath.FromSlash(name)):                             []byte("verified replacement"),
		filepath.Join(stage, "LICENSE"):                                            []byte("test license"),
		filepath.Join(stage, "THIRD_PARTY_NOTICES.md"):                             []byte("test notices"),
		filepath.Join(publisher, "internal", "buildinfo", "info.go"):               []byte("const Repository = \"" + Repository + "\""),
		filepath.Join(publisher, "internal", "buildinfo", "update-public-key.txt"): []byte(base64.StdEncoding.EncodeToString(public)),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	target := runtime.GOOS + "-" + runtime.GOARCH
	pack := exec.CommandContext(ctx, "bash", filepath.ToSlash(filepath.Join("..", "..", "scripts", "test-update.sh")),
		stage, assets, filepath.Join(publisher, "internal", "buildinfo", "update-public-key.txt"),
		"v1.1.0", strings.Repeat("a", 40), target, "1.1.0")
	pack.Env = append(os.Environ(), "MYGO_UPDATER_PRIVATE_KEY="+base64.StdEncoding.EncodeToString(private))
	if output, err := pack.CombinedOutput(); err != nil {
		t.Fatalf("package signed update with release script: %v\n%s", err, output)
	}
	data, err := os.ReadFile(filepath.Join(assets, "update-"+target+".json"))
	if err != nil {
		t.Fatal(err)
	}
	policy, err := DecodeUpdatePolicy(data, target, base64.StdEncoding.EncodeToString(public))
	if err != nil || !policy.RequiresUpdate("v1.0.0") || policy.RequiresUpdate("v1.1.0") {
		t.Fatalf("script-generated policy is not interoperable: %+v, %v", policy, err)
	}
	var manifest struct {
		Version         string `json:"version"`
		Notes           string `json:"notes"`
		Date            string `json:"date"`
		URL             string `json:"url"`
		Size            int64  `json:"size"`
		Signature       string `json:"signature"`
		Commit          string `json:"commit"`
		MinimumVersion  string `json:"minimumVersion"`
		PolicySignature string `json:"policySignature"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	archiveName := "YoloMCL-1.1.0-" + target + ".tar.gz"
	manifest.URL = server.URL + "/" + archiveName
	data, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "feed.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(assets, archiveName)
	signed, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	tampered := append([]byte(nil), signed...)
	tampered[len(tampered)/2] ^= 1
	if err := os.WriteFile(archive, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.CommandContext(ctx, exe).CombinedOutput(); err == nil || !strings.Contains(string(output), "not signed") {
		t.Fatalf("tampered update was not rejected by MyGo: %v, %s", err, output)
	}
	if data, err := os.ReadFile(exe); err != nil || string(data) != string(original) {
		t.Fatal("failed signature verification modified the installed application")
	}
	if err := os.WriteFile(archive, signed, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.CommandContext(ctx, exe).CombinedOutput(); err != nil {
		t.Fatalf("install signed update: %v\n%s", err, output)
	}
	if data, err := os.ReadFile(exe); err != nil || string(data) != "verified replacement" {
		t.Fatal("signed archive did not replace the isolated executable")
	}
}
