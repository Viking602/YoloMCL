package launcher

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	publisherconfig "github.com/Viking602/YoloMCL/internal/config"
)

func TestModIconsReuseScannedFingerprint(t *testing.T) {
	s := testService(t)
	r := Root{ID: "root", Path: t.TempDir()}
	s.cfg.Roots = []Root{r}
	s.publisher = publisherconfig.Config{CurseForgeAPIKey: "test-key"}
	path := filepath.Join(r.VersionDir(), "mods", "one.jar")
	testJar(t, path, "one", "One", nil)
	mods, err := s.ListMods(r.ID)
	if err != nil || len(mods) != 1 || mods[0].Fingerprint == 0 {
		t.Fatalf("scan failed: mods=%v err=%v", mods, err)
	}
	// The content hash and fingerprint already identify the same bytes. No
	// second file read should be necessary to look up their remote metadata.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	lookedUp := false
	s.http = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		body := `{}`
		switch req.URL.Path {
		case "/v2/version_files":
		case "/v1/fingerprints":
			var q struct {
				Fingerprints []uint32 `json:"fingerprints"`
			}
			if err := json.NewDecoder(req.Body).Decode(&q); err != nil || !slices.Equal(q.Fingerprints, []uint32{mods[0].Fingerprint}) {
				t.Fatalf("scan fingerprint was not reused: %v %v", q.Fingerprints, err)
			}
			lookedUp = true
		default:
			t.Fatalf("unexpected request: %s", req.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	if _, err := s.ModIconURLs(context.Background(), r.ID, "", mods); err != nil || !lookedUp {
		t.Fatalf("fingerprint lookup failed: lookedUp=%v err=%v", lookedUp, err)
	}
}
