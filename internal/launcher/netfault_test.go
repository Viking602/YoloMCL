package launcher

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestNetFaultSwitch(t *testing.T) {
	base := http.DefaultTransport
	t.Setenv("MCL_DEBUG_NETFAIL_FILE", "")
	if WithNetFault(base) != base {
		t.Fatal("unset var must not wrap")
	}
	p := filepath.Join(t.TempDir(), "off")
	t.Setenv("MCL_DEBUG_NETFAIL_FILE", p)
	rt := WithNetFault(base)
	os.WriteFile(p, nil, 0o644)
	req, _ := http.NewRequest("GET", "http://127.0.0.1:1/", nil)
	if _, e := rt.RoundTrip(req); e == nil || e.Error()[:14] != "debug netfail:" {
		t.Fatalf("want fault, got %v", e)
	}
	os.Remove(p)
	if _, e := rt.RoundTrip(req); e != nil && e.Error()[:14] == "debug netfail:" {
		t.Fatal("fault after file removed")
	}
}
