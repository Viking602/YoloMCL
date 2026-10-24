package config

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestPublisherConfigurationIsSeparateFromPlayerSettings(t *testing.T) {
	old := encoded
	t.Cleanup(func() { encoded = old })
	encoded = base64.StdEncoding.EncodeToString([]byte(`{"microsoftClientId":"12345678-1234-1234-1234-123456789abc"}`))
	t.Setenv("MCL_DEVELOPMENT", "")
	t.Setenv("CURSEFORGE_API_KEY", "")
	t.Chdir(t.TempDir())
	t.Setenv("MCL_CURSEFORGE_API_KEY", "not-for-the-client")
	c, e := Load()
	if e != nil {
		t.Fatal(e)
	}
	if !c.HasMicrosoft() || c.HasCurseForge() || c.CurseForgeAPIKey != "" {
		t.Fatal("publisher configuration not loaded safely")
	}
	c.CurseForgeAPIKey = "private-publisher-key"
	b, _ := json.Marshal(c)
	if strings.Contains(string(b), "private-publisher-key") {
		t.Fatal("private key must never be serialized")
	}
}
func TestPublisherValidation(t *testing.T) {
	c := Config{MicrosoftClientID: "enter-your-id"}
	if c.Validate() == nil {
		t.Fatal("invalid Microsoft app identity accepted")
	}
}

func TestBuildInjectedCurseForgeKeyIsNotPublic(t *testing.T) {
	old := encoded
	t.Cleanup(func() { encoded = old })
	t.Setenv("MCL_DEVELOPMENT", "")
	t.Setenv("CURSEFORGE_API_KEY", "")
	t.Chdir(t.TempDir())
	encoded = base64.StdEncoding.EncodeToString([]byte(`{"curseForgeApiKey":"publisher-build-token"}`))
	c, e := Load()
	if e != nil || !c.HasCurseForge() || c.CurseForgeAPIKey != "publisher-build-token" {
		t.Fatal("build-injected key not available to backend", e)
	}
	b, _ := json.Marshal(c)
	if strings.Contains(string(b), "publisher-build-token") {
		t.Fatal("build credentials entered public configuration")
	}
}

func TestCurseForgeKeyFromEnv(t *testing.T) {
	t.Setenv("MCL_DEVELOPMENT", "")
	t.Setenv("CURSEFORGE_API_KEY", " env-key ")
	c, e := Load()
	if e != nil || c.CurseForgeAPIKey != "env-key" {
		t.Fatalf("%v %q", e, c.CurseForgeAPIKey)
	}
	b, _ := json.Marshal(c)
	if strings.Contains(string(b), "env-key") {
		t.Fatal("key leaked into JSON")
	}
}

func TestEmbeddedCurseForgeKeyAndOverride(t *testing.T) {
	old := cfKeyObf
	t.Cleanup(func() { cfKeyObf = old })
	t.Setenv("MCL_DEVELOPMENT", "")
	t.Chdir(t.TempDir())
	t.Setenv("CURSEFORGE_API_KEY", "")
	cfKeyObf = hex.EncodeToString(xorMask([]byte("embedded-test-key")))
	c, e := Load()
	if e != nil || c.CurseForgeAPIKey != "embedded-test-key" {
		t.Fatalf("embedded key not decoded: %v", e)
	}
	t.Setenv("CURSEFORGE_API_KEY", "env-wins")
	c, _ = Load()
	if c.CurseForgeAPIKey != "env-wins" {
		t.Fatal("env must override embedded key")
	}
}
