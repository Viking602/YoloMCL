package launcher

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

// ResourceImage negotiates HTTP/2 with image CDNs. It shares the launcher's
// certificate and redirect checks, without the HTTP/1-only game CDN setting.
func (s *Service) ResourceImage(ctx context.Context, raw string) ([]byte, error) {
	if e := validateURL(raw); e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	client := *s.http
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
		client.Transport = tr
		defer tr.CloseIdleConnections()
	}
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		req, e := http.NewRequestWithContext(ctx, "GET", raw, nil)
		if e != nil {
			return nil, e
		}
		req.Header.Set("User-Agent", "MCL/"+appVersion)
		req.Header.Set("Accept", "image/webp,image/svg+xml,image/*;q=0.9,*/*;q=0.5")
		res, e := client.Do(req)
		if e == nil {
			if res.StatusCode == 200 {
				b, readErr := io.ReadAll(io.LimitReader(res.Body, (4<<20)+1))
				res.Body.Close()
				if readErr == nil && len(b) <= 4<<20 {
					return b, nil
				}
				if readErr != nil {
					last = readErr
				} else {
					return nil, errors.New("资源图标超过 4 MiB")
				}
			} else {
				res.Body.Close()
				last = errors.New("资源图标服务暂时不可用")
				if res.StatusCode == 404 || res.StatusCode == 403 {
					return nil, last
				}
			}
		} else {
			last = e
		}
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 300 * time.Millisecond):
			}
		}
	}
	return nil, last
}
