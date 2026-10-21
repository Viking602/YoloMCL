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
	req.Header.Set("User-Agent", "MCL/"+appVersion+" (Minecraft desktop launcher)")
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
func (s *Service) downloadOnce(ctx context.Context, f DownloadFile, dest string) error {
	res, err := s.request(ctx, "GET", f.URL, nil, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".mcl-download-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	a, b, c := sha1.New(), sha512.New(), sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, a, b, c), io.LimitReader(res.Body, (4<<30)+1))
	if err != nil {
		tmp.Close()
		return err
	}
	if n > 4<<30 {
		tmp.Close()
		return errors.New("下载文件超过 4 GiB 限制")
	}
	if f.Size > 0 && n != f.Size {
		tmp.Close()
		return fmt.Errorf("文件大小不匹配: %s", f.Name)
	}
	if f.SHA1 != "" && !strings.EqualFold(f.SHA1, hex.EncodeToString(a.Sum(nil))) {
		tmp.Close()
		return fmt.Errorf("SHA-1 校验失败: %s", f.Name)
	}
	if f.SHA512 != "" && !strings.EqualFold(f.SHA512, hex.EncodeToString(b.Sum(nil))) {
		tmp.Close()
		return fmt.Errorf("SHA-512 校验失败: %s", f.Name)
	}
	if f.SHA256 != "" && !strings.EqualFold(f.SHA256, hex.EncodeToString(c.Sum(nil))) {
		tmp.Close()
		return fmt.Errorf("SHA-256 校验失败: %s", f.Name)
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dest)
}
