//go:build linux

package vault

import (
	"bytes"
	"encoding/base64"
	"os"
	"testing"

	keyring "github.com/zalando/go-keyring"
)

func TestExistingSecretServiceKeyMigratesToLocalFallback(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	key := bytes.Repeat([]byte{0x42}, 32)
	if err := keyring.Set(keyringService, keyringAccount(dir), base64.StdEncoding.EncodeToString(key)); err != nil {
		t.Fatal(err)
	}
	secret := []byte("test-only-migrated-token")
	if err := Store(dir, "account", secret); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(localKeyPath(dir)); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(dir, "account"); err != nil || !bytes.Equal(got, secret) {
		t.Fatalf("old Secret Service credential did not migrate: %v", err)
	}
	keyring.MockInitWithError(os.ErrPermission)
	if got, err := Load(dir, "account"); err != nil || !bytes.Equal(got, secret) {
		t.Fatalf("migrated credential still depends on Secret Service: %v", err)
	}
}
