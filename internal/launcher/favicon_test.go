package launcher

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func pngOf(w, h int, c color.NRGBA) []byte {
	im := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(im.Pix); i += 4 {
		im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	var b bytes.Buffer
	_ = png.Encode(&b, im)
	return b.Bytes()
}

// icoOf builds an ICO with a 16px BMP entry and a 48px PNG entry.
func icoOf() []byte {
	w := 16
	dib := make([]byte, 40+w*w*4+((w+31)/32)*4*w)
	binary.LittleEndian.PutUint32(dib[0:], 40)
	binary.LittleEndian.PutUint32(dib[4:], uint32(w))
	binary.LittleEndian.PutUint32(dib[8:], uint32(w*2))
	binary.LittleEndian.PutUint16(dib[12:], 1)
	binary.LittleEndian.PutUint16(dib[14:], 32)
	for i := 40; i < 40+w*w*4; i += 4 {
		dib[i], dib[i+1], dib[i+2], dib[i+3] = 0, 0, 255, 255 // BGRA red
	}
	big := pngOf(48, 48, color.NRGBA{0, 0, 255, 255})
	var b bytes.Buffer
	_ = binary.Write(&b, binary.LittleEndian, []uint16{0, 1, 2})
	off := 6 + 32
	entry := func(sz int, n, o int) {
		b.Write([]byte{byte(sz), byte(sz), 0, 0})
		_ = binary.Write(&b, binary.LittleEndian, []uint16{1, 32})
		_ = binary.Write(&b, binary.LittleEndian, []uint32{uint32(n), uint32(o)})
	}
	entry(16, len(dib), off)
	entry(48, len(big), off+len(dib))
	b.Write(dib)
	b.Write(big)
	return b.Bytes()
}

func TestDecodeICOPicksLargest(t *testing.T) {
	img, e := decodeIcon(icoOf())
	if e != nil {
		t.Fatal(e)
	}
	if img.Bounds().Dx() != 48 {
		t.Fatalf("largest entry not chosen: %v", img.Bounds())
	}
	// BMP entry alone
	ico := icoOf()
	binary.LittleEndian.PutUint16(ico[4:], 1)
	img, e = decodeIcon(ico)
	if e != nil || img.Bounds().Dx() != 16 {
		t.Fatalf("bmp entry: %v %v", img, e)
	}
	if r, _, _, a := img.At(3, 3).RGBA(); r>>8 != 255 || a>>8 != 255 {
		t.Error("BGRA not converted")
	}
	if _, e := decodeIcon([]byte("<html>not an image</html>")); e == nil {
		t.Error("non-image must fail")
	}
}

func TestIconLinksOrder(t *testing.T) {
	html := `<link rel="icon" href="/16.png" sizes="16x16"><link rel="shortcut icon" href="/a.svg">
<link rel='apple-touch-icon' href='/touch.png'><link rel="icon" href="https://cdn.example/64.png" sizes="64x64"><link rel=icon href=/32.png sizes=32x32>`
	got := iconLinks("https://site.example/home/", html)
	want := []string{"https://site.example/32.png", "https://cdn.example/64.png", "https://site.example/touch.png", "https://site.example/16.png"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v", got)
	}
}

type iconSites struct {
	api, home *httptest.Server
	hits      map[string]*int32
}

func newIconSites(t *testing.T, homeHandler, apiFavicon http.HandlerFunc) *iconSites {
	t.Helper()
	is := &iconSites{hits: map[string]*int32{}}
	count := func(k string) { atomic.AddInt32(is.hits[k], 1) }
	for _, k := range []string{"meta", "page", "link", "homefav", "apifav"} {
		is.hits[k] = new(int32)
	}
	is.home = httptest.NewTLSServer(homeHandler)
	is.api = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/yggdrasil":
			count("meta")
			w.Write([]byte(`{"meta":{"serverName":"Test","links":{"homepage":"` + is.home.URL + `/"}}}`))
		case "/favicon.ico":
			count("apifav")
			apiFavicon(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(is.api.Close)
	t.Cleanup(is.home.Close)
	return is
}

func iconService(t *testing.T, is *iconSites) *Service {
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	s.http = is.api.Client()
	return s
}

func colorOf(t *testing.T, b []byte) color.NRGBA {
	img, e := png.Decode(bytes.NewReader(b))
	if e != nil {
		t.Fatal(e)
	}
	return color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA)
}

func TestAuthServerIconResolutionOrder(t *testing.T) {
	red, green, blue := color.NRGBA{255, 0, 0, 255}, color.NRGBA{0, 255, 0, 255}, color.NRGBA{0, 0, 255, 255}
	apiFav := func(w http.ResponseWriter, r *http.Request) { w.Write(pngOf(32, 32, blue)) }

	// 1. <link rel=icon> on homepage wins
	is := newIconSites(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`<html><head><link rel="icon" sizes="32x32" href="/i.png"></head></html>`))
		case "/i.png":
			w.Write(pngOf(256, 256, red))
		case "/favicon.ico":
			w.Write(pngOf(16, 16, green))
		}
	}, apiFav)
	s := iconService(t, is)
	b, e := s.AuthServerIcon(t.Context(), is.api.URL+"/api/yggdrasil")
	if e != nil || colorOf(t, b) != red {
		t.Fatalf("link icon expected: %v", e)
	}
	if img, _ := png.Decode(bytes.NewReader(b)); img.Bounds().Dx() != 64 {
		t.Errorf("large icon should be downscaled to 64, got %v", img.Bounds())
	}
	// cache hit: no new requests
	before := atomic.LoadInt32(is.hits["meta"])
	if _, e := s.AuthServerIcon(t.Context(), is.api.URL+"/api/yggdrasil/"); e != nil {
		t.Fatal(e)
	}
	if atomic.LoadInt32(is.hits["meta"]) != before {
		t.Error("cached icon should not refetch")
	}

	// 2. no link → homepage /favicon.ico (ICO)
	is = newIconSites(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`<html>no icons</html>`))
		case "/favicon.ico":
			w.Write(icoOf())
		}
	}, apiFav)
	b, e = iconService(t, is).AuthServerIcon(t.Context(), is.api.URL+"/api/yggdrasil")
	if e != nil || colorOf(t, b) != blue || func() bool { im, _ := png.Decode(bytes.NewReader(b)); return im.Bounds().Dx() != 48 }() {
		t.Fatalf("homepage favicon.ico expected: %v", e)
	}

	// 3. homepage has nothing (404 / oversize / non-image) → API origin favicon
	for name, h := range map[string]http.HandlerFunc{
		"404":       func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) },
		"oversize":  func(w http.ResponseWriter, r *http.Request) { w.Write(bytes.Repeat([]byte{1}, iconMaxBytes+10)) },
		"non-image": func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>hi</html>")) },
	} {
		is = newIconSites(t, h, apiFav)
		b, e = iconService(t, is).AuthServerIcon(t.Context(), is.api.URL+"/api/yggdrasil")
		if e != nil || colorOf(t, b) != blue {
			t.Errorf("%s: api favicon expected: %v", name, e)
		}
	}

	// 4. nothing anywhere → error (UI falls back to globe); failure cached
	none := func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }
	is = newIconSites(t, none, none)
	s = iconService(t, is)
	if _, e := s.AuthServerIcon(t.Context(), is.api.URL+"/api/yggdrasil"); e == nil {
		t.Error("expected failure")
	}
	before = atomic.LoadInt32(is.hits["meta"])
	_, _ = s.AuthServerIcon(t.Context(), is.api.URL+"/api/yggdrasil")
	if atomic.LoadInt32(is.hits["meta"]) != before {
		t.Error("recent failure should be cached")
	}
	if _, e := s.AuthServerIcon(t.Context(), "http://insecure.example"); e == nil {
		t.Error("http must be rejected")
	}
}
