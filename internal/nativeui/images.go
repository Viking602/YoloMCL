package nativeui

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

func decodeResourceImage(b []byte) (ui.ImageSource, error) {
	trim := bytes.TrimSpace(b)
	if bytes.HasPrefix(trim, []byte("<")) {
		svg, e := ui.ParseSVG(trim)
		if e != nil {
			return nil, e
		}
		return svg, nil
	}
	config, _, e := image.DecodeConfig(bytes.NewReader(b))
	if e != nil {
		return nil, e
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > 4096 || config.Height > 4096 || config.Width*config.Height > 16<<20 {
		return nil, errors.New("资源图标尺寸过大")
	}
	return ui.DecodeBitmap(b)
}
func (a *App) remoteImage(raw string) ui.ImageSource {
	if raw == "" {
		return nil
	}
	a.imageMu.Lock()
	cached := a.images[raw]
	pending := a.imagePending[raw]
	retry := a.imageRetry[raw]
	if cached != nil || pending || time.Now().Before(retry) {
		a.imageMu.Unlock()
		return cached
	}
	a.imagePending[raw] = true
	a.imageMu.Unlock()
	dataDir := a.snapshot.DataDir
	go func() {
		select {
		case a.imageSlots <- struct{}{}:
			defer func() { <-a.imageSlots }()
		case <-a.ctx.Done():
			return
		}
		sum := sha256.Sum256([]byte(raw))
		file := filepath.Join(dataDir, "cache", "resource-icons", hex.EncodeToString(sum[:])+".img")
		var source ui.ImageSource
		var err error
		if b, e := os.ReadFile(file); e == nil {
			source, _ = decodeResourceImage(b)
		}
		if source == nil {
			var b []byte
			b, err = a.service.ResourceImage(a.ctx, raw)
			if err == nil {
				source, err = decodeResourceImage(b)
			}
			if err == nil {
				if e := os.MkdirAll(filepath.Dir(file), 0700); e == nil {
					if tmp, e := os.CreateTemp(filepath.Dir(file), ".icon-*"); e == nil {
						_, writeErr := tmp.Write(b)
						closeErr := tmp.Close()
						if writeErr == nil && closeErr == nil {
							_ = os.Rename(tmp.Name(), file)
						}
						_ = os.Remove(tmp.Name())
					}
				}
			}
		}
		a.post(func() {
			a.imageMu.Lock()
			a.imagePending[raw] = false
			if err != nil {
				a.imageRetry[raw] = time.Now().Add(15 * time.Second)
			} else {
				a.images[raw] = source
				delete(a.imageRetry, raw)
			}
			a.imageMu.Unlock()
		})
	}()
	return nil
}

func imageFromData(data string) image.Image {
	b, e := base64.StdEncoding.DecodeString(strings.TrimPrefix(data, "data:image/png;base64,"))
	if e != nil {
		return nil
	}
	img, _, _ := image.Decode(strings.NewReader(string(b)))
	return img
}
