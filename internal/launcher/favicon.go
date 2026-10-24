package launcher

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	iconMaxBytes   = 512 << 10
	iconTimeout    = 5 * time.Second
	iconCacheTTL   = 7 * 24 * time.Hour
	iconMaxPixels  = 64
	iconFailureTTL = 6 * time.Hour
)

// AuthServerIcon returns a PNG (≤64px) of the auth server's site icon. It
// resolves links.homepage from the Yggdrasil API root → <link rel=icon> on
// that page → /favicon.ico on the homepage origin → /favicon.ico on the API
// origin. Results (and failures, briefly) are cached on disk.
func (s *Service) AuthServerIcon(ctx context.Context, server string) ([]byte, error) {
	u, e := NormalizeAuthServerURL(server)
	if e != nil {
		return nil, e
	}
	dir := filepath.Join(s.dir, "cache", "authicons")
	file := filepath.Join(dir, serverID(strings.ToLower(u))+".png")
	miss := file + ".miss"
	if st, e := os.Stat(file); e == nil && time.Since(st.ModTime()) < iconCacheTTL {
		return os.ReadFile(file)
	}
	if st, e := os.Stat(miss); e == nil && time.Since(st.ModTime()) < iconFailureTTL {
		return nil, errors.New("认证服务器没有可用图标")
	}
	b, e := s.resolveAuthIcon(ctx, u)
	_ = os.MkdirAll(dir, 0755)
	if e != nil {
		_ = os.WriteFile(miss, nil, 0644)
		if old, e2 := os.ReadFile(file); e2 == nil {
			return old, nil // keep a stale icon rather than none
		}
		return nil, e
	}
	_ = os.Remove(miss)
	if e := os.WriteFile(file, b, 0644); e != nil {
		return b, nil
	}
	return b, nil
}

func (s *Service) resolveAuthIcon(ctx context.Context, api string) ([]byte, error) {
	var candidates []string
	var meta struct {
		Meta struct {
			Links struct {
				Homepage string `json:"homepage"`
			} `json:"links"`
		} `json:"meta"`
	}
	mctx, cancel := context.WithTimeout(ctx, iconTimeout)
	_ = s.jsonRequest(mctx, "GET", api, nil, nil, &meta)
	cancel()
	if hp := strings.TrimSpace(meta.Meta.Links.Homepage); validateURL(hp) == nil {
		if html, e := s.fetchLimited(ctx, hp); e == nil {
			candidates = append(candidates, iconLinks(hp, string(html))...)
		}
		if o := originOf(hp); o != "" {
			candidates = append(candidates, o+"/favicon.ico")
		}
	}
	if o := originOf(api); o != "" {
		candidates = append(candidates, o+"/favicon.ico")
	}
	seen := map[string]bool{}
	last := errors.New("认证服务器没有可用图标")
	for _, c := range candidates {
		if seen[c] || validateURL(c) != nil {
			continue
		}
		seen[c] = true
		b, e := s.fetchLimited(ctx, c)
		if e != nil {
			last = e
			continue
		}
		img, e := decodeIcon(b)
		if e != nil {
			last = e
			continue
		}
		return encodeIconPNG(img)
	}
	return nil, last
}

func originOf(raw string) string {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Host == "" {
		return ""
	}
	return "https://" + u.Host
}

// fetchLimited GETs an HTTPS URL with a short timeout and a size cap.
func (s *Service) fetchLimited(ctx context.Context, raw string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, iconTimeout)
	defer cancel()
	res, e := s.request(ctx, "GET", raw, nil, nil)
	if e != nil {
		return nil, e
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, iconMaxBytes+1))
	if e != nil {
		return nil, e
	}
	if len(b) > iconMaxBytes {
		return nil, errors.New("图标文件过大")
	}
	return b, nil
}

var (
	linkTag  = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	attrExpr = regexp.MustCompile(`(?is)([a-z-]+)\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
)

// iconLinks extracts <link rel="icon|shortcut icon|apple-touch-icon"> hrefs,
// best first: the smallest declared size ≥32, then larger, then unsized,
// then smaller. SVG is skipped.
func iconLinks(page, html string) []string {
	base, e := url.Parse(page)
	if e != nil {
		return nil
	}
	type cand struct {
		href string
		size int
	}
	var list []cand
	for _, tag := range linkTag.FindAllString(html, -1) {
		attrs := map[string]string{}
		for _, m := range attrExpr.FindAllStringSubmatch(tag, -1) {
			attrs[strings.ToLower(m[1])] = strings.Trim(m[2], `"'`)
		}
		rel := strings.ToLower(attrs["rel"])
		if !strings.Contains(rel, "icon") || attrs["href"] == "" {
			continue
		}
		if strings.Contains(strings.ToLower(attrs["type"]), "svg") || strings.HasSuffix(strings.ToLower(strings.SplitN(attrs["href"], "?", 2)[0]), ".svg") {
			continue
		}
		ref, e := url.Parse(attrs["href"])
		if e != nil {
			continue
		}
		size := 0
		for _, f := range strings.Fields(strings.ToLower(attrs["sizes"])) {
			if w, _, ok := strings.Cut(f, "x"); ok {
				if n, e := strconv.Atoi(w); e == nil && n > size {
					size = n
				}
			}
		}
		if size == 0 && strings.Contains(rel, "apple-touch-icon") {
			size = 180
		}
		list = append(list, cand{base.ResolveReference(ref).String(), size})
	}
	rank := func(n int) int {
		switch {
		case n >= 32:
			return n // smallest ≥32 first
		case n == 0:
			return 1 << 20
		default:
			return 2<<20 - n
		}
	}
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && rank(list[j].size) < rank(list[j-1].size); j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
	out := make([]string, len(list))
	for i, c := range list {
		out[i] = c.href
	}
	return out
}

// decodeIcon decodes ICO (largest entry; PNG or BMP payload) and any format
// registered with image (png/jpeg/gif/webp).
func decodeIcon(b []byte) (image.Image, error) {
	if len(b) >= 6 && binary.LittleEndian.Uint16(b[0:]) == 0 && binary.LittleEndian.Uint16(b[2:]) == 1 {
		return decodeICO(b)
	}
	img, _, e := image.Decode(bytes.NewReader(b))
	if e != nil {
		return nil, errors.New("不是有效的图片")
	}
	return img, nil
}

func decodeICO(b []byte) (image.Image, error) {
	n := int(binary.LittleEndian.Uint16(b[4:]))
	if n == 0 || len(b) < 6+16*n {
		return nil, errors.New("ICO 文件损坏")
	}
	best, bestArea := -1, -1
	for i := 0; i < n; i++ {
		e := b[6+16*i:]
		w, h := int(e[0]), int(e[1])
		if w == 0 {
			w = 256
		}
		if h == 0 {
			h = 256
		}
		if w*h > bestArea {
			best, bestArea = i, w*h
		}
	}
	e := b[6+16*best:]
	size := int(binary.LittleEndian.Uint32(e[8:]))
	off := int(binary.LittleEndian.Uint32(e[12:]))
	if off < 0 || size <= 0 || off+size > len(b) {
		return nil, errors.New("ICO 文件损坏")
	}
	data := b[off : off+size]
	if bytes.HasPrefix(data, []byte("\x89PNG")) {
		return png.Decode(bytes.NewReader(data))
	}
	return decodeDIB(data)
}

// decodeDIB decodes a 32-bit or 24-bit (with AND mask) icon bitmap.
func decodeDIB(d []byte) (image.Image, error) {
	if len(d) < 40 {
		return nil, errors.New("ICO 位图损坏")
	}
	hdr := int(binary.LittleEndian.Uint32(d[0:]))
	w := int(int32(binary.LittleEndian.Uint32(d[4:])))
	h := int(int32(binary.LittleEndian.Uint32(d[8:]))) / 2
	bpp := int(binary.LittleEndian.Uint16(d[14:]))
	if w <= 0 || h <= 0 || w > 512 || h > 512 || (bpp != 32 && bpp != 24 && bpp != 8 && bpp != 4 && bpp != 1) {
		return nil, errors.New("不支持的 ICO 位图格式")
	}
	stride := ((w*bpp + 31) / 32) * 4
	maskStride := ((w + 31) / 32) * 4
	var pal []color.NRGBA
	if bpp <= 8 {
		n := int(binary.LittleEndian.Uint32(d[32:]))
		if n == 0 || n > 1<<bpp {
			n = 1 << bpp
		}
		if hdr < 40 || len(d) < hdr+n*4 {
			return nil, errors.New("ICO 位图损坏")
		}
		for i := 0; i < n; i++ {
			q := d[hdr+i*4:]
			pal = append(pal, color.NRGBA{q[2], q[1], q[0], 255})
		}
		hdr += n * 4
	}
	px := d[hdr:]
	if len(px) < stride*h {
		return nil, errors.New("ICO 位图损坏")
	}
	mask := px[stride*h:]
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		row := px[(h-1-y)*stride:]
		for x := 0; x < w; x++ {
			var c color.NRGBA
			if bpp == 32 {
				p := row[x*4:]
				c = color.NRGBA{p[2], p[1], p[0], p[3]}
			} else {
				if bpp == 24 {
					p := row[x*3:]
					c = color.NRGBA{p[2], p[1], p[0], 255}
				} else {
					bit := x * bpp
					idx := int(row[bit/8]>>(8-bpp-bit%8)) & (1<<bpp - 1)
					if idx < len(pal) {
						c = pal[idx]
					}
				}
				if len(mask) >= maskStride*h {
					m := mask[(h-1-y)*maskStride+x/8]
					if m&(0x80>>(x%8)) != 0 {
						c.A = 0
					}
				}
			}
			out.SetNRGBA(x, y, c)
		}
	}
	return out, nil
}

// encodeIconPNG re-encodes an icon as PNG, smoothly downscaled to ≤64px.
func encodeIconPNG(img image.Image) ([]byte, error) {
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return nil, errors.New("空图标")
	}
	if b.Dx() > iconMaxPixels || b.Dy() > iconMaxPixels {
		w, h := iconMaxPixels, iconMaxPixels
		if b.Dx() > b.Dy() {
			h = max(1, iconMaxPixels*b.Dy()/b.Dx())
		} else {
			w = max(1, iconMaxPixels*b.Dx()/b.Dy())
		}
		dst := image.NewNRGBA(image.Rect(0, 0, w, h))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Src, nil)
		img = dst
	}
	var buf bytes.Buffer
	if e := png.Encode(&buf, img); e != nil {
		return nil, e
	}
	return buf.Bytes(), nil
}
