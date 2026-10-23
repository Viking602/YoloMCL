package launcher

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func decodeJSON(r io.Reader, v any) error { return json.NewDecoder(r).Decode(v) }
func validateURL(raw string) error {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return errors.New("地址必须为有效的 HTTPS URL")
	}
	return nil
}
func (s *Service) request(ctx context.Context, method, raw string, body io.Reader, headers map[string]string) (*http.Response, error) {
	if err := validateURL(raw); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, raw, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "YoloMCL/"+appVersion+" (Minecraft desktop launcher)")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	var resp *http.Response
	attempts := 1
	if method == "GET" || method == "HEAD" || (method == "POST" && (strings.HasPrefix(raw, modrinthAPI+"/version_files") || strings.HasPrefix(raw, curseAPI+"/fingerprints"))) {
		attempts = 3
	}
	for attempt := 0; attempt < attempts; attempt++ {
		client := s.http
		if attempt == 1 {
			// Some CDNs reject HTTP/1-only negotiation. Retry a failed read
			// with HTTP/2, preserving the same certificates and redirect policy.
			if tr, ok := s.http.Transport.(*http.Transport); ok {
				tr = tr.Clone()
				tr.ForceAttemptHTTP2 = true
				tr.Protocols = new(http.Protocols)
				tr.Protocols.SetHTTP1(true)
				tr.Protocols.SetHTTP2(true)
				if tr.TLSClientConfig != nil {
					tr.TLSClientConfig = tr.TLSClientConfig.Clone()
					tr.TLSClientConfig.NextProtos = []string{"h2", "http/1.1"}
				}
				defer tr.CloseIdleConnections()
				copyClient := *s.http
				copyClient.Transport = tr
				client = &copyClient
			}
		}
		var next *http.Request
		if attempt == 0 {
			next = req
		} else {
			next = req.Clone(ctx)
			if req.GetBody != nil {
				next.Body, err = req.GetBody()
				if err != nil {
					return nil, err
				}
			}
		}
		resp, err = client.Do(next)
		if err == nil {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		if attempt+1 < attempts {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 250 * time.Millisecond):
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("网络请求失败 (%s): %w", req.URL.Host, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, fmt.Errorf("%s 返回 HTTP %d", req.URL.Host, resp.StatusCode)
	}
	return resp, nil
}
func (s *Service) jsonRequest(ctx context.Context, method, raw string, body any, headers map[string]string, out any) error {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	var r io.Reader
	h := map[string]string{"Accept": "application/json"}
	for k, v := range headers {
		h[k] = v
	}
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return e
		}
		r = bytes.NewReader(b)
		h["Content-Type"] = "application/json"
	}
	res, e := s.request(ctx, method, raw, r, h)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	if out == nil {
		_, e = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<20))
		return e
	}
	return decodeJSON(io.LimitReader(res.Body, 32<<20), out)
}
func (s *Service) getJSON(ctx context.Context, raw string, out any) error {
	return s.jsonRequest(ctx, "GET", raw, nil, nil, out)
}
func (s *Service) download(ctx context.Context, f DownloadFile, dest string) error {
	if checkHashes(dest, f.SHA1, f.SHA512) {
		return nil
	}
	if f.SHA256 != "" {
		if b, e := os.ReadFile(dest); e == nil {
			sum := sha256.Sum256(b)
			if strings.EqualFold(hex.EncodeToString(sum[:]), f.SHA256) {
				return nil
			}
		}
	}
	if err := validateURL(f.URL); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		last = s.downloadOnce(ctx, f, dest)
		if last == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 350 * time.Millisecond):
		}
	}
	return last
}

// downloadOnce fetches f into dest through dest+".mcl-part". A part left
// by an earlier attempt is resumed with an HTTP Range request when the
// file has a checksum to prove the joined result; otherwise, or if the
// server ignores the range, it starts over. A bad result drops the part.
func (s *Service) downloadOnce(ctx context.Context, f DownloadFile, dest string) error {
	part := dest + ".mcl-part"
	s.trackCreated(ctx, part)
	var have int64
	if st, e := os.Stat(part); e == nil && (f.SHA1 != "" || f.SHA512 != "" || f.SHA256 != "") && (f.Size == 0 || st.Size() < f.Size) {
		have = st.Size()
	}
	var hdr map[string]string
	if have > 0 {
		hdr = map[string]string{"Range": fmt.Sprintf("bytes=%d-", have)}
	}
	res, err := s.request(ctx, "GET", f.URL, nil, hdr)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	a, b, c := sha1.New(), sha512.New(), sha256.New()
	flag := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if have > 0 && res.StatusCode == http.StatusPartialContent {
		old, e := os.Open(part) // hash what is already there, then append
		if e != nil {
			return e
		}
		_, e = io.Copy(io.MultiWriter(a, b, c), old)
		old.Close()
		if e != nil {
			return e
		}
		flag = os.O_WRONLY | os.O_APPEND
	} else {
		have = 0
	}
	out, err := os.OpenFile(part, flag, 0o644)
	if err != nil {
		return err
	}
	fail := func(e error) error {
		out.Close()
		if !errors.Is(e, context.Canceled) && !isNetErr(e) {
			_ = os.Remove(part) // corrupt, not just interrupted
		}
		return e
	}
	addJobFileTotal(ctx, f.Size-have)
	n, err := io.Copy(io.MultiWriter(out, a, b, c, jobMeter{ctx}), io.LimitReader(res.Body, (4<<30)+1))
	n += have
	if err != nil {
		if ctx.Err() != nil {
			return fail(ctx.Err())
		}
		return fail(err)
	}
	if n > 4<<30 {
		return fail(errors.New("下载文件超过 4 GiB 限制"))
	}
	bad := func(msg string) error { return fail(fmt.Errorf("%s: %s", msg, f.Name)) }
	if f.Size > 0 && n != f.Size {
		return bad("文件大小不匹配")
	}
	if f.SHA1 != "" && !strings.EqualFold(f.SHA1, hex.EncodeToString(a.Sum(nil))) {
		return bad("SHA-1 校验失败")
	}
	if f.SHA512 != "" && !strings.EqualFold(f.SHA512, hex.EncodeToString(b.Sum(nil))) {
		return bad("SHA-512 校验失败")
	}
	if f.SHA256 != "" && !strings.EqualFold(f.SHA256, hex.EncodeToString(c.Sum(nil))) {
		return bad("SHA-256 校验失败")
	}
	if err = out.Sync(); err != nil {
		return fail(err)
	}
	if err = out.Close(); err != nil {
		return err
	}
	s.trackCreated(ctx, dest)
	return os.Rename(part, dest)
}

// isNetErr is an interrupted transfer, worth resuming rather than redoing.
func isNetErr(e error) bool {
	var ne net.Error
	return errors.As(e, &ne) || errors.Is(e, io.ErrUnexpectedEOF) || strings.Contains(e.Error(), "connection")
}

// jobMeter counts a download's bytes toward its task's live speed.
type jobMeter struct{ ctx context.Context }

func (m jobMeter) Write(p []byte) (int, error) {
	addJobBytes(m.ctx, int64(len(p)))
	return len(p), nil
}
