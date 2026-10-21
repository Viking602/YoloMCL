// Package catalogrelay keeps the publisher's CurseForge key on the publisher's
// server. The desktop client receives only the HTTPS catalog URL.
package catalogrelay

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var catalogPath = regexp.MustCompile(`^/mods/[0-9]+(?:/files(?:/[0-9]+)?)?$`)

type Handler struct {
	key    string
	client *http.Client
	slots  chan struct{}
}

func New(key string, transport http.RoundTripper) *Handler {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &Handler{key: key, client: &http.Client{Transport: transport, Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, slots: make(chan struct{}, 24)}
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.key == "" {
		http.Error(w, "catalog service unavailable", http.StatusServiceUnavailable)
		return
	}
	valid := r.Method == http.MethodGet && (r.URL.Path == "/mods/search" || catalogPath.MatchString(r.URL.Path)) || r.Method == http.MethodPost && r.URL.Path == "/fingerprints"
	if !valid || r.URL.RawPath != "" || strings.Contains(r.URL.RawQuery, "\r") {
		http.Error(w, "unsupported catalog operation", http.StatusNotFound)
		return
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		http.Error(w, "catalog busy", http.StatusTooManyRequests)
		return
	}
	var body io.Reader
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
		b, e := io.ReadAll(r.Body)
		if e != nil {
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		body = strings.NewReader(string(b))
	}
	u := url.URL{Scheme: "https", Host: "api.curseforge.com", Path: "/v1" + r.URL.Path, RawQuery: r.URL.RawQuery}
	req, e := http.NewRequestWithContext(r.Context(), r.Method, u.String(), body)
	if e != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	req.Header.Set("x-api-key", h.key)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "MCL-Catalog/0.2.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, e := h.client.Do(req)
	if e != nil {
		http.Error(w, "upstream catalog unavailable", http.StatusBadGateway)
		return
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		http.Error(w, "unexpected upstream redirect", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(res.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(res.Body, 12<<20))
}
