package launcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAuthServerPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, e := New(dir)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := s.SaveAuthServer("", "Skin A", " https://skin-a.example/api/yggdrasil/ "); e != nil {
		t.Fatal(e)
	}
	if _, e := s.SaveAuthServer("", "", "https://auth.example.com/api"); e != nil {
		t.Fatal(e)
	}
	// rename via previous URL
	if _, e := s.SaveAuthServer("https://auth.example.com/api", "Example", "https://auth.example.com/api"); e != nil {
		t.Fatal(e)
	}
	s2, e := New(dir)
	if e != nil {
		t.Fatal(e)
	}
	got := s2.Snapshot().AuthServers[len(BuiltinAuthServers):]
	if len(got) != 2 || got[0].Name != "Skin A" || got[0].URL != "https://skin-a.example/api/yggdrasil" || got[1].Name != "Example" {
		t.Fatalf("round trip: %+v", got)
	}
	if e := s2.DeleteAuthServer("https://skin-a.example/api/yggdrasil/"); e != nil {
		t.Fatal(e)
	}
	if got := s2.Snapshot().AuthServers[len(BuiltinAuthServers):]; len(got) != 1 || got[0].Name != "Example" {
		t.Fatalf("delete: %+v", got)
	}
}

func TestAuthServerURLValidation(t *testing.T) {
	for _, bad := range []string{"", "http://insecure.example.com", "ftp://x", "https://user:pw@x.com", "not a url", "https://"} {
		if _, e := NormalizeAuthServerURL(bad); e == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
	if v, e := NormalizeAuthServerURL("https://a.b/c/"); e != nil || v != "https://a.b/c" {
		t.Errorf("normalize: %q %v", v, e)
	}
}

// Configs written before AuthServers existed still load, and the server of
// an existing third-party account shows up in the saved list.
func TestAuthServerBackwardCompat(t *testing.T) {
	dir := t.TempDir()
	s, e := New(dir)
	if e != nil {
		t.Fatal(e)
	}
	s.mu.Lock()
	s.cfg.Accounts = append(s.cfg.Accounts, Account{ID: "yg-1", Name: "Old", UUID: "0123456789abcdef0123456789abcdef", Kind: "yggdrasil", Server: "https://skin.old.example/api/yggdrasil"})
	e = s.saveConfigLocked(s.cfg)
	s.mu.Unlock()
	if e != nil {
		t.Fatal(e)
	}
	// make sure the file has no authServers key (old format)
	matches, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	for _, m := range matches {
		b, _ := os.ReadFile(m)
		var raw map[string]any
		if json.Unmarshal(b, &raw) == nil {
			if _, ok := raw["authServers"]; ok {
				t.Errorf("%s: empty list should be omitted", m)
			}
		}
	}
	s2, e := New(dir)
	if e != nil {
		t.Fatal(e)
	}
	got := s2.Snapshot().AuthServers[len(BuiltinAuthServers):]
	if len(got) != 1 || got[0].URL != "https://skin.old.example/api/yggdrasil" || got[0].Name != "skin.old.example" {
		t.Fatalf("legacy account server not listed: %+v", got)
	}
	if len(s2.Snapshot().Accounts) != 1 {
		t.Fatal("legacy account lost")
	}
}
