package launcher

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/image/draw"
)

type InstanceScreenshot struct {
	File     string    `json:"file"`
	Modified time.Time `json:"modified"`
	Size     int64     `json:"size"`
}

// ScreenshotPreview produces a bounded thumbnail for the overview's image
// cache. Original multi-megapixel screenshots never stay in the UI cache.
// Call from an image worker, not from the render thread.
func (s *Service) ScreenshotPreview(rootID, versionID, file string) ([]byte, error) {
	if file == "" || filepath.Base(file) != file || strings.ContainsAny(file, `/\`) || !strings.EqualFold(filepath.Ext(file), ".png") {
		return nil, errors.New("invalid screenshot name")
	}
	r, e := s.rootAt(rootID, versionID)
	if e != nil {
		return nil, e
	}
	p, e := safeJoin(r.VersionDir(), "screenshots/"+file)
	if e != nil {
		return nil, e
	}
	f, e := os.Open(p)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	fi, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !fi.Mode().IsRegular() || fi.Size() > 32<<20 {
		return nil, errors.New("screenshot too large")
	}
	cfg, _, e := image.DecodeConfig(io.LimitReader(f, 32<<20))
	if e != nil {
		return nil, e
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 8192 || cfg.Height > 8192 || int64(cfg.Width)*int64(cfg.Height) > 32<<20 {
		return nil, errors.New("screenshot dimensions too large")
	}
	if _, e = f.Seek(0, io.SeekStart); e != nil {
		return nil, e
	}
	src, _, e := image.Decode(io.LimitReader(f, 32<<20))
	if e != nil {
		return nil, e
	}
	w, h := cfg.Width, cfg.Height
	if edge := max(w, h); edge > 960 {
		w, h = max(1, w*960/edge), max(1, h*960/edge)
	}
	thumb := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.ApproxBiLinear.Scale(thumb, thumb.Bounds(), src, src.Bounds(), draw.Src, nil)
	var out bytes.Buffer
	if e := jpeg.Encode(&out, thumb, &jpeg.Options{Quality: 86}); e != nil {
		return nil, e
	}
	return out.Bytes(), nil
}
