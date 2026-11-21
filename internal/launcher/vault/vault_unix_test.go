//go:build darwin || linux

package vault

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	keyring "github.com/zalando/go-keyring"
)

func TestCredentialRoundTripAndIsolation(t *testing.T) {
	keyring.MockInit()
	first, second := t.TempDir(), t.TempDir()
	secret := []byte(`{"access":"test-only-access-token","refresh":"` + strings.Repeat("r", 8192) + `"}`)
	if err := Store(first, "microsoft-123", secret); err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(secretPath(first, "microsoft-123"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stored, []byte("test-only-access-token")) || bytes.Contains(stored, []byte(strings.Repeat("r", 64))) {
		t.Fatal("credential file contains plaintext")
	}
	got, err := Load(first, "microsoft-123")
	if err != nil || !bytes.Equal(got, secret) {
		t.Fatalf("credential roundtrip failed: %v", err)
	}
	if _, err := Load(second, "microsoft-123"); err == nil {
		t.Fatal("credential leaked into another launcher data directory")
	}
	if _, err := Load(first, "third-party-123"); err == nil {
		t.Fatal("credential leaked into another account")
	}
	stored[len(stored)-1] ^= 1
	if err := os.WriteFile(secretPath(first, "microsoft-123"), stored, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(first, "microsoft-123"); err == nil {
		t.Fatal("tampered credential was accepted")
	}
	if err := Delete(first, "microsoft-123"); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(first, "microsoft-123"); err == nil {
		t.Fatal("deleted credential remained available")
	}
	if err := Delete(first, "microsoft-123"); err != nil {
		t.Fatalf("deleting a missing credential should succeed: %v", err)
	}
}

func TestUnavailableKeyringFallback(t *testing.T) {
	keyring.MockInitWithError(errors.New("keyring unavailable"))
	dir := t.TempDir()
	if runtime.GOOS == "linux" {
		secret := []byte("test-only-token")
		if err := Store(dir, "account", secret); err != nil {
			t.Fatalf("Linux should save without a desktop keyring: %v", err)
		}
		keyInfo, err := os.Stat(localKeyPath(dir))
		if err != nil || keyInfo.Mode().Perm() != 0600 {
			t.Fatalf("fallback key is missing or not owner-only: %v", err)
		}
		stored, err := os.ReadFile(secretPath(dir, "account"))
		if err != nil || bytes.Contains(stored, secret) {
			t.Fatalf("fallback credential file contains plaintext or is unreadable: %v", err)
		}
		got, err := Load(dir, "account")
		if err != nil || !bytes.Equal(got, secret) {
			t.Fatalf("fallback credential did not survive reload: %v", err)
		}
		return
	}
	if err := Store(dir, "account", []byte("test-only-token")); err == nil {
		t.Fatal("missing keyring did not fail closed")
	}
	if _, err := Load(dir, "account"); err == nil {
		t.Fatal("missing keyring was read as an empty credential")
	}
	if _, err := os.Stat(secretPath(dir, "account")); !os.IsNotExist(err) {
		t.Fatal("missing keyring left a plaintext or encrypted credential file")
	}
	if err := os.MkdirAll(filepath.Join(dir, "credentials"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secretPath(dir, "account"), []byte("encrypted-fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir, "account"); err == nil || !strings.Contains(err.Error(), "凭据库不可用") {
		t.Fatalf("unavailable keyring was not reported while reading a credential: %v", err)
	}
}
