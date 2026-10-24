package launcher

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNetFaultSwitch(t *testing.T) {
	base := &recordingTransport{}
	t.Setenv("MCL_DEBUG_NETFAIL_FILE", "")
	if withNetFault(base) != base {
		t.Fatal("unset var must not wrap")
	}
	p := filepath.Join(t.TempDir(), "off")
	t.Setenv("MCL_DEBUG_NETFAIL_FILE", p)
	rt := withNetFault(base)
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("GET", "https://example.invalid/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, e := rt.RoundTrip(req); e == nil || !strings.HasPrefix(e.Error(), "debug netfail:") {
		t.Fatalf("want fault, got %v", e)
	}
	if base.calls != 0 {
		t.Fatal("fault request reached the underlying transport")
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	res, err := rt.RoundTrip(req)
	if err != nil || base.calls != 1 {
		t.Fatalf("transport was not restored: calls=%d, err=%v", base.calls, err)
	}
	res.Body.Close()
}

type recordingTransport struct{ calls int }

func (t *recordingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.calls++
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}
