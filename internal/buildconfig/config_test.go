package buildconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/distribution"
)

const testClientID = "12345678-1234-1234-1234-123456789abc"

func TestConfigurationPrecedenceAndScope(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "repo")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, value string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(parent, ".env"), "CURSEFORGE_API_KEY=outside-repo")
	empty := func(string) string { return "" }
	config, err := Load(root, "", empty)
	if err != nil || config.HasCurseForge() {
		t.Fatal("build must not read parent configuration", err)
	}
	write(filepath.Join(root, ".env"), "MS_CLIENT_ID="+testClientID+"\nCURSEFORGE_API_KEY='$literal`$(no-exec)' # comment\n")
	config, err = Load(root, "", empty)
	if err != nil || config.MicrosoftClientID != testClientID || config.CurseForgeAPIKey != "$literal`$(no-exec)" {
		t.Fatal("repository configuration was not loaded literally", err)
	}
	env := func(key string) string {
		if key == "MCL_CURSEFORGE_API_KEY" {
			return "environment-key"
		}
		return ""
	}
	config, err = Load(root, "", env)
	if err != nil || config.CurseForgeAPIKey != "environment-key" {
		t.Fatal("environment alias must override .env", err)
	}
	publisher := filepath.Join(root, "publisher.json")
	write(publisher, `{"curseForgeApiKey":"publisher-key"}`)
	config, err = Load(root, publisher, env)
	if err != nil || config.CurseForgeAPIKey != "publisher-key" || config.MicrosoftClientID != testClientID {
		t.Fatal("explicit publisher file must override environment and retain missing defaults", err)
	}
}

func TestBuildArgumentsAcrossPlatforms(t *testing.T) {
	config := distribution.Config{MicrosoftClientID: testClientID, CurseForgeAPIKey: "private-test-key"}
	for _, goos := range []string{"windows", "darwin", "linux"} {
		for _, release := range []bool{false, true} {
			args, err := Arguments(config, goos, "-tags=custom,fixture", "build/output", release)
			if err != nil {
				t.Fatal(err)
			}
			joined := strings.Join(args, " ")
			if strings.Contains(joined, "private-test-key") {
				t.Fatal("private key must not appear as plain text in linker arguments")
			}
			if strings.Contains(joined, "-H=windowsgui") != (goos == "windows") {
				t.Fatal("Windows linker flags leaked to another platform")
			}
			if strings.Contains(joined, "mygo_noinspector,custom,fixture") != release || strings.Contains(joined, "production=1") != release {
				t.Fatal("release mode must retain caller tags and enable both production switches")
			}
		}
	}
	if _, err := Arguments(distribution.Config{}, "windows", "", "out", true); err == nil {
		t.Fatal("release accepted missing publisher configuration")
	}
	config.CurseForgeAPIKey, config.CurseForgeProxyURL = "", "https://catalog.example.test"
	if _, err := Arguments(config, "linux", "", "out", true); err != nil {
		t.Fatal("explicit catalog relay must work on every platform", err)
	}
}
