package buildinfo

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func signedPolicyFixture(t *testing.T, version, minimum, target string) ([]byte, string) {
	t.Helper()
	pub, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	p := UpdatePolicy{Version: version, MinimumVersion: minimum}
	payload := "YoloMCL update policy v1\n" + Repository + "\n" + target + "\n" + version + "\n" + minimum + "\n"
	p.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, []byte(payload)))
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return data, base64.StdEncoding.EncodeToString(pub)
}

func TestUpdatePolicyAuthenticationAndVersionFloor(t *testing.T) {
	data, key := signedPolicyFixture(t, "1.10.0", "1.2.0", "windows-amd64")
	p, err := DecodeUpdatePolicy(data, "windows-amd64", key)
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"v0.9.9", "1.1.99", "1.2.0-rc.1"} {
		if !p.RequiresUpdate(version) {
			t.Errorf("retired version is allowed: %s", version)
		}
	}
	for _, version := range []string{"v1.2.0", "1.9.0", "1.10.0", "2.0.0", "dev", ""} {
		if p.RequiresUpdate(version) {
			t.Errorf("supported/development version was retired: %s", version)
		}
	}
	for _, change := range []struct{ from, to string }{
		{"1.2.0", "1.10.0"}, {"1.10.0", "2.0.0"}, {p.Signature, ""},
	} {
		tampered := []byte(strings.Replace(string(data), change.from, change.to, 1))
		if _, err := DecodeUpdatePolicy(tampered, "windows-amd64", key); err == nil {
			t.Fatal("tampered policy was accepted")
		}
	}
	if _, err := DecodeUpdatePolicy(data, "linux-amd64", key); err == nil {
		t.Fatal("policy for a different target was accepted")
	}
	_, otherKey := signedPolicyFixture(t, "1.10.0", "1.2.0", "windows-amd64")
	if _, err := DecodeUpdatePolicy(data, "windows-amd64", otherKey); err == nil {
		t.Fatal("untrusted signing key was accepted")
	}
	for _, minimum := range []string{"", "2.0.0", "1.2.0-rc.1", "v1.2.0", "01.0.0"} {
		data, key := signedPolicyFixture(t, "1.10.0", minimum, "windows-amd64")
		if _, err := DecodeUpdatePolicy(data, "windows-amd64", key); err == nil {
			t.Errorf("invalid signed floor accepted: %q", minimum)
		}
	}
}

func TestUpdatePolicyCacheIsBoundedAndReverified(t *testing.T) {
	data, key := signedPolicyFixture(t, "1.2.0", "1.2.0", "windows-amd64")
	p, err := DecodeUpdatePolicy(data, "windows-amd64", key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "update-policy.json")
	for range 2 {
		if err := SaveUpdatePolicy(path, p); err != nil {
			t.Fatal(err)
		}
		cached, err := ReadUpdatePolicy(path, "windows-amd64", key)
		if err != nil || !cached.RequiresUpdate("1.1.0") {
			t.Fatalf("retirement lost after offline restart: %+v, %v", cached, err)
		}
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() > 512 {
		t.Fatalf("policy cache should contain only small signed metadata: %v, %v", info, err)
	}
	for _, bad := range []string{strings.Replace(string(data), "1.2.0", "0.0.0", -1), strings.Repeat(" ", MaxUpdatePolicySize+1)} {
		if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadUpdatePolicy(path, "windows-amd64", key); err == nil {
			t.Fatal("tampered or oversized local policy was trusted")
		}
	}
}

func TestUpdatePolicySemanticVersionOrdering(t *testing.T) {
	ordered := []string{
		"0.0.0", "1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta",
		"1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.0.1",
		"1.9.0", "1.10.0", "2.0.0", "99999999999999999999.0.0", "100000000000000000000.0.0",
	}
	for i, a := range ordered {
		if !ValidTag("v"+a) || CompareVersions("v"+a, a) != 0 {
			t.Fatalf("invalid version fixture or prefix comparison: %s", a)
		}
		for _, b := range ordered[i+1:] {
			if CompareVersions(a, b) >= 0 || CompareVersions(b, a) <= 0 {
				t.Fatalf("incorrect version order: %s, %s", a, b)
			}
		}
	}
}
