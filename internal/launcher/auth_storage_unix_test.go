//go:build darwin || linux

package launcher

import (
	"bytes"
	"errors"
	"os"
	"runtime"
	"testing"

	keyring "github.com/zalando/go-keyring"
)

func TestOnlineAccountPersistenceAcrossRestart(t *testing.T) {
	if runtime.GOOS == "linux" {
		keyring.MockInitWithError(errors.New("Secret Service unavailable"))
	} else {
		keyring.MockInit()
	}
	for _, kind := range []string{"microsoft", "yggdrasil"} {
		t.Run(kind, func(t *testing.T) {
			s := testService(t)
			a := Account{ID: kind + "-fixture", Name: "TestPlayer", Kind: kind}
			secret := tokenSecret{Access: kind + "-test-only-access-token", Refresh: kind + "-test-only-refresh-token"}
			if err := s.saveAccount(a, secret); err != nil {
				t.Fatal(err)
			}
			stored, err := os.ReadFile(s.secretPath(a.ID))
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(stored, []byte(secret.Access)) || bytes.Contains(stored, []byte(secret.Refresh)) {
				t.Fatal("saved account credential contains plaintext")
			}
			restarted, err := New(s.dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(restarted.Snapshot().Accounts) != 1 {
				t.Fatal("account did not persist across restart")
			}
			var restored tokenSecret
			if err := restarted.getSecret(a.ID, &restored); err != nil || restored != secret {
				t.Fatalf("credential did not persist across restart: %v", err)
			}
			if err := restarted.RemoveAccount(a.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(s.secretPath(a.ID)); !os.IsNotExist(err) {
				t.Fatal("removed account retained its credential file")
			}
		})
	}
}
