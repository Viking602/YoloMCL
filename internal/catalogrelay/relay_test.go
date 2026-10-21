package catalogrelay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestRelayKeepsPublisherKeyOnServerAndLimitsRoutes(t *testing.T) {
	calls := 0
	h := New("publisher-secret", roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Scheme != "https" || r.URL.Host != "api.curseforge.com" || r.Header.Get("x-api-key") != "publisher-secret" {
			t.Fatal("incorrect upstream or key")
		}
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Fatal("player credentials forwarded")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"data":[]}`))}, nil
	}))
	req := httptest.NewRequest("GET", "https://catalog.test/mods/search?gameId=432&searchFilter=sodium", nil)
	req.Header.Set("Cookie", "player-data")
	req.Header.Set("Authorization", "Bearer player-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 || calls != 1 || strings.Contains(w.Body.String(), "publisher-secret") {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, path := range []string{"/user", "/download?url=https://evil.test", "/mods/abc", "/mods/123/files/123/delete"} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatal("non-catalog route accepted", path, w.Code)
		}
	}
	if calls != 1 {
		t.Fatal("rejected route contacted upstream")
	}
}
func TestRelayRejectsRedirectAndOversizedRequest(t *testing.T) {
	h := New("key", roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://other.test"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/mods/1", nil))
	if w.Code != 502 {
		t.Fatal(w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/fingerprints", strings.NewReader(strings.Repeat("x", (2<<20)+1))))
	if w.Code != 413 {
		t.Fatal(w.Code)
	}
}
