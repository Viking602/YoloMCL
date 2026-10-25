package nativeui

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/buildinfo"
)

func TestLauncherFeedAuthenticatesPolicyAndBoundsInput(t *testing.T) {
	pub, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := base64.StdEncoding.EncodeToString(pub)
	p := buildinfo.UpdatePolicy{Version: "1.2.0", MinimumVersion: "1.1.0"}
	payload := "YoloMCL update policy v1\n" + buildinfo.Repository + "\nwindows-amd64\n1.2.0\n1.1.0\n"
	p.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, []byte(payload)))
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, body string
		status     int
		valid      bool
		policy     bool
	}{
		{"signed", string(data), 200, true, true},
		{"legacy", `{"version":"1.2.0"}`, 200, true, false},
		{"tampered", strings.Replace(string(data), "1.1.0", "1.2.0", 1), 200, false, false},
		{"unsigned", `{"version":"1.2.0","minimumVersion":"1.2.0"}`, 200, false, false},
		{"null-policy", `{"version":"1.2.0","minimumVersion":null,"policySignature":null}`, 200, false, false},
		{"bad-version", `{"version":"invalid"}`, 200, false, false},
		{"oversized", string(data) + strings.Repeat(" ", buildinfo.MaxUpdatePolicySize), 200, false, false},
		{"http-error", string(data), 503, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Cache-Control") != "no-cache" {
					t.Error("startup check allowed a cached transport response")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			version, policy, err := fetchLauncherUpdatePolicy(context.Background(), server.Client(), server.URL, "windows-amd64", key)
			if (err == nil) != tc.valid || (policy != nil) != tc.policy {
				t.Fatalf("unexpected feed result: %q, %+v, %v", version, policy, err)
			}
			if tc.valid && version != "1.2.0" {
				t.Fatal("release identity was lost")
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, _, err := fetchLauncherUpdatePolicy(ctx, server.Client(), server.URL, "windows-amd64", key); err == nil {
				t.Fatal("cancelled feed request was accepted")
			}
		})
	}
}

func TestLauncherPolicyCannotRollBackFromStaleOrLegacyFeed(t *testing.T) {
	cached := &buildinfo.UpdatePolicy{Version: "1.3.0", MinimumVersion: "1.2.0"}
	for _, incoming := range []*buildinfo.UpdatePolicy{
		nil, {Version: "1.1.0", MinimumVersion: "0.0.0"}, {Version: "1.3.0", MinimumVersion: "0.0.0"},
	} {
		if got := newerLauncherPolicy(cached, incoming); got != cached || !got.RequiresUpdate("1.1.0") {
			t.Fatal("old policy or missing fields cleared a known retirement")
		}
	}
	changed := &buildinfo.UpdatePolicy{Version: "1.4.0", MinimumVersion: "1.0.0"}
	if got := newerLauncherPolicy(cached, changed); got != changed || got.RequiresUpdate("1.1.0") {
		t.Fatal("a newer authenticated release cannot intentionally change the floor")
	}
}
