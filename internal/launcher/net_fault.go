package launcher

import (
	"errors"
	"net/http"
	"os"
)

// Debug-only fault switch: when MCL_DEBUG_NETFAIL_FILE names a file that
// exists, every request fails as if offline. Unset, the transport is untouched.
type netFault struct {
	base http.RoundTripper
	file string
}

func (f netFault) RoundTrip(r *http.Request) (*http.Response, error) {
	if _, e := os.Stat(f.file); e == nil {
		return nil, errors.New("debug netfail: dial tcp " + r.URL.Host + ": connection refused")
	}
	return f.base.RoundTrip(r)
}

// withNetFault wraps rt with the debug fault switch when it is enabled.
// It never mutates the process-wide HTTP client or default transport.
func withNetFault(rt http.RoundTripper) http.RoundTripper {
	if p := os.Getenv("MCL_DEBUG_NETFAIL_FILE"); p != "" {
		return netFault{rt, p}
	}
	return rt
}
