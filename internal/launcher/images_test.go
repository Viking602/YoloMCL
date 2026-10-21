package launcher

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestResourceImagesNegotiateHTTP2AndRetry(t *testing.T) {
	var body bytes.Buffer
	_ = png.Encode(&body, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	var calls atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor != 2 {
			t.Errorf("image CDN did not negotiate HTTP/2: %s", r.Proto)
		}
		if calls.Add(1) < 3 {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(body.Bytes())
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	s.http = server.Client()
	b, e := s.ResourceImage(context.Background(), server.URL+"/icon.png")
	if e != nil {
		t.Fatal(e)
	}
	if calls.Load() != 3 || !bytes.Equal(b, body.Bytes()) {
		t.Fatal("transient CDN response was not recovered")
	}
	if _, e = s.ResourceImage(context.Background(), "http://example.test/icon.png"); e == nil {
		t.Fatal("insecure image URL accepted")
	}
}

func TestDownloadReadRetriesHTTP2AfterHTTP1Disconnect(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 1 {
			conn, _, e := w.(http.Hijacker).Hijack()
			if e == nil {
				_ = conn.Close()
			}
			return
		}
		_, _ = w.Write([]byte("verified CDN data"))
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	s.http = server.Client()
	tr := s.http.Transport.(*http.Transport).Clone()
	tr.Protocols = new(http.Protocols)
	tr.Protocols.SetHTTP1(true)
	tr.ForceAttemptHTTP2 = false
	tr.TLSClientConfig = tr.TLSClientConfig.Clone()
	tr.TLSClientConfig.NextProtos = []string{"http/1.1"}
	s.http.Transport = tr
	res, e := s.request(context.Background(), "GET", server.URL+"/file", nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer res.Body.Close()
	if res.ProtoMajor != 2 {
		t.Fatal("failed HTTP/1 read did not recover through HTTP/2")
	}
}
